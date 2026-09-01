# 14 — Production deployment handbook

**Copy-paste, from an empty Ubuntu machine, with full absolute paths.**
Editor is `vi` throughout (99 §2).
**Date:** 2026-08-13

Everything here was **run on the development server** (`claudedev`) and works;
the production-only steps — the domain, the TLS certificate, the real
credentials — are marked ⚠️ and have not been run because those inputs do not
exist yet.

---

## 0. What you need before you start

| Input | Why | State |
|---|---|---|
| A Jakarta-region Ubuntu LTS node with sudo | UU PDP data residency | ⚠️ not provisioned |
| `evermore.co.id` DNS pointing at it | TLS and OG previews | ⚠️ not pointed |
| Real bank account details | Payment instructions | ⚠️ placeholder |
| Google Maps browser + server keys | Address pin picker | ⚠️ not issued |
| SMTP relay with SPF/DKIM/DMARC | Transactional email | ⚠️ mailpit only |
| Real kitchen list with map pins | Routing | ⚠️ placeholders |

**Do not point a public domain at this until the bank account is real.** The
seeded account reads `PT EVERMORE (DUMMY — REPLACE BEFORE LAUNCH)`, and a live
site would print those instructions to customers.

---

## 1. System packages

```bash
sudo apt-get update
sudo apt-get install -y postgresql-18 postgresql-18-postgis-3 nginx git curl ca-certificates
```

Go is only needed if you build on the box. The binary is static, so building
elsewhere and copying it is also fine.

```bash
# Only if building here:
curl -fsSLO https://go.dev/dl/go1.26.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.26.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
```

## 2. Database

```bash
sudo -u postgres createuser --pwprompt evermore
sudo -u postgres createdb -O evermore evermore
sudo -u postgres createdb -O evermore evermore_test

# Extensions need superuser and must exist BEFORE the first migration.
sudo -u postgres psql -d evermore      -c 'CREATE EXTENSION IF NOT EXISTS btree_gist; CREATE EXTENSION IF NOT EXISTS postgis; CREATE EXTENSION IF NOT EXISTS citext;'
sudo -u postgres psql -d evermore_test -c 'CREATE EXTENSION IF NOT EXISTS btree_gist; CREATE EXTENSION IF NOT EXISTS postgis; CREATE EXTENSION IF NOT EXISTS citext;'

# Verify before trusting a green migration run.
sudo -u postgres psql -d evermore -c '\dx'
```

## 3. Redis satellite

```bash
sudo apt-get install -y docker.io
sudo docker run -d --name redis-shared --restart unless-stopped \
  -p 127.0.0.1:6379:6379 redis:7-alpine
sudo docker exec redis-shared redis-cli ping   # expect PONG
```

## 4. The code

```bash
sudo mkdir -p /srv/evermore && sudo chown "$USER":"$USER" /srv/evermore
git clone https://github.com/stevenwilliam/healthy_catering.git /srv/evermore
cd /srv/evermore
go build -ldflags "-X main.version=$(git rev-parse --short HEAD)" -o /srv/evermore/bin/api ./cmd/api
/srv/evermore/bin/api version
```

The single-page app is built separately and served by the Go binary from
`web/dist`. **Skipping this step leaves `/app` serving nothing**, which looks
like a routing bug and is not one:

```bash
cd /srv/evermore/web
npm ci
npm run build
ls -l /srv/evermore/web/dist/index.html
```

Rebuild it on every deploy that touches `web/` — the binary serves whatever is
on disk, so a stale `dist` silently ships the previous UI.

## 5. Configuration and secrets

Nothing secret is in the repository. The service reads one root-owned file.

```bash
sudo mkdir -p /etc/evermore
sudo vi /etc/evermore/evermore.env
```

```ini
APP_ENV=production
APP_BIND=127.0.0.1
APP_PORT=8081
APP_BASE_URL=https://www.evermore.co.id
APP_TIMEZONE=Asia/Jakarta
LOG_LEVEL=info
CORS_ALLOWED_ORIGINS=https://www.evermore.co.id
TRUSTED_PROXIES=127.0.0.1

DATABASE_URL=postgres://evermore:REPLACE@127.0.0.1:5432/evermore?sslmode=disable
TEST_DATABASE_URL=postgres://evermore:REPLACE@127.0.0.1:5432/evermore_test?sslmode=disable

# openssl rand -base64 48
JWT_SIGNING_KEY=REPLACE

# openssl rand -base64 32 — encrypts staff 2FA secrets at rest.
#
# Leave it EMPTY and two-factor authentication is switched off entirely: the
# routes disappear and the boot log says so. That is deliberate — storing TOTP
# secrets in the clear would be worse than not offering the feature.
#
# CHANGING IT MAKES EVERY EXISTING ENROLMENT UNREADABLE, and admin/finance/staff
# have no way back in except a recovery code. Treat it like the database
# password, not like a rotatable token.
TOTP_ENCRYPTION_KEY=REPLACE

REDIS_URL=redis://127.0.0.1:6379/0
GOOGLE_MAPS_BROWSER_KEY=REPLACE
GOOGLE_MAPS_SERVER_KEY=REPLACE
TURNSTILE_SECRET=REPLACE
```

