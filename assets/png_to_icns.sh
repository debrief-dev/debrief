#!/bin/bash
# png_to_icns.sh - Generate macOS .icns icon from a source PNG
#
# Usage: ./png_to_icns.sh [input.png] [output.icns]
#   Defaults: input = ./assets/appicon.png, output = ./assets/appicon.icns
#
# Requires: sips, iconutil (available on macOS by default)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INPUT="${1:-$SCRIPT_DIR/appicon.png}"
OUTPUT="${2:-$SCRIPT_DIR/appicon.icns}"

if [ ! -f "$INPUT" ]; then
  echo "ERROR: Source PNG not found at $INPUT"
  exit 1
fi

for cmd in sips iconutil; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: Required tool '$cmd' is not installed (must run on macOS)"
    exit 1
  fi
done

ICONSET=$(mktemp -d)/appicon.iconset
mkdir -p "$ICONSET"

cleanup() {
  rm -rf "$(dirname "$ICONSET")"
}
trap cleanup EXIT

# Standard sizes: 16, 32, 128, 256, 512 at 1x and 2x
SIZES=(16 32 128 256 512)

for size in "${SIZES[@]}"; do
  double=$((size * 2))
  sips -z "$size" "$size" "$INPUT" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
  sips -z "$double" "$double" "$INPUT" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done

iconutil -c icns "$ICONSET" -o "$OUTPUT"

echo "Created $OUTPUT"
