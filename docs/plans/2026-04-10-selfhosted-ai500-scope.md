# Selfhosted AI500 Scope

**Goal:** Add a first-class, self-hosted replacement for the current NofxOS/AI500 signal provider that can be selected in Strategy Studio as `Selfhosted AI500`, runs as a separate Docker service, and exposes a compatible HTTP API for the current strategy engine.

**Primary outcome:** A running trader should be able to use:

- candidate coin selection
- quant coin detail
- OI ranking
- netflow ranking
- price ranking

without paying per request through `claw402`.

## Why this exists

Current live behavior:

- `AI500` candidate selection works again only because NofxOS requests are routed through `claw402`
- `claw402` charges per request
- the old shared public NofxOS key is deprecated

That means the current strategy path is operational, but not cost-stable for self-hosting.

## Hard constraint

This service must not be a toy `GET /api/ai500/list` shim.

For the existing NOFX strategy engine, a practical AI500 replacement must cover the full set of NofxOS-derived endpoints that the current trading loop depends on.

Relevant integration points:

- `provider/nofxos/ai500.go`
- `provider/nofxos/coin.go`
- `provider/nofxos/oi.go`
- `provider/nofxos/netflow.go`
- `provider/nofxos/price.go`
- `kernel/engine.go`
- `trader/auto_trader_loop.go`

## Scope decision

### In scope

- separate Dockerized service named `selfhosted-ai500`
- strategy-selectable provider named `Selfhosted AI500`
- HTTP API compatible with the NofxOS endpoints used by NOFX
- local scoring pipeline for candidate selection
- persistent local cache/history
- no dependency on `claw402` for market/signal data

### Out of scope

- exact reproduction of proprietary NofxOS scoring internals
- exact reproduction of true institution/personal flow labeling unless a paid upstream data source is added
- replacing OpenAI/Claude/DeepSeek model providers

## Product definition

`Selfhosted AI500` is a signal provider, not just a coin list.

In the strategy UI it should be selectable as a provider for all NofxOS-backed signal features, while existing source types remain available:

- `ai500`
- `oi_top`
- `oi_low`
- `mixed`

When `Selfhosted AI500` is selected, those source types and the downstream quant/ranking enrichments resolve against the self-hosted service instead of `nofxos.ai` / `claw402`.

## Preferred integration design

### Strategy model

Add a new strategy-level provider block instead of overloading `nofxos_api_key`.

Proposed config shape:

```json
{
  "signal_provider": {
    "type": "nofxos",
    "base_url": "https://nofxos.ai",
    "api_key": "..."
  }
}
```

Supported values:

- `nofxos`
- `selfhosted_ai500`

For self-hosted mode:

```json
{
  "signal_provider": {
    "type": "selfhosted_ai500",
    "base_url": "http://selfhosted-ai500:8081",
    "api_key": "local-token"
  }
}
```

### Why this design

This is the cleanest approach because the current NOFX engine uses the same provider family for:

- coin candidates
- coin detail
- OI ranking
- netflow ranking
- price ranking

If only `coin_source.source_type` were extended with `selfhosted_ai500`, the rest of the signal path would still be split and confusing.

## Backward compatibility

The current field `indicators.nofxos_api_key` should remain readable during migration.

Migration rule:

- if `signal_provider` exists, use it
- else fall back to current behavior based on `nofxos_api_key`

This keeps old strategies valid.

Migration examples:

```json
{
  "signal_provider": {
    "type": "nofxos",
    "api_key": "private-provider-key"
  }
}
```

```json
{
  "signal_provider": {
    "type": "selfhosted_ai500",
    "base_url": "http://selfhosted-ai500:8081",
    "api_key": "local-token"
  }
}
```

```json
{
  "indicators": {
    "nofxos_api_key": "legacy-key-still-readable-during-migration"
  }
}
```

Notes:

- legacy `official_nofxos` may still be read for backward compatibility
- newly saved configs should use canonical `nofxos`
- `selfhosted_ai500` requires `base_url`

## Required NOFX changes

### Backend

