# Build progress

Live status. Legend: ✅ done & tested · 🟡 partial · ⬜ not started.

**A ✅ has to be re-earned by running the gate, never inherited.**
Everything marked ✅ below was **run**, not just written. Where something was
written but not executed, it is 🟡 and says what is missing.

_Last updated: 2026-08-31 — M17 complete, and the demo data populated._

---

## Deployed right now

| | |
|---|---|
| URL | **http://192.168.88.101:8090** (nginx → 127.0.0.1:8081) |
| Service | `systemd` unit `evermore`, enabled, survives restart |
| API bind | `127.0.0.1` only — **verified unreachable from the LAN** |
| Secrets | `/etc/evermore/evermore.env`, `root:dev`, mode 640 |
| Database | PostgreSQL 18.4 + PostGIS 3.6, **30** migrations on disk (0001–0030), each with its `.down.sql` |
| ruuma | untouched — still owns `:80 default_server` |

## M0 — Definition ✅
Repo, `CLAUDE.md`, brand read and contrast calculated, brief received and
stored at `PROMPT.md`, locale settled.

## M0b — Planning documents ✅
`01` domain model · `02` decisions (34) · `03` open questions · `04` milestones.
All of Steven's answers folded in (D-30…D-34, D13–D16).

## M1 — Environment ✅
PostgreSQL 18 + PostGIS + btree_gist + citext, Redis satellite, `.env`,
self-hosted Erode and Inter with their licences.

## M2 — Schema ✅
Migrations 0001–0030, up **and** down. 0001–0013 were applied and re-applied
on a clean database in the first session; 0014–0030 have been applied to the
development database but the clean-database round trip has **not** been re-run
since — see `RUN-WHEN-BACK.md`. Constraints proved by direct SQL, not assumed: price overlap,
capacity oversell, append-only ledger and audit, sign checks, the 15-minute
grid.

## M3 — Domain layer ✅
`money` 92% · `pricing` 94% · `credit` 97% · `routing` 95% · `nutrition` 95% ·
`schedule` 91% · `order` 94% · `sanitize` · all pure, no I/O.

## M4 — Identity, RBAC, audit ✅
Registration, login, refresh rotation, deny-by-default permissions resolved per
request from the database, audit log, `api create-staff` first-run flow.
✅ **TOTP two-factor** — enrolment, confirmation, login challenge, recovery
codes. Mandatory for admin/finance/staff, refused as optional for those roles;
kitchen and courier are exempt (they sign in from shared phones on a service
floor). Secrets are AES-GCM encrypted at rest under `TOTP_ENCRYPTION_KEY`;
without that key the feature is OFF and the routes are absent, which the boot
log states. Verified end-to-end against the running service with codes
generated from the secret, including the case that matters: **the challenge
token issued after a correct password is refused as a session** (401
"Finish signing in first"). RFC 6238 vectors are pinned in a unit test.

## M5 — Master data & settings ✅
Customer types, diet types, allergens, slots, organisations, `sys_parameters`
CRUD with type checking, secret masking and full audit. Search on every list.

## M6 — Catalogue & menu calendar ✅
Foods with nutrition, meals composed of dishes, aggregation verified
(320+80+60 = 460 kcal), publish/unpublish, copy-week, publish horizon.

## M7 — Pricing ✅
Four tables, four URLs, exclusion constraints reporting the conflicting row,
scope→DEFAULT→promo resolution, flat tiers, the D-9 corporate warning.

## M8 — Ordering ✅
Cart, cut-off, capacity on both counters, routing per delivery, snapshots,
idempotency. **Oversell proved impossible**: 12 concurrent orders on 4
portions → 4 succeed, 8 refused, no oversell.

## M9 — Payments ✅
Verification queue oldest-first, locked verify, rejection with reason, proof
upload validation, auto-expiry returning both counters.
🟡 **Object storage not wired** — proofs are recorded by key, not yet uploaded.

## M10 — Packages & credits ✅
Purchase, activation on verification, append-only ledger, booking.
**Double-spend proved impossible**: 8 concurrent bookings on 1 credit → 1
succeeds, balance 0, never negative.

