#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$PROJECT_DIR/.build"
APP_DIR="$PROJECT_DIR/dist/macos/Mory.app"
SDK_PATH="${MORY_SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"

# Command Line Tools installations can expose an SDK newer than the bundled Swift compiler.
# Prefer the compatible 15.4 SDK when present; full Xcode installations retain their default SDK.
SDK_LINK_TARGET="$(readlink "$SDK_PATH" 2>/dev/null || true)"
if { [[ "$SDK_PATH" == *"CommandLineTools/SDKs/MacOSX26"* ]] || [[ "$SDK_LINK_TARGET" == MacOSX26* ]]; } && [[ -d "/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk" ]]; then
  SDK_PATH="/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk"
fi

mkdir -p "$PROJECT_DIR/.cache/clang" "$PROJECT_DIR/.cache/swiftpm"
mkdir -p "$PROJECT_DIR/.build/storage"
mkdir -p "$PROJECT_DIR/.build/cli"
"$PROJECT_DIR/scripts/build-macos-icons.sh"
env GOCACHE="$PROJECT_DIR/.cache/go-build" go build -trimpath -o "$PROJECT_DIR/.build/storage/mory-storage" "$PROJECT_DIR/cmd/mory-storage"
env GOCACHE="$PROJECT_DIR/.cache/go-build" go build -trimpath -o "$PROJECT_DIR/.build/cli/mory" "$PROJECT_DIR/cmd/mory"
env \
  CLANG_MODULE_CACHE_PATH="$PROJECT_DIR/.cache/clang" \
  SWIFTPM_MODULECACHE_OVERRIDE="$PROJECT_DIR/.cache/swiftpm" \
  SDKROOT="$SDK_PATH" \
  swift build -c release --scratch-path "$BUILD_DIR"

BIN_DIR="$(env CLANG_MODULE_CACHE_PATH="$PROJECT_DIR/.cache/clang" SWIFTPM_MODULECACHE_OVERRIDE="$PROJECT_DIR/.cache/swiftpm" SDKROOT="$SDK_PATH" swift build -c release --scratch-path "$BUILD_DIR" --show-bin-path)"
mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources/Web" "$APP_DIR/Contents/Resources/storage" "$APP_DIR/Contents/Resources/bin"
cp "$BIN_DIR/Mory" "$APP_DIR/Contents/MacOS/Mory"
cp "$PROJECT_DIR/macOS/Info.plist" "$APP_DIR/Contents/Info.plist"
cp -R "$PROJECT_DIR/Sources/Mory/Web/." "$APP_DIR/Contents/Resources/Web/"
cp "$PROJECT_DIR/assets/mory-icon.png" "$APP_DIR/Contents/Resources/icon.png"
cp "$PROJECT_DIR/.build/icons/icon.icns" "$APP_DIR/Contents/Resources/icon.icns"
cp "$PROJECT_DIR/.build/storage/mory-storage" "$APP_DIR/Contents/Resources/storage/mory-storage"
cp "$PROJECT_DIR/.build/cli/mory" "$APP_DIR/Contents/Resources/bin/mory"
codesign --force --deep --sign - "$APP_DIR"

echo "Mory macOS app generated: $APP_DIR"
