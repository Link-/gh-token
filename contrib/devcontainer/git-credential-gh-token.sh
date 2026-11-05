#!/usr/bin/env bash
# gh-token devcontainer integration: git credential helper, installed as
# /usr/local/bin/git-credential-gh-token inside the container.
#
#   git config --global credential.https://github.com.helper /usr/local/bin/git-credential-gh-token
#
# Answers "get" for the token's host over https with the current token from
# the mounted file. Without a token it answers quit=1, so git doesn't fall
# back to other helpers for that host. Other hosts and protocols are left to
# other helpers. "store" and "erase" are ignored: the token is managed on
# the host.
set -euo pipefail

token_dir="${GH_TOKEN_DIR:-/run/gh-token}"
action="${1:-}"

protocol=""
host=""
while IFS= read -r line && [ -n "$line" ]; do
  case "$line" in
    protocol=*) protocol="${line#protocol=}" ;;
    host=*) host="${line#host=}" ;;
  esac
done

[ "$action" = "get" ] || exit 0

expected_host="github.com"
if [ -r "$token_dir/host" ]; then
  expected_host="$(head -n 1 "$token_dir/host")"
fi

if [ "$protocol" != "https" ] || [ "$host" != "$expected_host" ]; then
  exit 0
fi

if [ ! -s "$token_dir/token" ]; then
  echo "git-credential-gh-token: no token at $token_dir/token; is the host refresher running?" >&2
  # Stop here: no fallback to later helpers (such as VS Code's forwarding
  # of the host's credentials), which can do far more than the token.
  echo "quit=1"
  exit 0
fi

printf 'username=x-access-token\npassword=%s\n' "$(cat "$token_dir/token")"
