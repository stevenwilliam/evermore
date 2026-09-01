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
