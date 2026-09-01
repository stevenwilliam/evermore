# Evermore — design reference

The working numbers. `docs/10-design-system.md` carries the reasoning and the
brand-guideline reading; this is the sheet to have open while building, so
nothing here is a judgement call — every ratio below was measured with
`scripts/contrast.py` and can be re-measured.

**This file is per project.** The `impeccable` skill beside it is portable; a
brand is not. A new project gets its own `design.md` in the same place.

---

## 1. Type

| Role | Family | Notes |
| --- | --- | --- |
| Display | **Erode** | titles, headlines, the wordmark's character |
| Body / UI | **Inter** (variable) | everything a customer reads |
| CJK | platform fonts | PingFang SC · Hiragino Sans GB · Microsoft YaHei · Noto Sans CJK SC · Noto Sans SC · Source Han Sans SC · WenQuanYi Micro Hei |

Both are **self-hosted** in `web/public/fonts/`. Never a font CDN — it hands a
third party every visitor's IP and the page they are on.

- **Erode ships four discrete cuts: 400 / 500 / 600 / 700.** Ask for anything
  else and the browser synthesises a fake bold, which smears the letterforms
  the wordmark is built from. Display weights land on 600 or 700 exactly.
- **Erode must not be subset.** The Fontshare licence forbids modifying the
  font software; the usual latin-subsetting trim is a modification.
- **Neither brand face covers CJK.** A Chinese headline is not set in Erode and
  cannot be — it falls through to the platform stack. Chinese pages carry the
  brand's colour, layout and wordmark, not its display voice.

### Scale and weight tokens

| Token | Size | | Token | Weight |
| --- | --- | --- | --- | --- |
| `--text-xs` | 13px | | `--w-display` | 700 (Erode) |
| `--text-sm` | 15px | | `--w-display-sub` | 600 (Erode) |
| `--text-base` | 17px | | `--w-body` | 450 (Inter) |
| `--text-lg` | 20px | | `--w-strong` | 600 |
| `--text-xl` | 24px | | `--w-ui` | 600 |
| `--text-2xl` | 32px | | `--w-eyebrow` | 700 |
| `--text-3xl` | 44px | | | |
| `--text-4xl` | 64px | | | |
| `--text-bar` | **19px** | the bar floor — see §3 | | |

Body is 17px rather than 16 because Inter at 450 sits on a coloured ground and
needs the extra pixel to hold its weight.

---

## 2. Palette

| Token | Hex | Role |
| --- | --- | --- |
| `--nourish-deep` | `#1C3D34` | **the page ground**, the ink on light surfaces, the logo ink |
| `--nourish` | `#468973` | **the bars** — masthead and footer |
| `--beige` | `#FFFAE0` | ink on both greens; the sheet panels sit on |
| `--beige-deep` | `#CCBDAA` | muted ink **on the deep ground only** |
| `--surface` | `#FFFFFF` | cards |
| `--ink-muted` | `#4A5D56` | secondary text on the sheet |
| `--brown-deep` | `#613F37` | warning / neutral emphasis |
| `--blue-deep` | `#2E55A3` | informational |
| `--berry-deep` | `#91253D` | destructive |
| `--blue-light` | `#B6DAFA` | tint; takes deep ink |
| `--orange-light` | `#FFBC8F` | tint; takes deep ink |
| `--orange-deep` | `#E0782D` | **decoration only** |
| `--berry-light` | `#CC6883` | background only |
| `--brown-light` | `#A36E50` | background only |
| gold | `#D4AF37` | the corner ribbon |

---

## 3. Measured contrast — check here before choosing a colour

**On the deep ground `#1C3D34`** — comfortable at any size:

| Ink | Ratio | |
| --- | ---: | --- |
| `#FFFFFF` | 11.89 | AAA |
| `#FFFAE0` beige | 11.32 | AAA |
| `#B6DAFA` blue light | 8.15 | AAA — links on dark |
| `#FFBC8F` orange light | 7.27 | AAA — this is how the orange earns its place |
| `#CCBDAA` beige deep | 6.47 | AA — muted text |

