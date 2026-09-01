# Design System — Evermore

**Version:** 0.3 (2026-08-31 — §4, the component layer, read off the mockup canvas)
**Source:** `docs/design_guideline/` — "evermore Mini Brand Guidelines", pages
17 (colour) and 19 (typeface), supplied by Steven — and, for §4, the Claude
Design project *"Healthy catering UI mockups"* (`Evermore Mockups.dc.html`),
supplied by Steven 2026-08-31.

This document is the **engineering reading** of those guidelines: the same
decisions expressed as tokens, with the contrast maths done. The PNGs are the
authority on intent; this file is the authority on what ships.

> **Carried into `evermore` on 2026-09-01. Read §4 with this in mind.**
>
> §1–§3 are the **brand**: the wordmark, the palette with every ratio measured,
> the typeface pairing. Evermore is Evermore, so those apply here whole and
> unaltered.
>
> §4 is the **component layer**, and it was read off a design canvas drawn for
> a different product's screens (`healthy_catering`, whose documents are now
> Evermore's baseline in `docs/`).
> Two halves, and they travel differently:
>
> - **The rules are portable and hard-won.** The AA-by-size policy (§4.1), the
>   `text-bar` collision that painted text at 1.00:1 for weeks (§4.1b), the
>   button, pill, chip, callout and table specs, the token scales. Keep all of
>   it.
> - **The artboard inventory is not.** §4 opens with a table of fourteen
>   artboards — menu calendars, packing labels, kitchen coverage maps — and
>   §4.10–§4.12 describe those screens and the departures made building them.
>   None of it exists here. Ignore the inventory until Evermore has screens of
>   its own, then rewrite those sections against them.
>
> The contrast tool that produced every ratio in §2 is
> `scripts/contrast.py` in the reference project. Carry it over before
> changing a colour.

---

## 1. Brand

The wordmark is **"evermore"**, set lowercase in a single weight of deep green,
with a **reversed final `e`** — the mark's one distinctive move. It is not a
typo and must never be "corrected".

- Asset: `docs/design_guideline/logo.png` — **7582×1989**, RGBA, transparent
- Ink: **Nourish Green deep, `#1C3D34`** — sampled from the file as
  `rgba(28, 61, 52, 255)`, the same value as the primary token
- Verified by decoding the PNG glyph by glyph: eight letterforms, and **only the
  final `e` is mirrored**. Both `r`s are in normal orientation — at small sizes
  the second `r` reads as mirrored next to the flipped `e`, and it is not. Do
  not "fix" either one.
- Never recolour it to a third colour, never restretch it, never redraw the
  letterforms.
- **On a dark or Nourish-Green fill the mark disappears**, so a reversed-out
  version is required. It now exists: `scripts/mklogo.py` derives it from the
  supplied artwork — see §3.2.

## 2. Palette

The guidelines structure colour in three levels and say application across them
is "intentionally dynamic and flexible". That flexibility is a design licence,
not an accessibility one — see the ratios below.

### 2.1 Primary — Nourish Green

| Token | Hex | Note |
|---|---|---|
| `--nourish-deep` | `#1C3D34` | the logo ink; **the page ground** since 2026-08-18 — see §2.7 |
| `--nourish` | `#468973` | **the bars** — masthead and footer — see §2.7 |

### 2.2 Secondary — Restore Beige

| Token | Hex | Note |
|---|---|---|
| `--beige` | `#FFFAE0` | primary ink on both greens, and the **sheet** for panels and the app |
| `--beige-deep` | `#CCBDAA` | borders and muted text **on the deep ground only** — 2.25 on a bar |

### 2.3 Tertiary

| Name | Dark | Light |
|---|---|---|
| Ground Brown | `#613F37` | `#A36E50` |
| Hydrate Blue | `#2E55A3` | `#B6DAFA` |
| Energize Orange | `#E0782D` | `#FFBC8F` |
| Revive Berry | `#91253D` | `#CC6883` |

### 2.4 Contrast — read this before using a colour

Calculated, not eyeballed. **AA body text needs 4.5:1.**

