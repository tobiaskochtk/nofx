# Selfhosted AI500 Implementation Plan

Related scope:

- `docs/plans/2026-04-10-selfhosted-ai500-scope.md`

## Goal

Implement `Selfhosted AI500` as a selectable signal provider in NOFX and back it with a separate Docker service that replaces the current paid NofxOS data path for:

- candidate selection
- quant coin detail
- OI ranking
- netflow ranking
- price ranking

## Delivery strategy

Use a staged rollout. Do not build the full service first and only then wire it into NOFX.

Recommended order:

1. provider plumbing in NOFX
2. service scaffold and health
3. AI500 list + OI + price ranking MVP
4. coin detail endpoint
5. netflow proxy
6. parity validation against current live behavior

## Phase 1: NOFX provider plumbing

### Outcome

NOFX can target either:

- official `nofxos`
- `selfhosted_ai500`

without changing the rest of the strategy logic.

### Files

#### `store/strategy.go`

Tasks:

- add `SignalProviderConfig`
- add it to `StrategyConfig`
- keep `Indicators.NofxOSAPIKey` readable for backward compatibility
- add default config generator values
- add validation rules:
  - provider type required when present
  - `base_url` required for `selfhosted_ai500`
  - `api_key` optional but supported

Suggested shape:

```go
type SignalProviderConfig struct {
    Type    string `json:"type,omitempty"`
    BaseURL string `json:"base_url,omitempty"`
    APIKey  string `json:"api_key,omitempty"`
}
```

Tests:

- legacy strategy without `signal_provider` still parses
- new strategy with `selfhosted_ai500` parses
- token estimate tests still pass

#### `web/src/types/strategy.ts`

Tasks:

- add `signal_provider` type
- keep old `nofxos_api_key` field for compatibility

Tests:

- TypeScript compile

#### `api/strategy.go`

Tasks:

- accept and validate `signal_provider`
- keep old request payloads valid

Tests:

- create strategy with no provider block
- create strategy with selfhosted provider block

#### `kernel/engine.go`

Tasks:

- stop constructing provider client with hardcoded `nofxos.DefaultBaseURL`
- add client factory logic:
  - if `signal_provider.type == selfhosted_ai500`, use its `base_url` and `api_key`
  - else use current behavior
- keep `claw402` routing logic intact for the official provider path

Implementation note:

- do not branch endpoint-by-endpoint
- branch once at client construction time

Tests:

- strategy engine uses custom base URL
- legacy path still uses old behavior
- `claw402` path still initializes for official provider

#### `provider/nofxos/client.go`

Tasks:

- no schema change required
- ensure custom `baseURL` and `authKey` work cleanly for non-NofxOS hosts
- keep `doRequest()` generic

Tests:

- query auth appended correctly
- no duplicate `auth` param

#### `api/server.go`

Tasks:

- update API docs examples
- remove the misleading statement that `nofxos_api_key` is always the deprecated public key

### Frontend UI

#### `web/src/components/strategy/IndicatorEditor.tsx`

Tasks:

- add provider mode selector:
  - `Official NOFXOS`
  - `Selfhosted AI500`
- when selfhosted is selected:
  - show base URL
  - show auth token
  - hide deprecated default-key helper button
- when official is selected:
  - preserve current API-key path

Tests:

- strategy save/load roundtrip in UI

#### `web/src/components/trader/TraderConfigModal.tsx`

Tasks:

- show chosen signal provider in summary

### Phase 1 gate

Ship only when:

- `go build ./...` passes
- `npm run build` passes
- a strategy can be saved with `selfhosted_ai500`
- backend requests are pointed at a configurable base URL

## Phase 2: selfhosted-ai500 service scaffold

### Outcome

Separate service runs in Docker and exposes:

- `/health`

with config and persistence ready.

### New files

#### `services/selfhosted-ai500/go.mod`

Tasks:

- isolated module or workspace-compatible package setup

