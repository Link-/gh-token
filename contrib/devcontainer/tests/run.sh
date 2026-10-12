#!/usr/bin/env bash
# Tests for the devcontainer scripts. Run: bash contrib/devcontainer/tests/run.sh
# shellcheck disable=SC2015 # pass always succeeds, so "A && pass || fail" is safe
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

failures=0
pass() { echo "ok   $1"; }
fail() {
  echo "FAIL $1" >&2
  failures=$((failures + 1))
}
check() { # name expected actual
  if [ "$2" = "$3" ]; then pass "$1"; else
    fail "$1"
    printf '  expected: %q\n  actual:   %q\n' "$2" "$3" >&2
  fi
}

# --- git credential helper -------------------------------------------------
helper="$here/git-credential-gh-token.sh"
export GH_TOKEN_DIR="$tmp/tokens"
mkdir -p "$GH_TOKEN_DIR"
printf 'ghs_abc\n' >"$GH_TOKEN_DIR/token"

out="$(printf 'protocol=https\nhost=github.com\npath=o/r.git\n\n' | bash "$helper" get)"
check "helper get answers for github.com" $'username=x-access-token\npassword=ghs_abc' "$out"

out="$(printf 'protocol=https\nhost=gitlab.com\n\n' | bash "$helper" get)"
check "helper get ignores other hosts" "" "$out"

out="$(printf 'protocol=http\nhost=github.com\n\n' | bash "$helper" get)"
check "helper get ignores plain http" "" "$out"

out="$(printf 'protocol=https\nhost=github.com\nusername=x\npassword=y\n\n' | bash "$helper" store)"
check "helper store is ignored" "" "$out"

out="$(printf 'protocol=https\nhost=github.com\n\n' | bash "$helper" erase)"
check "helper erase is ignored" "" "$out"

printf 'ghes.example.com\n' >"$GH_TOKEN_DIR/host"
out="$(printf 'protocol=https\nhost=ghes.example.com\n\n' | bash "$helper" get)"
check "helper get answers for the GHES host" $'username=x-access-token\npassword=ghs_abc' "$out"
out="$(printf 'protocol=https\nhost=github.com\n\n' | bash "$helper" get)"
check "helper get ignores github.com when serving GHES" "" "$out"
rm "$GH_TOKEN_DIR/host"

mv "$GH_TOKEN_DIR/token" "$GH_TOKEN_DIR/token.bak"
out="$(printf 'protocol=https\nhost=github.com\n\n' | bash "$helper" get 2>"$tmp/err")"
check "helper get without token answers quit" "quit=1" "$out"
grep -q "no token" "$tmp/err" && pass "helper explains missing token" || fail "helper explains missing token"
out="$(printf 'protocol=https\nhost=gitlab.com\n\n' | bash "$helper" get 2>/dev/null)"
check "helper without token leaves other hosts alone" "" "$out"
mv "$GH_TOKEN_DIR/token.bak" "$GH_TOKEN_DIR/token"

# Through git, with a second helper standing in for forwarded host credentials.
fallback="$tmp/fallback-helper"
cat >"$fallback" <<'FAKE'
#!/bin/sh
[ "$1" = get ] && printf 'username=x\npassword=HOST-CREDENTIAL\n'
exit 0
FAKE
chmod +x "$fallback"
git_fill() { # host
  printf 'protocol=https\nhost=%s\n\n' "$1" |
    HOME="$tmp/githome" GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0 git \
      -c "credential.helper=$helper" -c "credential.helper=$fallback" credential fill 2>&1
}
mkdir -p "$tmp/githome"
out="$(git_fill github.com || true)"
case "$out" in
  *"password=ghs_abc"*) pass "git credential fill uses the token" ;;
  *) fail "git credential fill uses the token: $out" ;;
esac
mv "$GH_TOKEN_DIR/token" "$GH_TOKEN_DIR/token.bak"
out="$(git_fill github.com || true)"
case "$out" in
  *HOST-CREDENTIAL*) fail "without a token git must not fall back to the next helper: $out" ;;
  *"told us to quit"*) pass "without a token git stops instead of falling back" ;;
  *) fail "without a token git stops instead of falling back: $out" ;;