- `store/strategy.go`
  - add `SignalProviderConfig`
  - preserve old `nofxos_api_key` as compatibility input
- `kernel/engine.go`
  - stop hardcoding `nofxos.DefaultBaseURL`
  - create provider client from strategy config
- `api/strategy.go`
  - validate `signal_provider`
- `api/server.go`
  - update generated API docs and examples

### Frontend

- `web/src/types/strategy.ts`
  - add `signal_provider`
- `web/src/components/strategy/CoinSourceEditor.tsx`
  - keep source-type selector as-is
- `web/src/components/strategy/IndicatorEditor.tsx`
  - add provider selector
  - add base URL / token fields for self-hosted mode
- `web/src/components/trader/TraderConfigModal.tsx`
  - surface selected provider in strategy summary

### Docker

- `docker-compose.yml`
  - add `selfhosted-ai500` service
- optionally `docker-compose.prod.yml`
  - add optional service block behind a profile
- `.env.example`
  - add provider service config

## API surface required for parity

### P0 required compatibility

These endpoints are required for current strategy execution.

#### AI500 endpoints

- `GET /api/ai500/list`
- `GET /api/ai500/:symbol`
- `GET /api/ai500/stats`

#### Quant endpoints

- `GET /api/coin/:symbol`

Supported `include` values:

- `netflow`
- `oi`
- `price`
- `ai500`

#### Ranking endpoints

- `GET /api/oi/top-ranking`
- `GET /api/oi/low-ranking`
- `GET /api/netflow/top-ranking`
- `GET /api/netflow/low-ranking`
- `GET /api/price/ranking`

### P1 optional compatibility

These are not currently required by the live strategy loop, but matter if we want a fuller NofxOS replacement:

- `GET /api/ai300/list`
- `GET /api/ai300/stats`
- `GET /api/funding-rate/top`
- `GET /api/funding-rate/low`
- `GET /api/funding-rate/:symbol`
- `GET /api/long-short/list`
- `GET /api/long-short/:symbol`
- `GET /api/oi-cap/ranking`
- `GET /api/heatmap/*`

## Response compatibility requirement

The service should be wire-compatible with the shapes expected by the current NOFX parsers.

That means:

- top-level `success`
- same `data` field shapes
- same field names currently parsed by:
  - `provider/nofxos/ai500.go`
  - `provider/nofxos/coin.go`
  - `provider/nofxos/oi.go`
  - `provider/nofxos/netflow.go`
  - `provider/nofxos/price.go`

Avoid inventing a new schema in phase 1.

## Separate service architecture

### Service name

- Docker service: `selfhosted-ai500`
- repo path: `services/selfhosted-ai500/`

### Recommended implementation language

- Go

Reason:

- current NOFX backend is already Go
- the expected wire contracts are already defined in Go structs
- easier reuse of symbol normalization and testing style

### Internal modules

```text
services/selfhosted-ai500/
  cmd/server/
  internal/api/
  internal/config/
  internal/storage/
  internal/collectors/
  internal/scoring/
  internal/rankings/
  internal/cache/
  internal/scheduler/
  Dockerfile
```

### Service responsibilities

#### 1. Universe builder

Builds the tradable coin universe from supported exchanges.

Initial exchanges:

- Hyperliquid
- Binance futures
- Bybit futures

Optional later:

- OKX

#### 2. Market data collector

Collects and stores:

- price klines
- volume
- OI
- funding
- basis where available
- taker buy/sell volume if available

#### 3. Score engine

Produces an `AI500`-style score per symbol, `0-100`.

#### 4. Ranking engine

Produces:

- OI increase/decrease rankings
- price gainers/losers
- netflow proxy rankings

#### 5. API layer

Serves NofxOS-compatible responses from cached score snapshots and aggregates.

## Data storage

### P0 recommendation

- SQLite persisted volume
- local write-ahead logging enabled
- in-memory cache for hot responses

Why:

- lowest operational burden
- good enough for one self-hosted node
- no extra database container required in the first milestone

### P1 recommendation

If symbol count or refresh pressure becomes high:

- add Redis for hot cache and job coordination
- keep SQLite or move to Postgres for longer history

## Scoring model

This needs to be explicit. Otherwise the service degenerates into random coin ranking.

### Candidate score families

Proposed components:

- `s1_liquidity`
  - exchange volume
  - OI notional
  - spread sanity
- `s2_oi_expansion`
  - OI delta percent
  - OI delta notional
  - persistence across windows
- `s3_price_confirmation`
  - price above start regime
  - short/mid timeframe trend alignment
- `s4_flow_confirmation`
  - taker imbalance
  - buy/sell pressure proxies
  - funding/basis alignment
- `s5_relative_strength`
  - symbol return vs BTC
  - symbol return vs market basket
- `s6_regime_quality`
  - avoid dead chop
  - avoid extreme exhaustion
- `s7_risk_penalty`
  - unstable listing
  - extreme funding
  - thin book
  - invalid exchange venue

### Score output

Per symbol:

- `score`
- `last_score`
- `max_score`
- `start_time`
- `start_price`
- `max_price`
- `increase_percent`
- `reason_codes`

### Important honesty constraint

The current NofxOS `institution` vs `personal` netflow is likely not reproducible exactly from public exchange APIs alone.

So the self-hosted service must support two possible modes:

#### Mode A: proxy netflow

Derived from public market structure:

- taker buy/sell imbalance
- OI delta
- price response
- liquidation skew
- funding/basis pressure

This is implementable and cheap, but only approximate.

#### Mode B: premium data mode

If a separate paid data source is later added, we can populate:

- institution flow
- personal flow

with better semantics.

The API stays the same in both modes.

## Endpoint behavior details

### `GET /api/ai500/list`

Purpose:

- feed `kernel.getAI500Coins()`

Must return:

- active symbols sorted by score descending
- response shape compatible with `provider/nofxos/ai500.go`

Selection policy:

- score threshold configurable, default `70`
- liquidity threshold configurable
- recent trend confirmation required
- blacklist invalid venues

### `GET /api/ai500/:symbol`

Purpose:

- detail page parity
- debugging score composition

Should include:

- current score
- current rank
- score components
- current price
- start regime info

### `GET /api/ai500/stats`

Purpose:

- overview / monitoring

Should include:

- active count
- median score
- top score
- score distribution buckets
- refresh timestamp

### `GET /api/coin/:symbol`

Purpose:

- feeds `FetchQuantData`

Must cover:

- `price_change`
- `oi`
- `netflow`
- optional `ai500`

Notes:

- exact field naming must remain compatible
- unknown symbols return a clean structured response, not panics

### `GET /api/oi/top-ranking` and `low-ranking`

Purpose:

- feeds `FetchOIRankingData`

Must provide:

- rank
- symbol
- price
- current OI
- delta
- delta percent
- delta value
- price delta percent

### `GET /api/netflow/top-ranking` and `low-ranking`

Purpose:

- feeds `FetchNetFlowRankingData`

Must support query params:

- `duration`
- `limit`
- `type`
- `trade`

In P0, `type=institution|personal` may be backed by proxy buckets if premium attribution is unavailable.

### `GET /api/price/ranking`

Purpose:

- feeds `FetchPriceRankingData`

Must support:

- multi-duration requests like `1h,4h,24h`

## Refresh model

### Target refresh cadence

- AI500 score snapshots: every `60s`
- OI ranking: every `60s`
- price ranking: every `60s`
- netflow proxy ranking: every `60s`
- coin detail aggregates: on schedule plus cache-on-read

### Cache strategy

- precompute rankings on a schedule
- serve API from hot cache
- avoid recalculating scores in request path

## Docker deployment scope

### Compose service

Example target shape:

```yaml
services:
  selfhosted-ai500:
    build:
      context: .
      dockerfile: ./services/selfhosted-ai500/Dockerfile
    container_name: selfhosted-ai500
    restart: unless-stopped
    ports:
      - "8081:8081"
    env_file:
      - .env
    volumes:
      - ./data/selfhosted-ai500:/app/data
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8081/health"]
```

### Service env

