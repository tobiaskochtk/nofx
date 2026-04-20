# Deal Review + AI Optimizer Scope

Date: 2026-04-12
Owner: Codex implementation pass
Status: In implementation

## Problem

Before this scope, the system stored cycle-level decision logs, but it did not persist a compact, deal-centric review object with exactly the two entries that matter most later:

- why a position was opened
- why a position was closed

That made it hard to review win/loss quality per deal without sifting through full decision history.

## Goals

- [x] Persist one deal review case per position/deal.
- [x] Persist two compact decision entries per deal where possible: `open` and `close`.
- [x] Persist realized profit/loss at deal level.
- [x] Keep storage compact by storing deal events, not every cycle as a first-class review object.
- [x] Add a dedicated review page with filters and detail drill-down.
- [x] Add an AI scan that analyzes a filtered dataset of deals.
- [x] Allow choosing the configured AI account and remote model variant for the scan.
- [x] Allow applying AI-generated strategy patches back into the strategy config.

## Data Model

- [x] Added `deal_review_cases`
  - one row per position/deal
  - stores trader, symbol, side, entry/exit, PnL, hold time, open/close event references
- [x] Added `deal_review_events`
  - one compact row for the open reason and one for the close reason
  - stores cycle number, reasoning, action, SL/TP, confidence, selection bucket, candidate sources
  - stores a compact snapshot payload for later review
- [x] Added `deal_review_ai_scans`
  - stores dataset filters, model used, AI result JSON, strategy patch JSON, apply status

## Backend Scope

- [x] Create and migrate new deal-review tables in store init.
- [x] Hook live order execution into pending deal-review event creation.
- [x] Link pending events to positions after position/order sync.
- [x] Sync open/close position lifecycle into deal-review cases.
- [x] Persist open/close PnL outcome on close events.
- [x] Backfill existing `trader_positions` into deal-review cases on startup.
- [x] Heuristically attach historical decision context from `decision_records` for older positions when no pending event exists.
- [x] Expose list/detail APIs for review cases.
- [x] Expose AI scan history APIs.
- [x] Expose AI scan run/apply APIs.
- [x] Expose available-model catalog API for configured providers.

## Frontend Scope

- [x] Add dedicated `/deal-review` page.
- [x] Add header navigation entry.
- [x] Add dataset filters:
  - symbol
  - side
  - status
  - outcome
  - date range
  - min/max PnL
- [x] Add summary cards:
  - deal count
  - win rate
  - net PnL
  - average hold time
- [x] Add filtered deal list.
- [x] Add per-deal detail card with open/close rationale.
- [x] Add candidate-source and execution-log exposure where available.
- [x] Add AI scan controls and scan history list.
- [x] Add apply-patch action from saved scan.

## AI Optimizer Scope

- [x] Build compact analysis payload from filtered deal dataset.
- [x] Include trader config and strategy config in scan context.
- [x] Include winners, losers, patterns, immediate actions, experiments, and a `strategy_patch` in AI response contract.
- [x] Persist raw structured AI result for later comparison.
- [x] Support choosing a configured AI model config plus remote model variant.
- [x] Apply patch into current strategy config with validation before saving.
- [x] Reload trader config after applying a patch.

## Historical Data Behavior

- [x] Existing positions are backfilled into review cases automatically.
- [x] Existing positions without captured pending events get synthetic review events.
- [x] When a nearby matching historical decision exists, synthetic events inherit:
  - cycle number
  - reasoning
  - confidence
  - SL/TP
  - selection bucket
  - candidate sources
  - execution log snapshot
- [ ] Full historical account/position snapshot backfill for older deals.
  - reason: old `decision_records` do not currently persist full account/position snapshots.

## Quality / Safety

- [x] Added regression test for historical decision-context backfill into deal review.
- [x] Added regression test for nested strategy patch merge behavior.
- [x] Backend compile check completed.
- [x] Frontend production build completed.
- [x] Static schema snapshots updated.

## Immediate Product Value

- [x] Review winners and losers deal-by-deal without reading every cycle.
- [x] See what triggered entry vs exit.
- [x] Compare what different models would recommend on the same filtered dataset.
- [x] Apply model suggestions directly into the active strategy after validation.

## Recommended Next Steps

- [x] Add side-by-side diff view for multiple AI scans on the same dataset.
- [ ] Add mandatory simulation/backtest gate before applying a strategy patch.
- [x] Add patch versioning and one-click rollback.
- [x] Add dataset tagging and analyst notes per deal.
- [x] Add manual labels such as `good entry / bad exit / avoidable loss / rule violation`.
- [ ] Add regime filters such as trend, volatility, BTC-relative-strength, and venue.
- [ ] Add “why this was a bad trade” classifier trained from review labels.
- [ ] Add strategy recommendation confidence bands and estimated evidence strength.
- [ ] Add auto-generated experiment bundles instead of only direct config patches.
- [ ] Add approval workflow for fully automatic patch application.
- [x] Add scan-to-scan diffing by model, date window, and symbol cohort.
- [x] Add anomaly detection for recurring bad closes, weak buckets, and overtraded symbols.

## Recommended Next Steps Progress

- [x] Scan history now supports pairwise comparison with filter diffs, patch diffs, overlap checks, and model-aware review.
- [x] Strategy applies now persist version snapshots and expose rollback actions from the review page.
- [x] Deal cases now support manual labels and free-form analyst notes.
- [x] The review page now surfaces anomaly heuristics for weak symbols, weak buckets, weak close reasons, and overtrading concentration.
- [x] Bybit trigger-order history is now synced into local order storage, so filled stop/TP exits can be attributed from exchange trigger orders instead of only from price-proximity inference.
- [x] Deal review events now preserve the AI input bundle for new decisions: market/input prompt, system prompt, decision JSON, raw response, and structured account/position snapshot.
- [x] Existing review events now backfill stored AI prompt/context artifacts from matching decision records when historical cycles are available.
- [x] Deal details now persist and display per-cycle price/PnL timeline points, including a graph and MFE/MAE-style summary when decision snapshots contain matching position state.
- [ ] Backtest/simulation gate still needs a hard blocking workflow before strategy apply.
- [ ] Regime-aware dataset filters still need persisted metadata at deal level before the UI can filter reliably.
- [ ] Learned classifiers, confidence bands, experiment bundles, and approval workflows remain open.

## Notes

- The storage model intentionally stays compact: deal-centric review rows plus two review events, instead of promoting every cycle into a first-class review entity.
- New trades will have the richest review data because live execution now emits pending open/close snapshots before the position sync step.
- Older trades can now appear in the module, but their detail quality depends on whether a matching historical decision record exists.
