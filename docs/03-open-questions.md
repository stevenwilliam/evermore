# 03 — Open questions

**Version:** 0.1 — planning.
**Date:** 2026-08-12

Ambiguities in the master prompt, each with **the concrete answer I need**, a
**working assumption** so nothing stalls, and **what it blocks**. Per
`CLAUDE.md` §6 every question carries a default — answer with `ven:` lines, or
"all defaults" to take every assumption below as decided.

**Steven answered Q-1, Q-2, Q-4 and Q-13 on 2026-08-12** — recorded as D-30…D-34
in `02-decisions.md` and folded into the model. Their entries below are kept and
marked ✅ with what still follows from them.

---

## Answered

### ✅ Q-1 — Is PPN charged, and are displayed prices tax-inclusive?

**Answer: prices are tax-inclusive; the database splits base price and tax; the
rate is editable in the back office.** → D-30. The split is snapshotted per
order, not stored on the price row, so a rate change never rewrites the price
tables. Formula and worked example in `01-domain-model.md` §3.11.

**Q-1a still needed before M6 ships:** the **rate** to seed (11%? 12%?), whether
Evermore is **PKP-registered**, and the **NPWP + legal entity name** a corporate
invoice must carry. The engine is built and seeded from a parameter, so this is a
settings value rather than a code change — but the first real invoice cannot go
out without it.

<details><summary>Original question</summary>

The brief never mentions tax. Indonesian VAT on prepared food supplied by a
PKP-registered caterer is a real question, and it decides whether
`unit_price_idr` is gross or net, whether the order needs `tax_base_idr` /
`tax_idr` columns, and what the invoice for Siloam and the corporate accounts
must show. **This is a money-model question, so it cannot be retrofitted
cheaply.**

**I need:** is Evermore PKP-registered? Is PPN charged on these sales, at what
rate, and are the prices staff type into the four price tables tax-**inclusive**
or tax-**exclusive**? Does corporate invoicing need PPN broken out and an NPWP on
the invoice?

**Blocks:** the pricing engine (M5) and everything downstream of it.
*Recommendation if you want one:* store prices **tax-inclusive**, carry
`tax_rate_bps` in `sys_parameters`, and compute the tax base by back-calculation
on the invoice — that way a rate change never rewrites the price tables.
</details>

### ✅ Q-13 — Refund policy in money terms

**Answer: no refund.** → D-31. Compensation is in **credits**, never rupiah;
unused credits are forfeited at expiry. `REFUNDED` remains admin-only for
correcting an erroneous or duplicate transfer, which is returning money that was
never owed rather than honouring a policy.

**Follow-on:** forfeiture-at-expiry and no-refund must both be stated plainly on
the terms page before launch (Q-27).

### ✅ Q-2 — What is 1 credit?

**Answer: 1 meal contains several foods; 1 credit buys 1 meal, even when that
meal is a single dish.** → D-32, D-33. The meal became an entity
(`scheduled_meal` + `scheduled_meal_item`), the customer picks meals rather than
individual foods, and a meal's nutrition panel is aggregated from its foods.

### 🟡 Q-4 — The kitchens at launch

**Answer on the rule: "auto assign location customer to nearest kitchen."** →
D-34, which is the router already specified. Every kitchen is seeded at equal
`priority` so ranking collapses to nearest-first, with `priority` left as the
manual override.

**The data is still outstanding** and is now a launch-blocking input rather than
a design question — see the table at the end of this document.

<details><summary>Original question</summary>

The entire routing design is theoretical until real kitchens exist.

**I need, per kitchen:** name, code, full address, the map pin (lat/lng),
which of the two slots it serves, operating days and hours, service radius in km,
daily and per-slot portion capacity, priority order, and the PIC's name and
phone.

**Blocks:** kitchen routing (M3) can be *built* with seeded fake kitchens, but
cannot be *verified* — and "is this address serviceable" is on the homepage.
</details>

---

## Superseded

<details><summary>Q-13, as originally asked</summary>

§6.3 has `REFUNDED` and §7 has a `REFUND` ledger entry, but they are different
things: one returns rupiah, the other returns a credit. The brief never states
when actual money goes back.

**I need:** can a customer get cash back at all, or only credits? Who approves
(finance alone, or admin)? Is a partial refund possible on a multi-line order?
What happens to an unused package at expiry — forfeited, or refundable pro-rata?
Forfeiture is normal in this market but must be on the terms page before launch,
not decided at the first complaint.

**Blocks:** finance module (M7) and the terms page.
</details>

---

## Rules the brief left ambiguous

<details><summary>Q-2, as originally asked — answered above by D-32</summary>

`D-7` allows a set menu (main + side + dessert) for one date+diet+slot. §6.2.3
says "each picked portion consumes 1 credit". So does 1 credit buy **the whole
set** for that sitting, or **one food item**?

