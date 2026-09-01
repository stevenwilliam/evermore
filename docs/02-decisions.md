# 02 — Decisions awaiting confirmation

**Version:** 0.2 — **confirmed by Steven on 2026-08-13** (`code start`).
Part A (the stack) is decided; the operational answers are in Part 0b.
**Date:** 2026-08-12

Every `[DECIDE]` in the master prompt, plus five conflicts between the prompt and
`CLAUDE.md` that the prompt could not have known about. Each carries a
recommendation, the reasoning, **what it costs if the answer is wrong**, and how
reversible it is — because reversibility is the only honest way to rank how hard
to argue about a decision.

Answer format, per `CLAUDE.md` §6: paste the list back with a `ven:` line under
any item you want to change. Silence on an item = the recommendation stands.
"all defaults" takes every recommendation below.

**Legend — reversibility:** 🟢 a later migration · 🟡 a refactor · 🔴 a rebuild.

---

## Part 0b — Answered by Steven, 2026-08-13

| # | Answer | Effect |
|---|---|---|
| **D-1…D-5** | **`code start`** — the stack is confirmed as recommended: Go + gin + gorm, numbered SQL migrations, Redis satellite, Go templates for public routes + React 18/Vite for the app. | Nothing to change. D10 in `00` moves from *decided by default* to decided. |
| **B2** | Kitchens operate **every day**, temporary data for now. | `kitchen_operating_day` gains weekday 7 (migration 0012). The two placeholder kitchens stay until real data arrives — the homepage widget is still answering from fiction, which is fine in development and not at launch. |
| **B3** | **PPN 11%**, changeable in the back office. | Already seeded at `1100` bps. PKP status and NPWP remain outstanding for the first real invoice. |
| **B4** | Dummy bank account, changeable later. | Account holder now says `PT EVERMORE (DUMMY — REPLACE BEFORE LAUNCH)` so nobody mistakes it for real. |
| **B5** | Google Maps keys deferred. | The address pin picker cannot be exercised end to end until they exist. Coordinates can still be set by staff, audit-logged, which is the documented escape hatch anyway (D-17). |
| **B6** | **Free delivery, every distance, every order value** — but configurable. | One open-ended band at zero. The fee engine still runs on every order and every report, so turning charging on is a settings edit and not a code path that has never executed. |
| **B7** | SMTP borrowed from ruuma, changeable in the back office. | ruuma's mailpit satellite on `127.0.0.1:1025`. Mail settings **moved from env into `sys_parameters`**; the password is secret-flagged and masked, and the env var still overrides so a production secret need never be in the database. |
| **B8** | **WAHA.** | Matches 99 §9. The shared container on `127.0.0.1:3000` is already running. Left switched **off** until a sender number exists. The tradeoff is recorded: WAHA is free and unofficial, and a banned number takes the channel down mid-service. |
| **B9** | Local, by IP, different port from ruuma. | ruuma is on 8080; Evermore is on 8081. No conflict. |
| **B10** | Erode is free from Fontshare — self-host it. | Done, with Inter. **Not subset** — see D15 in `00`. The reversed-out logo question is still open. |
| **B11** | A cart **may span several dates**, tier on the order's total. | Mon–Fri × 2 meals = 10 meals, reaching the 10–19 tier. More generous than per-date, and the interpretation customers assume. |
| **B12** | **One cut-off** for both slots. | 18:00 on D-1. The per-slot override columns stay in the schema unused, so tuning dinner later is a settings change. |
| **B13** | **No minimum order.** | Seeded `order.min_qty = 1`, `order.min_value_idr = 0`. |


## Part 0c — Steven's answers, 2026-08-13 (second batch)

Recorded verbatim in intent, with what each one cost to implement. All of it is
data or settings, not schema — which is the point of having put these in
`sys_parameters` and seed rows in the first place. Landed as migration `0014`.