```bash
sudo chown root:evermore /etc/evermore/evermore.env
sudo chmod 640 /etc/evermore/evermore.env
```

**Production refuses to start** without both Maps keys and a CORS list — the
config validates that deliberately, so a misconfigured production node fails
loudly at boot rather than quietly at the first order.

## 6. The service

```bash
sudo cp /srv/evermore/deploy/evermore.service /etc/systemd/system/evermore.service
sudo vi /etc/systemd/system/evermore.service     # set User=, WorkingDirectory=, paths
sudo systemctl daemon-reload
sudo systemctl enable --now evermore
systemctl status evermore --no-pager
```

Migrations run in `ExecStartPre`, so **the schema is applied before the new
binary serves** — a rollout can never serve against a schema it does not
expect.

## 7. First admin account

There is no default admin and no seeded password.

```bash
cd /srv/evermore
set -a; . /etc/evermore/evermore.env; set +a
/srv/evermore/bin/api create-staff --email ven@evermore.co.id --role admin --name "Steven William"
# The password is typed, never echoed, and never reaches the shell history.
```

### 7.1 Enrol two-factor immediately

Two-factor authentication is **mandatory for admin, finance and staff** and
cannot be switched off from those accounts. Sign in at
`https://www.evermore.co.id/app/login`, open **Keamanan**, scan the secret into
an authenticator app and confirm with a code.

**Write the eight recovery codes down before leaving that screen.** They are
shown once, each works once, and for a mandatory role they are the only way
back in after a lost phone.

Kitchen and courier roles are exempt by design — they sign in from shared
phones on a service floor (docs/03 Q-16).

## 8. nginx and TLS

```bash
sudo cp /srv/evermore/deploy/nginx/evermore.conf /etc/nginx/sites-available/evermore
sudo vi /etc/nginx/sites-available/evermore   # listen 80/443, real server_name
sudo ln -sf /etc/nginx/sites-available/evermore /etc/nginx/sites-enabled/evermore
sudo nginx -t && sudo systemctl reload nginx
```

⚠️ Then, once DNS resolves:

```bash
sudo apt-get install -y certbot python3-certbot-nginx
sudo certbot --nginx -d www.evermore.co.id -d evermore.co.id
sudo certbot renew --dry-run
```

Only after the certificate exists, uncomment the `return 301 https://…` line
and the TLS server block. Doing it earlier breaks the ACME challenge.

## 8a. Firewall — the port must be opened explicitly

`ufw` defaults to `deny (incoming)`. nginx binding `0.0.0.0:8090` is **not**
enough on its own: the listener is up, the host answers on `127.0.0.1` and on
its own LAN address from a shell on the box, and every request from anywhere
else is dropped before it reaches nginx. The symptom is a browser that hangs
and then times out, with **nothing at all in the nginx access log** — no 403,
no 404, no entry, because the packet never arrived.

```bash
sudo ufw status verbose          # is the port listed at all?
sudo ss -tlnp | grep 8090        # is anything actually listening?
```

Open it scoped to the network that needs it, not to the world:

```bash
sudo ufw allow from 192.168.88.0/24 to any port 8090 proto tcp \
     comment 'evermore dev (LAN only)'
sudo ufw status numbered
```

### The trap: scope the rule to the address the client *actually arrives from*

Opening the port to "the LAN" is only correct if the browser really reaches
this host from the LAN subnet. On `claudedev` it does not. The box is a VM on
`ens32` at `192.168.88.101`, but Steven's PC arrives as **`172.16.0.1`** — the
VMware host-side virtual adapter — so a rule scoped to `192.168.88.0/24` keeps
dropping him and the browser keeps timing out. Guessing the subnet is what
costs the second round trip; **read it out of the block log instead**:

```bash
sudo grep 'DPT=8090' /var/log/ufw.log /var/log/ufw.log.1 \
  | grep -o 'SRC=[0-9.]* DST=[0-9.]*' | sort | uniq -c | sort -rn
#   36 SRC=172.16.0.1 DST=192.168.88.101      <- the real client address
```

An empty nginx access log tells you the packet never arrived; this tells you
*who* was turned away. Then open it to that source:

```bash
sudo ufw allow from 172.16.0.0/24 to any port 8090 proto tcp \
     comment 'evermore dev (VMware host net)'
```

Both rules are needed on this host — `192.168.88.0/24` for anything on the
physical LAN, `172.16.0.0/24` for the PC hosting the VM.

⚠️ Port 80 on the bare IP is **ruuma's** `default_server`, not Evermore. Typing
`http://192.168.88.101/` loads the wrong project and looks like a deploy that
went wrong; Evermore is `http://192.168.88.101:8090/` until DNS and TLS exist.

