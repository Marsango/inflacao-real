# VPS deployment

The production stack runs PostgreSQL, the Go core, and all three current scrapers.
Collection runs daily at **09:00 America/Sao_Paulo**. There is no public website yet.
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
shop settings with `http://core:8080`. Secrets are root-readable only. The scraper config
is readable inside its container; the containing host directory is root-only.
To edit, use `sudoedit /etc/nossa-inflacao/production.env` or the TOML file, then rerun up.
Changing POSTGRES_PASSWORD in that file does not change an existing database's password;
password rotation requires an explicit matching PostgreSQL change.

The fixed Compose project name is `nossa-inflacao`. Named volumes `nossa-inflacao_postgres_data`
and `nossa-inflacao_scraper_data` survive container replacement and checkout relocation.
Do not run Compose down with --volumes. Application containers run as UID 10001, drop
capabilities, and cannot acquire new privileges. Docker logs rotate at 3 × 10 MB per service.
Host configuration, database data, and pending scraper batches never belong in Git.
The bootstrap leaves SSH and firewall rules unchanged. PostgreSQL has no published port;
the core is bound only to 127.0.0.1:8080. No reverse proxy, public TLS, or Docker socket
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
