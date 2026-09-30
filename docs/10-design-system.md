# Design System — Evermore

**Re-skinned 2026-09-30 (D32) to the Thenie palette.** Steven: *"forget all
design I mention before for evermore but follow as close as possible to thenie
project, my boss really like this thenie design — but keep the font type that
evermore has before."*

Two consequences, and they are the whole brief:

- **Colour, surfaces, radii and component shapes come from Thenie.** The source
  is `docs/design_reference/thenie_site_FINAL_14.html` — a 7,117,002-byte
  single-file capture, SHA-256 `e5227336e7c3feef8f80e4ab5f71f5bc121290beb4feb9cded10cca404134ad0`.
  The deep-green "Nourish" palette that this document used to describe is
  **retired**. Nothing from it survives.
- **Typography does not change.** Thenie is set in Baloo 2 throughout. Evermore
  stays on **Erode + Inter**, self-hosted. §3 is carried over intact.

Every ratio below is **computed by `scripts/contrast.py`**, not estimated.
CLAUDE.md §7: *contrast is calculated, not eyeballed.* Run
`scripts/contrast.py` to re-earn these numbers rather than trusting the table.

---

## 1. What was taken from Thenie, and what was corrected

Thenie's palette is read verbatim into §2. **Five of its own values fail WCAG
AA**, and a brief to copy a design is not a licence to copy an accessibility
defect — CLAUDE.md §7 sets AA as the floor. Each is corrected by the smallest
change that reaches AA, hue and saturation held, and each correction is
**checked in `contrast.py` beside the value it replaces** so the original is
recorded rather than quietly dropped.

| Thenie value | Measured | Why it fails | Evermore uses | Measured |
|---|---|---|---|---|
| `--maroon #E1614A` as a **button fill** under white text | **3.49** | under the 4.5 body floor — this is the primary action on every page | `--coral-deep #B84B39` | **5.12** |
| `--ink-faint #8B8271` | **3.80** white / **3.49** band | body text below 4.5 on both grounds | `#787061` (−6.8% lightness) | **4.90** / **4.50** |
| `--focus-ring #8FC4EC` on white | **1.86** | an invisible focus ring is a keyboard user with no cursor | `#1F6FB2` on light, `#8FC4EC` kept on dark | **5.28** / **8.05** |
| `--gold #D9A867` as text | **2.15** | fails AA even as large text, and fails 1.4.11 as a border | gold is **fill and rule only** on light grounds | ink on gold **6.96** |
| `--paper-line` at 0.45 alpha as a **control** edge | **1.64** | under 1.4.11's 3:1 | `--border-strong #968773` for controls; paper-line stays decorative | **3.49** |

`--coral #E1614A` itself is **kept exactly** — it is the colour the brand is
recognised by. It is simply not a body-text colour: legal as large text
(≥24px, or ≥19px bold) and as a non-text edge, and nothing smaller.

Two more traps found while measuring, recorded so they are not re-proposed:

- **`--olive #5F7A33` as the footer bar** puts cream-soft at **4.47** — four
  hundredths under the body floor. The bar is `--olive-deep #3E5321` (**7.83**).
- **`--coral` as a link on the dark ground** is **4.29**, also under the floor.
  Dark grounds take `--coral-light #F08A75` (**6.14**).

---

## 2. Palette

Implemented in `web/public/css/tokens.css`. Token names follow Thenie's own,
with `maroon` renamed `coral` because the colour is coral and the old name
misled every reader of the source file.

### 2.1 Surfaces

| Token | Value | Use |
|---|---|---|
| `--cream` | `#FFFFFF` | page ground |
| `--cream-soft` | `#FAF5E9` | alternating section band, table stripes |
| `--cream-raised` | `#FFFFFF` | cards, nav pill, modal |

The page ground is **white**. This is the single biggest change from the
retired design, which grounded every page in deep green.

### 2.2 Ink

| Token | Value | on white | on `--cream-soft` |
|---|---|---|---|
| `--ink` | `#2B2620` | **14.99** AAA | **13.78** AAA |
| `--ink-soft` | `#615848` | **7.01** AAA | **6.44** AA |
| `--ink-faint` | `#787061` | **4.90** AA | **4.50** AA |