In production this stage disappears: the site moves to 443, which the
`Nginx Full` profile already allows, and 8090 should then be **removed**:

```bash
sudo ufw status numbered         # find the rule number
sudo ufw delete <number>
```

⚠️ Check what else is open while you are here. On the dev host `5432/tcp` is
allowed from **Anywhere** and Postgres listens on `0.0.0.0`, which puts the
database on the network. That is fine for a lab box behind a router and is
wrong on anything public — bind Postgres to `127.0.0.1` and drop the rule, or
scope it to a single admin address.

## 8b. Sample menu (non-production convenience)

Fills the calendar so the public menu pages are not empty on a fresh
environment. Idempotent — it never touches a menu that already exists.

```bash
cd /home/dev/projects/healthy_catering
set -a && . ./.env && set +a
./bin/api seed-menu        # today and the next two days
./bin/api seed-menu 7      # a week
```

Placeholder pictures for those menus, downloaded once and stored locally:

```bash
./bin/api seed-menu-images
```

⚠️ These are **random** photographs from Lorem Picsum (Unsplash licence, free
commercially). They are not pictures of this food — a menu called "Ayam
Panggang" may show a doorway. They exist so the cards are not blank during
development and **must not reach a customer**. Clear them with:

```bash
psql "$DATABASE_URL" -c "UPDATE scheduled_meal SET hero_photo_key = NULL;"
```

The cards then fall back to the illustrated band in each diet type's colour,
which is honest about being artwork.

**Not for production once real menus are being entered.** It only ever inserts
where nothing is scheduled, so it cannot overwrite a real menu, but a
production calendar should be filled by the kitchen through the back office.

## 9. Verify the deployment

```bash
curl -s https://www.evermore.co.id/healthz
curl -s https://www.evermore.co.id/robots.txt | head -3

# What a WhatsApp preview bot sees. Use curl, not a browser — the browser
# hides exactly what this shows (99 §13).
curl -s https://www.evermore.co.id/ | grep -i 'og:\|<title'
```

## 10. Backups

The script ships in the repository and **has been run**, including the restore
drill — which is the only part that proves anything.

```bash
sudo mkdir -p /var/backups/evermore
sudo install -m 0755 /srv/evermore/scripts/backup.sh /usr/local/bin/evermore-backup.sh
sudo install -m 0755 /srv/evermore/scripts/restore-check.sh /usr/local/bin/evermore-restore-check.sh
```

Nightly, as root:

```bash
sudo vi /etc/cron.d/evermore-backup
```

```
15 2 * * * root . /etc/evermore/evermore.env; /usr/local/bin/evermore-backup.sh >> /var/log/evermore-backup.log 2>&1
```

The script refuses to leave a broken dump behind: it verifies the gzip stream
and checks the dump ends with the `PostgreSQL database dump complete` marker,
because a truncated file still *looks* like a backup in a directory listing.

⚠️ **The copy still lands on the same machine as the database.** Uncomment the
`aws s3 cp` or `rclone copy` line at the bottom of the script once a bucket
exists. A backup that burns with the server is not a backup.

### 10.1 The restore drill — run this monthly

```bash
sudo -E /usr/local/bin/evermore-restore-check.sh
```

It drops and recreates `healthy_catering_test`, restores the newest dump into
it, prints the row counts, and runs the security suite **against the restored
copy**. A dump nobody has restored is a hypothesis, and the moment you need it
is the worst possible time to find that out.

Two things to know:

- It runs as the **database superuser** (`sudo -u postgres`), because a
  `--clean` dump recreates extensions and only a superuser may do that. This is
  normal for a restore and is exactly why the application role is not one.
- It **destroys** `healthy_catering_test`. That database exists for this and for
  the test suite; never point `RESTORE_DB` at production.

## 11. Rollback

```bash
cd /srv/evermore
git log --oneline -5
git checkout <previous-sha>
go build -ldflags "-X main.version=$(git rev-parse --short HEAD)" -o /srv/evermore/bin/api ./cmd/api
sudo systemctl restart evermore
```

Migrations are **forward-only in production**. Rolling the binary back is safe;
rolling the schema back is not, and `api migrate down` refuses in production
unless `ALLOW_MIGRATE_DOWN=yes` is set deliberately.

---

## 12. What is deployed on `claudedev` today

Run and verified on 2026-08-13:

| Item | Value |
|---|---|
| URL | `http://192.168.88.101:8090` (IP + own port, as Steven asked) |
| API bind | `127.0.0.1:8081` — **not reachable from the LAN**, verified |
| nginx | `/etc/nginx/sites-available/evermore`, listening on 8090 |
| systemd | `evermore.service`, enabled, restarts cleanly |
| Secrets | `/etc/evermore/evermore.env`, `root:dev`, mode 640 |
| Database | `healthy_catering` on the shared PostgreSQL 18 |

ruuma is untouched: it still owns `listen 80 default_server` and answers on the
bare IP.