## M11 — Reports ✅
All eight, kitchen-scoped, CSV-safe against spreadsheet formula injection.

## M12 — Notifications 🟡
Nine templates in both languages, SMTP sender, escaping and header-injection
tested. **The queue and the scheduled sends are not wired** — nothing is
actually emailed yet; the verification link is logged with a warning.

## M13 — Public site ✅
Home and per-diet menu pages, server-rendered, OG/Twitter tags static in the
HTML (verified with `curl`), robots.txt, sitemap.xml, JSON-LD.
**Verified by looking at it**: screenshots at 360px and 1280px in
`docs/screenshots/`, fonts confirmed resolved from the DOM.

**Configurable header menu, 2026-08-19 (Steven).** The header is data
(migration 0026): `nav_item` rows with visibility and sort order, edited
through `/api/v1/admin/nav-items` with a CSV export like every other grid. What
is NOT editable is the label, the path or the kind — a label typed into an
admin box would exist in one language on a trilingual site, and a typed path
could point at a route that does not exist; the wording stays in the message
catalogue. On Steven's instruction the header is now just **Home | Menu**
(migrations 0027, 0028), Menu pointing straight at the Healthy category. The
hidden pages still exist, still render in three languages and are still in the
sitemap — only the header stopped linking to them.

The home page gained the package price TABLE after the hero and the Benefits
block below it, both rendered from the same partial and the same editable
content as their standalone pages, so the two placements cannot disagree.

**Certification badges are images, 2026-08-19 (Steven).** Each of the three is
an `<img>` whose path is its own `sys_parameters` row (migration 0025), so the
certifying body's own logo file replaces the default without a deploy — those
marks are trademarks and come from the issuer, never redrawn here. The defaults
are Evermore's own seal artwork from `scripts/mkseals.py`. Clearing a path
hides that badge; that needed the code to pass NO default, because
`Store.String` returns its default whenever a value is empty, which would have
made a cleared badge reappear. Proved by clearing one and counting.

**Corner ribbon and certification badges, 2026-08-19 (Steven).** A gold
diagonal ribbon in the top-right of every public page with an animated shine,
and three large HALAL / HACCP / ISO 22000 badges at the foot of the home page.
Both are `sys_parameters` switches (`public.ribbon_enabled`,
`public.certifications_enabled`) with wording editable on the Content screen —
the ribbon because "free delivery" is only true while `delivery.fee_bands` says
so, the badges because certificates expire. Steven confirms Evermore holds all
three certificates. Contrast measured on both: deep ink on the base gold is
5.65 and the shimmer only ever sweeps lighter, so no frame of the animation
drops below it; the beige badge discs are 11.32 against the ground and 11.32
for the ink on them.

**CSV exports, career form, Benefits page, 2026-08-18 (Steven).** New standing
rule in `99` §8 and CLAUDE.md §7: every report and data grid exports to CSV,
**pipe-delimited**. Audited and fixed — the delimiter was a comma everywhere,
and `/coverage` and `/unpaid` had no export at all. All 8 reports export now,
plus the payment queue, the deliveries grid and both career grids, each
honouring the search and filters on screen. Still missing and worth knowing:
**there is no reports SCREEN in the app at all** — the eight reports are
API-only, so there is no button to put an export button on yet.

Career is a real form (migration 0022): open positions listed at the top from
`job_opening` and editable in the back office, the position field populated
from those same rows so nobody can apply for a closed role, every field
sanitised server-side, rate-limited per IP, and **no file uploads** — multipart
is refused with 415 and the multipart parser is never reached. Verified end to
end: valid submission stored, a `curl -F` upload attempt rejected, per-field
errors on bad input. A Benefits page joins the header beside Price list, and
the price list leads with packages.

**Benefit section and rich text, 2026-08-18 (Steven).** A Benefit block after
the price list, editable in the back office with a WYSIWYG editor and seeded
with initial wording in all three languages (migration 0021). `public_content`
gained `is_html`; rich values are sanitised against an allowlist on write, on
render and on the admin read path (`internal/platform/richtext`, bluemonday).
The editor is hand-rolled — six buttons did not justify a 100–300 kB dependency
on a 76 kB bundle. Package wording is days rather than credits, with the
validity window dropped from the cards (migrations 0020, 0021), and Category
leads the header nav.

