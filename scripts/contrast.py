#!/usr/bin/env python3
"""Measure WCAG contrast ratios for the Evermore palette.

CLAUDE.md §7: "Accessibility is AA minimum, and contrast is *calculated*, not
eyeballed." This is the tool that calculates it. design.md §3 is the recorded
output; run this to re-earn those numbers rather than trusting them.

Usage:
    scripts/contrast.py                 # check every documented pairing
    scripts/contrast.py '#1C3D34' '#FFFAE0'
    scripts/contrast.py --alpha 'rgba(28,61,52,0.60)' '#FFFAE0'
"""

from __future__ import annotations

import re
import sys

# --- WCAG 2.1 relative luminance and contrast ------------------------------


def parse_hex(s: str) -> tuple[int, int, int]:
    s = s.strip().lstrip("#")
    if len(s) == 3:
        s = "".join(c * 2 for c in s)
    if len(s) != 6:
        raise ValueError(f"not a hex colour: {s!r}")
    return int(s[0:2], 16), int(s[2:4], 16), int(s[4:6], 16)


RGBA_RE = re.compile(
    r"rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*([\d.]+)\s*)?\)"
)


def parse_rgba(s: str) -> tuple[int, int, int, float]:
    m = RGBA_RE.fullmatch(s.strip())
    if not m:
        raise ValueError(f"not an rgba() colour: {s!r}")
    r, g, b = (int(m.group(i)) for i in (1, 2, 3))
    a = float(m.group(4)) if m.group(4) is not None else 1.0
    return r, g, b, a


def composite(fg: tuple[int, int, int, float], bg: tuple[int, int, int]) -> tuple[int, int, int]:
    """Flatten a translucent colour onto an opaque background.

    A border at 0.60 alpha is not the colour you typed; it is what that colour
    becomes over the surface behind it. Measuring the unflattened value is how
    a 1.69 ratio gets recorded as if it were 11.32.
    """
    r, g, b, a = fg
    return tuple(round(a * c + (1 - a) * d) for c, d in zip((r, g, b), bg))


def _channel(c: int) -> float:
    s = c / 255.0
    return s / 12.92 if s <= 0.04045 else ((s + 0.055) / 1.055) ** 2.4


