# Clean Exit Conditions Scope

Date: 2026-04-17
Owner: Codex draft for implementation
Status: Slices 1-7 implemented, unknown-exit cohort review and OKX/Gate trigger sync live

## Goal

Make deal exits explainable enough that a closed deal can answer, with concrete evidence:

- what actually closed it
- where that conclusion came from
- how confident that conclusion is
- whether it was AI-driven, trigger-order-driven, risk-system-driven, trailing-driven, or still unknown

This scope is explicitly about making exit attribution trustworthy in deal review.

## Why This Needs Its Own Scope

The current review module stores a final `close_reason`, but that is not the same as a clean exit explanation.

Today a large part of the system still collapses multiple different exit paths into ambiguous buckets such as `manual_exit`, especially when the final close is reconstructed from synced exchange data instead of a stored AI close cycle.

That creates two problems:

- the user cannot reliably understand why the position closed
- later AI review and strategy tuning learn from blurred labels

## Audit Snapshot From Live Data

This scope is based on a fresh audit of the current live DB copy.

- [x] `352` close events exist in `deal_review_events`.
- [x] `348` of those close events come from `sync_position`, only `4` from direct `ai_decision`.
- [x] `128` final deals currently end with `close_reason = manual_exit`.
- [x] The exact text `Position closed via synced exchange event (manual_exit, inferred from matched synced close fill / order).` currently appears `130` times.
- [x] Of those `manual_exit` deals, `51` are already matched to a filled `Stop` order, so they are not usefully “manual”.
- [x] `29` of those `manual_exit + Stop` cases even closed in profit, which strongly suggests profit-protecting or trailing-like exits in at least part of the cohort.
- [x] `120` final deals currently end with `stop_loss`.
- [x] `8` `stop_loss` deals are profitable, which again suggests moved protection stops / trailing-like behavior.
- [x] `0` final deals currently end with `trailing_stop`, so trailing exits are effectively not being attributed today.

## Product Outcome

After this scope, a deal detail should be able to show:

- final `close_reason`
- `exit_origin`
- `exit_reason_quality`
- structured `exit_evidence`
- a human-readable evidence summary

Examples:

- `take_profit` via `synced_trigger_order`, quality `explicit`, evidence `filled TakeProfit order`
- `stop_loss` via `target_proximity`, quality `high_confidence_inferred`
- `manual_exit` via `synced_market_order`, quality `high_confidence_inferred`
- `trailing_stop` via `trailing_engine`, quality `explicit`
- `unknown` via `exchange_sync_unknown`, quality `low_confidence_inferred`

## Scope Sections

## A. Exit Origin / Evidence Data Model

### Goal

Store more than one flat close label.

### Draft Scope

- [x] Add `exit_origin` to both deal-review cases and close events.
- [x] Add `exit_reason_quality` to both deal-review cases and close events.
- [x] Add `exit_evidence_summary` for short human-readable explanations.
- [x] Add structured `exit_evidence_json` for machine-readable evidence.
- [x] Expose those fields in API responses.

### Acceptance

- [x] A close can preserve both the normalized reason and the evidence provenance that produced it.

## B. Exit Inference Hardening

### Goal

Stop overloading `manual_exit` as the generic fallback for everything that was merely synced from exchange data.

### Draft Scope

- [x] Keep the current reason normalization for backward compatibility.
- [x] Add a richer inference result with:
  - normalized reason
  - origin
  - quality
  - evidence summary
  - evidence payload
- [x] Distinguish at least:
  - `ai_decision`
  - `synced_trigger_order`
  - `synced_market_order`
  - `synced_close_fill`
  - `target_proximity`
  - `stored_position_reason`
- [x] Add explicit `exchange_sync_unknown` fallback when no order/fill/target evidence exists.
- [x] Upgrade synthetic close reasoning text so the UI already becomes clearer even before larger UI work lands.

### Acceptance

- [x] A generic synced close should no longer only say `manual_exit, inferred from matched synced close fill / order` when stronger evidence exists.

## C. Trailing-Stop Attribution

### Goal

Make trailing exits show up as trailing exits.

### Draft Scope

- [x] Persist trailing-stop state updates instead of keeping them only in memory.
- [x] Journal every successful trailing stop move with timestamp and target stop price.
- [x] Match close fills / trigger orders against the last trailing state.
- [x] Classify matched closes as `trailing_stop`.

### Acceptance

- [x] New trailing-triggered exits can be distinguished from plain stop-loss exits.

## D. Internal Exit Intent Journal

### Goal

Capture internal system-initiated closes before the exchange close lands.

### Draft Scope

