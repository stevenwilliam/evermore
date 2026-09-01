# 99 — Steven's preferences & engineering DNA (portable)

**Owner:** Steven ("ven") · itdept.sfg@gmail.com
**Version:** 1.0
**Date:** 2 August 2026
**Scope:** deliberately **project-agnostic**. Everything here is how *I* want
software built, not anything specific to ruuma. Copy this file into any new
project unchanged.

---

## 0. How to use this file in a new project

1. Copy this file to `docs/99-steven-preference.md` in the new repo.
2. Generate that project's `CLAUDE.md` from sections 3–9 below, then add only
   the project-specific parts (domain, locale, stack deviations).
3. Create the numbered doc set from section 10.
4. **Install the `impeccable` skill** (section 15) at
   `.claude/skills/impeccable/SKILL.md` before writing any code. It is the
   standard of work, and it is worth most on day one.
5. Ask me the open product questions **in one batch, each with a proposed
   default** (section 2), then build.

Where this file conflicts with a project's own `CLAUDE.md`, the project wins —
it is the newer, more specific decision. Where it conflicts with a habit,
this file wins.

---

## 1. Who I am and how I answer

- Call me **Steven**; my nickname is **ven**.
- When you send me a list of questions and I paste it back, **a line beginning
  `ven:` is my answer** to the question directly above it. Everything after
  `ven:` is my instruction.
- I answer fast and short. Terse does not mean unconsidered — take a one-word
  `yes` as a real decision and move.
- If I say "all defaults", take every default you proposed and go.
- I write in English and Indonesian; the doc set stays in English.

### My control words

| I say | You do |
|---|---|
| **`coding stop`** | **Change nothing.** No edits, no new files, no commits, no migrations, no deploys, no config changes — until I say `coding start`. |
| **`coding start`** | The hold is lifted. Resume normally. |

`coding stop` is a hard gate, not a preference to weigh against the task. It
holds across turns until I lift it — a new request while it is on is a request
to *discuss and plan*, not a licence to resume. If I ask for something that
needs a change while the hold is on, tell me what you would do and wait.

Reading, searching, running read-only commands, answering questions, drafting a
plan and explaining trade-offs are all still fine. What stops is anything that
writes: the filesystem, the database, a running service, or a remote.

If you are unsure whether the hold is still on, it is. Ask.

---

## 2. How I want Claude to work

- **Ask everything at once, up front, with a default per question.** One batch
  before starting, not a drip of questions mid-build. Every question carries a
  proposed default so I can answer "yes" or "all defaults".
- **Never stop partway.** If the plan says "build all modules A–Z", build all of
  them in one push. Do not deliver two modules and ask whether to continue.
- **Once the documents, requirements and business rules are agreed, BUILD TO
  THE END WITHOUT STOPPING.** The planning phase is where I answer questions;
  the build phase is where you work. During the build:
  - Do not stop to ask me to confirm a milestone, review an interim result, or
    choose between two reasonable options. Pick the better one, write down why,
    and keep going.
  - **A blocker does not stop the build.** Work around it, note it, and carry
    on with everything that does not depend on it — then hand me the whole list
    at the end. One batch of blockers after a finished build beats five
    interruptions during it.
  - **Fine-tuning and correction come after, not during.** Something imperfect
    that works is a note for the end; only something *wrong* gets fixed on the
    spot.
  - Report at the end: what was built, what was verified by running it, what is
    still blocked and on whom. That is when I answer.
  This is the single thing that most changes how much gets done in a session:
  every stop costs a context switch for both of us, and I would rather read one
  honest report than approve nine checkpoints.
- **Auto-commit and push after every completed change**, without asking. Small,
  focused commits, conventional-commit messages (`feat(...)`, `fix(...)`,
  `docs(...)`). `main` is the working branch unless I say otherwise.
- **Update the related documents on every interaction** — including talk-only
  turns that settle a decision, in the same commit as the change. A decision
  that isn't in the docs didn't happen.
- **Tell me the truth about what was verified.** If a test didn't run because a
  tool is missing, say so and put the step in `RUN-WHEN-BACK.md`. Never report
  "done and tested" for something you only wrote.
