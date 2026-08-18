#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SOURCE_PATH="$PROJECT_DIR/assets/mory-icon.png"
ICNS_PATH="$PROJECT_DIR/.build/icons/icon.icns"
ICON_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/mory-icon.XXXXXX")"
ICONSET_PATH="$ICON_ROOT/Mory.iconset"

cleanup() {
  rm -R "$ICON_ROOT"
}
trap cleanup EXIT

if [[ ! -f "$SOURCE_PATH" ]]; then
  echo "Mory icon source not found: $SOURCE_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$ICNS_PATH")" "$ICONSET_PATH"

render_icon() {
  local size="$1"
  local name="$2"
  sips -z "$size" "$size" "$SOURCE_PATH" --out "$ICONSET_PATH/$name" >/dev/null
}

# Include standard and Retina representations for crisp Finder and Dock rendering.
render_icon 16 icon_16x16.png
render_icon 32 icon_16x16@2x.png
render_icon 32 icon_32x32.png
render_icon 64 icon_32x32@2x.png
render_icon 128 icon_128x128.png
render_icon 256 icon_128x128@2x.png
render_icon 256 icon_256x256.png
render_icon 512 icon_256x256@2x.png
render_icon 512 icon_512x512.png
render_icon 1024 icon_512x512@2x.png

node "$PROJECT_DIR/scripts/build-icns.mjs" "$ICONSET_PATH" "$ICNS_PATH"
echo "Mory macOS icon generated: $ICNS_PATH"
