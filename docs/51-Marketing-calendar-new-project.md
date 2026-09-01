# Kick-off prompt — internal superapp (Maxx Coffee · Ruuma · Sunshine)

**What this file is.** A self-contained brief to paste into a fresh Claude
session in the new repository. It carries Steven's working preferences, the
product definition, and the questions that must be answered before any code is
written.

**How to use it**

1. Create the new repo, `git init`, `main` as the working branch.
2. Copy `99-steven-preference.md` into `docs/` there.
3. Paste **everything from "PROMPT BEGINS" to the end of this file** into a new
   Claude session in that repo.
4. Answer the question batch in §9. A line beginning `ven:` is an answer to the
   question above it; `all defaults` takes every proposed default.
5. Claude then builds the doc set, then the modules, without stopping.

Written 2026-09-01. Nothing in this file has been built — it is a brief.

---

---

# PROMPT BEGINS

You are starting a new project. Read this whole brief before doing anything,
then ask me the question batch at the end — all of it at once, nothing before.

## 1. Who I am and how to work with me

- Call me **Steven** (nickname **ven**).
- When I paste your question list back, **a line beginning `ven:` is my answer**
  to the question directly above it. "all defaults" means take every default.
- I answer fast and short. A one-word `yes` is a real decision — move.
- **`coding stop`** means change nothing until I say **`coding start`**. It is a
  hard gate across turns. Reading, searching and planning stay fine.

**How I want you to work**

- **Ask everything at once, up front, with a proposed default per question.**
  One batch before starting, never a drip mid-build.
- **Once the docs and business rules are agreed, build to the end without
  stopping.** Do not pause for milestone approval. A blocker does not stop the
  build: work around it, note it, keep going on everything that does not depend
  on it, and hand me the whole list at the end.
- **Auto-commit and push after every completed change.** Small, focused,
  conventional-commit messages. `main` is the working branch.
- **Update the docs in the same commit as the change.** A decision that is not
  in the docs did not happen.
- **Tell me the truth about what was verified.** "Done" means *run*, not
  written. If a test did not run, say so and put the step in
  `docs/RUN-WHEN-BACK.md`.
- **Flag consequences I did not ask about** — an abuse case, a hole in a state
  machine, a contradiction with an earlier decision. One or two sentences, a
  proposed fix, then carry on.
- `vi` in every runbook, never `nano`. Absolute paths in every OS instruction.
- Install the **`impeccable` skill** at `.claude/skills/impeccable/SKILL.md`
  before writing any code. It is the standard of work and it is worth most on
  day one.

## 2. What we are building

An **internal superapp** for a group that operates three brands:

| Brand | Business |
|---|---|
| **Maxx Coffee** | coffee shops |
| **Ruuma** | restaurants |
| **Sunshine** | catering |

One application, many capabilities, **self-hosted and not public facing**. Every
user is a staff member. There is no customer-facing surface, no public sign-up
and no anonymous access.

**The approach is modular.** Each capability is a module that can be enabled per
company, and modules share one platform: identity, master data, approvals,
notifications, reporting and audit.

**Phase 1 is the Marketing Calendar module** (§5 below), built on the shared
data source (§4) and the platform modules (§3). Everything else is roadmap.

### Deviations from my usual defaults, because this is internal

- **No SEO.** §13 of my preference file does not apply — there is no public
  page. Do not build titles, Open Graph, `robots.txt`, `sitemap.xml` or JSON-LD.
- **No customer self-registration.** Accounts are created by an administrator.
- Everything else in `99-steven-preference.md` stands: hexagonal Go, PostgreSQL,
  integer money, UUIDv7, numbered SQL migrations, deny-by-default authorisation,
  ASVS L2, pipe-delimited CSV export, a search box on every list, configurable
  values in `sys_parameters`, WCAG AA with measured contrast, mobile-first with
  light and dark tokens, message catalogues rather than inline strings.

## 3. Module architecture

Two layers. Get this right first — every later module depends on it, and the
value of a superapp is that the twentieth module is cheap because the first one
built these properly.

**Platform modules** — built once, reused by every business module:

| Module | Responsibility |
|---|---|
| `identity` | users, roles, permissions, sessions, staff MFA |
| `masterdata` | company, site, site group, and their scoping rules |
| `approval` | **a generic, configurable approval-chain engine** |
| `notify` | email and WhatsApp behind one port, with recipient lists |
| `reporting` | query → grid → pipe-delimited CSV, one implementation |
| `audit` | append-only log of who did what, when, from where |
| `params` | `sys_parameters` with admin CRUD |
| `importer` | ingesting transaction data, idempotently |

**Business modules** — phase 1 is the first row:

| Module | Phase |
|---|---|
| Marketing calendar (targets, promotions, approvals, reports) | **1** |
| Suggested roadmap in §8 | later |

> **The approval engine must be generic from day one.** A superapp will need
> approvals for purchase orders, leave, discounts, write-offs and price changes.
> If the chain is written inside the promotion module it gets rewritten five
> times. Build it as `approval` with a subject type, and let promotions be its
> first consumer.

## 4. Data source

The shared master and fact data. Field lists below are mine; extend them where
the model needs it, and tell me what you added.

```
company        (company_id, company_name)
site           (site_id, site_name, site_type)
site_group     (site_group_id, site_group_name)
history_txn    (date, site_id, selling_type[normal|promo], promo_id, amount)
role
app_user
user_role
```

- `site_type` distinguishes coffee shop / restaurant / catering, and is what
  lets a module apply to some sites and not others.
- **Every site gets its own site group automatically when the site is created**,
  so a single-site promotion needs no set-up.
- `history_txn.amount` is money: **integers, `BIGINT`, whole rupiah**, no
  floating point anywhere near it.

### Four problems with this data model — please fix them, do not implement them as written

1. **`site_group` as specified cannot hold a group.** Putting `site_id` on the
   `site_group` row makes it one-site-per-group, but a promotion targets a group
   of stores. It must be a many-to-many: `site_group` plus
   `site_group_member(site_group_id, site_id)`, with the auto-created
   per-site group being a group that happens to have one member.

2. **Receipt count is missing from the fact table.** Promotions carry a *target
   total receipt count*, and there is nothing to measure it against. The fact
   table needs a receipt count — or, better, transaction-level rows that can be
   counted.

3. **Order mode is missing from the fact table.** A promotion is planned for
   dine-in or takeaway, so actuals have to be attributable to one or the other,
   or the promotion report cannot be produced.

4. **Nothing says where `history_txn` comes from.** It is presumably exported
   from a POS. That means an ingestion path — file import or API — with
   idempotency (re-importing the same day must not double the sales), a
   reconciliation view, and a record of what was imported when. This is a real
   module, not a detail.

## 5. Phase 1 — the Marketing Calendar module

### 5a. Targets

Targets are maintained for **normal sales** and **promo sales** separately.

- **Year target** — set first, for the year.
- **Month target** — the next layer down.
- The twelve month targets **do not have to sum to the year target.** Over or
  under is legitimate and must not be blocked. Show the variance; do not
  enforce it.

### 5b. Promotions

**Creating a promotion plan.** The user supplies:

- the promo period, as a **date-range picker**, which may span several months
- the promo name
- the site group
- target sales amount for the promotion
- target total receipt count
- order mode: dine-in or takeaway
- the promo rule, as a **large free-text area**

**Lead time.** The earliest a promo may start is **7 working days** from today —
if today is 1 September 2026, the earliest start is 10 September 2026. The 7 is
**configurable in the back office**.

**Auto-cancel.** On the **5th day before the promo starts**, a scheduled job
cancels any promotion whose approval is still incomplete.

**Approval.** A created plan enters an approval chain before it is released.

- The chain is **configured in the back office**, and supports a step that any
  one of several roles can satisfy:
  `Role_1 → Role_2 → (Role_3 OR Role_4) → Role_5`
- **Changing the chain affects only new plans.** A plan in flight keeps the
  chain it started with.
- A **superadmin can force-release** all approvals.
- When the last approver approves, **an email is sent automatically**, to a
  recipient list maintained in the back office.

### 5c. Reports

A promotion report, showing planned versus actual. **Every report exports to CSV
with a pipe (`|`) delimiter**, honouring the on-screen filters and search.

### Eight things §5 does not settle — decide them, tell me what you chose

1. **"Working days" needs a holiday calendar.** Counting weekdays alone will
   compute the wrong lead time around Idul Fitri, Christmas and Nyepi. This
   needs an Indonesian public-holiday table that an administrator can maintain.