**Per-portion prices hidden, 2026-08-18 (Steven).** `public.show_meal_prices`
is off, so the price list shows packages and a request-a-quote panel instead of
the per-meal table. A setting rather than deleted markup — turning it back on
is a value change on the settings screen, not a release. The rows are not
fetched into the page at all when hidden, so the figures are absent from the
HTML rather than merely unrendered.

**Header navigation and four new pages, 2026-08-18 (Steven).** The masthead is
now Price list · Contact · About us · Career · Category, with the six diet
types moved into the Category submenu — a `<details>` disclosure like the
language picker, so the pages still ship no JavaScript and the diet links stay
plain crawlable `<a>`s. `/price-list`, `/contact`, `/about` and `/career` are
real server-rendered routes in all three languages, with hreflang, canonicals
and sitemap entries (33 URLs now). The price list reads the live pricing tables
— DEFAULT scope only, valid today, so a negotiated corporate rate can never
appear — and About/Career/Contact copy is editable in the back office
(migration 0018). Nutrition badges are pinned to the foot of each menu card so
the figures line up across a row.

**Sample menu, 2026-08-18 (Steven).** `./bin/api seed-menu [days]` fills the
calendar: 18 dishes with full nutrition panels, and one published lunch menu
per diet type per day, cycling three variants so a week is not the same plate
seven times. A command rather than a migration because the dates are relative —
a migration with a hard date in it is wrong the next morning. Idempotent on
both halves. Verified: 3 meals live per category, and the nutrition aggregate
is exact (220+78+55 = 353 kcal, 31+6.3+3.7 = 41 g protein).
Menu cards now carry a picture band — a real photo when
`scheduled_meal.hero_photo_key` is set, otherwise an illustrated band in the
diet type's colour from the same glyph source as the home cards. **AI-generated
food photography is still blocked on an image-generation key** (RUN-WHEN-BACK
§A2); in the meantime `./bin/api seed-menu-images` attaches a random Lorem
Picsum photograph per menu, downloaded once and served locally. Random means
random — a chicken dish may show a doorway — so these are development
scaffolding, not launch assets, and the handbook says how to clear them.

**Editable hero copy, 2026-08-18 (Steven).** The home hero wording moved out of
the binary into `public_content` (migration 0017) and is edited at
`/app/admin/content`: Indonesian is the source, English and Chinese are derived
from it, either can be overridden by hand, and an override whose Indonesian has
since changed is flagged stale rather than silently refreshed. The picture is
the `public.hero_image` setting. Machine translation is built behind a
`Translator` port with a Google Cloud Translation v2 adapter — **no key is
configured**, so translations are hand-written today and the screen says so.
Details in `docs/11` §6.

**Screenshot coverage, 2026-08-18.** The four authenticated-screen captures
(`spa-menu`, `spa-order`, `spa-cart-mobile`, `spa-admin-payments`) were deleted
rather than kept: they were taken on 2026-08-13 and still showed the beige
ground, the text wordmark and the untranslated UI, so as documentation they
were actively misleading. Re-taking them needs a signed-in session in the
headless browser — added to `RUN-WHEN-BACK.md` §B. The login screen, which
needs no session, is captured in both English and Chinese.

**Trilingual, 2026-08-18 (Steven).** Indonesian, English and Simplified
Chinese, with a flag-and-name language selector on both surfaces. Public pages
are path-prefixed (`/`, `/en/`, `/zh/`) with hreflang alternates and a
per-locale sitemap; the app stores the preference. Copy lives in catalogues
(`internal/adapter/http/messages.go`, `web/src/lib/messages.ts`) and the
notification templates gained Chinese. Details and the limits in `docs/11`.
**Not done:** the app's inner screens (~140 strings) are still Indonesian, and
menu CONTENT is single-language by schema — raised as docs/03 Q-24.

