#!/bin/bash
set -euo pipefail

SUPERCOMPRESS=false
if [ "${1:-}" = "--supercompress" ]; then
  SUPERCOMPRESS=true
  shift
fi

if [ $# -lt 1 ]; then
  echo "Usage: $0 [--supercompress] <video1> <video2> [video3 ...]"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Output
GIF_OUT="$SCRIPT_DIR/demo.gif"
WEBM_OUT="$SCRIPT_DIR/demo.webm"

# Crop to app window (detected from frame analysis)
CROP="crop=1618:1218:10:2"

if [ "$SUPERCOMPRESS" = true ]; then
  GIF_SCALE=""
  GIF_LOSSY=100
else
  GIF_SCALE=""
  GIF_LOSSY=80
fi

# Create concat list from arguments
CONCAT_LIST=$(mktemp /tmp/concat_XXXXXX.txt)
PALETTE=$(mktemp /tmp/palette_XXXXXX.png)
trap "rm -f $CONCAT_LIST $PALETTE" EXIT

for f in "$@"; do
  echo "file '$(cd "$(dirname "$f")" && pwd)/$(basename "$f")'" >> "$CONCAT_LIST"
done

echo "==> Creating WebM..."
ffmpeg -y -f concat -safe 0 -i "$CONCAT_LIST" \
  -an \
  -vf "$CROP,scale=809:609" \
  -c:v libvpx-vp9 -crf 30 -b:v 0 \
  "$WEBM_OUT"

echo "==> Creating GIF (palette pass)..."
ffmpeg -y -f concat -safe 0 -i "$CONCAT_LIST" \
  -an \
  -vf "${CROP}${GIF_SCALE},fps=20,palettegen=stats_mode=full:max_colors=128" \
  "$PALETTE"

echo "==> Creating GIF (render pass)..."
ffmpeg -y -f concat -safe 0 -i "$CONCAT_LIST" -i "$PALETTE" \
  -an \
  -filter_complex "[0:v]${CROP}${GIF_SCALE},fps=20[v];[v][1:v]paletteuse=dither=floyd_steinberg" \
  "$GIF_OUT"

echo "==> Optimizing GIF with gifsicle..."
gifsicle -O3 --lossy=$GIF_LOSSY "$GIF_OUT" -o "$GIF_OUT"

echo "==> Done!"
ls -lh "$GIF_OUT" "$WEBM_OUT"