| Colour | on `#FFFAE0` | on `#FFFFFF` | White text on it |
|---|---:|---:|---:|
| `#1C3D34` Nourish deep | **11.32** | **11.89** | **11.89** |
| `#613F37` Ground Brown | **8.79** | **9.23** | **9.23** |
| `#91253D` Revive Berry | **7.89** | **8.28** | **8.28** |
| `#2E55A3` Hydrate Blue | **6.79** | **7.13** | **7.13** |
| `#468973` Nourish mid | ✗ 3.93 | ✗ 4.13 | ✗ 4.13 |
| `#A36E50` Brown light | ✗ 4.08 | ✗ 4.28 | — |
| `#CC6883` Berry light | ✗ 3.40 | ✗ 3.57 | — |
| `#E0782D` Energize Orange | ✗ 2.90 | ✗ 3.05 | ✗ 3.05 |

**Consequences, decided now so they are not rediscovered later:**

1. **`#468973` is not a text colour and not a button fill.** It fails in both
   directions — 4.13 as ink on white, and 4.13 for white ink on it. Use
   `#1C3D34` for green text and green fills. Reserve `#468973` for large
   decorative areas, illustration and non-informational shapes.
2. **`#E0782D` fails badly (2.90–3.05).** It is a highlight colour, not a
   label, a link, a price or a button. Dark ink on the orange was checked
   rather than assumed: `#1C3D34` on `#E0782D` is **3.90** — enough for large
   text (≥24px, or ≥19px bold) and for non-text UI under 1.4.11, **not** enough
   for body text or a small button label. `#000000` on `#E0782D` reaches
   **6.89**. So: an orange button is allowed only with near-black ink at a large
   size, and never with white ink.
3. **The light tertiaries are backgrounds, not inks.** Dark ink on them is fine;
   they are never the foreground. Measured with `#1C3D34` ink: Blue light
   **8.15** ✓, Orange light **7.27** ✓, Berry light ✗ 3.33, Brown light ✗ 2.78.
   Only the blue and orange tints take body text.
4. Colour is **never the only signal**. A state carries an icon or a word too.

### 2.5 On a Nourish-Green surface

The primary green is going to be the header, the footer and every dark panel,
so the inks that survive on it are decided here rather than per component.
Ink on `#1C3D34`:

| Ink | Ratio | Verdict |
|---|---:|---|
| `#FFFAE0` Beige | **11.32** | ✓ primary text on dark |
| `#B6DAFA` Blue light | **8.15** | ✓ links / informational on dark |
| `#FFBC8F` Orange light | **7.27** | ✓ **this is how Energize Orange earns its place** — as its tint on the dark green, not as the dark orange on light |
| `#CCBDAA` Beige deep | **6.47** | ✓ secondary/muted text on dark |
| `#CC6883` Berry light | ✗ 3.33 | large text only |
| `#468973` Nourish mid | ✗ 2.88 | never text on dark green |

### 2.6 Borders and non-text contrast (1.4.11 — 3:1)

`--beige-deep` `#CCBDAA` on the beige canvas is **1.75**. That is invisible to a
low-vision user, so **`#CCBDAA` is not an input border, a focus ring or any
boundary that carries meaning** — it is a divider between filled surfaces only.
Form field outlines, focus rings and control boundaries use `#1C3D34`, solid or
at an opacity that still measures ≥3:1. `#468973` on beige is 3.93, so it is a
legal *non-text* boundary even though it is not a legal text colour.

The four passing colours give a full semantic set without inventing anything:
Berry for destructive, Blue for informational, Brown for warning-ish/neutral
emphasis, Nourish deep for primary.

### 2.7 The two grounds (Steven, 2026-08-18)

The page ground moved from Restore Beige to Nourish Green `#468973`, and then
the two greens were **swapped**: the PAGE is deep `#1C3D34` and the BARS — the
masthead and the fixed footer — are mid `#468973`.

That swap matters because `#468973` sits at relative luminance 0.204, almost
exactly midway between black and white, and **nothing in the brand reaches AA
4.5 on it**:

| On mid green `#468973` | Ratio | Verdict |
|---|---|---|
| white `#FFFFFF` | **4.13** | AA large text / UI boundary only |
| beige `#FFFAE0` | **3.93** | AA large text / UI boundary only |
| deep ink `#1C3D34` | **2.88** | fails everything |
| `#CCBDAA` | **2.25** | fails everything — never use it on a bar |
| black `#000000` | 5.09 | passes, but is not a brand colour |

On the deep ground everything is comfortable:

| On deep green `#1C3D34` | Ratio | Verdict |
|---|---|---|
| beige `#FFFAE0` | **11.32** | AAA |
| white `#FFFFFF` | **11.89** | AAA |

So the rules are:

> **The page is deep green and body copy sits straight on it in beige.** Panels
> and cards are design decisions here, not accessibility crutches.
>
> **The bars are mid green and everything on them is LARGE text** — `--text-bar`
> is 19px, the first whole pixel above WCAG's 18.66px threshold, at weight 700.
> A 15px nav link on a bar looks fine and is an AA failure.

Consequences worth knowing before editing anything:

| Element | Rule | Why |
|---|---|---|
| masthead nav, footer, language selector | `--text-bar` @700 | 3.93 is AA for large text only |
| `--on-dark-muted` `#CCBDAA` | never on a bar | 2.25 there; it is a deep-ground token |
| the CTA | beige fill, deep ink | 11.32 on the page ✓ |
| focus rings on the page | beige | 11.32 ✓ |
| the WhatsApp float | keeps its beige ring | see below |

**The WhatsApp float has needed its ring under both grounds, for two different
reasons.** On the mid-green ground `#128C7E` measured **1.00:1** — the two were
within 0.0003 of the same luminance, so the button was a shape findable only by
hue. On the deep ground it measures **2.87**, still under the 3:1 that 1.4.11
requires of a control. The permanent 3px beige ring bounds it either way: 11.32
against the page and 3.94 against the teal. Its focus state is **white**, not
beige, because the resting ring is already beige and an outer deep-green ring
would be invisible against the page — focus and resting would look identical.
The ring is an accessibility mechanism, not styling. Do not remove it.

### 2.8 The background — drifting produce (Steven, 2026-08-27)

**Supersedes the tiled motif of 2026-08-19.** That tile was Steven's supplied
artwork, extracted to beige-on-transparent by `scripts/mkpattern` and painted
at 36% of native, peaking at 14.9% alpha. It is retired here; the file and the
generator stay, because **the SPA (`web/src/index.css`) still uses it** and
restyling the app was not part of this change.

**Why it was replaced, beyond the brief.** Its 14.9% peak left beige text at
7.29 — measured, recorded, and correct. But the muted token was never checked
at the same point: `--beige-deep` `#CCBDAA` on that composite is **4.16, below
AA's 4.50**, everywhere the linework crossed muted text. The comment in
`public.css` claiming "6.47 on the ground ✓" was true of the bare ground and
false of the page as it actually rendered.

**What replaced it.** Two fixed layers of produce linework — citrus, leaves,
berries, wheat, avocado, broccoli, carrot — generated by
`scripts/mkfoodpattern.py` from a fixed seed, so re-running produces
byte-identical files and a diff means somebody changed the drawing.

| Layer | Tile | Opacity | Drift |
| --- | --- | --- | --- |
| `body::before` far | 340px | 0.06 | 150s, down-right |
| `body::after` near | 220px | 0.05 | 95s, down-left |

The ground moved from `body` to `html`, because a body background propagates to
the canvas and would paint over anything at `z-index: -1`.

**The alpha is a contrast decision.** Public-page text sits directly on the
ground, so the binding token is `--beige-deep`, not beige:

| Ink | arithmetic bound | lightest pixel rendered | |
| --- | ---: | ---: | --- |
| beige `#FFFAE0` | 8.29 | 8.43 | AAA |
| white `#FFFFFF` | 8.71 | 8.85 | AAA |
| `--beige-deep` `#CCBDAA` | 4.74 | **4.82** | AA — binds |

"Rendered" is the lightest pixel found in a 400×400 capture of the background
with all content hidden (`#335044`). Bound and measurement agree to a
hundredth, and that agreement is the point. An earlier pass at 0.09/0.07
measured 4.89 and looked safe — but that was one paused frame in which no two
strokes crossed. **The bound governs**, because over a 150-second drift across
a whole viewport the crossing does happen.

**Visibility is bought with coverage, not opacity.** The per-pixel peak is what
costs contrast; heavier strokes and larger glyphs put more drawing on screen
without moving the peak. That is why the generator draws at 2.6/2.0 stroke
weight rather than hairlines.

Motion is transform-only on a fixed layer, oversized by exactly one tile and
translating by exactly one tile so the loop is seamless.
`background-position` is deliberately **not** animated — it repaints a
full-viewport image every frame, on a page mostly read on mid-range Android.
`prefers-reduced-motion` stops both layers, leaving the drawing as a static
texture.

## 3. Typography

Two faces, both **self-hosted** — never a font CDN, which leaks every visitor's
IP and page to a third party.

