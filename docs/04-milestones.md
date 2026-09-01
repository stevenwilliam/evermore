# 04 — Milestones

**Version:** 0.1 — planning. **Not approved.**
**Date:** 2026-08-12

Build order, sliced so **every milestone ends in something you can be shown in a
browser**, not a layer that only makes sense with the next one attached. Each
milestone's gate is run before it is marked ✅ in `PROGRESS.md`, and a ✅ is
re-earned by running the gate, never inherited (`CLAUDE.md` §5).

Per `CLAUDE.md` §6 the plan runs A→Z without stopping to ask whether to continue.
The per-milestone checkpoint the brief asks for (§0.3 — tests, lint, changelog,
a ≤10-line summary) is a **report**, not a gate I wait behind.

---

## 0. Sequencing logic

Three constraints set the order:

1. **The pricing engine, the credit ledger and the kitchen router are the risk.**
   Each is pure domain code with near-100% coverage required (brief §14). They
   are built early and in isolation, before any UI depends on their shape.
2. **Geography comes before ordering.** An order cannot be placed without a
   serviceable address, so kitchens and routing precede the cart.
3. **Master data precedes the things that reference it.** Customer types before
   prices, diet types before schedules, slots before kitchens.

Blocked on answers: **M6** needs `Q-1` (tax), **M3** needs `Q-4` (real
kitchens) to be *verified* though not to be *built*, **M8** needs `Q-19` (bank
accounts) and `Q-13` (refund policy).

---

## 1. The milestones

Effort is a working estimate for one engineer, given the decisions in `02` land
as recommended. It is a range because `Q-2` (what a credit buys) can move M7 and
M9 materially.

| # | Milestone | Ships | Effort |
|---|---|---|---|
| **M0** | Foundation | Walking skeleton | 2–3 d |
| **M1** | Identity, RBAC, audit | Login and permissions | 3–4 d |
| **M2** | Settings & master data | The back-office shell | 3–4 d |
| **M3** | Kitchens & geography | "Do we deliver to you?" | 4–6 d |
| **M4** | Catalogue | Food pages with nutrition | 3–4 d |
| **M5** | Menu calendar | Staff schedule a week | 4–5 d |
| **M6** | Pricing engine | Four price forms + resolver | 5–7 d |
| **M7** | Meal ordering (Flow A) | Cart to unpaid order | 5–7 d |
| **M8** | Payment & finance | Verified, paid orders | 4–5 d |
| **M9** | Packages & credits (Flow B) | Buy credits, book meals | 6–8 d |
| **M10** | Fulfilment & reports | Kitchens and couriers work from it | 5–7 d |
| **M11** | Notifications | Email + WhatsApp | 3–4 d |
| **M12** | Public site & SEO | evermore.co.id | 5–7 d |
| **M13** | Hardening | ASVS L2 evidence | 4–6 d |
| **M14** | Handbooks | Deploy, user, admin guides | 3–4 d |

Roughly **11–15 weeks** of single-engineer work to a launchable phase 1.

---

### M0 — Foundation

**Ships:** repo layout per `D-3`; `sys_parameters`, config, structured logging
with request ids, `apierror`, UUIDv7, migration runner with `go:embed`, health
endpoint, Prometheus metrics, Docker satellites (Postgres extensions, Redis,
MinIO, mailpit), `make dev` one-command boot, CI running lint + tests +
`govulncheck` + `gosec`, OpenAPI 3.1 scaffold and the TS type generator.
`internal/platform/*` carried over from ruuma and adapted, not reinvented.

**Demo:** `make dev` boots the stack; `/healthz` is green; `make migrate-up` then
`migrate-down` cleanly round-trips; CI passes on a pull request.

**Gate:** migrations up and down on an empty database; CI green.

---

### M1 — Identity, RBAC, audit

**Ships:** users, six roles, permissions, argon2id, short-lived access + rotating
refresh tokens with a `jti` denylist, TOTP enrolment for staff (`Q-16`), email
verification, rate limiting on auth, Turnstile on registration, `audit_log` with
the before/after writer, and the **deny-by-default** permission middleware — every
handler declares its permission or does not compile into the router.

**Demo:** register a customer, verify by email (mailpit), log in; log in as
admin with TOTP; a staff role hitting an endpoint it lacks gets a typed 403; the
audit log shows a row per privileged action.

**Gate:** negative authorization test per role per resource; JWT tampering and
expiry tests; rate-limit test. These run in CI from here on.

---

### M2 — Settings and master data

**Ships:** the back-office shell (nav, tables, forms, **search box on every
list**), `sys_parameters` CRUD with secret masking, customer types,
organisations, diet types + subtypes, allergens, delivery time slots seeded to
`11:30 → Lunch` and `18:30 → Dinner`, staff user management.

**Demo:** an admin adds a customer type, edits the cut-off time, and adds a diet
type that then appears everywhere it should.

**Gate:** every list screen has a working search; a non-admin cannot reach any of
these screens; parameter changes appear in the audit log.

---

### M3 — Kitchens and geography

