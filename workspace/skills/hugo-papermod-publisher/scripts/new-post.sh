#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: new-post.sh --title "Post title" [options]

Create a new post markdown file with standardized front matter.

Options:
  --site-dir PATH     Site directory (default: blog)
  --title TEXT        Post title (required)
  --slug TEXT         Optional slug
  --draft BOOL        true|false (default: true)
  --category TEXT     Default category (default: oncology)
  -h, --help          Show help
USAGE
}

site_dir="blog"
title=""
slug=""
draft="true"
category="oncology"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --site-dir) site_dir="${2:-}"; shift 2 ;;
    --title) title="${2:-}"; shift 2 ;;
    --slug) slug="${2:-}"; shift 2 ;;
    --draft) draft="${2:-}"; shift 2 ;;
    --category) category="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ -z "$title" ]]; then
  echo "--title is required." >&2
  usage
  exit 1
fi

if [[ "$draft" != "true" && "$draft" != "false" ]]; then
  echo "--draft must be true or false." >&2
  exit 1
fi

if [[ ! -d "$site_dir" ]]; then
  echo "Site directory does not exist: $site_dir" >&2
  exit 1
fi

slugify() {
  local text="$1"
  text="$(echo "$text" | tr '[:upper:]' '[:lower:]')"
  text="$(echo "$text" | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//')"
  echo "$text"
}

if [[ -z "$slug" ]]; then
  slug="$(slugify "$title")"
fi
if [[ -z "$slug" ]]; then
  slug="post-$(date +%Y%m%d-%H%M%S)"
fi

date_prefix="$(date +%Y-%m-%d)"
ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
post_path="$site_dir/content/posts/${date_prefix}-${slug}.md"

if [[ -e "$post_path" ]]; then
  echo "Post already exists: $post_path" >&2
  exit 1
fi

mkdir -p "$(dirname "$post_path")"

cat > "$post_path" <<EOF
---
title: "$title"
date: $ts
draft: $draft
tags: []
categories: ["$category"]
summary: ""
description: ""
---

Write your content here.
EOF

echo "Post created: $post_path"
