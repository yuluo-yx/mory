#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_BINARY="$PROJECT_DIR/dist/macos/Mory.app/Contents/MacOS/Mory"

if [[ ! -x "$APP_BINARY" ]]; then
  echo "Run npm run build:mac before starting the drag smoke test."
  exit 1
fi

env MORY_DRAG_SMOKE=1 "$APP_BINARY"
