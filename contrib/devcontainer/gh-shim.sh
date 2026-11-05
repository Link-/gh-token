#!/usr/bin/env bash
# gh-token devcontainer integration: installed as /usr/local/bin/gh inside
# the container, ahead of the real gh. Reads the token on every call, so
# host-side refreshes take effect without restarting anything.
set -euo pipefail

token_dir="${GH_TOKEN_DIR:-/run/gh-token}"
real_gh="${GH_TOKEN_REAL_GH:-@REAL_GH@}"

if [ ! -x "$real_gh" ]; then
  echo "gh shim: real gh not found at $real_gh (set GH_TOKEN_REAL_GH)" >&2
  exit 127
fi

if [ ! -s "$token_dir/token" ]; then
  echo "gh shim: no token at $token_dir/token. Is the host refresher running?" >&2
  echo "gh shim: check refresh.log in the token directory on the host and rebuild or restart the container." >&2
  exit 1
fi
token="$(cat "$token_dir/token")"

# Hint when the token has expired (the host refresher may have stopped).
if [ -r "$token_dir/token.json" ]; then
  expires="$(sed -n 's/.*"expires_at": *"\([^"]*\)".*/\1/p' "$token_dir/token.json" | head -n 1)"
  if [ -n "$expires" ]; then
    expires_epoch="$(date -d "$expires" +%s 2>/dev/null || true)"
    if [ -n "$expires_epoch" ] && [ "$expires_epoch" -le "$(date +%s)" ]; then
      echo "gh shim: warning: the token expired at $expires; the host refresher may have stopped (see refresh.log on the host)" >&2
    fi
  fi
fi

host="github.com"
if [ -r "$token_dir/host" ]; then
  host="$(head -n 1 "$token_dir/host")"
fi

# Only the scoped token: drop any other token gh would pick up.
unset GH_TOKEN GITHUB_TOKEN GH_ENTERPRISE_TOKEN GITHUB_ENTERPRISE_TOKEN

if [ "$host" = "github.com" ] || [[ "$host" == *.ghe.com ]]; then
  GH_TOKEN="$token" exec "$real_gh" "$@"
fi
# gh reads GH_ENTERPRISE_TOKEN for GitHub Enterprise Server hosts.
GH_HOST="${GH_HOST:-$host}" GH_ENTERPRISE_TOKEN="$token" exec "$real_gh" "$@"
