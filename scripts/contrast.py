#!/usr/bin/env python3
"""Measure WCAG contrast ratios for the Evermore palette (Thenie, D32).

CLAUDE.md §7: "Accessibility is AA minimum, and contrast is *calculated*, not
eyeballed." This is the tool that calculates it. design.md §3 is the recorded
output; run this to re-earn those numbers rather than trusting them.

Usage:
    scripts/contrast.py                 # check every documented pairing
    scripts/contrast.py '#1C3D34' '#FFFAE0'
    scripts/contrast.py --alpha 'rgba(28,61,52,0.60)' '#FFFAE0'
"""

from __future__ import annotations

import pathlib
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
# ATELIER (D43, 2026-10-13). Modern and elegant; replaces Orchard, which read
# as retro. Only the typefaces have survived every redesign.

PAPER = "#F9F9F6"
CARD = "#FFFFFF"
WARM = "#EDEAE2"
INK = "#121614"
INK_SOFT = "#555D58"
INK_FAINT = "#656B67"
BRAND = "#13543E"
BRAND_HI = "#0D3C2C"
BRAND_LT = "#5FD1A4"
BRAND_WASH = "#E7F0EB"
ACCENT = "#D8F26B"
ACCENT_WASH = "#F3FADA"
INFO = "#2F6690"
INFO_WASH = "#E4EDF4"
LINE = "#E2E2DC"
LINE_STRONG = "#828883"
FOCUS = "#1F5FD0"
DARK = "#101311"
DARK_CARD = "#181C19"
WHITE = "#FFFFFF"

