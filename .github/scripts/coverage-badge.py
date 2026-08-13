#!/usr/bin/env python3
"""Shields-style coverage badge SVG. Usage: coverage-badge.py 42.1 out.svg"""

from __future__ import annotations

import sys
from pathlib import Path
from xml.sax.saxutils import escape


def color_for(pct: float) -> str:
    if pct >= 80:
        return "#4c1"
    if pct >= 50:
        return "#dfb317"
    if pct >= 40:
        return "#fe7d37"
    return "#e05d44"


def text_width(s: str) -> int:
    return round(len(s) * 6.5 + 10)


def main() -> None:
    if len(sys.argv) != 3:
        sys.exit("usage: coverage-badge.py PERCENT OUT.svg")
    raw = sys.argv[1].strip().rstrip("%")
    pct = float(raw)
    label = "coverage"
    value = f"{raw}%"
    color = color_for(pct)
    lw, vw = text_width(label), text_width(value)
    w = lw + vw
    label_x = lw * 5
    value_x = int((lw + vw / 2) * 10)
    svg = f"""<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="20" role="img" aria-label="{escape(label)}: {escape(value)}">
  <title>{escape(label)}: {escape(value)}</title>
  <linearGradient id="s" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>
  <clipPath id="r"><rect width="{w}" height="20" rx="3" fill="#fff"/></clipPath>
  <g clip-path="url(#r)">
    <rect width="{lw}" height="20" fill="#555"/>
    <rect x="{lw}" width="{vw}" height="20" fill="{color}"/>
    <rect width="{w}" height="20" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="110">
    <text x="{label_x}" y="140" transform="scale(.1)">{escape(label)}</text>
    <text x="{value_x}" y="140" transform="scale(.1)">{escape(value)}</text>
  </g>
</svg>
"""
    out = Path(sys.argv[2])
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(svg, encoding="utf-8")


if __name__ == "__main__":
    main()
