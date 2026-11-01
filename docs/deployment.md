# VPS deployment

The production stack runs PostgreSQL, the Go core, and all three current scrapers.
Collection runs daily at **09:00 America/Sao_Paulo**. The frontend is optional and enabled by configuring a public domain.
The scheduler does not catch up on missed runs; after 09:00 its first run is tomorrow.
The existing local-development Compose configuration remains separate.

## First installation

Use an Ubuntu server with Git, Python 3.12+, Bash, OpenSSL, sudo, systemd, and Internet access.
The installer can install Docker Engine and Compose on Ubuntu 24.04/26.04 LTS.
Existing Docker/Compose installations are reused, including the current VPS's Ubuntu 25.04
installation. Its unsupported OS is retained; this deployment does not upgrade Ubuntu.

Commit and push the deployment files before cloning. Production builds require a clean
checkout so the backup's Git revision identifies its source. Existing uncommitted local
feature work must be committed deliberately if it should be part of the release.

```sh
git clone https://github.com/Marsango/nossa-inflacao.git
cd nossa-inflacao
./deploy up
./deploy status
./deploy logs
```

The command obtains sudo, enables Docker at boot, builds the images, initializes the
production database, runs embedded migrations, probes the authenticated monitoring
endpoint, registers shops idempotently, and starts scheduling. First installation uses
an empty database; local history is not imported. Builds finish before collection stops.
Migration/readiness failures prevent starting collection; inspect logs and fix the cause.

Configuration is created once under `/etc/nossa-inflacao/`: `production.env` contains
random secrets and `COLLECTION_TIME=09:00`; `scrapers.toml` contains the existing regional
shop settings with `http://127.0.0.1:8080`. Secrets are root-readable only. The scraper config
is readable inside its container; the containing host directory is root-only.
To edit, use `sudoedit /etc/nossa-inflacao/production.env` or the TOML file, then rerun up.
Changing POSTGRES_PASSWORD in that file does not change an existing database's password;
password rotation requires an explicit matching PostgreSQL change.

