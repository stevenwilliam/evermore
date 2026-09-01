# healthy_catering — Document Set

**Version:** 0.2 (brief received, planning documents written)
**Date:** 12 August 2026
**Status:** the product brief landed on 2026-08-12 and is stored verbatim at
`PROMPT.md`. Evermore is a **B2C healthy-catering ordering website for Jakarta**
(`www.evermore.co.id`). The four planning documents it asks for — `01`
domain model, `02` decisions, `03` open questions, `04` milestones — are written
and **awaiting Steven's confirmation. No application code until he approves.**

---

## 1. What this document set is

The engineering and product spec for `healthy_catering`, built in the house
style. House style itself — how Steven works, and his stack, database and
security preferences — lives in `99-steven-preference.md` and is portable
between projects.

`02-business-rules.md` will be **normative**: where it conflicts with any other
document, it wins. Build and working conventions live in `../CLAUDE.md`.

| # | Document | Purpose | State |
|---|---|---|---|
| 00 | This file | Index, decision log, open questions | ✅ |
| 01 | `01-PRD.md` | Problem, personas, scope, requirements, metrics | ⬜ awaiting brief |
| 02 | `02-business-rules.md` | Normative business logic, `BR-x.y` | ⬜ awaiting brief |
| 03 | `03-data-model.md` | Schema, ERD, DDL, migration notes | ⬜ |
| 04 | `04-api-specification.md` | REST contract, error model, idempotency, auth | ⬜ |
| 05 | `05-architecture-and-nfr.md` | Service architecture, security, performance | ⬜ |
| 06 | `06-domain-operations.md` | Domain-specific operational logic, runbooks | ⬜ |
| 07 | `07-test-plan.md` | Test strategy, critical scenarios, QA checklist | ⬜ |
| 08 | `08-roadmap.md` | Phasing, release plan, sequencing | ⬜ |
| 09 | `09-deployment.md` | Production deployment | ⬜ |
| 10 | `10-design-system.md` | Palette, typography, components, a11y | 🟡 brand received |
| 11 | `11-local-dev-setup.md` | Local dev environment, everyday commands | ⬜ |
| 12 | `12-security.md` | OWASP ASVS L2 / Top-10 control map, abuse cases | ⬜ |
| 13a | `13a-development-server-preparation.md` | Dev-server handbook — Part A (server once) + Part B (onboard a project) | ⬜ |
| 14 | `14-production-deployment-handbook.md` | Empty-machine deployment, copy-paste | ⬜ |
| 15 | `15-user-guide.md` | Customer guide | ⬜ |
| 16 | `16-admin-guide.md` | Staff guide | ⬜ |
| 99 | `99-steven-preference.md` | Portable engineering DNA — **project-agnostic** | ✅ |

Plus `PROGRESS.md` (live build status) and `RUN-WHEN-BACK.md` (interactive steps).

### 1.1 Planning documents — temporary, and they occupy numbers the house set wants

The master brief (`PROMPT.md` §0) asks for four documents under names that
collide with the house set above. Nothing collides on disk today because none of
the house documents exist yet, and **nothing has been renamed unilaterally**.

| File | Purpose | State |
|---|---|---|
| `PROMPT.md` | Steven's master brief, verbatim — the source for all four | ✅ |
| `01-domain-model.md` | Entities, relationships, state machines, Mermaid ERD | ✅ awaiting confirmation |
| `02-decisions.md` | All 23 `[DECIDE]` items + 5 stack conflicts, with recommendations | ✅ awaiting confirmation |
| `03-open-questions.md` | 29 ambiguities, each with the answer needed and a default | ✅ awaiting confirmation |
| `04-milestones.md` | M0–M14, each slice demoable | ✅ awaiting confirmation |

`04-milestones.md` §2 proposes folding these four into the house numbering on
approval — `01` into `03-data-model` and `02-business-rules`, `02` and `03` into
this file, `04` into `08-roadmap` — so `02` is not left meaning two different
things. **Steven's call; see `03-open-questions.md`.**

---

## 2. Decision log

Record every decision that changes behaviour here, with a date, and reflect it
in the affected docs the same day.

