# Web interface — React

## Scope and responsibilities

The React application makes price histories and inflation insights easy to explore for nontechnical users. Retrieve application data through the Go core API.

The core owns authoritative calculations and persistence. Keep presentation, navigation, filters, and interaction behavior here.

## Implementation patterns

- Use functional components and hooks. Keep components focused on a clear user task and extract reusable behavior when reuse or complexity warrants it.
- Group feature-specific components, hooks, and helpers together. Move elements into shared modules when they serve multiple features.
- Centralize core API access and error handling. Keep transport response mapping out of display components and follow the documented API contract.
- Keep local interaction state close to the component that owns it. Distinguish fetched data from local UI state; introduce global state only for concrete shared needs.
- Use URL parameters for shareable filters such as product, shop, and date range when those features are introduced.
- Reserve effects for synchronization with external systems. Derive display values directly from existing state when possible.
- Reuse established design and tooling conventions as the application develops. The React framework, build tooling, and component library have not yet been selected.

## User experience

- Use Brazilian Portuguese for user-facing content and Brazilian conventions for currency, numbers, and dates.
- Use locale-aware formatters and explicit timezone handling. Preserve monetary precision through the API and formatting boundary.
- Build responsive layouts using semantic HTML, labeled controls, keyboard navigation, and visible focus states.
- Make price histories, shop comparisons, and period selection understandable without technical knowledge.
- Provide clear loading, empty, stale-data, insufficient-data, and error states. Offer recovery where an action can resolve the issue.
- Explain metrics in plain language and display units, selected periods, collection freshness, and relevant coverage or methodology context.
- Distinguish a product's price change from an aggregate inflation metric. Explain limitations supplied by the API rather than inferring broader conclusions.
- Keep charts readable, label axes and units, and provide accessible text or tabular alternatives. Do not rely on color alone to communicate increases and decreases.
- Do not reconstruct missing metrics or treat absent observations as unchanged prices in the browser.

## Verification

Test user-visible behavior at meaningful boundaries: selecting filters, displaying returned data, and handling empty or failed requests. Use representative API fixtures that follow the shared contract.

Check Brazilian formatting, responsive layouts, keyboard access, and chart alternatives for affected views. Use end-to-end checks for essential journeys when those journeys exist, and keep routine tests independent of live shop data.

Document setup, development, build, lint, type-check, and test commands as the corresponding tooling is introduced. Run the checks appropriate to each change; do not claim commands exist before the application is configured.