CHECKS: list[tuple[str, str, str, dict]] = [
    # --- ink on the three light surfaces
    ("ink on paper", INK, PAPER, {}),
    ("ink-soft on paper", INK_SOFT, PAPER, {}),
    ("ink-faint on paper", INK_FAINT, PAPER, {}),
    ("ink on a card", INK, CARD, {}),
    ("ink-faint on a card", INK_FAINT, CARD, {}),
    ("ink on the warm band", INK, WARM, {}),
    ("ink-soft on the warm band", INK_SOFT, WARM, {}),
    ("ink-faint on the warm band", INK_FAINT, WARM, {}),

    # --- brand
    ("brand as text on paper", BRAND, PAPER, {}),
    ("brand as text on a card", BRAND, CARD, {}),
    ("ink on the brand wash", INK, BRAND_WASH, {}),
    ("ink-soft on the brand wash", INK_SOFT, BRAND_WASH, {}),

    # --- accent: a FILL, never ink
    ("accent as text on paper (REJECTED)", ACCENT, PAPER, {}),
    ("on-brand on the accent fill", INK, ACCENT, {}),
    ("ink on the accent wash", INK, ACCENT_WASH, {}),

    # --- info
    ("info as text on paper", INFO, PAPER, {}),
    ("ink on the info wash", INK, INFO_WASH, {}),

    # --- controls, pinned in both schemes (the D42 rule)
    ("white on the brand control", WHITE, BRAND, {}),
    ("white on the brand hover", WHITE, BRAND_HI, {}),
    ("on-brand on the accent control", INK, ACCENT, {}),
    ("UNPINNED white on brand-lt (REJECTED)", WHITE, BRAND_LT, {}),

    # --- lines and focus
    ("line on paper (decorative)", LINE, PAPER, {"non_text": True}),
    ("line-strong on paper (control edge)", LINE_STRONG, PAPER, {"non_text": True}),
    ("line-strong on a card", LINE_STRONG, CARD, {"non_text": True}),
    ("focus ring on paper", FOCUS, PAPER, {"non_text": True}),
    ("focus ring on a card", FOCUS, CARD, {"non_text": True}),
    ("line-strong on the warm band", LINE_STRONG, WARM, {"non_text": True}),
    ("dark control edge on the ground", "#5E6962", DARK, {"non_text": True}),
    ("dark control edge on the card", "#5E6962", DARK_CARD, {"non_text": True}),
    ("REJECTED #9BA09C as a control edge", "#9BA09C", PAPER, {"non_text": True}),

    # --- footer, ink-grounded in both schemes
    ("paper on the ink footer", PAPER, INK, {}),
    ("fineprint #A9ADA9 on the footer", "#A9ADA9", INK, {}),
    ("accent link on the footer", ACCENT, INK, {}),

    # --- dark scheme
    ("ink on the dark ground", "#F2F3EF", DARK, {}),
    ("ink-soft on the dark ground", "#B4BBB6", DARK, {}),
    ("ink-faint on the dark ground", "#8E9691", DARK, {}),
    ("ink on the dark card", "#F2F3EF", DARK_CARD, {}),
    ("ink-faint on the dark card", "#8E9691", DARK_CARD, {}),
    ("brand-lt on the dark ground", BRAND_LT, DARK, {}),
    ("brand on the dark ground (REJECTED)", BRAND, DARK, {}),
    ("accent on the dark ground", ACCENT, DARK, {}),
    ("info-lt on the dark ground", "#8FB8DC", DARK, {}),
    ("focus-lt on the dark ground", "#9CC0FF", DARK, {"non_text": True}),

    # --- surfaces over the signature ribbon (D60)
    #
    # The ribbon's hairline strokes run along the same path as the wide ones
    # and so stack. #77968A is the darkest pixel it can produce over paper;
    # the two "under glass" grounds below are that pixel seen through a 50%
    # card, which is the worst background any text on these cards can get.
    ("ink on the quiet day-card head, worst ribbon pixel", INK, "#B3C4BC", {}),
    ("ink-on-glass on the quiet day-card head, worst ribbon pixel", "#454C48", "#B3C4BC", {}),
    ("ink-faint on the quiet day-card head, worst pixel (REJECTED)", "#656B67", "#B3C4BC", {}),
    ("brand on the day-card body, worst ribbon pixel", BRAND, "#BBCBC5", {}),
    ("brand-lt on the dark day-card body, lightest ribbon pixel", BRAND_LT, "#304F42", {}),
    ("ink on glass over paper", INK, "#FCFCFA", {}),
    ("ink-on-glass over paper", "#454C48", "#FCFCFA", {}),
    ("ink-on-glass on tinted glass over paper", "#454C48", "#F2F5F1", {}),
    ("ink-on-glass under glass, worst ribbon pixel", "#454C48", "#BBCBC5", {}),
    ("ink-on-glass under tinted glass, worst ribbon pixel", "#454C48", "#B0C4BB", {}),
    ("ink-faint under glass, worst ribbon pixel (REJECTED)", "#656B67", "#BBCBC5", {}),
    ("ink-soft under glass, worst ribbon pixel (REJECTED)", "#555D58", "#BBCBC5", {}),
    ("ink-on-glass on the dark glass, lightest ribbon pixel", "#C3C9C5", "#304F42", {}),

    # --- the hero's Indonesia map (D64). 7% of brand over the ground, which is
    # the darkest the hero can get where an island sits behind the type.
    ("ink on the hero, over the map", INK, "#E9EDE9", {}),
    ("ink-soft on the hero, over the map", "#555D58", "#E9EDE9", {}),
    ("ink-faint on the hero, over the map", "#656B67", "#E9EDE9", {}),
    ("brand script on the hero, over the map", BRAND, "#E9EDE9", {}),
    ("ink on the dark hero, over the map", "#F2F3EF", "#16201B", {}),
    ("ink-soft on the dark hero, over the map", "#B4BBB6", "#16201B", {}),

    # --- the WhatsApp float
    ("WA teal on paper", "#128C7E", PAPER, {"non_text": True}),
    ("WA white glyph on the teal", WHITE, "#128C7E", {"non_text": True}),
]