| Role | Family | Use |
|---|---|---|
| Display | **Erode** | titles, headlines, subtitles — "prominent typographic elements" |
| Body / UI | **Inter** | body copy, longer text, smaller-scale applications |

Weights shown in the guidelines: Regular, Medium, Semi Bold, Bold — for both.

**Erode is not on Google Fonts.** It is an Indian Type Foundry face distributed
through Fontshare; download the web package and check its licence terms before
first use. Inter is SIL OFL and can come from the Google Fonts static files, as
in ruuma. Both go in `web/public/fonts/` with their licence beside them.

Serif display against a sans body is the same structure ruuma landed on, and the
reason holds here: the wordmark has character, and setting headings in the body
sans would leave nothing on screen echoing it.

### 3.1 How the two are mixed (reworked 2026-08-18)

The pairing works by **contrast, not blend**: Erode heavy and tight, Inter open
and even, and one small uppercase Inter label bridging them. Erode ships four
discrete cuts, so a display weight must land on 600 or 700 exactly — ask for
650 and the browser synthesises a fake bold, which smears the letterforms the
wordmark is built from.

| Role | Face | Size | Weight | Tracking |
|---|---|---|---|---|
| `h1` | Erode | 44px → 64px ≥768px | 700 | −0.022em |
| `h2` | Erode | 32px | 700 | −0.015em |
| `h3`, card titles | Erode | 24px | 600 | −0.01em |
| `.lede` | Inter | 20px | 600 | — |
| body | Inter | 17px | 450 | — |
| `.eyebrow`, `.when` | Inter | 13px | 700 | +0.1em, uppercase |
| nav, buttons, labels | Inter | 15–17px | 600–700 | — |

Sizes and weights went up a step across the board on Steven's instruction
("more bold and bigger in overall design"). Body is 17px rather than 16 because
Inter at 450 sits on a coloured ground here and needs the extra pixel to hold
its weight. Two of these numbers are **load-bearing for accessibility**, not
taste: the `.lede` at 20px/600 and the `h2` at 32px are what make beige-on-green
legal as large text (§2.7). Reducing either size or weight silently drops that
text below AA.

### 3.2 The wordmark

The supplied artwork is `docs/design_guideline/logo.png` — 7582×1989, flat
`#1C3D34` lettering. `scripts/mklogo.py` reduces it to a coverage mask and
re-emits it in any colour, which is what produces both cuts from one source:

- `web/public/images/evermore-wordmark-light.png` — beige, for the deep-green
  masthead. **This answers §5's reversed-out logo question**: derived, not
  supplied.
- `web/public/images/evermore-wordmark-deep.png` — for light surfaces.
- `web/public/images/og-default.png` — 1200×630 share card.

One trap is recorded in the script: the artwork's ink is `#1C3D34`, whose luma
is 50, so naive darkness-as-coverage caps at 205 and emits a solid stroke at 80%
alpha. The wordmark would then render as a *tint* of whatever sits behind it,
quietly losing the 11.32:1 this document claims for it. Coverage is normalised
against the darkest pixel present so a solid stroke is genuinely solid.

### 2.9 Home motion — the expressive tier (2026-08-27)

The home page ran the subtle tier first and Steven could not see it, which was
the correct outcome of a 12px/400ms rule. **On the home page only**, travel is
now 28px and durations 400–560ms. Every other page keeps the subtle tier in §4.

Still **no JavaScript**. The CSP would permit a same-origin script; nothing here
needs one, and the home page is the LCP path.

| What | Motion | Driven by |
| --- | --- | --- |
| `.hero-copy` children | rise 28px + fade, 90ms apart | time, 460ms |
| `.hero-art` (the frame) | fade | time, 560ms |
| `.hero-art img` | parallax pan ±3.2% inside a 1.08 scale | scroll, `view()` |
| price table `tbody tr` | rise 28px, no fade | scroll, `cover 25%..70%` |
| `.check .panel`, `.diets .card-diet` | rise 28px, no fade | scroll, `cover 25%..70%` |
| section-head rule | `scaleX` wipe from the left | scroll, `cover 20%..60%` |
| `.diets .card-diet` hover/focus-within | `scale(1.02)` + deeper shadow | interaction |

**Not animated, deliberately: the price figures.** A count-up renders a money
value that is briefly wrong and starts at zero, and this project has already
shipped `Rp 0` against real prices once. The table reveals by row instead.

Three things here are one word away from silently not working, and each was
found by measurement rather than by looking:

- **`overflow: clip`, not `hidden`, on `.hero-art`.** Both crop the oversized
  image identically, but `hidden` makes the element a scroll container, and
  `view()` resolves against the nearest scrollport. With `hidden` the image's
  timeline was measured against a frame that never scrolls: pinned at
  `currentTime` 49.9967%, parallax frozen at every scroll offset.
- **A scroll-driven animation needs a non-zero `animation-duration`.**
  `animation: hero-pan linear both` leaves the shorthand's duration at its
  initial `0s`, and there is then no progress for a range to map onto.
- **Individual `translate`/`scale`, not the `transform` shorthand.** A card
  carries both a scroll reveal and a hover lift; written as `transform` the
  animation's fill state wins permanently and the hover does nothing at all.

**Ranges are swept, never guessed.** `entry` percentages play entirely at the
viewport's bottom edge. Measured at 390×844, card top in px as the motion runs:
`cover 5%..35%` → 785→616 (invisible), `cover 25%..70%` → 587→336 (the middle
of the screen). Re-sweep after any change.

**Motion is never the reason content is visible.** Base state painted,
animation additive, all of it inside `prefers-reduced-motion: no-preference`.
Anything on a *scroll* timeline moves but never fades: a scroll timeline only
advances while active, so a faded element would be stranded invisible if the
range were mis-scoped or the container did not scroll. Time-based animations
always finish, so those may fade. Measured 2026-08-27 at 390px: under
`prefers-reduced-motion: reduce` the first viewport differs from the
pre-animation build by **113 pixels of 329,160, max channel delta 2** — the
ribbon shimmer at a different phase, nothing else.

The section-head rule is a **static** design element declared outside the motion
block, so with motion off it is simply present; only its wipe is animated.

`scripts/verify-motion.js` asserts the invariant on every element.

## 4. Component layer — the mockup canvas (2026-08-31)

**Source:** the Claude Design project *"Healthy catering UI mockups"*,
file `Evermore Mockups.dc.html`
(`claude.ai/design/p/e433f21e-1ed8-4854-9e59-4f11cb0046e4`). Fourteen artboards:

| | Artboards |
|---|---|
| Marketing | **M1** home · desktop 1440 · **M2** buy package · **M3** pick slot from credit |
| Print | **P1** kitchen production sheet · A4 · **P2** packing label 100×150 mm + 100×50 mm |
| Back office | **S1** daily dashboard · **S2** menu schedule calendar · **S3** four price forms · **S4** payment verification queue · **S5** kitchen coverage map |
| Mobile B2C | **01** menu calendar · **02** meal detail · **03** cart · **04** delivery · **05** transfer · **06** package credit |

The canvas takes its palette and type from §2 and §3 — it introduces **no new
colour and no new typeface**. What it adds is the component layer below, which
until now existed only as ad-hoc Tailwind in the SPA. That is what this section
makes normative.

### 4.1 The canvas is the specification. Type size is what makes it legal.

**Decision, Steven, 2026-08-31: the canvas wins on colour; AA is met by SIZE.**
Not one hex in the fourteen artboards is altered. Where a pairing measures
under AA at the size drawn, the *type* moves to WCAG's large-text threshold —
**19px at weight 700**, the first whole pixel above 18.66px — which lowers the
requirement from 4.5:1 to 3:1 and makes the canvas's own colour compliant as
drawn. This supersedes the substitution table that shipped earlier the same
day; `#CC6883` fills and `#468973` row fills are back exactly as the canvas
draws them.

Recomputed with `scripts/contrast.py`. Everything not listed here passes at the
size the canvas draws it.

| Element | Canvas | Measured | Need | Ships as |
|---|---|---:|---:|---|
| Capacity pill "40 / 40" (S1, S2, S5) | `#CC6883` fill, beige ink, 14px/700 | **3.40** | 4.5 | same fill, **19px/700** → need 3.0 ✓ |
| Selected queue row (S4) | `#468973` fill, beige ink, 15px | **3.93** | 4.5 | same fill, **19px/700** ✓ |
| Sticky bottom-bar total (01, 03, 04, M2, M3) | beige on bar, 17px/700 | **3.93** | 4.5 | **19px/700** ✓ |
| Sticky bottom-bar kicker | beige on bar, 12px/700 | **3.93** | 4.5 | **19px/700** ✓ |
| Sidebar nav, inactive (S1) | beige on bar, 15px/500 | **3.93** | 4.5 | **19px/700** ✓ |