**Assumption:** 1 credit = **one complete meal set for one person at one slot** —
every `PUBLISHED` food row for that date+diet+slot goes in the box. Anything else
makes a package's value depend on how many components staff scheduled that day.

**Consequence, so it is not a surprise:** §6.1's "customer picks food(s)" for
à-la-carte then does not mean picking individual dishes either — the customer
picks a **date + slot + diet type**, and the set is what is scheduled.
**Confirmed by Steven — this is exactly the model (D-32).**
</details>

### Q-3 — Delivery fee real numbers

`D-19` proposes 0–5 km free · 5–10 km Rp 15.000 · >10 km Rp 25.000, free above
Rp 300.000. **All four are invented placeholders.**
**Assumption:** ship the engine seeded with those, and you retune in
`sys_parameters` before launch without a deploy. Also: do **package** customers
pay delivery per drop, or is it included in the package price?
*Assumed included* — otherwise a 20-credit package has an unpredictable total.

### Q-5 — Cut-off, per slot

18:00 on D-1 applied to a dinner slot at 18:30 means a 24.5-hour lead time,
against 17.5 hours for lunch. That may be exactly right for kitchen planning, or
it may cost you same-day dinner orders.

**Assumption:** one global cut-off (18:00, D-1) for both slots as written, but
the parameter is **per slot** in the schema from day one so tuning dinner later
is a settings change, not a migration.

### Q-6 — Is `diet_subtype` ever a pricing or scheduling axis?

**Assumption: no.** Subtypes (`Diabetic`, `Cholesterol`) describe the customer
and the food; the schedule and all four price tables key on `diet_type` only.
Otherwise the price matrix multiplies by subtype and staff maintain several times
as many rows. If a Diabetic menu is genuinely cooked separately from a general
Special Diet menu, it should be its own **diet type**, not a subtype.

### Q-7 — Can one order contain several dates?

§6.1 describes picking a date, singular; a cart implies several.
**Assumption: yes, multi-date and multi-slot in one order** — a customer buying
Monday-to-Friday lunches should pay once. This drives tiering: the tier is
resolved on the **order's total quantity** across all dates, which is more
generous than per-date and is the interpretation customers assume.
**Confirm — it changes what a five-day order costs.**

### Q-8 — A delivery the courier could not complete

**Assumption:** `FAILED` does **not** return a credit automatically (the food was
cooked and sent), staff can post a goodwill `ADJUSTMENT` with a reason, and the
delivery can be rescheduled. Nothing automatic, per `99-steven-preference.md` §8
— "humans cancel; the system surfaces the queue".

### Q-9 — A prepaid package customer meets a full kitchen

They have paid for 20 credits, and on the day they want, their kitchen is at
capacity. Refusing them is a much worse experience than refusing a new order.

**Assumption:** capacity is checked at **slot-pick** time and blocks with a
"choose another date or slot" message plus the next three available dates.
**Recommended addition:** reserve a configurable share of each kitchen's daily
capacity for package holders (`package_capacity_reserve_pct` in
`sys_parameters`, seeded 0 = off). Cheap to build with the capacity table, and
it is the answer when this complaint arrives.

### Q-10 — Can a package be redeemed across different diet types?

**Assumption:** yes unless the package is restricted (`D-12`) — a customer can
take Weight Loss on Monday and High Protein on Tuesday from a generic package.
If diets differ in cost enough that this matters commercially, restrict the
package instead of adding a rule.

### Q-11 — Coordinate sanity bounds

Server-side validation needs an envelope so `(0,0)` and a mis-signed longitude
are rejected. **Assumption:** Jabodetabek envelope, roughly
lat `-6.60 … -5.90`, lng `106.50 … 107.10`, held in `sys_parameters` so
expansion to another city is a settings change. Outside the envelope = rejected
as an input error; inside but outside every kitchen = "not serviceable yet",
which is a different message and gets logged for the coverage report.

### Q-12 — Minimum order

**Assumption:** minimum 1 portion, no minimum rupiah value; the tier table
handles volume pricing. If corporate accounts have contractual minimums, that is
an `organisation` field and I need the rule.

### Q-14 — Who may cancel, and does auto-expiry conflict with the house rule?

`99-steven-preference.md` §8 says **nothing automated cancels a customer's
booking**. The brief's §10 explicitly asks for auto-expiry of unpaid orders.

**Assumption:** the brief wins for **unpaid** orders only — auto-expiry applies
strictly before payment, where nothing has been promised and capacity is being
held. Nothing automated ever touches a `PAID` order, a scheduled delivery or a
package. Customer self-cancellation is allowed **only** before the cut-off and
only for unpaid orders; everything else goes to staff, who see a queue.

### Q-15 — Registration and account rules

