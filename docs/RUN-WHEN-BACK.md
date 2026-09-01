# Run when you're back — interactive steps

Steps needing an interactive terminal, a browser, or credentials that do not
exist yet. Use `vi` for any edits.

_Updated: 2026-08-31 — §A6 added: the service must be restarted onto the new
binary before the canvas rework is visible._

## A. One command — this is why you still cannot open the site · DO THIS FIRST

The firewall rule added on 2026-08-13 was scoped to the wrong network. Your PC
does not reach this VM from `192.168.88.0/24`; it arrives as **`172.16.0.1`**,
the VMware host-side adapter. `ufw` logged 36 dropped connections from it to
port 8090 — the packets never reached nginx, which is why the tab just spins.

I could not run this myself: changing the firewall needs a permission this
session does not have. Paste it into the terminal (or type `! <command>` in
this session so the output lands here):

```bash
sudo ufw allow from 172.16.0.0/24 to any port 8090 proto tcp \
     comment 'evermore dev (VMware host net)'
sudo ufw status numbered
```

Then open **`http://192.168.88.101:8090/`**.

Not `http://192.168.88.101/` — port 80 on the bare IP is still ruuma's
`default_server`, so it loads the other project and looks like a broken deploy.

Keep the existing `192.168.88.0/24` rule; it covers anything on the physical
LAN. Both rules go away when the site moves to 443 (`docs/14` §8a).

## A1. If a change does not appear, HARD-RELOAD once

Fixed on 2026-08-19, but one hard reload is still needed to escape a cache
already poisoned.

nginx was serving every `.css`, `.js`, `.png`, `.jpg` and `.svg` with
`Cache-Control: public, immutable` for 30 days. `immutable` tells a browser
never to revalidate — not even a conditional request — so a visitor who loaded
the site once kept that stylesheet through every later change. The symptom was
"the burger menu is not showing any button": the markup was current and the
stylesheet was many edits behind.

Two fixes, both in place:

- nginx now only marks the woff2 fonts immutable, since those are the only
  files whose bytes never change under a stable name. Everything else is
  `must-revalidate` with a one-hour freshness.
- The templates append `?v=<fingerprint>` to the stylesheets and the wordmark,
  so a changed file is a NEW URL and arrives immediately regardless of any
  cache header.

Your browser may still be holding the poisoned copy, because the old response
said not to ask again. **Ctrl-Shift-R once** and it will never recur.

## A2. Two things blocked on a key you have to supply

**AI images.** Asked for twice now — the hero picture (which you then supplied
yourself as a photograph) and a picture per menu. Neither can be generated on
this box: there is no `GEMINI_API_KEY` anywhere, the `google-genai` package is
not installed, and there is no `pip` to install it. The network is open, so a
key is the only missing piece — the REST endpoint can be called with `curl` and
the image decoded with the standard library, no SDK needed.

```bash
export GEMINI_API_KEY='...'      # or add it to ~/.claude/.env
```

Until then the menu cards show an illustrated band in each diet type's colour,
drawn from the same glyph source as the home page corner marks. A real
photograph replaces it per meal by setting `scheduled_meal.hero_photo_key` —
the card already prefers it when present.

**Machine translation.** Same shape of problem: the back office at
`/app/admin/content` edits the hero copy in three languages, but with no
provider configured English and Chinese have to be typed by hand.

```bash
TRANSLATE_PROVIDER=google
TRANSLATE_API_KEY='...'
```

Billable per character; see `docs/11` §6.

## A3. Chrome works again — here is the invocation that works

**Superseded 2026-08-27.** This section said headless Chrome timed out on every
URL and that "both Playwright builds under `~/.cache/ms-playwright/` fail
identically". That is no longer true, and the second half was never quite the
whole story: it is the **npm package**, not the browser, that decides which
build gets driven, and there are two of those cached.

- `~/.npm/_npx/f0a362733743bae2` is Playwright **1.49.1** — this one hangs at
  `chromium.launch()` and never returns. It is almost certainly what was being
  reached when this section was written.
- `~/.npm/_npx/e41f203b7505f1fb` is Playwright **1.62.1** — this one launches,
  navigates, and screenshots. Verified on 2026-08-27 against the running site.

So point `NODE_PATH` at the working one:

```bash
export NODE_PATH=/home/dev/.npm/_npx/e41f203b7505f1fb/node_modules
node -e "const{chromium}=require('playwright');(async()=>{const b=await chromium.launch();
  const p=await b.newPage({viewport:{width:390,height:844}});
  await p.goto('http://127.0.0.1:8090/',{waitUntil:'domcontentloaded'});
  await p.waitForTimeout(1500); await p.screenshot({path:'/tmp/home.png'}); await b.close();})()"
```