- **Flag consequences I didn't ask about.** If my answer creates an abuse case,
  a hole in a state machine or a contradiction with an earlier decision, say so
  in a sentence or two, propose the fix, and keep going.
- **Anything needing an interactive terminal** (Docker, live servers, approval
  prompts, OAuth consent screens) goes into `docs/RUN-WHEN-BACK.md` as
  copy-paste steps — written, not guessed at.
- **`vi` is the editor** in every runbook, shell instruction and docs example.
  Never `nano`.
- **OS/server guides use full absolute paths**, never relative ones, so a
  copy-pasted command can never run in the wrong directory.
- Prefer editing existing files and reusing `platform/*` over new scaffolding.

---

## 3. Delivery workflow

The sequence I run every project through:

1. **Initial git setup** — repo, remotes, conventions, `CLAUDE.md`.
2. **Steven — preparation.** I give PRD and business-rules feedback, tuning and
   final confirmation. Nothing downstream starts until I confirm.
3. **Claude — build all documents A→Z** from the confirmed PRD/business rules.
4. **Claude — build all modules in one shot, A→Z.** Every module end to end.
5. **Claude — test, debug and security-harden, A→Z.** The whole system.
6. **Claude — production deployment handbook** (copy-paste, empty machine, full
   absolute paths), **then** the user guide, **then** the admin guide.

---

## 4. Architecture

Hexagonal / clean layering, dependencies pointing **inward only**:
`adapter → app → domain`, with `platform` available to all. The domain imports
no framework, no driver, no `net/http`, no SQL.

```
cmd/api/main.go            # thin entrypoint: wire + run subcommands (serve, migrate, seed)
internal/
  domain/                  # pure business logic + types; exhaustively unit-tested; no I/O
  app/                     # use-cases / services; orchestrates domain + ports
  adapter/
    http/                  #   handlers, request/response mapping
    postgres/              #   repositories (raw SQL on money paths)
    storage/               #   S3 / MinIO
    notify/                #   email / WhatsApp / outbound
  platform/                # cross-cutting infra, business-agnostic, reusable across projects
    config/ logging/ metrics/ apierror/ id/ security/ ratelimit/ database/
db/
  migrations/NNNN_name.up.sql + NNNN_name.down.sql
  embed.go                 # go:embed migrations
web/                       # SPA, if the project has a UI
```

`internal/platform/*` is meant to be **portable** — carry it between projects
and adapt rather than reinvent.

---

## 5. Language & stack preferences

**Backend: Go (latest).** `gin` for HTTP, `gorm` + `gorm.io/driver/postgres` for
persistence, `golang-jwt/jwt/v5`, `google/uuid` (v7), `minio-go/v7`,
`prometheus/client_golang`, `golang.org/x/crypto`. Standard library first;
a dependency has to earn its place.

**Database: PostgreSQL (latest major).** See section 6.

**Frontend (when there is a UI): React 18 + Vite + TypeScript + Tailwind.**
Pin React to 18 — not 19. Structure `web/src/{components,lib,pages}`. Node 20.
No PWA unless I ask for one.

**Not my defaults, don't reach for them unprompted:** an ORM's automigrate as
the source of truth, GraphQL, microservices, Kubernetes, a NoSQL primary store,
server-side rendering frameworks, CSS-in-JS.

---

## 6. Database conventions

- **Money is integers.** Store the appropriate whole unit as `BIGINT` and do all
  arithmetic in integers. Floating point is **prohibited** in any code path
  touching money. Percentages round half-up:
  `floor((amount * bps + 5000) / 10000)`. Rates are held in **basis points**.
- **Money paths use explicit raw SQL** (`gorm.Exec` / `Raw` with placeholders),
  never ORM arithmetic — even in a project where the ORM handles everything else.
- **Primary keys are UUIDv7** — time-ordered for index locality, not sequential
  in a way that leaks volume. Human-facing codes use CSPRNG + Crockford base32.
- **Migrations are numbered SQL**, `NNNN_name.up.sql` with a matching
  `.down.sql`, embedded via `go:embed`, **forward-only in production**. The
  migrations are the source of truth; ORM models map onto them.
- **The database enforces the invariant, not just the application.** Foreign
  keys, `NOT NULL`, `CHECK` constraints, partial and unique indexes. If a
  counter must never exceed a maximum, a `CHECK` says so, so the database itself
  refuses the bad write even under a race.