**Assumptions:** email + password with argon2id; **email verification required
before the first order** (not before browsing); phone captured at registration
but OTP-verified only at first checkout; **no guest checkout** — every order is
tied to an account, because deliveries, credits and addresses all need an owner.
CAPTCHA on registration: **Cloudflare Turnstile** rather than reCAPTCHA (no
extra Google dependency, no PII to a second Google product).

### Q-16 — 2FA scope

§3 says "staff accounts require 2FA (TOTP)". **Assumption:** mandatory for
`admin`, `finance` and `staff`; **optional for `kitchen` and `courier`**, who
work from shared or phone devices on a service floor where a TOTP app is a real
obstacle. Their accounts are read-mostly and scoped to one kitchen. Say the word
and it becomes mandatory for all five.

### Q-17 — Menu publication horizon

Package customers picking slots need a published menu to pick from.
**Assumption:** the menu is published at least **7 days ahead**, tracked as an
operational target and surfaced on the staff dashboard as a warning when the
horizon drops below it. Customers can only pick slots that are published.

### Q-18 — Un-publishing a schedule that already has orders

**Assumption:** blocked. Once a `food_schedule` row has a delivery against it,
it cannot be un-published or deleted — only the food can be **substituted**,
which writes an audit row and notifies affected customers. Silently vanishing
menus are how people end up with an empty box.

---

## Operational inputs I need before launch, not before building

| # | Need | Blocks |
|---|---|---|
| ✅ Q-4 | **The real kitchens** | **Answered 2026-08-13.** Five kitchens, 20 km each, every day, no capacity, one shared phone. Migration `0014`. |
| 🟡 Q-1a | PPN rate, **PKP status**, NPWP, legal entity | Entity and address answered. **PKP status still unanswered**, and the NPWP given (`123 123 123`) is not a valid number. Blocks the first faktur pajak; if not PKP, 11% PPN may not be charged at all. |
| ✅ Q-19 | Bank account details | **Answered 2026-08-13.** Nobu · 16830226665 · PT Sunshine Food International · Menara Matahari. |
| 🟡 Q-20 | Legal entity, address, NPWP, contact | Name, address and phone answered. NPWP is a placeholder — see Q-1a. |
| Q-21 | SMTP relay host, port 587 credentials, and the `From` domain with SPF/DKIM/DMARC set up | All email (M10) |
| 🟡 Q-22 | Google Cloud billing, key ownership, quota alarms | A working key was supplied and the kitchens were geocoded with it. It is **unrestricted (`0.0.0.0/0`), shared between browser and server roles, and uncapped** — see `RUN-WHEN-BACK.md` §10. |
| 🟡 Q-23 | WhatsApp provider and sender number | **Answered 2026-08-13:** WAHA, 08176315568. Wired to the container shared with ruuma, already bound to that number — but its session reports `FAILED`, so nothing sends until it is re-linked. |
| Q-24 | Production host: provider and Jakarta region confirmation (UU PDP data residency) | Deploy (M12) |
| Q-25 | Food photography — who shoots it, and is there a phase-1 set? | Public site (M11) |
| Q-26 | Existing customer/order data to migrate, or a clean start? | M1 |
| Q-27 | Terms of service, privacy policy and refund policy copy — I can draft, someone must approve | Legal pages, launch |
| Q-28 | Erode font licence for web embedding (carried from `00` Q9) | Any public page |
| 🟡 Q-29 | Reversed-out logo | **Deferred by Steven 2026-08-13** — the text wordmark stays for now, to be fine-tuned later. |

---

## Q-24 — Translated catalogue content (raised 2026-08-18)

The UI is now trilingual: Indonesian, English and Simplified Chinese, with the
copy in message catalogues (`internal/adapter/http/messages.go` and
`web/src/lib/messages.ts`) and a language selector on both surfaces.

**What is still single-language is the CONTENT**, because it is database rows
rather than UI strings:

| Table | Columns | Shows up as |
|---|---|---|
| `diet_type` | `name`, `description`, `seo_title`, `seo_description` | nav links, menu page headline and lede, `<title>` |
| `meal` | `name` | the card title on a menu page |
| `food` | `name` | the item list inside each card |

So a reader on `/zh/menu/keto` gets Chinese chrome around an Indonesian menu.
That is better than nothing and is visibly half-done.

**Proposed default:** add a `translations` JSONB column per table, keyed by
locale, e.g. `{"en": {"name": "...", "description": "..."}, "zh": {...}}`, with
the existing columns staying as the fallback. One migration, one admin form
section per record, no fan-out of columns as locales are added. Reads fall back
to the base column when a locale is absent, exactly as the message catalogues
already do.

**Needs Steven to decide:** whether the menu content is translated at all (it
is real ongoing work — every meal, every week, in three languages), or whether
only the marketing chrome is multilingual and the food keeps its Indonesian
names. The second is a legitimate choice and is cheaper forever; a lot of
Jakarta restaurants do exactly that.