Initial env set:

- `SELFHOSTED_AI500_PORT=8081`
- `SELFHOSTED_AI500_AUTH_TOKEN=...`
- `SELFHOSTED_AI500_DB_PATH=/app/data/selfhosted-ai500.db`
- `SELFHOSTED_AI500_REFRESH_SECS=60`
- `SELFHOSTED_AI500_EXCHANGES=hyperliquid,binance,bybit`
- `SELFHOSTED_AI500_SCORE_THRESHOLD=70`

## Strategy Studio UX scope

### New provider selection

Add a provider selector in Strategy Studio:

- `NOFXOS`
- `Selfhosted AI500`

When `Selfhosted AI500` is selected:

- show `base_url`
- show `auth_token`
- hide `nofxos_api_key`

### Behavior

- existing `ai500`, `oi_top`, `oi_low`, `mixed` options remain
- the selected provider defines where those signals are pulled from

This is lower risk than inventing a parallel source-type tree.

## Acceptance criteria

### Functional

- strategy can select `Selfhosted AI500`
- trader `2026` can run without `claw402-data` payments
- `GET /api/ai500/list` returns candidates from the self-hosted service
- OI ranking, netflow ranking, and price ranking return `200`
- a full decision cycle completes with:
  - non-empty candidate list
  - no deprecated-key error
  - no `claw402-data` payment logs

### Compatibility

- old strategies without `signal_provider` continue to run
- current NOFX provider parsers do not need endpoint-specific forks in phase 1

### Operational

- Docker service survives restart
- score cache rebuilds from persisted data
- health endpoint reflects dependency readiness

## Implementation phases

### Phase 1: provider plumbing

- add `signal_provider` config
- allow configurable base URL/token
- wire strategy engine to custom provider endpoint
- add UI selector

### Phase 2: selfhosted-ai500 service MVP

- implement:
  - `/health`
  - `/api/ai500/list`
  - `/api/ai500/:symbol`
  - `/api/ai500/stats`
  - `/api/oi/top-ranking`
  - `/api/oi/low-ranking`
  - `/api/price/ranking`
  - `/api/coin/:symbol` with `price` + `oi` + `ai500`

### Phase 3: quant parity

- add `netflow` proxy generation
- fill `/api/netflow/*`
- fill `/api/coin/:symbol?include=netflow,...`

### Phase 4: calibration and observability

- score audits
- response snapshots
- comparison mode against current NofxOS/claw402 output
- drift dashboards

## Effort estimate

### Phase 1

- `1 to 2` days

### Phase 2

- `4 to 6` days

### Phase 3

- `4 to 7` days

### Phase 4

- `2 to 4` days

### Total

- realistic initial full scope: `11 to 19` working days

This is a real subsystem, not a one-file patch.

## Main risks

### Risk 1: fake parity

A service that only returns symbols but not credible downstream quant data will make the AI worse, not better.

### Risk 2: netflow semantics

Institution/personal flow is the weakest part of pure self-hosting. We should treat this as a modeled proxy unless a premium source is explicitly added.

### Risk 3: symbol coverage

Some AI500 candidates are obscure tickers. The service must handle:

- unsupported venue symbols
- newly listed coins
- alias normalization

### Risk 4: request-path recomputation

If ranking is computed in the HTTP handler, latency and instability will be unacceptable. Precompute first, serve cached snapshots.

## Explicit recommendation

Build this as a provider-compatible service with a clean strategy-level provider switch.

Do not:

- hardcode more fallback keys
- keep coupling the feature to `claw402`
- create a one-off `selfhosted_ai500` coin list without the ranking and quant endpoints

## First implementation target

The smallest version worth shipping is:

- provider selection in strategy
- separate `selfhosted-ai500` Docker service
- working `/api/ai500/list`
- working `/api/oi/*`
- working `/api/price/ranking`
- working `/api/coin/:symbol` for `price`, `oi`, `ai500`
- temporary proxy `netflow` with explicit labeling in code/docs

That is the first version that can replace paid `claw402-data` for strategy execution without pretending to solve more than it does.