- **Concurrency is tested, not assumed.** Anything that reserves a limited
  resource takes `SELECT ... FOR UPDATE` (or a constraint-backed counter) inside
  one transaction, and ships with a concurrency test that proves it can't
  oversell.
- **Timestamps are `timestamptz` in UTC.** Business-day logic converts to the
  operating timezone **explicitly** — never rely on the server's local time.
- **Append-only tables for history** — events, audit log, payment events. No
  updates, no deletes; the table's migration spells that out.
- **Multi-tenant / multi-site scoping is a column plus an index plus a
  repository-layer filter**, e.g. `store_id NOT NULL`. Uniqueness constraints
  are per tenant, not global. Scope is enforced in the repository, not only in
  handlers.
- **Seed data lives in its own numbered migration** and is realistic enough to
  demo the product.

---

## 7. Security posture

Target **OWASP ASVS v4 Level 2** and cover every **OWASP Top 10 (2021)**
category explicitly, in a `docs/12-security.md` that maps each control to where
it is implemented **and to the test that proves it**. Non-negotiables:

- **Deny-by-default authorization.** Every handler declares its required
  permission. Every object read is scoped by owner **and** by tenant. Admin
  routes live in a separate router group. Negative authz and IDOR tests exist
  per role and per resource.
- **Passwords: argon2id** with tuned parameters. Never bcrypt-by-default, never
  plaintext, never a homegrown hash.
- **JWT: short-lived access (~15 min) + rotating refresh tokens** stored hashed
  and revocable, with a `jti` denylist on logout. A new token on any privilege
  change. OTP codes are hashed, 6 digits, single-use, short TTL, attempt-capped.
- **Injection:** parameter binding everywhere, raw SQL only with placeholders,
  never string concatenation. Allow-list validation at the adapter edge; the
  domain assumes valid input. No `dangerouslySetInnerHTML`. No shell-outs.
- **Validate and sanitize every input on BOTH sides — frontend and backend.**
  They are two different jobs and neither replaces the other:
  - The **frontend** validates for *feedback*: inline, immediate, in the user's
    language, so nobody discovers a bad field after a round trip. It is a
    convenience and it is **never** a control.
  - The **backend** validates because **the frontend can be bypassed**. Anyone
    with `curl` skips every rule the browser enforces, so the server re-checks
    everything from scratch — presence, type, length, range, format, allow-listed
    enum values, ownership and authorization — and treats the client as hostile.
    A rule that exists only in the browser does not exist.
  - **Same rules, one source.** The two sides must not drift: share the schema
    where the languages allow it, and where they do not (a Go API with a TS
    frontend), generate the client's validation from the server's contract —
    OpenAPI → types + schema. Two hand-written copies of a rule become two
    different rules within a month.
  - **Sanitize on the way in *and* encode on the way out.** Store what the user
    typed, escape it for the context it lands in — HTML, an attribute, a URL, a
    CSV cell, a log line, a filename. Sanitizing input alone does not stop XSS;
    encoding at the point of output does. A CSV export is a real attack surface:
    a cell starting `=`, `+`, `-` or `@` is a formula in Excel.
  - **Reject, do not repair.** Silently "fixing" input hides an attack and
    surprises the user. Say what was wrong and which field.
  - **Normalize before you validate** — trim, Unicode-normalize, case-fold an
    email — or the same value passes one check and fails another.
- **Rate limiting** per identifier and per IP on login, OTP, lookup and any
  brute-forceable endpoint, with progressive lockout and a documented unlock path.
- **File uploads** are type-checked by **magic bytes** (not extension), size- and
  dimension-limited, re-encoded, stored under generated names in a **private**
  bucket, served only by presigned URL.
- **Security headers** via middleware: CSP without `unsafe-inline`,
  `X-Content-Type-Options`, `X-Frame-Options: DENY` / `frame-ancestors 'none'`,
  `Referrer-Policy`, `Permissions-Policy`, HSTS. TLS 1.2+ only.
- **Secrets only via env/config.** Nothing secret in git; `.env.example` is the
  documented surface and the real `.env` is ignored. Rotation is documented.
  No default admin credentials — first-run setup flow instead.
