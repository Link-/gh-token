#!/usr/bin/env bash
# gh-token devcontainer integration: runs INSIDE the container on every
# attach, from postAttachCommand.
#
# VS Code forwards the host's SSH agent into the container through a socket
# in /tmp. Any key loaded on the host is usable through it, with access far
# beyond the token's scope, so remove the sockets owned by this user.
# devcontainer.json also sets SSH_AUTH_SOCK to empty in remoteEnv.
set -euo pipefail

dir="${GH_TOKEN_SSH_SOCKET_DIR:-/tmp}"
find "$dir" -maxdepth 1 -name 'vscode-ssh-auth-*.sock' -user "$(id -u)" -print -delete 2>/dev/null |
  sed 's/^/gh-token post-attach: removed forwarded SSH agent socket /' >&2 || true
