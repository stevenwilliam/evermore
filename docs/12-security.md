# 12 — Security

**Target:** OWASP ASVS v4 Level 2, every OWASP Top 10 (2021) category mapped to
a control **and to the test that proves it** (99 §7).
**Date:** 2026-08-13

A control with no test is a claim. Every row below names the file that
implements it and the test that fails if it regresses. Where something is not
yet done, the row says so rather than being omitted.

---

## 1. OWASP Top 10 (2021) control map

### A01 — Broken access control

| Control | Where | Proven by |
|---|---|---|
| Deny-by-default: a route that declares no permission serves nobody | `adapter/http/auth.go` `RequirePermission` | Live probe: a customer gets `FORBIDDEN` on every `/admin` route |
| Permissions resolved from the **database per request**, never from the token | `app/auth.go` `Resolve` | A revoked role takes effect on the next request, not in 15 minutes |
| Ownership scoped in the **WHERE clause**, not checked after the read | `postgres/order.go` `AddressForCustomer`, `order_read.go` `GetForCustomer` | Live probe: customer B gets `NOT_FOUND` for A's order and cannot use A's address |
| Own-scoped permissions say "your own X", never *which* X | migration 0013 | `security_test.go` `TestAddressesAreOwnedRows` |
| Kitchen-scoped staff cannot widen their scope from the query string | `app/reports.go` `resolve` | The caller's `KitchenID` overrides the request |
| Admin-only refunds | `domain/order` transition table | `order_test.go` `TestRefundIsAdminOnly` |

### A02 — Cryptographic failures

| Control | Where | Proven by |
|---|---|---|
| argon2id password hashing, tuned | `platform/security/password.go` | `security_test.go` (platform) |
| Refresh tokens stored **hashed**; a database leak hands over no live session | `postgres/user.go` `StoreRefreshToken` | — |
| Verification tokens hashed, single-use, expiring | `postgres/user.go` `ConsumeVerificationToken` | Live probe |
| TOTP secrets stored as ciphertext | migration 0002 `user_totp.secret_cipher`, AES-GCM under `TOTP_ENCRYPTION_KEY` | ✅ `TestStoredTOTPSecretIsNotPlaintext` |
| TLS 1.2+ only, HSTS in production | `deploy/nginx/evermore.conf`, `middleware.go` | ⬜ needs the certificate |

### A03 — Injection

| Control | Where | Proven by |
|---|---|---|
| Parameter binding everywhere; no string concatenation into SQL | every repository | `security_test.go` `TestSQLInjectionIsData` — five payloads land as data, 53 tables survive |
| `ORDER BY` columns are **allow-listed**, never interpolated | `postgres/query.go` `Normalise` | Sort values outside the list fall back to the default |
| `LIKE` wildcards escaped | `postgres/query.go` `SearchPattern` | Live probe: `?q=%` matches 0 rows, not everything |
| Output encoded for its context — HTML, attribute, **CSV cell** | `platform/sanitize`, `notify/templates.go` | `sanitize_test.go` `TestCSVCellNeutralisesFormulas`; live probe on the manifest export |
| No `dangerouslySetInnerHTML`, no shell-outs | — | No such call exists in the tree |

### A04 — Insecure design

| Control | Where | Proven by |
|---|---|---|
| Money is integers; floating point prohibited on money paths | `domain/money` | `money_test.go`; no `float64` in the package |
| Capacity cannot oversell, under any race | `postgres/order.go`, CHECK constraints | `TestCapacityCannotOversell` (20 writers, 5 win), `TestCheckConstraintIsTheLastLine` |
| A credit cannot be double-spent | row lock + unique index | Live probe: 8 concurrent bookings on 1 credit → 1 wins, balance 0; `TestOneRedeemPerDelivery` |
| Prices cannot overlap | `EXCLUDE USING gist` | `TestPriceOverlapRefusedUnderConcurrency` |
| History is append-only | triggers on `credit_ledger`, `audit_log` | `TestLedgerAndAuditAreAppendOnly` |
| A missing price **blocks** the sale rather than guessing | `domain/pricing` | `pricing_test.go` `TestBothScopesMissingBlocks` |