def luminance(rgb: tuple[int, int, int]) -> float:
    r, g, b = (_channel(c) for c in rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def ratio(a: tuple[int, int, int], b: tuple[int, int, int]) -> float:
    la, lb = luminance(a), luminance(b)
    hi, lo = max(la, lb), min(la, lb)
    return (hi + 0.05) / (lo + 0.05)


def verdict(r: float, *, large: bool = False, non_text: bool = False) -> str:
    """AA needs 4.5:1 for body text, 3:1 for large text and non-text."""
    if non_text:
        return "PASS 1.4.11" if r >= 3.0 else "FAIL 1.4.11"
    if large:
        if r >= 4.5:
            return "AAA (large)"
        return "AA (large)" if r >= 3.0 else "FAIL"
    if r >= 7.0:
        return "AAA"
    if r >= 4.5:
        return "AA"
    return "FAIL"


def to_rgb(s: str, over: tuple[int, int, int] | None = None) -> tuple[int, int, int]:
    if s.strip().startswith("rgb"):
        r, g, b, a = parse_rgba(s)
        if a >= 1.0:
            return (r, g, b)
        if over is None:
            raise ValueError("a translucent colour needs a background to composite over")
        return composite((r, g, b, a), over)
    return parse_hex(s)


# --- the documented palette -------------------------------------------------

NOURISH_DEEP = "#1C3D34"
NOURISH = "#468973"
BEIGE = "#FFFAE0"
WHITE = "#FFFFFF"

CHECKS: list[tuple[str, str, str, dict]] = [
    # (label, ink, ground, options)
    ("white on deep ground", WHITE, NOURISH_DEEP, {}),
    ("beige on deep ground", BEIGE, NOURISH_DEEP, {}),
    ("blue-light on deep ground", "#B6DAFA", NOURISH_DEEP, {}),
    ("orange-light on deep ground", "#FFBC8F", NOURISH_DEEP, {}),
    ("beige-deep on deep ground", "#CCBDAA", NOURISH_DEEP, {}),

    ("white on mid-green bar", WHITE, NOURISH, {"large": True}),
    ("beige on mid-green bar", BEIGE, NOURISH, {"large": True}),
    ("deep ink on mid-green bar", NOURISH_DEEP, NOURISH, {}),
    ("beige-deep on mid-green bar", "#CCBDAA", NOURISH, {}),

    ("deep ink on beige sheet", NOURISH_DEEP, BEIGE, {}),
    ("brown-deep on beige", "#613F37", BEIGE, {}),
    ("berry-deep on beige", "#91253D", BEIGE, {}),
    ("blue-deep on beige", "#2E55A3", BEIGE, {}),
    ("ink-muted on beige", "#4A5D56", BEIGE, {}),
    ("orange-deep on beige", "#E0782D", BEIGE, {}),
    ("deep ink on white card", NOURISH_DEEP, WHITE, {}),

    ("--border over the sheet", "rgba(28,61,52,0.60)", BEIGE, {"non_text": True}),
    ("--border-subtle over the sheet", "rgba(28,61,52,0.28)", BEIGE, {"non_text": True}),
    ("beige-deep as a border on beige", "#CCBDAA", BEIGE, {"non_text": True}),

    ("deep ink on blue-light", NOURISH_DEEP, "#B6DAFA", {}),
    ("deep ink on orange-light", NOURISH_DEEP, "#FFBC8F", {}),
    ("deep ink on gold ribbon", NOURISH_DEEP, "#D4AF37", {}),
    ("deep ink on orange-deep button", NOURISH_DEEP, "#E0782D", {"large": True}),
    ("black on orange-deep button", "#000000", "#E0782D", {"large": True}),

    # The WhatsApp float (design.md §3). The teal is WhatsApp's own colour and
    # is NOT recoloured: the affordance is the beige ring, and the glyph is an
    # icon, so 1.4.11's 3:1 applies rather than 4.5.
    ("WA teal fill on the deep ground", "#128C7E", NOURISH_DEEP, {"non_text": True}),
    ("WA beige ring on the deep ground", BEIGE, NOURISH_DEEP, {"non_text": True}),
    ("WA beige ring against the teal", BEIGE, "#128C7E", {"non_text": True}),
    ("WA white glyph on the teal", WHITE, "#128C7E", {"non_text": True}),
]

# What design.md §3 records. A drift between the sheet and the arithmetic is
# a defect in one of them, so the checker reports it rather than printing a
# number nobody compares.
RECORDED = {
    "white on deep ground": 11.89,
    "beige on deep ground": 11.32,
    "blue-light on deep ground": 8.15,
    "orange-light on deep ground": 7.27,
    "beige-deep on deep ground": 6.47,
    "white on mid-green bar": 4.13,
    "beige on mid-green bar": 3.93,
    "deep ink on mid-green bar": 2.88,
    "beige-deep on mid-green bar": 2.25,
    "deep ink on beige sheet": 11.32,
    "brown-deep on beige": 8.79,
    "berry-deep on beige": 7.89,
    "blue-deep on beige": 6.79,
    "ink-muted on beige": 6.69,
    "orange-deep on beige": 2.90,
    "deep ink on white card": 11.89,
    "--border over the sheet": 3.55,
    "--border-subtle over the sheet": 1.69,
    "beige-deep as a border on beige": 1.75,
    "deep ink on blue-light": 8.15,
    "deep ink on orange-light": 7.27,
    "deep ink on gold ribbon": 5.65,
    "deep ink on orange-deep button": 3.90,
    "black on orange-deep button": 6.89,
    "WA teal fill on the deep ground": 2.87,
    "WA beige ring on the deep ground": 11.32,
    "WA beige ring against the teal": 3.94,
    "WA white glyph on the teal": 4.14,
}


def main(argv: list[str]) -> int:
    args = [a for a in argv[1:] if a != "--alpha"]
    if len(args) == 2:
        ground = to_rgb(args[1])
        ink = to_rgb(args[0], over=ground)
        r = ratio(ink, ground)
        print(f"{args[0]} on {args[1]} = {r:.2f}  {verdict(r)}")
        return 0

    drift = 0
    width = max(len(label) for label, *_ in CHECKS)
    print(f"{'pairing'.ljust(width)}   ratio   verdict          recorded")
    print("-" * (width + 40))
    for label, ink_s, ground_s, opts in CHECKS:
        ground = to_rgb(ground_s)
        ink = to_rgb(ink_s, over=ground)
        r = ratio(ink, ground)
        rec = RECORDED.get(label)
        note = ""
        if rec is not None and abs(rec - r) > 0.015:
            note = f"  <-- DRIFT, design.md says {rec:.2f}"
            drift += 1
        recorded = f"{rec:.2f}" if rec is not None else "—"
        print(f"{label.ljust(width)}   {r:5.2f}   {verdict(r, **opts).ljust(15)}  {recorded}{note}")

    print()
    if drift:
        print(f"{drift} pairing(s) disagree with design.md §3 — one of them is wrong.")
        return 1
    print(f"All {len(CHECKS)} pairings match the numbers recorded in design.md §3.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
