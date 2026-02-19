#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: init-site.sh [options]

Initialize a Hugo site and configure PaperMod theme.

Options:
  --site-dir PATH        Site directory (default: blog)
  --title TEXT           Site title (default: Oncology Community)
  --base-url URL         Base URL (default: https://example.org/)
  --language-code CODE   Language code (default: zh-cn)
  --theme-repo URL       PaperMod repo (default: https://github.com/adityatelange/hugo-PaperMod.git)
  --force                Overwrite generated config/archetype files
  -h, --help             Show help
USAGE
}

site_dir="blog"
title="Oncology Community"
base_url="https://example.org/"
language_code="zh-cn"
theme_repo="https://github.com/adityatelange/hugo-PaperMod.git"
force=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --site-dir) site_dir="${2:-}"; shift 2 ;;
    --title) title="${2:-}"; shift 2 ;;
    --base-url) base_url="${2:-}"; shift 2 ;;
    --language-code) language_code="${2:-}"; shift 2 ;;
    --theme-repo) theme_repo="${2:-}"; shift 2 ;;
    --force) force=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if ! command -v hugo >/dev/null 2>&1; then
  echo "hugo is required. Run install-hugo.sh first." >&2
  exit 1
fi
if ! command -v git >/dev/null 2>&1; then
  echo "git is required." >&2
  exit 1
fi

if [[ ! -d "$site_dir" ]]; then
  echo "Creating Hugo site in: $site_dir"
  hugo new site "$site_dir"
fi

cd "$site_dir"

if [[ ! -d ".git" ]]; then
  git init >/dev/null 2>&1
fi

mkdir -p themes
if [[ ! -d "themes/PaperMod" ]]; then
  echo "Installing PaperMod theme..."
  if git submodule add --depth 1 "$theme_repo" themes/PaperMod >/dev/null 2>&1; then
    echo "PaperMod installed as git submodule."
  else
    rm -rf themes/PaperMod
    git clone --depth 1 "$theme_repo" themes/PaperMod >/dev/null 2>&1
    echo "PaperMod cloned (submodule fallback)."
  fi
fi

write_hugo_yaml() {
  cat > hugo.yaml <<EOF
baseURL: "$base_url"
languageCode: "$language_code"
title: "$title"
theme: "PaperMod"
enableRobotsTXT: true
pagination:
  pagerSize: 10
params:
  defaultTheme: auto
  ShowReadingTime: true
  ShowPostNavLinks: true
  ShowCodeCopyButtons: true
  ShowShareButtons: true
outputs:
  home:
    - HTML
    - RSS
    - JSON
menu:
  main:
    - identifier: archives
      name: Archives
      url: /archives/
      weight: 10
    - identifier: search
      name: Search
      url: /search/
      weight: 20
EOF
}

if [[ "$force" == true ]]; then
  write_hugo_yaml
elif [[ ! -f "hugo.yaml" && ! -f "config.yaml" && ! -f "config.toml" && ! -f "config.json" ]]; then
  write_hugo_yaml
else
  if [[ -f "hugo.yaml" ]] && ! grep -qE '^\s*theme\s*:' hugo.yaml; then
    printf '\ntheme: "PaperMod"\n' >> hugo.yaml
  elif [[ -f "config.toml" ]] && ! grep -qE '^\s*theme\s*=' config.toml; then
    printf '\ntheme = "PaperMod"\n' >> config.toml
  fi
fi

mkdir -p archetypes
if [[ "$force" == true || ! -f "archetypes/default.md" ]]; then
  cat > archetypes/default.md <<'EOF'
---
title: "{{ replace .Name "-" " " | title }}"
date: {{ .Date }}
draft: true
tags: []
categories: ["oncology"]
summary: ""
description: ""
---
EOF
fi

mkdir -p content/posts
if [[ ! -f content/archives.md ]]; then
  cat > content/archives.md <<'EOF'
---
title: "Archives"
layout: "archives"
url: "/archives/"
summary: archives
---
EOF
fi

if [[ ! -f content/search.md ]]; then
  cat > content/search.md <<'EOF'
---
title: "Search"
layout: "search"
url: "/search/"
summary: search
placeholder: "Search..."
---
EOF
fi

touch .gitignore
grep -q "^/public/$" .gitignore || echo "/public/" >> .gitignore
grep -q "^/resources/_gen/$" .gitignore || echo "/resources/_gen/" >> .gitignore
grep -q "^/.hugo_build.lock$" .gitignore || echo "/.hugo_build.lock" >> .gitignore

echo "Site initialized at: $site_dir"
echo "Preview with: hugo server --source \"$site_dir\""