**`#CCBDAA` can never sit on `#468973`.** It measures **2.25** — under the 3:1
floor that large text itself has to clear, so there is no size at which it is
legal. Two responses, and which one applies depends on whether the element can
move:

*Short labels locked inside the rail become beige at 19px/700.* These are the
only two places a colour changes:

| Element | Canvas | Measured | Ships as |
|---|---|---:|---|
| Sidebar "BACK OFFICE" kicker (S1) | `#CCBDAA` on the bar | **2.25** ✗ | `#FFFAE0`, 19px/700 |
| Sidebar staff sub-label (S1) | `#CCBDAA` on the bar | **2.25** ✗ | `#FFFAE0`, 19px/700 |

*Long supporting text moves off the bar instead.* S3's tax note and S5's
polygon rule are sentences, not labels; blown up to 19px beige they would
dominate the bar they are subordinate to. They keep the canvas's **exact
colour and exact size** and drop one row, onto the deep ground, where
`#CCBDAA` is **6.47** ✓. A 40px relocation is a smaller departure from the
artboard than a 36% type increase, and it is the reading of "exact colours,
legal sizes" that costs the design least.

**One border stays as drawn.** `1px solid #468973` on the ground is **2.88**,
under 1.4.11's 3:1. It is kept — because in every place the canvas uses it the
border is *not* what identifies the control: a chip carries its label and a
selected sibling, a slot option carries its own radio dot and text. 1.4.11
governs boundaries that are needed to identify a component, and these are not.
Where a border genuinely is the only affordance — a bare input — it is
`#CCBDAA` at **6.47**.

**What this costs, stated plainly.** The back-office rail is visibly heavier
than the artboard: ten nav items at 19px/700 rather than 15px/500. That is the
price of keeping `#468973` as the rail fill. The alternative — moving the rail
to the deep ground, where beige is 11.32 and 15px is legal — would keep the
canvas's type but change a fill, so it was not taken. Worth revisiting.

**The device status strip in 01–06 is not built.** "09.41 · evermore.co.id" at
13px on the bar measures 3.93 and fails, but it is the artboard's drawing of
the *phone's own* status bar, not product UI. It has no implementation to make
compliant.

### 4.1b The `text-bar` collision — a latent invisible-text bug

Found 2026-08-31 while building the stepper, and it predates the canvas work.

`theme.extend.colors` carried a key `bar` (`#468973`, the masthead fill) and
`theme.extend.fontSize` carried a key `bar` (19px, the size that makes beige
legal on that fill). Tailwind resolves `text-{key}` against **both** scales, so
one utility emitted two rules:

```css
.text-bar { font-size: 1.1875rem; line-height: 1.2 }
.text-bar { color: rgb(70 137 115) }        /* #468973 */
```

Any element using it for size alone was painted mid-green — **1.00:1 on the
mid-green bar**. Not low-contrast: absent. The quantity in the meal-detail
stepper rendered as an empty gap, and it took a computed-style probe to see
why, because the source reads `text-bar font-bold` and looks like a size.

The size key is now **`onbar`**; `text-onbar` emits font-size only.
`scripts/verify-tailwind-keys.js` fails if any key ever appears in both scales
again — it was checked against the real bug before being trusted.

The lesson generalises: a design token name that is meaningful in two scales is
a name that will eventually be used in the wrong one, and the failure is
silent. Nothing in review catches it, because the collision exists only in the
generated CSS.

### 4.2 Buttons — pills, and the CTA is beige

On the deep ground the primary action is a **beige fill with deep ink**
(11.32 both as fill-against-ground and as ink-on-fill), not the deep-green fill
the SPA used while it lived on a beige sheet. Every button is a full pill,
`border-radius: 999px`, weight 700, and clears 44px.

| Variant | Fill | Ink | Border | Height |
|---|---|---|---|---|
| `.btn-primary` | `#FFFAE0` | `#1C3D34` | none | 44 · 48 full-width · 52 hero |
| `.btn-ghost` | transparent | `#FFFAE0` | `2px #FFFAE0` | 44 |
| `.btn-danger` | transparent | `#FFFAE0` | `2px #CC6883` | 44 · 48 |
| `.btn-quiet` | transparent | `#CCBDAA` | `2px #468973` | 40 — inside a framed panel only |
| `.btn-icon` | `#FFFAE0` | `#1C3D34` | none | 44×44, round |

A 2px rule, not 1px: at 1px a beige outline on the ground reads as a hairline
and disappears against the produce background (§2.8).

