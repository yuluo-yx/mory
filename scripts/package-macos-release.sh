#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_PATH="$PROJECT_DIR/dist/macos/Mory.app"
RELEASE_DIR="$PROJECT_DIR/dist/releases"
VERSION="$(node -p "require('$PROJECT_DIR/package.json').version")"

case "$(uname -m)" in
  arm64)
    RELEASE_ARCH="arm64"
    ;;
  x86_64)
    RELEASE_ARCH="x64"
    ;;
  *)
    echo "不支持的 macOS 架构：$(uname -m)" >&2
    exit 1
    ;;
esac

if [[ ! -d "$APP_PATH" ]]; then
  echo "未找到应用包：$APP_PATH。请先执行 npm run build:mac。" >&2
  exit 1
fi

ARTIFACT_BASE="Mory-${VERSION}-macos-${RELEASE_ARCH}"
DMG_PATH="$RELEASE_DIR/${ARTIFACT_BASE}.dmg"
ZIP_PATH="$RELEASE_DIR/${ARTIFACT_BASE}.zip"
CHECKSUM_PATH="$RELEASE_DIR/${ARTIFACT_BASE}-SHA256SUMS.txt"
STAGING_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/mory-release.XXXXXX")"
STAGING_DIR="$STAGING_ROOT/Mory"

cleanup() {
  rm -R "$STAGING_ROOT"
}
trap cleanup EXIT

mkdir -p "$RELEASE_DIR" "$STAGING_DIR"
ditto "$APP_PATH" "$STAGING_DIR/Mory.app"
ln -s /Applications "$STAGING_DIR/Applications"

hdiutil create \
  -volname "Mory" \
  -srcfolder "$STAGING_DIR" \
  -ov \
  -format UDZO \
  "$DMG_PATH"

ditto -c -k --sequesterRsrc --keepParent \
  "$APP_PATH" \
  "$STAGING_ROOT/${ARTIFACT_BASE}.zip"
mv -f "$STAGING_ROOT/${ARTIFACT_BASE}.zip" "$ZIP_PATH"

(
  cd "$RELEASE_DIR"
  shasum -a 256 "$(basename "$DMG_PATH")" "$(basename "$ZIP_PATH")"
) > "$STAGING_ROOT/$(basename "$CHECKSUM_PATH")"
mv -f "$STAGING_ROOT/$(basename "$CHECKSUM_PATH")" "$CHECKSUM_PATH"

echo "Mory macOS 分发制品已生成："
ls -lh "$DMG_PATH" "$ZIP_PATH" "$CHECKSUM_PATH"
