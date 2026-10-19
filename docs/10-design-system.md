# Design System — Evermore · ATELIER

**2026-10-13 (D43).** Steven, on the Orchard redesign shipped hours earlier:
*"the design is too retro and old / make me modern design / young color /
elegan / humanize / not seems created by AI."*

He was right. Orchard answered "not AI" with a **risograph** language — 2px ink
outlines, solid offset shadows, rotated cards, paper grain. That is a 1990s
zine idiom, and reading as retro is exactly what it does. Atelier is the
opposite register.

| Orchard (retro) | Atelier (modern) |
|---|---|
| 2px ink outlines on everything | **hairlines**, and only where a boundary is needed |
| solid `4px 4px 0` offset shadow | **soft layered elevation** |
| cards rotated by `nth-child` | square to the grid; depth does the work |
| paper grain overlay | clean surfaces |
| filled, tilted sticker eyebrow | a tracked label with a short accent rule |
| bouncy overshoot easing | a long, even glide, y-axis only |
| accent everywhere | **one accent, used sparingly** |

**The typefaces still do not change.** Erode + Inter. They have now survived
four palettes and are the only constant in the project.

## 1. "Not created by AI", in a modern idiom

The riso trick is no longer available, so the humanising work moves elsewhere:

- **Editorial asymmetry.** Section heads sit in a 54ch measure on the left while
  content runs wider beneath. Generated layouts centre everything.
- **Restraint.** The citrus accent appears on roughly one element per screen.
  Over-decoration is the tell; confidence is knowing what to leave out.
- **Real space.** The spacing scale tops out at 148px. Generated pages crowd.
- **What is avoided:** violet gradients, glassmorphism panels, a symmetrical row
  of three identical feature cards, and an icon beside every heading.

---

## 2. Palette

| Token | Value | Role |
|---|---|---|
| `--paper` | `#F9F9F6` | page ground, a whisper warm |
| `--card` | `#FFFFFF` | raised surfaces |
| `--warm` | `#EDEAE2` | the quiet band |
| `--ink` | `#121614` | near-black with a green cast |
| `--ink-soft` | `#555D58` | body |
| `--ink-faint` | `#656B67` | meta — **the floor** |
| `--brand` | `#13543E` | deep forest; links and the filled control |
| `--accent` | `#D8F26B` | citrus — the young note, **a fill only** |
| `--info` | `#2F6690` | informational |

### 2.1 Ink

| Pairing | Ratio | Verdict |
|---|---|---|
| ink on paper | **17.30** | AAA |
| ink-soft on paper | **6.44** | AA |
| ink-faint on paper | **5.17** | AA |
| ink on a card | **18.25** | AAA |
| ink-faint on a card | **5.45** | AA |
| ink on the warm band | **15.18** | AAA |
| ink-soft on the warm band | **5.65** | AA |
| ink-faint on the warm band | **4.53** | AA |

`--ink-faint` is measured against the **worst** of the three surfaces, the warm
band, not the most flattering.

### 2.2 Brand and accent

| Pairing | Ratio | Verdict |
|---|---|---|
| brand as text on paper | **8.42** | AAA |
| brand as text on a card | **8.88** | AAA |
| ink on the brand wash | **15.70** | AAA |
| ink-soft on the brand wash | **5.84** | AA |
| accent as text on paper (REJECTED) | **1.18** | FAIL |
| on-brand on the accent fill | **14.63** | AAA |
| ink on the accent wash | **16.95** | AAA |
| info as text on paper | **5.81** | AA |
| ink on the info wash | **15.40** | AAA |

The citrus is **never ink** — 1.18 on paper. It is a fill that carries
`--on-brand`, and that is also why it works as the one bright note: a colour
used in a single role cannot be overused.

### 2.3 Controls are pinned in both schemes

The D42 rule is carried forward unchanged, because it is the fix for a bug that
recurred three times:

> **Any element whose background is a fixed brand colour takes `--on-brand`,
> and `--on-brand` is never redefined in a dark block.**