- **Errors are typed** through `platform/apierror` and map to one consistent JSON
  error model. Driver errors, stack traces and internal messages never reach a
  client. Auth errors are generic and never reveal account existence.
- **Logging:** structured, with a request id propagated through context; an
  append-only audit log for every privileged action, price change, refund and
  parameter change (actor, before/after, IP, user agent). No PII in logs or URLs.
- **Idempotency** on every mutating endpoint that creates money or reserves
  capacity (`Idempotency-Key`).
- **Supply chain:** `govulncheck`, `gosec`, `staticcheck`, `npm audit` wired into
  both the Makefile and CI. Dependencies pinned.
- **Abuse cases are written down** with the control for each — resource
  squatting, OTP flooding, code brute-force, scraping, enumeration.
- **A security test suite ships with the product**: negative authz per role,
  IDOR per resource, rate-limit tests, injection fuzz on every input, a
  concurrency test, cross-tenant access tests, and JWT tampering/expiry tests.

---

## 8. Product & UI conventions

- **Search box on every list.** Every screen rendering a list or table has a
  debounced search box that filters that data. No exceptions — a list without
  search is incomplete.
- **Every report and every data grid has an Export to CSV button**, and the
  delimiter is a **pipe (`|`)**, not a comma. No exceptions: if a screen shows
  a table, it exports. A report I can only read on screen is a report I have to
  retype into a spreadsheet.
  - Pipe because the data is Indonesian — addresses, dish names and notes have
    commas in them constantly, and a comma-delimited file of that data opens
    misaligned in Excel often enough to be useless.
  - The export is still a real CSV, quoted per RFC 4180 with `|` as the
    separator, not a hand-joined string. A value containing a pipe, a quote or
    a newline must survive the round trip.
  - Cells are still guarded against spreadsheet formula injection: anything
    starting `=`, `+`, `-`, `@`, tab or CR is prefixed with an apostrophe. A
    CSV is an executable document in Excel.
  - The export honours the filters and the search on screen. Exporting
    something other than what is displayed is worse than no export.
- **Configurable values live in a `sys_parameters` table**, never hard-coded:
  company phone/email/address, tax rates, thresholds, feature toggles,
  operational timings. Every one ships with full CRUD (list + search, create,
  read, update, delete) behind an admin permission, is attributed
  (`updated_by`), and secret-flagged parameters are masked in UI and logs.
  If I might want to change it without a deploy, it is a parameter.
- **Operational timings are parameters too** — lead times, cutoffs, capacities,
  hold windows. I will retune these in production; never bake them into code.
- **Nothing automated cancels a customer's booking** unless I explicitly ask for
  it. Humans cancel; the system surfaces the queue for them.
- **Accessibility to WCAG AA**: measured contrast (state the ratios), visible
  focus rings, real labels, keyboard-operable pickers, announced errors,
  respects `prefers-reduced-motion` and `prefers-color-scheme`. Colour is never
  the only signal.
- **Mobile-first**, designed at 360px, light **and** dark themes as tokens.
- **Multi-language via message catalogues**, never inline strings.
- **Disabled states explain themselves** — show the reason, not a grey box.

---

## 9. Infrastructure defaults

- Development happens on a **shared dev server** (`claudedev`), not a laptop.
  Projects live at `/home/dev/projects/<project>`, per-project config at
  `/etc/<project>/<project>.env`, shared config at `/etc/claudedev/`.
- **nginx reverse-proxies each project's local port**; only 80/443 are open.
- **PostgreSQL runs natively** on the dev server and is shared across projects
  (one database per project, plus a `<project>_test` database for integration
  and concurrency tests). Don't stand up a second Postgres in Docker.
- **Docker is for the satellites** — MinIO, mailpit, WAHA — not for the database.
- **WhatsApp notifications go through WAHA** (self-hosted, one shared container)
  behind a `notify.Provider` port, with the official Meta Cloud API as the
  documented swap-in for production.
- **Every outbound integration sits behind a port/interface** with at least two
  implementations planned, so swapping a provider is an adapter change and never
  a reshape of the core flow.
