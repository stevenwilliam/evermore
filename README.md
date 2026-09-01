# evermore

Seeded 2026-09-01 with Steven's engineering standards and the Evermore brand.

**There is no product here yet** — no brief, no domain model, no code. What
exists is the contract for how the work gets done once there is one.

## Read in this order

| | |
|---|---|
| **`CLAUDE.md`** | The contract for this project. Read it first, every session. |
| `docs/99-steven-preference.md` | Steven's portable engineering DNA — the source `CLAUDE.md` is generated from. |
| `.claude/skills/impeccable/SKILL.md` | The standard of work. Read before writing a change and again before calling one done. |
| `docs/10-design-system.md` | The Evermore brand as tokens, with the contrast arithmetic already done. |
| `docs/11-i18n.md` | Locale, timezone and message-catalogue patterns. |
| `docs/design_guideline/` | The supplied brand artwork. |
| `docs/reference/` | A **prior build**, for reading. Its README explains what it is and is not. |

## What happens next

`CLAUDE.md` §9 step 2: Steven gives the PRD and business-rules brief. Nothing
downstream starts until he confirms it — no scaffold, no "just to get started"
`main.go`. The bootstrap checklist for the step after that is
`docs/99-steven-preference.md` §12.

## The one thing to get right on day one

Install and read `.claude/skills/impeccable/SKILL.md` before writing any code.
Every rule in it was written after the matching bug reached a running site, and
the incident log at the end names them. It is worth most before there is
anything to fix.