2. **Your sentence "if promo not approved until the last chain" is unfinished.**
   Read together with the 5-day job, I assume it means it is auto-cancelled.
   Confirm.

3. **Can an approved promotion be edited?** If it can, the approval means
   nothing. Proposal: approval **locks** the plan; any change creates a new
   version that re-enters the chain, and the previous version is retained.

4. **May two promotions overlap** on the same site group and dates? This is the
   single most common real-world mess in promo planning. Proposal: allow it but
   **warn loudly** at creation and flag it on the calendar, because a genuine
   stacked promotion is sometimes intended.

5. **What is a target attached to?** Company, site, or site group, and for which
   sales type. Proposal: **per site, per month, per sales type**, rolled up to
   group and company for display.

6. **Do promo sales count toward the month target?** Proposal: yes — normal plus
   promo equals total, and the report shows all three.

7. **Auto-cancel is an automated destructive action.** My own rule is that
   nothing automated cancels a customer's booking. A promo plan is not a
   customer booking, so the rule does not bite — but it should still be
   audited, notify the creator and the pending approver, be configurable, and be
   revivable by a superadmin rather than deleted.

8. **Force-release needs a reason and an audit row.** A superadmin bypassing an
   approval chain with no recorded justification is the control most likely to
   be questioned later.

### Non-negotiable engineering for this module

- **Approval state is a state machine in the domain layer**, with the illegal
  transitions rejected there and the transition table as a unit test.
- **Concurrency is tested.** Two approvers acting at once, and the same approver
  acting twice, must not both succeed. `SELECT … FOR UPDATE` in one transaction,
  with a test that proves it.
- **The approval history is append-only** — no updates, no deletes, and the
  migration says so.
- **Every amount is `BIGINT` integer rupiah**, with money paths in explicit SQL.
- **Every list has a debounced search box. Every grid exports pipe-delimited
  CSV**, guarded against formula injection (`=`, `+`, `-`, `@`, tab, CR).
- **Deny by default.** Every handler declares its permission; every query is
  scoped by company and site. Negative authorisation tests per role.

## 6. Stack and architecture

Hexagonal, dependencies pointing inward only: `adapter → app → domain`, with
`platform` available to all. The domain imports no framework, no driver, no
`net/http` and no SQL.

```
cmd/api/main.go          # thin: wire + run (serve, migrate, seed)
internal/
  domain/                # pure logic, exhaustively unit-tested, no I/O
  app/                   # use-cases
  adapter/http|postgres|storage|notify
  platform/config logging metrics apierror id security ratelimit database
db/migrations/NNNN_name.up.sql + .down.sql
  embed.go               # go:embed
web/                     # React 18 + Vite + TypeScript + Tailwind
```

Go (latest) · `gin` · `gorm` (+ raw SQL on money paths) · PostgreSQL (latest
major) · `golang-jwt/jwt/v5` · `google/uuid` v7 · Prometheus · React 18 pinned,
Node 20.

**Not defaults — do not reach for them:** automigrate as the source of truth,
GraphQL, microservices, Kubernetes, a NoSQL primary store, SSR, CSS-in-JS.

**Database:** UUIDv7 keys, numbered forward-only migrations as the source of
truth, `timestamptz` in UTC with business dates converted to Asia/Jakarta
explicitly, the database enforcing invariants through `CHECK`, foreign keys and
partial unique indexes, append-only history tables, and multi-tenant scoping as
a column plus an index plus a repository-layer filter.

**Infrastructure:** dev on the shared `claudedev` server at
`/home/dev/projects/<project>`, config at `/etc/<project>/<project>.env`, nginx
reverse-proxying a local port, PostgreSQL native and shared (one database plus
`<project>_test`), Docker only for satellites (MinIO, mailpit, WAHA).

## 7. What to deliver

1. `CLAUDE.md`, generated from `99-steven-preference.md` plus this brief.
2. The numbered doc set: `00` decision log · `01` PRD · `02` business rules
   (normative, `BR-x.y` IDs referenced by code and tests) · `03` data model ·
   `04` API spec · `05` architecture and NFR · `06` domain operations ·
   `07` test plan · `08` roadmap · `09` deployment · `10` design system with
   measured contrast · `11` local dev · `12` security (ASVS L2 map, each control
   tied to the test that proves it) · plus `PROGRESS.md` and `RUN-WHEN-BACK.md`.