- Production: single Ubuntu node, nginx + certbot TLS, native PostgreSQL, MinIO
  under systemd, multi-stage Dockerfile → small static Go binary, migrations run
  before the new binary serves, documented backups and rollback.

---

## 10. Doc set convention

Numbered, in `docs/`, kept in sync on every change:

| # | File | Purpose |
|---|---|---|
| 00 | `00-README-and-decisions.md` | Index, **decision log** (`D1…`, dated, with docs touched), open questions |
| 01 | `01-PRD.md` | Problem, personas, scope, requirements, metrics |
| 02 | `02-business-rules.md` | **Normative** — rules carry `BR-x.y` IDs; code comments and test names reference them |
| 03 | `03-data-model.md` | Schema, mermaid ERD, DDL, constraints, indexes, migration notes |
| 04 | `04-api-specification.md` | REST contract, error model, idempotency, auth, pagination |
| 05 | `05-architecture-and-nfr.md` | Architecture, security, performance, observability |
| 06 | `06-domain-operations.md` | Domain-specific operational logic & runbooks |
| 07 | `07-test-plan.md` | Strategy, critical scenarios, QA checklist |
| 08 | `08-roadmap.md` | Phasing and sequencing rationale |
| 09 | `09-deployment.md` | Production deployment, TLS, backups, rollback |
| 10 | `10-design-system.md` | Palette (with measured contrast), typography, components, a11y |
| 11 | `11-local-dev-setup.md` | Local/dev environment and everyday commands |
| 12 | `12-security.md` | ASVS L2 / Top-10 control map, abuse cases, security test suite |
| 13a | `13a-development-server-preparation.md` | Dev-server handbook — Part A (server once) + Part B (onboard a project) |
| 99 | `99-steven-preference.md` | This file — portable preferences |
| — | `PROGRESS.md` | Live build status: ✅ done & tested · 🟡 partial · ⬜ not started |
| — | `RUN-WHEN-BACK.md` | Copy-paste steps that need an interactive terminal |

Rules: `02` is normative and wins over the other docs on product logic;
every behaviour-changing decision gets a **dated row in the `00` decision log**
naming the docs it touched; `PROGRESS.md` is updated as work lands.

---

## 11. Things I don't want to see

- `nano` in a runbook, or relative paths in an OS guide.
- **Validation on one side only** — a rule enforced in the browser and trusted
  by the server, or a server that validates while the form lets the user find
  out after a round trip.
- User input rendered without output encoding for the context it lands in.
- Floating point anywhere near money.
- Secrets in git, or a default admin password.
- A list screen without a search box.
- A configurable value hard-coded in a handler.
- Stopping halfway through an agreed A–Z step to ask whether to continue.
- "Done and tested" for something that was never run.
- Silent scope changes — narrowing, widening or reinterpreting what I asked for.
- Business logic in a handler, or a domain package importing a driver.
- An ORM's automigrate treated as the schema's source of truth.

---

## 12. New-project bootstrap checklist

- [ ] `git init`, remote, `main` as working branch, `.gitignore`, `.gitattributes`
- [ ] `CLAUDE.md` generated from this file + the project's domain and locale
- [ ] `docs/` set from section 10, with `00` decision log started at D1
- [ ] Ask the open product questions **in one batch, defaults proposed**
- [ ] `.env.example`, Makefile, Dockerfile, docker-compose (satellites only), CI
- [ ] `internal/platform/*` carried over and adapted
- [ ] Migrations `0001…` + realistic seed
- [ ] Domain packages with exhaustive unit tests referencing `BR-x.y`
- [ ] `sys_parameters` table + admin CRUD before any configurable value is used
- [ ] Auth, roles and the permissions matrix, with negative tests per role
- [ ] `docs/12-security.md` green, with the tests that prove each control
- [ ] Deployment handbook → user guide → admin guide
- [ ] SEO baseline from section 13 (titles, OG, robots, sitemap, JSON-LD)
- [ ] Claude Code plugin baseline installed (section 14)

---

## 13. SEO — every public web project ships this

Anything with a public-facing page is **SEO-friendly from the first commit**,
not retrofitted before launch. The baseline, in rough order of what actually
costs money when it is missing:

- **Per-route `<title>` and `<meta name="description">`.** A SPA that never
  changes its title gives every page the same name in search results, browser
  tabs and history. One small hook called from each page; no library.
