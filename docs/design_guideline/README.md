# Design guideline — source assets

Supplied by Steven, copied from `/home/aidev/asset/` on 2026-08-12. **Copied,
not moved** — the originals are still in place there.

| File | Was | Content |
|---|---|---|
| `logo.png` | `Logo/Logo.png` | "evermore" wordmark, 2000×525, transparent, `#1C3D34` ink |
| `color-palette.png` | `Color_Palette/Color Palette.png` | page 17 of the Mini Brand Guidelines |
| `font.png` | `Font/FONT.png` | page 19 — Erode (primary) and Inter (secondary) |

These are the **authority on intent**. `docs/10-design-system.md` is the
engineering reading of them — the same decisions as tokens, with the contrast
maths done and the failures called out.

**Read the design system before using a colour.** Four of the brand's colours do
not reach WCAG AA as text or as button fills, including the mid Nourish Green
and Energize Orange. That is a property of the palette, not a mistake to fix in
code, and it is documented so it is designed around rather than discovered in
review.

## Missing

The colour page refers to **page 13, "Logo on Color Palette"**, for recommended
combinations and applications. That page was not supplied and would settle
several of the open questions in `10-design-system.md` §5 — worth asking for.