**On the mid-green bars `#468973`** — nothing reaches AA at reading size:

| Ink | Ratio | |
| --- | ---: | --- |
| `#FFFFFF` | 4.13 | **large text only** |
| `#FFFAE0` beige | 3.93 | **large text only** |
| `#1C3D34` deep ink | 2.88 | ✗ never |
| `#CCBDAA` | 2.25 | ✗ never — it is a deep-ground token |

> **The bar rule.** Every string on a bar is `--text-bar` (19px) at weight 700 —
> the first whole pixel above WCAG's 18.66px "large text" threshold. A 15px nav
> label on the bar looks fine and is an AA failure. This is why the header type
> is chunkier than it looks like it needs to be.

**On the beige sheet `#FFFAE0`** and on white cards:

| Ink | on beige | on white |
| --- | ---: | ---: |
| `#1C3D34` | 11.32 | 11.89 |
| `#613F37` brown deep | 8.79 | — |
| `#91253D` berry deep | 7.89 | — |
| `#2E55A3` blue deep | 6.79 | — |
| `#4A5D56` ink muted | 6.69 | — |
| `#E0782D` orange deep | **2.90 ✗** | — |

**Non-text contrast — WCAG 1.4.11, the 3:1 boundary.** A border that has to be
*found* is a control boundary and must clear 3:1; a border that merely separates
two filled surfaces need not. Confusing the two is how an input ends up with an
edge nobody can see.

| Token | over the sheet | |
| --- | ---: | --- |
| `--border` `rgba(28,61,52,.60)` → `#778979` | 3.55 | ✓ the real control boundary |
| `--border-subtle` `rgba(28,61,52,.28)` → `#BFC5B0` | 1.69 | ✗ **decorative only** — never a control edge or a focus ring |
| `--beige-deep` `#CCBDAA` | 1.75 | ✗ **never a border here** — it is a deep-ground token (6.47 there) |

**The corner ribbon.** Deep ink on the base gold `#D4AF37` is **5.65**, and the
animated shimmer only ever lifts it — `#FFF3C4` is **10.68** and `#FFFDF0` is
**11.64**. The base gold is therefore the worst case and the one that governs;
the sweep can never make the ribbon less readable than it is at rest.

**The WhatsApp float.** Its `#128C7E` fill measures **2.87** against the deep
ground — that is deliberate and not a defect, because the affordance is the
**beige ring**, which measures **11.32** against the ground and **3.94** against
the teal. Both clear 1.4.11. The white glyph on the teal is **4.14**: an icon,
so 3:1 applies, not 4.5. Do not "fix" the fill; it is WhatsApp's own colour and
recolouring it stops the mark reading as WhatsApp.

**Other pairings in use:** deep ink on blue-light **8.15** and on orange-light
**7.27**.

---

## 4. Rules that are not taste

- **Two grounds.** The page is deep green and body copy sits straight on it.
  The bars are mid green and everything on them is large text.
- **The CTA inverts by surface.** On the ground: beige fill, deep ink (11.32).
  On a beige panel: deep-green fill, beige ink (11.32). The inversion dates
  from when the ground was the MID green, where a deep-green fill measured 2.88
  and failed 1.4.11 as a control you must be able to find. On today's deep
  ground a deep-green fill would be 1.00 — the same colour — so the rule stands
  for a stronger reason than the one it was written for.
- **Focus rings**: deep ink normally, **beige on the ground** — deep ink there
  is an indicator you cannot see.
- **Colour is never the only signal.** The current nav item carries an
  underline *and* `aria-current`; the current language carries a tick *and*
  `aria-checked`.
- **AA is the floor, and contrast is calculated.** State the ratio next to the
  token. `python3 scripts/contrast.py '#RRGGBB' '#RRGGBB'`.