| Question | Answer | Consequence |
|---|---|---|
| Real kitchens | Siloam Lippo Village, Siloam MRCCC, Ruuma Cideng, Siloam TB Simatupang, Siloam Kebon Jeruk | Five kitchens replace the two placeholders, which are **deactivated not deleted** — nine deliveries reference them. Coordinates came from the Geocoding API using the real key, and each row records the address it resolved to. |
| Service radius | 20 km, all the same | Bekasi is now genuinely served (17.6 km from MRCCC). Bogor and Bandung are correctly refused. |
| Daily capacity | remove | `default_daily_capacity` stays NULL. It was never the oversell control — `scheduled_meal.qty_capacity` is, per date + diet + slot, and that is the row a booking locks. A number on the kitchen would have looked like a limit while enforcing nothing. |
| Contact | single number 08176315568, plus a floating WhatsApp button on the home page | One `company.phone`/`company.whatsapp`. The button is a plain anchor — the CSP has no `unsafe-inline`, so an inline handler would not run. |
| Opening hours | every day, no closing time | Seven `kitchen_operating_day` rows each, `00:00`–`23:59`. Nothing reads these columns yet; the 18:00 cut-off is a separate setting and is unaffected. |
| Bank | Nobu · 16830226665 · PT Sunshine Food International · Menara Matahari, with a copy button | Dummy deactivated. The number is rendered large and monospaced with a copy control, because it is the field customers mistype. |
| Legal entity | PT Sunshine Food International, Menara Matahari 2nd Floor, Lippo Karawaci | Evermore stays the customer-facing brand; the PT is the entity on the invoice and the bank account. |
| NPWP | `123 123 123` | ⚠️ Stored as given and editable, but a real NPWP is 15 digits (16 since NIK). **Must not reach a faktur pajak.** PKP status was not answered, and without it the 11% PPN cannot be confirmed as permitted. |
| Domain | `dev.evermore.co.id` now, production later | nginx `server_name` and `APP_BASE_URL` updated; `_` retained so the bare IP keeps working while DNS is unset. |
| Logo | keep the wordmark, fine-tune later | No change. |
| WhatsApp | use WAHA, 08176315568 | Wired to the WAHA container **shared with ruuma**, whose session is already bound to 628176315568. Channel switched on; the session reports `FAILED`, so messages queue and retry until it is re-linked. |

### D-35 — the copy control had to carry its own fallback

`navigator.clipboard` is undefined outside a secure context, and the dev host is
plain HTTP. A copy button that silently does nothing is worse than no button:
the customer believes they are holding the account number. `copyText` falls back
to a `document.execCommand` path and the control **announces** the outcome
through `role="status"`, so failure is stated rather than merely uncoloured.

This is the third time this class of bug has appeared — after `crypto.randomUUID`
in checkout. Anything reaching for a browser API guarded by secure context needs
a fallback or an explicit refusal, not an optional chain.

### D-36 — the WhatsApp button is not WhatsApp green

`#25D366` measures **1.98:1** against a white glyph and **1.89:1** against our
beige page: it fails WCAG 1.4.11 twice, once for the icon and once for the
control being distinguishable at all. `#128C7E` — WhatsApp's own darker brand
teal — measures 4.14:1 and 3.94:1 and is used instead. Calculated, not eyeballed.

---

## Part 0 — Answered by Steven, 2026-08-12

Four answers landed, and the last one changed the model. Recorded here as
settled; the reasoning that follows from each is in `01-domain-model.md`.

