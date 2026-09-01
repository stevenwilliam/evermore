# Evermore — Master Prompt for Claude Code

> Steven's master brief, received 2026-08-12, stored verbatim as the source of
> truth for `01-domain-model.md`, `02-decisions.md`, `03-open-questions.md` and
> `04-milestones.md`. Sections marked **[DECIDE]** are open choices — Claude must
> ask before assuming; each is answered in `02-decisions.md`.

---

## 0. Role and working method

You are the lead engineer for **Evermore**, a B2C healthy-catering ordering website
(`www.evermore.co.id`), serving Jakarta, Indonesia.

Work like this, and do not skip ahead:

1. **Do not write application code yet.** First read this document end to end and produce:
   - `docs/01-domain-model.md` — entities, relationships, state machines, ERD in Mermaid
   - `docs/02-decisions.md` — every **[DECIDE]** item with your recommendation and reasoning
   - `docs/03-open-questions.md` — anything ambiguous, with the concrete answer you need
   - `docs/04-milestones.md` — build order, sliced so each slice is demoable
2. **Stop and ask me to confirm** those four documents before generating code.
3. Then build milestone by milestone. After each milestone: run tests, run lint,
   update `docs/CHANGELOG.md`, and summarise what changed in 10 lines or fewer.
4. Never invent business rules silently. If this document is ambiguous, add it to
   `docs/03-open-questions.md` and ask.
5. Every schema change goes through a migration file. No manual SQL on a live DB.

---

## 1. Product scope

**Phase 1 — website only.** Public marketing + menu pages, customer account,
ordering, payment, and a full staff back office. **No PWA, no service worker.**

**Phase 2 — later.** Native Android + iOS apps.
→ Therefore: build a **versioned REST API (`/api/v1`) as the single source of truth**,
with the website as its first client. No business logic in the frontend. Document the
API with OpenAPI 3.1 from day one so the mobile team can generate clients.

---

## 2. Technical stack **[DECIDE — confirm with me first]**

Proposed, chosen to match existing infrastructure:

| Layer | Choice | Notes |
|---|---|---|
| Database | PostgreSQL 16 | `btree_gist` (see §5.3) **and `postgis`** (see §9) required |
| Maps | Google Maps JavaScript + Places + Geocoding API | address pin capture, kitchen coverage map |
| Cache / jobs | Redis 7 | config cache, rate limits, job queue, idempotency keys |
| Backend | TypeScript + NestJS + Prisma (or Drizzle) | swap-able — tell me if you prefer Go or Laravel |
| Frontend | Next.js (App Router) + TypeScript + Tailwind | SSR/SSG needed for SEO on menu + marketing pages |
| Files | S3-compatible bucket | food photos, payment proofs (private prefix) |
| Mail | SMTP relay :587 | transactional email |
| Deploy | Docker image + Caddy (auto Let's Encrypt) | Ubuntu LTS, Jakarta region |
| Timezone | `Asia/Jakarta` everywhere | store UTC, render WIB |
| Money | `BIGINT` minor-unit-free IDR (whole rupiah), never float | format `Rp 500.000` |

Repo layout: monorepo — `apps/api`, `apps/web`, `packages/shared` (types, validation
schemas shared between API and web), `docs`, `infra`.

---

## 3. Users and roles

- `customer` — self-registered
- `staff` — menu, food schedule, customer type, help customers pick schedules
- `finance` — verify payments, refunds, reports
- `kitchen` — production sheets, mark meals prepared
- `courier` — delivery manifest, mark delivered **[DECIDE — phase 1 or later?]**
- `admin` — all of the above + system settings + user management

Enforce RBAC in the API layer, not just the UI. Staff accounts require 2FA (TOTP).
Every staff write action that touches money, prices, customer type, credits, or
package expiry must write an **audit log** row (actor, action, entity, before, after, IP, timestamp).

---

## 4. Core domain

### 4.1 Customer types
Table-driven, not an enum. Seed: `Customer Default`, `Siloam Customer`, `Company A`,
`Company B`. Admin can add more. New registrations get `Customer Default`.
Only staff/admin can change a customer's type — and that change is audit-logged.

Corporate types (`Siloam`, `Company A/B`) imply the customer is tied to an organisation.
Model an optional `organisation` entity now (name, PIC, billing email, PO number,
`is_invoice_billing`) so corporate invoicing in §13 doesn't need a migration later.

### 4.2 Diet types
Table-driven, admin-manageable. Seed: `Healthy`, `Weight Gain`, `Weight Loss`,
`High Protein`, `Special Diet`. Special Diet has sub-categories (`Diabetic`,
`Cholesterol`, others) — model as `diet_type` → `diet_subtype` (nullable) rather than
hard-coding.
Each diet type has: name, slug, description, hero image, sort order, `is_active`.

### 4.3 Food
`food`: name, slug, description, photos[], diet types it can belong to, allergens[],
`is_active`, portion size.
`food_nutrition` (per food, per portion): calories (kcal), protein, fat, saturated fat,
carbohydrate, sugar, fibre, sodium, cholesterol, plus free-form extras as JSONB.
Show a proper nutrition-facts panel on the food page.

**Recommended:** an optional `ingredient` table with per-100g nutrition, so nutrition
facts can be auto-computed and kept consistent instead of typed per dish.
**[DECIDE — build now or later?]**

### 4.4 Food schedule (the menu calendar)
`food_schedule`: `date`, `diet_type_id`, `meal_slot` (lunch/dinner), `food_id`, `qty_capacity` (nullable).
Unique on (date, diet_type, meal_slot, food) — and **[DECIDE]** whether more than one
food may be scheduled for the same date+diet+slot (a set menu of main + side + fruit)
or exactly one. I recommend allowing multiple, with a `role` column
(`main`, `side`, `dessert`, `drink`), because catering menus are almost always composed.

Staff need a **calendar UI** (month + week view) with copy-week, duplicate-day, and
bulk-publish. A schedule row is `draft` until published; customers only see `published`.

---

## 5. Pricing engine — the hardest part, get this right

There are two purchase modes: **meals** (à la carte, tiered by qty) and **packages**
(prepaid meal credits with an active period).

### 5.1 Price scope
Every price row is scoped by a **price scope** = either a specific `customer_type_id`
or the literal `DEFAULT`. Resolution:

1. Look for a price whose scope = the customer's customer type.
2. If none exists for that date, fall back to scope = `DEFAULT`.
3. If neither exists → **block the purchase** with a clear error; never guess a price.

### 5.2 Normal vs promotional price
- `meal_price_normal`, `meal_price_promo`, `package_price_normal`, `package_price_promo`
  are **four separate tables** with **separate admin forms** (explicit requirement).
- If a promo price exists for the resolved scope and date, it **overrides** the normal price.
- The UI must show **both**: normal price struck through, promo price highlighted,
  plus promo label and validity ("Promo until 31 Aug").
- **[DECIDE]** Cross-scope interaction: if `Company A` has a *normal* price and there is
  a *promo* price only on `DEFAULT`, which wins? My recommendation: **resolve scope first
  (customer type, then DEFAULT), then apply promo within that scope** — so Company A's
  normal price wins over the DEFAULT promo. Confirm this, because it materially changes
  what customers pay.

### 5.3 No date collision — enforce in the database
For each price table, the pair (scope, tier-or-package) may have **at most one active
row on any given date**. Do not rely on application checks alone — a concurrent
double-submit will beat them.

Use a validity period as a `daterange` plus a PostgreSQL exclusion constraint:

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE meal_price_normal
  ADD CONSTRAINT meal_price_normal_no_overlap
  EXCLUDE USING gist (
    scope_key      WITH =,   -- 'CT:<uuid>' or 'DEFAULT'
    diet_type_id   WITH =,
    tier_id        WITH =,
    validity       WITH &&   -- daterange, inclusive-exclusive '[)'
  );
```

Same pattern for the promo table (scoped to itself — promo *is* allowed to overlap
normal, that's the point) and for both package price tables keyed by `package_id`.
Open-ended prices use `daterange(valid_from, NULL)`. On save, surface the conflicting
row to the admin ("overlaps with price valid 01–15 Aug"), don't just throw a 500.

### 5.4 Meal tiering
`meal_price_tier`: `min_qty`, `max_qty` (nullable = ∞), label. Tier ranges must not
overlap and must not leave gaps from 1 to the max order qty — validate on save.

**[DECIDE]** Tier semantics. **Flat (recommended):** the whole order qty is priced at
the rate of the tier the total qty falls into (10 meals × tier-2 rate). **Marginal:**
first 4 at tier-1 rate, next 6 at tier-2 rate. Flat is what Indonesian catering
customers expect and is far easier to explain on the cart page. Pick one and document it.

**[DECIDE]** Is the meal price per diet type, or one price for all diet types?
The schema above assumes per diet type — confirm.

### 5.5 Packages
`package`: name, `meal_credits` (qty), `validity_days` (active period), allowed diet
types **[DECIDE — restricted or any?]**, `is_active`, description.
Price comes from the package price tables using the same scope + promo + no-overlap rules.

### 5.6 Price snapshotting
When an order is placed, **copy** the resolved unit price, the price row id, the normal
price, the promo flag, the food name and the nutrition facts onto the order line.
Historical orders must never change when someone edits a price later. This is
non-negotiable.

---

## 6. Ordering

- Max order qty per line: **999**, stored in system settings, editable in backend.
- **Cut-off:** orders for delivery date *D* close at **18:00 (WIB) on D-1**. Both the
  time and the lead-day count live in system settings. The same cut-off governs
  cancellations and package schedule changes.
- Show a live countdown on the menu page ("Order for tomorrow closes in 3h 12m").
- Validate the cut-off **server-side against the server clock**, never the browser's.
- If a schedule row has `qty_capacity`, decrement atomically and reject oversell.

### 6.1 Flow A — buying meals
1. Customer picks date + meal slot + diet type + food(s), qty, **delivery address**
   (chosen from their saved addresses), delivery time. The serving kitchen is resolved
   automatically from the address coordinates (§9.3) — show it as confirmation, not as a choice.
2. Cart shows tiered unit price, strike-through normal price when a promo applies, subtotal, delivery fee, total.
3. Checkout → order created as `AWAITING_PAYMENT` with a payment deadline
   **[DECIDE — recommend 2 hours, then auto-expire and release capacity]**.
4. Customer transfers to the displayed bank account and uploads proof.
5. Finance verifies → order becomes `PAID` → **confirmation email sent** → kitchen sees it.

### 6.2 Flow B — buying a package
1. Customer buys a package → order `AWAITING_PAYMENT`.
2. Payment verified → credits issued; **active period starts automatically**
   (**[DECIDE]** start = payment-verified date, recommended, vs purchase date) and
   `expires_at = start + validity_days`.
3. Customer then picks food schedule slots — any number of deliveries across lunch
   and/or dinner, any qty per slot — while credits remain and the package is not expired.
   Each picked portion consumes 1 credit and creates a `delivery` row.
4. Staff can pick slots on the customer's behalf, and can **extend the expiry date**
   from the backend (audit-logged, with a reason field).

### 6.3 Order state machine
`DRAFT → AWAITING_PAYMENT → PAYMENT_SUBMITTED → PAID → SCHEDULED → PREPARING → OUT_FOR_DELIVERY → DELIVERED`
plus `EXPIRED`, `CANCELLED`, `REFUNDED`. Illegal transitions must be rejected by the API.
Deliveries have their own lifecycle (`SCHEDULED → PREPARING → OUT_FOR_DELIVERY → DELIVERED / FAILED / SKIPPED`).

---

## 7. Credits — use a ledger, not a counter

Do **not** store `remaining_credit` as a mutable integer. Store an append-only
`credit_ledger`:

| column | meaning |
|---|---|
| `customer_id`, `customer_package_id` | owner |
| `entry_type` | `PURCHASE`, `REDEEM`, `REFUND`, `EXPIRE`, `ADJUSTMENT` |
| `qty` | signed (+20, −1) |
| `reference_type` / `reference_id` | order, delivery, staff adjustment |
| `occurred_at`, `created_by`, `note` | audit |

`remaining = SUM(qty)`. Redemption must be inside a transaction with a row-level lock
(or a `SELECT ... FOR UPDATE` on the package) so two tabs can't spend the same last credit.
Expiry is a scheduled job that posts a negative `EXPIRE` entry — so the balance history
always reconciles.

This directly produces the required **Customer Credit Report**:
`datetime_today | date_purchase | date_expired | purchased_credit | remaining_credit`,
plus a drill-down of every movement. Export to Excel and PDF.

---

## 8. Delivery addresses and time slots

### 8.1 Delivery time slots
Table-driven: `time` (15-minute grid: 10:00, 10:15, 10:30 …), `alias` (e.g. "Lunch",
"Dinner"), `is_active`, sort order. Customers see **only the alias**; the exact time is
internal. Admin maintains both. Seed exactly two active slots: `11:30 → "Lunch"`,
`18:30 → "Dinner"`; everything else inactive.

### 8.2 Customer addresses — multiple, geocoded, mandatory pin
A customer has **many** delivery addresses. Table `customer_address`:

| column | notes |
|---|---|
| `label` | "Home", "Office", free text |
| `recipient_name`, `recipient_phone` | may differ from the account holder |
| `address_line`, `district`, `city`, `province`, `postal_code` | text |
| `latitude`, `longitude` | **NOT NULL — mandatory** |
| `geom` | `geography(Point,4326)`, generated from lat/lng, GIST-indexed |
| `google_place_id`, `formatted_address` | returned by Places, stored for reference |
| `driver_note` | "grey gate, ring the bell" |
| `is_default`, `is_active` | exactly one default per customer |

- Coordinates are captured through **Google Maps**, not typed: a Places Autocomplete
  search box, a **draggable pin** on a map, and a "use my current location" button.
  The form **cannot be saved without a pin** — validate `lat`/`lng` presence and
  plausible bounds server-side too, not only in the browser.
- Reverse-geocode the pin to prefill the text fields, but let the customer correct them;
  the **pin is the source of truth** for routing, the text is for the driver.
- On save, immediately run the serviceability check (§9.3) and tell the customer which
  kitchen will serve them — or that the area isn't covered yet, before they try to order.
- Addresses are **soft-deleted** when referenced by an order.
- **Google Maps API keys:** a browser key restricted by HTTP referrer (`*.evermore.co.id`)
  and a separate server key restricted by IP; enable only Maps JavaScript, Places and
  Geocoding; set daily quotas and billing alerts. Never commit keys.
- **[DECIDE]** Fallback if Places is unavailable or the customer is offline-ish: allow
  manual lat/lng entry, or block? I recommend blocking, with a support WhatsApp link.

### 8.3 Choosing an address at checkout
- The customer **must select one saved address per delivery** (default preselected).
  For package customers this happens per picked schedule slot — lunch to the office,
  dinner to home is a normal pattern, so the address lives on the **delivery** row, not
  only on the order.
- On selection, the API returns the assigned kitchen, serviceability, and delivery fee.
- **Snapshot** the address text and coordinates onto the delivery at confirmation time —
  editing the saved address later must not rewrite delivery history.
- Package deliveries: allow **skip / pause / reschedule** before cut-off. A skipped
  delivery returns the credit (`REFUND` ledger entry); after cut-off it does not.

---

## 9. Kitchens and automatic order routing

Evermore operates **several kitchens**. Every delivery is routed to exactly one kitchen,
automatically, based on the delivery address coordinates.

### 9.1 Kitchen master
`kitchen`: `code`, `name`, `address_line`, `latitude`, `longitude` (**mandatory**, picked
on Google Maps the same way as customer addresses), `geom geography(Point,4326)`,
`phone`, `pic_name`, operating days, operating hours, **which delivery slots it serves**
(many-to-many with §8.1 slots), service-area definition (§9.2), `daily_capacity` /
per-slot capacity, `priority` (integer, lower = preferred), `is_active`, notes.
Full CRUD in the back office, audit-logged.

### 9.2 Service area — range to serve
Two supported modes on the same table:

- `service_radius_km NUMERIC` — simple radius around the kitchen point.
- `service_area geography(Polygon,4326)` — nullable; **when present it overrides the radius**,
  for the cases where a river or toll road makes a circle wrong.

Coverage test:

```sql
-- radius mode
ST_DWithin(k.geom, a.geom, k.service_radius_km * 1000)
-- polygon mode
ST_Covers(k.service_area, a.geom)
```

Both are index-backed (`CREATE INDEX ON kitchen USING gist (geom)` and on `service_area`).
Start with radius; keep the polygon column from day one so no migration is needed later.

**[DECIDE]** Straight-line distance ignores Jakarta traffic. A later upgrade is to rank
candidates by **Google Distance Matrix travel time** instead of crow-flight distance,
cached per (kitchen, address) pair. I recommend shipping with distance and adding
travel time once volumes justify the API cost.

### 9.3 Routing algorithm
Must be deterministic and explainable — staff will be asked "why did this go to Kitchen B?".

1. **Candidates** = kitchens that are active, serve the requested delivery slot, are
   open on that date, and cover the address (polygon if set, else radius).
2. **Drop** kitchens already at capacity for that date + slot (§9.4).
3. **Rank** by `priority` ASC → distance ASC → remaining capacity DESC.
4. **Assign** the top candidate. Persist on the delivery row: `kitchen_id`,
   `assigned_distance_m`, `assignment_mode` (`AUTO` / `MANUAL`), `assigned_at`,
   `assignment_reason` (a short human-readable string: "nearest covering kitchen, 3.2 km").
5. **No candidate** → the address is **not serviceable**: block the order with a clear
   message, and log the attempt with its coordinates. That log becomes the map of where
   to open the next kitchen. Offer "notify me when you deliver here".

Rules:
- Routing runs **per delivery**, not per order — one package can produce deliveries to
  different addresses and therefore different kitchens.
- Staff can **manually reassign** a delivery to any kitchen (audit-logged, reason
  required). Manual assignment sets `assignment_mode = MANUAL` and is **never**
  overwritten by the auto-router.
- Re-routing is allowed only **before cut-off**, and only when the address coordinates
  changed, a kitchen was deactivated, or staff triggered a rebalance. Never after cut-off.
- Expose `POST /api/v1/delivery-area/check` with `{lat, lng, slot, date}` →
  `{serviceable, kitchen, distance_km, delivery_fee}`. Use it on the address form, at
  checkout, and as a **"do we deliver to you?" widget on the homepage** — a cheap,
  high-converting piece of marketing.

### 9.4 Capacity
`kitchen_capacity` (kitchen, date, slot, max_portions) with a default inherited from the
kitchen. Decrement atomically inside the order transaction; reject oversell with a clear
"this slot is full at your nearest kitchen" message. Show staff a capacity heatmap per
kitchen per day.

### 9.5 Menu per kitchen **[DECIDE]**
Is the food schedule (§4.4) **global** or **per kitchen**? I recommend global — one menu
across the city, per-kitchen capacity only. Add `kitchen_id` to `food_schedule` only if
kitchens genuinely cook different menus, because it multiplies the staff's scheduling
workload by the number of kitchens.

### 9.6 Delivery fee **[DECIDE]**
Recommended: distance bands measured **from the assigned kitchen** (e.g. 0–5 km free,
5–10 km Rp 15.000, >10 km Rp 25.000), configurable in settings, with free delivery above
an order value threshold. Confirm whether phase 1 charges for delivery at all.

### 9.7 Admin coverage map
One screen showing every kitchen pin with its radius circle or polygon, overlaid with
recent order pins (green = served, red = rejected as out of range). This immediately
shows coverage gaps, overlaps, and where demand is unserved.

---

## 10. Payment

Phase 1: **manual bank transfer.** Bank name, account number, recipient name are
**system settings editable from the backend** — support more than one bank account.

- Show unique payment instructions per order. **Recommended:** add a unique 3-digit
  suffix to the transfer amount (Rp 500.123) — standard Indonesian practice that makes
  matching bank mutations far easier. **[DECIDE]**
- Customer uploads transfer proof (image/PDF, ≤5 MB, stored in a private bucket).
- Finance verification queue: filter by date, amount, status; verify or reject with a reason.
- Auto-expire unpaid orders at the deadline and release held capacity/credits.

Phase 2: **QRIS**. Build a `PaymentProvider` interface now (`createCharge`,
`handleWebhook`, `getStatus`) with `ManualTransferProvider` as the first implementation,
so adding Midtrans/Xendit QRIS later is a new class, not a refactor. Webhooks must be
signature-verified and idempotent.

---

## 11. Notifications

Email (transactional templates): registration/verification, order created with payment
instructions, payment verified, order scheduled, delivery reminder (evening before),
credits running low (≤2 left), package expiring in 3 days, package expired.

**Strongly recommended for the Indonesian market:** WhatsApp notifications alongside
email (WhatsApp Cloud API, or a local provider like Fonnte/Wablas). Abstract behind a
`NotificationChannel` interface with `email` and `whatsapp` implementations; per-customer
channel preferences. **[DECIDE — include in phase 1?]**

All sends go through a queue with retries, and are logged (`notification_log`) so
support can prove what was sent.

---

## 12. Back office (staff)

Dashboard: today's orders, unverified payments, meals to cook per slot **per kitchen**,
expiring packages, out-of-range order attempts.
Staff whose role is scoped to one kitchen should see only that kitchen's data —
add `kitchen_id` to staff users and filter accordingly. **[DECIDE — phase 1 or later?]**
Modules: customers (+ type change, + addresses, + order/credit history), organisations,
**kitchens (master, service area, capacity, coverage map)**, diet types,
foods & nutrition, food schedule calendar, packages, **four separate price forms**
(meal normal / meal promo / package normal / package promo) each with a conflict-aware
date picker, orders, deliveries, payment verification, settings, audit log, reports.

**Operational reports you will need on day one — the brief only lists one:**
1. **Customer credit report** (as specified in §7).
2. **Kitchen production sheet** — **filtered by kitchen**, for a given date + slot:
   total portions per food, per diet type. This is what each kitchen actually cooks from.
3. **Packing labels** — one label per delivery: customer name, phone, address, slot,
   diet type, food names, allergens, kitchen code.
4. **Courier manifest** — per kitchen, per date + slot, ordered by distance from the
   kitchen, with addresses, coordinates, a Google Maps deep link per stop, and phone numbers.
4b. **Coverage / rejection report** — orders blocked as out of range, with coordinates
   and counts by district. Drives the decision on where to open the next kitchen.
5. **Sales report** — revenue by day/week/month, by customer type, by diet type,
   meals vs packages, promo impact (normal minus promo = discount given).
6. **Unpaid / expiring** — orders awaiting payment, packages expiring in N days.
7. **Customer retention** — repeat rate, churn after package expiry.

All reports: date-range filter, CSV/Excel export, printable A4 layout.

---

## 13. Recommended extras (propose, don't build without approval)

Ordered by value for a catering B2C in Jakarta:

1. **Calorie/TDEE calculator** on the marketing site (height, weight, age, activity →
   recommended kcal → suggested diet type). Excellent lead magnet and it feeds the
   diet-type recommendation.
2. **Allergen & dislike profile** per customer; warn (or hide) scheduled foods that clash.
3. **Public menu calendar page** (SSG, per diet type) — big SEO win, drives organic traffic.
4. **Auto-renew reminder + one-click repurchase** of the last package.
5. **Referral codes** and **voucher codes** — keep them in a separate discount layer,
   applied *after* price resolution, so they never pollute the price tables.
6. **Corporate invoicing** for Siloam / Company A / B — monthly consolidated invoice,
   PO number, multiple employees ordering under one organisation with a per-employee
   credit allocation. This is likely where the real revenue is; design the data model
   for it now even if the UI comes later.
7. **Per-meal rating & feedback** after delivery — drives menu decisions.
8. **Customer-facing nutrition summary** — weekly kcal/protein intake chart from
   delivered meals. Very sticky, cheap to build once nutrition data exists.
9. Bahasa Indonesia + English i18n (default `id-ID`).
10. WhatsApp customer-service deep link on every order page.

---

## 14. Non-functional requirements

- **Security (OWASP ASVS L2 target):** argon2id password hashing, secure httpOnly
  session cookies or short-lived JWT + rotating refresh tokens, strict CORS, CSP,
  HSTS, rate limiting on auth/order endpoints, CAPTCHA on registration, server-side
  validation of every input (Zod schemas shared with the frontend), parameterised
  queries only, signed URLs for private files, no secrets in the repo (`.env.example` only).
- **IDOR is the top risk here** — every order, address, package and payment-proof
  fetch must verify ownership, not just authentication. Write tests for this.
- **UU PDP compliance:** consent at registration, privacy policy page, data-export and
  account-deletion flows, data hosted in the Jakarta region, PII redacted from logs.
- **Idempotency keys** on order creation and payment webhooks.
- **Geolocation is personal data** under UU PDP: coordinates are PII. Never log raw
  coordinates in application logs, restrict address reads to the owner and authorised
  staff, and include addresses in the data-export and deletion flows.
- **Testing:** unit tests for the pricing resolver, credit ledger and **kitchen router**
  (these three get near-100% coverage — overlap rejection, scope fallback, promo
  override, tier boundaries, concurrent redemption, and for routing: address inside one
  kitchen's radius, inside two overlapping radii, inside a polygon but outside the
  radius, outside everything, kitchen at capacity, kitchen inactive, manual assignment
  not overwritten), integration tests per API endpoint,
  Playwright E2E for the two main purchase flows.
- **Observability:** structured JSON logs with request ids, health endpoint,
  Sentry-compatible error reporting, daily automated `pg_dump` to object storage with
  a tested restore procedure.
- **Performance:** menu pages cached/SSG with tag-based revalidation on publish;
  system settings cached in Redis with invalidation on write.
- Seed data + a `make dev` (or `docker compose up`) one-command local setup.

---

## 15. Design direction

Clean, bright, food-forward. Large photography, generous whitespace, a fresh
green/earth palette, clear nutrition badges (kcal / protein) on every food card.
Mobile-first — most Indonesian B2C traffic is phone-based even without a PWA.
Checkout must be reachable in ≤3 taps from the menu. Prices in `Rp 500.000` format.

---

## 16. First deliverable

Reply with the four documents from §0, the completed **[DECIDE]** list with your
recommendation for each, and your proposed milestone plan. **Do not write code until
I approve them.**
