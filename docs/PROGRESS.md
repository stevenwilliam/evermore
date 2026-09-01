# Evermore — build status

✅ done & tested · 🟡 partial · ⬜ not started

**A ✅ here has been re-earned by running the gate in THIS repo** (CLAUDE.md §5).
Nothing is inherited from the prior build. The previous contents of this file
described that build; it was replaced on 2026-09-01 when this one started.

**Last updated:** 2026-09-01, end of the first build session.

---

## Deployed right now

| | |
|---|---|
| URL | **http://192.168.88.101:8091** |
| Service | `evermore-app.service` — active, enabled, survives reboot |
| API | `127.0.0.1:8082`, loopback only, unreachable from the LAN |
| nginx | `/etc/nginx/sites-available/evermore-app`, listening on 8091 |
| Config | `/etc/evermore/app.env` (0640 root:dev) |
| Database | `evermore` + `evermore_test`, own role, independent of every other project |
| Satellites | Redis db 1, MinIO bucket `evermore-app`, mailpit 1025, WAHA (off) |
| Coexists with | `evermore.service` (healthy_catering) on :8090 — untouched, still 200 |

---

## Modules

| # | Module | State | What was actually run |
|---|---|---|---|
| M1 | Environment & config | ✅ | Service boots from `/etc/evermore/app.env`; startup line logs every secret masked |
| M2 | Schema | ✅ | 6 migrations on a real PG 18. 58 tables, 86 CHECK, 79 FK, 380 NOT NULL, 6 EXCLUDE, 4 partial uniques, 3 append-only triggers |
| M3 | Domain layer | ✅ | 6 pure packages; every matrix in `01-domain-model.md` §5 asserted item-by-item |
| M4 | Identity, RBAC, audit | 🟡 | Login, refresh rotation with reuse detection, lockout, registration, deny-by-default permissions across 5 roles — all tested end-to-end. **No staff TOTP, no audit-log writes on business events yet** |
| M5 | Master data & settings | 🟡 | 26 `sys_parameters` seeded and read throughout. **No admin CRUD screen** |
| M6 | Catalogue & menu calendar | 🟡 | Public read path, back-office calendar and publish action all work. **No meal/food editor** |
| M7 | Pricing | 🟡 | Resolver, tiers and the four tables proven; live cart resolves 78/75/71 correctly. **No admin price forms** |
| M8 | Ordering | ✅ | Quote and checkout with tier-on-cart-total, tax split, routing, capacity under lock, idempotency. Driven end-to-end on the deployed site |
| M9 | Payments | ✅ | Instructions, proof upload with byte-sniffed type, verification queue, verify/reject, expiry releasing capacity and suffix |
| M10 | Packages & credits | 🟡 | Ledger rules tested; balance and history render; purchase activates on verification. **No purchase checkout or credit redemption flow** |
| M11 | Reports & CSV export | 🟡 | `platform/csvexport` written and tested (pipe-delimited, formula-guarded). **No report screens built on it** |
| M12 | Notifications | ⬜ | Tables exist; no sender |
| M13 | Public site (SEO) | ✅ | 7 routes, robots + sitemap + JSON-LD, one h1 per page |
| M14 | Security hardening | 🟡 | Headers, CSP, CORS, rate limits, trusted proxies, IDOR scoping, negative authz tests. **No full ASVS L2 pass, no `12-security.md` rewritten for Evermore** |
| M15 | Deployment (dev server) | ✅ | systemd + nginx, enabled, verified on the deployed URL |
| M16 | Documents | 🟡 | This file, the index, the decision log and the handbooks all repointed to Evermore. **Guides 15/16 still describe features this build does not have yet** |
| M17 | Print artefacts | ⬜ | Production sheet and packing labels not built |
| M18 | Demo data | ✅ | Idempotent seed, verified by reading back |

---

## Gates that were actually run