| ID | Date | Decision | Rationale | Docs touched |
|----|------|----------|-----------|--------------|
| D1 | 2026-08-12 | **Adopt Steven's house style verbatim** — `99-steven-preference.md` copied unchanged from ruuma, and `CLAUDE.md` generated from its sections 3–9. Hexagonal Go (`adapter → app → domain`), gin + gorm + PostgreSQL, money as integers with raw SQL on money paths, UUIDv7, numbered forward-only migrations, search box on every list, configurable values in `sys_parameters`, docs updated in the same commit, auto-commit and push to `main`. | Proven on ruuma; the preference file is written to be project-agnostic precisely so it can be copied. | CLAUDE.md, 99 |
| D2 | 2026-08-12 | **`coding stop` / `coding start` is a hard gate.** `coding stop` means change nothing — no edits, files, commits, migrations, deploys or config — until `coding start`. It holds across turns, and a new request while it is on is a request to plan, not a licence to resume. | Steven's directive, 2026-08-12. | CLAUDE.md §6, 99 §1 |
| D3 | 2026-08-12 | **Brand is Evermore**, from the guidelines supplied at `/home/aidev/asset/` and copied to `docs/design_guideline/`. Primary Nourish Green `#1C3D34`, secondary Restore Beige `#FFFAE0`, four tertiary accents; display **Erode**, body **Inter**, both self-hosted. **Four brand colours fail WCAG AA as text or button fills** — `#468973`, `#E0782D`, `#A36E50`, `#CC6883` — so `#1C3D34` carries green text and green fills, and orange is a highlight rather than a label. Calculated, not eyeballed, before any component exists. | Supplied brand; contrast checked on receipt so the build is not designed into a corner. | 10, design_guideline |
| D4 | 2026-08-12 | **Assets were copied, not moved.** Steven asked for a move; the source is another user's home (`aidev`) and a move there is not reversible by this project. The originals remain at `/home/aidev/asset/`. | Reversible beats literal when the difference is destroying someone else's copy. Flagged for Steven to confirm. | 00, design_guideline |
| D5 | 2026-08-12 | **Dark-surface and non-text contrast decided up front.** On `#1C3D34`: beige 11.32, blue light 8.15, orange light 7.27, beige deep 6.47 all pass; `#468973` (2.88) and `#CC6883` (3.33) are never text on it. `#CCBDAA` on beige is **1.75**, so it is *not* an input border, focus ring or any meaningful boundary — those use `#1C3D34`. Energize Orange as a button needs near-black ink at large size (`#1C3D34` on it = 3.90, `#000000` = 6.89), never white. Logo re-verified by decoding the PNG: 7582×1989, ink `rgba(28,61,52)`, **only the final `e` is mirrored** — both `r`s are normal. | The header is going to be Nourish Green and the canvas is going to be beige; deciding the legal inks and borders now costs one calculation, and rediscovering them in review costs a redesign. Closes the "verify before shipping" TODO left on the orange button. | 10 §1, §2.4–2.6 |
| D6 | 2026-08-12 | **Product brief received and stored verbatim at `PROMPT.md`.** Evermore is a B2C healthy-catering ordering website for Jakarta, `www.evermore.co.id`: marketing + menu pages, customer accounts, à-la-carte meal orders and prepaid credit packages, manual bank transfer, automatic per-delivery routing to one of several kitchens by coordinates, full back office. Phase 1 web only, no PWA; phase 2 mobile against the same `/api/v1`. Planning documents `01`–`04` written; **no code until Steven confirms them**. | Step 2 of the delivery workflow (`CLAUDE.md` §9) is complete; step 3 is gated on his confirmation. | CLAUDE.md §1 §10, 00, 01–04, PROGRESS |
| D7 | 2026-08-12 | **Locale settled: IDR as `BIGINT` whole rupiah, `Asia/Jakarta`, `id-ID` + `en`, `www.evermore.co.id`, Jakarta hosting for UU PDP residency. Evermore is the customer-facing brand**; `healthy_catering` is the repo codename. Answers `00` §3 Q2–Q6. | Stated in the brief. Sen is obsolete in retail so the rupiah is the minor unit; business dates are `DATE` and convert through `Asia/Jakarta` explicitly, never server-local. | CLAUDE.md §10, 00 §3, 01 |
| D8 | 2026-08-12 | **The stack is contested and unresolved.** `CLAUDE.md` §3 pins Go + gin + gorm + React 18/Vite; the brief §2 proposes TypeScript + NestJS + Prisma + Next.js and marks it `[DECIDE]`. Five sub-decisions (`02-decisions.md` D-1…D-5) are open and **block M0**. Recommendation: Go + gin + gorm, numbered SQL migrations, Redis as a satellite, and Go templates for the eight public routes beside a React 18 + Vite SPA. | Prisma has no first-class support for `daterange`, exclusion constraints or PostGIS — the three things this product's hardest parts need — so the ORM's benefit disappears exactly where it would be paid for. Recorded as contested rather than silently resolved either way. | 02, CLAUDE.md §3 (pending) |
| D9 | 2026-08-12 | **Steven's answers to the three blocking questions.** (a) **Prices are tax-inclusive**; the DB splits base and tax; the rate is a back-office parameter. The split is snapshotted per order, never stored on the price row. (b) **No refund** — compensation is in credits, unused credits are forfeited at expiry; `REFUNDED` is admin-only for erroneous or duplicate transfers. (c) **1 meal contains several foods; 1 credit buys 1 meal** even when it holds a single dish; **meal nutrition is aggregated from its foods**. (d) Routing = **nearest kitchen**. | Direct answers, 2026-08-12. (c) restructured the model: `scheduled_meal` + `scheduled_meal_item` replace the brief's flat `food_schedule`, because capacity, publication and the credit boundary all belong to the meal, not the food. | 01 §3.3 §3.4 §3.6 §3.7 §3.11 §4, 02 Part 0, 03 |
| D10 | 2026-08-12 | **Stack settled by default under `code start`.** Steven gave the go-ahead without overriding the recommendation, and `02-decisions.md` states that silence on an item takes it. So: **Go + gin + gorm**, numbered forward-only SQL migrations, Redis as a satellite for cache/rate-limit/queue with idempotency keys in Postgres, **Go html/template for the public routes + React 18 + Vite SPA** for the transactional surface, house repo layout with OpenAPI 3.1 generating the TypeScript client. `CLAUDE.md` §3 therefore stands unamended. | Prisma has no first-class `daterange`, `EXCLUDE` or PostGIS support — the three things this product's hardest paths need — so NestJS + Prisma would mean raw SQL anyway, having paid the ORM's cost. Recorded as *decided by default* rather than *confirmed*, so it is visible as a reversible assumption if he disagrees on his return. | 02 Part A, CLAUDE.md §3 |
| D11 | 2026-08-13 | **Every input is validated and sanitized on both sides — frontend and backend.** Added to `99-steven-preference.md` §7 (portable) and `CLAUDE.md` §4 (local). The frontend validates for *feedback*; the backend validates because the frontend can be bypassed with `curl`, so it re-checks presence, type, length, range, format, allow-listed enums, ownership and authorization from scratch. Same rules from one source — the server's OpenAPI contract generates the web app's validation, so the two cannot drift. Sanitize in **and encode out** for the context (HTML, attribute, URL, **CSV cell**, log line, filename). **Reject, never silently repair.** Normalize before validating. | Steven's directive, 2026-08-13. It is a genuine addition rather than a restatement: §7 already required allow-list validation at the adapter edge, but said nothing about the client side or about output encoding, and a rule that exists only in the browser does not exist. | 99 §7 §11, CLAUDE.md §4, 00 |
| D12 | 2026-08-13 | **`platform/sanitize` implements D11**, with `Text`, `Required`, `Email`, `Phone` (Indonesian normalisation to +62), `Enum` (allow-list), `Slug` and `CSVCell`, plus a 1 MiB request-body cap. Applied to the one handler that already existed. | Recording the rule without implementing it would have left it aspirational. The CSV guard is the non-obvious half: **every report in this system exports to Excel** (PROMPT §12), and a cell beginning `=`, `+`, `-` or `@` is a formula — a customer's delivery note becomes code execution on whoever opens the export. | sanitize, adapter/http |
| D13 | 2026-08-13 | **`code start` — the stack is confirmed, not assumed.** Go + gin + gorm, numbered forward-only SQL migrations, Redis satellite, Go html/template for the public routes beside React 18 + Vite for the transactional surface, OpenAPI 3.1 generating the TypeScript client. D10 is upgraded from *decided by default* to **decided**. `CLAUDE.md` §3 stands unamended. | Steven, 2026-08-13. | 02 Part A, CLAUDE.md §3, 00 |
| D14 | 2026-08-13 | **Operational answers, all of them settings rather than code.** Kitchens operate **every day** (was Mon–Sat). **PPN 11%**, changeable in the back office. **Delivery is free at every distance and every order value**, with the band engine still evaluated on every order so charging later is one settings edit. **Dummy bank account**, replaceable in the back office. **SMTP borrowed from ruuma's mailpit satellite**, moved into `sys_parameters` so it is changeable without a deploy. **WhatsApp via WAHA**, matching 99 §9, still switched off until a sender number exists. **No minimum order.** **One cut-off for both slots.** **A cart may span several dates, and the tier resolves on the order's total quantity** — Mon–Fri × 2 meals reaches the 10–19 tier. | Steven, 2026-08-13, answering B2–B13. Every one landed as an `INSERT`/`UPDATE` in migration 0012 rather than a code change, which is what `sys_parameters` was for. | 0012, 02, 03, PROGRESS |
| D15 | 2026-08-13 | **Erode and Inter are self-hosted, and Erode is NOT subset.** Steven confirmed Erode is free from Fontshare. Reading the bundled EULA settles two things: self-hosting is not merely permitted but is what ITF instructs — the download ships a WEB kit and a README telling you to serve it from your own site, and §02's prohibition targets font-*serving* technologies (EOT, Cufon, sIFR), not `@font-face` from your own origin. But **§05 forbids modifying the font software, so the usual latin-subsetting is off the table** — it would trim ~40% of the bytes and would also be a modification. `font-display: swap` plus preloading the two first-screen faces covers the difference. Inter ships as the variable file under SIL OFL: every weight in one 352 KB request, smaller than four static cuts. Licence text sits beside the fonts. Closes `00` §3 Q9. | Steven, 2026-08-13. The subsetting constraint is the non-obvious half and would otherwise have been discovered as a licence breach after launch. | 10, web/public/fonts |
| D16 | 2026-08-13 | **`--border` is `rgba(28,61,52,0.60)`, not 0.28.** The first draft of the token sheet used 0.28 alpha, which composites to **1.69:1** on the beige canvas — the same 1.4.11 failure as `--beige-deep` at 1.75, reintroduced with a transparent ink one line after ruling it out. Anything below 0.55 alpha fails. The 0.28 value survives as `--border-subtle`, labelled decorative. | Caught by calculating rather than eyeballing, which is exactly why D5 made that the rule. | 10 §2.6, web/public/css/tokens.css |
| D17 | 2026-08-13 | **Built and deployed to the development server in one push**, per Steven's "continue all till finish". M1–M15: identity/RBAC/audit, master data and settings, catalogue and menu calendar, the four price forms, meal ordering, payments, packages and credits, eight reports, notifications, the server-rendered public site, the security suite, and a systemd + nginx deployment on `192.168.88.101:8090`. The API binds 127.0.0.1 and is unreachable from the LAN; ruuma is untouched on `:80`. | The build phase is where the work happens, not where the questions are asked (99 §2). Blockers were worked around and collected rather than stopping the build: the eight outstanding inputs are listed in `PROGRESS.md`. | PROGRESS, 12, 14, 15, 16 |
| D18 | 2026-08-13 | **Four classes of bug were only findable by running it**, and each is now guarded: (a) gorm maps `PriceIDR` to `price_id_r`, so every money field silently read as **zero** — explicit column tags plus `TestMoneyFieldsHaveExplicitColumnTags`; (b) a literal `?` inside a SQL string is consumed by gorm as a bind placeholder — the Maps URL's `?api=1` shifted every argument; (c) `html/template` refuses `Parse` after `Execute`, so a shared root template would have made the **first** email work and every later one fail; (d) an empty list serialised as JSON `null` rather than `[]`. | All four fail silently or far from their cause, which is exactly the class of thing that unit tests on pure functions cannot catch. Recorded so the next project checks for them. | 00, various |