**Brand pass, 2026-08-18 (Steven).** Page ground is now Nourish Green `#468973`;
the supplied wordmark replaces the text logo on both surfaces; the footer is the
masthead's fill, fixed to the bottom and thin; the WhatsApp float is larger and
on every page, public and app; the Erode/Inter pairing is bigger and bolder
throughout. Two accessibility consequences are recorded in `10` §2.7: nothing
reaches AA at reading size on the new ground, so body copy sits on beige sheets
and white cards, and the WhatsApp teal measures **1.00:1** against it and now
carries a beige ring. Re-verified by screenshot on home, menu, 404 and the app
at 390px and 1280px. `/images` was also never mounted, so every page's
`og:image` had been 404ing — fixed, and the card is generated.

**Then the two greens were swapped** (Steven): the page is deep `#1C3D34` and
the bars are mid `#468973`. Net accessibility gain — body copy on the page is
11.32 — at the cost of the bars, where beige is 3.93 and every string must be
19px/700 "large text". Favicon derived from the wordmark's leading `e`; a split
hero with a picture on the right, its source a `sys_parameters` row (migration
0015). `docs/10` §2.7 carries the measured tables.

## M14 — Security suite ✅
Injection, oversell, constraint-as-last-line, append-only, price overlap,
order reconciliation, no negative balances, one REDEEM per delivery, plus the
MFA suite (challenge token is not a session, TOTP not spendable twice
concurrently, recovery code consumed exactly once, secret not stored in
plaintext).

**Re-run 2026-08-27 against the live development database: 14 tests, all pass**
(`TestEveryOrderReconciles` reconciled 12 orders). Runs against a real database
and skips loudly without one — which means **`go test ./...` on its own is
green without proving any of it**. To actually earn this ✅:

```
TEST_DATABASE_URL="$(grep -oE '^DATABASE_URL=.*' /home/dev/projects/healthy_catering/.env | cut -d= -f2-)" \
  /usr/local/go/bin/go test -count=1 ./test/security/...
```

## M15 — Deployment ✅ (development server)
systemd + nginx + hardened unit + first-run admin. Handbook at `14`.

## M17 — The design canvas ✅ / 🟡
Steven supplied the Claude Design project *"Healthy catering UI mockups"*
(`Evermore Mockups.dc.html`) on 2026-08-31 — fourteen artboards. Read into
`docs/10-design-system.md` **§4**, which is now the normative component layer,
and built out.

**Contrast first.** Four of the canvas's own pairings fail AA and ship altered;
§4.1 is the record and every number came from `scripts/contrast.py`. The
capacity pill was the worst: `#CC6883` filled carries 3.40 with beige ink and
3.33 with deep, under 13px text that is not large text either way. It ships
`#91253D` at **7.89** with a `#CC6883` ring, because that fill is only 1.44
against the ground and would otherwise have no edge.

✅ **The component layer** — `web/src/index.css` rebuilt to §4: beige pill CTAs
on the ground (the app came *off* its single beige sheet), 2px outline
secondaries, left-rule callouts, framed grid tables, stat tiles, status pills,
chips, the mobile bottom bar and the back-office rail. Radius scale and the new
component roles are tokens in `web/public/css/tokens.css`.

✅ **Six new screens**, all against endpoints that already existed:
S1 dashboard · S2 menu calendar · S3 the four price tables + the live resolver
· S5 kitchens & coverage · P1 A4 production sheet · P2 packing labels
(100×150 and 100×50 mm), with a print stylesheet.

✅ **One new endpoint** — `GET /admin/kitchens?date=`, kitchen coverage plus
per-slot capacity for that service date. Repo → app → handler, raw SQL for the
PostGIS and capacity join. **No Go test yet** — see below.

✅ **M1 home page** — service-area badge, two CTAs, two *counted* figures (diet
types and active kitchens, so neither can go stale in copy), and the mid-green
menu band with the package card.

✅ **Verified by looking at it.** `scripts/shoot-screens.js` captures all eleven
screens; `docs/screenshots/` is current. Nine defects were found in the
screenshots and fixed, including three that reading the CSS would never have
caught: a framed grid clipping two slot columns behind `overflow-hidden`, a
`:last-child` selector that stripped the row rule from every row's last
*column* rather than the last row, and percentage-height circles rendering as
ellipses in a 4:3 box. Two header/cell mismatches were caught the same way.

