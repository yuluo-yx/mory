#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_BINARY="$PROJECT_DIR/dist/macos/Mory.app/Contents/MacOS/Mory"

if [[ ! -x "$APP_BINARY" ]]; then
  echo "请先执行 npm run build:mac"
  exit 1
fi

env MORY_DRAG_SMOKE=1 "$APP_BINARY"