- **44px minimum touch target**; motion is one subtle tier — 200–400ms,
  ease-out, ≤12px travel, transform and opacity only — and
  `prefers-reduced-motion` zeroes delays as well as durations. The **home page
  alone** runs an expressive tier: 400–560ms, 28px, parallax and hover lift
  (`docs/10` §2.9).
- **Scroll-driven motion is measured, not guessed.** `entry` percentages play
  at the viewport's bottom edge where nobody is looking; sweep the range and
  record the numbers. And three one-word traps, each of which silently does
  nothing: `overflow: hidden` makes a scroll container and freezes a `view()`
  timeline (use `clip`); a scroll animation with no `animation-duration` has no
  progress to map; the `transform` shorthand lets an animation's fill state
  erase a hover, where `translate`/`scale` compose.
- **Motion is never the reason content is visible.** Paint the base state and
  add the animation on top; never `opacity: 0` in the base rule revealed by a
  keyframe. A stylesheet that arrives stale then hides the page instead of
  merely failing to animate it. This binds twice as hard for a scroll-driven
  timeline, which only advances while it is active: a `view()` reveal may move
  content, never fade it in from zero. See `docs/10-design-system.md` §2.9.
- Mobile-first, designed at 360px.

---

## 5. Brand assets

- **Wordmark**: lowercase "evermore" with a **reversed final `e`** — the mark's
  one distinctive move. It is not a typo and must never be corrected. Both
  `r`s are normal orientation. Never recolour, restretch or redraw it.
  Generated in both cuts by `scripts/mklogo.py` from
  `docs/design_guideline/logo.png`.
- **The masthead lockup** (2026-08-27) is the square **mark** then the wordmark:
  26px mark + 18px type on phones, 30px + 21px above 48rem. The type is **30%
  shorter** than the 26px/30px it used to carry. Shrinking it also cleared two
  things: the ribbon used to cover the reversed final `e` by 46.7px at 390px,
  and the page no longer scrolls horizontally (390/390, was 400/390).
- **The mark** is the wordmark's own leading `e`, masked and re-inked by
  `scripts/mklogo.py` — the identical operation the favicon has always used.
  **It is not a redraw**, which §5 forbids. On the mid-green bar it is a
  **beige field with deep ink**: beige on `#468973` is 3.93 and clears 1.4.11's
  3:1 for the badge's edge, deep ink on the beige field is 11.32. The other way
  round measures 2.88 and the badge would have no findable edge.
- **Favicon** is the same `e` the other way up — deep field, beige ink — on the
  masthead fill, because a tab strip may be light or dark. The full lockup is
  560×60 (**9.33:1**; `mklogo.py` draws the glyph run at 104:11 and pads to the
  round canvas) and turns to mush at 16px. The templates declare the intrinsic
  `width`/`height` of every logo file exactly, which is what stops the header
  shifting as the PNGs arrive.
- **Background** (2026-08-27): two drifting layers of **produce linework** —
  citrus, leaves, berries, wheat, avocado, broccoli, carrot — generated
  deterministically by `scripts/mkfoodpattern.py`. Far layer 340px at 0.06,
  near layer 220px at 0.05, drifting 150s and 95s in different directions,
  stopped by `prefers-reduced-motion`. **The binding token is `--beige-deep`,
  not beige**: at the combined peak the lightest rendered pixel is `#335044`,
  where beige is 8.43, white 8.85 and beige-deep **4.82** against AA's 4.50.
  Raise the coverage — heavier strokes, larger glyphs — never the opacity; the
  per-pixel peak is what costs contrast.
  This replaced Steven's tile (`scripts/mkpattern`, 36% of native, peak 14.9%).
  That tile left beige at 7.29, which was measured and correct, but nobody had
  checked the muted token: it was at **4.16, below AA**, wherever the linework
  crossed muted text.
- **Certification badges** are the issuer's own files when supplied — never
  redrawn, never sourced from image search. Defaults are Evermore's own seals
  from `scripts/mkseals.py`.