| Pairing | Ratio | Verdict |
|---|---|---|
| white on the brand control | **8.88** | AAA |
| white on the brand hover | **12.35** | AAA |
| on-brand on the accent control | **14.63** | AAA |
| UNPINNED white on brand-lt (REJECTED) | **1.88** | FAIL |

### 2.4 Lines and focus

Two line tokens, and the distinction is load-bearing:

| Pairing | Ratio | Verdict |
|---|---|---|
| line on paper (decorative) | **1.23** | FAIL 1.4.11 |
| line-strong on paper (control edge) | **3.43** | PASS 1.4.11 |
| line-strong on a card | **3.62** | PASS 1.4.11 |
| line-strong on the warm band | **3.01** | PASS 1.4.11 |
| REJECTED #9BA09C as a control edge | **2.52** | FAIL 1.4.11 |
| focus ring on paper | **5.52** | PASS 1.4.11 |
| focus ring on a card | **5.82** | PASS 1.4.11 |
| dark control edge on the ground | **3.27** | PASS 1.4.11 |
| dark control edge on the card | **3.01** | PASS 1.4.11 |

`--line` at **1.23** is decorative only — table rules, the footer hairline.
Every **control** edge is `--line-strong`, which clears 1.4.11's 3:1 on all
three surfaces. The first draft used `#9BA09C`, which measures **2.52** and
fails; it is kept as a REJECTED row so it cannot drift back.

### 2.5 Footer and dark scheme

| Pairing | Ratio | Verdict |
|---|---|---|
| paper on the ink footer | **17.30** | AAA |
| fineprint #A9ADA9 on the footer | **8.03** | AAA |
| accent link on the footer | **14.63** | AAA |
| ink on the dark ground | **16.77** | AAA |
| ink-soft on the dark ground | **9.55** | AAA |
| ink-faint on the dark ground | **6.16** | AA |
| ink on the dark card | **15.46** | AAA |
| ink-faint on the dark card | **5.68** | AA |
| brand-lt on the dark ground | **9.92** | AAA |
| brand on the dark ground (REJECTED) | **2.11** | FAIL |
| accent on the dark ground | **14.99** | AAA |
| info-lt on the dark ground | **8.95** | AAA |
| focus-lt on the dark ground | **10.15** | PASS 1.4.11 |

### 2.6 Surfaces over the signature ribbon (D60)

The cards in a ribboned section are translucent — the week strip's day cards
including their headers — so the signature reads through them. Today's day-card
header is the one exception: it is a pinned brand fill (D42) and stays solid. The ribbon's hairline strokes run along the same path as its wide ones
and therefore stack: the darkest pixel it can put under a card is `#77968A`,
which through a 50% card leaves `#BBCBC5`. That is the worst background any
text on these cards can sit on, and it is what the "worst ribbon pixel" rows
below are measured against — not the common case, which is far lighter.

`--ink-faint` fails there at 3.23 and `--ink-soft` at 4.03. That is the entire
reason `--ink-on-glass` (`#454C48`) exists: it holds at 5.24 on the worst pixel
and still reads as muted (8.59) on the ordinary one.

| Pairing | Ratio | Verdict |
|---|---|---|
| ink on the quiet day-card head, worst ribbon pixel | **10.03** | AAA |
| ink-on-glass on the quiet day-card head, worst ribbon pixel | **4.85** | AA |
| ink-faint on the quiet day-card head, worst pixel (REJECTED) | **3.00** | FAIL |
| brand on the day-card body, worst ribbon pixel | **5.27** | AA |
| brand-lt on the dark day-card body, lightest ribbon pixel | **4.80** | AA |
| ink on glass over paper | **17.77** | AAA |
| ink-on-glass over paper | **8.59** | AAA |
| ink-on-glass on tinted glass over paper | **8.03** | AAA |
| ink-on-glass under glass, worst ribbon pixel | **5.24** | AA |
| ink-on-glass under tinted glass, worst ribbon pixel | **4.81** | AA |
| ink-faint under glass, worst ribbon pixel (REJECTED) | **3.23** | FAIL |
| ink-soft under glass, worst ribbon pixel (REJECTED) | **4.03** | FAIL |
| ink-on-glass on the dark glass, lightest ribbon pixel | **5.37** | AA |

