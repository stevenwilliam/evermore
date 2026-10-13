# Design System — Evermore · ORCHARD

**A complete redesign, 2026-10-13 (D41).** Steven: *"forget all the design
guideline, only preserve the font type, redo all design 100%, make a new design
that very fresh color, young soft enthusiast, humanize, not seems created by AI,
with moving animation."*

Everything before this is retired — the deep-green Nourish palette (pre-D32)
and the Thenie coral/olive palette (D32–D39). **The typefaces are the only
thing carried over**, because they were the one thing asked to survive.

A full backup was taken before any file was touched:

```
/home/dev/backups/evermore-20261013-1019.bundle        31/31 commits, verified by restore
/home/dev/backups/evermore-20261013-1019-full.tar.gz   1570 entries, .git and .env included
```

Every ratio below is computed by `scripts/contrast.py`. Run it to re-earn these
numbers rather than trusting the table.

---

## 1. The idea, and why it looks like this

"Not seems created by AI" is a design constraint, so it is worth stating what
it rules **out**. Generated pages converge on a narrow set of moves: soft
purple-blue gradients, one radius on everything, perfectly even grids, blurred
drop shadows, full symmetry. This design goes the other way on each of them.

| Generic | Orchard |
|---|---|
| gradient ground | **warm paper `#FDFBF6`** with a real grain texture |
| blurred drop shadow | **solid offset shadow**, `4px 4px 0` in ink |
| hairline borders | **2px ink outlines** |
| one radius everywhere | **five different radii**, plus an asymmetric blob |
| perfectly aligned grid | cards rotated **−0.5°, +0.35°, −0.2°** by `nth-child` |
| pure `#FFFFFF` | paper, with white reserved for cards *on* it |
| static | drifting blobs, scroll reveal, counters, press-into-shadow buttons |

The reference is **risograph printing and sticker books**: flat spot colours,
visible registration offsets, things placed slightly askew by hand. The
rotations are fractions of a degree on purpose — a large tilt reads as a bug,
a small one reads as a decision.

---

## 2. Palette

| Token | Value | Role |
|---|---|---|
| `--paper` | `#FDFBF6` | page ground — warm, deliberately not white |
| `--paper-2` | `#F6F1E6` | alternating band |
| `--card` | `#FFFFFF` | cards, brighter than the paper they sit on |
| `--ink` | `#2A2521` | a soft black-brown, never `#000` |
| `--ink-soft` | `#5C544A` | body and meta |
| `--ink-faint` | `#766D62` | placeholders — **the floor** |
| `--green` | `#2F7D5C` | primary: links, the filled button |
| `--green-hi` | `#246349` | hover |
| `--green-lt` | `#5FBF95` | dark-scheme text |
| `--green-wash` | `#DCEFE5` | tinted panels, callouts |
| `--orange` | `#F2784B` | the accent — **a fill, never ink on paper** |
| `--orange-deep` | `#C8471D` | the orange that carries white text |
| `--blue` | `#4C6FBF` | informational |
| `--butter` | `#F6C65B` | the eyebrow sticker, warn pills |
| `--focus` | `#1F4FD8` | focus ring |

### 2.1 Ink on the three light grounds

There are three, and `--ink-faint` is measured against the **worst** of them
(the band) rather than the most flattering:

| Pairing | Ratio | Verdict |
|---|---|---|
| ink on paper | **14.66** | AAA |
| ink-soft on paper | **7.20** | AAA |
| ink-faint on paper | **4.91** | AA |
| ink on the band | **13.46** | AAA |
| ink-soft on the band | **6.61** | AA |
| ink-faint on the band | **4.51** | AA |
| ink on a card | **15.16** | AAA |
| ink-soft on a card | **7.44** | AAA |
| ink-faint on a card (placeholder) | **5.08** | AA |

`--ink-faint` was first set to `#857B6E`, which measures **4.02 / 3.69 / 4.15**
— under the body floor on all three. It is `#766D62` precisely so the
placeholder text in a search box is legal on every ground it can land on.

### 2.2 Green — the primary

| Pairing | Ratio | Verdict |
|---|---|---|
| green as text on paper | **4.82** | AA |
| green as text on a card | **4.99** | AA |
| white on the green button | **4.99** | AA |
| white on green hover | **7.10** | AAA |
| ink on the green wash | **12.65** | AAA |
| ink-soft on the green wash | **6.21** | AA |

