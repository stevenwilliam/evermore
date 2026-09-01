# 16 — Admin guide (staff)

**For:** Evermore staff — admin, finance, kitchen, courier.
**Date:** 2026-08-13.

Everything here is reachable from the API today. Screens are described by what
they do; the back-office UI is API-complete but the React shell is not built
yet, so today these are exercised through the API or a REST client.

---

## 1. Roles and what each can do

| Role | Can |
|---|---|
| `admin` | Everything, plus settings and staff accounts |
| `staff` | Customers, organisations, catalogue, menu calendar, orders, deliveries |
| `finance` | Verify and reject payments, credit adjustments, financial reports |
| `kitchen` | Production sheets, mark meals prepared — **their own kitchen only** |
| `courier` | Their manifest, mark delivered |

Access is **deny-by-default**: a role that has not been granted a permission
gets a clear refusal, not a partial screen. Kitchen and courier accounts can be
scoped to one kitchen, and that scope cannot be widened from the browser.

Creating staff is done on the server, once, by an administrator:

```bash
/srv/evermore/bin/api create-staff --email name@evermore.co.id --role finance
```

There is no default admin account and no shared password.

## 2. Daily rhythm

**Morning**
1. Check the payment queue — oldest first. Those customers have waited longest.
2. Print the production sheet for each kitchen and slot.
3. Print packing labels and the courier manifest.

**Afternoon**
4. Watch the publish horizon warning. If the menu is published fewer than seven
   days ahead, package customers cannot book beyond that point.
5. Check the coverage report for addresses you could not serve.

**Before 18:00**
6. The cut-off passes. After it, orders for tomorrow are closed, skips are no
   longer allowed, and the kitchen numbers are final.

## 3. The menu calendar

A **meal** is one date + one diet type + one slot, containing one or more
dishes. That is the thing customers buy and the thing one credit pays for.

- Build a week, then use **copy-week** to duplicate it forward. Copies always
  arrive as **drafts**, whatever the source was — publishing stays deliberate.
- Customers only ever see **published** meals.
- Capacity is per meal. Leave it empty for unlimited.
- A meal needs a main dish, and no dish twice.

**You cannot unpublish a meal customers have already ordered.** Substitute a
dish instead — that is audited and the customers are notified. A menu that
vanishes after an order is how somebody receives an empty box.

## 4. Prices

Four separate screens, on purpose:

| Screen | Holds |
|---|---|
| Meal price — normal | The standing price per diet type per quantity tier |
| Meal price — promo | A time-limited price that overrides the normal one |
| Package price — normal | The standing package price |
| Package price — promo | A time-limited package price |

Rules worth knowing:

- **Every price you type includes tax.** The system splits base and tax on each
  order and remembers the rate that applied that day.
- **Scope wins over promo.** We resolve the customer's type first, then look for
  a promo *within that scope*. So a public promotion does **not** undercut a
  negotiated corporate rate — which means a corporate customer can pay more than
  a walk-in during a promotion. When you create a public promo, the screen tells
  you exactly which corporate scopes are now dearer so you can decide.
- **Tiers are flat.** Ten meals are all priced at the ten-meal rate.
- **Two prices cannot cover the same date** for the same scope and tier. If they
  would, you are told which existing row conflicts and for which dates.
- If no price exists, the sale is **blocked**. We never guess a price.

## 5. Verifying payments

The queue shows the order code, the customer, the expected amount **including
their unique three-digit code**, how long they have waited, and their proof.

- Match the amount to your bank statement — the last three digits are what make
  each transfer unique.
- **Verify** marks the order paid. For a package order this also issues the
  credits and starts the validity period from that moment.
- **Reject** requires a reason; the customer sees it and can upload again.
- If a colleague has already verified it, you are told so plainly.

**There are no refunds.** If something goes wrong, compensate in credits with a
reason — that is recorded against the customer. Returning actual money is an
admin-only action reserved for a genuine mistake such as a duplicate transfer.

## 6. Reports

| Report | Use |
|---|---|
| Production sheet | What each kitchen cooks: portions per dish, per diet, per slot |
| Packing labels | One label per delivery, with allergens and the driver note |
| Courier manifest | Per kitchen, ordered by distance, with a Maps link per stop |
| Coverage | Addresses refused, grouped by district — where to open next |
| Sales | Revenue by day/week/month, with the discount promotions actually cost |
| Customer credits | Every package with its live balance and full movement history |
| Unpaid / expiring | Who to chase |
| Retention | Repeat rate by cohort |

All of them take a date range and export to CSV.

## 7. Settings

Anything you might reasonably want to change without a developer is a setting:
the cut-off time and lead days, the payment window, the tax rate, delivery fee
bands, the low-credit threshold, the publish-horizon target, mail and WhatsApp
configuration, the coordinate envelope.

Every change is **audited** — who, when, from what, to what, and why. Values are
type-checked, so a cut-off of "6pm" is refused rather than silently ignored.

Secret settings such as the mail password are masked everywhere, including in
the audit log.

## 7a. Two-factor authentication

Your account needs a code from your phone as well as a password. For **admin,
finance and staff** this is required and cannot be switched off. Kitchen and
courier accounts do not need it — those sign-ins happen on shared phones on the
service floor.

**Setting it up.** Open **Keamanan** from the menu, press *Aktifkan verifikasi
dua langkah*, and add the secret shown to Google Authenticator, Authy or a
password manager that generates codes. Type the six-digit code back to finish.
Nothing changes on your account until that code is correct — if you scanned it
wrong, you simply try again.

**Your recovery codes.** Confirming shows eight codes. Save them somewhere that
is not the phone generating the codes. Each works once. This is the only time
they are shown, and for a required role they are the only way back in if the
phone is lost or wiped.

**Signing in afterwards.** Enter your email and password as usual, then the
six-digit code on the next screen. A recovery code can be typed into the same
box instead.

**Everyday problems.**

| What you see | What it means |
|---|---|
| *Kode tidak dapat diverifikasi* | Usually the phone's clock. Set the phone's date and time to automatic and try the next code. |
| *That code has already been used* | Each code works once. Wait for the next one to appear. |
| *That sign-in attempt expired* | The code screen was left open too long. Sign in again from the start. |
| Lost phone, out of recovery codes | Only another admin can help: they must clear the enrolment for your account directly in the database (`DELETE FROM user_totp WHERE user_id = …`), after confirming by some other means that it is really you. |

## 8. Things the system will not let you do

Not bugs — deliberate:

- Deactivate the default customer type (every registration lands on it)
- Unpublish a meal that has deliveries
- Create overlapping prices
- Oversell a meal or a kitchen slot
- Edit or delete a credit-ledger entry or an audit row — correct with a
  compensating adjustment instead
- Verify the same payment twice
- Refund money as anyone other than an admin
