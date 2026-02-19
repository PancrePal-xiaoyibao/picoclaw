#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: dev-server.sh [options]

Run local Hugo development server.

Options:
  --site-dir PATH     Site directory (default: blog)
  --port PORT         Local port (default: 1313)
  --drafts BOOL       true|false (default: true)
  -h, --help          Show help
USAGE
}

site_dir="blog"
port="1313"
drafts="true"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --site-dir) site_dir="${2:-}"; shift 2 ;;
    --port) port="${2:-}"; shift 2 ;;
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

cmd=(
  hugo
  server
  --source "$site_dir"
  --bind "0.0.0.0"
  --port "$port"
  --disableFastRender
)

if [[ "$drafts" == "true" ]]; then
  cmd+=(--buildDrafts --buildFuture)
fi

echo "Starting Hugo dev server at http://localhost:$port"
exec "${cmd[@]}"