| # | Answer | What it settles |
|---|---|---|
| **D-30** | *"All price is included price, however in database split base price and tax; tax percentage can be changed via backend."* | **Prices are tax-inclusive.** The four price tables hold only the inclusive number. The **split is computed and snapshotted per order**, not stored on the price row — otherwise a rate change from 11% to 12% rewrites every price row, and old rows carry a rate they were never sold under. `tax_rate_bps` is a `sys_parameters` row. Integer half-up back-calculation on the **line total**, with tax taken as the residue so base + tax always equals the price exactly. Delivery fee is taxed the same way; the payment suffix is not taxable. (`01` §3.11) |
| **D-31** | *"No refund."* | **No money goes back.** A staff-cancelled paid order and an unfulfilled delivery are compensated in **credits** (`ADJUSTMENT` with a reason), never rupiah. Unused credits are **forfeited at expiry** — which must be on the terms page before launch, not explained at the first complaint. `REFUNDED` stays on the machine for **admin-only correction of an erroneous or duplicate transfer**, because manual bank transfer guarantees that case and finance moving money with no record of it is worse than modelling it. |
| **D-32** | *"1 meal contains several food. 1 credit is used for 1 meal (even 1 food delivery for package)."* | **The meal is the unit of sale**, not the food. `scheduled_meal` (date + diet type + slot) replaces the brief's flat `food_schedule`; `scheduled_meal_item` holds its foods with their roles. Capacity, publication and credit redemption all attach to the **meal**. One credit = one meal whatever it contains. Tier quantities count meals. Customers pick a meal, never individual dishes. |
| **D-33** | *"Each food can have nutrition facts; meal nutrition will be aggregated from food."* | **Meal nutrition is computed, never typed** — `SUM` over the meal's foods, in pure domain code, snapshotted onto the order line at purchase. This is what makes D-24 (integers in milligrams) load-bearing rather than tidy: summing integers is exact, and a few mg of drift per dish is visible on the weekly intake chart. |
| **D-34** | *"Auto assign location customer to nearest kitchen."* | Confirms the router. Reconciled with the brief's own §9.3 ranking by **seeding every kitchen at the same `priority`**, so ranking collapses to nearest-first exactly as asked, while `priority` remains the manual override for the "3 km away but across the toll road" case. **This answers the rule, not the data** — the real kitchen list (Q-4) is still needed before launch and has moved to the operational-inputs table in `03`. |

---

## Part A — Stack. These five conflict with `CLAUDE.md` and must be settled first

`CLAUDE.md` is the standing contract and says Go + gin + gorm, React 18 + Vite,
numbered SQL migrations, no SSR frameworks. The master prompt §2 proposes
TypeScript + NestJS + Prisma + Next.js, and marks the whole table `[DECIDE]`
with "swap-able — tell me if you prefer Go or Laravel". So the prompt invites
exactly this conversation rather than overriding the contract silently.

### D-1 — Backend language and framework 🔴

| | Go + gin + gorm (`CLAUDE.md`) | TypeScript + NestJS + Prisma (prompt §2) |
|---|---|---|
| Contract | ✅ the standing house stack | ❌ needs `CLAUDE.md` §3 amended |
| Money as integers | native `int64`, no footguns | `bigint` → JS `BigInt`, and every JSON boundary is a chance to turn a rupiah into a float |
| PostGIS + `daterange` + `EXCLUDE` | raw SQL, which the house rules already mandate on money paths | Prisma has **no first-class support** for `daterange`, exclusion constraints or PostGIS — all of it becomes `$queryRaw` and unsafe casts, so the ORM's main benefit evaporates precisely where this product is hardest |
| Shared validation with the web app | ❌ not possible — see D-3 | ✅ Zod shared through `packages/shared` |
| Ops | one static binary | Node runtime, `node_modules`, heavier image |
| Portable `platform/*` from ruuma | ✅ already written and proven | ❌ rewrite |

**Recommendation: Go + gin + gorm.** The decisive argument is not the contract,
it is that this product's three hard parts — the exclusion-constraint price
tables, the PostGIS router and the money arithmetic — are the three things Prisma
handles worst. Choosing Prisma means writing them in raw SQL anyway, having paid
the ORM's cost. Go also lets `internal/platform/*` be carried over from ruuma
rather than reinvented (`CLAUDE.md` §2).

**Cost if wrong:** if the team that maintains this is a TypeScript team and you
have no Go people, that outweighs everything above. It is a hiring question, not
a technical one, and only you can answer it.

### D-2 — Frontend, and how the SEO requirement is actually met 🔴

