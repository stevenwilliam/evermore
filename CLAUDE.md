# evermore — engineering & product DNA

This file is the contract for how this project is built. Read it first, every
session, before touching code or docs. Where it conflicts with a habit, this
file wins. Where it conflicts with `docs/02-business-rules.md` on *product*
logic, that document wins — this file governs *how* we build, not *what* the
product does.

It is generated from `docs/99-steven-preference.md` §3–§9, which is Steven's
portable engineering DNA. **That file is the source; this one is the local
application of it.** When the two disagree, this file wins — it is the newer,
more specific decision.

---

## 1. What this is

**Repo:** https://github.com/stevenwilliam/evermore
**Owner:** stevenwilliam (itdept.sfg@gmail.com)
**Brand:** Evermore — see §7 and `docs/design_guideline/`

**The `healthy_catering` documents are Evermore's working baseline.** Seeded
2026-09-01 with Steven's standards and the Evermore brand; on 2026-09-01 Steven
promoted the whole of `docs/reference/` into `docs/` to be used, and the folder
was removed. Evermore is built on that catering product's domain model,
decisions, security map and guides.

**What that does and does not settle:**

- **No application code yet.** Not a scaffold, not a "just to get started"
  main.go. §9 step 2 is Steven's, and nothing downstream begins until he
  confirms the brief against this baseline.
- **Do not invent business rules.** Anything the baseline does not state goes
  to `docs/03-open-questions.md` with a proposed default, never into code.
- **The baseline is inherited, not re-earned.** Its 34 decisions in
  `docs/02-decisions.md` are the starting position, and every one is open to
  Steven changing it for Evermore. Its ✅ marks in `docs/PROGRESS.md` were
  earned by `healthy_catering`'s code, which does not exist in this repo — per
  §5 they carry no weight here until re-run. Its file paths point into
  `healthy_catering`.

What *is* already decided is everything below, because it is Steven's standing
preference rather than a product choice.

---

## 2. Architecture — non-negotiable

Hexagonal / clean layering. Dependencies point **inward only**:
`adapter → app → domain`, with `platform` available to all. `domain` imports no
framework, no driver, no `net/http`, no SQL.

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

`internal/platform/*` is meant to be **portable**. Carry it over from an
existing project and adapt rather than reinvent — `ruuma`
(`/home/dev/projects/ruuma/internal/platform/`) and `healthy_catering`
(`/home/dev/projects/healthy_catering/internal/platform/`) both have proven
shapes for `config`, `logging`, `apierror`, `id`, `security` and `ratelimit`.

---

## 3. Stack

Backend: **Go (latest)** · **`gin`** · **`gorm`** + `gorm.io/driver/postgres` ·
**PostgreSQL (latest major)** · `golang-jwt/jwt/v5` · `google/uuid` (v7) ·
S3/MinIO (`minio-go/v7`) · Prometheus (`client_golang`) · `golang.org/x/crypto`.
Standard library first; a dependency has to earn its place.

Frontend, when there is a UI: **React 18** + **Vite** + **TypeScript** +
**Tailwind**, `web/src/{components,lib,pages}`. Pin React to 18, not 19.
Node 20. No PWA unless asked.

ORM is `gorm`. **Exception:** any code path touching money uses explicit
`gorm.Exec`/`Raw` with placeholders and integer arithmetic — never the ORM for
money math, even here.

Not defaults, do not reach for them unprompted: automigrate as the source of
truth, GraphQL, microservices, Kubernetes, a NoSQL primary store, SSR
frameworks, CSS-in-JS.

---

## 4. Hard rules

- **Money is integers.** Store the whole minor unit as `BIGINT` and do all
  arithmetic in integers. Floating point is prohibited in any code path
  touching money. Rates are basis points, rounded half-up:
  `floor((amount * bps + 5000) / 10000)`.
- **IDs are UUIDv7.** Human-facing codes use CSPRNG + Crockford base32.
- **The domain layer is pure and exhaustively unit-tested.** Adapters get
  integration tests.
- **Migrations are forward-only in production**, numbered, each with a matching
  `.down.sql`, embedded via `go:embed`. The migrations are the source of truth.
- **The database enforces the invariant**, not just the application — foreign
  keys, `NOT NULL`, `CHECK`, partial and unique indexes. If a counter must
  never exceed a maximum, a `CHECK` says so, so the database refuses the bad
  write even under a race.
- **Concurrency is tested, not assumed.** Anything reserving a limited resource
  takes `SELECT … FOR UPDATE` inside one transaction and ships with a test that
  proves it cannot oversell.
- **Timestamps are `timestamptz` in UTC.** Business-day logic converts to the
  operating timezone explicitly — never server-local.