🟡 **The signed-in screens have only been rendered against FIXTURES.** The
harness stubs the API, so the components, stylesheet and response shapes are
proven and the server's actual responses are not. Signing in needs a staff
account, which needs a database write this environment does not grant.
`RUN-WHEN-BACK.md` §B1.

🟡 **The M1 menu band has never rendered with data** — the seeded calendar ends
2026-08-20, so it correctly renders nothing today. §B2.

🟡 **S5's map is a schematic**, not a tile layer: real coordinates and radii
projected to scale, no streets. Needs the browser Maps key handed to the SPA.
§B3 and `docs/10` §4.12.

## M17b — The canvas as the specification (phase 1 of 2) 🟡
Steven, 2026-08-31: **"100% like the canvas — forget the last design."** Four
decisions taken up front, all four his:

1. **Colour is the canvas's; SIZE carries AA.** Not one hex altered. Where a
   pairing measures under AA at the size drawn, the type moves to 19px/700 —
   WCAG's large-text threshold — which drops the requirement from 4.5 to 3.0
   and makes the artboard's own colour legal. `#CC6883` capacity pills and
   `#468973` selected rows are back exactly as drawn. §4.1 is the table.
   **Three inks could not be saved**: `#CCBDAA` on `#468973` is **2.25**,
   under the floor large text itself has to clear, so the rail's kicker, the
   staff sub-label and S3's tax note are beige.
2. **Responsive**, exact at 390 and 1440.
3. **Photo upload**, with the illustrated fallback until filled.
4. **Canvas copy is the spec** — its Indonesian is now the `id-ID` catalogue,
   with English and Chinese written to match. ~120 new keys across three
   languages, and `t()` grew positional `{0}` slots so Chinese can put the
   number where Chinese puts numbers.

✅ **The customer flow, which did not exist.** Artboards 01–06, M2 and M3:
menu calendar with day chips and the live cut-off countdown · meal detail with
the six-cell nutrition panel and allergen chips · cart with the tier ladder and
the "tambah 4 porsi lagi" nudge · checkout with slot chips, struck-through full
slots and a courier note · manual transfer with the three-digit suffix picked
out · package purchase · credit balance and ledger · booking a slot from
credit. A client-side cart (`lib/cart.tsx`) and the phone shell
(`components/mobile.tsx`) underpin them; the artboards mount ABOVE the app
shell, because the desktop masthead sat as a dead band over the design and the
fixed footer printed across the sticky total.

✅ **Backend, five additions.** `GET /api/v1/public/prices` (the tier ladder a
customer may see — DEFAULT scope filtered in the query, so it cannot leak a
negotiated corporate rate) · `GET /delivery-slots/availability` (per-slot
routing for one address and date, with `CheckInput.NoLog` so asking four times
per page view does not turn the coverage report into a page-view counter) ·
`portion_size` and `allergens` on the meal read path · a per-delivery courier
note that overrides the address's standing one without erasing it.

✅ **Phase 2, the back office and the public home.** S2–S5 moved onto the
canvas's TOP-BAR variant (`components/backoffice.tsx`), S1 gained the five-tile
row it draws — meals, deliveries, awaiting verification, verified revenue, out
of range — and S4 was rebuilt as the queue-plus-detail-panel the artboard
specifies, with the proof image behind a ten-minute presigned URL and a
rejection that will not proceed without a reason for the audit row.

✅ **M1's masthead** — the canvas's five nav items and its button pair, from
migration 0031. The nav stays DATA (migration 0026), so the labels are
catalogue keys and not typed wording. This SUPERSEDES the 2026-08-19 reduction
to "Beranda | Menu"; the newer, more specific decision wins.

✅ **Meal photography can now be uploaded** — `POST/DELETE
/admin/calendar/meals/:id/photo`, audited, with magic-byte sniffing in the
store so a .jpg that is really a script is refused at the door rather than on
the page that renders it. `GET /media/:key` presigns for ten minutes. Clearing
a key leaves the object in the bucket: deleting it would break a packing label
already printed from a snapshot that referenced it.

**Three invisible-text defects found by looking, none visible in the source:**

