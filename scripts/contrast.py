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
# ORCHARD (D41, 2026-10-13). A complete redesign; nothing survives from the
# Nourish palette (pre-D32) or the Thenie palette (D32-D39) but the typefaces.

PAPER = "#FDFBF6"        # warm paper, deliberately not #FFFFFF
PAPER2 = "#F6F1E6"       # alternating band
CARD = "#FFFFFF"
INK = "#2A2521"
INK_SOFT = "#5C544A"
INK_FAINT = "#766D62"
GREEN = "#2F7D5C"
GREEN_HI = "#246349"
GREEN_LT = "#5FBF95"
GREEN_WASH = "#DCEFE5"
ORANGE = "#F2784B"       # a FILL that carries ink, never ink on paper
ORANGE_DEEP = "#C8471D"  # the text/white-on cut
ORANGE_WASH = "#FDE6DC"
BLUE = "#4C6FBF"
BLUE_WASH = "#E1E7F8"
BUTTER = "#F6C65B"
BUTTER_WASH = "#FDF2D6"
FOCUS = "#1F4FD8"
RULE_SOFT = "#D8D0C2"
DARK = "#1C1A17"         # dark-scheme ground
DARK_CARD = "#262320"
WHITE = "#FFFFFF"

CHECKS: list[tuple[str, str, str, dict]] = [
    # --- ink on the three light grounds
    ("ink on paper", INK, PAPER, {}),
    ("ink-soft on paper", INK_SOFT, PAPER, {}),
    ("ink-faint on paper", INK_FAINT, PAPER, {}),
    ("ink on the band", INK, PAPER2, {}),
    ("ink-soft on the band", INK_SOFT, PAPER2, {}),
    ("ink-faint on the band", INK_FAINT, PAPER2, {}),
    ("ink on a card", INK, CARD, {}),
    ("ink-soft on a card", INK_SOFT, CARD, {}),
    ("ink-faint on a card (placeholder)", INK_FAINT, CARD, {}),

    # --- green: text and the primary button
    ("green as text on paper", GREEN, PAPER, {}),
    ("green as text on a card", GREEN, CARD, {}),
    ("white on the green button", WHITE, GREEN, {}),
    ("white on green hover", WHITE, GREEN_HI, {}),
    ("ink on the green wash", INK, GREEN_WASH, {}),
    ("ink-soft on the green wash", INK_SOFT, GREEN_WASH, {}),

    # --- orange: a fill, not an ink
    ("orange as text on paper (REJECTED)", ORANGE, PAPER, {}),
    ("ink on the orange fill", INK, ORANGE, {}),
    ("orange-deep as text on paper", ORANGE_DEEP, PAPER, {}),
    ("white on orange-deep", WHITE, ORANGE_DEEP, {}),
    ("ink on the orange wash", INK, ORANGE_WASH, {}),

    # --- butter: the eyebrow sticker and the warn pill
    ("ink on the butter sticker", INK, BUTTER, {}),
    ("butter as text on paper (REJECTED)", BUTTER, PAPER, {}),
    ("ink on the butter wash", INK, BUTTER_WASH, {}),

    # --- blue
    ("blue as text on paper", BLUE, PAPER, {}),
    ("ink on the blue wash", INK, BLUE_WASH, {}),

    # --- lines and focus
    ("the ink rule on paper", INK, PAPER, {"non_text": True}),
    ("rule-soft on paper (decorative)", RULE_SOFT, PAPER, {"non_text": True}),
    ("focus ring on paper", FOCUS, PAPER, {"non_text": True}),
    ("focus ring on a card", FOCUS, CARD, {"non_text": True}),
    ("focus ring on the band", FOCUS, PAPER2, {"non_text": True}),

    # --- the footer, which is ink-grounded in both schemes
    ("paper on the ink footer", PAPER, INK, {}),
    ("fineprint #CFC7B9 on the footer", "#CFC7B9", INK, {}),
    ("butter link on the footer", BUTTER, INK, {}),

    # --- dark scheme
    ("ink on the dark ground", "#F6F2E9", DARK, {}),
    ("ink-soft on the dark ground", "#C2B9AC", DARK, {}),
    ("ink-faint on the dark ground", "#9C9284", DARK, {}),
    ("ink on the dark card", "#F6F2E9", DARK_CARD, {}),
    ("green-lt on the dark ground", GREEN_LT, DARK, {}),
    ("green on the dark ground (REJECTED)", GREEN, DARK, {}),
    ("orange-lt on the dark ground", "#FF9B6E", DARK, {}),
    ("blue-lt on the dark ground", "#8FA8EE", DARK, {}),
    ("butter on the dark ground", BUTTER, DARK, {}),
    ("focus-lt on the dark ground", "#9DB6FF", DARK, {"non_text": True}),

    # --- controls are PINNED in both schemes (D41).
    # Any element whose background is a fixed brand colour takes --on-brand,
    # which is never redefined in a dark block. The three REJECTED rows are
    # what the dark scheme actually measured before pinning.
    ("white on the pinned green control", WHITE, "#2F7D5C", {}),
    ("white on the pinned green hover", WHITE, "#246349", {}),
    ("on-brand on the pinned butter control", INK, BUTTER, {}),
    ("on-brand on the pinned orange control", INK, ORANGE, {}),
    ("UNPINNED white on dark green (REJECTED)", WHITE, "#5FBF95", {}),
    ("UNPINNED dark ink on butter (REJECTED)", "#F6F2E9", BUTTER, {}),
    ("UNPINNED dark ink on orange-lt (REJECTED)", "#F6F2E9", "#FF9B6E", {}),

    # --- the WhatsApp float keeps its own colour
    ("WA teal on paper", "#128C7E", PAPER, {"non_text": True}),
    ("WA white glyph on the teal", WHITE, "#128C7E", {"non_text": True}),
]