The brief needs SSG/SSR for menu + marketing (§2, §13.3, §14), and
`99-steven-preference.md` §13 is blunt about why: **link-preview bots do not run
JavaScript**, so a pure SPA shows a blank card when a customer pastes a menu link
into WhatsApp. That is the single highest-value SEO item for an Indonesian B2C
business, and it has nothing to do with Google. Three ways to satisfy it:

| | **C — Go templates + Vite SPA** (recommended) | A — Next.js 14 (App Router) | B — Vite SPA + build-time prerender |
|---|---|---|---|
| Public pages | server-rendered by the Go binary | Next SSG/ISR | prerendered at build |
| App + back office | React 18 + Vite SPA | same Next app | same SPA |
| OG tags | always correct, always fresh | always correct | stale between builds |
| Daily menu freshness | automatic — rendered from the DB | ISR + tag revalidate on publish | needs a rebuild per publish |
| Production units | **1** (Go binary + static assets) | 2 (Go binary + Node) | 1 |
| `CLAUDE.md` §3 | ✅ complies | ❌ SSR framework, needs amendment | ✅ complies |
| React 18 pin | ✅ | ✅ Next 14 runs React 18 | ✅ |
| Cost | design system expressed twice — Tailwind in templates *and* in React | one codebase, one mental model | prerender pipeline is fiddly and silently rots |

**Recommendation: C.** The public surface is small and mostly static in shape —
home, one page per diet type, the menu calendar, food detail, TDEE calculator,
"do we deliver to you?", legal pages. Everything transactional (cart, checkout,
account, all of the back office) is authenticated, must never be indexed, and
gains nothing from SSR. Rendering those eight public routes from the Go binary
gives perfect, always-fresh OG tags with **no second runtime in production** and
no `CLAUDE.md` amendment. The real cost is duplicating the design system across
Go templates and React components; shared Tailwind config and design tokens keep
that to markup rather than decisions.

**Pick A instead if** you want one frontend codebase and one team, and are happy
to run Node in production and amend `CLAUDE.md` §3. It is the conventional
answer and nobody would call it wrong — it just buys convenience with a second
deploy unit.

**Cost if wrong:** low either way; the API is the source of truth (prompt §1) so
the frontend is replaceable. This is the least dangerous of the Part A five.

### D-3 — Repo layout and shared validation 🟡

The prompt wants a monorepo with `packages/shared` holding Zod schemas used by
both API and web. **That only works if the API is TypeScript.** If D-1 lands on
Go, shared runtime validation is impossible and the replacement is:

**OpenAPI 3.1 generated from the Go handlers → TypeScript types + a Zod schema
generated for the web app.** The prompt already requires OpenAPI 3.1 from day one
for the phase-2 mobile team, so this costs one generator step and gives the same
guarantee: one contract, no hand-copied types.

**Recommendation:** house layout from `CLAUDE.md` §2 (`cmd/`, `internal/`, `db/`,
`web/`, `docs/`, `deploy/`), plus `api/openapi.yaml` and a `make generate` that
produces `web/src/lib/api/` types. Not `apps/api` + `apps/web`.

### D-4 — Redis 🟢

Not in `CLAUDE.md`'s stack, and the house rule is "Docker is for the satellites"
(§9) — Redis qualifies as a satellite. The prompt wants it for config cache, rate
limits, job queue and idempotency keys.

**Recommendation: yes, one shared Redis container**, used for the settings cache,
rate limiting and the notification/job queue. **But idempotency keys go in
Postgres**, not Redis: an idempotency key protects an order that creates money,
and it must survive a Redis restart and be in the same transaction as the write
it guards. A key that evaporates on eviction is worse than none, because it looks
like protection.

### D-5 — Prisma/Drizzle vs numbered SQL migrations 🟡

**Recommendation: numbered forward-only SQL migrations** (`NNNN_name.up.sql` +
`.down.sql`, `go:embed`), which is `CLAUDE.md` §4 and also the only way to
express the exclusion constraints, generated `geography` columns and partial
unique indexes this design leans on. The prompt's own §0.5 — "every schema change
goes through a migration file, no manual SQL on a live DB" — agrees.

---

## Part B — Domain decisions from the prompt