---

## 3. Open questions

Answers go here as they land; each one that changes behaviour becomes a decision.

**Q1–Q6 are answered** by the brief of 2026-08-12 (D6, D7). The product questions
it raised in turn now live in **`03-open-questions.md`** — 29 of them, each with
the concrete answer needed and a default — and the choices it left open live in
**`02-decisions.md`**. This table keeps only what is still open here.

| # | Question | Status / default if unanswered |
|---|---|---|
| Q1 | **What is the product?** | ✅ answered — B2C healthy catering, Jakarta (D6) |
| Q2 | Is **Evermore** the customer-facing name? | ✅ yes; `healthy_catering` is the repo codename (D7) |
| Q3 | Currency | ✅ IDR, `BIGINT` whole rupiah (D7) |
| Q4 | Business timezone | ✅ `Asia/Jakarta` (D7) |
| Q5 | UI languages | ✅ `id-ID` default + `en` (D7) |
| Q6 | Production domain | ✅ `www.evermore.co.id` (D7) |
| Q7 | Can we have **page 13** of the brand guidelines ("Logo on Color Palette")? It is referenced by the colour page and was not supplied. | still open — proceed without it |
| Q8 | Is there a **reversed-out logo** for dark fills? The supplied mark is dark ink only and disappears on Nourish Green. | still open — derive one, as ruuma did. Now urgent: the brand's green header is on every page of M12 |
| Q9 | **Erode** licence terms for web embedding | ✅ closed — Fontshare Free Font EULA permits self-hosting; **must not be subset** (D15) |
| Q10 | **Which stack?** `CLAUDE.md` §3 (Go) vs the brief §2 (NestJS/Next.js) | ✅ closed — Go confirmed by `code start`, 2026-08-13 (D13) |
| Q11 | Do the four planning documents fold into the house numbering on approval, or keep their names? | fold, per `04-milestones.md` §2 |
