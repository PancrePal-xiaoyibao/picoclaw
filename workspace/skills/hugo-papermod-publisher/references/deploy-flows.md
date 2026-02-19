# Deploy Flows

## 1) Git branch publish (default)

Use `publish-git.sh` to push built static files to a branch such as `gh-pages`.

Common case:
- main source in `main`
- static output pushed to `gh-pages`
- platform (GitHub Pages, Gitee Pages, etc.) serves from `gh-pages`

Example:
```bash
scripts/publish-git.sh --site-dir blog --branch gh-pages --remote origin
```

If site repo has no remote configured:
```bash
scripts/publish-git.sh \
  --site-dir blog \
  --branch gh-pages \
  --remote-url git@github.com:owner/repo.git
```

## 2) CI/CD publish (recommended for teams)

Local flow:
- edit content
- commit source only

CI flow:
- run `hugo --minify`
- deploy `public/` as static artifact

Benefits:
- no force push from local machine
- better audit trail and reproducibility

## 3) Base URL rules

- For production publishing, set stable URL:
  - `https://owner.github.io/repo/` (project pages)
  - `https://owner.github.io/` (user/org pages)
- For local preview, use default (`http://localhost:1313/`) and avoid hard-coding prod URL.

You can override build-time URL:
```bash
scripts/build-site.sh --site-dir blog --base-url "https://owner.github.io/repo/"
```

## 4) PaperMod notes

- `content/search.md` and `content/archives.md` are created by `init-site.sh`.
- Keep `theme: PaperMod` in Hugo config.
- Prefer `hugo.yaml` for readability and easier automation.