### D-6 — `ingredient` table: build now or later? (§4.3) 🟢

**Recommendation: later — model now, build in phase 2.** Auto-computed nutrition
is genuinely better data, but it requires per-100g nutrition for every
ingredient *and* per-dish gram quantities, which is a data-entry project the
kitchen has to staff before a single food page can go live. Phase 1 types the
panel per dish (nine integer columns, already in the model). The tables are
designed so adding them later is additive.

### D-7 — One food per date+diet+slot, or several? (§4.4) 🟢

**Recommendation: several, with an `item_role` column** (`MAIN`, `SIDE`,
`DESSERT`, `DRINK`) — as the prompt itself recommends. Catering menus are
composed, the packing label and production sheet both need the components
listed separately, and collapsing a set menu into one `food` row would make the
nutrition panel a lie. Unique on (date, diet, slot, food) still prevents
scheduling the same dish twice in one sitting.

### D-8 — Menu global or per kitchen? (§9.5) 🟢

**Recommendation: global.** One city-wide menu; kitchens differ only in
capacity and coverage. Per-kitchen menus multiply the staff's daily scheduling
work by the number of kitchens for a benefit no one has asked for yet. Adding
`kitchen_id` to `food_schedule` later is an additive migration with a backfill
of the existing rows to "all kitchens".

### D-9 — Cross-scope promo interaction (§5.2) 🟡 — **the one that changes what customers pay**

The case: `Company A` has a *normal* price of Rp 55.000. There is a *promo* on
`DEFAULT` at Rp 45.000. What does a Company A customer pay?

**Recommendation: resolve scope first, then promo within that scope — Company A
pays 55.000.** As the prompt proposes. The reasoning that matters: a corporate
scope exists because that price was *negotiated*, usually in a contract, and a
public promo is a customer-acquisition tool aimed at people who are not on a
contract. Letting a DEFAULT promo undercut a negotiated rate means every public
discount silently reprices every corporate account, which finance will discover
at invoice time.

**Flagging the consequence you did not ask about:** this rule means a corporate
customer can pay *more* than a walk-in during a promo, and they will notice.
The clean fix is operational, not architectural — when a promo is created on
`DEFAULT`, the admin form shows which corporate scopes are now more expensive
than the public price and offers to create matching promo rows for them. One
screen, no schema change. I recommend building that warning into the promo form
in the same slice.

### D-10 — Tier semantics: flat or marginal? (§5.4) 🟡

**Recommendation: flat.** The whole order is priced at the rate of the tier its
total quantity lands in. It is what Indonesian catering customers expect, it is
one line to explain on the cart page, and marginal pricing produces a cart where
no displayed unit price matches any line — a support burden out of proportion to
the few rupiah it saves. Documented in `02-business-rules.md` as normative once
approved.

### D-11 — Is the meal price per diet type? (§5.4) 🟢

**Recommendation: yes, per diet type.** High Protein and Weight Loss have
different food costs, and a single price across diets removes the ability to
price them apart forever without a migration. If today's commercial answer is
"same price for all", that is expressed as identical rows per diet type — seeded
in one action by the admin form — not as a missing column.

### D-12 — Are packages restricted to certain diet types? (§5.5) 🟢

**Recommendation: restricted, via a `package_diet_type` join table, where **no
rows means "any diet type"**.** This gives both behaviours without a flag, lets
you sell a "Weight Loss 20-pack" and a generic "20-pack" side by side, and costs
one table.

### D-13 — Payment deadline (§6.1) 🟢

**Recommendation: 2 hours, but hard-capped at the order cut-off.** Both values
live in `sys_parameters`. The cap is the part the prompt does not mention and
matters: an order placed at 17:30 for tomorrow with a flat 2-hour deadline would
expire at 19:30 — 90 minutes *after* the 18:00 cut-off it was placed against,
having held capacity through the entire window in which someone else could have
bought it. Deadline = `min(now + 2h, cut-off)`, and if that leaves under 15
minutes the checkout says so before payment rather than after.

### D-14 — When does a package's active period start? (§6.2) 🟢