- **History tables are append-only** — events, audit log, payment events. No
  updates, no deletes; the migration spells that out.
- **Every input is validated and sanitized on both sides — web and API.**
  The frontend validates for *feedback*; the backend validates because the
  frontend can be bypassed with `curl`. Same rules, one source: generate the
  client's types and validation from the server's contract so the two cannot
  drift. Sanitize on the way in **and** encode on the way out for the context —
  HTML, attribute, URL, CSV cell, log line, filename. **Reject, never silently
  repair.** Normalize (trim, Unicode, case-fold) before validating. A rule that
  exists only in the browser does not exist.
- **Deny-by-default authorization.** Every handler declares its permission;
  every object read is scoped by owner and tenant in the *query*, not checked
  afterwards. Negative authz and IDOR tests per role and per resource.
- **Passwords are argon2id.** Access tokens ~15 min, refresh tokens rotating,
  stored hashed and revocable, `jti` denylist on logout.
- **Errors are typed** through `platform/apierror`; one JSON error model. Never
  leak driver errors to clients.
- **Secrets only via config/env.** Nothing secret in git. `.env.example` is the
  documented surface; the real `.env` is git-ignored.

Security targets **OWASP ASVS v4 Level 2** and covers every **OWASP Top 10
(2021)** category in `docs/12-security.md`, mapping each control to where it is
implemented **and to the test that proves it**.
`docs/12-security.md` is that map, inherited from the previous build — every
row names a file and a proof, and its paths point into `healthy_catering` until
they are repointed here.

---

## 5. Docs discipline

- Docs live in `docs/`, numbered per `99-steven-preference.md` §10.
  `docs/02-business-rules.md` is **normative** — rules carry `BR-x.y` IDs and
  code/tests reference those IDs.
- **Keep all docs in sync on every decision**, in the same commit as the
  change. A decision that isn't in the docs didn't happen. Every
  behaviour-changing decision gets a dated row in the `00` decision log naming
  the docs it touched.
- `docs/PROGRESS.md` is live build status (✅ done & tested · 🟡 partial ·
  ⬜ not started). **A ✅ has to be re-earned by running the gate, never
  inherited.**
- `docs/RUN-WHEN-BACK.md` holds steps needing an interactive terminal.
- `docs/99-steven-preference.md` is portable and project-agnostic. Improvements
  that are not specific to this project belong there, so they reach the next
  project too.
- **The promoted docs (`00`–`04`, `12`, `14`–`16`, `PROMPT.md`, `PROGRESS.md`,
  `RUN-WHEN-BACK.md`, `screenshots/`) describe the `healthy_catering` build.**
  They are Evermore's baseline to work from and amend in place — not a record
  of anything this repo has built. Correct them as Evermore diverges.

---

## 6. Working conventions

- **`.claude/skills/impeccable/SKILL.md` is the standard of work.** Read it
  before writing a change and again before reporting one as done. Its rules are
  not general advice — each was written after the matching bug reached a
  running site, and the incident log names them. When a new class of silent
  failure bites, add the row and the rule. It is mirrored in
  `docs/99-steven-preference.md` §15 so it carries to the next project.
- **Owner is Steven, nickname "ven".** When he answers a quoted list of
  questions, a line beginning `ven:` is his answer to the question above it.
- **`coding stop` means change nothing** — no edits, no new files, no commits,
  no migrations, no deploys, no config changes — until he says `coding start`.
  It is a hard gate, not a preference to weigh against the task, and it **holds
  across turns** until lifted. A new request while the hold is on is a request
  to discuss and plan, not a licence to resume: say what you would do, then
  wait. Reading, searching, read-only commands, answering and planning stay
  fine; what stops is anything that writes — the filesystem, the database, a
  running service, or a remote. If unsure whether the hold is on, it is.
- **Ask everything at once, up front, with a default per question.** One batch
  before starting, not a drip of questions mid-build.
- **Once the docs and business rules are agreed, build to the end without
  stopping.** Planning is when Steven answers questions; the build is when you
  work. Do not pause for milestone approval, and **do not let a blocker stop
  the build** — work around it, note it, keep going on everything that does not
  depend on it, and hand him the whole list at the end. Fine-tuning and
  correction are end-of-build items; only something *wrong* gets fixed on the
  spot. Then report: built, verified by running, still blocked and on whom.
- **Never stop partway.** If the plan says "build all modules A–Z", build all of
  them in one push.
- **Update related documents on every interaction** — including talk-only turns
  that settle a decision.
- **Auto-commit + push after every completed change**, without asking. Small,
  focused commits, conventional-commit messages. `main` is the working branch.
- **Tell the truth about what was verified.** If a test did not run, say so and
  put the step in `RUN-WHEN-BACK.md`. Never report "done and tested" for
  something only written.
