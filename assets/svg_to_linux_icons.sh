#!/bin/bash
# Generate Linux icon PNGs at multiple sizes from appicon-linux.svg
# Usage: ./svg_to_linux_icons.sh
#
# Requires ImageMagick:
#   sudo apt install imagemagick
#   winget install ImageMagick.ImageMagick

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INPUT="$SCRIPT_DIR/appicon-linux.svg"
OUTDIR="$SCRIPT_DIR/linux-icons"

mkdir -p "$OUTDIR"

SIZES=(16 24 32 48 64 128 256 512)

for SIZE in "${SIZES[@]}"; do
    OUTPUT="${OUTDIR}/debrief-${SIZE}.png"
    echo "Generating ${SIZE}x${SIZE}..."

    if command -v magick &> /dev/null; then
        magick -background none -density 300 "$INPUT" -resize "${SIZE}x${SIZE}" "$OUTPUT"
    elif command -v convert &> /dev/null; then
        convert -background none -density 300 "$INPUT" -resize "${SIZE}x${SIZE}" "$OUTPUT"
    else
        echo "Error: No SVG converter found. Install imagemagick"
        exit 1
    fi
done

echo "Done! Generated PNGs in ${OUTDIR}/"