- **Open Graph + Twitter card tags with an absolute image URL.** This is the
  one people skip and regret: **link-preview bots do not execute JavaScript.**
  A client-rendered app with no OG tags in the served HTML shows a blank card
  when the link is pasted into WhatsApp, Instagram DM or Slack. For a business
  whose customers share links in chat, that is the highest-value SEO item on
  the list, and it has nothing to do with Google.
- **`robots.txt`, and it must disallow the private surface** — admin, cart,
  checkout, order history, auth. Crawlers do reach them, and a transactional
  page in an index is a support problem, not a ranking one.
- **`sitemap.xml`** for the pages that should be indexed, referenced from
  `robots.txt`.
- **One `<h1>` per page**, headings in order, no skipping levels for styling.
- **`<html lang>` set, and updated when the language toggle changes.**
- **Canonical URL** on every page, absolute, on the production domain.
- **JSON-LD structured data** matching the domain — `Restaurant` + `Menu` for
  food, `Product` + `Offer` for commerce, `Organization` otherwise. This is
  what produces a rich result rather than a plain blue link.
- **Real URLs for real things.** Filters and sort belong in the query string so
  a state can be linked and shared, not held only in component state.

**Client-side rendering is the constraint behind most of the above.** Google
executes JS; nothing else reliably does. Static tags in `index.html` cover the
whole site with one correct preview, which is usually enough for phase 1. When
per-page previews start to matter — a dish, a product, an article — the fix is
prerendering or SSR, and it is a real project. Decide it deliberately rather
than discovering it after launch.

Verify with `curl`, not a browser: `curl -s <url> | grep -i 'og:\|<title'`
shows what a preview bot sees, which is exactly what the browser hides from you.

## 14. Claude Code tooling baseline

Plugins are installed at **user scope** so they carry across every project on
the machine rather than being re-chosen per repo:

```bash
claude plugin install security-guidance@claude-plugins-official --scope user
claude plugin install gopls-lsp@claude-plugins-official          --scope user

claude plugin marketplace add nextlevelbuilder/ui-ux-pro-max-skill
claude plugin install ui-ux-pro-max@ui-ux-pro-max-skill --scope user
```

| Plugin | Why it is in the baseline | Cost |
| --- | --- | --- |
| `security-guidance` | Four harness hooks — `SessionStart`, `UserPromptSubmit`, `PostToolUse`, `Stop`. Pattern warnings on every edit, LLM diff review on stop, agentic commit reviewer covering injection, XSS, SSRF, hardcoded secrets and 25+ other classes. This is **enforced by the harness, not by Claude's judgement**, which is what makes it worth having next to "auto-commit and push without asking" (section 2) — without it nothing stands between a generated secret and `origin/main`. | ~0 tokens (hooks are harness-only) |
| `ui-ux-pro-max` | Seven skills over a local design database — 84 UI styles, 192 palettes, 74 font pairings, 98 UX guidelines, 161 reasoning rules, 25 chart types, 22 stacks. MIT, no network calls, no account; the ~1.2 MB of CSVs is queried on demand by local Python, never loaded into context. Chosen for the **UX rules and accessibility checks** — loading states, empty states, breakpoints, touch targets, focus states, ARIA — more than for its palette picker. | ~741 tok always-on, ~1.6–3.2k per skill invoked |
| `gopls-lsp` | Go language server — compiler-accurate references, call hierarchies and implementations. Beats any heuristic index on a Go codebase. | LSP, no model context |

**Run one design skill, not two.** Two design skills firing on the same task
give conflicting direction and you cannot tell which produced a bad result.
`ui-ux-pro-max` displaced Anthropic's `frontend-design` for that reason, not
because `frontend-design` is weak — it is the better pick when the brand is
already fixed and what you want is taste rather than a checklist. Swapping back
is one command each way.

Three of the seven skills — `banner-design`, `slides`, `brand` — are dead weight
on a product codebase but only cost ~220 tokens of always-on description
between them. Not worth forking the plugin over.

**Skills are model-triggered, not automatic.** Claude sees only each skill's
name and one-line description, and invokes it when the task matches; a slash
command (`/frontend-design`) forces it. Anything that must run **every** time is
a hook, not a skill — that is the whole reason `security-guidance` is in the
baseline rather than a written instruction.