1. `.masthead a` at specificity (0,1,1) beat `.cta-bar` at (0,1,0), so
   "Lihat menu" rendered beige-on-beige at **1.00:1** — an invisible label on
   the site's primary call to action.
2. The ≥80rem media query re-placed the language picker into what had become
   the actions' column, painting the flag on top of that same button.
3. **`text-bar` was both a colour and a size.** `theme.extend.colors.bar` and
   `theme.extend.fontSize.bar` both existed — this PREDATES the canvas work —
   and Tailwind emitted two rules for one utility, so anything using it for
   size alone was painted `#468973`: **1.00:1 on the mid-green bar**. The
   stepper's quantity was an empty gap. The size key is now `onbar`, and
   `scripts/verify-tailwind-keys.js` fails the build on any future collision —
   checked against the real bug before being trusted. docs/10 §4.1b.

⬜ **The seeded menu photography is wrong** and this is live today. Every file
in `web/public/images/menu/` is generic stock landscape, not food:
`ayam-panggang-brokoli.jpg` is a suspension bridge,
`udang-padang-nasi-merah.jpg` is a muddy road at sunset. Nineteen files, all
distinct, all wrong. The public menu pages render them now. The illustrated
diet-tint fallback already exists, so clearing `hero_photo_key` is a one-line
UPDATE and is better than what ships today — but it is Steven's data decision,
not mine. `RUN-WHEN-BACK.md` §B4.

## M18 — Demo data ✅
Steven: "populate dummy data, all menu all price." Two re-runnable commands,
both idempotent, neither destructive by default.

```bash
./bin/api seed-menu 14        # menu, capacity, allergens
./bin/api seed-prices         # all four price tables
./bin/api seed-prices replace # …superseding what is already there
```

**What was actually missing, beyond prices.** `kitchen_capacity` was
**empty** — so no slot was OPEN on any date, the dashboard read "closed"
everywhere and the slot picker refused every booking. Nothing seeded was
orderable. `food_allergen` was **empty** too, so every meal reported "no
allergens recorded" and every packing label printed a blank allergen line —
which reads as "none", a different claim from "not yet recorded", and the one
that gets somebody hurt. And only 2 of the canvas's 4 slots existed: 07.00 was
in the reference data but **inactive**, so a breakfast the design serves the
database refused.

**Now in place:** 355 published meals across 6 diets × 4 slots × 14 days ·
1,070 meal items · 280 capacity rows · 11 allergen links · **96 active meal
prices** (4 scopes × 6 diets × 4 tiers, no gaps) · 2 live meal promos · 3
package prices · 1 package promo. 96 superseded rows remain as `is_active =
FALSE`, which is also what gives artboard S3 its "Arsip" state real rows.

**The seeder verifies its own claims and refuses to finish otherwise.** Two
invariants are re-read from the table after writing: every tier ladder must get
CHEAPER with quantity, and every diet × tier must have a DEFAULT price. Both
caught a real fault on the first run — reference data priced healthy at 55.000
for 1–4 and 48.000 for 10–19, the seeder filled the two gaps at 75.000 and
68.000, and the resulting ladder charged **more per portion for five meals than
for one**. The cart's "add four more and every portion drops" nudge would have
been a lie on screen. `seed-prices` now aborts on that and names the rows.

`replace` deliberately supersedes EVERY active row rather than sparing those
with a `created_by`. The first version spared them — and the two rows that made
the ladder incoherent had one, so `replace` achieved nothing while looking like
it had. A discriminator that does not separate seed data from real data is
worse than none.

## M16 — Documents ✅ / 🟡
✅ `12-security` (control map with the test that proves each) ·
✅ `14` deployment handbook · ✅ `15` user guide · ✅ `16` admin guide ·
✅ `11-i18n` (languages, locale negotiation, the selector, and what is *not*
translated) ·
⬜ `01-PRD`, `02-business-rules` with `BR-x.y` ids, `03-data-model`,
`04-api-specification`, `05`, `06`, `07`, `08`, `13a` — the planning
documents cover their content but the house numbering has not been folded in
(see `04-milestones.md` §2).

---

## Not built — be clear about it

