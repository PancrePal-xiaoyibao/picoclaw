#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: install-hugo.sh [--manager auto|brew|apt|dnf|yum|pacman|snap]

Install Hugo using an available package manager.
USAGE
}

manager="auto"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manager)
      manager="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if command -v hugo >/dev/null 2>&1; then
  echo "hugo already installed: $(hugo version | head -n 1)"
  exit 0
fi

run_root_cmd() {
  if [[ "$(id -u)" -eq 0 ]]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    echo "Need root privileges to run: $*" >&2
    exit 1
  fi
}

install_by_manager() {
  local m="$1"
  case "$m" in
    brew)
      command -v brew >/dev/null 2>&1 || return 1
      brew install hugo
      ;;
    apt)
      command -v apt-get >/dev/null 2>&1 || return 1
      run_root_cmd apt-get update
      run_root_cmd apt-get install -y hugo
      ;;
    dnf)
      command -v dnf >/dev/null 2>&1 || return 1
      run_root_cmd dnf install -y hugo
      ;;
    yum)
      command -v yum >/dev/null 2>&1 || return 1
      run_root_cmd yum install -y hugo
      ;;
    pacman)
      command -v pacman >/dev/null 2>&1 || return 1
      run_root_cmd pacman -Sy --noconfirm hugo
      ;;
    snap)
      command -v snap >/dev/null 2>&1 || return 1
      run_root_cmd snap install hugo
      ;;
    *)
      echo "Unsupported manager: $m" >&2
      return 1
      ;;
  esac
}

if [[ "$manager" == "auto" ]]; then
  for m in brew apt dnf yum pacman snap; do
    if install_by_manager "$m"; then
      manager="$m"
      break
    fi
  done
else
  install_by_manager "$manager"
fi

if ! command -v hugo >/dev/null 2>&1; then
  echo "Failed to install hugo. Install manually from https://gohugo.io/installation/" >&2
  exit 1
fi

echo "hugo installed via $manager: $(hugo version | head -n 1)"
if ! hugo version | grep -qi "extended"; then
  echo "[warn] non-extended build detected; PaperMod usually still works."
fi
