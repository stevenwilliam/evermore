# Evermore — build status

✅ done & tested · 🟡 partial · ⬜ not started

**A ✅ here has been re-earned by running the gate in THIS repo** (CLAUDE.md §5).
Nothing is inherited from `healthy_catering`. The previous contents of this
file described that build; it was replaced on 2026-09-01 when this one started.

**Last updated:** 2026-09-01, during the build.

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
| M2 | Schema | ✅ | 6 migrations applied to a real PG 18. 58 tables, 86 CHECK, 79 FK, 380 NOT NULL, 6 EXCLUDE, 4 partial uniques, 3 append-only triggers |
| M3 | Domain layer | ✅ | 6 pure packages, all matrices from `01-domain-model.md` §5 asserted item-by-item |
| M4 | Identity, RBAC, audit | 🟡 | Schema, 26 permissions and 5 deny-by-default roles seeded; argon2id + JWT + refresh rotation written and unit-tested. **No login endpoint yet** |
| M5 | Master data & settings | 🟡 | 26 `sys_parameters` seeded and read by the site. **No admin CRUD yet** |
| M6 | Catalogue & menu calendar | 🟡 | Schema, seed and the public read path work. **No back-office editor yet** |
| M7 | Pricing | 🟡 | Resolver + tier validation unit-tested; four price tables with EXCLUDE constraints proven. **No admin forms yet** |
| M8 | Ordering | ⬜ | Domain state machine done; no cart, checkout or order write path |
| M9 | Payments | ⬜ | Schema and the suffix algorithm done; no upload, no verification queue |
| M10 | Packages & credits | 🟡 | Ledger rules unit-tested, demo package seeded and balance verified at 13. **No purchase or redemption endpoint** |
| M11 | Reports & CSV export | 🟡 | `platform/csvexport` written and tested (pipe-delimited, formula-guarded). **No reports built on it yet** |
| M12 | Notifications | ⬜ | Tables exist; no sender |
| M13 | Public site (SEO) | ✅ | 7 routes live, robots + sitemap + JSON-LD, 14 page/viewport combinations probe clean |
| M14 | Security hardening | 🟡 | Headers, CORS, body limits, trusted proxies, CSP live and verified through nginx. **No authz tests, no full ASVS pass** |
| M15 | Deployment (dev server) | ✅ | systemd + nginx installed, enabled, verified on the deployed URL |
| M16 | Documents | 🟡 | This file, the decision log and `02-business-rules.md` current; guides still describe healthy_catering |
| M17 | Print artefacts | ⬜ | Production sheet and packing labels not built |
| M18 | Demo data | ✅ | Idempotent seed; meal panel and credit balance verified by reading back |

---

## Gates that were actually run

| Gate | Result |
|---|---|
| `go test ./...` | **10 packages pass** |
| `go test ./test/... -shuffle=on` | **passes, 6 consecutive runs** — order-independent |
| Oversell test | 40 concurrent writers vs capacity 20 → **20 win, reserved lands 20/20** |
| Order numbering | 50 concurrent callers → **50 distinct numbers** |
| `scripts/contrast.py` | **24/24 pairings match design.md §3** |
| `tools/shot` on the deployed URL | **14/14 clean** — 0 contrast failures, 0 bar-rule violations, no overflow, no undersized tap targets |
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

The transactional surface does not exist yet. Specifically: there is no login,
no cart, no checkout, no payment upload, no verification queue, no back office,
no reports, no notifications and no print artefacts. The `/app/*` links in the
public site's navigation currently 404.

## Blocked on Steven

| # | Item | Effect |
|---|---|---|
| 1 | TLS for the deployed host | HSTS, COOP and `upgrade-insecure-requests` are all withheld off production. The headers are correct and inert until TLS terminates. |
| 2 | Real bank account | `sys_parameters` carries the artifact's dummy BCA 5391184402. |
| 3 | WhatsApp sender number | WAHA is wired and switched off. |
| 4 | Meal photography | Every card shows a "foto meal" placeholder. |
| 5 | PKP registration, NPWP, legal entity name | Needed before a corporate invoice is correct (`03-open-questions.md` Q-1a). |
| 6 | Production domain | `APP_BASE_URL` is the dev server's IP, which is what canonical and Open Graph URLs currently carry. |