#### `services/selfhosted-ai500/cmd/server/main.go`

Tasks:

- load config
- initialize store
- start HTTP server

#### `services/selfhosted-ai500/internal/config/config.go`

Tasks:

- port
- auth token
- db path
- refresh interval
- enabled exchanges
- score threshold

#### `services/selfhosted-ai500/internal/api/router.go`

Tasks:

- register routes
- auth middleware

#### `services/selfhosted-ai500/internal/api/health.go`

Tasks:

- return service readiness

#### `services/selfhosted-ai500/internal/storage/sqlite.go`

Tasks:

- initialize sqlite
- create tables or apply schema

#### `services/selfhosted-ai500/Dockerfile`

Tasks:

- build image
- expose service port

#### `docker-compose.yml`

Tasks:

- add `selfhosted-ai500` service
- mount `./data/selfhosted-ai500`
- join `nofx-network`

#### `.env.example`

Tasks:

- add selfhosted service variables

### Phase 2 gate

Ship only when:

- `docker compose up -d selfhosted-ai500` starts
- `GET /health` returns `200`
- service persists its local DB across restart

## Phase 3: AI500 list MVP

### Outcome

NOFX can fetch candidate coins from the self-hosted service.

### New files

#### `services/selfhosted-ai500/internal/collectors/universe.go`

Tasks:

- build coin universe
- normalize symbols to `XXXUSDT`
- include exchange availability metadata

#### `services/selfhosted-ai500/internal/collectors/price.go`

Tasks:

- fetch recent price data
- store returns by timeframe

#### `services/selfhosted-ai500/internal/collectors/oi.go`

Tasks:

- fetch OI current and delta windows

#### `services/selfhosted-ai500/internal/scoring/ai500.go`

Tasks:

- compute `score`
- compute active candidate list
- compute:
  - `start_time`
  - `start_price`
  - `last_score`
  - `max_score`
  - `max_price`
  - `increase_percent`

#### `services/selfhosted-ai500/internal/api/ai500.go`

Tasks:

- implement:
  - `GET /api/ai500/list`
  - `GET /api/ai500/:symbol`
  - `GET /api/ai500/stats`

### Response contract

Must match the current parser expectations in:

- `provider/nofxos/ai500.go`

### Validation task

Point a local strategy at `selfhosted-ai500` and verify:

- `candidate coins > 0`
- no `claw402-data` payment logs

### Phase 3 gate

Ship only when:

- `2026`-equivalent strategy can complete candidate selection
- `No candidate coins available` is gone

## Phase 4: OI ranking and price ranking

### Outcome

Market-wide ranking enrichments no longer depend on NofxOS/claw402.

### New files

#### `services/selfhosted-ai500/internal/rankings/oi.go`

Tasks:

- compute top/low OI rankings
- support durations:
  - `1h`
  - `4h`
  - `24h`

#### `services/selfhosted-ai500/internal/rankings/price.go`

Tasks:

- compute gainers/losers by duration
- support comma-separated durations

#### `services/selfhosted-ai500/internal/api/oi.go`

Tasks:

- implement:
  - `GET /api/oi/top-ranking`
  - `GET /api/oi/low-ranking`

#### `services/selfhosted-ai500/internal/api/price.go`

Tasks:

- implement:
  - `GET /api/price/ranking`

### Validation task

Verify the current strategy engine methods work unmodified:

- `FetchOIRankingData()`
- `FetchPriceRankingData()`

### Phase 4 gate

Ship only when:

- `OI ranking data ready`
- `Price ranking data ready`

appear in trader logs with the self-hosted provider.

## Phase 5: coin detail endpoint

### Outcome

`FetchQuantDataBatch()` works against the self-hosted service.

### New files

#### `services/selfhosted-ai500/internal/aggregates/coin.go`

Tasks:

- aggregate per-symbol:
  - `price_change`
  - `oi`
  - optional `ai500`
  - placeholder/proxy `netflow`

