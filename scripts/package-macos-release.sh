#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_PATH="$PROJECT_DIR/dist/macos/Mory.app"
RELEASE_DIR="$PROJECT_DIR/dist/releases"
VERSION="$(node -p "require('$PROJECT_DIR/package.json').version")"
DMG_BACKGROUND="$PROJECT_DIR/assets/dmg-background.png"
DMG_LAYOUT_SCRIPT="$PROJECT_DIR/scripts/configure-macos-dmg.applescript"
VOLUME_NAME="Mory"

case "$(uname -m)" in
  arm64)
    RELEASE_ARCH="arm64"
    ;;
  x86_64)
    RELEASE_ARCH="x64"
    ;;
  *)
    echo "Unsupported macOS architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

if [[ ! -d "$APP_PATH" ]]; then
  echo "App bundle not found: $APP_PATH. Run npm run build:mac first." >&2
  exit 1
fi

ARTIFACT_BASE="Mory-${VERSION}-macos-${RELEASE_ARCH}"
DMG_PATH="$RELEASE_DIR/${ARTIFACT_BASE}.dmg"
ZIP_PATH="$RELEASE_DIR/${ARTIFACT_BASE}.zip"
CHECKSUM_PATH="$RELEASE_DIR/${ARTIFACT_BASE}-SHA256SUMS.txt"
CLI_PATH="$RELEASE_DIR/${ARTIFACT_BASE}-cli"
STAGING_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/mory-release.XXXXXX")"
STAGING_DIR="$STAGING_ROOT/Mory"
MOUNT_DIR="$STAGING_ROOT/$(basename "$STAGING_ROOT")"
MOUNT_NAME="$(basename "$MOUNT_DIR")"
RW_DMG_PATH="$STAGING_ROOT/${ARTIFACT_BASE}-rw.dmg"
ATTACHED_DEVICE=""

cleanup() {
  if [[ -n "$ATTACHED_DEVICE" ]]; then
    hdiutil detach "$ATTACHED_DEVICE" -quiet || true
  fi
  rm -R "$STAGING_ROOT"
}
trap cleanup EXIT

if [[ ! -f "$DMG_BACKGROUND" ]]; then
  echo "DMG background not found: $DMG_BACKGROUND" >&2
  exit 1
fi

if [[ ! -f "$DMG_LAYOUT_SCRIPT" ]]; then
  echo "DMG layout script not found: $DMG_LAYOUT_SCRIPT" >&2
  exit 1
fi

APP_SIZE_KB="$(du -sk "$APP_PATH" | awk '{print $1}')"
DMG_SIZE_MB="$((APP_SIZE_KB / 1024 + 32))"

mkdir -p "$RELEASE_DIR" "$STAGING_DIR/.background" "$MOUNT_DIR"
ditto "$APP_PATH" "$STAGING_DIR/Mory.app"
cp "$DMG_BACKGROUND" "$STAGING_DIR/.background/dmg-background.png"
ln -s /Applications "$STAGING_DIR/Applications"

hdiutil create \
  -srcfolder "$STAGING_DIR" \
  -size "${DMG_SIZE_MB}m" \
  -fs HFS+ \
  -volname "$VOLUME_NAME" \
  -ov \
  -format UDRW \
  "$RW_DMG_PATH"

ATTACHED_DEVICE="$(
  hdiutil attach \
    -readwrite \
    -noverify \
    -noautoopen \
    -mountpoint "$MOUNT_DIR" \
    "$RW_DMG_PATH" |
    awk '/^\/dev\// { print $1; exit }'
)"

if [[ -z "$ATTACHED_DEVICE" ]]; then
  echo "Unable to attach writable DMG: $RW_DMG_PATH" >&2
  exit 1
fi

osascript "$DMG_LAYOUT_SCRIPT" "$MOUNT_NAME" "$MOUNT_DIR"
sync

for _ in {1..10}; do
  if [[ -f "$MOUNT_DIR/.DS_Store" ]]; then
    break
  fi
  sleep 1
done

if [[ ! -f "$MOUNT_DIR/.DS_Store" ]]; then
  echo "Finder did not persist the DMG layout metadata." >&2
  exit 1
fi

if [[ -d "$MOUNT_DIR/.fseventsd" ]]; then
  rm -R "$MOUNT_DIR/.fseventsd"
fi

hdiutil detach "$ATTACHED_DEVICE" -quiet
ATTACHED_DEVICE=""

hdiutil convert \
  "$RW_DMG_PATH" \
  -ov \
  -format UDZO \
  -imagekey zlib-level=9 \
  -o "$DMG_PATH"

ditto -c -k --sequesterRsrc --keepParent \
  "$APP_PATH" \
  "$STAGING_ROOT/${ARTIFACT_BASE}.zip"
mv -f "$STAGING_ROOT/${ARTIFACT_BASE}.zip" "$ZIP_PATH"
cp "$APP_PATH/Contents/Resources/bin/mory" "$CLI_PATH"
chmod 755 "$CLI_PATH"

(
  cd "$RELEASE_DIR"
  shasum -a 256 "$(basename "$DMG_PATH")" "$(basename "$ZIP_PATH")" "$(basename "$CLI_PATH")"
) > "$STAGING_ROOT/$(basename "$CHECKSUM_PATH")"
mv -f "$STAGING_ROOT/$(basename "$CHECKSUM_PATH")" "$CHECKSUM_PATH"

echo "Mory macOS release artifacts generated:"
ls -lh "$DMG_PATH" "$ZIP_PATH" "$CLI_PATH" "$CHECKSUM_PATH"
