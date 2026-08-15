#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SDK_PATH="${MORY_SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"
OUTPUT_PATH="$PROJECT_DIR/.build/mory-mac-ime-smoke"

if ! defaults read com.apple.HIToolbox AppleSelectedInputSources 2>/dev/null | rg -q 'com.apple.inputmethod.SCIM.ITABC'; then
  echo "跳过真实简体拼音测试：当前输入源不是 macOS 简体拼音。"
  exit 0
fi

if [[ "$SDK_PATH" == *"CommandLineTools/SDKs/MacOSX26"* ]] && [[ -d "/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk" ]]; then
  SDK_PATH="/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk"
fi

mkdir -p "$PROJECT_DIR/.build" "$PROJECT_DIR/.cache/clang"
env CLANG_MODULE_CACHE_PATH="$PROJECT_DIR/.cache/clang" SDKROOT="$SDK_PATH" \
  swiftc -sdk "$SDK_PATH" -framework AppKit -framework WebKit \
  "$PROJECT_DIR/Tests/MacIMEInputSmoke.swift" -o "$OUTPUT_PATH"

cd "$PROJECT_DIR"
"$OUTPUT_PATH"