### 4.3 Radii

The canvas is consistently rounder than the old token set. `--radius` moves
from 10px to 12px and three sizes join it.

| Token | px | Used for |
|---|---:|---|
| `--radius-sm` | 6 | focus ring inset, tiny chips |
| `--radius` | **12** | inputs, notes, small cards, icon buttons |
| `--radius-card` | **14** | calendar cells, stat tiles, inner panels |
| `--radius-lg` | 16 | framed panels and tables |
| `--radius-panel` | **20** | meal cards, feature cards |
| `--radius-xl` | 24 | hero art |
| `--radius-frame` | **34** | the mobile app frame |
| `--radius-full` | 999 | every button, every pill |

### 4.4 The kicker

The canvas's most-repeated element, above almost every heading and inside every
tile: **12–13px, weight 700, `letter-spacing: 0.08em–0.1em`, uppercase**, in
`#CCBDAA` (6.47 ✓) — or `#FFBC8F` (7.27) / `#B6DAFA` (8.15) when it carries a
state. It is `.kicker`; it is never a heading element, because it is a label,
not an outline level.

### 4.5 Callouts — the left rule

One pattern, three voices. `border-left: 3px solid <accent>`, background the
same accent at **8%** over the ground, `--radius`, `12px 14px` padding. Body
text stays beige and measures 9.29–10.57 on every one of them.

| Class | Rule | Carries |
|---|---|---|
| `.note-info` | `#B6DAFA` | how a thing works — cut-off times, credit rules |
| `.note-emph` | `#FFBC8F` | a deadline or a caution — "pay within 2h 47m" |
| `.note-warn` | `#CC6883` | something is wrong or blocked |

Colour is never the only signal (§2.4 rule 4): each carries a word or an icon.

### 4.6 Framed tables

Data grids are CSS grid inside a frame, not `<table>` borders:
`1px solid #CCBDAA`, `--radius-lg`, `overflow: hidden`. The header row is
`.kicker` in `#CCBDAA` over a `#CCBDAA` bottom rule; body rows are separated by
`1px solid #468973`. The selected row is a `#468973` **fill** exactly as the
canvas draws it, with its text at 19px/700 so beige on it clears AA as large
text (§4.1).

**On the print sheets the frame inverts:** header cells are `#1C3D34` filled
with beige ink, and row rules drop to `rgba(28,61,52,0.2)` on the beige sheet.

### 4.7 Stat tiles

`1px solid #468973`, `--radius-card`, 16px padding. Label `.kicker` at 13px in
`#CCBDAA`; the number in Erode **40px/700** at `-0.022em`; a 13px sub-line in
`#CCBDAA`. An alerting tile swaps border **and** label to `#FFBC8F` or
`#CC6883` together — the border alone would be colour-as-only-signal.

### 4.8 Status pills

`padding: 4px 12px`, `--radius-full`, 13px/700.

| State | Ships as | Measured | Size |
|---|---|---:|---|
| affirmative — "Cocok", "Ya", "Published" | `#FFFAE0` fill, deep ink | 11.32 ✓ | 13px |
| emphasis — "Paling laris", a queue count | `#FFBC8F` fill, deep ink | 7.27 ✓ | 13px |
| informational | `#B6DAFA` fill, deep ink | 8.15 ✓ | 13px |
| **at capacity / rejected** | `#CC6883` fill, beige ink | 3.40 | **19px/700** (§4.1) |
| archived / inactive | transparent, `1px #CCBDAA`, `#CCBDAA` ink | 6.47 ✓ | 13px |

The capacity pill is the canvas's colour untouched. It is the one pill that
carries large type, and that is not a style choice — at 13px it would be an AA
failure, and the fill is what the artboards draw.

### 4.9 Chips — the segmented filter

Diet filters, price-table tabs, queue tabs. Selected: `#FFFAE0` fill, deep ink,
weight 700. Unselected: transparent, `1px #CCBDAA` (§4.1 #3 — it is the chip's
only edge), weight 500. Both are pills clearing 44px.

### 4.10 The two shells

**Mobile — 390×844 (01–06, M2, M3).** A `#468973` status strip, then a
`#468973` header bar carrying a 44px round beige back button and a 19px/700
title, then the scrolling body on the ground, then a **sticky bottom bar** in
`#468973`: the running total on the left as a `.kicker` over Erode 24–26px/700,
and the primary action on the right. Every string on either bar is `--text-bar`
(19px/700) or larger — §2.7's rule, and the reason the bottom bar's total is
Erode rather than Inter.