esac
out="$(git_fill gitlab.com || true)"
case "$out" in
  *"password=HOST-CREDENTIAL"*) pass "other hosts still reach the next helper" ;;
  *) fail "other hosts still reach the next helper: $out" ;;
esac
mv "$GH_TOKEN_DIR/token.bak" "$GH_TOKEN_DIR/token"

# --- gh shim ---------------------------------------------------------------
# Tokens from the environment must not leak through the shim.
export GH_TOKEN=leaked GITHUB_TOKEN=leaked GH_ENTERPRISE_TOKEN=leaked GITHUB_ENTERPRISE_TOKEN=leaked
fake_gh="$tmp/real-gh"
cat >"$fake_gh" <<'FAKE'
#!/usr/bin/env bash
echo "GH_TOKEN=${GH_TOKEN:-} GH_ENTERPRISE_TOKEN=${GH_ENTERPRISE_TOKEN:-} GH_HOST=${GH_HOST:-}${GITHUB_TOKEN:+ GITHUB_TOKEN}${GITHUB_ENTERPRISE_TOKEN:+ GITHUB_ENTERPRISE_TOKEN} args=$*"
FAKE
chmod +x "$fake_gh"
export GH_TOKEN_REAL_GH="$fake_gh"
shim="$here/gh-shim.sh"

out="$(bash "$shim" pr list --limit 1)"
check "shim passes GH_TOKEN and args" "GH_TOKEN=ghs_abc GH_ENTERPRISE_TOKEN= GH_HOST= args=pr list --limit 1" "$out"

printf 'ghs_new\n' >"$GH_TOKEN_DIR/token"
out="$(bash "$shim" auth status)"
check "shim reads the token on every call" "GH_TOKEN=ghs_new GH_ENTERPRISE_TOKEN= GH_HOST= args=auth status" "$out"

printf 'ghes.example.com\n' >"$GH_TOKEN_DIR/host"
out="$(bash "$shim" api user)"
check "shim uses GH_ENTERPRISE_TOKEN for GHES" "GH_TOKEN= GH_ENTERPRISE_TOKEN=ghs_new GH_HOST=ghes.example.com args=api user" "$out"
rm "$GH_TOKEN_DIR/host"

printf '{\n  "expires_at": "2000-01-01T00:00:00Z"\n}\n' >"$GH_TOKEN_DIR/token.json"
bash "$shim" status >/dev/null 2>"$tmp/err"
if date -d "2000-01-01T00:00:00Z" +%s >/dev/null 2>&1; then
  grep -q "expired at 2000-01-01T00:00:00Z" "$tmp/err" && pass "shim warns about an expired token" || fail "shim warns about an expired token"
fi
rm "$GH_TOKEN_DIR/token.json"

rm "$GH_TOKEN_DIR/token"
if bash "$shim" status >/dev/null 2>"$tmp/err"; then fail "shim fails without a token"; else pass "shim fails without a token"; fi
grep -q "no token at" "$tmp/err" && pass "shim explains missing token" || fail "shim explains missing token"
unset GH_TOKEN_DIR GH_TOKEN_REAL_GH GH_TOKEN GITHUB_TOKEN GH_ENTERPRISE_TOKEN GITHUB_ENTERPRISE_TOKEN

# --- host-init -------------------------------------------------------------
fake_cli="$tmp/fake-gh-token.sh"
cat >"$fake_cli" <<'FAKE'
#!/usr/bin/env bash
# Records its arguments, then behaves according to FAKE_MODE.
printf '%s\n' "$@" >"$FAKE_ARGS"
out=""
while [ $# -gt 0 ]; do
  [ "$1" = "--output-file" ] && out="$2"
  shift
done
case "$FAKE_MODE" in
  ok) printf 'ghs_%s\n' "$RANDOM$RANDOM" >"$out"; exec sleep 30 ;;
  fail) echo "Error: failed resolving installation ID: unexpected status code: 404" >&2; exit 1 ;;
  running) exit 0 ;;
  hang) exec sleep 30 ;;