### 2.7 The hero's Indonesia map (D64)

The archipelago behind the hero is a mask over `--brand` at 7% with a 3px
blur — background filler, so it is measured where it is darkest: an island
sitting directly behind the type.

| Pairing | Ratio | Verdict |
|---|---|---|
| ink on the hero, over the map | **15.43** | AAA |
| ink-soft on the hero, over the map | **5.74** | AA |
| ink-faint on the hero, over the map | **4.61** | AA |
| brand script on the hero, over the map | **7.51** | AAA |
| ink on the dark hero, over the map | **15.00** | AAA |
| ink-soft on the dark hero, over the map | **8.53** | AAA |

### 2.8 The WhatsApp float

| Pairing | Ratio | Verdict |
|---|---|---|
| WA teal on paper | **3.92** | PASS 1.4.11 |
| WA white glyph on the teal | **4.14** | PASS 1.4.11 |

---

## 3. Typography — the constant

| Role | Face | Size | Weight |
|---|---|---|---|
| `h1` | Erode | 54px → 84px ≥768px | 700 |
| `h2` | Erode | 38px | 700 |
| `h3` | Erode | 21px | 600 |
| `.price`, `.stat-item b` | Erode | 27–38px | 700 |
| `.lede` | Inter | 21px | 420 |
| body | Inter | 17px | 420 |
| `.eyebrow` | Inter | 13px | 600, `0.16em`, uppercase |

Display tracking is **−0.03em**. Tight display letter-spacing against an open
body is the clearest typographic signal of a current design; Orchard's −0.022em
was looser and read older. Body weight drops to 420 for a lighter, airier
colour on the page.

All figures are **tabular**, so digits do not shift width as a counter animates.

---

## 4. Surfaces and depth

Three elevation steps, each a **layered** shadow — one tight contact shadow
plus one wide ambient. A single blurred shadow is the flat, generated look.

| Token | Use |
|---|---|
| `--shadow-1` | resting cards, inputs, the masthead |
| `--shadow-2` | raised panels |
| `--shadow-3` | hover, the hero frame, floating chrome |

Radii are soft and consistent — 8 / 14 / 22 / 32 / pill — the modern register.
Orchard deliberately used five *uneven* radii; that was part of why it read
hand-made and retro.

The masthead and the floating nav use `backdrop-filter` with a
`color-mix` ground, so content passes under them rather than hitting a wall.

---

## 5. Motion

Smooth and restrained. No overshoot, no scale on reveal, y-axis only.

| Effect | Where |
|---|---|
| soft drift | one large brand wash behind the hero, 26s |
| scroll reveal | heads, cards, callouts, tables, stat strips |
| stagger | siblings 70ms apart, capped at 6 |
| counters | `.count` animates to the number already in the HTML |
| hover lift | −4px with elevation 1 → 3 |
| image scale | card photos 1.04 on hover |
| underline | a 3px accent hairline drawing across on reveal |
| **Bekal Bot** | the hero box: 6s float, a 5.5s uneven blink, drag to turn, and on tap a three-stage open — walls fall over 700ms, the food reads in the base, then six pieces scatter (D45) |

Every duration is a **token**, so `prefers-reduced-motion` collapses all of it
in one place. Nothing is hidden by CSS alone — `.reveal` is added by
JavaScript, so with JS off the page is simply visible.

---

## 6. Verification

```
/home/dev/projects/evermore/scripts/contrast.py
```

45 pairings, including every **REJECTED** value, so a regression that restores
one is caught by name. The script also asserts every ratio it computes appears
in this document.
