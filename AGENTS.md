# Inflação Real

## Project purpose

Inflação Real is an independent price-monitoring platform for the Brazilian retail market. It collects prices from online shops, tracks how they change over time, and turns those observations into accessible insights about inflation and purchasing power.

The goal is to help people understand the price changes they encounter in everyday shopping and compare them with official inflation indicators when appropriate. Conclusions must follow the collected evidence.

## Architecture and ownership

```text
Brazilian shops → Python scrapers → Go core ↔ Database
                                      ↕
                                  React web UI
```

| Component | Responsibility | Local guidance |
| --- | --- | --- |
| `scrapers/` — Python | Collect shop data and submit standardized price observations. | [scrapers/AGENTS.md](scrapers/AGENTS.md) |
| `core/` — Go | Validate and store observations, calculate insights, and serve ingestion and web APIs. | [core/AGENTS.md](core/AGENTS.md) |
| `web/` — React | Present price histories and insights in an accessible interface for nontechnical users. | [web/AGENTS.md](web/AGENTS.md) |

The core owns business rules and persistence. Scrapers and the web UI communicate with it through documented APIs.

## Shared rules

- Follow KISS (Keep it stupid simple): choose the simplest clear solution that meets the current requirements.
- Don't overengineer anything. Don't build features that weren't asked for or add complexity for speculative edge cases.
- Use the shared, versioned [ingestion contract](docs/contracts/ingestion/v1/README.md) for all shops. Schemas and examples live in `docs/contracts/`; update affected consumers together with contract changes.
- Preserve observation provenance and price conditions. Represent unknown values explicitly; a missing price is never zero.
- Use exact decimal representations or integer minor units for money and timestamps with an explicit timezone.
- Make metrics traceable to their data and methodology. Explain coverage, periods, and methodology differences when comparing with official indicators.
- Keep credentials and environment-specific configuration out of source control.
- Prefer a working vertical slice first: one shop, ingestion, persisted history, and a useful web view. Expand incrementally.
- PostgreSQL is the database; `compose.yaml` provides the local instance. React tooling, scheduling, and deployment choices remain open. Introduce infrastructure and abstractions when concrete requirements justify them.
- Always use disposable Testcontainers for integration-test infrastructure. Reuse shared setup helpers that start the required containers, apply migrations, and register automatic cleanup after each test, including failures. Never run integration tests against existing development or production databases.

## Using these instructions

This file applies throughout the repository. Read the `AGENTS.md` in each component you change for its implementation patterns and verification guidance. Keep project-wide rules here and component-specific details in their respective files; avoid duplicating guidance. Add deeper instructions only when a subdirectory needs distinct conventions.