### 2.3 Orange and butter — fills, not inks

| Pairing | Ratio | Verdict |
|---|---|---|
| orange as text on paper (REJECTED) | **2.68** | FAIL |
| ink on the orange fill | **5.46** | AA |
| orange-deep as text on paper | **4.64** | AA |
| white on orange-deep | **4.80** | AA |
| ink on the orange wash | **12.66** | AAA |
| ink on the butter sticker | **9.49** | AAA |
| butter as text on paper (REJECTED) | **1.54** | FAIL |
| ink on the butter wash | **13.61** | AAA |

Both rejected rows are kept in `contrast.py` by name, so using either as a text
colour is caught rather than argued about. `--orange-deep` is the cut that
carries white.

### 2.4 Blue, lines and focus

| Pairing | Ratio | Verdict |
|---|---|---|
| blue as text on paper | **4.69** | AA |
| ink on the blue wash | **12.26** | AAA |
| the ink rule on paper | **14.66** | PASS 1.4.11 |
| rule-soft on paper (decorative) | **1.48** | FAIL 1.4.11 |
| focus ring on paper | **6.41** | PASS 1.4.11 |
| focus ring on a card | **6.63** | PASS 1.4.11 |
| focus ring on the band | **5.88** | PASS 1.4.11 |

`--rule-soft` is **decorative only** at 1.48 — table row separators and the
footer hairline. Every control edge is the full ink rule at 2px.

### 2.5 The footer

Ink-grounded in both schemes, so it reads the same way day or night.

| Pairing | Ratio | Verdict |
|---|---|---|
| paper on the ink footer | **14.66** | AAA |
| fineprint #CFC7B9 on the footer | **9.04** | AAA |
| butter link on the footer | **9.49** | AAA |

### 2.6 Dark scheme

The paper becomes board and the ink becomes chalk. The brand colours **lighten**
rather than invert, because the mid green fails outright on the dark ground.
Button fills keep their light-scheme values so a control stays recognisable.

| Pairing | Ratio | Verdict |
|---|---|---|
| ink on the dark ground | **15.54** | AAA |
| ink-soft on the dark ground | **8.95** | AAA |
| ink-faint on the dark ground | **5.67** | AA |
| ink on the dark card | **13.99** | AAA |
| green-lt on the dark ground | **7.75** | AAA |
| green on the dark ground (REJECTED) | **3.48** | FAIL |
| orange-lt on the dark ground | **8.40** | AAA |
| blue-lt on the dark ground | **7.44** | AAA |
| butter on the dark ground | **10.87** | AAA |
| focus-lt on the dark ground | **8.74** | PASS 1.4.11 |

### 2.7 The WhatsApp float

| Pairing | Ratio | Verdict |
|---|---|---|
| WA teal on paper | **4.00** | PASS 1.4.11 |
| WA white glyph on the teal | **4.14** | PASS 1.4.11 |

WhatsApp's own green is never recoloured; the 2px ink ring is what separates it
from any ground.

---

## 3. Typography — the one thing carried over

Two faces, both **self-hosted** — never a font CDN, which leaks every visitor's
IP and page to a third party.

| Role | Family |
|---|---|
| Display | **Erode** — headings, prices, stat figures |
| Body / UI | **Inter** — everything else |

Erode ships four discrete cuts, so a display weight must land on 600 or 700
exactly; asking for 650 makes the browser synthesise a fake bold.

| Role | Face | Size | Weight |
|---|---|---|---|
| `h1` | Erode | 48px → 72px ≥768px | 700 |
| `h2` | Erode | 34px | 700 |
| `h3` | Erode | 25px | 600 |
| `.price`, `.stat-item b` | Erode | 25–34px | 700 |
| `.lede` | Inter | 20px | 450 |
| body | Inter | 17px | 450 |
| `.eyebrow` | Inter | 13px | 700, `0.12em`, uppercase |

Every figure that lands in a column — `.num`, `.price`, `.count`, `.eq`,
table cells — is set in **tabular numerals**, so digits do not shift width as
a counter animates or a total updates.

---

## 4. Components

### 4.1 Radii — deliberately uneven