Deliberately **not** installed, and why:

- **Cloud-vendor database plugins** (`neon`, `supabase`, `prisma`, `alloydb`,
  `cloud-sql-postgresql`, `aiven`) — every one assumes managed Postgres. The
  house default is native PostgreSQL on a single node (section 9).
- **Generic PostgreSQL "best practice" skills** — they teach `NUMERIC`/`DECIMAL`
  for money, which contradicts the money-as-integers rule (section 6). Revisit
  only for operational work — `EXPLAIN ANALYZE`, index health, VACUUM/MVCC —
  once there is real query volume.
- **Commercial SaaS scanners** (`aikido`, `42crunch`, `stackhawk`,
  `sonatype-guide`, `vanta`) — all require paid accounts.
- **`superdesign`** — sends codebase context to an external design canvas.
- **`frontend-design`** (Anthropic) — installed then swapped out for
  `ui-ux-pro-max`; see the one-design-skill rule above. Cheaper (~59 tok
  always-on vs ~741) and better when the visual system is already decided, so
  it is the obvious fallback if the database-driven approach grates.
- **`graphify`** — knowledge-graph indexer. Its value starts around 150k+ LOC or
  where architecture is undocumented; a project built to section 4's layering
  with a written dependency rule already states what the graph would infer, and
  `gopls-lsp` is more accurate for the Go side. Reconsider if a project grows
  past ~150k LOC or needs a cross-language view (Go → SQL → React → docs) that
  an LSP cannot give.
- **Go idiom skill packs** (e.g. `samber/cc-skills-golang`) — genuinely useful
  language-fundamentals skills, but a third of the pack evangelises the author's
  own libraries (`lo`, `mo`, `do`, `oops`, `slog`) and DI frameworks
  (`uber-fx`, `uber-dig`, `google-wire`) that conflict with the pinned stack
  (section 5) and manual wiring in `cmd/api/main.go`. Install selectively or not
  at all.

---

## 15. The `impeccable` skill — install it in every project

Save the block below as `.claude/skills/impeccable/SKILL.md` in each new repo,
frontmatter included. It travels with this file on purpose: it is the standard
of work, and the whole point is that it applies before there is a codebase to
learn it from.

Beside it, each project keeps its own **`design.md`** in the same folder:
fonts, palette, and every colour pairing with its MEASURED contrast ratio, plus
the handful of rules that are not taste. The skill is portable; a brand is not.
Having the numbers one file away from the standard that demands them is what
stops "checked the contrast" becoming a thing people say rather than do.

Its rules are not generic advice. Every one was written after the matching bug
reached a running site on healthy_catering, and the incident log at the end is
the evidence. **Keep the log.** A rule with its incident attached gets followed;
the same rule as a slogan does not. When a new class of silent failure bites,
add the row and the rule — that is how this file earns its keep across projects.

