# Evermore — docs index

**These documents are Evermore's working baseline.** They were written for
`healthy_catering`, a B2C healthy-catering ordering site built for Jakarta
between 2026-08-12 and 2026-08-31, and lived in `docs/reference/`. On
2026-09-01 Steven promoted the whole folder into `docs/` to be used, and
`docs/reference/` was removed. See **D19** in `00-README-and-decisions.md`.

Two things follow, and they matter more than the promotion itself:

1. **The 34 decisions in `02-decisions.md` are the starting position, not a
   contract.** Every one is open to Steven changing it for Evermore. Nothing
   here has been re-confirmed against a brief for this product.
2. **The ✅ marks in `PROGRESS.md` were earned by `healthy_catering`'s code,
   which is not in this repo.** `CLAUDE.md` §5: a ✅ is re-earned by running the
   gate, never inherited. File paths throughout point into `healthy_catering`
   until they are repointed here.

---

## The live contract

| | |
|---|---|
| `../CLAUDE.md` | How this project is built. Where it conflicts with a habit, it wins. |
| `99-steven-preference.md` | Steven's portable engineering DNA. `CLAUDE.md` is generated from §3–§9. |
| `10-design-system.md` | The Evermore brand — same brand, so §1–§3 apply directly. See the caveat below. |
| `11-i18n.md` | Locale, timezone and catalogue patterns. |
| `design_guideline/` | The supplied brand artwork. |
| `../.claude/skills/impeccable/` | The standard of work. Read before writing a change and again before reporting one done. |

### One caveat on the design system

`10-design-system.md` §1–3 is the brand: the wordmark, the palette with its
contrast arithmetic, the typeface pairing. That is Evermore's and it carries
over whole.

§4 is the component layer, and it was read off a design canvas drawn for
**healthy_catering's screens**. The *rules* are portable and hard-won — the
AA-by-size policy in §4.1, the button and pill specs, the callout pattern, the
`text-bar` collision in §4.1b that painted text at 1.00:1 for weeks. The
*artboard inventory* in §4 and §4.12 describes screens that may not survive
into Evermore: menu calendars, packing labels, kitchen coverage maps. Take the
rules; treat the inventory as the baseline's, not as settled here.

---

## The promoted baseline

| File | What it is | Worth reading for |
|---|---|---|
| `PROMPT.md` | The original brief Steven supplied | How a brief that produced a working system was actually worded |
| `00-README-and-decisions.md` | Index, decision log (D1–D19), open questions | The decision-log discipline |
| `01-domain-model.md` | Entities, invariants, state machines | Money as integers, capacity that cannot oversell, price resolution with a stored trace |
| `02-decisions.md` | 34 settled decisions, with the reasoning | The *form* of a decision record. **Not** normative business rules — see the numbering note below |
| `03-open-questions.md` | What was asked and not answered | How to park a question instead of guessing |
| `04-milestones.md` | The delivery plan | Its §2 sets out the numbering collision below |
| `12-security.md` | Control map: every control, and the test that proves it | **The template worth stealing.** Every row names a file and a proof; its paths point into `healthy_catering` |
| `14-production-deployment-handbook.md` | Empty-machine deploy, absolute paths | The shape of a handbook that works |
| `15-user-guide.md`, `16-admin-guide.md` | End-user documentation | — |
| `PROGRESS.md` | Build status at `healthy_catering`'s hand-off | The ✅/🟡/⬜ discipline — and see point 2 above |
| `RUN-WHEN-BACK.md` | Steps needing a terminal, a browser or credentials | How to hand off blocked work without hiding it |
| `screenshots/` | That build's UI at hand-off (23 images) | What the design system produced in practice |

### A numbering collision still to settle

The house doc set (`99-steven-preference.md` §10) reserves `01-PRD`,
**`02-business-rules` (normative)**, `03-data-model`, `04-api-spec`. The
promoted planning documents occupy `01`–`04` with different content, so **`02`
is a decision list, not the normative business rules** someone opening it would
expect. `04-milestones.md` §2 proposes the resolution: fold the planning
documents into the house set, then delete them, before any code is written.
Open until Steven decides.

---

## Three things from that build worth carrying, whatever Evermore turns out to be

1. **The database enforced the invariant, not just the application.** Foreign
   keys, `CHECK`, partial uniques, and a GiST exclusion on overlapping price
   ranges. The oversell test passed with twenty concurrent writers because the
   constraint was there, not because the code was careful.

2. **Every ✅ was re-earned by running the gate.** `PROGRESS.md` distinguishes
   "written" from "run" throughout, and says so where something was only
   written. That is the habit `99-steven-preference.md` is trying to install.

3. **Three separate defects made text invisible at 1.00:1**, and none of them
   was visible in the source — a CSS specificity collision, a grid-column
   collision, and a Tailwind key that existed in two scales at once. All three
   were found by looking at a screenshot and probing computed styles. That is
   why "verify visual work by looking at it" is a rule and not advice.
