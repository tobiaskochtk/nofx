# Bybit Paper Trading Scope

Date: 2026-04-20
Owner: Codex draft for implementation
Status: In progress, foundation slice implemented

## Progress Update

- [x] Foundation slice implemented for explicit `live` / `testnet` / `paper` exchange environments.
- [x] Paper exchange accounts can be created from the UI with starting capital, asset, fee, slippage, funding, and liquidation settings.
- [x] Trader creation/update now preserves paper starting capital and skips live balance probing for paper accounts.
- [x] Paper accounts now have a durable wallet state plus ledger seed/config-sync events.
- [x] Account-state endpoints now read paper wallet balances instead of static config only.
- [x] Runtime no longer silently falls back to a live Bybit client for paper accounts; it fails explicitly until the paper adapter exists.
- [ ] True paper trader execution adapter is still pending.
- [ ] Paper order/fill/position execution, fee/funding/liquidation accounting, and review provenance are still pending.

## Goal

Implement a true Bybit-focused paper-trading mode that behaves like a normal trader from the strategy/runtime perspective, but never sends real orders to an exchange.

The user must be able to:

- create a paper wallet for Bybit from the UI
- configure any starting capital for that wallet
- attach a normal trader to that paper wallet
- let the trader run through the normal decision cycles
- open and close paper positions with realistic fees and PnL tracking
- feed those resulting deals into deal review, learned patterns, optimizer evidence, and semantic memory

This scope is about real internal paper execution, not just "some testnet switch".

## Why This Needs Its Own Scope

The currently shipped `paper / simulation` wording is not a real simulator.

Current state after code audit:

- [x] Challenger `paper` mode currently requires a `testnet` exchange account.
- [x] There is no generic standalone paper-trader type in the normal trader creation flow.
- [x] Trader creation currently probes the selected exchange and can overwrite the user-provided initial balance with the fetched live balance.
- [x] Existing review / pattern / optimizer systems already consume structured orders, fills, positions, decision snapshots, and review cases, so they can be reused if paper execution writes into the same storage model.

That means the current system is not enough for the requested outcome:

- arbitrary start capital
- no real execution
- same strategy behavior
- same downstream review learning

## Requirements Confirmed From User Request

- [x] Bybit is the first-class target venue.
- [x] Starting capital must be freely configurable.
- [x] The trader should behave like a normal trader, except that execution is virtual.
- [x] Fees and resulting win/loss must be tracked.
- [x] Resulting deals should feed the lessons-learned / pattern-learning / optimizer knowledge stack.
- [x] The system should preserve real strategy context, tags, reasons, and reviewability.

## Core Design Decision

- [x] V1 should be a true internal paper-execution system, not only a Bybit testnet integration.
- [x] Bybit remains the market-data / venue-behavior source for paper execution.
- [x] Real Bybit testnet support can be added later as an optional validation lane, but it is not sufficient for the main requirement because arbitrary virtual capital is a hard requirement.

Rationale:

- a real Bybit testnet account still has exchange-owned balances and exchange-side constraints
- a true paper wallet can be seeded with any capital, reset, cloned, and used safely by the optimizer for challenger experiments
- internal paper execution can produce cleaner exit evidence than the current synced-close inference path, because the system itself knows exactly which stop / take-profit / trailing / AI close caused the exit

## Product Outcome

After this scope, a user should be able to:

1. Create a `Bybit Paper` wallet in the UI with e.g. `10000 USDT`.
2. Create a normal AI trader that uses that wallet.
3. Start it without API keys and without sending any authenticated trade requests.
4. See balance, equity, open positions, closed deals, fees, and realized PnL as if it were a normal trader.
5. Review those deals in `/deal-review`, including reasoning snapshots, price path, exit reason, and quality metrics.
6. Use those paper deals in Pattern Lab and optimizer datasets, with explicit provenance so paper and live are distinguishable.
7. Reuse the same infrastructure later for true optimizer challenger paper runs.

## Non-Goals For V1

To keep the first version practical, these are explicitly out of scope unless promoted later:

- full exchange-grade order book matching
- partial-fill microstructure simulation
- exact venue liquidation engine parity with every Bybit edge case
- full grid-trading / advanced limit-order strategy parity on day one
- cross-venue paper execution beyond Bybit in the initial slice

V1 should target the currently important directional AI trader path:

- market entries
- market exits
- local stop loss / take profit
- trailing stop
- fee / funding / margin accounting

## Architecture Principles

