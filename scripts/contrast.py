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
# D32 (2026-09-30): the Thenie palette. Read out of
# docs/design_reference/thenie_site_FINAL_14.html. Five of its own values fail
# AA; each correction is checked below beside the value it replaces, so the
# failure is recorded rather than quietly fixed.

CREAM = "#FFFFFF"          # page ground
SOFT = "#FAF5E9"           # alternating band
INK = "#2B2620"
INK_SOFT = "#615848"
INK_FAINT = "#787061"      # CORRECTED from Thenie's #8B8271
CORAL = "#E1614A"          # large text and borders only
CORAL_DEEP = "#B84B39"     # the fill under white text
CORAL_DEEPEST = "#9C3E2E"  # fill hover
CORAL_LIGHT = "#F08A75"    # links on a dark ground
OLIVE = "#5F7A33"
OLIVE_DEEP = "#3E5321"
OLIVE_TINT = "#E7EEDA"
GOLD = "#D9A867"           # decoration only on light grounds
DARK = "#1E1A16"           # dark-scheme ground

CHECKS: list[tuple[str, str, str, dict]] = [
    # --- ink on the two light grounds
    ("ink on white", INK, CREAM, {}),
    ("ink-soft on white", INK_SOFT, CREAM, {}),
    ("ink-faint on white", INK_FAINT, CREAM, {}),
    ("ink on cream-soft band", INK, SOFT, {}),
    ("ink-soft on cream-soft band", INK_SOFT, SOFT, {}),
    ("ink-faint on cream-soft band", INK_FAINT, SOFT, {}),
    # Thenie's own value, kept so nobody restores it. 3.80 / 3.49 — both under
    # the 4.5 body floor, which is why --ink-faint was darkened.
    ("THENIE #8B8271 on white (REJECTED)", "#8B8271", CREAM, {}),
    ("THENIE #8B8271 on cream-soft (REJECTED)", "#8B8271", SOFT, {}),

    # --- coral. The headline failure of the source palette.
    ("white on coral #E1614A (REJECTED fill)", CREAM, CORAL, {}),
    ("white on coral-deep (the fill)", CREAM, CORAL_DEEP, {}),
    ("white on coral-deepest (hover)", CREAM, CORAL_DEEPEST, {}),
    ("coral as body text on white (REJECTED)", CORAL, CREAM, {}),
    ("coral as LARGE text on white", CORAL, CREAM, {"large": True}),
    ("coral as a border on white", CORAL, CREAM, {"non_text": True}),
    ("coral-deep as text on white", CORAL_DEEP, CREAM, {}),
    ("coral-deep as text on cream-soft", CORAL_DEEP, SOFT, {}),
    ("ink on coral-tint", INK, "#FCE3DE", {}),

    # --- olive
    ("olive-deep on olive-tint (ghost btn)", OLIVE_DEEP, OLIVE_TINT, {}),
    ("white on olive (ghost hover)", CREAM, OLIVE, {}),
    ("olive as text on white", OLIVE, CREAM, {}),
    ("olive-deep as text on white", OLIVE_DEEP, CREAM, {}),
    # The footer bar. --olive is four hundredths under the floor, which is why
    # the bar is olive-deep.
    ("cream-soft on olive (REJECTED bar)", SOFT, OLIVE, {}),
    ("cream-soft on olive-deep (the bar)", SOFT, OLIVE_DEEP, {}),
    # .card-media's lightest gradient stop — this is what keeps the bar rule.
    ("cream-soft on olive (card-media)", SOFT, OLIVE, {"large": True}),

    # --- gold: decoration only
    ("gold as text on white (REJECTED)", GOLD, CREAM, {}),
    ("gold as LARGE text on white (REJECTED)", GOLD, CREAM, {"large": True}),
    ("gold as a border on white (REJECTED)", GOLD, CREAM, {"non_text": True}),
    ("ink on the gold fill", INK, GOLD, {}),
    ("ink on the ribbon", INK, "#E4D3AA", {}),

    # --- lines and focus
    ("paper-line over white (decorative)", "rgba(150,135,115,0.45)", CREAM, {"non_text": True}),
    ("border-strong on white", "#968773", CREAM, {"non_text": True}),
    ("border-strong on cream-soft", "#968773", SOFT, {"non_text": True}),
    ("THENIE focus #8FC4EC on white (REJECTED)", "#8FC4EC", CREAM, {"non_text": True}),
    ("focus-ring #1F6FB2 on white", "#1F6FB2", CREAM, {"non_text": True}),
    ("focus-ring #1F6FB2 on cream-soft", "#1F6FB2", SOFT, {"non_text": True}),
    ("focus-ring-on-dark on ink", "#8FC4EC", INK, {"non_text": True}),

    # --- the dark ground (.ground-deep = --ink)
    ("cream-soft on the dark ground", SOFT, INK, {}),
    ("muted #CDC3B2 on the dark ground", "#CDC3B2", INK, {}),
    ("coral on the dark ground (REJECTED link)", CORAL, INK, {}),
    ("coral-light on the dark ground", CORAL_LIGHT, INK, {}),
    ("cream .55 border on the dark ground", "rgba(250,245,233,0.55)", INK, {"non_text": True}),

    # --- dark scheme
    ("cream-soft on the dark scheme ground", SOFT, DARK, {}),
    ("ink-soft-dark on the dark scheme", "#CDC3B2", DARK, {}),
    ("ink-faint-dark on the dark scheme", "#A79D8C", DARK, {}),
    ("coral-light on the dark scheme", CORAL_LIGHT, DARK, {}),
    ("gold as text on the dark scheme", GOLD, DARK, {}),
    ("olive on the dark scheme (REJECTED)", OLIVE, DARK, {}),
    ("olive-light on the dark scheme", "#A3C16B", DARK, {}),

    # --- semantic rules
    ("info rule on white", "#2E55A3", CREAM, {}),
    ("warn rule on white", "#8A5A26", CREAM, {}),
    ("danger rule on white", "#91253D", CREAM, {}),
    ("ink on the info tint", INK, "#DCE9F7", {}),
    ("ink on the warn tint", INK, "#F7E8CE", {}),

    # The WhatsApp float. The teal is WhatsApp's own colour and is NOT
    # recoloured. On the retired deep-green ground it measured 2.87 and the
    # beige ring was the affordance; on both of these grounds it clears 3:1
    # unaided, so the ring is decoration (10 §4.7).
    ("WA teal on the white page", "#128C7E", CREAM, {"non_text": True}),
    ("WA teal on the dark scheme", "#128C7E", DARK, {"non_text": True}),
    ("WA white glyph on the teal", CREAM, "#128C7E", {"non_text": True}),
    ("cream ring on white (REJECTED)", SOFT, CREAM, {"non_text": True}),

    # The D34 hero scrim. The photograph behind it is unknown, so every stop
    # is measured against the WORST case: a pure-white pixel underneath.
    ("hero scrim .88 — the copy band", CREAM, "rgba(9,7,5,0.88)", {"over_white": True}),
    ("hero scrim .62 — the wordmark band", CREAM, "rgba(9,7,5,0.62)", {"over_white": True}),
    ("hero scrim .56 — lightest band", CREAM, "rgba(9,7,5,0.56)", {"over_white": True}),
    ("THENIE scrim .15 (REJECTED)", CREAM, "rgba(9,7,5,0.15)", {"over_white": True}),
]