#### `services/selfhosted-ai500/internal/api/coin.go`

Tasks:

- implement:
  - `GET /api/coin/:symbol`
- support `include=...`

### Key compatibility requirement

Response must satisfy parsers in:

- `provider/nofxos/coin.go`

### Validation task

Run a trader cycle and confirm:

- quant detail fetches return non-zero symbols
- no per-symbol `404` for valid candidates

### Phase 5 gate

Ship only when:

- `Successfully fetched quantitative data for N symbols` is non-zero in the live cycle

## Phase 6: netflow proxy

### Outcome

Netflow-dependent prompt context no longer depends on NofxOS/claw402.

### Important constraint

This is a proxy model unless a premium source is added.

### New files

#### `services/selfhosted-ai500/internal/rankings/netflow.go`

Tasks:

- compute proxy inflow/outflow ranking from:
  - taker imbalance
  - OI delta
  - price impact
  - liquidation skew
  - funding/basis context when available

#### `services/selfhosted-ai500/internal/api/netflow.go`

Tasks:

- implement:
  - `GET /api/netflow/top-ranking`
  - `GET /api/netflow/low-ranking`

### Documentation requirement

Add explicit note in service README:

- `institution` and `personal` are modeled buckets in self-hosted mode unless a premium attribution source is configured

### Phase 6 gate

Ship only when:

- `NetFlow ranking data ready`

appears in live logs without `claw402-data` usage.

## Phase 7: observability and parity tools

### Outcome

We can compare self-hosted output to the current official path before switching live traders permanently.

### New files

#### `services/selfhosted-ai500/internal/api/debug.go`

Tasks:

- debug endpoint for score breakdown
- debug endpoint for ranking snapshots

#### `scripts/compare_signal_provider.go`

Tasks:

- compare official vs self-hosted responses for:
  - ai500 list
  - OI ranking
  - price ranking
  - coin detail

### Validation task

Run side-by-side comparisons on a sample universe and capture:

- overlap rate
- rank correlation
- missing-symbol rate

## Testing plan

### NOFX repo tests

#### Backend

- `go build ./...`
- targeted tests for:
  - `store`
  - `kernel`
  - `api`

#### Frontend

- `npm run build`

### Service tests

#### Unit tests

- symbol normalization
- score calculation
- ranking sort order
- duration parsing
- auth middleware

#### Integration tests

- endpoint contract tests against expected JSON shapes
- sqlite persistence tests
- scheduled refresh tests

#### End-to-end tests

- NOFX strategy pointed at local `selfhosted-ai500`
- full cycle completes
- no `claw402-data` payment logs

## Rollout plan

### Step 1

Implement Phase 1 only and keep all traders on current provider.

### Step 2

Bring up `selfhosted-ai500` locally and verify endpoint responses manually.

### Step 3

Create one duplicate test strategy:

- source type `ai500`
- provider `selfhosted_ai500`

Do not switch the main trader first.

### Step 4

Observe at least:

- 10 cycles
- candidate stability
- missing quant symbol count
- decision quality

### Step 5

Only then switch `2026`.

## Work breakdown estimate

### Phase 1

- `1 to 2` days

### Phase 2

- `1 to 2` days

### Phase 3

- `2 to 4` days

### Phase 4

- `2 to 3` days

### Phase 5

- `2 to 4` days

### Phase 6

- `3 to 5` days

### Phase 7

- `1 to 2` days

## Recommended first coding slice

Start with this exact slice:

1. `store/strategy.go`
2. `web/src/types/strategy.ts`
3. `api/strategy.go`
4. `kernel/engine.go`
5. `web/src/components/strategy/IndicatorEditor.tsx`
6. `services/selfhosted-ai500` scaffold
7. `/api/ai500/list`
8. `/api/oi/*`
9. `/api/price/ranking`

That gives us the cheapest path to remove paid `claw402-data` usage from candidate selection and ranking context before tackling full per-symbol quant parity.