- Reuse the existing trader runtime, decision cycle, order store, fill store, position store, and deal-review pipeline wherever possible.
- Make paper execution a first-class execution environment, not a hidden special case.
- Persist immutable provenance so future review can always tell whether a deal was `live`, `testnet`, or `paper`.
- Keep paper wallets resettable and clonable for optimizer challenger workflows.
- Never allow a paper wallet configuration to send authenticated trading requests.

## Scope Sections

## A. Execution Environment Model

### Goal

Represent `live`, `testnet`, and `paper` explicitly across the system.

### Draft Scope

- [x] Add an explicit execution environment on exchange accounts, such as:
  - `live`
  - `testnet`
  - `paper`
- [x] Add source-venue metadata for paper accounts so V1 can declare:
  - execution environment = `paper`
  - source venue = `bybit`
- [x] Add paper-account configuration fields:
  - display name
  - source venue
  - starting capital
  - base currency
  - fee profile
  - slippage profile
  - funding enabled / disabled
  - liquidation enabled / disabled
- [ ] Persist immutable execution provenance onto downstream trading artifacts where needed:
  - trader orders
  - trader fills
  - trader positions
  - deal review cases
  - deal review events where applicable

### Recommendation

Prefer keeping the trader bound to an `exchange_id`, but allow that exchange record to represent a paper wallet.

This minimizes disruption because:

- trader creation already selects an `exchange_id`
- runtime loading already resolves trader -> exchange config
- UI already understands exchange-account selection

## B. Paper Wallet And Ledger Layer

### Goal

Make the virtual wallet durable, auditable, and resettable.

### Draft Scope

- [x] Add a durable paper-wallet state model that tracks:
  - starting balance
  - available balance
  - used margin
  - unrealized PnL
  - realized PnL
  - total fees
  - total funding
  - current equity
- [x] Add an initial paper-wallet ledger / journal for state changes:
  - seed
  - config-sync / reseed-while-pristine
- [ ] Extend the paper-wallet ledger / journal for execution lifecycle state changes:
  - reset
  - order open fill
  - partial reduce or close
  - fee debit
  - funding debit / credit
  - liquidation / forced close
- [ ] Support wallet reset / reseed without corrupting historical deals.
- [ ] Support wallet cloning later for challenger / optimizer workflows.

### Acceptance

- [ ] A paper wallet can be inspected and reconstructed from durable state plus ledger events.
- [ ] Resetting a wallet starts a new paper session without deleting old review data.

## C. Bybit Paper Trader Adapter

### Goal

Implement a Bybit-flavored paper trader that satisfies the existing `Trader` interface and plugs into `AutoTrader`.

### Draft Scope

- [ ] Implement a new paper trader adapter for Bybit venue behavior.
- [ ] The adapter must implement the core `Trader` interface methods:
  - `GetBalance`
  - `GetPositions`
  - `OpenLong`
  - `OpenShort`
  - `CloseLong`
  - `CloseShort`
  - `SetLeverage`
  - `SetMarginMode`
  - `GetMarketPrice`
  - `SetStopLoss`
  - `SetTakeProfit`
  - `CancelStopLossOrders`
  - `CancelTakeProfitOrders`
  - `CancelAllOrders`
  - `CancelStopOrders`
  - `FormatQuantity`
  - `GetOrderStatus`
  - `GetClosedPnL`
  - `GetOpenOrders`
- [ ] Generate deterministic virtual exchange IDs for:
  - paper orders
  - paper trades / fills
  - paper positions
- [ ] Write through the existing generic stores:
  - `trader_orders`
  - `trader_fills`
  - `trader_positions`

### Acceptance

- [ ] A normal `AutoTrader` can run against the paper adapter without a separate AI decision path.

## D. Fill, Fee, Funding, And Liquidation Model

### Goal

Keep paper results realistic enough that the resulting dataset is useful for strategy review.

### Draft Scope

- [ ] Define the V1 fill model for market entries and exits:
  - fill from current observed Bybit price
  - apply configurable adverse slippage
- [ ] Define local trigger execution for:
  - stop loss
  - take profit
  - trailing stop
- [ ] Apply configurable fee logic:
  - maker / taker bps or venue profile
  - persist actual charged fee per fill
- [ ] Apply funding accrual for open perp positions at funding windows when enabled.
- [ ] Simulate liquidation or forced close when equity / margin rules are breached.
- [ ] Persist funding and liquidation evidence so review and debugging remain explainable.