`--ink-faint` is the corrected value. Thenie's `#8B8271` measures **3.80** and
**3.49** — recorded in `contrast.py` as `THENIE #8B8271 (REJECTED)`.

`--ink-faint` on the band is **exactly 4.50**. It has no margin: darken the
band or lighten the ink and it drops below AA silently.

### 2.3 Coral — the primary brand mark

| Token | Value | Role |
|---|---|---|
| `--coral` | `#E1614A` | eyebrows, accents, borders, **large text only** |
| `--coral-deep` | `#B84B39` | the filled button; body-size text on white |
| `--coral-deepest` | `#9C3E2E` | filled-button hover |
| `--coral-light` | `#F08A75` | links on a dark ground |
| `--coral-tint` | `#FCE3DE` | tinted panels — ink on it **12.27** |
| `--coral-tint-strong` | `#F6C7BC` | stronger tint |

| Pairing | Ratio | Verdict |
|---|---|---|
| white on `--coral` | **3.49** | **FAIL** — never the fill under white text |
| white on `--coral-deep` | **5.12** | AA — this is the button |
| white on `--coral-deepest` | **6.68** | AA — hover |
| `--coral` as body text on white | **3.49** | **FAIL** |
| `--coral` as **large** text on white | **3.49** | AA (large) |
| `--coral` as a border on white | **3.49** | PASS 1.4.11 |
| `--coral-deep` as text on white | **5.12** | AA |
| `--coral-deep` as text on the band | **4.70** | AA |
| `--coral` on the dark ground | **4.29** | **FAIL** — use `--coral-light` |
| `--coral-light` on the dark ground | **6.14** | AA |

**The eyebrow.** Thenie's `.eyebrow` is `0.72rem` uppercase in `--maroon`.
At 11.5px that is small text at **3.49** and illegal. Evermore's eyebrow is
`--coral-deep` (**5.12**) at `--text-xs` 13px / weight 700 / `0.1em` tracking.

### 2.4 Olive — the secondary

| Token | Value | Note |
|---|---|---|
| `--olive` | `#5F7A33` | white on it **4.86** AA |
| `--olive-deep` | `#3E5321` | **8.52** on white; the footer bar |
| `--olive-tint` | `#E7EEDA` | ghost-button ground |
| `--olive-tint-strong` | `#D6E3C2` | |

The ghost button is Thenie's `.btn-ghost-olive`: `--olive-deep` on
`--olive-tint`, **7.16** AAA, hovering to white on `--olive` (**4.86**).

**The bar rule.** The retired design forced every bar string to 19px/700
because nothing reached AA on the mid-green. The masthead is now `--ink`,
where cream-soft is **13.78**, and the rule is no longer load-bearing there.
It is kept, for two reasons: it is the established masthead look, and it still
**binds** on `.card-media`, whose lightest gradient stop is `--olive` — where
cream-soft is **4.47** and is large text only.

### 2.5 Gold — decoration only

| Token | Value |
|---|---|
| `--gold` | `#D9A867` |
| `--ribbon` | `#E4D3AA` |

| Pairing | Ratio | Verdict |
|---|---|---|
| gold as text on white | **2.15** | **FAIL** |
| gold as **large** text on white | **2.15** | **FAIL** — it does not even reach 3:1 |
| gold as a border on white | **2.15** | **FAIL 1.4.11** |
| `--ink` on the gold fill | **6.96** | AA |
| `--ink` on the ribbon | **10.13** | AAA |

On light grounds gold is a **fill or a rule that carries ink**, never ink
itself and never a load-bearing edge. In the dark scheme it becomes legal as
text (**8.03**) — the one colour that gains from the flip.

### 2.6 Lines and focus