| | Why it matters |
|---|---|
| **Turnstile CAPTCHA** | Chosen, needs a key. Registration and login are rate-limited meanwhile, which is weaker. |
| **UU PDP export/deletion** | Required before launch. Note the tension: tax records must be kept, so deletion should anonymise rather than delete. |
| **WhatsApp sending** | WAHA chosen and the queue is generic, but no sender number exists so the channel is off. |
| **Off-machine backup copy** | `scripts/backup.sh` runs and `scripts/restore-check.sh` has been run against a real dump, but the copy still lands on the SAME MACHINE as the database. The S3/rclone line is commented out awaiting a bucket. A backup that dies with the server is not a backup. |
| **CI has never run on a runner** | `.github/workflows/ci.yml` exists and every step in it passes locally, but no push has exercised it on GitHub Actions. Until it does, treat the pipeline as written-not-proven. |
| **§13 extras** | TDEE calculator, allergen warnings, vouchers, ratings, nutrition chart — all phase 2 per D-23. |

### Recently completed (previously in the table above)

Corrected because the list had gone stale and a stale "not built" is as
misleading as a false ✅:

| | Evidence |
|---|---|
| **Customer SPA + back-office screens** | Login (incl. 2FA step), Register, Menu/cart, Addresses, Orders, OrderDetail, Packages, Security, AdminPayments, AdminDeliveries, AdminSettings. Driven in a real browser at 390px and 1280px. |
| **Notification queue** | Postgres queue with `FOR UPDATE SKIP LOCKED` and backoff; real mail delivered end-to-end through the relay and logged `SENT`. |
| **Object storage** | Private MinIO bucket, server-generated keys, magic-byte type detection, presigned reads. Verified: presigned 200, bare URL 403. |
| **Delivery lifecycle transitions** | Domain state machine plus kitchen/courier endpoints, kitchen-scoped. |
| **TOTP two-factor** | See M4. |
| **Backup + restore drill** | `scripts/backup.sh` verifies the gzip stream and the dump's completion marker; `scripts/restore-check.sh` restores the newest dump into a scratch database and runs the security suite against the restored copy. Both have been run. |

## Blocked on Steven

Answered 2026-08-13 (second batch) — see `docs/02-decisions.md` Part 0c:

| # | Needed | Status |
|---|---|---|
| 1 | Real kitchens | ✅ five, geocoded from the real Maps key (migration 0014) |
| 2 | Bank account | ✅ Nobu 16830226665, PT Sunshine Food International |
| 3 | Google Maps keys | 🟡 one key supplied and working; **unrestricted and unsplit** — see below |
| 4 | SMTP relay + SPF/DKIM/DMARC | ⬜ **still open** — the key given under this heading was the Maps key |
| 5 | Domain + production host | 🟡 `dev.evermore.co.id` chosen; DNS does not resolve yet |
| 6 | Legal entity, NPWP | 🟡 name and address landed; **NPWP is a placeholder**, PKP status unanswered |
| 7 | Reversed-out logo | 🟡 deferred by Steven — text wordmark stays for now |
| 8 | WhatsApp sender | 🟡 wired to the shared WAHA; **session is FAILED, must be re-linked** |

### What is still genuinely blocking

| | Why it matters |
|---|---|
| **SMTP relay** | Item 4 was never answered — the key pasted there was the Google Maps key. Real email still goes nowhere but the local trap. |
| **PKP status** | If the company is not PKP, charging 11% PPN is not permitted. This is a legal answer, not a code change; the rate is already a setting. |
| **A real NPWP** | `123 123 123` is stored because it was given, but a real NPWP is 15 digits (16 since the NIK migration). It must not reach a faktur pajak. |
| **Maps key hygiene** | One key does both browser and server duty and allows `0.0.0.0/0`. It ships in page source, so anyone viewing source can spend against it. Split and cap before anything public. |
| **DNS for `dev.evermore.co.id`** | nginx and `APP_BASE_URL` are set; the name does not resolve, so the host answers on its IP today. |
| **WAHA session** | Bound to the right number (628176315568) but reporting `FAILED`. WhatsApp queues and retries; nothing sends until someone re-scans the QR. |
