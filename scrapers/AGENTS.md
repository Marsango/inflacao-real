# Scrapers — Python

## Scope and responsibilities

This directory contains shop-specific collectors and shared collection utilities. Collectors may run on multiple machines and submit observations over HTTP to a centralized Go core using the shared [ingestion v1 contract](../docs/contracts/ingestion/v1/README.md) and [JSON Schema](../docs/contracts/ingestion/v1/schema.json). Use its examples when implementing Pydantic models and delivery.

The core owns cross-shop product matching, inflation calculations, and persistence. Scrapers must not access the application database directly.

## Selected stack

- **Botasaurus:** website collection requiring browser interaction or browser-like HTTP requests. Keep its decorators, driver objects, and execution settings inside the collection layer.
- **HTTPX:** public API collection, straightforward HTTP fetching where appropriate, and the shared client that submits batches to the core. Reuse configured clients with explicit timeouts and connection limits. Use synchronous clients initially; introduce `AsyncClient` for collection workloads that benefit from bounded concurrency.
- **Pydantic:** explicit models for observations, submission envelopes, and configuration. Validate extracted data before batching and serialization. Keep these models aligned with the core-owned contract rather than defining a separate Python-only wire format.

These are the initial libraries. Do not introduce another browser framework or HTTP client without a concrete collection requirement. Select compatible dependency versions and document installation and browser requirements when the Python project is initialized.

## Collection strategy

For each shop, prefer a public API that supplies the required price and location context. Otherwise fetch the available HTML or embedded structured data directly. Use a Botasaurus browser when the required data or interactions need browser execution.

Choose the method per shop rather than requiring every collector to launch a browser. Both API and website collectors must produce the same observation model and use the same delivery pipeline.

## Implementation patterns

- Isolate each shop's selectors, pagination rules, and source-specific parsing in its own module or package.
- Register each shop's `Adapter` in `src/inflacao_scrapers/adapters/registry.py`. Use a shop-specific subclass of shared settings and shared `ParsedPage` results. The runner executes enabled `[[scrapers]]` entries sequentially with isolated clients and queues; adapters must not implement their own batching or delivery.
- Separate fetching, parsing, and submission. Parsing should accept saved source data and return observations without network calls, making page changes easy to diagnose and test.
- Use a common pipeline: collection → parsing → Pydantic validation → bounded batching → durable delivery to the core. Shop adapters should not implement their own submission clients.
- Share HTTP configuration and the core API client where behavior is common. Introduce shared extraction abstractions only after multiple shops demonstrate the need.
- The three current shops share VTEX fetching and exact parsing in `src/inflacao_scrapers/vtex/`. Angeloni browses the catalog once per run and filters the complete core scope locally; Festval and Muffato use target searches. Regional delivery adapters must resolve the configured CEP, require the expected seller, and preserve the resulting context before collecting prices.
- Prefer straightforward functions and explicit data models over deep class hierarchies. Use type hints at module boundaries.
- Parse prices using exact decimal arithmetic. Handle Brazilian separators explicitly; never pass monetary values through binary floating-point arithmetic, including when decoding API JSON. Serialize money according to the shared contract.
- Configure Pydantic validation deliberately so invalid or missing values cannot become plausible observations through unintended coercion. Require timezone-aware observation timestamps and reject non-finite monetary values.
- Keep shop URLs, collection scope, credentials, and runtime settings configurable. Do not hardcode environment-specific values.

## Botasaurus configuration

- Disable collection-result caching for fresh price measurements. Saved fixtures and deliberate replay are separate workflows and must preserve the original observation time.
- Configure explicit task error propagation, such as `raise_exception=True`, and report failures at the run boundary. A failed task must not silently become an empty successful result.
- Disable automatic exports with `output=None` where the shared delivery layer owns output. Diagnostic files and saved fixtures should have explicit retention and must not replace the delivery spool.
- Reuse browser instances within a defined shop/store session where appropriate. Isolate cookies and session state between distinct store, account, or delivery-region contexts.
- Bound browser concurrency and retries. Avoid multiplying framework retries with outer collection and delivery retries without a shared attempt/time budget.
- Keep blocking browser work out of an async event loop. If async API collection is introduced, separate its execution from synchronous browser tasks.
- Verify actual price extraction from the intended runtime machines. Browser-detection claims are not evidence that a shop is supported; report blocked or challenged runs explicitly.

## Observation quality

Follow the contract's required and optional fields. Capture the available context needed to interpret each price:

- Shop, source product identifier, product URL, name, brand, and identifiers such as a barcode.
- Package quantity and unit, including size changes.
- Price, currency, and promotion, membership, payment, or quantity conditions.
- Availability, seller, store branch, and delivery region when relevant.
- Observation time with a timezone; the core records receipt time separately.

Preserve source identifiers and context rather than guessing missing values or product equivalence. Do not submit a missing or unparseable price as zero. Surface malformed observations and unexpected page structures so a broken scraper cannot silently appear successful.

## Collection and delivery

- Respect applicable access restrictions and use conservative, configurable request rates.
- Set request timeouts and bound retries with backoff. Honor server retry guidance when supplied. Configure HTTPX redirect behavior explicitly for shop fetching; do not forward core credentials to an unexpected redirect destination.
- Distinguish transient network failures from parser errors and invalid submissions; retry only failures that can reasonably succeed later.
- Bound submission batches by both observation count and encoded request size according to the core contract. Validate a batch before sending it; the initial core design accepts or rejects each submitted batch as a whole.
- Send outgoing batches directly to the core. Only persist a batch locally when delivery fails after bounded retries, preserving the exact attempted bytes for redelivery. Open the durable spool lazily so successful collection does not create local storage. Stop collection explicitly after saving a failed batch, and report spool exhaustion without continuing collection.
- Follow the contract's idempotency rules when retrying delivery. Reuse the scraper identity, submission ID, payload content, observation identities, and timestamps rather than creating a new measurement on every attempt.
- Mark a local batch delivered after the ingestion endpoint returns `202 Accepted` for a durably stored batch or `200 OK` for an identical duplicate. Both responses have empty bodies. A lost response must trigger safe redelivery. Keep submission IDs in delivery logs; background processing is managed internally by the core.
- Surface validation errors and submission-ID conflicts for investigation instead of retrying them indefinitely. Corrected content requires a new submission ID while preserving observation identity as defined by the contract.
- Report collection counts, rejected observations, and failures with enough shop and run context to investigate. Keep credentials out of logs.
- Make runs restartable and report incomplete collection explicitly. Add scheduling or additional distributed infrastructure when operational requirements call for it.

## Verification

Test API and HTML parsers with small, representative saved fixtures. Cover Brazilian price formats, exact JSON monetary decoding, package quantities, conditional discounts, unavailable products, and missing fields where the shop supports them.

Verify Pydantic models and serialized payloads against shared contract examples. Exercise durable-spool recovery, lost-response redelivery with unchanged identity/content, batch limits, and rejected submissions when implementing delivery. Check that caching cannot substitute old prices for fresh observations and that collection failures remain visible.

Keep routine tests independent of live shops and browsers; use explicit smoke checks when validating actual collection and browser runtime requirements.

Document actual setup and run commands when tooling is introduced. Do not describe unimplemented commands as available.
