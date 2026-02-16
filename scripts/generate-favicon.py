#!/usr/bin/env python3
"""
Generate favicon files from SVG logo.
Requires: pip install cairosvg pillow
"""

import os
import sys
from pathlib import Path

try:
    import cairosvg
    from PIL import Image
    import io
except ImportError:
    print("Error: Required packages not installed")
    print("Install with: pip install cairosvg pillow")
    sys.exit(1)

# Paths
SCRIPT_DIR = Path(__file__).parent
PROJECT_ROOT = SCRIPT_DIR.parent
SVG_PATH = PROJECT_ROOT / "assets/static/images/logo.svg"
OUTPUT_DIR = PROJECT_ROOT / "assets/static/images"

# Sizes to generate
SIZES = [16, 32, 48, 64, 128, 256]

def svg_to_png(svg_path, output_path, size):
    """Convert SVG to PNG at specified size."""
    png_data = cairosvg.svg2png(
        url=str(svg_path),
        output_width=size,
        output_height=size
    )
    with open(output_path, 'wb') as f:
        f.write(png_data)
    print(f"✓ Generated {output_path.name} ({size}x{size})")

def create_ico(png_files, output_path):
    """Create multi-resolution ICO file from PNG files."""
    images = []
    for png_file in png_files:
        img = Image.open(png_file)
        images.append(img)

    images[0].save(
        output_path,
        format='ICO',
        sizes=[(img.width, img.height) for img in images]
    )
    print(f"✓ Generated {output_path.name}")

def main():
    if not SVG_PATH.exists():
        print(f"Error: SVG file not found at {SVG_PATH}")
        sys.exit(1)

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    print("Generating favicon files from logo.svg...")

    # Generate PNGs
    png_files = []
    for size in SIZES:
        png_path = OUTPUT_DIR / f"favicon-{size}.png"
        svg_to_png(SVG_PATH, png_path, size)
        png_files.append(png_path)

    # Generate apple-touch-icon
    apple_icon_path = OUTPUT_DIR / "apple-touch-icon.png"
    svg_to_png(SVG_PATH, apple_icon_path, 180)

    # Generate ICO file (using 16, 32, 48 for compatibility)
    ico_path = OUTPUT_DIR / "favicon.ico"
    create_ico([OUTPUT_DIR / f"favicon-{s}.png" for s in [16, 32, 48]], ico_path)

    print("\n✅ Favicon generation complete!")
    print(f"\nGenerated files in {OUTPUT_DIR}:")
    print("  - favicon.ico (multi-resolution)")
    print("  - favicon-*.png (various sizes)")
    print("  - apple-touch-icon.png (iOS)")

if __name__ == "__main__":
    main()
