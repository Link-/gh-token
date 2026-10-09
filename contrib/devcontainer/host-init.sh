#!/usr/bin/env bash
# gh-token devcontainer integration: runs on the HOST, from initializeCommand.
#
# Starts `gh token generate --refresh` in the background, writing a
# downscoped installation token to a directory outside the workspace that
# devcontainer.json bind-mounts read-only at /run/gh-token. Waits until a
# fresh token is written so the container never starts without credentials.
#
# Usage: host-init.sh [token-dir]
#   token-dir defaults to ${XDG_RUNTIME_DIR:-$HOME/.cache}/gh-token/<hash of $PWD>.
#   Pass the same path as the mount source, e.g.
#   "${localEnv:HOME}/.cache/gh-token/${devcontainerId}".
#
# Environment (host):
#   GH_TOKEN_APP_ID      GitHub App ID (required)
#   GH_TOKEN_KEY         path to the App's private key (required; never enters the container)
#   GH_TOKEN_HOSTNAME    GitHub Enterprise Server hostname (optional, default github.com)
#   GH_TOKEN_SCOPE_FILE  scope file (optional, default .devcontainer/devcontainer.json).
#                        Point it outside the repository to pin the scope.
#   GH_TOKEN_CMD         command to run gh-token (optional, default gh-token or "gh token")
#   GH_TOKEN_TIMEOUT     seconds to wait for the first token (optional, default 30)
set -euo pipefail

die() {
  echo "gh-token host-init: $*" >&2
  exit 1
}

workspace="$(pwd -P)"

token_dir="${1:-}"
# shellcheck disable=SC2016 # matches a literal, unsubstituted ${...}
case "$token_dir" in
  "" | *'${'*)
    # Not given, or the tool didn't substitute ${devcontainerId}.
    if command -v sha256sum >/dev/null 2>&1; then
      hash="$(printf '%s' "$workspace" | sha256sum | cut -c1-16)"
    else
      hash="$(printf '%s' "$workspace" | shasum -a 256 | cut -c1-16)"
    fi
    token_dir="${XDG_RUNTIME_DIR:-$HOME/.cache}/gh-token/$hash"
    ;;
esac

[ -n "${GH_TOKEN_APP_ID:-}" ] || die "GH_TOKEN_APP_ID is not set in the host environment"
[ -n "${GH_TOKEN_KEY:-}" ] || die "GH_TOKEN_KEY (path to the App private key) is not set in the host environment"
[ -r "$GH_TOKEN_KEY" ] || die "cannot read private key $GH_TOKEN_KEY"

scope_file="${GH_TOKEN_SCOPE_FILE:-.devcontainer/devcontainer.json}"
[ -r "$scope_file" ] || die "cannot read scope file $scope_file"

mkdir -p "$token_dir"
chmod 700 "$token_dir"
token_dir="$(cd "$token_dir" && pwd -P)"

# The agent must not be able to commit or read the token dir via the workspace.
case "$token_dir/" in
  "$workspace"/*) die "token dir $token_dir is inside the workspace $workspace; choose a path outside it" ;;
esac

host="${GH_TOKEN_HOSTNAME:-github.com}"
host="${host#https://}"
host="${host%/}"
host="${host%/api/v3}"
# The gh shim and credential helper read this to know which host they serve.
printf '%s\n' "$host" >"$token_dir/host"

if [ -n "${GH_TOKEN_CMD:-}" ]; then
  read -r -a cmd <<<"$GH_TOKEN_CMD"
elif command -v gh-token >/dev/null 2>&1; then
  cmd=(gh-token)
elif command -v gh >/dev/null 2>&1 && gh token --help >/dev/null 2>&1; then
  cmd=(gh token)
else
  die "neither gh-token nor the gh token extension is installed (gh extension install Link-/gh-token)"
fi

args=(generate
  --app-id "$GH_TOKEN_APP_ID"
  --key "$GH_TOKEN_KEY"
  --scope-file "$scope_file"
  --git-dir "$workspace"
  --output-file "$token_dir/token"
  --refresh
  --pid-file "$token_dir/pid")
if [ "$host" != "github.com" ]; then
  args+=(--hostname "$host")
fi

log="$token_dir/refresh.log"
checksum() { cksum <"$1" 2>/dev/null || true; }
before="$(checksum "$token_dir/token")"

echo "gh-token host-init: starting refresher, token dir $token_dir (log: $log)" >&2
launcher=(nohup)
if command -v setsid >/dev/null 2>&1; then
  launcher=(setsid nohup)
fi
"${launcher[@]}" "${cmd[@]}" "${args[@]}" </dev/null >>"$log" 2>&1 &
pid=$!

timeout="${GH_TOKEN_TIMEOUT:-30}"
deadline=$((SECONDS + timeout))
while [ "$SECONDS" -lt "$deadline" ]; do
  if [ -s "$token_dir/token" ] && [ "$(checksum "$token_dir/token")" != "$before" ]; then
    echo "gh-token host-init: token ready" >&2
    exit 0
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    if wait "$pid"; then
      # Exit 0 without a new token: a refresher with the same scope is
      # already running and keeps the existing token fresh.
      [ -s "$token_dir/token" ] || die "refresher exited without a token; see $log"
      echo "gh-token host-init: refresher already running, reusing its token" >&2
      exit 0
    fi
    echo "gh-token host-init: refresher failed; last lines of $log:" >&2
    tail -n 20 "$log" >&2 || true
    echo "gh-token host-init: if the App is not installed on this repository (404), install it on the repository or its owner" >&2
    exit 1
  fi
  sleep 0.5
done

echo "gh-token host-init: no token after ${timeout}s; last lines of $log:" >&2
tail -n 20 "$log" >&2 || true
exit 1
