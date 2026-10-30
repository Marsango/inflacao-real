# Shared contracts

The [scraper ingestion schema](ingestion/v1/schema.json) defines the shared payload
that Python scrapers send and the Go core reads. Python models and Go types must
follow this same schema.

- [Scraper ingestion v1](ingestion/v1/README.md): payload fields, examples, and validation rules.
- [Monitoring scope v1](monitoring/v1/README.md): shared collection targets, discovery
  rules, and coverage semantics for all shop adapters.

- [Web API v1](web/v1/README.md): public product catalog, daily histories,
  price-basis and buying-condition policies, coverage and overview methodology.

The Go core implements submission validation, durable queueing, and background
normalization and observation processing. See [the core worker documentation](../../core/README.md#background-worker)
for retry settings and operational behavior.