RECORDED: dict[str, float] = {
    "ink on paper": 17.30,
    "ink-soft on paper": 6.44,
    "ink-faint on paper": 5.17,
    "ink on a card": 18.25,
    "ink-faint on a card": 5.45,
    "ink on the warm band": 15.18,
    "ink-soft on the warm band": 5.65,
    "ink-faint on the warm band": 4.53,
    "brand as text on paper": 8.42,
    "brand as text on a card": 8.88,
    "ink on the brand wash": 15.70,
    "ink-soft on the brand wash": 5.84,
    "accent as text on paper (REJECTED)": 1.18,
    "on-brand on the accent fill": 14.63,
    "ink on the accent wash": 16.95,
    "info as text on paper": 5.81,
    "ink on the info wash": 15.40,
    "white on the brand control": 8.88,
    "white on the brand hover": 12.35,
    "on-brand on the accent control": 14.63,
    "UNPINNED white on brand-lt (REJECTED)": 1.88,
    "line on paper (decorative)": 1.23,
    "line-strong on paper (control edge)": 3.43,
    "line-strong on a card": 3.62,
    "focus ring on paper": 5.52,
    "focus ring on a card": 5.82,
    "line-strong on the warm band": 3.01,
    "dark control edge on the ground": 3.27,
    "dark control edge on the card": 3.01,
    "REJECTED #9BA09C as a control edge": 2.52,
    "paper on the ink footer": 17.30,
    "fineprint #A9ADA9 on the footer": 8.03,
    "accent link on the footer": 14.63,
    "ink on the dark ground": 16.77,
    "ink-soft on the dark ground": 9.55,
    "ink-faint on the dark ground": 6.16,
    "ink on the dark card": 15.46,
    "ink-faint on the dark card": 5.68,
    "brand-lt on the dark ground": 9.92,
    "brand on the dark ground (REJECTED)": 2.11,
    "accent on the dark ground": 14.99,
    "info-lt on the dark ground": 8.95,
    "focus-lt on the dark ground": 10.15,
    "ink on the quiet day-card head, worst ribbon pixel": 10.03,
    "ink-on-glass on the quiet day-card head, worst ribbon pixel": 4.85,
    "ink-faint on the quiet day-card head, worst pixel (REJECTED)": 3.00,
    "brand on the day-card body, worst ribbon pixel": 5.27,
    "brand-lt on the dark day-card body, lightest ribbon pixel": 4.80,
    "ink on glass over paper": 17.77,
    "ink-on-glass over paper": 8.59,
    "ink-on-glass on tinted glass over paper": 8.03,
    "ink-on-glass under glass, worst ribbon pixel": 5.24,
    "ink-on-glass under tinted glass, worst ribbon pixel": 4.81,
    "ink-faint under glass, worst ribbon pixel (REJECTED)": 3.23,
    "ink-soft under glass, worst ribbon pixel (REJECTED)": 4.03,
    "ink-on-glass on the dark glass, lightest ribbon pixel": 5.37,
    "ink on the hero, over the map": 15.43,
    "ink-soft on the hero, over the map": 5.74,
    "ink-faint on the hero, over the map": 4.61,
    "brand script on the hero, over the map": 7.51,
    "ink on the dark hero, over the map": 15.00,
    "ink-soft on the dark hero, over the map": 8.53,
    "WA teal on paper": 3.92,
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
        opts = dict(opts)
        # A translucent GROUND (the hero scrim) is flattened onto white first:
        # the worst case for text over an unknown photograph is the lightest
        # pixel the photo can supply.
        ground = to_rgb(ground_s, over=(255, 255, 255)) if opts.pop("over_white", False) else to_rgb(ground_s)
        ink = to_rgb(ink_s, over=ground)
        r = ratio(ink, ground)
        rec = RECORDED.get(label)
        note = ""
        if rec is not None and abs(rec - r) > 0.015:
            note = f"  <-- DRIFT, the sheet says {rec:.2f}"
            drift += 1
        recorded = f"{rec:.2f}" if rec is not None else "—"
        print(f"{label.ljust(width)}   {r:5.2f}   {verdict(r, **opts).ljust(15)}  {recorded}{note}")

    print()
    if drift:
        print(f"{drift} pairing(s) disagree with the recorded table — one of them is wrong.")
        return 1

    # The RECORDED table above is a hand-kept copy of design.md §3, and a
    # hand-kept copy drifts. The incident log has a guard that silently stopped
    # guarding because its oracle was stale, so this asserts the sheet and the
    # arithmetic still agree instead of claiming it.
    missing = _sheet_drift()
    if missing:
        print(f"{len(missing)} ratio(s) are NOT written in 10-design-system.md:")
        for label, value in missing:
            print(f"  {value:.2f}  ({label})")
        print("The arithmetic and the sheet disagree. Update 10-design-system.md.")
        return 1

    print(f"All {len(CHECKS)} pairings match, and every ratio appears in 10-design-system.md.")
    return 0


def _sheet_drift() -> list[tuple[str, float]]:
    """Return recorded ratios that do not appear in the design sheet.

    Deliberately a weak check on strong data: it looks for the NUMBER, not for
    surrounding prose, because prose wanders and a check that reads prose will
    eventually be argued into agreeing with the bug.
    """
    sheet = pathlib.Path(__file__).resolve().parent.parent / "docs/10-design-system.md"
    if not sheet.exists():
        return []
    text = sheet.read_text()
    out = []
    for label, value in RECORDED.items():
        if f"{value:.2f}" not in text:
            out.append((label, value))
    return out


if __name__ == "__main__":
    sys.exit(main(sys.argv))
