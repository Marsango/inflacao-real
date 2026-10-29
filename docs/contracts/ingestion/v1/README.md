# Scraper ingestion contract v1

This defines one shared payload format: Python scrapers produce it, and the Go core validates and reads it. [schema.json](schema.json) is the machine-readable source of truth, using [JSON Schema 2020-12](https://json-schema.org/draft/2020-12/schema). The Go endpoint validates and durably queues submissions. A background worker normalizes submissions and persists historical observations atomically.

## Submit observations

```http
POST /api/v1/ingestion/submissions
Authorization: Bearer <scraper-token>
Content-Type: application/json
```

Use HTTPS outside local development. Credentials are configured independently of the payload. Authenticate each scraper identity and authorize its allowed shops; the body must not be able to impersonate another scraper. Never put credentials in URLs or submitted data.

Send UTF-8 JSON with one registered `shop_id` per submission. Initial limits are **100 observations** and **1 MiB (1,048,576 bytes) of encoded request body**, including whitespace. Both limits apply independently. The worker's batch size is a separate setting. Empty submissions are invalid; split larger runs into bounded submissions before delivery. Request compression is not supported in v1: reject non-identity content encoding with `415`.

All documented object fields are required unless the schema says otherwise. Use explicit `null` for unknown nullable values, not empty strings, zero, or invented values. Unknown fields and duplicate JSON object keys are invalid. Reject non-finite numbers and invalid Unicode. UUIDs are lowercase UUIDv4 strings. Timestamps are valid UTC RFC 3339 values ending in `Z`, with optional fractional seconds of up to six digits; convert source offsets to UTC before submission. The endpoint also rejects U+0000, which PostgreSQL JSONB cannot store, and JSON nesting beyond 64 levels.

See [examples/submission.json](examples/submission.json) for regular and conditional offers,
[examples/unavailable.json](examples/unavailable.json) for an observation with no known price,
and [examples/per-kilogram.json](examples/per-kilogram.json) for a loose product priced per kg,
with null package contents and an explicit fractional purchase minimum.
All shops and products in these examples are fictional.

## Envelope

| Field | Meaning |
| --- | --- |
| `schema_version` | Exact string `"1.0"`. |
| `submission_id` | Scraper-generated UUID, created once per immutable batch. |
| `scraper_id` | Registered producer identity; separate credentials/identities per independent scraper installation. |
| `shop_id` | Registered source retailer or marketplace, not its display name. |
| `run_id` | UUID for a collection run; multiple submissions can belong to one run. |
| `created_at` | Time this batch was assembled; keep unchanged on retries. |
| `collector` | Adapter name and deployed code version, such as a release or commit ID. |
| `observations` | Between 1 and 100 observation objects. |

The core supplies receipt time. A delayed upload keeps its original observation time. An empty batch is not a run-completion signal; this contract does not represent collection coverage or run completion.

## Observations and product identity

Each observation describes one source product/variant, seller/location context, and measurement time. Include all offers seen for that context in its `offers` array; do not create a different observation for each payment method.

- `observation_id`: scraper-generated UUID assigned when a measurement is first collected. Preserve it across retries, spooling, and rebatching. A genuinely new collection gets a new ID, even if its price is unchanged.
- `observed_at`: when the source response or rendered price was actually obtained. Never replace it with retry/upload time or label cached data as newly observed.
- `collection_method`: `api`, `html`, or `browser`, describing how the source data was obtained.
- `source_url`: API/page URL actually fetched. `product_url`: customer-facing product page. Both must be absolute HTTP(S) URLs with a host and no embedded credentials. Remove secret tokens before sending; keep location context in its explicit fields. The core treats URLs as provenance and must not fetch them automatically.
- `product.source_product_id`: shop-scoped identifier of the purchasable variant. Preserve leading zeros. Prefer source SKU/variant IDs; if absent, document a stable canonical-product-URL-based fallback in the adapter. Never use the display name as an ID.
- `product.name` is required; `brand`, `gtin`, and `package` may be null. GTIN is a string preserving leading zeros. Provided GTINs must have 8, 12, 13, or 14 digits and a valid check digit. The schema enforces shape; the worker checks the digit.
- `package.quantity` is the content of one component, `unit` is `g`, `kg`, `ml`, `l`, or `unit`, and `count` is the number of components in the complete sold package. Six 350 ml cans are `{"quantity":"350","unit":"ml","count":6}`. A 5 kg bag is `{"quantity":"5","unit":"kg","count":1}`. An unknown package is null; do not guess a count or size.
- `context` records seller ID, branch/store ID, eight-digit delivery/pickup postal code, city, Brazilian state code, and channel (`delivery`, `pickup`, `online`, or `unknown`). Unknown fields are null. `online` means a known online context without delivery/pickup selection. Missing context does not mean nationwide availability. Avoid personal account information.
- `availability` is `in_stock`, `out_of_stock`, or `unknown`. An empty `offers` array means no price was obtained, not a zero price. An unavailable product can still have displayed offers; retain both facts and let analytics choose which observations qualify.

## Offers and price conditions

`offers` contains up to 20 distinct offers. Each contains:

| Field | Meaning |
| --- | --- |
| `amount` | Displayed total price for one `basis`, as an exact decimal string with two fractional digits, e.g. `"24.90"`. |
| `currency` | `"BRL"` in v1. |
| `basis` | `package` for the complete sold package, or `kg`, `l`, or `unit` for one of that measure. |
| `reference_amount` | Nullable crossed-out/list price for the same basis and conditions; never another independent offer. |
| `conditions` | Explicit payment, installment, membership, coupon, minimum-quantity, and other restrictions. |

Money ranges from `"0.00"` to `"999999999.99"`; JSON numeric prices, exponents, grouping separators, and currency symbols are invalid. A genuine zero price may be recorded; missing prices must not be manufactured as zero. Quantity strings are positive decimals with at most six fractional digits, no leading integer zeros except the zero before a decimal point, and no trailing fractional zeros (`"1"`, `"0.5"`, `"350"`). Use the canonical forms in the schema, not locale-formatted strings.

An offer of `"6.99"` with basis `kg` is R$6.99 per kilogram, not the total price of an unknown package. A package-basis amount applies to the whole package, including every component in `count`. Do not calculate a unit price in the scraper when the source only supplies a package price; the core derives comparable quantities.

Conditions must be explicit:

- `payment_method`: `any`, `pix`, `cash`, `debit_card`, `credit_card`, `boleto`, `other`, or `unknown`. Use `any` only when known to apply across payment methods. Explain `other` in `description`.
- `installments`: number of installments for a credit-card offer, or null. `amount` is the total payable price, never a single installment. Omit an unresolvable installment-only offer and report the extraction issue.
- `membership_required` / `coupon_required`: true, false, or null for unknown. `membership_program` is a name when known, otherwise null; it must be null if membership is explicitly not required.
- `minimum_quantity`: amount purchased to qualify, in the offer's basis, or null if unknown. `"1"` means no multi-quantity minimum. A discount for three packages uses basis `package` and minimum `"3"`; `amount` still means the price per package at that threshold.
- `description`: other restrictions, promotion wording, and details that cannot be expressed in the structured fields, or null.

Do not pick only the cheapest price and discard its restrictions. Unknown conditions remain unknown. Product prices exclude separately charged freight/delivery fees and cart-wide adjustments; preserve an explicit limitation in `description` when necessary. `reference_amount` must be at least `amount` when present; the worker checks this relationship.

## Delivery, identity, and retries

The API validates JSON, schema, request limits, identity authorization, and unique observation IDs within a batch before storing anything. Reject the entire request on failure; there is no partial acceptance. Normalization, GTIN checks, and other domain processing happen in the worker.

After the submission transaction commits, return an empty `202 Accepted` response. The scraper can then mark the batch delivered. This confirms durable storage; background processing is managed internally by the core.

Submission uniqueness is `(scraper_id, submission_id)`. The initial queue deduplicates by identity only; it stores no payload fingerprint and does not compare retried content. Scrapers must keep each submission immutable and reuse its original body on retries. Changed content under an existing identity is ignored, including after raw-payload cleanup; it does not produce `409 Conflict`.

- Existing identity: return an empty `200 OK` after normal authentication and validation. Do not enqueue again, replace stored data, or reset processing state, even when pending, processed, or failed.
- New submission: preserve the complete envelope and queue metadata in the same transaction. Concurrent requests for one identity must resolve through a database uniqueness constraint.

Observation uniqueness is `(scraper_id, observation_id)`, independent of submission ID. The endpoint rejects duplicate observation IDs within one batch. The worker retains identities and fingerprints independently of raw-payload cleanup: identical observations in another batch are safe no-ops, while conflicting content for the same observation identity fails that entire batch internally. Separate scrapers' independent measurements remain separate observations. Corrections require an explicit future correction workflow, not silently replacing an observation with the same ID. Every historical offer requires the scraper identity and scraper-generated observation ID; PostgreSQL does not generate replacement observation IDs.

[fingerprints.json](fingerprints.json) retains reference SHA-256 vectors over [RFC 8785 JCS](https://www.rfc-editor.org/rfc/rfc8785.html): the complete envelope for a submission, and `{"shop_id": <shop_id>, "observation": <complete observation object>}` for an observation. The worker uses the observation fingerprint; submission fingerprints are not stored or checked. Strings are not Unicode-normalized or trimmed; object-key order, whitespace, equivalent integer spellings, and equivalent JSON escape spelling do not matter, while array order and actual field values do.

Send outgoing batches directly to the core. If delivery fails after bounded retries, persist the exact attempted body locally and stop that scraper's collection. Reuse the same body and IDs after network failures, timeouts, or lost responses. A process interruption before fallback persistence can lose the in-memory batch. Retry `429` and transient `5xx` responses with bounded backoff and jitter, honoring `Retry-After`. An ambiguous `5xx` can occur after a commit, so retries must keep their identity. Authentication, validation, and conflict errors require investigation rather than indefinite retries. Delivery uses HTTP status codes; there are no response-body schemas in this contract.

Submission IDs survive successful raw-payload cleanup as internal deduplication records. V1 defines no automatic expiry for those records. An accepted payload can be removed after the configured successful-payload retention period (initially seven days after processing); pending and failed payloads are excluded. Automated cleanup is not implemented yet. Observation identities, run/collector provenance, collection methods, branch/store IDs and coupon conditions survive raw-payload removal. If source replay beyond that period is needed, verify a separate archive before cleanup.

## Compatibility and verification

V1 rejects unknown fields. Changes to accepted fields, enums, units, limits, or interpretation require an explicitly supported new `schema_version`, with coordinated consumers. Typo fixes to documentation that do not change behavior do not require a new version. Route versioning and payload versioning are distinct; the route can support explicitly listed payload versions in future.

Both producers and core must enable JSON Schema `format` validation; shape-only checking is insufficient for dates. Authorization, unique observation IDs within a batch, and URL host/credential checks require application validation at ingestion. The background worker checks GTIN check digits and reference-amount comparisons. Schema validation alone does not implement this API.

Errors use `{"error":"<code>"}`. The endpoint returns `400` for invalid JSON, schema, or application validation; `401` for missing/invalid credentials; `403` for an unauthorized scraper/shop identity; `413` above the byte limit; `415` for unsupported content type, charset, or content encoding; and `500` when persistence fails. Successful `200` and `202` responses have no body.

Run the executable contract checks without installing scraper/browser dependencies:

```sh
python3 -m venv /tmp/inflacao-real-contracts-venv
/tmp/inflacao-real-contracts-venv/bin/pip install -r docs/contracts/ingestion/v1/requirements.txt
/tmp/inflacao-real-contracts-venv/bin/python -B -m unittest discover -s docs/contracts/ingestion/v1 -p 'test_*.py'
```

Checks validate the schema, examples, representative invalid inputs, and canonicalization behavior. Runtime integration tests must additionally cover the API and worker guarantees above when implemented.
