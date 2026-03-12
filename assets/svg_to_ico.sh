#!/bin/bash
# svg_to_ico.sh - Generate Windows .ico icon from an SVG source
#
# Usage: ./svg_to_ico.sh [input.svg] [output.ico]
#   Defaults: input = ./assets/appicon.svg, output = ./assets/appicon.ico
#
# Requires: ImageMagick (magick or convert)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INPUT="${1:-$SCRIPT_DIR/appicon.svg}"
OUTPUT="${2:-$SCRIPT_DIR/appicon.ico}"

if [ ! -f "$INPUT" ]; then
  echo "ERROR: Source SVG not found at $INPUT"
  exit 1
fi

if command -v magick &>/dev/null; then
  magick "$INPUT" -background none -define icon:auto-resize=256,48,32,16 "$OUTPUT"
elif command -v convert &>/dev/null; then
  convert "$INPUT" -background none -define icon:auto-resize=256,48,32,16 "$OUTPUT"
else
  echo "ERROR: ImageMagick is not installed"
  exit 1
fi

echo "Created $OUTPUT"