The CLI form also works and needs no NODE_PATH:

```bash
npx --no-install playwright screenshot --viewport-size=390,844 \
    --wait-for-timeout=1500 http://127.0.0.1:8090/ /tmp/home.png
```

Two things follow. **Visual work can be verified by eye again**, which is what
CLAUDE.md §6 requires and what several changes went without. And `waitUntil:
'networkidle'` hangs on this site — use `domcontentloaded` plus a fixed wait;
that behaviour is what made the failure look like a broken browser.

## A4. Restart the service after a CSS or template change

`assetVersion` is computed **once per process** (`sync.OnceValue`), so editing
`web/public/css/public.css` changes the bytes nginx serves but NOT the `?v=`
the templates emit. A first-time visitor sees the change immediately; a
returning visitor keeps their cached copy until nginx's one-hour
`must-revalidate` window lapses.

This session cannot restart the unit — it needs interactive authentication:

```bash
sudo systemctl restart evermore
curl -s http://127.0.0.1:8090/ | grep -oE 'public\.css\?v=[a-z0-9]+'
```

The fingerprint should differ from `q95cuvfac046`, which is what it was serving
when the home-screen animation was added on 2026-08-27.

## A5. The masthead markup needs a REBUILD, not just a restart

The templates are Go string constants compiled into the binary, so the new
logo lockup (`wordmark-mark` + `wordmark-type`, 2026-08-27) does not appear
until the binary is rebuilt and the unit restarted — a plain `systemctl
restart` of the old binary is not enough:

```bash
cd /home/dev/projects/healthy_catering
/usr/local/go/bin/go build -o bin/api ./cmd/api
sudo systemctl restart evermore
curl -s http://127.0.0.1:8090/ | grep -o 'wordmark-mark'
```

That last line should print `wordmark-mark`. Until it does, the page serves the
OLD markup with the NEW stylesheet. That combination is safe — `.wordmark img`
carries an 18px fallback height for exactly this window — but the mark is
absent. Without that fallback the classless `<img>` fell back to its intrinsic
560×60 and pushed a 390px page out to 717px; measured, on the running service.

## A6. RESTART THE SERVICE · the canvas rework is not live until you do

`systemctl restart` needs interactive auth, which this session does not have,
so the running process is still the pre-canvas binary. Everything is built,
migrated and committed; the templates and the masthead simply are not being
served yet.

```bash
sudo systemctl restart evermore
```

Then one hard reload (Ctrl+Shift+R). The asset-version token is computed ONCE
at boot from the CSS file mtimes (`assetVersion`, a `sync.OnceValue`), so
without the restart every browser is handed a URL it already has cached and
keeps the old stylesheet — which is exactly why the last change appeared not to
have shipped. Migration 0031 is already applied to this database.

## B. The signed-in screenshots — captured, but READ THIS FIRST

`scripts/shoot-screens.js` now captures every screen, including the ones behind
a login, and `docs/screenshots/` is current as of 2026-08-31:

```bash
cd /home/dev/projects/healthy_catering
export NODE_PATH=/home/dev/.npm/_npx/e41f203b7505f1fb/node_modules
node scripts/shoot-screens.js            # defaults to 127.0.0.1:8081
```

**What the signed-in shots do and do not prove.** They put a session in
`localStorage` and serve the API from fixtures in the script, because
`RequireAuth` bounces a session-less browser to `/login` and `api create-staff`
needs a database write this environment does not grant. So they prove the real
React components, the real compiled stylesheet and the real response *shapes* —
layout, colour, overflow, and every empty/loading/error branch. They do **not**
prove the server returns those shapes. A screen that looks right in
`docs/screenshots/` can still 500 against the live API.

The three public captures (`public-home-*`, `spa-login`) are the real server
with real data and carry no such caveat.

### B1. Verify the six new back-office screens against the REAL API

The screens built on 2026-08-31 — dashboard, menu calendar, pricing, coverage,
production sheet, packing labels — have never been rendered against live data.
That needs a staff account, which needs a database write:

```bash
cd /home/dev/projects/healthy_catering
set -a && . ./.env && set +a
./bin/api create-staff            # interactive; admin role
# then sign in at http://127.0.0.1:8081/app/login and walk:
#   /app/admin  /app/admin/calendar  /app/admin/pricing
#   /app/admin/coverage  /app/admin/production  /app/admin/labels
```