| Token | Value | Ratio | Verdict |
|---|---|---|---|
| `--paper-line` | `rgba(150,135,115,0.45)` | **1.64** | decorative hairline **only** |
| `--border-strong` | `#968773` | **3.49** white / **3.21** band | PASS 1.4.11 — control edges |
| `--focus-ring` | `#1F6FB2` | **5.28** white / **4.86** band | PASS 1.4.11 |
| `--focus-ring-on-dark` | `#8FC4EC` | **8.05** on `--ink` | PASS 1.4.11 |

Thenie's `#8FC4EC` on white measures **1.86**. It is kept for dark grounds,
where it is excellent, and replaced on light ones.

### 2.7 The dark ground (`.ground-deep`)

Applied to the hero and any section that wants the inverse. The ground is
`--ink #2B2620`, and it is a **token remap**, not a background swap — the light
tokens are not merely different there, they are wrong in a way that hides text.

| Role | Value | Ratio |
|---|---|---|
| headings and body | `--cream-soft` | **13.78** |
| muted text | `#CDC3B2` | **8.60** |
| links | `--coral-light` | **6.14** (`--coral` is **4.29** and fails) |
| control borders | `rgba(250,245,233,0.55)` | **5.23** |
| focus ring | `--focus-ring-on-dark` | **8.05** |

Any element inside it that paints its own light surface — card, callout,
table, input — restores the light tokens through a single `:is()` rule. That
is one rule over a set, not a rule per element, because the per-element version
is what previously missed callouts and table headers.

### 2.8 Semantic rules and tints

| Role | Rule | on white | Tint | ink on tint |
|---|---|---|---|---|
| informational | `#2E55A3` | **7.13** | `#DCE9F7` | **12.17** |
| warning | `#8A5A26` | **5.88** | `#F7E8CE` | **12.41** |
| destructive | `#91253D` | **8.28** | — | — |
| success | `--olive-deep` | **8.52** | `--olive-tint` | **12.59** |

### 2.9 Dark scheme

Thenie ships a single light theme. CLAUDE.md §7 requires
`prefers-color-scheme` to be respected, so this is **derived, not supplied**:
the ink becomes the ground and the cream becomes the ink.

| Role | Value | on `#1E1A16` |
|---|---|---|
| ground / card | `#1E1A16` / `#2B2620` | — |
| body ink | `#FAF5E9` | **15.89** |
| soft ink | `#CDC3B2` | **9.92** |
| faint ink | `#A79D8C` | **6.46** |
| links | `--coral-light` | **7.08** |
| gold as text | `#D9A867` | **8.03** |
| olive | `#A3C16B` | **8.56** |

`--olive #5F7A33` does **not** survive the flip — **3.56** — which is why the
dark scheme substitutes the lighter olive rather than reusing the token.

### 2.10 The hero scrim (D34)

The homepage hero is a photograph filling the viewport with white text over it.
The photograph is **unknown** — it changes with the season and the menu — so
every scrim stop is set from the **worst case: the pixel behind it is pure
white**, and measured composited against white.

| Stop | Position | Composite | White text | Verdict |
|---|---|---|---|---|
| `0.88` | bottom — where the copy sits | `#272523` | **15.27** | AAA |
| `0.62` | top — where the wordmark sits | `#666564` | **5.82** | AA |
| `0.56` | the lightest middle band | `#908F8E` | **4.66** | AA |
| `0.15` | **Thenie's own middle stop** | — | **1.40** | **REJECTED** |

Thenie's middle stop is `0.15`, which composites to **1.40** against a white
photo. Nothing is positioned in that band today, so it is a *latent* trap
rather than a live defect — raised to `0.56` so it cannot become one when a
section is later moved up.

This is arithmetic on the worst case, not a sample of the rendered pixels.
Sampling the real composite needs a screenshot, and **screenshots were
unavailable when this was written** — recorded in `RUN-WHEN-BACK.md §Z2`.

### 2.11 The bars — one chrome (D39)

The masthead was `--ink` (near-black) while the footer was green. On a page
carrying both — every `/app/*` screen — that read as two unrelated chromes.
They are now **one colour**, and both read the same pair of tokens so they
cannot drift apart again:

```css
.masthead, .footer { background: var(--bar-bg); color: var(--bar-ink); }
```

