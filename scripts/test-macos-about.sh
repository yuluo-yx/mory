#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_BINARY="$PROJECT_DIR/dist/macos/Mory.app/Contents/MacOS/Mory"

if [[ ! -x "$APP_BINARY" ]]; then
  echo "Run npm run build:mac before starting the About panel smoke test."
  exit 1
fi

node "$PROJECT_DIR/scripts/run-native-smoke.mjs" 15 env MORY_ABOUT_SMOKE=1 "$APP_BINARY"
