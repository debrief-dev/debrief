#!/bin/bash
# Convert SVG to PNG at multiple sizes (1000x1000, 500x500, 256x256)
# Usage: ./svg_to_png.sh input.svg [output_directory]

# ImageMagic installation
# winget install ImageMagick.ImageMagick
# sudo apt install imagemagick

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <input.svg> [output_directory]"
    exit 1
fi

PNG_DPI="300"

INPUT="$1"
BASENAME=$(basename "$INPUT" .svg)
OUTDIR="${2:-.}"

mkdir -p "$OUTDIR"

SIZES=(128 220 256 512)

for SIZE in "${SIZES[@]}"; do
    OUTPUT="${OUTDIR}/${BASENAME}_${SIZE}x${SIZE}.png"
    echo "Generating ${OUTPUT}..."

    if command -v magick &> /dev/null; then
        magick -background none -density "$PNG_DPI" "$INPUT" -resize "${SIZE}x${SIZE}" "$OUTPUT"
    elif command -v convert &> /dev/null; then
        convert -background none -density "$PNG_DPI" "$INPUT" -resize "${SIZE}x${SIZE}" "$OUTPUT"
    else
        echo "Error: No SVG converter found. Install imagemagick"
        exit 1
    fi
done

echo "Done! Generated PNGs in ${OUTDIR}/"
