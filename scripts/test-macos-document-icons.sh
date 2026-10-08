#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_PATH="$PROJECT_DIR/dist/macos/Mory.app"
SDK_PATH="${MORY_SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"
OUTPUT_PATH="$PROJECT_DIR/.build/mory-document-icon-smoke"
SDK_LINK_TARGET="$(readlink "$SDK_PATH" 2>/dev/null || true)"
if { [[ "$SDK_PATH" == *"CommandLineTools/SDKs/MacOSX26"* ]] || [[ "$SDK_LINK_TARGET" == MacOSX26* ]]; } && [[ -d "/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk" ]]; then
  SDK_PATH="/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk"
fi

cd "$PROJECT_DIR"
env MORY_TEST_APP_BUNDLE="$APP_PATH" node --test Tests/mac-document-icons.test.cjs
mkdir -p "$PROJECT_DIR/.build" "$PROJECT_DIR/.cache/clang"
env CLANG_MODULE_CACHE_PATH="$PROJECT_DIR/.cache/clang" SDKROOT="$SDK_PATH" \
  swiftc -parse-as-library -sdk "$SDK_PATH" -framework AppKit \
  "$PROJECT_DIR/Tests/MacDocumentIconSmoke.swift" -o "$OUTPUT_PATH"
node "$PROJECT_DIR/scripts/run-native-smoke.mjs" 30 "$OUTPUT_PATH" "$APP_PATH" "$PROJECT_DIR/assets/mory-icon.png"