### A05 — Security misconfiguration

| Control | Where | Proven by |
|---|---|---|
| CSP without `unsafe-inline`; `frame-ancestors 'none'`; nosniff; Referrer-Policy; Permissions-Policy | `adapter/http/middleware.go` | Response headers on every route |
| Driver errors, causes and stack traces never reach a client | `platform/apierror`, `middleware.Fail` | Live probe: a failing query returned a generic 500 with a request id |
| No default admin account, no seeded password | `cmd/api/setup.go` | The only way to make one is `api create-staff`, typing the password |
| Secrets only in `/etc/evermore/evermore.env`, mode 640, root:dev | deployment | Nothing secret in git; `.env` is git-ignored |
| systemd hardening: `NoNewPrivileges`, `ProtectSystem=strict`, `MemoryDenyWriteExecute` | `deploy/evermore.service` | `systemctl show evermore` |
| The API binds **127.0.0.1** and is unreachable from the LAN | `config.App.Bind` | Live probe: `:8081` from the LAN is refused; only nginx's port answers |

### A06 — Vulnerable and outdated components

| Control | Where | Proven by |
|---|---|---|
| Dependencies pinned in `go.mod`/`go.sum` | repo root | — |
| `govulncheck`, `gosec`, `staticcheck` in the Makefile and CI | ⬜ **not yet wired** | — |

### A07 — Identification and authentication failures

| Control | Where | Proven by |
|---|---|---|
| Identical answer whether or not an account exists | `app/auth.go` `Login` | Live probe: wrong password and unknown address return the same message |
| **Timing** does not leak account existence either | `security.VerifyPasswordDummy` | A missing account still pays for one argon2id derivation |
| Registration is not an enumeration oracle | `auth.go` `Register` | Live probe: registering an existing address returns the identical reply |
| Progressive lockout, configurable | `RecordLoginFailure` + `sys_parameters` | — |
| Refresh rotation, single-use | `ConsumeRefreshToken` (`UPDATE … RETURNING`) | Live probe: replaying a consumed token is refused |
| Rate limiting per identifier **and** per IP | `middleware.RateLimit` | Applied to auth and order creation |
| Email verification required before the first order | `RequireVerifiedEmail` | Live probe |
| CAPTCHA on registration | ⬜ **Turnstile chosen, not wired** (needs a key) | — |
| 2FA for admin/finance/staff | ✅ enrolment, challenge, recovery codes | `Role.RequiresTOTP`, `TestMFAChallengeTokenIsNotASession` |
| A challenge token is not a session | ✅ `Claims.Purpose` + the `RequireAuth` refusal | `TestMFAChallengeTokenIsNotASession` |
| A TOTP code is single-use | ✅ `last_used_step` guarded in SQL | `TestTOTPCodeCannotBeSpentTwiceConcurrently` |
| A recovery code is single-use | ✅ removed from the JSONB set in one guarded UPDATE | `TestRecoveryCodeIsConsumedExactlyOnce` |

### A08 — Software and data integrity failures

| Control | Where | Proven by |
|---|---|---|
| Price, meal composition and nutrition **snapshotted** onto the order | `postgres/order.go` `mealSnapshot` | Live probe: a later price edit does not move a historical order |
| Address snapshotted onto the delivery | same | Editing a saved address does not rewrite history |
| Idempotency keys in Postgres, in the same transaction as the write | migration 0001, `PlaceOrder` | Live probe: a replayed key is refused |
| Every privileged write audited with before/after, actor, IP | `postgres/audit.go` | Live probe: `18:00 → 17:30` with the reason recorded |

### A09 — Security logging and monitoring failures

