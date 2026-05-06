#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --dry-run   ビルド後に dry-run で動作確認する
  --run       ビルド後に本番実行する（systemd 経由ではなく直接）
  -h, --help  このヘルプを表示
EOF
}

DRY_RUN=false
RUN=false

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    --run)     RUN=true ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $arg"; usage; exit 1 ;;
  esac
done

echo "==> git pull"
git pull origin develop

echo "==> docker compose build"
docker compose build

if $DRY_RUN; then
  echo "==> dry-run"
  docker compose run --rm favofeeder -dry-run -v
fi

if $RUN; then
  echo "==> run"
  docker compose run --rm favofeeder
fi

echo "==> done"