Watch specifically for the one thing fixtures cannot catch: a field name that
differs between the Go struct's JSON tag and the TypeScript type. `GET
/admin/kitchens` is brand new (added with these screens) and has no Go test yet.

### B2. The M1 menu band has never rendered with data

The home page's menu band shows this week's PUBLISHED meals. The seeded
calendar ends 2026-08-20, so on any date after that the band correctly renders
nothing and the page is a hero plus the price table. To see the band, publish a
current week from the back office (`/app/admin/calendar` → **Publish this
week**) and reload `/`.

### B4. The seeded menu photography is not food · DECIDE THIS

Every image in `web/public/images/menu/` is generic stock landscape, and the
public menu pages render them today:

```bash
cd /home/dev/projects/healthy_catering
# Look at any two:
#   ayam-panggang-brokoli.jpg   ("grilled chicken & broccoli")  -> a suspension bridge
#   udang-padang-nasi-merah.jpg ("Padang prawns, red rice")     -> a muddy road at sunset
ls web/public/images/menu/          # 19 files, all distinct, all wrong
```

The illustrated diet-tint fallback is already in place (`MealPhoto`), so the
honest interim is to clear the keys and let it show:

```sql
UPDATE scheduled_meal SET hero_photo_key = NULL;
```

That is a data decision and a brand one, so it is yours. A bridge captioned
"ayam panggang" is worse than a coloured tile, but neither is a photograph of
the food, and only you know when real photography lands.

### B3. The coverage map is a schematic, not Google Maps

`/app/admin/coverage` plots the real kitchen coordinates and service radii to
scale, but draws no streets. The tile layer needs the browser Maps key handed
to the SPA — the public pages already receive one as `PageData.MapsKey`, the
SPA does not. See §6 for the keys themselves; wiring is a small change to the
SPA bootstrap once a key exists. Recorded in `docs/10-design-system.md` §4.12.

## 0. What is already done — do NOT redo these

The build session completed the environment setup that this file previously
listed as pending. Verified on this server:

- PostgreSQL 18.4 with `healthy_catering` + `healthy_catering_test`, owned by
  the `healthy_catering` role; `postgis` 3.6.4, `btree_gist` and `citext`
  created in both.
- Redis satellite container `redis-shared` on 127.0.0.1:6379.
- `/home/dev/projects/healthy_catering/.env` written, mode 0600, git-ignored,
  with generated `JWT_SIGNING_KEY` and `TOTP_ENCRYPTION_KEY`.
- Migrations 0001–0011 applied.

To run it:

```bash
cd /home/dev/projects/healthy_catering
go build -o bin/api ./cmd/api
set -a && . ./.env && set +a
./bin/api migrate status
./bin/api serve
# then, in another terminal:
curl -s -X POST http://127.0.0.1:8081/api/v1/delivery-area/check \
  -H 'Content-Type: application/json' \
  -d '{"lat":-6.2200,"lng":106.8300}'
```

**The database password is only in `.env`.** If you need it:
`sudo vi /home/dev/projects/healthy_catering/.env`

## 1. Confirm the planning documents — this is the gate

Nothing below matters until you have read and confirmed the four planning
documents, and **the stack decision blocks the first line of code**:

- `docs/02-decisions.md` — 23 decisions, **D-1…D-5 (the stack) first**
- `docs/03-open-questions.md` — Q-1 (tax), Q-4 (real kitchens) and Q-13 (refund
  policy) have no safe default
- `docs/01-domain-model.md`, `docs/04-milestones.md`

Still open from the brand pass, and now urgent because M12 puts a Nourish-Green
header on every public page:

- Is there a **reversed-out logo** for dark fills, or should one be derived?
- Can we get **page 13** of the Mini Brand Guidelines, "Logo on Color Palette"?
- What are the **Erode** licence terms for web embedding?

## 2. Fonts

Inter is SIL OFL and can be pulled from the Google Fonts static files. **Erode
is not on Google Fonts** — it is an Indian Type Foundry face distributed via
Fontshare, so it needs downloading and its licence reading before first use.

```bash
# Both go here, with their licence text beside them.
mkdir -p /home/dev/projects/healthy_catering/web/public/fonts
```

## 3. Assets were copied, not moved

You asked for a move. The source is another user's home and a move there is not
reversible by this project, so the originals are still at `/home/aidev/asset/`.
To complete the move once you are happy the copies are right:

```bash
sudo rm -rf /home/aidev/asset/Logo /home/aidev/asset/Color_Palette /home/aidev/asset/Font
```

## 4. Database — already done here, kept for the next machine

**This is done on `claudedev` (see §0).** It is kept because the production node
and any second dev machine still need it, and because both extensions are
**required**, not optional: `btree_gist` for the price-overlap exclusion
constraints (`PROMPT.md` §5.3) and `postgis` for kitchen routing (§9).

```bash
sudo -u postgres createuser --pwprompt healthy_catering
sudo -u postgres createdb -O healthy_catering healthy_catering
sudo -u postgres createdb -O healthy_catering healthy_catering_test

