#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  cat <<'USAGE'
Usage: publish-git.sh [options]

Build Hugo site (optional) and force-push static output to a git branch.

Options:
  --site-dir PATH         Site directory (default: blog)
  --branch NAME           Target branch (default: gh-pages)
  --remote NAME           Remote name (default: origin)
  --remote-url URL        Remote URL (optional; auto-resolve from site repo if omitted)
  --base-url URL          Build-time base URL override
  --drafts BOOL           true|false for build step (default: false)
  --skip-build            Skip build step and publish existing public/
  --commit-message TEXT   Commit message (default: publish: <utc-time>)
  --dry-run               Print push target without pushing
  -h, --help              Show help
USAGE
}

site_dir="blog"
branch="gh-pages"
remote="origin"
remote_url=""
base_url=""
drafts="false"
skip_build=false
commit_message=""
dry_run=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --site-dir) site_dir="${2:-}"; shift 2 ;;
    --branch) branch="${2:-}"; shift 2 ;;
    --remote) remote="${2:-}"; shift 2 ;;
    --remote-url) remote_url="${2:-}"; shift 2 ;;
    --base-url) base_url="${2:-}"; shift 2 ;;
    --drafts) drafts="${2:-}"; shift 2 ;;
    --skip-build) skip_build=true; shift ;;
    --commit-message) commit_message="${2:-}"; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if ! command -v git >/dev/null 2>&1; then
  echo "git is required." >&2
  exit 1
fi
if [[ ! -d "$site_dir" ]]; then
  echo "Site directory does not exist: $site_dir" >&2
  exit 1
fi
if [[ "$drafts" != "true" && "$drafts" != "false" ]]; then
  echo "--drafts must be true or false." >&2
  exit 1
fi

if [[ "$skip_build" != true ]]; then
  build_cmd=(
    "$SCRIPT_DIR/build-site.sh"
    --site-dir "$site_dir"
    --destination public
    --drafts "$drafts"
  )
  if [[ -n "$base_url" ]]; then
    build_cmd+=(--base-url "$base_url")
  fi
  "${build_cmd[@]}"
fi

public_dir="$site_dir/public"
if [[ ! -d "$public_dir" ]]; then
  echo "Build output not found: $public_dir" >&2
  exit 1
fi

if [[ -z "$remote_url" ]] && git -C "$site_dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  remote_url="$(git -C "$site_dir" remote get-url "$remote" 2>/dev/null || true)"
fi
if [[ -z "$remote_url" ]]; then
  echo "Remote URL not found. Set --remote-url or configure remote '$remote' in $site_dir." >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

cp -a "$public_dir"/. "$tmp_dir"/

pushd "$tmp_dir" >/dev/null
git init -q
git checkout -q -b "$branch"
git add -A

if git diff --cached --quiet; then
  echo "No files to publish in $public_dir"
  popd >/dev/null
  exit 0
fi

git config user.name "${PUBLISH_GIT_NAME:-picoclaw-bot}"
git config user.email "${PUBLISH_GIT_EMAIL:-picoclaw-bot@example.com}"

if [[ -z "$commit_message" ]]; then
  commit_message="publish: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi
git commit -q -m "$commit_message"

git remote add "$remote" "$remote_url"

if [[ "$dry_run" == true ]]; then
  echo "[dry-run] ready to push: $remote_url (branch: $branch)"
else
  git push -f "$remote" "HEAD:$branch"
  echo "Published to $remote_url (branch: $branch)"
fi
popd >/dev/null