| Token | Value | Used by |
|---|---|---|
| `--radius-xs` | 6px | focus ring |
| `--radius-sm` | 10px | inputs, callouts, tables |
| `--radius-md` | 18px | cards, the nav panel |
| `--radius-lg` | 26px | the hero frame |
| `--radius-pill` | 999px | buttons, chips, pills |
| `--radius-blob` | `62% 38% 55% 45% / 45% 55% 45% 55%` | the drifting hero shapes |

### 4.2 Buttons

Pill, 2px ink outline, solid offset shadow. **Pressing moves the button into
its own shadow** (`translate(4px,4px)`, shadow removed) — the one interaction
that reads as physical rather than rendered.

| Variant | Fill | Ink | Ratio |
|---|---|---|---|
| `.btn-primary` / `.btn-solid` | `--green` | white | **4.99** → hover **7.10** |
| `.btn-secondary` | `--butter` | ink | **9.49** → hover orange **5.46** |
| `.btn-outline` | transparent | ink | **14.66** |

### 4.3 The eyebrow is a sticker

Filled butter, 2px outline, small offset shadow, rotated −1.4°. The hero's is
orange and rotated the other way, so the two never look stamped from one die.

### 4.4 Cards

White on paper, 2px outline, `4px 4px 0` shadow, and a per-position rotation
from `nth-child`. On hover a card straightens to 0° and lifts — the motion
resolves the tilt rather than adding to it.

### 4.5 Controls are pinned in both schemes

A control keeps its identity day and night — a green button is the same green.
That is partly a design choice and mostly a **defect fix**.

Brand fills like `--green`, `--butter` and `--orange` are redefined in the dark
scheme, and `--ink` flips too. Put a flipping ink on a fixed fill, or fixed
white on a flipping fill, and the pair silently stops working. Measured in dark
before this was fixed:

| Pairing | Ratio | |
|---|---|---|
| white on the lightened green button | **2.24** | **REJECTED** |
| `--ink` on the butter button, eyebrow and warn pill | **1.43** | **REJECTED** |
| `--ink` on the lightened orange hero eyebrow | **1.85** | **REJECTED** |

The rule, and it is structural rather than another patch:

> **Any element whose background is a fixed brand colour takes `--on-brand` as
> its ink, and `--on-brand` is never redefined in a dark block.**

| Token | Value | Ink | Ratio |
|---|---|---|---|
| `--ctl-green` | `#2F7D5C` | white | **4.99** |
| `--ctl-green-hi` | `#246349` | white | **7.10** |
| `--ctl-butter` | `#F6C65B` | `--on-brand` | **9.49** |
| `--ctl-orange` | `#F2784B` | `--on-brand` | **5.46** |

This is the **third** time a flipping token has broken a fixed fill — D34
`.hero-art`, D39 `.footer`, and this. All three were found by looking at the
rendered page in both schemes, never by reading the CSS. The three rejected
rows above are kept in `contrast.py` by name so the pattern cannot return
quietly a fourth time.

### 4.6 Torn band edges

`.band-raised` draws its top and bottom edge with a repeating radial mask, so
the join between bands is a scalloped tear rather than a ruled line. It is
geometry, not an image, so it recolours with the theme.

---

## 5. Motion

Motion is on by default and governed entirely by duration **tokens**, so
`prefers-reduced-motion` collapses it in one place rather than per rule.

| Effect | Where |
|---|---|
| drifting blobs | behind the hero, 14s alternating |
| scroll reveal | section heads, cards, callouts, tables, stat strips |
| stagger | siblings reveal 70ms apart, capped at 6 |
| counters | `.count` animates to the number already in the HTML |
| press | buttons, chips and the float move into their own shadow |
| image scale | card photos grow 4.5% on hover |
| underline draw | `.underline-draw` paints a butter highlight on reveal |

**Two rules this obeys.** Nothing is hidden by CSS alone — `.reveal` is added
by JavaScript, so with JS off every element is simply visible, which is the
usual failure of scroll-animation libraries. And the counter reads its target
from the markup, so a visitor without JS sees the real number rather than a
zero.

Under `prefers-reduced-motion: reduce` the duration tokens drop to **1ms**
(not 0, so `transitionend` still fires and nothing waiting on it hangs), the
blob animation stops, the card rotations are removed, and the reveal and
underline are forced to their finished state.

---

## 6. Verification

```
/home/dev/projects/evermore/scripts/contrast.py
```

52 pairings, including every **REJECTED** value, so a regression that restores
one is caught by name. The script also asserts that every ratio it computes
appears in this document.
