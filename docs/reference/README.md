# Reference — the healthy_catering build

**Nothing in this folder is a decision about Evermore.**

These are the working documents of a different project: `healthy_catering`, a
B2C healthy-catering ordering site built for Jakarta between 2026-08-12 and
2026-08-31. They are here because they are worth reading, not because they
apply.

Read them the way you would read someone else's finished project: for the
shape of the reasoning, the traps that were hit, and the wording of things that
turned out to be hard to word. Do not read `02-decisions.md` and conclude that
Evermore has settled 34 decisions. It has settled none.

---

## What is live, and lives one directory up

| | |
|---|---|
| `../99-steven-preference.md` | Steven's portable engineering DNA. **This is the contract.** |
| `../10-design-system.md` | The Evermore brand — same brand, so it applies directly. See the caveat below. |
| `../11-i18n.md` | Locale, timezone and catalogue patterns. |
| `../design_guideline/` | The supplied brand artwork. |
| `../../.claude/skills/impeccable/` | The standard of work. |

### One caveat on the design system

`10-design-system.md` §1–3 is the brand: the wordmark, the palette with its
contrast arithmetic, the typeface pairing. That is Evermore's and it carries
over whole.

§4 is the component layer, and it was read off a design canvas drawn for
**healthy_catering's screens**. The *rules* are portable and hard-won — the
AA-by-size policy in §4.1, the button and pill specs, the callout pattern, the
`text-bar` collision in §4.1b that painted text at 1.00:1 for weeks. The
*artboard inventory* in §4 and §4.12 describes screens that do not exist here:
menu calendars, packing labels, kitchen coverage maps. Take the rules; ignore
the inventory until Evermore has screens of its own to name.

---

## What each file is

| File | What it was | Worth reading for |
|---|---|---|
| `PROMPT.md` | The original brief Steven supplied | How a brief that produced a working system was actually worded |
| `00-README-and-decisions.md` | Index and decision log | — |
| `01-domain-model.md` | Entities, invariants, state machines | Money as integers, capacity that cannot oversell, price resolution with a stored trace |
| `02-decisions.md` | 34 settled decisions, with the reasoning | The *form* of a decision record |
| `03-open-questions.md` | What was asked and not answered | How to park a question instead of guessing |
| `04-milestones.md` | The delivery plan | — |
| `12-security.md` | Control map: every control, and the test that proves it | **The template worth stealing.** Every row names a file and a proof; the ✅ marks were re-earned by running, not inherited. Its file paths point into healthy_catering. |
| `14-production-deployment-handbook.md` | Empty-machine deploy, absolute paths | The shape of a handbook that works |
| `15-user-guide.md`, `16-admin-guide.md` | End-user documentation | — |
| `PROGRESS.md` | Live build status at hand-off | The ✅/🟡/⬜ discipline, and how partial work was reported honestly |
| `RUN-WHEN-BACK.md` | Steps needing a terminal, a browser or credentials | How to hand off blocked work without hiding it |
| `screenshots/` | The built UI at hand-off | What the design system produced in practice |

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