### Important Note

Paper trades that feed learned-pattern and optimizer systems should not silently ignore venue costs. A paper dataset without fee and funding realism will overstate edge and poison later tuning decisions.

### Acceptance

- [ ] A paper deal’s final PnL includes fees.
- [ ] Funding-sensitive holds can differ from fee-only holds.
- [ ] Over-leveraged paper positions can be force-closed rather than floating unrealistically forever.

## E. Runtime Integration

### Goal

Load and run paper traders through the same runtime manager as live traders.

### Draft Scope

- [x] Prevent paper trader loading from instantiating a live exchange client by mistake.
- [ ] Extend trader loading so `execution_environment=paper` instantiates the Bybit paper adapter instead of a real exchange client.
- [x] Skip live credential validation for paper accounts.
- [x] Skip live balance probing when creating or updating a trader on a paper account.
- [x] Preserve the configured starting capital for paper traders instead of overwriting it from exchange balance.
- [x] Make account-state endpoints return paper wallet balances for paper accounts.
- [ ] Ensure trader dashboard / history / competition-style summaries can read paper trader balances and PnL.

### Acceptance

- [ ] A paper trader can be created and started without API keys.
- [ ] Its `initial_balance` and current equity are internally controlled, not externally probed.

## F. Deal Review, Pattern Learning, And Optimizer Integration

### Goal

Use paper results as first-class reviewable evidence without confusing them with live trades.

### Draft Scope

- [ ] Feed paper trades into deal review with the same artifacts as live:
  - open and close reasoning snapshot linkage
  - prompt bundle
  - decision-cycle price path
  - quality metrics
  - close reason
  - exit evidence
- [ ] Add execution-environment provenance into review payloads and detail views.
- [ ] Add filters for `live`, `testnet`, and `paper` in:
  - Deal Review
  - Pattern Lab
  - optimizer evidence payload selection where applicable
- [ ] Ensure learned patterns and optimizer payloads can include or exclude paper deals explicitly.
- [ ] Decide a safe default:
  - paper data should be visible
  - paper data should not be silently mixed into live-only analysis without the user being able to tell

### Recommendation

Default to visible provenance and explicit filtering, not hidden mixing.

That allows paper data to help pattern discovery without pretending it is identical to live execution quality.

### Acceptance

- [ ] A user can filter to only paper deals, only live deals, or both.
- [ ] Pattern and optimizer consumers can see the environment provenance of the evidence they are using.

## G. UI: Create And Operate A Paper Trader

### Goal

Make paper trading usable without DB edits or API-only workarounds.

### Draft Scope

- [x] Add a paper-account creation flow in the settings / exchanges UI:
  - choose `Bybit Paper`
  - set account name
  - set starting capital
  - set fee / slippage defaults
  - optionally set funding / liquidation toggles
- [x] Show clear environment badges in exchange lists:
  - `live`
  - `testnet`
  - `paper`
- [ ] Make paper accounts selectable in the normal trader-create modal.
- [ ] Show paper / live badge in trader cards and trader detail surfaces.
- [ ] Add paper wallet actions:
  - reset / reseed
  - optional clone

### Acceptance

- [ ] A user can create a new Bybit paper account from the UI.
- [ ] A user can create a new trader from the UI using that paper account.

## H. Optimizer Challenger Reuse

### Goal

Turn the previously planned "paper challenger" into a real paper-trading workflow.

### Draft Scope

- [ ] Reuse the paper-wallet / paper-trader infrastructure for challenger compares.
- [ ] Let challenger compare launch on cloned paper wallets instead of testnet-only exchange accounts.
- [ ] Keep the incumbent and challenger isolated from each other by default in paper mode.
- [ ] Allow challenger compare to seed its wallet from:
  - fixed configured paper capital
  - incumbent initial balance
  - optional cloned current equity baseline

### Acceptance

- [ ] The optimizer’s "paper" compare mode becomes a true virtual-wallet experiment, not just a renamed testnet lane.

## I. Data Provenance And Safety Guardrails

### Goal

Prevent accidental live / paper confusion.

### Draft Scope

- [ ] Persist execution-environment provenance immutably at the trade / review layer.
- [ ] Prevent a paper account from storing live credentials accidentally.
- [ ] Prevent a paper account from making authenticated trade requests.
- [ ] Make paper traders visually obvious across UI surfaces.
- [ ] Ensure that switching a trader from paper to live requires an explicit exchange-account swap, not an implicit toggle on the same historical account identity.