# What 10-design-system.md records. A drift between the sheet and the
# arithmetic is a defect in one of them, so the checker reports it rather
# than printing a number nobody compares.
RECORDED: dict[str, float] = {
    "ink on white": 14.99,
    "ink-soft on white": 7.01,
    "ink-faint on white": 4.90,
    "ink on cream-soft band": 13.78,
    "ink-soft on cream-soft band": 6.44,
    "ink-faint on cream-soft band": 4.50,
    "THENIE #8B8271 on white (REJECTED)": 3.80,
    "THENIE #8B8271 on cream-soft (REJECTED)": 3.49,
    "white on coral #E1614A (REJECTED fill)": 3.49,
    "white on coral-deep (the fill)": 5.12,
    "white on coral-deepest (hover)": 6.68,
    "coral as body text on white (REJECTED)": 3.49,
    "coral as LARGE text on white": 3.49,
    "coral as a border on white": 3.49,
    "coral-deep as text on white": 5.12,
    "coral-deep as text on cream-soft": 4.70,
    "ink on coral-tint": 12.27,
    "olive-deep on olive-tint (ghost btn)": 7.16,
    "white on olive (ghost hover)": 4.86,
    "olive as text on white": 4.86,
    "olive-deep as text on white": 8.52,
    "cream-soft on olive (REJECTED bar)": 4.47,
    "cream-soft on olive-deep (the bar)": 7.83,
    "cream-soft on olive (card-media)": 4.47,
    "gold as text on white (REJECTED)": 2.15,
    "gold as LARGE text on white (REJECTED)": 2.15,
    "gold as a border on white (REJECTED)": 2.15,
    "ink on the gold fill": 6.96,
    "ink on the ribbon": 10.13,
    "paper-line over white (decorative)": 1.64,
    "border-strong on white": 3.49,
    "border-strong on cream-soft": 3.21,
    "THENIE focus #8FC4EC on white (REJECTED)": 1.86,
    "focus-ring #1F6FB2 on white": 5.28,
    "focus-ring #1F6FB2 on cream-soft": 4.86,
    "focus-ring-on-dark on ink": 8.05,
    "cream-soft on the dark ground": 13.78,
    "muted #CDC3B2 on the dark ground": 8.60,
    "coral on the dark ground (REJECTED link)": 4.29,
    "coral-light on the dark ground": 6.14,
    "cream .55 border on the dark ground": 5.23,
    "cream-soft on the dark scheme ground": 15.89,
    "ink-soft-dark on the dark scheme": 9.92,
    "ink-faint-dark on the dark scheme": 6.46,
    "coral-light on the dark scheme": 7.08,
    "gold as text on the dark scheme": 8.03,
    "olive on the dark scheme (REJECTED)": 3.56,
    "olive-light on the dark scheme": 8.56,
    "info rule on white": 7.13,
    "warn rule on white": 5.88,
    "danger rule on white": 8.28,
    "ink on the info tint": 12.17,
    "ink on the warn tint": 12.41,
    "WA teal on the white page": 4.14,
    "WA teal on the dark scheme": 4.18,
    "WA white glyph on the teal": 4.14,
    "cream ring on white (REJECTED)": 1.09,
    "hero scrim .88 — the copy band": 15.27,
    "hero scrim .62 — the wordmark band": 5.82,
    "hero scrim .56 — lightest band": 4.66,
    "THENIE scrim .15 (REJECTED)": 1.40,
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