3. Then build every module A→Z without stopping: platform modules first, then
   the marketing calendar.
4. Then test, debug and security-harden A→Z.
5. Then the deployment handbook, the user guide and the admin guide, in that
   order.

## 8. Suggested roadmap — tell me which you want

Offered because you said additional features are welcome. Each reuses the
platform modules, which is the argument for building those properly first.

**Close to the marketing calendar**

- **Promo performance vs target dashboard** — actual against plan, by site,
  group, brand and order mode, with the variance highlighted.
- **Promo P&L** — discount cost against incremental sales, so a promotion that
  raised revenue and destroyed margin is visible.
- **Cannibalisation view** — normal sales during a promo compared with the
  preceding weeks.
- **Promo calendar wall** — a month grid across all brands, which is the screen
  a marketing manager actually lives in.

**Operations**

- Daily sales close and cash reconciliation
- Inventory, stock take and wastage
- Purchasing and goods receipt, reusing the approval engine
- Recipe / bill of materials and food cost
- Supplier master and price lists

**People**

- Staff roster and shift planning
- Leave requests, reusing the approval engine
- Attendance

**Governance**

- Document repository with expiry alerts (permits, halal, certifications)
- Asset register and maintenance schedule
- Incident and complaint log

## 9. Questions — answer these before anything is built

Answer with `ven:` under each, or `all defaults`.

**Scope and rollout**

1. Is phase 1 **only** the marketing calendar, with the platform modules built
   to support it? *Default: yes.*
2. All three brands from day one, or Maxx Coffee first?
   *Default: all three — the model is multi-company from the start either way.*
3. Roughly how many sites per brand today, and the expected ceiling?
   *Default: assume up to 200 sites total.*

**Data**

4. Where does `history_txn` come from, and how does it arrive — nightly CSV
   drop, POS API, or manual upload? *Default: nightly CSV import with an
   idempotent, re-runnable importer and a reconciliation screen.*
5. Grain of the fact table: one row per receipt, or one aggregated row per
   site/day/selling-type/order-mode? *Default: **per receipt**, because receipt
   count is a promo target and aggregates cannot be un-summed later.*
6. Currency: rupiah only? *Default: yes, `BIGINT` whole rupiah.*
7. How far back does history need loading? *Default: 24 months.*

**Targets**

8. Targets are set per site, per month, per sales type, and roll up.
   *Default: yes.*
9. Who sets targets, and does setting one need approval? *Default: a Finance or
   Marketing Head role sets them; no approval chain in phase 1.*

**Promotions and approval**

10. Confirm: a promo whose chain is incomplete on the 5th day before start is
    **auto-cancelled**. *Default: yes — audited, notifying creator and pending
    approver, and revivable by a superadmin.*
11. Approval locks the plan; an edit creates a new version that re-enters the
    chain. *Default: yes.*
12. Overlapping promos on one site group: allow with a loud warning.
    *Default: yes.*
13. What are the actual roles and the real chain? *Default: Marketing Staff
    creates → Marketing Manager → Finance Manager → (Brand Head **or**
    Operations Head) → Director, with Superadmin able to force-release.*
14. Rejection: does it return to the creator for editing, or kill the plan?
    *Default: returns to the creator with a mandatory reason; the plan keeps its
    history.*
15. Notification channels: email only, or WhatsApp too? *Default: email in phase
    1, WhatsApp behind the same port for later.*

**Access and environment**

16. How do staff reach an internal app — office LAN, VPN, or IP allowlist?
    *Default: nginx on the internal network with an IP allowlist, TLS on.*
17. MFA for staff: mandatory, or admin-only? *Default: mandatory TOTP for every
    staff account, since one login sees all three brands' sales.*
18. Can a user belong to more than one company, and see across brands?
    *Default: yes — a user has roles scoped per company, and a group-level role
    sees all.*
19. Languages: Indonesian, English, or both? *Default: both, via message
    catalogues, Indonesian as the default.*
20. Operating timezone. *Default: Asia/Jakarta, storage in UTC.*

**Working days**

21. Confirm the holiday calendar is needed, and that an administrator maintains
    it. *Default: yes — Indonesian public holidays, editable, seeded for the
    current and next year.*

---

*Answer the batch and I will build the documents, then every module, A→Z,
without stopping.*