esac
FAKE

workspace="$tmp/workspace"
mkdir -p "$workspace/.devcontainer"
echo '{"customizations": {}}' >"$workspace/.devcontainer/devcontainer.json"
printf 'key' >"$tmp/key.pem"
export GH_TOKEN_APP_ID=123 GH_TOKEN_KEY="$tmp/key.pem" GH_TOKEN_CMD="bash $fake_cli" FAKE_ARGS="$tmp/args"
dir="$tmp/state/gh-token/abc"

run_init() { (cd "$workspace" && bash "$here/host-init.sh" "$@"); }
cleanup_fake() { pkill -f "$fake_cli" 2>/dev/null || true; }

FAKE_MODE=ok run_init "$dir" 2>"$tmp/err" && pass "host-init succeeds once a token is written" || {
  fail "host-init succeeds once a token is written"
  cat "$tmp/err" >&2
}
cleanup_fake
[ -s "$dir/token" ] && pass "host-init leaves the token in the token dir" || fail "host-init leaves the token in the token dir"
check "host-init writes the host file" "github.com" "$(cat "$dir/host")"
check "host-init token dir is 0700" "700" "$(stat -c %a "$dir" 2>/dev/null || stat -f %Lp "$dir")"
grep -qx -- "--refresh" "$tmp/args" && grep -qx -- "$dir/pid" "$tmp/args" && grep -qx -- "$dir/token" "$tmp/args" &&
  pass "host-init passes --refresh, --pid-file and --output-file" || fail "host-init passes --refresh, --pid-file and --output-file"
grep -qx -- "--hostname" "$tmp/args" && fail "host-init omits --hostname for github.com" || pass "host-init omits --hostname for github.com"

FAKE_MODE=running run_init "$dir" 2>"$tmp/err" && grep -q "already running" "$tmp/err" &&
  pass "host-init reuses a running refresher" || fail "host-init reuses a running refresher"

if FAKE_MODE=fail run_init "$dir" 2>"$tmp/err"; then fail "host-init fails when the refresher fails"; else
  pass "host-init fails when the refresher fails"
fi
grep -q "unexpected status code: 404" "$tmp/err" && grep -q "install it on the repository" "$tmp/err" &&
  pass "host-init shows the log tail and the 404 hint" || fail "host-init shows the log tail and the 404 hint"

if FAKE_MODE=hang GH_TOKEN_TIMEOUT=1 run_init "$dir" 2>"$tmp/err"; then fail "host-init times out"; else pass "host-init times out"; fi
cleanup_fake
grep -q "no token after 1s" "$tmp/err" && pass "host-init explains the timeout" || fail "host-init explains the timeout"

FAKE_MODE=ok GH_TOKEN_HOSTNAME=ghes.example.com run_init "$dir" 2>/dev/null
cleanup_fake
check "host-init writes the GHES host" "ghes.example.com" "$(cat "$dir/host")"
grep -qx -- "ghes.example.com" "$tmp/args" && pass "host-init passes --hostname for GHES" || fail "host-init passes --hostname for GHES"

if FAKE_MODE=ok run_init "$workspace/.tokens" 2>"$tmp/err"; then fail "host-init rejects a token dir inside the workspace"; else
  grep -q "inside the workspace" "$tmp/err" && pass "host-init rejects a token dir inside the workspace" || fail "host-init rejects a token dir inside the workspace"
fi

# shellcheck disable=SC2016 # literal, as written in devcontainer.json
XDG_RUNTIME_DIR="$tmp/xdg" FAKE_MODE=ok run_init '/x/${devcontainerId}' 2>/dev/null
cleanup_fake
compgen -G "$tmp/xdg/gh-token/*/token" >/dev/null && pass "host-init falls back to a hashed dir when \${devcontainerId} is not substituted" ||
  fail "host-init falls back to a hashed dir when \${devcontainerId} is not substituted"