The core's calculated history cache is enabled automatically on fresh installs and
updates. It uses process memory, so no service or persisted configuration migration
is needed. Optional `HISTORY_CACHE_TTL=5m`, `HISTORY_CACHE_MAX_ENTRIES=500` and
`HISTORY_CACHE_MAX_BYTES=33554432` overrides can be added to `production.env`;
rerun `./deploy up` to recreate the core with those settings. Existing secrets and
scraper configuration are preserved. See the [core cache documentation](../core/README.md#calculated-history-cache)
for invalidation, memory accounting and log counters.

The fixed Compose project name is `nossa-inflacao`. Named volumes `nossa-inflacao_postgres_data`
and `nossa-inflacao_scraper_data` survive container replacement and checkout relocation.
Do not run Compose down with --volumes. Application containers run as UID 10001, drop
capabilities, and cannot acquire new privileges. Docker logs rotate at 3 × 10 MB per service.
Host configuration, database data, and pending scraper batches never belong in Git.
The bootstrap leaves SSH and firewall rules unchanged. PostgreSQL has no published port;
the core is bound only to 127.0.0.1:8080. Scrapers share the core container's
network namespace and reach its API over loopback; HTTPS validation remains unchanged. No reverse proxy, public TLS, or Docker socket
mount is needed. Publishing container ports can bypass UFW: retain these explicit bindings.

## Updates and operations

```sh
git pull --ff-only
./deploy up
./deploy status
./deploy logs
./deploy collect
./deploy deliver
```

`collect` runs a full configured collection immediately; `deliver` retries existing
queued batches. Both pause the scheduler and resume it afterwards, avoiding overlapping
local runs. Their errors remain visible. Manual commands and deployment/recovery commands
are serialized by a host lock; status and logs remain available independently.
The daily scheduler continues after collection failures; a running scheduler does not
prove fresh observations. Inspect collection summaries, retry queues, and core processing
logs. API acceptance means queued, not successfully processed. No alert service is installed.

To inspect the private API from your computer:

```sh
ssh -N -L 8080:127.0.0.1:8080 vps
```

Use your production ingestion token for authenticated requests. To inspect PostgreSQL
on the VPS without exposing it:

```sh
sudo docker compose --project-name nossa-inflacao \
  --env-file /etc/nossa-inflacao/production.env -f deployment/compose.yaml \
  exec postgres psql -U nossa_inflacao -d nossa_inflacao
```

## Manual encrypted backups

```sh
sudo install -d -m 700 /var/backups/nossa-inflacao
./deploy backup /var/backups/nossa-inflacao/history.enc
```

Supply and confirm a strong passphrase interactively; it is not placed in command arguments.
Backups contain a custom-format PostgreSQL dump, retry queues, production configuration
including secrets, and the deployed Git revision. Application writers pause during the
snapshot and restart in a finally block even if dumping fails. The snapshot uses temporary
local disk space and needs enough free capacity for the dump, queues, and archive.
Encryption is OpenSSL AES-256-CBC with PBKDF2 and 600,000 iterations; it does not authenticate
the archive. Restore only trusted, privately stored backups. Existing backup files are
never overwritten. Keep the passphrase separately; it cannot be recovered from the server.

Download to your workstation (the remote file is deliberately root-only):

```sh
ssh vps 'sudo cat /var/backups/nossa-inflacao/history.enc' > history.enc
chmod 600 history.enc
```

Compare `sha256sum` locally with `ssh vps 'sudo sha256sum /var/backups/nossa-inflacao/history.enc'`.
Keep an off-server copy; copies on the VPS do not protect against losing the VPS.
There is no automatic backup schedule. Take a backup before deploying migrations;
migrations are not automatically downgraded.

## Restore after losing or reinstalling the VPS

Clone the repository, check out the commit recorded with your backup, and copy the backup
to a private directory on the replacement server. The restore command also reports the
required commit if you start from a different revision. Record that revision alongside
your off-server backup to make recovery straightforward.

**Do not run up first.** Restore requires empty production configuration and no existing
production volumes. This protects any existing installation from accidental replacement.

```sh
git checkout <recorded-commit>
./deploy restore /absolute/path/history.enc
./deploy status
```

Restore decrypts into a private temporary directory, validates the archive members and Git
revision, restores configuration, builds the images, starts an empty PostgreSQL instance,
restores its dump and the queues, then starts the core and scheduler. Startup reuses the
normal migration and readiness checks. Authentication secrets are preserved from the backup.
A failed restore leaves its partial installation stopped for inspection; it does not erase
volumes to retry automatically. Resolve the reported failure deliberately before continuing.
After restoring the recorded release, take a backup before pulling and deploying newer code.

`NOSSA_CONFIG_DIR` can select an alternative absolute configuration directory; the wrapper
preserves it through sudo. The Compose project identity stays fixed, so this is not a way
to create a second production stack. Integration checks must use disposable Testcontainers,
never this deployment's database.

## Verification status

Both application images built locally before local testing was stopped at the owner's request.
No local integration or backup/restore test suite was run. VPS installation, repeated up,
readiness, shop registration, 09:00 scheduling, private ports, persisted observations, and
backup/restore recovery remain to be verified after the owner clones the committed files.

## Public frontend and HTTPS

The existing private deployment remains available with `./deploy up`. To enable the
frontend and its public read API, use a clean committed checkout containing both
frontend and backend changes:

```sh
./deploy up --domain nossainflacao.com.br
```

The domain is saved as `PUBLIC_DOMAIN` in `/etc/nossa-inflacao/production.env` without
changing existing credentials or the 09:00 scheduler. Later `./deploy up` commands
reuse it. The wrapper needs sudo and UFW for public web port configuration. Do not
edit Compose files or put ingestion tokens in frontend environment variables.

In Registro.br, keep the existing nameservers and add these DNS records:

| Type | Name | Value |
| --- | --- | --- |
| A | Root domain / empty name | 51.210.11.104 |
| CNAME | www | nossainflacao.com.br |

No AAAA record is required initially. Preserve unrelated mail/TXT records. Public
DNS must resolve to the VPS before certificate issuance can succeed; negative DNS
responses may remain cached while changes propagate. If CAA records are introduced,
allow the certificate authority used by Caddy. No Registro.br credentials are needed
by this deployment.

A pinned multi-stage image builds static assets with npm ci and serves them using
Caddy. Only TCP 80/443 are published by the website. HTTP redirects to HTTPS and www
redirects to the root domain. SSH remains on 49999. Caddy proxies GET requests to
`/api/v1/products` and `/api/v1/history`, plus the specific authenticated admin
paths and methods documented below. Ingestion, monitoring and other API routes
return 404 at the public edge. The core still binds only to host
loopback and PostgreSQL remains unpublished. The browser uses same-origin /api and
never receives the ingestion token. Hashed assets use immutable caching, HTML is
revalidated, and API no-store headers pass through unchanged.

The domain activates the public Compose profile. The deployment builds before
stopping collection, updates the core, verifies the web API exists, and starts the
website after the core is healthy. An older core without the web API fails the
readiness check instead of silently serving a broken dashboard. The website process
runs as UID 10001 and persists TLS state in `nossa-inflacao_caddy_data` and
`nossa-inflacao_caddy_config`. Do not delete these volumes during updates.
Website container health checks confirm Caddy is running; they do not guarantee
public DNS or certificate availability. Inspect TLS errors with `./deploy logs` and
verify the actual public URL before declaring a rollout complete.

Existing encrypted backups automatically include the domain inside production.env;
the archive format is unchanged. Certificate volumes survive updates on this VPS.
A fresh-server restore obtains replacement certificates automatically using the
restored domain and public DNS. Preserve certificate volumes when possible to avoid
unnecessary issuance and certificate-authority rate limits.

For updates, commit and push locally, then update the VPS checkout and run `./deploy up`.
If the VPS has the earlier agent-created deployment commit while your branch has a
replacement commit with the same changes, preserve the deployed revision on a backup
branch and align the clean checkout with your chosen release; do not create an
unreviewed merge commit. Back up before any update containing database migrations.
The frontend build is image-contained; no Node server or Vite development server runs
in production. Rolling back means deploying the previous compatible code revision;
it does not automatically downgrade database migrations.

Verification covers frontend type checking, unit tests, production build, and isolated
Testcontainers checks of static serving, frontend navigation, cache headers, domain
configuration, redirects, public API proxying and private-route denial. Integration
checks never use the existing VPS database. Final public acceptance includes valid TLS,
root/www redirects, both public API routes, and confirmation that private API paths and
the database remain unreachable externally. Do not start a collection for these checks.

## Product equivalence administration

The optional Portuguese interface is at `/admin`; its API is documented in
[admin v1](contracts/web/v1/admin.md). Only the explicitly listed admin methods
and paths pass through Caddy. Scraper ingestion and monitoring remain private.
Admin requires HTTPS, including during development (the cookie is always Secure).

Generate a password hash locally, without putting the password in shell history:

```bash
cd core
read -rs -p 'Owner password: ' owner_password
printf '%s' "$owner_password" | go run ./cmd/admin-password
unset owner_password
```

Put `ADMIN_USERNAME` and `ADMIN_PASSWORD_HASH` in the existing production secrets
file outside the checkout. Single-quote the hash in `production.env` to preserve
its literal `$` characters. Compose passes these optional values only to core.
No credentials means disabled routes. Invalid or partial credentials stop startup.
Changing the hash does not revoke existing sessions: use logout, or deliberately
clear `admin_sessions` when rotating a compromised owner credential.

Before deploying migration 00012, create and verify a production backup using
the existing backup procedure. Deploy the code/migration first: automatic generic
grouping starts **disabled**. Log in, inspect the generic preview and source links,
and verify representative banana groups, distinct patinho preparations, real
brands and ambiguous records. Activate only after reviewing the report. Activation
backfills existing eligible products in one catalog transaction; later ingestion
applies the same versioned rules. Product identities and observations remain intact.

Check member-ID histories, global coverage, manual representative selection,
removal and dissolution after activation. Reversal records exclusions so ingestion
cannot recreate the rejected unions. Deactivation alone stops future automatic
decisions; it does not dissolve existing groups. Keep the backup until these
checks succeed. No production migration, activation or source verification is
performed by the automated test suite.


### Adding ready collectors on an existing VPS

After committing and pushing adapter changes, pull them on the VPS and run
`./deploy up`. Deployment builds the updated images, stops the old collector,
appends missing adapters from `scrapers/scrapers.example.toml` to the persisted
`/etc/nossa-inflacao/scrapers.toml`, registers the shops and recreates the scheduler.
Existing collector entries, custom settings, spool paths and explicit disables
are preserved. `NOSSA_CONFIG_DIR` selects an alternative configuration directory.
New adapters are enabled by default in the template; an existing `enabled = false`
entry stays disabled until explicitly changed.

The scheduler uses `COLLECTION_TIME` from `production.env` (default `09:00`) in
`America/Sao_Paulo`. Collectors run sequentially, starting at that time; they do
not all start simultaneously. Deploying after the scheduled time waits until the
next day. `./deploy collect` remains the explicit immediate-collection command.