| Token | Light | Dark |
|---|---|---|
| `--bar-bg` | `#55702E` | `#2B3A18` |
| `--bar-ink` | `--cream-soft` | `--cream-soft` |

| Pairing | Ratio | Verdict |
|---|---|---|
| cream-soft on the bar `#55702E` | **5.16** | AA |
| white on the bar `#55702E` | **5.61** | AA |
| cream-soft on the dark bar `#2B3A18` | **11.21** | AAA |
| muted `#CDC3B2` on the fine-print band `#3E5321` | **4.89** | AA |
| link tint on the fine-print band | **6.92** | AA |
| `--ink` on the bar | **2.67** | **REJECTED** |

**Why `#55702E` and not `--olive #5F7A33`.** The lighter olive is the obvious
choice and puts cream-soft at **4.47** — under the body floor, legal only for
as long as every string in a bar stays 19px/700. `#55702E` clears AA at
**body** size, so the bar is safe whatever size a string ends up.

**The 2.67 row is a defect this change introduced and then fixed.** `.footer`
read `--nourish` and `--beige`, which are **scheme-flipping aliases**: in dark
mode `--beige` resolves to `#2B2620`, so the footer painted dark ink on the
mid-green bar while the masthead went dark green — a contrast failure and the
two-chrome problem back again, in the other scheme. Reading `--bar-bg` and
`--bar-ink` makes the bars one thing by construction. It is the same class of
bug as `.hero-art` in D34: an alias that changes meaning with the scheme.

The fine-print strip is `#3E5321` — a **deeper green**, not the near-black it
used to be, so the footer steps down within one colour family.

---

## 3. Typography — unchanged

**This section is carried over intact.** Thenie's Baloo 2 is not adopted.

Two faces, both **self-hosted** — never a font CDN, which leaks every visitor's
IP and page to a third party.

| Role | Family | Use |
|---|---|---|
| Display | **Erode** | titles, headlines, subtitles |
| Body / UI | **Inter** | body copy, longer text, smaller-scale applications |

**Erode is not on Google Fonts.** It is an Indian Type Foundry face distributed
through Fontshare; check its licence before first use. Inter is SIL OFL. Both
live in `web/public/fonts/` with their licence beside them.

Erode ships four discrete cuts, so a display weight must land on 600 or 700
exactly — ask for 650 and the browser synthesises a fake bold.

| Role | Face | Size | Weight | Tracking |
|---|---|---|---|---|
| `h1` | Erode | 44px → 64px ≥768px | 700 | −0.022em |
| `h2` | Erode | 32px | 700 | −0.015em |
| `h3`, card titles | Erode | 24px | 600 | −0.01em |
| `.lede` | Inter | 20px | 600 | — |
| body | Inter | 17px | 450 | — |
| `.eyebrow` | Inter | 13px | 700 | +0.1em, uppercase |
| nav, buttons, labels | Inter | 15–17px | 600–700 | — |

One number moved with the re-skin: the `.lede` and `h2` sizes were
**load-bearing for accessibility** on the retired beige-on-green pairing, which
no longer exists. They are kept at those sizes as a type decision rather than a
contrast workaround — on the white ground every ink clears AA at every size in
this table.

### 3.1 The wordmark

`docs/design_guideline/logo.png` is 7582×1989 flat `#1C3D34` lettering, and
`#1C3D34` is a retired colour. `scripts/mklogo.py` reduces the artwork to a
coverage mask and re-emits it in any colour, so the cuts are regenerated rather
than re-drawn:

- `evermore-wordmark-deep.png` → re-emit in `--ink #2B2620` for light surfaces
- `evermore-wordmark-light.png` → re-emit in `--cream-soft` for the dark bar
- `og-default.png` → 1200×630 share card

**This is an outstanding build step**, listed in `RUN-WHEN-BACK.md`: the PNGs
on disk are still the old green and have not been regenerated.

---

## 4. Component layer

Shapes follow Thenie. Sizes and type follow §3.

### 4.1 Radii

Thenie is a **pill-and-round-card** system: `999px` on every interactive pill
(16 uses), `16px` on cards, `8–20px` elsewhere.

