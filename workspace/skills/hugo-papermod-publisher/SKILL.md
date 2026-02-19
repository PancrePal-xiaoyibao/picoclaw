---
name: hugo-papermod-publisher
description: Deploy and operate Hugo sites with PaperMod template, including bootstrap, post creation, build, and git-based publishing.
metadata: {"nanobot":{"emoji":"📰","requires":{"bins":["git","hugo"]},"install":[{"id":"brew-hugo","kind":"brew","formula":"hugo","bins":["hugo"],"label":"Install Hugo (brew)"},{"id":"apt-hugo","kind":"apt","package":"hugo","bins":["hugo"],"label":"Install Hugo (apt)"}]}}
---

# Hugo PaperMod Publisher

Use this skill when the user asks to:
- deploy or initialize a Hugo blog
- use PaperMod as the theme/template
- create and publish articles
- automate build and publishing workflows

This skill standardizes the workflow with reusable scripts.

## Script Toolkit

All scripts are in `{baseDir}/scripts/`:

- `check-prereqs.sh`: Verify required tools (`git`, `hugo`) and report versions.
- `install-hugo.sh`: Install Hugo via package manager (`brew/apt/dnf/yum/pacman/snap`).
- `bootstrap.sh`: One-shot setup (`install-hugo` + `init-site`).
- `init-site.sh`: Initialize Hugo site and configure PaperMod.
- `new-post.sh`: Generate a new post markdown file with standardized front matter.
- `dev-server.sh`: Start local preview server.
- `build-site.sh`: Build static site into `public/`.
- `publish-git.sh`: Build and publish `public/` to a git branch (default `gh-pages`).

## Default Workflow

1. Bootstrap site:
```bash
{baseDir}/scripts/bootstrap.sh \
  --site-dir blog \
  --title "Oncology Community" \
  --base-url "https://example.org/"
```

2. Create a post:
```bash
{baseDir}/scripts/new-post.sh \
  --site-dir blog \
  --title "ESMO 2026 highlights in NSCLC"
```

3. Preview:
```bash
{baseDir}/scripts/dev-server.sh --site-dir blog --port 1313
```

4. Build:
```bash
{baseDir}/scripts/build-site.sh --site-dir blog
```

5. Publish to git branch:
```bash
{baseDir}/scripts/publish-git.sh \
  --site-dir blog \
  --branch gh-pages \
  --remote origin
```

## Publish Safety Rules

- Always confirm with the user before publishing to production remotes.
- `publish-git.sh` uses force push to the target branch by design.
- If remote is not configured in site repo, provide `--remote-url`.

## Advanced References

- Deployment options and branching guidance: `references/deploy-flows.md`