- [x] Journal explicit internal exit intents for AI close submission.
- [x] Journal explicit internal exit intents for drawdown guard / emergency close.
- [x] Journal explicit internal exit intents for manual UI close.
- [x] Journal explicit internal exit intents for grid emergency exits.
- [x] Extend the same journal pattern to the currently remaining grid automation modules (`grid_decision_close`, `grid_breakout_close_all`, `grid_level_stop_loss`).
- [x] Link exit intents to the eventual close event/case.
- [x] Preserve intent timestamps and source module names.

### Acceptance

- [x] A market close triggered by the system no longer looks identical to a user/manual close for AI/manual-UI/risk-covered paths.

## E. Exchange Trigger Order Enrichment

### Goal

Use more real exchange evidence instead of only post-hoc inference.

### Draft Scope

- [x] Persist richer trigger-order subtype metadata where the venue provides it.
- [x] Prefer explicit trigger-order evidence over target-proximity inference.
- [x] Extend close evidence so the UI can show trigger type, trigger price, and matched order ID.
- [x] Extend synced trigger-order history beyond Bybit to the currently active OKX and Gate paths.
- [x] Reuse venue-native subtype hints so synced filled close orders can override generic fill-only evidence.

### Acceptance

- [x] Trigger-based closes are attributed from actual order evidence wherever possible on supported synced venues.

## F. Review UI Evidence Panel

### Goal

Make the review page understandable without opening the DB or logs.

### Draft Scope

- [x] Show `exit_origin` on the deal detail view.
- [x] Show `exit_reason_quality` on the deal detail view.
- [x] Show `exit_evidence_summary` on the deal detail view.
- [x] Show the structured evidence block when present.
- [x] Add a dedicated anomaly/drilldown card for `unknown` / `low_confidence` exit cohorts.

### Acceptance

- [x] A user can explain why the deal closed by reading the deal detail alone.

## G. Historical Backfill And Audit Metrics

### Goal

Improve old deals where the evidence is already recoverable.

### Draft Scope

- [x] Re-sync existing positions to repopulate exit-origin / evidence fields.
- [x] Report how many `manual_exit` cases were reclassified or clarified.
- [x] Report how many closes remain low-confidence after backfill.

### Acceptance

- [x] The historical dataset becomes measurably cleaner, not only future deals.

### Current Live Snapshot After Backfill

- [x] Live closed deals with `exit_origin`: `398 / 398`
- [x] Live closed deals with `exit_reason_quality`: `398 / 398`
- [x] Live closed deals with `exit_evidence_summary`: `398 / 398`
- [x] `manual_exit` dropped from the earlier audit baseline `134` to `78` after the sync-unknown / trigger-order pass.
- [x] `59` deals now land in `close_reason = unknown` instead of the previously misleading `manual_exit` fallback.
- [x] `trailing_stop` increased from `0` in the original audit to `25` attributed exits.
- [x] Low-confidence closes currently remaining after backfill: `60`.
- [x] Current close reason mix: `stop_loss 127`, `manual_exit 78`, `ai_exit 61`, `unknown 59`, `take_profit 48`, `trailing_stop 25`.
- [x] Current exit-origin mix: `synced_trigger_order 168`, `synced_market_order 78`, `ai_decision 67`, `target_proximity 59`, `trailing_engine 25`, `stored_position_reason 1`.
- [x] `manual_exit` is now confined to the synced-market-order cohort in the current live dataset.
- [x] The explicit `exchange_sync_unknown` fallback is implemented and covered by tests; the current live dataset simply does not happen to need it yet.

## Current Delivery Slice

This implementation pass starts with the smallest slice that materially improves understanding right away.

- [x] Add the exit-origin / quality / evidence fields.
- [x] Upgrade close inference to populate them.
- [x] Improve the review detail UI so these fields are visible.
- [x] Persist trailing-stop updates and use them for close attribution.
- [x] Backfill exit attribution in the background on startup.
- [x] Persist and link internal exit intents for AI/manual-UI/risk closes.
- [x] Persist and link internal exit intents for the remaining current grid automation close paths.
- [x] Persist richer trigger-order metadata and expose it in exit evidence.
- [x] Replace ambiguous synced fallback cases with `unknown` / `exchange_sync_unknown` instead of misleading `manual_exit`.
- [x] Add a dedicated review drilldown for `unknown` / `low_confidence` exit cohorts plus an `exit_reason_quality` filter.
- [x] Extend historical trigger-order sync to OKX algo orders and Gate finished price-trigger orders.

## Recommended Next Steps After This Slice

These remain intentionally open after the current pass:

- future new automation modules should reuse the same exit-intent journal pattern as they are added
- extend venue-specific trigger sync to any still-unsupported live venues as they are enabled
- add a dedicated `exit_origin` / `trigger_source` filter layer for even narrower cohort drilldown
- add a small exit-attribution dashboard per trader so `unknown`, `low_confidence`, `trailing_stop`, `risk_guard`, and `manual_ui_close` trends are visible without opening single deals
