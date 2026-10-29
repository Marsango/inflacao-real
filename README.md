# Inflação Real

Independent monitoring of Brazilian retail prices, built with Python scrapers,
a Go core, and a React web interface. See [AGENTS.md](AGENTS.md) for the architecture
and component guidance.

The [shared ingestion contract](docs/contracts/ingestion/v1/README.md) defines
the JSON payloads scrapers submit to the core, including examples and validation
checks. The core exposes an authenticated ingestion endpoint that queues submissions
in PostgreSQL for later processing.

The [Go core](core/README.md) applies embedded Goose migrations on startup and runs
the Gin ingestion API and background worker. The worker validates queued payloads,
automatically creates or matches products and listings, persists price histories,
and handles observation deduplication and retries. Shops are registered manually.
See its README for configuration and a submission example.

## Local database

Install Docker Engine and the Docker Compose plugin. Run commands from the
repository root.

Create your local configuration:

```sh
cp .env.example .env
```

Set `POSTGRES_PASSWORD` in `.env` to your own local password. The file is ignored
by Git. If `.env` already exists, keep it and edit its values as needed.

Start PostgreSQL and wait for it to become healthy:

```sh
docker compose up -d --wait
```

The setup uses the official `postgres:18.6` image, persistent named storage, and a
readiness health check. PostgreSQL is available on `127.0.0.1:5432` by default;
change `POSTGRES_PORT` in `.env` if that port is already in use.

Connection settings for a core process running on the host:

| Setting | Value |
| --- | --- |
| Host | `127.0.0.1` |
| Port | `POSTGRES_PORT` from `.env` (default `5432`) |
| Database | `POSTGRES_DB` from `.env` (default `inflacao_real`) |
| User | `POSTGRES_USER` from `.env` (default `inflacao_real`) |
| Password | `POSTGRES_PASSWORD` from `.env` |
| SSL mode | `disable` for this local setup |

A future core container on the same Compose network should connect to
`postgres:5432` instead of the host address and published port.

Open a SQL console without installing PostgreSQL on the host:

```sh
docker compose exec postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

Check status, view logs, or stop the database:

```sh
docker compose ps
docker compose logs -f postgres
docker compose down
```

`docker compose down` preserves the database volume. Adding `--volumes` deletes
the local database data. Database/user/password environment variables initialize
an empty volume; editing `.env` does not change credentials in an existing database.

Only PostgreSQL is containerized at this stage. The core, scrapers, and web UI
will be added as their implementations are introduced.