| Gate | Result |
|---|---|
| `go test ./...` | **10 packages pass** |
| `go test ./test/... -shuffle=on` | **passes, 6 consecutive runs** — order-independent |
| Oversell test | 40 concurrent writers vs capacity 20 → **20 win, reserved lands 20/20** |
| Order numbering | 50 concurrent callers → **50 distinct numbers** |
| `scripts/contrast.py` | **24/24 pairings match design.md §3** |
| `tools/shot` on the deployed URL | **30/30 clean** — 0 contrast failures, 0 bar-rule violations, no overflow, no undersized tap targets, across public, customer and back-office screens |
| Full purchase flow on the deployed site | cart Rp 450.000 → order → **Transfer Rp 450.926, BCA 5391184402, suffix 926** |
| Back office as ops manager | dashboard, menu calendar, payment queue all 200; the **same pages 403 for a customer token** |
| Concurrent checkout | 12 checkouts vs 4 reachable portions → exactly 4 succeed, 0 oversold |
| Seed idempotency | second run reports **0 new rows** |
| Migration down-file coverage | every `.up.sql` has a matching `.down.sql`, asserted by the loader |

---

## Defects found by running, and fixed

Recorded because each was invisible in the source.

1. **`int4range(min, COALESCE(max, 2147483647), '[]')` overflows int4** — an
   inclusive upper bound is normalised by adding 1. Found by a constraint test,
   fixed to half-open before it shipped anywhere.
2. **The integration suite was order-dependent** — an unbounded tier band
   covered every band allocated after it. Found with `-shuffle=on`.
3. **1.05:1, text effectively invisible** — cards inside the dark section
   inherited beige ink and painted it on their own white ground. Found by
   probing computed styles, not by reading CSS.
4. **1.69:1 eyebrow** — `.section-head p` (0,1,1) silently beat `.eyebrow`
   (0,1,0). A specificity collision, exactly the class design.md §4 warns about.
5. **Menu cards priced at the cheapest tier** — Rp 71.000 shown next to a
   button that charges Rp 78.000 for one portion. Found by looking at the page.
6. **`upgrade-insecure-requests` broke every asset on the deployed URL** — the
   browser rewrote all subresources to https:// on a plain-HTTP host. Invisible
   on localhost, which browsers treat as already trustworthy.
7. **`sanitize.Email` accepted a dotless domain** the DB CHECK refused, turning
   a field error into a 500.
8. **A dropped `.Scan`** on `SinglePortionPrice` — caught by the compiler, but
   it is the silent-failure class rule 2 exists for.

---

## Not built — be clear about it

The commercial path works end to end: browse → cart → checkout → transfer
instructions → proof upload → staff verification → PAID. What does **not**
exist yet:

- **Package purchase and credit redemption.** A package can be seeded and its
  balance renders, and verification activates one, but there is no screen to
  buy a package or to spend a credit on a slot.
- **Reports.** `platform/csvexport` is written and tested; nothing is built on
  it. No CSV export button exists anywhere yet.
- **Notifications.** No email or WhatsApp is ever sent. The tables are there.
- **Print artefacts.** No kitchen production sheet, no packing labels.
- **Admin CRUD.** No screens for settings, prices, foods, meals, kitchens,
  customers or users. The seed is currently the only way data gets in.
- **Staff TOTP.** The schema and encryption key exist; the flow does not.
- **Audit-log writes.** The table and its append-only trigger exist and are
  tested, but business events do not yet write to it.
- **Delivery fulfilment.** No screen moves a delivery through
  PREPARING → OUT_FOR_DELIVERY → DELIVERED.

## Blocked on Steven

| # | Item | Effect |
|---|---|---|
| 1 | TLS for the deployed host | HSTS, COOP and `upgrade-insecure-requests` are all withheld off production. The headers are correct and inert until TLS terminates. |
| 2 | Real bank account | `sys_parameters` carries the artifact's dummy BCA 5391184402. |
| 3 | WhatsApp sender number | WAHA is wired and switched off. |
| 4 | Meal photography | Every card shows a "foto meal" placeholder. |
| 5 | PKP registration, NPWP, legal entity name | Needed before a corporate invoice is correct (`03-open-questions.md` Q-1a). |
| 6 | Production domain | `APP_BASE_URL` is the dev server's IP, which is what canonical and Open Graph URLs currently carry. |
| 7 | **Ruling on D26** | The transactional screens are server-rendered Go templates, not the React SPA that D13 reserved for them. Reversible; the API exists either way. |