RECORDED: dict[str, float] = {
    "ink on paper": 14.66,
    "ink-soft on paper": 7.20,
    "ink-faint on paper": 4.91,
    "ink on the band": 13.46,
    "ink-soft on the band": 6.61,
    "ink-faint on the band": 4.51,
    "ink on a card": 15.16,
    "ink-soft on a card": 7.44,
    "ink-faint on a card (placeholder)": 5.08,
    "green as text on paper": 4.82,
    "green as text on a card": 4.99,
    "white on the green button": 4.99,
    "white on green hover": 7.10,
    "ink on the green wash": 12.65,
    "ink-soft on the green wash": 6.21,
    "orange as text on paper (REJECTED)": 2.68,
    "ink on the orange fill": 5.46,
    "orange-deep as text on paper": 4.64,
    "white on orange-deep": 4.80,
    "ink on the orange wash": 12.66,
    "ink on the butter sticker": 9.49,
    "butter as text on paper (REJECTED)": 1.54,
    "ink on the butter wash": 13.61,
    "blue as text on paper": 4.69,
    "ink on the blue wash": 12.26,
    "the ink rule on paper": 14.66,
    "rule-soft on paper (decorative)": 1.48,
    "focus ring on paper": 6.41,
    "focus ring on a card": 6.63,
    "focus ring on the band": 5.88,
    "paper on the ink footer": 14.66,
    "fineprint #CFC7B9 on the footer": 9.04,
    "butter link on the footer": 9.49,
    "ink on the dark ground": 15.54,
    "ink-soft on the dark ground": 8.95,
    "ink-faint on the dark ground": 5.67,
    "ink on the dark card": 13.99,
    "green-lt on the dark ground": 7.75,
    "green on the dark ground (REJECTED)": 3.48,
    "orange-lt on the dark ground": 8.40,
    "blue-lt on the dark ground": 7.44,
    "butter on the dark ground": 10.87,
    "focus-lt on the dark ground": 8.74,
    "white on the pinned green control": 4.99,
    "white on the pinned green hover": 7.10,
    "on-brand on the pinned butter control": 9.49,
    "on-brand on the pinned orange control": 5.46,
    "UNPINNED white on dark green (REJECTED)": 2.24,
    "UNPINNED dark ink on butter (REJECTED)": 1.43,
    "UNPINNED dark ink on orange-lt (REJECTED)": 1.85,
    "WA teal on paper": 4.00,
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