**Recommendation: on payment verification**, as the prompt proposes.
Manual bank transfer means verification can lag the purchase by hours or a
weekend, and charging the customer validity days for time they could not order
in produces exactly the support ticket that ends in a free extension. Both
timestamps are stored (`purchased_at`, `activated_at`); only `activated_at`
drives `expires_at`.

### D-15 — Order states vs delivery states (§6.3) 🟡 — **not in your list; I am raising it**

§6.3 puts `SCHEDULED → PREPARING → OUT_FOR_DELIVERY → DELIVERED` on the *order*.
A package order produces twenty deliveries over a month, to different addresses,
through different kitchens — it cannot be "out for delivery".

**Recommendation:** the order owns the commercial lifecycle
(`DRAFT → AWAITING_PAYMENT → PAYMENT_SUBMITTED → PAID → COMPLETED`, plus
`EXPIRED`/`CANCELLED`/`REFUNDED`); the delivery owns fulfilment, with exactly the
names §6.3 gives. The API still exposes an order-level `fulfilment_status`,
**derived** from its deliveries, so nothing in the brief is lost and the mobile
team gets the field they would expect.

### D-16 — Unique 3-digit transfer suffix (§10) 🟢

**Recommendation: yes.** Standard Indonesian practice and it turns payment
matching from eyeballing a bank statement into a lookup. Two consequences worth
stating now:

- The suffix must be **unique among currently-unpaid orders**, not globally —
  enforced by a partial unique index on (`payment_amount_idr`, `bank_account_id`)
  `WHERE status IN ('AWAITING_PAYMENT','PAYMENT_SUBMITTED')`. With 999 suffixes,
  collisions start mattering at a few hundred concurrent unpaid orders; when the
  index rejects, the system retries a different suffix and, if none is free,
  falls back to no suffix and flags the order for manual matching.
- **The amount transferred is then not the sum of the lines.** The difference is
  stored as `payment_rounding_idr` on the order so the sales report reconciles to
  the bank statement to the rupiah instead of drifting by up to Rp 999 per order.

### D-17 — Places unavailable: manual lat/lng, or block? (§8.2) 🟢

**Recommendation: block, with a WhatsApp CS link**, as the prompt proposes.
A hand-typed coordinate is the single most likely source of a delivery sent to
the wrong side of Jakarta, and the router treats coordinates as ground truth.
Staff *can* set coordinates from the back office (audit-logged) so CS can rescue
a stuck customer — that keeps the escape hatch behind someone accountable.

### D-18 — Distance Matrix travel time vs straight-line (§9.2) 🟢

**Recommendation: straight-line now, travel time later**, as proposed. Add the
cache table (`kitchen`, `address`, `duration_s`, `distance_m`, `fetched_at`) in
the same milestone as routing so the upgrade is a resolver swap, not a schema
change. Ranking is `priority → distance → remaining capacity`, and `priority` is
the manual override that handles the "the map says 3 km but it is across the
toll road" case until travel time arrives.

### D-19 — Delivery fee: charged in phase 1? (§9.6) 🟢

**Recommendation: yes, distance bands from the *assigned kitchen*, with free
delivery above an order-value threshold.** Bands and threshold in
`sys_parameters`, seeded 0–5 km free · 5–10 km Rp 15.000 · >10 km Rp 25.000,
free above Rp 300.000 — **all four numbers are placeholders and need your real
ones** (`03-open-questions.md` Q-3). Building the fee engine now and seeding it
to all-zero is a config change; retrofitting it after launch touches the cart,
the order, the invoice and every report.

### D-20 — Courier role in phase 1? (§3) 🟡

**Recommendation: yes, but minimal.** The courier manifest (§12.4) is needed on
day one regardless. What "courier role" adds is a login that sees only today's
manifest for one kitchen and can mark delivered. That is one list screen, two
endpoints and a role row — small. Without it, someone in the office marks
twenty deliveries delivered from WhatsApp messages, and delivery timestamps
become fiction, which then corrupts the retention report.