```markdown
---
name: impeccable
description: The standard of work for this codebase. Use when writing, reviewing or finishing any change, and ALWAYS before reporting that something is done. Covers verifying before claiming, catching silent failures, measuring instead of eyeballing, and refusing to ship claims the system cannot back.
---

# Impeccable

Impeccable is not "careful". It is a specific set of habits, each of which
exists because its absence already shipped a bug here. The incidents are at the
bottom; read them once, then work by the rules.

## 1. Never claim what you have not verified

- "Done" means **run**, not written. If a test did not run, say so.
- If verification is impossible — no browser, no key, no data — **say which
  claim is unverified and why**, in the same breath as delivering it. A quiet
  "should work" is the failure.
- Verify the claim you just wrote *in a comment or a migration description*.
  Those are claims too, and they are believed for years.

## 2. Assume every edit silently did nothing

The most expensive bugs here were not wrong logic. They were operations that
succeeded at doing nothing.

- After a string replacement, **assert it changed something**. `str.replace`
  with a stale anchor returns the original happily.
- After a scan into a struct, **check a value came back**. A scan into a column
  that does not exist does not error; it leaves the zero value.
- After a lookup by key, **check the key existed**. A missing catalogue key
  renders as the key.
- Prefer a guard test over vigilance. If a class of silent failure is possible,
  write the test that makes it loud, then fix the instance.
- **A guard is only as good as its oracle.** Derive it from a source of truth —
  the migrations, the AST, the schema — never from text that prose can wander
  into. A check that reads comments will eventually be taught that the bug is
  fine, by the comment warning about the bug.

## 3. Measure — do not eyeball, and do not argue

**The numbers for this project are in `design.md` beside this file** — fonts,
palette, every measured contrast ratio, and the rules that are not taste. Read
it before choosing a colour or a type size, rather than after a review.

- **Contrast is calculated.** Every colour pairing that carries text gets a
  measured ratio, recorded next to the token. `scripts/contrast.py`.
- **Money is integers.** Whole rupiah in BIGINT, integer arithmetic, explicit
  gorm column tags on any `…IDR` field.
- When two people could disagree about whether something looks wrong, **produce
  a number**: a wrap discontinuity as a ratio, an alpha step as a percentage, a
  cascade resolved by parsing the stylesheet. A number ends the argument; an
  opinion restarts it.

## 4. Know which rule actually wins

CSS bit this project three times. Specificity first, then source order.

- A rule that wins on **position** is correct until someone reorders the file.
  Win on **specificity**.
- `.masthead a` matches links inside every panel in the masthead. Scope panel
  rules with their container.
- When unsure, resolve it mechanically rather than by reading.

## 5. Cache like the filename tells the truth

- `immutable` is a promise that the bytes at this URL never change. It is only
  ever correct for a **content-hashed filename**.
- Anything served under a stable name must revalidate, and its URL should carry
  a version so a change arrives immediately.

## 6. Do not ship a claim the system cannot back

- An advertised promise ("free delivery") must be **switchable without a
  deploy**, because the thing that makes it true is a parameter that will
  change.
- A regulated claim (halal, HACCP, ISO) needs the issuer's own file. Do not
  redraw a certification mark, and do not download one of unknown provenance —
  the wrong mark is worse than a plain one.
- Alt text describes the image, not the page. A caption a person cannot see is
  still a statement to somebody.

## 7. Data, schema and configuration are different things

- **Schema** goes in migrations, forward-only, numbered, with a `.down.sql`.
- **Relative-dated sample data** goes in a re-runnable command, never a
  migration — a migration with today's date in it is wrong tomorrow.
- **Anything the business might change without a deploy** is a
  `sys_parameters` row with full CRUD, not a constant.

## 8. Before saying it is done

1. `go vet ./...` and the full test suite — actually run, output read.
2. The change exercised against the running service, not just compiled.
3. Every new user-facing string in all three languages.
4. Every new colour pairing measured.
5. Docs updated **in the same commit** — a decision not in the docs did not
   happen.
6. The report states plainly: what was built, what was verified and how, what
   is still blocked and on whom.

---

## Incident log

Each rule above earned its place:

| What shipped or nearly shipped | Cause |
| --- | --- |
| A public price list showing **Rp 0** against real 55.000 and 48.000 rows | gorm maps `UnitPriceIDR` to `unit_price_id_r`; a scan into a missing column zeroes silently |
| `price.col_amount` rendered as literal text in a table header, in all three languages | a string replacement no-opped after gofmt realigned the map; nothing asserted it matched |
| The home hero's subtitle would render as the literal string `home.lede` on any database without that content row | a template key with no catalogue entry; `T` echoes unknown keys |
| Every CSS change for a week was invisible to anyone who had already visited | nginx marked stable-named `.css` `immutable` for 30 days — the browser never revalidated |
| A phone would have shown the burger **and** the full nav row | two rules of equal specificity; the base one came later in the file |
| The burger drawer rendered beige text on the beige sheet | `.masthead a` tied on specificity and sat later than `.nav-drawer a` |
| Everything below the hero jumped when the photo loaded | intrinsic size hard-coded 800×800 against an 800×533 file |
| "Clearing this hides the badge" — it did not | `Store.String` returns its default when a value is empty, not only when the row is missing |
| A guard test for the Rp 0 bug silently stopped guarding it | its oracle read raw file text, so the COMMENT explaining the bug — "renders `UnitPriceIDR` as unit_price_id_r" — was parsed as a valid column name |

```