| Control | Where | Proven by |
|---|---|---|
| Structured logs with a request id on every line | `middleware.Logger` | Every error reply carries the same id |
| The **route template** is logged, never the resolved path | `middleware.Logger` | A path carries ids and sometimes an email |
| PII redacted from logs | `platform/logging` | The `create-staff` run logged `email=[redacted]` |
| Coordinates never logged | `app/serviceability.go` | Coordinates appear only in the database |
| Append-only audit log | migration 0001 | `TestLedgerAndAuditAreAppendOnly` |
| Prometheus metrics, health endpoint | `/metrics`, `/healthz` | Live |
| Sentry-compatible error reporting | ⬜ **not wired** | — |

### A10 — Server-side request forgery

| Control | Where | Proven by |
|---|---|---|
| The service makes no outbound request to a user-supplied URL | — | The only outbound calls are SMTP and WAHA, both from configuration |
| Google Maps calls are client-side with a referrer-restricted key | `config.Maps` | ⬜ blocked on the keys |

---

## 2. Abuse cases

| Abuse | Control | State |
|---|---|---|
| Oversell a sold-out meal by racing checkout | Guarded UPDATE + CHECK constraint | ✅ proved, 20 writers |
| Spend the same credit twice from two tabs | Row lock + unique REDEEM index | ✅ proved, 8 writers |
| Read another customer's order, address or ledger | Owner in the WHERE clause | ✅ proved |
| Enumerate accounts through login or registration | Identical replies and identical timing | ✅ proved |
| Brute-force a password | Lockout + rate limit | ✅ built, ⬜ lockout not yet load-tested |
| Inject a spreadsheet formula through a delivery note | `sanitize.CSVCell` on every export | ✅ proved |
| Inject a mail header through a name or subject | CRLF refused | ✅ proved |
| Escape the upload prefix with `../` | Object key validated as a key | ✅ proved |
| Upload an executable as a payment proof | Content type allow-listed, 5 MB cap | ✅ proved. ⬜ magic-byte check pending real uploads |
| Hold capacity indefinitely without paying | Deadline capped at the cut-off, auto-expiry returns both counters | ✅ built |
| Scrape the menu | Public by design; robots.txt disallows the transactional surface | ✅ |
| Squat a delivery slot from an unserviceable address | Serviceability checked at save AND at checkout | ✅ proved |

---

## 3. Honest gaps

These are **not** done, and are listed so nobody reads a green table as a
finished audit:

1. **Turnstile** — chosen (no second Google dependency), not wired; needs a key.
   Registration and login are rate-limited in the meantime, which is weaker.
2. **CI has never run on a runner** — `.github/workflows/ci.yml` exists and each
   of its steps (`gofmt`, `go vet`, migrate up/down/up, tests, `govulncheck`,
   `staticcheck`, `gosec`, `npm audit`) passes locally, but no push has yet
   exercised it on GitHub Actions. Written is not the same as proven.
3. **UU PDP data export and account deletion** — required before launch, not
   built. Note the tension to resolve: financial records must be retained for
   tax, so deletion should anonymise the customer and keep the order.
4. **Backups are still on the same machine** — the script and a *tested* restore
   drill both exist, but the off-machine copy is a commented-out line awaiting a
   bucket. A backup that burns with the server is not a backup.
5. **No 2FA rate limit distinct from login** — `/auth/mfa` shares the `auth`
   limiter bucket (10/window). That bounds guessing at a six-digit code, but a
   dedicated per-challenge attempt counter would be tighter.
6. **`TOTP_ENCRYPTION_KEY` has no rotation path** — changing it makes every
   existing enrolment undecryptable, and a required role has no way back in
   except a recovery code. Documented in `.env.example`; a re-encrypting
   migration would be the real fix.
6. **Sentry-compatible reporting** and a **tested `pg_dump` restore** — the
   backup script is not written, and an untested restore is not a backup.
7. **Load and lockout testing** — the rate limiter is in-memory and correct for
   one node; it has not been exercised under load.
