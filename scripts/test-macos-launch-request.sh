#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mory-launch-test.XXXXXX")"
trap 'rm -R "$OUTPUT_DIR"' EXIT
SDK_PATH="${MORY_SDKROOT:-$(xcrun --sdk macosx --show-sdk-path)}"
SDK_LINK_TARGET="$(readlink "$SDK_PATH" 2>/dev/null || true)"
if { [[ "$SDK_PATH" == *"CommandLineTools/SDKs/MacOSX26"* ]] || [[ "$SDK_LINK_TARGET" == MacOSX26* ]]; } && [[ -d "/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk" ]]; then
  SDK_PATH="/Library/Developer/CommandLineTools/SDKs/MacOSX15.4.sdk"
fi
mkdir -p "$PROJECT_DIR/.cache/clang"

env CLANG_MODULE_CACHE_PATH="$PROJECT_DIR/.cache/clang" SDKROOT="$SDK_PATH" swiftc \
  "$PROJECT_DIR/Sources/Mory/LaunchRequest.swift" \
  "$PROJECT_DIR/Sources/Mory/WorkspaceManager.swift" \
  "$PROJECT_DIR/Tests/MacLaunchRequestSmoke.swift" \
  -o "$OUTPUT_DIR/MacLaunchRequestSmoke"
"$OUTPUT_DIR/MacLaunchRequestSmoke"
