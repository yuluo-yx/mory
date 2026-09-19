#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SDK_PATH="${MORY_SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"
OUTPUT_PATH="$PROJECT_DIR/.build/mory-mac-ime-smoke"

SDK_LINK_TARGET="$(readlink "$SDK_PATH" 2>/dev/null || true)"
if { [[ "$SDK_PATH" == *"CommandLineTools/SDKs/MacOSX26"* ]] || [[ "$SDK_LINK_TARGET" == MacOSX26* ]]; } && [[ -d "/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk" ]]; then
  SDK_PATH="/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk"
fi

mkdir -p "$PROJECT_DIR/.build" "$PROJECT_DIR/.cache/clang"
env CLANG_MODULE_CACHE_PATH="$PROJECT_DIR/.cache/clang" SDKROOT="$SDK_PATH" \
  swiftc -sdk "$SDK_PATH" -framework AppKit -framework WebKit -framework Carbon \
  "$PROJECT_DIR/Tests/MacIMEInputSmoke.swift" -o "$OUTPUT_PATH"

cd "$PROJECT_DIR"
node "$PROJECT_DIR/scripts/run-native-smoke.mjs" 40 "$OUTPUT_PATH"

MISSING_SOURCE_LOG="$PROJECT_DIR/.build/mory-ime-missing-source.log"
if node "$PROJECT_DIR/scripts/run-native-smoke.mjs" 40 "$OUTPUT_PATH" --missing-input-source > "$MISSING_SOURCE_LOG" 2>&1; then
  cat "$MISSING_SOURCE_LOG"
  echo "An unavailable input source was incorrectly accepted."
  exit 1
else
  MISSING_SOURCE_STATUS=$?
fi
if [[ "$MISSING_SOURCE_STATUS" != 77 ]] || ! grep -q 'SKIP: Simplified Pinyin is not installed or selectable.' "$MISSING_SOURCE_LOG"; then
  cat "$MISSING_SOURCE_LOG"
  echo "An unavailable input source did not produce the expected inconclusive result."
  exit 1
fi
echo "Native IME unavailable-source detection passed: exit 77 without a crash."

# An unrelated native key must produce an explicit inconclusive result, never a pass.
INTERFERENCE_LOG="$PROJECT_DIR/.build/mory-ime-interference.log"
if node "$PROJECT_DIR/scripts/run-native-smoke.mjs" 40 "$OUTPUT_PATH" --inject-unrelated-key > "$INTERFERENCE_LOG" 2>&1; then
  cat "$INTERFERENCE_LOG"
  echo "Native IME interference was incorrectly accepted."
  exit 1
else
  INTERFERENCE_STATUS=$?
fi
if [[ "$INTERFERENCE_STATUS" != 77 ]] || ! grep -q 'SKIP: Unrelated keyboard input' "$INTERFERENCE_LOG"; then
  cat "$INTERFERENCE_LOG"
  echo "Native IME interference did not produce the expected inconclusive result."
  exit 1
fi
echo "Native IME interference detection passed: unrelated input exits 77."