# PostGIS is not in a default PostgreSQL install. This server runs PG 18,
# not the 16 the brief assumed.
sudo apt-get install -y postgresql-18-postgis-3

# Extensions need superuser; do this once per database, before migration 0001.
sudo -u postgres psql -d healthy_catering      -c 'CREATE EXTENSION IF NOT EXISTS btree_gist; CREATE EXTENSION IF NOT EXISTS postgis;'
sudo -u postgres psql -d healthy_catering_test -c 'CREATE EXTENSION IF NOT EXISTS btree_gist; CREATE EXTENSION IF NOT EXISTS postgis;'

# Verify before trusting a green migration run.
sudo -u postgres psql -d healthy_catering -c '\dx'
```

## 5. Redis satellite — already done here (§0), kept for the next machine

Per `99-steven-preference.md` §9, Docker is for satellites. Redis carries the
settings cache, rate limits and the notification queue — **not** idempotency
keys, which stay in Postgres (`02-decisions.md` D-4).

```bash
docker run -d --name redis-shared --restart unless-stopped \
  -p 127.0.0.1:6379:6379 redis:7-alpine
```

## 6. Google Maps — keys, restrictions, quotas · STILL NEEDED

Browser work, not terminal work. **Two separate keys**, neither in git.

The console's menu labels move between redesigns, so each step below says what
the thing *does* as well as what it is currently called.

### 6.1 Project and billing

1. <https://console.cloud.google.com> → project picker (top bar) → **New
   project**. Name it `evermore-maps`. Note the project ID.
2. **Billing** → link a billing account. Maps returns
   `BillingNotEnabledMapError` without one **even inside the free allowance** —
   the card must be on file before any key works.
3. Google has changed the Maps free-tier model more than once. Read the current
   allowance in the console rather than trusting a remembered figure, then set
   the caps in §6.4 regardless.

### 6.2 Enable exactly three APIs

**APIs & Services → Library**, enable only:

| API | Used by | Why |
|---|---|---|
| Maps JavaScript API | browser key | renders the pin picker |
| Places API | browser key | address autocomplete |
| Geocoding API | server key | address → coordinates, server-side |

**Not** Distance Matrix or Routes: routing ranks by straight-line distance
(D-18), and travel time is a later resolver swap. Every extra enabled API is
extra billable surface for a stolen key.

If the library offers both *Places API* and *Places API (New)*, take the New
one — the legacy service is on a deprecation path — and say which you enabled,
because the client call differs.

### 6.3 The two keys

**APIs & Services → Credentials → Create credentials → API key**, twice. Rename
them immediately (`evermore-browser`, `evermore-server`) — two keys called
"API key 1" and "API key 2" is how the wrong one ends up in the wrong place.

**Browser key** — it ships inside the HTML and is world-readable, so the
referrer restriction is the *only* thing protecting it:

- Application restrictions → **Websites**:
  - `https://www.evermore.co.id/*`
  - `https://evermore.co.id/*`
  - `http://192.168.88.101:8090/*` (this dev host — remove at launch)
- API restrictions → **Maps JavaScript API** and **Places API** only.

**Server key** — never leaves the machine:

- Application restrictions → **IP addresses**: the production node's public IP,
  plus this dev host while building.
- API restrictions → **Geocoding API** only.

### 6.4 Caps and alerts — do not skip

An address form is a loop an attacker can run. Without a ceiling the first
symptom is the invoice.

1. **APIs & Services → <each API> → Quotas** → set a daily request cap. Start
   low (1 000/day); a real launch will tell you the real number.
2. **Billing → Budgets & alerts** → a budget with alerts at 50/90/100%.
3. A budget alert **notifies, it does not stop spending**. The quota caps in
   step 1 are the thing that actually halts it.

### 6.5 Install and verify

```bash
sudo vi /etc/evermore/evermore.env
```

```ini
GOOGLE_MAPS_BROWSER_KEY=AIza...
GOOGLE_MAPS_SERVER_KEY=AIza...
```

```bash
sudo systemctl restart evermore
systemctl is-active evermore
```

Verify the server key — run this **from the machine whose IP you allow-listed**,
or the restriction correctly rejects it:

```bash
set -a; . /etc/evermore/evermore.env; set +a
curl -s "https://maps.googleapis.com/maps/api/geocode/json?address=Kemang,+Jakarta+Selatan&key=$GOOGLE_MAPS_SERVER_KEY" \
  | head -c 300
```

`"status" : "OK"` with a `location` is a working key. `REQUEST_DENIED` with an
error message names the cause — usually the API not enabled, or the calling IP
not in the allow-list.

The browser key cannot be tested with `curl`: a referrer restriction only holds
for a real browser request. It is exercised the first time the picker loads.

### 6.6 Note on production boot

Production **refuses to start** without both keys. That is deliberate — a
misconfigured production node should fail loudly at boot rather than quietly at
the first order — but it means these keys block deployment, not just the picker.

## 7. Credentials still needed from you · STILL NEEDED

Each blocks a milestone — see `03-open-questions.md` Q-19…Q-29.

```bash
# Edit the real env file with vi; it is not in git.
sudo vi /etc/evermore/evermore.env
```

- **Bank accounts** for payment instructions — bank, number, holder (M8).
- **SMTP relay** host, port 587 credentials, and the `From` domain with SPF,
  DKIM and DMARC records published (M11).
- **WhatsApp**: either Meta Cloud API business verification with templates
  submitted ~1 week ahead, or a WAHA container on a spare number (M11).
- **Legal entity** name, address, NPWP for invoices and the site footer.
- **Production host** in the Jakarta region, for UU PDP data residency (M14).

## 8. WhatsApp — re-link the WAHA session · STILL NEEDED

Needs a phone and a QR scan, so it cannot be done from here.

The channel is switched on and pointed at the WAHA container **shared with
ruuma** (`http://127.0.0.1:3000`, session `default`), which is already bound to
`628176315568` — the number Steven gave. But the session reported
`status=FAILED` when it was wired, which means **nothing sends**. Messages queue
and retry with backoff rather than vanishing, so they will flow once it is
healthy.

Note this is ruuma's container too: a failed session means ruuma's WhatsApp is
down as well.

```bash
# What the session thinks it is doing:
set -a; . /home/dev/projects/ruuma/.env; set +a
curl -s -H "X-Api-Key: $WAHA_API_KEY" "$WAHA_URL/api/sessions" | python3 -m json.tool

# Restart it, then fetch the QR and scan it with the WhatsApp app on that phone:
curl -s -X POST -H "X-Api-Key: $WAHA_API_KEY" "$WAHA_URL/api/sessions/default/restart"
curl -s -H "X-Api-Key: $WAHA_API_KEY" "$WAHA_URL/api/default/auth/qr" -o /tmp/waha-qr.png
```

Confirm with `status=WORKING`, then send one real message to yourself before
trusting it:

```bash
curl -s -X POST "$WAHA_URL/api/sendText" -H "X-Api-Key: $WAHA_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"session":"default","chatId":"628176315568@c.us","text":"Evermore test"}'
```

While the session is `FAILED` this returns `422 Session status is not as
expected`, which is exactly what the app surfaces into the job row — so a
failure here and a failure in the queue have the same cause and the same text.

### Rotate the WAHA API key

The key was briefly committed in migration `0014` (commit `112ef88`) before
being moved to the environment. The gateway only listens on `127.0.0.1` and the
repository is private, so the exposure is limited — but the key is in git
history, and **rotating it is cheaper than rewriting published history**.

After rotating, put the new value in `WAHA_API_KEY` in
`/etc/evermore/evermore.env` and in ruuma's `.env` (the container is shared),
then restart both services. Nothing else needs to change: the key is read from
the environment, not from the database or a migration.

## 9. DNS for dev.evermore.co.id · STILL NEEDED

nginx answers on `dev.evermore.co.id` and `APP_BASE_URL` points at it, but the
name does not resolve, so everything is reached by IP today. Verification links
in email already carry the hostname, so those links are dead until this lands.

Point an A record at this host and confirm:

```bash
getent hosts dev.evermore.co.id
curl -sI http://dev.evermore.co.id:8090/ | head -1
```

## 10. Google Maps key hygiene · STILL NEEDED

The supplied key works — the five kitchens were geocoded with it. Two problems
remain, and both are billing risks rather than bugs:

1. **It is unrestricted** (`0.0.0.0/0`) and is used for both the browser and the
   server role. The browser role ships the key in page source, so anyone who
   views source can spend against it.
2. **There are no quota caps.** An address form is a loop somebody can script.

Follow §6.3 and §6.4 to split it into a referrer-restricted browser key and an
IP-restricted server key, and set per-API daily caps. A budget alert notifies;
only the quota cap stops the spending.