| Token | Value |
|---|---|
| `--radius-sm` | 8px |
| `--radius-md` | 12px |
| `--radius-lg` | 16px — cards |
| `--radius-xl` | 20px |
| `--radius-pill` | 999px — buttons, chips, nav, status pills |

### 4.2 Buttons

All pills, `2px` transparent border, `min-height: var(--tap)` 44px,
`translateY(-2px)` on hover — Thenie's `.btn`.

| Variant | Rest | Hover | Ratio |
|---|---|---|---|
| `.btn-primary` | `--coral-deep` / white | `--coral-deepest` | **5.12** → **6.68** |
| `.btn-secondary` | `--olive-tint` / `--olive-deep` | `--olive` / white | **7.16** → **4.86** |
| `.btn-outline` | transparent / `--ink`, `--ink` border | `--ink` / white | **14.99** |

On `.ground-deep` the primary inverts to a cream fill with ink (**13.78**) and
the secondary becomes a cream outline — otherwise a coral-deep pill sits on
near-black at **2.93**.

### 4.3 Cards

`--cream-soft` ground, `1px --paper-line`, `--radius-lg` 16px, 26px padding,
`0 6px 20px rgba(43,38,32,0.07)`. Thenie's `.card` verbatim, including the
local `--ink*` re-declaration that lets a card sit on any ground without a
per-context override.

### 4.4 The floating nav

Thenie's signature: a fixed pill, bottom-centre, `--cream-raised`,
`1px --paper-line`, `999px`, `0 14px 34px rgba(43,38,32,0.22)`, with a
`--coral-deep` CTA pill inside it.

### 4.5 Shadows

| Token | Value |
|---|---|
| `--shadow-sm` | `0 1px 2px rgba(43,38,32,0.08)` |
| `--shadow-md` | `0 6px 20px rgba(43,38,32,0.07)` |
| `--shadow-lg` | `0 14px 34px rgba(43,38,32,0.22)` |

### 4.6 The WhatsApp float

`#128C7E` is WhatsApp's own green and is **not recoloured** — recolouring stops
the mark reading as WhatsApp.

| Pairing | Ratio | Verdict |
|---|---|---|
| teal on the white page | **4.14** | PASS 1.4.11 |
| teal on the dark scheme | **4.18** | PASS 1.4.11 |
| white glyph on the teal | **4.14** | PASS 1.4.11 (it is an icon) |
| cream ring on white | **1.09** | **REJECTED** |

**D32 moved what carries the affordance.** On the retired deep-green ground the
teal measured 2.87 and failed 1.4.11 alone, so a beige ring was the findable
boundary. On both of the new grounds the teal clears 3:1 unaided, so the ring
is **decoration** and is painted `--page-bg` — a cream ring on a white page
measures **1.09** and separates nothing.

`tools/shot/verify-ui.mjs` asserted the retired beige `#FFFAE0` by hex and
would have failed forever after the re-skin. It now asserts the ring matches
the computed page ground.

### 4.7 Container

`--container: 1180px` (Thenie's `.container`), 24px side padding.

---

## 5. What is not carried over from Thenie

- **Baloo 2.** Erode + Inter stay (§3).
- **`font-family:'Brush Script MT', cursive`** — appears in the source with no
  webfont behind it, so it renders differently on every machine. Thenie's
  `.script` class uses italic Baloo 2 anyway; Evermore uses italic Erode.
- **The single-theme comment.** The source says *"single committed dark theme:
  no OS/toggle variants needed"* while shipping a light theme. Evermore ships
  both schemes (§2.9).
- **The inline base64 images.** 45 of them, 5.1 MB. Evermore serves real files.

---

## 6. Verification

```
/home/dev/projects/evermore/scripts/contrast.py
```

53 pairings, including every **REJECTED** source value, so a regression that
restores one is caught by name. The script also asserts that every ratio it
computes actually appears in this document — a design sheet and its checker
that disagree mean one of them is wrong, and the incident log records a guard
that silently stopped guarding because nobody compared them.
