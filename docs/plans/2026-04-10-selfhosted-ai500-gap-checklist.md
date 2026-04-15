# Selfhosted AI500 Gap Checklist

Source documents:

- `docs/plans/2026-04-10-selfhosted-ai500-implementation-plan.md`
- `docs/plans/2026-04-10-selfhosted-ai500-scope.md`

Status legend:

- `[x]` done
- `[ ]` open

## P0 Contract And Runtime Correctness

- [x] Canonicalize the signal provider contract to `nofxos | selfhosted_ai500` while keeping backward compatibility for legacy `official_nofxos`
- [x] Validate `signal_provider` on strategy create/update and reject invalid provider types or selfhosted configs without `base_url`
- [x] Update API docs so they no longer claim `indicators.nofxos_api_key` is always the deprecated public key
- [x] Add `include=ai500` support to `GET /api/coin/:symbol`
- [x] Expand `GET /api/ai500/:symbol` to include the scope-required detail payload: current price, score components, start-regime info
- [x] Expand `GET /api/ai500/stats` with score distribution buckets and monitoring fields

## P1 Service Behavior And Compatibility

- [x] Add full query compatibility for netflow ranking, including explicit `trade` handling instead of hardcoding `future`
- [x] Precompute and cache ranking payloads instead of sorting them on every request
- [x] Add structured handling for unknown symbols and parity-oriented response details where the scope expects richer metadata

## P1 Product Surface And UX

- [x] Show the selected signal provider in the trader strategy summary UI
- [x] Normalize frontend provider values to the canonical contract everywhere and remove remaining legacy-only assumptions
- [x] Add explicit provider contract examples to strategy-related API examples and migration notes
- [x] Preserve `selection_bucket` metadata when AI500 candidates flow through mixed-source strategies
- [x] Expose explicit Bollinger band values in the compact non-grid decision payload instead of only derived volatility states
- [x] Make the compact RSI context strategy-aware so the configured RSI period is visible to the AI runtime
- [x] Surface the custom `f4`-`f7` signal blocks as selectable strategy options in the UI while keeping legacy strategies effectively enabled by default

## P2 Observability, Calibration, And Test Depth

- [x] Add debug endpoints for score breakdown and ranking snapshots
- [x] Add a reusable `scripts/compare_signal_provider.go` parity tool
- [x] Add adaptive thresholding and a bounded exploration bucket so thin candidate pools can widen without admitting hard-risk or low-depth names
- [x] Add bucket-level live logging so 12-24h runtime observation can distinguish `primary`, `adaptive`, and `exploration` picks
- [x] Add a 12-24h review template for bucket-usage evaluation and follow-up tuning
- [x] Expose a per-trader 24h AI500 bucket review in the dashboard UI, backed by persisted decision-record telemetry instead of a separate batch job
- [x] Add service-level unit tests for scoring, duration parsing, auth middleware, and ranking order
- [x] Add service-level integration tests for endpoint contracts and sqlite persistence
- [x] Fix the grid runtime indicator path so EMA20/EMA50 come from a coherent timeframe series instead of a mixed short/long fallback

## P2 Data Breadth

- [x] Decide whether to keep Hyperliquid-only as the intended P0 implementation or add the exchange-configurable universe described in the scope
- [x] If multi-exchange remains in scope, add config and collectors for the additional venues instead of the current Hyperliquid-only pipeline

## P3 Optional Scope Backlog

- [ ] Add the optional parity endpoints from the full scope document: `/api/ai300/*`, `/api/funding-rate/*`, `/api/long-short/*`, `/api/oi-cap/ranking`, `/api/heatmap/*`
- [ ] Add automated scheduled refresh tests for the selfhosted service loop
- [ ] Add end-to-end trader-cycle verification against a local `selfhosted-ai500` instance
