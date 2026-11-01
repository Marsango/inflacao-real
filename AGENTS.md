# Nossa inflação

## Project purpose

Nossa inflação is an independent price-monitoring platform for the Brazilian retail market. It collects prices from online shops, tracks how they change over time, and turns those observations into accessible insights about inflation and purchasing power.

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
- For JavaScript and TypeScript, add only well-trusted, actively maintained dependencies. Prefer dependencies maintained by established companies such as Meta, Microsoft, or Google. Check trust and maintenance before adding a package; if it does not meet this standard, build a small component ourselves instead.
- Use the shared, versioned [ingestion contract](docs/contracts/ingestion/v1/README.md) for all shops. Schemas and examples live in `docs/contracts/`; update affected consumers together with contract changes.
- Preserve observation provenance and price conditions. Represent unknown values explicitly; a missing price is never zero.
- Use exact decimal representations or integer minor units for money and timestamps with an explicit timezone.
- Make metrics traceable to their data and methodology. Explain coverage, periods, and methodology differences when comparing with official indicators.
- Keep credentials and environment-specific configuration out of source control.
- Put task plans, implementation reports, research notes and other temporary or nonessential Markdown in `docs/plans/`, which must remain Git-ignored and untracked. This includes retailer onboarding reports such as `sao-paulo-retailers.md` and `sonda-retailer.md`. Keep maintained project instructions, setup documentation and API contracts in their established locations.
- Prefer a working vertical slice first: one shop, ingestion, persisted history, and a useful web view. Expand incrementally.
- Add every new adapter that is ready for collection to the existing schedule by default. Update the active configuration, versioned schedule template, shop registration and ingestion authorization together; preserve existing entries, the authorized collector identity and separate spool paths. Do not leave a ready adapter disabled or opt-in unless explicitly requested. If a concrete blocker prevents activation, explain it and distinguish configured scheduling from a running scheduler.
- Verify the existing-VPS deployment path when adding adapters: `git pull` followed by `./deploy up` must append missing ready adapters to the persisted `${NOSSA_CONFIG_DIR:-/etc/nossa-inflacao}/scrapers.toml` before recreating the scheduler. Updating only `scrapers/scrapers.example.toml` is insufficient because initialization preserves existing files. Preserve custom settings, secrets, spool paths and explicit disables; keep deployment registration and authorization aligned. Test upgrades from an existing configuration as well as fresh installs.
- PostgreSQL is the database; `compose.yaml` provides the local instance. React tooling, scheduling, and deployment choices remain open. Introduce infrastructure and abstractions when concrete requirements justify them.
- Always use disposable Testcontainers for integration-test infrastructure. Reuse shared setup helpers that start the required containers, apply migrations, and register automatic cleanup after each test, including failures. Never run integration tests against existing development or production databases.

## Code design and Object Calisthenics

Apply these rules across the repository, based on [William Durand's Object Calisthenics](https://williamdurand.fr/2013/06/03/object-calisthenics/):

- Keep every function and method to **20 lines or fewer**. Each must do one clearly defined thing. Extract smaller functions with descriptive names when necessary; keep orchestration separate from individual operations.
- Use only one level of control-flow indentation per function or method. Extract nested loops, conditionals, and error-handling blocks into focused helpers.
- Avoid `else`. Use guard clauses and early returns to keep the main path clear.
- Follow one dot per line: avoid chained calls and reaching through multiple objects. Work with immediate collaborators. Fluent interfaces are exempt.
- Use full, descriptive names rather than abbreviations. Names should explain the responsibility without requiring readers to inspect the implementation.
- Keep entities small: classes must not exceed 50 lines, and packages must not exceed 10 files. Split them by cohesive responsibilities.
- Prefer behavior methods over getters, setters, and properties that expose state for decisions elsewhere. Keep decisions with the object that owns the state; reading state for presentation is acceptable.

Preserve KISS: extracting a focused function does not require introducing a class or design pattern.

## Using these instructions

This file applies throughout the repository. Read the `AGENTS.md` in each component you change for its implementation patterns and verification guidance. Keep project-wide rules here and component-specific details in their respective files; avoid duplicating guidance. Add deeper instructions only when a subdirectory needs distinct conventions.
