#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SDK_PATH="${MORY_SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"
OUTPUT_PATH="$PROJECT_DIR/.build/mory-mac-workspace-watcher-smoke"

if [[ "$SDK_PATH" == *"CommandLineTools/SDKs/MacOSX26"* ]] && [[ -d "/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk" ]]; then
  SDK_PATH="/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk"
fi

mkdir -p "$PROJECT_DIR/.build" "$PROJECT_DIR/.cache/clang"
env CLANG_MODULE_CACHE_PATH="$PROJECT_DIR/.cache/clang" SDKROOT="$SDK_PATH" \
  swiftc -sdk "$SDK_PATH" -framework CoreServices \
  "$PROJECT_DIR/Sources/Mory/WorkspaceWatcher.swift" \
  "$PROJECT_DIR/Tests/MacWorkspaceWatcherSmoke.swift" \
  -o "$OUTPUT_PATH"

"$OUTPUT_PATH"
