#!/usr/bin/env bash
# Checks run INSIDE a devcontainer set up with contrib/devcontainer, by the
# Scoped Token Integration workflow. The scope is expected to be
# contents: read and pull_requests: write on the clone's repository.
#
# Usage: in-container.sh <check> <owner/repo> [run-id] [pr-number]
#        in-container.sh no-key <owner/repo> <sha256 of the key's first base64 line>
set -euo pipefail

check="${1:?check}"
repo="${2:?owner/repo}"
run_id="${3:-local}"
pr="${4:-}"
token_dir="${GH_TOKEN_DIR:-/run/gh-token}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

case "$check" in
  no-key)
    # $3 is the SHA-256 of the first base64 line of the App's key, so the
    # key itself never enters the container. Other keys (test fixtures,
    # documentation) don't match.
    key_line_hash="${3:?sha256 of the first base64 line of the key}"
    found=0
    while IFS= read -r -d '' file; do
      while IFS= read -r line; do
        line="${line%$'\r'}"
        if [ "$(printf '%s' "$line" | sha256sum | cut -d' ' -f1)" = "$key_line_hash" ]; then
          echo "the App's private key is in $file" >&2
          found=1
        fi
      done <"$file"
    done < <(grep -rIlZ -e "PRIVATE KEY-----" "$token_dir" "$PWD" "$HOME" 2>/dev/null || true)
    [ "$found" = 0 ] || fail "the App's private key is in the container"
    echo "the App's private key is not in the container"
    ;;

  gh-and-git)
    gh_path="$(command -v gh)"
    echo "gh is $gh_path"
    grep -q "gh shim:" "$gh_path" || fail "gh is not the shim"
    # Informational only: it may try to look up a user, which installation tokens don't have.
    gh auth status || true
    [ "$(gh api "repos/$repo" --jq .full_name)" = "$repo" ] || fail "gh api repos/$repo"
    gh pr list --limit 1
    out="$(printf 'protocol=https\nhost=github.com\n\n' | git credential fill)"
    grep -q '^username=x-access-token$' <<<"$out" || fail "credential helper username"
    grep -qF "password=$(cat "$token_dir/token")" <<<"$out" || fail "credential helper password"
    git ls-remote origin HEAD
    echo "gh and git use the scoped token"
    ;;

  push-refused)
    branch="gh-token-it/$run_id"
    git checkout -q -B "$branch"
    if git push --dry-run origin "HEAD:refs/heads/$branch" >/tmp/push.out 2>&1; then
      fail "push was allowed with contents: read"
    fi
    cat /tmp/push.out
    grep -Eqi '403|denied|not allowed' /tmp/push.out || fail "push failed for another reason"
    echo "push refused"
    ;;

  issues-refused)
    label="gh-token-it-$run_id"
    if gh api -X POST "repos/$repo/labels" -f name="$label" -f color=ededed >/tmp/label.out 2>&1; then
      gh api -X DELETE "repos/$repo/labels/$label" || true
      fail "label creation was allowed without the issues permission"
    fi
    cat /tmp/label.out
    grep -q "HTTP 403" /tmp/label.out || fail "label creation failed for another reason"
    echo "issues write refused"
    ;;

  pr-write)
    [ -n "$pr" ] || fail "pr-write needs a pull request number"
    gh pr view "$pr" --repo "$repo" --json number --jq .number
    id="$(gh api -X POST "repos/$repo/issues/$pr/comments" \
      -f body="gh-token integration test from a devcontainer (run $run_id); deleted right away." --jq .id)"
    gh api -X DELETE "repos/$repo/issues/comments/$id"
    echo "created and deleted comment $id on pull request $pr"
    ;;

  *)
    fail "unknown check $check"
    ;;
esac
