#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: check-prereqs.sh

Validate required binaries for Hugo + PaperMod workflow.
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

missing=0

check_bin() {
  local bin="$1"
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "[missing] $bin"
    missing=1
  else
    echo "[ok] $bin: $("$bin" --version 2>/dev/null | head -n 1 || true)"
  fi
}

check_bin git
check_bin hugo

if command -v hugo >/dev/null 2>&1; then
  if hugo version 2>/dev/null | grep -qi "extended"; then
    echo "[ok] hugo extended build detected"
  else
    echo "[warn] hugo is not extended build (usually still works for PaperMod)"
  fi
fi

if [[ $missing -ne 0 ]]; then
  echo "One or more dependencies are missing."
  echo "Run install-hugo.sh for Hugo, and install git if needed."
  exit 1
fi

echo "All required dependencies are available."
