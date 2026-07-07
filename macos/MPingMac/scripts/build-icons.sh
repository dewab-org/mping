#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SVG_INPUT="${1:-"$PROJECT_DIR/Sources/MPingMac/Resources/app-icon.svg"}"
ICONSET_DIR="${ICONSET_DIR:-"$PROJECT_DIR/AppIcon.iconset"}"
OUTPUT_ICNS="${OUTPUT_ICNS:-"$PROJECT_DIR/Sources/MPingMac/Resources/AppIcon.icns"}"
RENDERER="${ICON_RENDERER:-auto}"

if [ ! -f "$SVG_INPUT" ]; then
  echo "SVG not found: $SVG_INPUT" >&2
  exit 1
fi

mkdir -p "$ICONSET_DIR"
rm -f "$ICONSET_DIR"/*.png

choose_renderer() {
  local choice="$1"
  if [ "$choice" = "rsvg" ] && command -v rsvg-convert >/dev/null 2>&1; then
    echo "rsvg"
  elif [ "$choice" = "inkscape" ] && command -v inkscape >/dev/null 2>&1; then
    echo "inkscape"
  elif [ "$choice" = "magick" ] && (command -v magick >/dev/null 2>&1 || command -v convert >/dev/null 2>&1); then
    echo "magick"
  elif [ "$choice" = "auto" ]; then
    if command -v rsvg-convert >/dev/null 2>&1; then
      echo "rsvg"
    elif command -v inkscape >/dev/null 2>&1; then
      echo "inkscape"
    elif command -v magick >/dev/null 2>&1 || command -v convert >/dev/null 2>&1; then
      echo "magick"
    fi
  fi
}

RENDER_WITH="$(choose_renderer "$RENDERER")"

if [ -z "$RENDER_WITH" ]; then
  echo "No renderer available. Install one of: rsvg-convert (best), inkscape, or ImageMagick (magick/convert)." >&2
  exit 1
fi

render() {
  local size="$1"
  local name="$2"
  case "$RENDER_WITH" in
    rsvg)
      rsvg-convert -w "$size" -h "$size" -o "$ICONSET_DIR/$name" "$SVG_INPUT"
      ;;
    inkscape)
      inkscape "$SVG_INPUT" --export-type=png --export-filename="$ICONSET_DIR/$name" --export-width="$size" --export-height="$size" --export-background-opacity=0
      ;;
    magick)
      if command -v magick >/dev/null 2>&1; then
        magick convert "$SVG_INPUT" -background none -resize "${size}x${size}" "$ICONSET_DIR/$name"
      else
        # IMv7 warns when calling `convert` directly; prefer a sibling `magick` if present.
        CONVERT_PATH="$(command -v convert)"
        CONVERT_DIR="$(cd "$(dirname "$CONVERT_PATH")" && pwd)"
        if [ -x "$CONVERT_DIR/magick" ]; then
          "$CONVERT_DIR/magick" convert "$SVG_INPUT" -background none -resize "${size}x${size}" "$ICONSET_DIR/$name"
        else
          convert "$SVG_INPUT" -background none -resize "${size}x${size}" "$ICONSET_DIR/$name"
          echo "NOTE: Using ImageMagick 'convert'; IMv7 may warn. Install 'magick' or ensure it is on PATH to avoid this." >&2
        fi
      fi
      ;;
  esac
}

for base in 16 32 128 256 512; do
  render "$base" "icon_${base}x${base}.png"
  render "$((base * 2))" "icon_${base}x${base}@2x.png"
done

iconutil -c icns "$ICONSET_DIR" -o "$OUTPUT_ICNS"
echo "Built $OUTPUT_ICNS"