### D-21 — Staff scoped to one kitchen in phase 1? (§12) 🟢

**Recommendation: yes, phase 1.** `kitchen_id` nullable on `staff_profile`
(null = all kitchens), enforced as a repository-level filter, not a UI one —
`CLAUDE.md` §4's tenant-scoping rule applied to kitchens. Adding the column later
is trivial; adding the *filter* later means auditing every query in the back
office, which is exactly the kind of change that misses one.

### D-22 — WhatsApp notifications in phase 1? (§11) 🟡

**Recommendation: yes, behind the `NotificationChannel` port, with email as the
only channel switched on at launch.** Indonesian customers read WhatsApp and
ignore email, so a delivery reminder by email alone will underperform. But note
a conflict with `99-steven-preference.md` §9, which specifies **WAHA**
(self-hosted, one shared container) rather than the Cloud API or Fonnte/Wablas.

**Recommendation on the provider:** WAHA in development, and for production
decide between WAHA (cheap, unofficial, account-ban risk on a business number)
and the **Meta Cloud API** (per-conversation cost, template pre-approval, no ban
risk). For a business whose order confirmations are the message, I recommend the
**Cloud API in production** — a banned number takes the entire notification
channel down mid-service. Templates need submitting ~1 week before launch, which
is a schedule item, not a code item.

### D-23 — §13 extras: what is in phase 1? 🟢

"Propose, don't build without approval." Proposed split, ordered by value per
unit of work:

| # | Extra | Phase | Why |
|---|---|---|---|
| 3 | Public menu calendar page (SSG per diet type) | **1** | The SEO asset. Falls out of D-2 almost free. |
| 1 | Calorie/TDEE calculator | **1** | Pure client-side arithmetic, no schema. Best lead magnet per hour spent. |
| 10 | WhatsApp CS deep link on every order page | **1** | One `sys_parameter` and a link. |
| 9 | id-ID + en i18n, default `id-ID` | **1** | Message catalogues from the first string, per house rules. Retrofitting means touching every component. |
| 2 | Allergen & dislike profile with clash warnings | **1 (warn only)** | Data already exists. **Warn, never hide** — hiding a dish silently makes the menu look thin. |
| 6 | Corporate invoicing | **1 = data model, 2 = UI** | `organisation` exists now; monthly consolidated invoicing is real work and probably where the revenue is. |
| 4 | Auto-renew reminder + one-click repurchase | 2 | Needs a repeat-purchase population first. |
| 5 | Referral + voucher codes | 2 | A separate discount layer applied *after* price resolution, never in the four price tables. |
| 7 | Per-meal rating & feedback | 2 | Needs delivered volume to be worth reading. |
| 8 | Customer nutrition summary chart | 2 | Sticky, but only after weeks of delivered meals exist. |

---

## Part C — Decisions this planning pass forced, for the record

| # | Decision | Rationale |
|---|---|---|
| D-24 | Nutrition stored as **integers in milligrams** (kcal whole) | Same reason money is integers; the weekly-intake chart then sums exactly. Display divides at the edge. |
| D-25 | `scope_key` is a **generated column** from `customer_type_id` | `NULL <> NULL` in an exclusion constraint, so two `DEFAULT` rows would both be accepted. The text key is what makes §5.3's constraint actually hold. |
| D-26 | Credit balance is **never stored**, only summed | The prompt's §7 rule, made explicit: `credit_ledger` has no `UPDATE`/`DELETE` path, stated in its migration. |
| D-27 | Delivery date must be **≤ package `expires_at`** | Otherwise a credit redeemed on the last valid day schedules a delivery a month later and the package never really expires. |
| D-28 | Extending an expired package posts a **compensating `ADJUSTMENT`** | The `EXPIRE` entry is never deleted; the ledger stays append-only and reconcilable. |
| D-29 | Public-page contrast follows `10-design-system.md` §2.4–2.6 | Nutrition badges are the risk: `#E0782D` on beige is 2.90 and fails. kcal/protein badges use `#1C3D34`, or `#FFBC8F` on the dark green. |