### Acceptance

- [ ] A paper trader cannot accidentally become live through a hidden toggle.
- [ ] Review and optimizer consumers can always tell what environment produced the evidence.

## J. Migration And Backward Compatibility

### Goal

Add paper support without breaking the current live system.

### Draft Scope

- [x] Add the required DB migrations for execution environment and paper account fields.
- [ ] Default all existing historical artifacts to `live` provenance.
- [x] Keep the current `testnet` field working where it already exists.
- [x] Avoid breaking current trader creation for real exchanges.

### Acceptance

- [ ] Existing live traders keep working unchanged after migration.

## K. Testing And Verification

### Goal

Make the paper dataset trustworthy enough to use for downstream learning.

### Draft Scope

- [ ] Unit tests for wallet accounting:
  - open
  - add
  - reduce
  - full close
  - fee charging
  - funding accrual
  - liquidation
- [ ] Runtime tests for stop / take-profit / trailing-trigger closes.
- [x] API/store integration tests for:
  - create paper account
  - preserve configured starting capital
- [ ] API tests for:
  - create paper trader
- [ ] Review tests proving that paper closes generate normal deal-review artifacts.
- [ ] Pattern / optimizer tests proving provenance is visible and filterable.

### Acceptance

- [ ] A paper trade lifecycle can be replayed in tests from open to review case.

## L. Optional Later Extension: Real Bybit Testnet Lane

### Goal

Keep a path open for real Bybit testnet support without confusing it with internal paper trading.

### Draft Scope

- [ ] Add real Bybit testnet API routing as a separate execution environment or venue variant.
- [ ] Keep it distinct from internal paper wallets.
- [ ] Use it only as an optional external sanity-check lane, not as the core paper-trading solution.

### Note

This should come after the internal paper system, not instead of it.

## Recommended Delivery Order

1. Foundation
   - execution environment model
   - paper account config
   - migrations
2. Core runtime
   - Bybit paper trader adapter
   - wallet state
   - basic market order execution
   - stop / take-profit / trailing support
3. Realism
   - fee model
   - funding
   - liquidation
4. UX
   - UI account creation
   - UI trader creation
   - environment badges
5. Knowledge integration
   - review provenance
   - pattern / optimizer filters
6. Challenger reuse
   - true paper challenger compare on cloned paper wallets

## Definition Of Done

- [ ] A user can create a `Bybit Paper` wallet in the UI with arbitrary start capital.
- [ ] A user can create and start a trader on that wallet without exchange API credentials.
- [ ] The trader opens and closes positions through the normal runtime path.
- [ ] Paper deals create normal orders, fills, positions, and deal-review cases.
- [ ] Fees are included in realized PnL.
- [ ] Funding and liquidation are represented when enabled.
- [ ] Deal Review and Pattern Lab can filter paper vs live evidence.
- [ ] Optimizer paper-challenger mode can later reuse the same infrastructure.

## Likely Code Touchpoints

Backend / runtime:

- `store/exchange.go`
- `api/handler_exchange.go`
- `api/handler_trader.go`
- `api/exchange_account_state.go`
- `manager/trader_manager.go`
- `trader/auto_trader.go`
- `trader/auto_trader_decision.go`
- `store/order.go`
- `store/position.go`
- `store/deal_review.go`

Frontend:

- `web/src/pages/SettingsPage.tsx`
- `web/src/components/trader/ExchangeConfigModal.tsx`
- `web/src/components/trader/TraderConfigModal.tsx`
- `web/src/pages/DealReviewPage.tsx`
- `web/src/types/config.ts`
- `web/src/types/trading.ts`

Related existing scopes:

- `docs/plans/2026-04-15-deal-review-phase-2-scope-draft.md`
- `docs/plans/2026-04-16-gamma-ray-autonomous-self-optimizer-scope.md`
- `docs/plans/2026-04-17-clean-exit-conditions-scope.md`
- `docs/plans/2026-04-18-pgvector-semantic-memory-scope.md`
- `docs/plans/2026-04-19-indicator-pattern-learning-scope.md`

## Recommended Immediate Next Step

Implement the first executable runtime slice:

- add the Bybit paper trader adapter that satisfies the existing `Trader` interface
- wire market entry / exit plus local stop-loss / take-profit / trailing closes into the paper wallet ledger
- persist paper orders, fills, and positions through the existing generic stores

The current foundation is now clean enough to build execution on top without scattering paper-specific hacks through live code paths.