**Back office — 1440×900 (S1–S5).** A 228px `#468973` sidebar: wordmark, a
"Back office" kicker, nav items at 15px, the active one a `#FFFAE0` fill with
deep ink at `--radius` and a 12px side inset, a `#FFBC8F` count badge where one
applies, and the signed-in staff member pinned to the bottom. S2–S5 also show a
top-bar variant — wordmark at 116px plus a 19px/700 screen title — used where a
screen needs its full width.

Sidebar nav items are **19px/700**, not the artboard's 15px/500: beige on
`#468973` is 3.93, which is AA for large text and nothing else, and the rail
fill is the canvas's (§4.1). The rail is therefore visibly heavier than the
artboard — the one place where "legal sizes" is obvious to the eye. The
"BACK OFFICE" kicker and the staff sub-label are `#FFFAE0` rather than the
canvas's `#CCBDAA`, which measures 2.25 there and is illegal at every size.

### 4.11 Print artifacts

**P1 — kitchen production sheet, A4 (794×1123 at 96dpi).** Beige sheet, deep
ink, the inverted table frame of §4.6, a 4-up stat row where the first tile is
deep-filled, and a signature block pinned to the bottom.

**P2 — packing label, 100×150 mm (378×567) and a 100×50 mm compact variant.**
Beige sheet, deep ink, the deep wordmark, a diet pill, the recipient block in
Erode 32px, a two-up delivery/kitchen pair, contents with allergens in bold, and
the order code beside an 84px QR block.

Both print on the beige sheet, never on the ground: a deep-green flood fill is
about 90% ink coverage on a kitchen printer, and the pages exist to be printed
every morning.

### 4.12 Where the build departs from the canvas, and why

Beyond §4.1's size rule, these are the deliberate departures. Each is a
decision, not a shortfall — and none is invisible, so it is written down rather
than left for someone to "fix" back.

1. **The back-office rail is heavier than the artboard.** Ten items at
   19px/700 rather than 15px/500, because beige on `#468973` is 3.93 and the
   rail fill is the canvas's (§4.1). The alternative — moving the rail to the
   deep ground, where 15px beige is 11.32 — would keep the canvas's type at the
   cost of one fill. Worth revisiting; it was not taken because the fill is
   drawn.
2. **The rail is hidden below `lg`, not reflowed.** A 228px rail on a 390px
   phone leaves 162px of content. The staff artboards are drawn at 1440×900.
3. **`.btn-quiet` is 44px, not the canvas's 40px.** The touch floor in §5 is
   not negotiable against a style.
4. **S5's map is a schematic.** Real coordinates and radii, projected with one
   kilometre scale on both axes and drawn to scale, so the relative geography
   is truthful. No streets: the tile layer needs the browser Maps key handed to
   the SPA, which `RUN-WHEN-BACK.md` §B3 records.
5. **Two columns were renamed rather than filled with invented data.** S3's
   "Basis + pajak" column: `PriceRow` carries no base/tax split, because that
   division happens when an *order* is priced, not on a price row — which is
   what the note above the table already says. It reads "Promo". P1's second
   component column showed `item_role`, so it is headed "Role".
6. **The phone artboards render with no masthead and no fixed footer.** Each is
   a whole screen with its own bars. Stacked under the desktop masthead, the
   artboard sat below a dead green band and the fixed footer printed across its
   sticky total.
7. **The floating WhatsApp button is absent from the customer artboards and the
   back office.** The canvas draws it on neither, and on S1 it covered the
   needs-action list's own button. It remains on every public page.
8. **The artboards' 44px device status strip is not built.** "09.41 ·
   evermore.co.id" is the phone's own status bar, not product UI.
9. **Two of M1's five nav destinations have no page of their own.** "Untuk
   kantor" points at the contact page, where a corporate enquiry already lands;
   "Wilayah antar" points at the home page's coverage checker. What a dedicated
   corporate page should *say* is a business question, and migration 0031 sends
   it to `docs/03-open-questions.md` rather than inventing copy for it.
10. **M2's "Paling laris" badge is positional, not data.** It marks the middle
    bundle. There is no bestseller flag in the schema, and inventing one from
    sales figures would be a different claim than the artboard makes.

The M1 menu band renders nothing when no meal is published for the coming seven
days. That is its correct state: the home page has to render on a Thursday when
next week's menu is still in draft.