- **Verify visual work by looking at it.** Screenshot the rendered page and
  probe computed styles; do not conclude from reading CSS. Three separate
  defects on the previous project made text invisible at 1.00:1 and none was
  visible in the source.
- **Editor is `vi`** in every runbook and docs example — never `nano`.
- **OS/server guides use full absolute paths**, never relative ones.
- Prefer editing existing files and reusing `platform/*` over new scaffolding.

---

## 7. Product & UI conventions

- **Search box on every list.** Every screen rendering a list or table has a
  debounced search box that filters it. No exceptions.
- **Every report and every data grid ships an Export to CSV button**, and the
  delimiter is a **pipe (`|`)**, never a comma — Indonesian addresses, names and
  notes contain commas constantly. It is still a real RFC 4180 CSV with `|` as
  the separator, still guarded against formula injection (`=`, `+`, `-`, `@`,
  tab, CR get an apostrophe), and it exports **what the screen is currently
  showing** — filters and search included.
- **Configurable values live in `sys_parameters`.** Anything that could change
  without a code change — company phone, email, address, tax rate, feature
  toggles, thresholds, lead times, cut-offs, capacities — is a row in that
  table, not a constant, and ships with full CRUD behind an admin permission,
  attributed via `updated_by`, with secret-flagged values masked in UI and logs.
- **Nothing automated cancels a customer's booking** unless Steven asks for it.
  Humans cancel; the system surfaces the queue for them.
- **Brand is Evermore.** The supplied guidelines are in
  `docs/design_guideline/`; `docs/10-design-system.md` is the engineering
  reading of them. **Several brand colours fail WCAG AA as text or as button
  fills** — that is documented there with measured ratios and must be
  respected, not rediscovered. `scripts/contrast.py` in the reference project
  is the tool that measures them.
- **Accessibility is AA minimum**, and contrast is *calculated*, not eyeballed.
  Visible focus rings, real labels, keyboard-operable pickers, announced
  errors, `prefers-reduced-motion` and `prefers-color-scheme` respected. Colour
  is never the only signal.
- **Mobile-first**, designed at 360px, light and dark as tokens.
- **Multi-language via message catalogues**, never inline strings.
- **Disabled states explain themselves** — show the reason, not a grey box.
- **Every public page ships the SEO baseline** from `99-steven-preference.md`
  §13: per-route title and description, Open Graph tags **static in the served
  HTML** (preview bots do not run JavaScript), `robots.txt` disallowing the
  transactional surface, `sitemap.xml`, one `<h1>` per page, JSON-LD.

---

## 8. Document control

Always update related documents in the same commit as the change — PRD,
business rules, data model, API spec, deployment/user/admin guides. A change
whose docs are stale is not done.

---

## 9. Delivery workflow

1. **Initial git setup** — repo, remotes, conventions, this file. ← done
   2026-09-01
2. **Steven — preparation.** He gives the PRD and business-rules brief, tuning,
   and final confirmation. **Nothing downstream starts until he confirms.**
   ← we are here. The `healthy_catering` document set was promoted into `docs/`
   on 2026-09-01 (D19) as the baseline to work from; it is a starting position,
   not the confirmation.
3. **Claude — build all documents A→Z** from the confirmed brief.
4. **Claude — build all modules in one shot, A→Z.** Do not stop partway.
5. **Claude — test, debug and security-harden, A→Z.** Do not stop partway.
6. **Claude — production deployment handbook** (copy-paste, empty machine, full
   absolute paths), **then** the user guide, **then** the admin guide.

The bootstrap checklist for step 3 onward is `99-steven-preference.md` §12.

---

## 10. Locale / environment

**Not yet decided.** The previous project settled on IDR / `Asia/Jakarta` /
`id-ID`+`en`+`zh-Hans`, and Steven's infrastructure defaults (§9 of the
preference file) point the same way — but locale is a product decision and this
product has no brief. Record it here when he gives one, and until then:

- Money: currency and minor unit **undecided**. Whatever it is, it is `BIGINT`
  integers (§4).
- Timezone: operating zone **undecided**. Whatever it is, storage is UTC and
  business-day logic converts explicitly (§4).
- Languages: **undecided**. Whatever they are, catalogues from the first string
  (§7).
- Production domain and host: **undecided**.

Development runs on the shared dev server `claudedev` at
`/home/dev/projects/evermore`, per-project config at
`/etc/evermore/evermore.env`, nginx reverse-proxying a local port, PostgreSQL
native and shared (one database plus `evermore_test`), Docker for satellites
only — MinIO, mailpit, WAHA. That much is `99-steven-preference.md` §9 and does
not need a brief.