**Ships:** PostGIS; kitchen CRUD with a **Google Maps pin picker**, radius and
optional polygon service area, operating days/hours, slot mapping,
`kitchen_capacity` with defaults; the **router** as pure domain code;
`POST /api/v1/delivery-area/check`; the admin coverage map; the
`out_of_range_attempt` log; customer address CRUD with Places autocomplete,
draggable pin, "use my location", server-side bounds validation (`Q-11`), the
partial unique index for one default address, and soft delete.

**Demo:** the public **"do we deliver to you?"** widget answers with a kitchen
name and distance for a real Jakarta address, and offers "notify me" for one
outside coverage. The coverage map shows kitchens, radii and recent attempt pins.

**Gate:** the full router test matrix from `01-domain-model.md` §5.3 — including
polygon-overrides-radius and manual-assignment-not-overwritten — at near-100%
coverage. Coordinates absent from application logs (a test greps for them).

---

### M4 — Catalogue

**Ships:** foods, slugs, photo upload to a private bucket with magic-byte type
checking and re-encode, `food_nutrition` in integer milligrams, allergens,
diet-type mapping, and the nutrition-facts panel component.

**Demo:** staff create a dish with photos and a full nutrition panel; it renders
on a food page with kcal and protein badges in **`#1C3D34`, not the orange that
fails contrast** (`10-design-system.md` §2.4).

**Gate:** upload rejects a renamed executable and an oversized file; nutrition
arithmetic has no float anywhere.

---

### M5 — Menu calendar

**Ships:** `food_schedule` with `item_role`, month and week calendar views,
copy-week, duplicate-day, bulk publish, draft/published states, per-row
`qty_capacity`, the publish-horizon warning (`Q-17`), and the substitution flow
with its audit row (`Q-18`).

**Demo:** staff build next week's menu for three diet types across both slots,
copy it to the following week, and publish — and only then does it appear on the
public menu.

**Gate:** a customer-scoped query cannot return a `DRAFT` row (repository-level
test, not a handler test); un-publishing a row with deliveries is refused.

---

### M6 — Pricing engine · *blocked on `Q-1` (tax)*

**Ships:** `meal_price_tier` with gap/overlap validation; the four price tables
with `daterange` + `btree_gist` exclusion constraints and the generated
`scope_key`; the pure resolver (scope → DEFAULT → promo within scope); the
`price_resolution_trace`; **four separate admin forms** each with a
conflict-aware date picker that names the overlapping row rather than throwing;
and the D-9 warning that lists corporate scopes now dearer than a new DEFAULT
promo.

**Demo:** the same cart priced for `Customer Default` and for `Company A`, one in
promo and one not, with both prices shown and the trace explaining each. Then an
attempt to save an overlapping price row, refused with "overlaps with the price
valid 01–15 Aug".

**Gate:** the full resolver matrix from `01-domain-model.md` §5.1 at near-100%
coverage, plus a **concurrent double-submit test** proving the database — not the
application — rejects the overlap.

---

### M7 — Meal ordering, Flow A · *depends on `Q-2`, `Q-7`*

**Ships:** cart (multi-date if `Q-7` confirms), address selection per delivery,
tiered pricing display with struck-through normal price and promo label,
delivery-fee bands, the live **cut-off countdown** validated server-side, atomic
capacity decrement against both `food_schedule.qty_capacity` and
`kitchen_capacity` in one transaction with a fixed lock order, order creation
with an idempotency key, **price and food snapshots**, and per-delivery routing.

**Demo:** a customer orders three days of lunches to the office and one dinner
home, sees the tier price and the promo, and lands on an `AWAITING_PAYMENT`
order with the correct deadline and two different kitchens assigned.

**Gate:** oversell concurrency test on both capacity counters; cut-off enforced
against the server clock with a faked-browser-clock test; snapshot test proving a
later price edit does not move a historical order.

---

### M8 — Payment and finance · *blocked on `Q-19`, `Q-13`*

**Ships:** `bank_account` master, the unique-suffix allocator with its partial
unique index and fallback, payment instructions, proof upload to a private
prefix served only by presigned URL, the finance verification queue with filters
and rejection reasons, auto-expiry of unpaid orders releasing capacity, the
`PaymentProvider` interface with `ManualTransferProvider`, and refunds per
`Q-13`.

**Demo:** a customer uploads a transfer proof; finance verifies it; the order
becomes `PAID` and appears on the kitchen's list. A second order is left unpaid
and auto-expires, and its capacity comes back.

**Gate:** IDOR test — customer B cannot fetch customer A's payment proof by id;
suffix collision test; expiry job idempotent when run twice.

---

### M9 — Packages and credits, Flow B

