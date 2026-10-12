#!/usr/bin/env bash
# gh-token devcontainer integration: runs INSIDE the container, from
# postCreateCommand. Installs the gh shim and the git credential helper and
# points gh at the clone's own repository.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
token_dir="${GH_TOKEN_DIR:-/run/gh-token}"
bin_dir="${GH_TOKEN_BIN_DIR:-/usr/local/bin}"
libexec_dir="${GH_TOKEN_LIBEXEC_DIR:-/usr/local/libexec/gh-token}"

sudo_cmd=()
if [ "$(id -u)" -ne 0 ] && [ ! -w "$bin_dir" ]; then
  command -v sudo >/dev/null 2>&1 || {
    echo "gh-token post-create: need root or sudo to install into $bin_dir" >&2
    exit 1
  }
  sudo_cmd=(sudo)
fi

if [ ! -s "$token_dir/token" ]; then
  # Optional mode on a host without gh-token set up, or a failed refresher:
  # leave gh and git as they are rather than install a shim that fails on
  # every call.
  echo "gh-token post-create: WARNING: no token at $token_dir/token; gh and git are left unchanged." >&2
  echo "gh-token post-create: set up the host (see gh token devcontainer-init) and rebuild the container to use a scoped token." >&2
  exit 0
fi

# Find the real gh, skipping a previously installed shim.
real_gh=""
for candidate in "$libexec_dir/gh" $(type -ap gh); do
  if [ -x "$candidate" ] && ! grep -q "gh shim:" "$candidate" 2>/dev/null; then
    real_gh="$candidate"
    break
  fi
done
if [ -z "$real_gh" ]; then
  echo "gh-token post-create: gh is not installed; add the github-cli feature (ghcr.io/devcontainers/features/github-cli)" >&2
  exit 1
fi
# A real gh at the shim's own path is moved out of the way.
if [ "$real_gh" = "$bin_dir/gh" ]; then
  "${sudo_cmd[@]}" mkdir -p "$libexec_dir"
  "${sudo_cmd[@]}" mv "$bin_dir/gh" "$libexec_dir/gh"
  real_gh="$libexec_dir/gh"
fi

tmp="$(mktemp)"
sed "s|@REAL_GH@|$real_gh|" "$here/gh-shim.sh" >"$tmp"
"${sudo_cmd[@]}" install -m 0755 "$tmp" "$bin_dir/gh"
rm -f "$tmp"
"${sudo_cmd[@]}" install -m 0755 "$here/git-credential-gh-token.sh" "$bin_dir/git-credential-gh-token"
hash -r

host="github.com"
if [ -r "$token_dir/host" ]; then
  host="$(head -n 1 "$token_dir/host")"
fi

# The empty value clears helpers configured earlier (for example VS Code's
# forwarding of the host's git credentials), then ours is the only one.
git config --global --unset-all "credential.https://$host.helper" 2>/dev/null || true
git config --global --add "credential.https://$host.helper" ""
git config --global --add "credential.https://$host.helper" "$bin_dir/git-credential-gh-token"
# Send SSH remotes for this host through https, so pushes use the scoped
# token rather than a forwarded SSH agent.
git config --global --unset-all "url.https://$host/.insteadOf" 2>/dev/null || true
git config --global --add "url.https://$host/.insteadOf" "git@$host:"
git config --global --add "url.https://$host/.insteadOf" "ssh://git@$host/"

# gh may otherwise resolve to an "upstream" remote the token cannot reach.
if origin="$(git remote get-url origin 2>/dev/null)"; then
  repo="$(printf '%s\n' "$origin" | sed -E 's#^(https?://([^@/]+@)?|ssh://([^@/]+@)?|[^@/]+@)?[^/:]+(:[0-9]+)?[:/]##; s#\.git$##; s#/$##')"
  if [ -n "$repo" ] && "$bin_dir/gh" repo set-default "$host/$repo" >/dev/null 2>&1; then
    echo "gh-token post-create: gh default repository set to $repo" >&2
  else
    echo "gh-token post-create: warning: could not run gh repo set-default $repo" >&2
  fi
fi

echo "gh-token post-create: gh and git now use the token from $token_dir" >&2
