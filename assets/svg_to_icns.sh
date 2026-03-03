#!/bin/bash
# png_to_icns.sh - Generate macOS .icns icon from an SVG source
#
# Usage: ./png_to_icns.sh [input.svg] [output.icns]
#   Defaults: input = ./assets/appicon.svg, output = ./assets/appicon.icns
#
# Requires: rsvg-convert (brew install librsvg), iconutil

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INPUT="${1:-$SCRIPT_DIR/appicon.svg}"
OUTPUT="${2:-$SCRIPT_DIR/appicon.icns}"

if [ ! -f "$INPUT" ]; then
  echo "ERROR: Source SVG not found at $INPUT"
  exit 1
fi

for cmd in rsvg-convert iconutil; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: Required tool '$cmd' is not installed"
    echo "  brew install librsvg"
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
  rsvg-convert -w "$size" -h "$size" "$INPUT" -o "$ICONSET/icon_${size}x${size}.png"
  rsvg-convert -w "$double" -h "$double" "$INPUT" -o "$ICONSET/icon_${size}x${size}@2x.png"
done

iconutil -c icns "$ICONSET" -o "$OUTPUT"

echo "Created $OUTPUT"
