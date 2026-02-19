#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  cat <<'USAGE'
Usage: bootstrap.sh [options]

One-shot setup for Hugo + PaperMod:
1) Install Hugo (optional)
2) Initialize site and PaperMod

Options:
  --site-dir PATH        Site directory (default: blog)
  --title TEXT           Site title (default: Oncology Community)
  --base-url URL         Base URL (default: https://example.org/)
  --language-code CODE   Language code (default: zh-cn)
  --skip-install         Skip Hugo installation step
  --force                Overwrite generated config/archetype files
  -h, --help             Show help
USAGE
}

site_dir="blog"
title="Oncology Community"
base_url="https://example.org/"
language_code="zh-cn"
skip_install=false
force=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --site-dir) site_dir="${2:-}"; shift 2 ;;
    --title) title="${2:-}"; shift 2 ;;
    --base-url) base_url="${2:-}"; shift 2 ;;
    --language-code) language_code="${2:-}"; shift 2 ;;
    --skip-install) skip_install=true; shift ;;
    --force) force=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ "$skip_install" != true ]]; then
  "$SCRIPT_DIR/install-hugo.sh"
fi

"$SCRIPT_DIR/init-site.sh" \
  --site-dir "$site_dir" \
  --title "$title" \
  --base-url "$base_url" \
  --language-code "$language_code" \
  $([[ "$force" == true ]] && echo "--force")

echo "Bootstrap done."
echo "Next:"
echo "  $SCRIPT_DIR/new-post.sh --site-dir \"$site_dir\" --title \"Your post title\""
echo "  $SCRIPT_DIR/dev-server.sh --site-dir \"$site_dir\""