if (unset GH_TOKEN_APP_ID && FAKE_MODE=ok run_init "$dir" 2>"$tmp/err"); then fail "host-init requires GH_TOKEN_APP_ID"; else
  grep -q "GH_TOKEN_APP_ID is not set" "$tmp/err" && pass "host-init requires GH_TOKEN_APP_ID" || fail "host-init requires GH_TOKEN_APP_ID"
fi

# --- post-create -----------------------------------------------------------
pc="$tmp/pc"
mkdir -p "$pc/bin" "$pc/usr-bin" "$pc/home" "$pc/tokens" "$pc/repo"
cat >"$pc/usr-bin/gh" <<'FAKE'
#!/usr/bin/env bash
echo "real gh token=${GH_TOKEN:-} args=$*" >>"$FAKE_GH_LOG"
FAKE
chmod +x "$pc/usr-bin/gh"
printf 'ghs_pc\n' >"$pc/tokens/token"
git -C "$pc/repo" init -q
git -C "$pc/repo" remote add origin git@github.com:my-user/some-fork.git
git -C "$pc/repo" remote add upstream https://github.com/upstream-org/some-fork.git

run_post_create() {
  (cd "$pc/repo" && HOME="$pc/home" GIT_CONFIG_NOSYSTEM=1 PATH="$pc/bin:$pc/usr-bin:$PATH" \
    GH_TOKEN_DIR="$pc/tokens" GH_TOKEN_BIN_DIR="$pc/bin" GH_TOKEN_LIBEXEC_DIR="$pc/libexec" \
    FAKE_GH_LOG="$pc/gh.log" bash "$here/post-create.sh")
}
gitcfg() { HOME="$pc/home" GIT_CONFIG_NOSYSTEM=1 git config --global "$@"; }

if run_post_create 2>"$tmp/err"; then pass "post-create succeeds"; else
  fail "post-create succeeds"
  cat "$tmp/err" >&2
fi
grep -q "gh shim:" "$pc/bin/gh" && pass "post-create installs the gh shim" || fail "post-create installs the gh shim"
grep -q "$pc/usr-bin/gh" "$pc/bin/gh" && pass "post-create points the shim at the real gh" || fail "post-create points the shim at the real gh"
[ -x "$pc/bin/git-credential-gh-token" ] && pass "post-create installs the credential helper" || fail "post-create installs the credential helper"
check "post-create configures only our helper" "$(printf '\n%s' "$pc/bin/git-credential-gh-token")" "$(gitcfg --get-all credential.https://github.com.helper)"
check "post-create sends SSH remotes through https" "$(printf 'git@github.com:\nssh://git@github.com/')" "$(gitcfg --get-all url.https://github.com/.insteadOf)"
grep -q "real gh token=ghs_pc args=repo set-default github.com/my-user/some-fork" "$pc/gh.log" &&
  pass "post-create sets the gh default repo to origin" || fail "post-create sets the gh default repo to origin"
out="$(cd "$pc/repo" && HOME="$pc/home" GIT_CONFIG_NOSYSTEM=1 GH_TOKEN_DIR="$pc/tokens" PATH="$pc/bin:$PATH" \
  git credential fill <<<$'protocol=https\nhost=github.com\n' 2>/dev/null)"
case "$out" in
  *"password=ghs_pc"*) pass "git credential fill uses the token" ;;
  *) fail "git credential fill uses the token" ;;
esac

if run_post_create 2>"$tmp/err"; then pass "post-create is idempotent"; else
  fail "post-create is idempotent"
  cat "$tmp/err" >&2
fi
grep -q "$pc/usr-bin/gh" "$pc/bin/gh" && pass "re-running keeps pointing at the real gh" || fail "re-running keeps pointing at the real gh"
check "re-running doesn't duplicate helpers" "$(printf '\n%s' "$pc/bin/git-credential-gh-token")" "$(gitcfg --get-all credential.https://github.com.helper)"

if [ "$failures" -gt 0 ]; then
  echo "$failures failure(s)" >&2
  exit 1
fi
echo "all devcontainer script tests passed"