**Ships:** package master with allowed diet types, the two package price tables,
purchase flow, `customer_package`, the **append-only `credit_ledger`**, slot
picking against the published calendar (with `D-27`'s expiry rule),
skip/pause/reschedule before cut-off with credit return, the expiry job posting
`EXPIRE` entries, staff extension with a compensating `ADJUSTMENT` and a required
reason, and the **customer credit report** with its movement drill-down.

**Demo:** buy a 20-credit package; finance verifies; the active period starts;
schedule five deliveries across both slots to two addresses; skip one before
cut-off and watch the credit return; an admin extends an expired package and the
ledger still reconciles.

**Gate:** the credit matrix from `01-domain-model.md` §5.2 at near-100% coverage,
including **two concurrent redemptions of the last credit** — one wins.

---

### M10 — Fulfilment and reports

**Ships:** delivery lifecycle and transitions, the courier role's manifest view
(`D-20`), staff manual reassignment with reason, and all eight reports —
production sheet per kitchen, packing labels, courier manifest ordered by
distance with Maps deep links, coverage/rejection, sales with promo impact,
unpaid/expiring, retention, plus the credit report from M9. Every report gets a
date-range filter, CSV/Excel export and a printable A4 layout.

**Demo:** print tomorrow's production sheet for one kitchen, the packing labels
for its deliveries, and the courier manifest — and actually cook and deliver from
them on paper.

**Gate:** report totals reconcile against the ledger and the order lines to the
rupiah, including the payment-suffix rounding (`D-16`).

---

### M11 — Notifications

**Ships:** the queue with retries, `NotificationChannel` with email and WhatsApp
implementations, all nine transactional templates in both languages,
per-customer channel preferences, `notification_log`, and the WhatsApp CS deep
link.

**Demo:** place an order and receive the instruction email and WhatsApp message;
support finds the send in the log.

**Gate:** no PII in logs; a failed send retries and is recorded, not lost.

---

### M12 — Public site and SEO

**Ships:** home, per-diet-type pages, the public **menu calendar** per diet type,
food detail pages, the TDEE calculator, the delivery-area widget, id-ID/en
message catalogues, and the whole SEO baseline — per-route title and
description, **OG tags static in the served HTML**, canonical URLs, one `<h1>`
per page, `robots.txt` disallowing the transactional surface, `sitemap.xml`, and
`Restaurant` + `Menu` JSON-LD.

**Demo:** paste a menu link into WhatsApp and get a correct preview card with a
photo — verified with `curl`, not a browser, per
`99-steven-preference.md` §13. Lighthouse ≥ 90 on mobile.

**Gate:** `curl -s <url> | grep -i 'og:\|<title'` returns the right tags per
route; the transactional surface is disallowed in `robots.txt`; **contrast
measured, not eyeballed**, against `10-design-system.md`; screenshots taken and
looked at, per `CLAUDE.md` §6.

---

### M13 — Hardening

**Ships:** the security suite the house rules require — negative authz per role,
IDOR per resource, rate limits, injection fuzz on every input, the concurrency
tests, cross-scope access, JWT tampering and expiry; security headers with a CSP
that has no `unsafe-inline`; UU PDP data-export and account-deletion flows with
PII redaction; `pg_dump` to object storage **with a restore actually performed**;
Sentry-compatible error reporting; menu-page caching with tag revalidation and
the Redis settings cache with write invalidation.

**Demo:** `docs/12-security.md` green — every OWASP Top-10 category mapped to a
control **and to the test that proves it**.

**Gate:** the whole suite in CI; a restore drill completed and timed, not
described.

---

### M14 — Handbooks

**Ships:** `14-production-deployment-handbook.md` (copy-paste, empty machine,
full absolute paths, `vi`), then `15-user-guide.md`, then `16-admin-guide.md` —
in that order, per `CLAUDE.md` §9.6.

**Gate:** the deployment handbook is followed on a clean machine and works, or it
says plainly which steps were not run.

---

## 2. Document set — a numbering collision to settle

The master prompt asks for `01-domain-model`, `02-decisions`,
`03-open-questions`, `04-milestones`. The house doc set (`99` §10) reserves
`01-PRD`, **`02-business-rules` (normative)**, `03-data-model`, `04-api-spec`.
Both cannot own those numbers, and `02` in particular is dangerous: someone
opening `02` expecting normative business rules would find a decision list.

No file collides *today* — none of the house documents exist yet. **Proposed
resolution on approval:** these four planning documents fold into the house set
and are then deleted, so the numbering is restored before any code is written.

| Planning doc | Folds into |
|---|---|
| `01-domain-model.md` | `03-data-model.md`, with the state machines in `02-business-rules.md` |
| `02-decisions.md` | the decision log in `00-README-and-decisions.md` (D6 onwards) |
| `03-open-questions.md` | `00-README-and-decisions.md` §3 |
| `04-milestones.md` | `08-roadmap.md` |

Then step 3 of `CLAUDE.md` §9 builds the full set A→Z: `01-PRD`,
`02-business-rules` (every rule with a `BR-x.y` id that code and tests reference),
`03-data-model`, `04-api-specification`, `05-architecture-and-nfr`,
`06-domain-operations`, `07-test-plan`, `08-roadmap`, `09-deployment`,
`11-local-dev-setup`, `12-security`, `13a-development-server-preparation`.

**Say the word if you would rather keep these four filenames as they are** — it
is your doc set. I have not renamed anything unilaterally.
