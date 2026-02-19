#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: build-site.sh [options]

Build Hugo site to static files.

Options:
  --site-dir PATH        Site directory (default: blog)
  --destination PATH     Output directory (default: public)
  --base-url URL         Override base URL
  --drafts BOOL          true|false (default: false)
  -h, --help             Show help
USAGE
}

site_dir="blog"
destination="public"
base_url=""
drafts="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --site-dir) site_dir="${2:-}"; shift 2 ;;
    --destination) destination="${2:-}"; shift 2 ;;
    --base-url) base_url="${2:-}"; shift 2 ;;
    --drafts) drafts="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if ! command -v hugo >/dev/null 2>&1; then
  echo "hugo is required." >&2
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

full_destination="$destination"
if [[ "$destination" != /* ]]; then
  full_destination="$site_dir/$destination"
fi

cmd=(
  hugo
  --source "$site_dir"
  --destination "$full_destination"
  --minify
  --gc
  --cleanDestinationDir
)

if [[ -n "$base_url" ]]; then
  cmd+=(--baseURL "$base_url")
fi
if [[ "$drafts" == "true" ]]; then
  cmd+=(--buildDrafts --buildFuture)
fi

"${cmd[@]}"

echo "Build completed: $full_destination"
