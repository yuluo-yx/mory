#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$PROJECT_DIR/.build"
APP_DIR="$PROJECT_DIR/dist/macos/Mory.app"
SDK_PATH="${MORY_SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"

# 某些仅安装 Command Line Tools 的机器会出现默认 SDK 与 swiftc 补丁版本不匹配。
# 如果同时存在兼容的 15.4 SDK，则优先使用它；完整 Xcode 环境仍使用默认 SDK。
if [[ "$SDK_PATH" == *"CommandLineTools/SDKs/MacOSX26"* ]] && [[ -d "/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk" ]]; then
  SDK_PATH="/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk"
fi

mkdir -p "$PROJECT_DIR/.cache/clang" "$PROJECT_DIR/.cache/swiftpm"
mkdir -p "$PROJECT_DIR/.build/storage"
env GOCACHE="$PROJECT_DIR/.cache/go-build" go build -trimpath -o "$PROJECT_DIR/.build/storage/mory-storage" "$PROJECT_DIR/cmd/mory-storage"
env \
  CLANG_MODULE_CACHE_PATH="$PROJECT_DIR/.cache/clang" \
  SWIFTPM_MODULECACHE_OVERRIDE="$PROJECT_DIR/.cache/swiftpm" \
  SDKROOT="$SDK_PATH" \
  swift build -c release --scratch-path "$BUILD_DIR"

BIN_DIR="$(swift build -c release --scratch-path "$BUILD_DIR" --show-bin-path)"
mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources/Web" "$APP_DIR/Contents/Resources/storage"
cp "$BIN_DIR/Mory" "$APP_DIR/Contents/MacOS/Mory"
cp "$PROJECT_DIR/macOS/Info.plist" "$APP_DIR/Contents/Info.plist"
cp -R "$PROJECT_DIR/Sources/Mory/Web/." "$APP_DIR/Contents/Resources/Web/"
cp "$PROJECT_DIR/build/icon.png" "$APP_DIR/Contents/Resources/icon.png"
cp "$PROJECT_DIR/.build/storage/mory-storage" "$APP_DIR/Contents/Resources/storage/mory-storage"
codesign --force --deep --sign - "$APP_DIR"

echo "Mory macOS 应用已生成：$APP_DIR"
