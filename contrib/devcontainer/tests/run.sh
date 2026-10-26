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

# --- post-attach -----------------------------------------------------------
socks="$tmp/socks"
mkdir -p "$socks"
touch "$socks/vscode-ssh-auth-1234.sock" "$socks/other.sock"
GH_TOKEN_SSH_SOCKET_DIR="$socks" bash "$here/post-attach.sh" 2>"$tmp/err"
[ ! -e "$socks/vscode-ssh-auth-1234.sock" ] && pass "post-attach removes the forwarded SSH agent socket" ||
  fail "post-attach removes the forwarded SSH agent socket"
[ -e "$socks/other.sock" ] && pass "post-attach leaves other sockets alone" || fail "post-attach leaves other sockets alone"
grep -q "removed forwarded SSH agent socket" "$tmp/err" && pass "post-attach says what it removed" || fail "post-attach says what it removed"
GH_TOKEN_SSH_SOCKET_DIR="$socks" bash "$here/post-attach.sh" && pass "post-attach is idempotent" || fail "post-attach is idempotent"

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

mv "$pc/tokens/token" "$pc/tokens/token.bak"
if run_post_create 2>"$tmp/err"; then pass "post-create without a token succeeds"; else fail "post-create without a token succeeds"; fi
grep -q "WARNING: no token" "$tmp/err" && pass "post-create without a token warns" || fail "post-create without a token warns"
[ ! -e "$pc/bin/gh" ] && [ ! -e "$pc/bin/git-credential-gh-token" ] &&
  pass "post-create without a token installs nothing" || fail "post-create without a token installs nothing"
[ -z "$(gitcfg --get-all credential.https://github.com.helper 2>/dev/null)" ] &&
  pass "post-create without a token leaves git config alone" || fail "post-create without a token leaves git config alone"
mv "$pc/tokens/token.bak" "$pc/tokens/token"

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
