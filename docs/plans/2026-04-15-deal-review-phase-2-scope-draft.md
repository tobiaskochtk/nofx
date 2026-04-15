# Deal Review Phase 2 Scope Draft

Date: 2026-04-15
Owner: Codex draft for user review
Status: Draft for editing before implementation

## Purpose

Phase 1 made the review module usable for deal-centric inspection:

- deal list and detail
- open/close rationale
- prompt bundle and snapshots
- anomaly scan
- AI scans, patch apply, rollback
- per-cycle deal timeline when historical position snapshots exist

Phase 2 should shift the module from "good post-mortem viewer" to "strategy research workstation".

The target is:

- learn why strategies work or fail in specific market regimes
- separate entry quality from exit quality
- turn review findings into safe, testable strategy changes
- measure whether a patch improved the intended cohort after deployment
- compare incumbent vs challenger trader variants before keeping a new strategy change

## Product Outcome

After this phase, a trader owner should be able to answer:

- In which regimes does this strategy actually make money?
- Which losses are caused by bad entries, bad exits, or bad risk sizing?
- Which symbols, sessions, or buckets should be reduced, disabled, or reweighted?
- Which AI recommendation is supported by enough evidence to test or apply?
- Did the last applied patch improve the same cohort it was meant to fix?
- Should the old trader or the patched challenger trader survive after a timed live comparison?

## Design Principles

- Keep the module deal-centric, but enrich deals with regime metadata and quality metrics.
- Prefer structured, comparable metrics over additional free-text.
- Do not allow direct live patch application without a hard validation gate.
- Prefer challenger comparison over in-place replacement for material strategy changes.
- Keep challenger execution mode selectable, because paper/sim and live wallet compare serve different purposes.
- Treat AI as an analyst and experiment generator first, not an auto-pilot.
- Make every recommendation attributable to evidence, cohort, and post-change outcome.

## Current Foundation Already Live

- Review cases and open/close review events
- Prompt bundle persistence and historical backfill
- AI scans, scan compare, patch apply, version history, rollback
- Manual labels and analyst notes
- Anomaly heuristics
- Close-reason attribution from trigger-order sync
- Deal timeline points from decision-cycle position snapshots

These are not part of this draft scope because they already exist.

## Proposed Priority Order

1. Regime metadata persistence and filtering
2. Entry/exit quality metrics and cohort summaries
3. Hard validation gate plus old-vs-new challenger trader compare before patch promotion
4. Structured review taxonomy and bulk analyst workflow
5. Experiment bundles instead of direct config-only patches
6. Post-apply attribution and before/after cohort tracking
7. Label-trained classifiers and evidence scoring
8. Cross-model disagreement and recommendation ranking

## User Decisions Already Confirmed

- [x] Challenger comparison is optional, not mandatory, after a validated patch.
- [x] Challenger comparison execution mode should be selectable:
  - paper / simulation
  - isolated wallet live
  - shared wallet live when explicitly chosen
- [x] Challenger comparison should allow selecting which wallet is used.
- [x] Winner selection should be based on pure PnL, not a weighted composite score.
- [x] If the result is tied at the end of the selected comparison window, the comparison extends by another 12 hours.
- [x] Shared-wallet live mode should not be hidden behind an extra advanced-risk confirmation.
- [x] The worse trader should be deactivated automatically when the comparison resolves.

## Scope Sections

## A. Regime-Aware Dataset Layer

### Goal

Persist enough market-context metadata per deal so the review UI can filter by regime, not only by symbol/PnL/outcome.

### Draft Scope

- [ ] Persist open-side regime snapshot on each deal case.
- [ ] Persist close-side regime snapshot on each deal case.
- [ ] Add deal-level regime fields for:
  - trend regime
  - volatility regime
  - BTC-relative-strength regime
  - funding regime
  - OI regime
  - session / time-of-day bucket
  - weekday bucket
  - venue / tradability / liquidity tier
  - spread / slippage bucket where data is available
- [ ] Add backend filters for regime fields.
- [ ] Add UI filters for regime fields on `/deal-review`.
- [ ] Add quick cohort chips such as:
  - trend + high vol
  - low vol chop
  - BTC-leading alt weakness
  - funding extreme longs
  - Asia / EU / US session

### Acceptance

- [ ] A user can isolate deals like "Aggressive Altcoin Hunter losses during low-vol chop on altcoins in Asia session".
- [ ] AI scans can run on those regime-filtered cohorts directly.

## B. Entry / Exit / Risk Quality Layer

### Goal

Separate "good idea, bad management" from "bad idea from the start".

### Draft Scope

- [ ] Persist derived trade-quality metrics per deal:
  - max favorable excursion
  - max adverse excursion
  - MFE captured %
  - profit given back before exit
  - time to first profit
  - time to max drawdown
  - exit efficiency score
  - entry timing score
  - risk sizing score
- [ ] Add summary cards for:
  - bad entries
  - bad exits
  - avoidable losses
  - strong entries with weak exits
  - weak entries with lucky exits
- [ ] Add per-deal quality badges in the list and detail view.
- [ ] Add new anomaly panels for:
  - frequent profit give-back
  - repeated early stop-outs
  - outsized losses from leverage / sizing
  - close-reason quality by cohort

### Acceptance

- [ ] The UI can surface "entry was valid but exit captured only 18% of available MFE".
- [ ] The AI scan payload includes these quality metrics as structured inputs.

## C. Structured Analyst Review System

### Goal

Turn manual review from free-text into training data.

### Draft Scope

- [ ] Replace free-text-only label entry with a controlled label library.
- [ ] Keep free-form note as optional, but make structured labels primary.
- [ ] Add label groups:
  - entry quality
  - exit quality
  - risk sizing
  - rule adherence
  - market regime mismatch
  - execution quality
- [ ] Add bulk actions:
  - multi-select deals
  - apply label to cohort
  - save filtered cohort as named review set
- [ ] Add saved cohort presets.

### Suggested Core Label Library

- [ ] `good_entry`
- [ ] `late_entry`
- [ ] `chased_breakout`
- [ ] `bad_exit`
- [ ] `premature_take_profit`
- [ ] `stop_too_wide`
- [ ] `stop_too_tight`
- [ ] `oversized_risk`
- [ ] `avoidable_loss`
- [ ] `rule_violation`
- [ ] `regime_mismatch`
- [ ] `execution_slippage_issue`

### Acceptance

- [ ] Labels become machine-usable without heavy cleanup.
- [ ] Analysts can annotate 20-50 deals quickly without repetitive typing.

## D. AI Recommendation Confidence Layer

### Goal

Do not treat all model recommendations as equally trustworthy.

### Draft Scope

- [ ] Add evidence scoring to AI scan output.
- [ ] Add recommendation confidence bands:
  - low confidence
  - medium confidence
  - high confidence
- [ ] Require the AI scan result to cite:
  - dataset size
  - closed-deal count
  - relevant cohorts used
  - strongest supporting patterns
  - known uncertainty / caveats
- [ ] Display evidence strength in the scan cards and compare view.
- [ ] Add "insufficient evidence" state that blocks apply recommendations from being shown as ready.

### Acceptance

- [ ] A scan cannot recommend a material config change without showing supporting cohort size and confidence.

## E. Experiment Bundles

### Goal

Move from one-shot patching toward structured experimentation.

### Draft Scope

- [ ] Extend AI scan contract to return:
  - direct patch
  - experiment bundle
  - expected upside
  - expected risk
  - target cohort
- [ ] Support experiment types such as:
  - disable bucket in specific regime
  - lower leverage for a cohort
  - tighten / widen SL only in one regime
  - raise minimum confidence only for one session
  - block specific symbols until re-enabled
- [ ] Show experiments separately from direct patch actions in the UI.
- [ ] Allow user to adopt one experiment at a time rather than a broad patch blob.

### Acceptance

- [ ] Review results can generate small, testable changes instead of only broad strategy mutations.

## F. Hard Validation Gate + Challenger Trader Compare

### Goal

No strategy patch should be directly promoted over the incumbent trader without passing validation and, when enabled, a timed old-vs-new challenger comparison.

### Draft Scope

- [x] Introduce a mandatory validation workflow before a patch can become promotion-ready.
- [x] Validation must run on:
  - training slice
  - holdout slice
  - optional recent live-like slice
- Current implementation:
  - evidence gate + training/holdout split are live
  - recent live-like validation now also runs on the freshest subset of holdout deals
  - supported risk/cohort patch fields also run through a historical replay check
  - replay coverage now also includes restrictive symbol filters and conservative leverage-cap downscaling
  - replay-backed metric gates now enforce sample-size checks plus projected profit-factor, drawdown, win-rate-delta, expectancy-delta, and holdout degradation floors
  - challenger compares now have history, detail view, protocol timeline, auto winner resolution, tie extension, and manual stop / manual resolve controls
  - broader replay coverage and the full backtest engine are still pending
- [x] Define pass/fail gates for:
  - minimum sample size
  - profit factor floor
  - max drawdown ceiling
  - win-rate delta
  - expectancy delta
  - no severe degradation in holdout
- [x] Persist validation results beside each scan / patch.
- [x] Disable direct patch promotion from scans that have not passed validation.
- [x] Add a visible "blocked by validation" state in the UI.
- [x] Add a challenger creation workflow from a validated scan:
  - clone the incumbent trader config into a challenger trader
  - apply the proposed changed settings only to the challenger
  - keep the incumbent trader unchanged during the comparison window
  - link incumbent trader, challenger trader, source scan, and source strategy version
- [x] Keep challenger launch optional after validation:
  - user can stop at "validated, ready"
  - user can start challenger compare manually
- [x] Add challenger execution-mode options:
  - paper / simulation compare
  - isolated-wallet live compare
  - shared-wallet live compare
- [x] Add wallet-selection controls for challenger launch:
  - choose existing wallet / exchange binding
  - optionally use the same wallet as the incumbent
  - store the selected wallet mode in the comparison record
- [x] Add selectable challenger evaluation windows:
  - 12h
  - 24h
  - 36h
  - 48h
  - 96h
- [x] Persist a challenger comparison record with:
  - incumbent trader id
  - challenger trader id
  - evaluation start / end
  - selected comparison window
  - source scan id
  - source strategy version id
  - evaluation status
  - winner
  - loser
  - comparison summary note
  - comparison detail link
- [x] Define comparison metrics for incumbent vs challenger:
  - realized PnL as the deciding winner metric
  - supporting diagnostics such as PnL %, trade count, hold time, and cohort notes for interpretation only
- [x] Add a winner-selection policy:
  - winner keeps running
  - loser is deactivated automatically
  - if PnL is tied when the timer ends, extend the comparison by another 12 hours
  - repeat the 12-hour extension rule until a winner exists or the user stops the comparison manually
- [x] Add a comparison note and link in strategy history so each promoted or rejected patch can be traced back to its challenger result.
- [ ] Add a UI state for:
  - validation passed but challenger not started
  - challenger running
  - challenger finished
  - winner promoted
  - challenger rejected

### Acceptance

- [x] The UI cannot promote a patch that failed or skipped validation.
- [x] A validated scan can spawn a challenger trader with changed settings while the incumbent keeps running.
- [x] After 12h, 24h, 36h, 48h, or 96h, the system can automatically keep the better trader and deactivate the worse one.
- [x] If incumbent and challenger are tied on PnL at the end of the selected window, the comparison automatically extends by 12 hours.
- [x] Strategy history contains a comparison note and detail link for each challenger outcome.

## G. Post-Apply / Post-Challenger Attribution

### Goal

Measure whether a deployed patch or challenger winner actually improved what it was supposed to improve.

### Draft Scope

- [ ] Link each applied strategy version to:
  - source scan
  - target cohort
  - expected effect
  - applied timestamp
  - challenger comparison id where applicable
- [ ] Compare before/after performance for:
  - full strategy
  - target cohort
  - non-target cohort
- [ ] Add a strategy-version detail view with:
  - intended change
  - real outcome
  - regression warnings
  - rollback suggestion
- [x] Add a challenger-comparison detail view with:
  - incumbent summary
  - challenger summary
  - winner rationale
  - loser deactivation timestamp
  - link back to the source AI scan
- [ ] Add automatic "patch underperforming its target cohort" warning.

### Acceptance

- [ ] Every applied patch or challenger winner can be reviewed as a hypothesis with measured outcome.

## H. Learned Review Classifiers

### Goal

Use analyst labels and historical trade outcomes to pre-score bad or suspicious trades.

### Draft Scope

- [ ] Add heuristic classifier trained from structured labels.
- [ ] Add AI-assisted classifier for:
  - likely bad trade
  - likely bad exit
  - likely avoidable loss
  - likely regime mismatch
- [ ] Show classifier suggestions as review assists, not silent auto-labels by default.
- [ ] Allow analyst acceptance / rejection to improve future classifier quality.

### Acceptance

- [ ] The review list can highlight likely-problematic deals before manual inspection.

## I. Cross-Model Disagreement Analysis

### Goal

Use multiple models to identify fragile recommendations and robust consensus.

### Draft Scope

- [ ] Add scan compare metrics for:
  - recommendation overlap
  - conflict score
  - shared evidence
  - conflicting target cohorts
- [ ] Add leaderboard of model usefulness by cohort:
  - trend regime
  - chop regime
  - high-vol regime
  - certain symbols / buckets
- [ ] Add UI flags for:
  - strong consensus
  - mixed recommendation
  - low-confidence disagreement

### Acceptance

- [ ] The user can tell not only what two models suggested, but whether their disagreement matters.

## J. UI / Workflow Improvements

### Goal

Reduce friction so the module can actually be used as a daily research tool.

### Draft Scope

- [ ] Add cohort preset save/load.
- [ ] Add one-click anomaly-to-filter drill-down.
- [ ] Add one-click "scan this cohort" from anomaly cards.
- [ ] Add side-by-side deal compare for two selected deals.
- [ ] Add keyboard-friendly review workflow for next/previous deal.
- [ ] Add export for filtered dataset and scan outputs.
- [x] Add challenger compare launch flow from validated scans.
- [x] Add challenger compare history with status, timer, winner, and loser.
- [x] Show challenger execution mode and selected wallet in compare history and detail views.
- [x] Add compare protocol timeline and manual stop / resolve controls in the challenger detail view.
- [x] Add strategy-history note links that open the comparison detail directly.
- [ ] Add "review queue" mode:
  - unlabeled losses
  - biggest give-back exits
  - regime mismatch candidates
- [ ] Add "what changed after last patch?" home card for the module.

## Out of Scope For First Implementation Pass

- [ ] Tick-by-tick replay or full charting suite
- [ ] Fully automatic live self-modifying strategy without approval
- [ ] Deep ML training pipeline outside the existing AI scan flow
- [ ] Cross-exchange portfolio optimization

## Decisions To Confirm Before Implementation

- [ ] Which regime fields are must-have in v1 of Phase 2, and which can wait?
- [ ] Should structured labels replace free text immediately, or run side-by-side first?
- [ ] What validation rules should block patch apply in the first gate version?
- [x] Challenger comparison should remain optional after validation, not mandatory.
- [x] Challenger comparison should support both simulation and live-wallet modes, with wallet selection at launch time.
- [x] Winner vs loser should be decided by pure PnL.
- [x] If the result is tied after 12h / 24h / 36h / 48h / 96h, the comparison extends by another 12 hours.
- [x] Shared-wallet challenger mode should be available without an extra advanced-risk confirmation.
- [x] Challenger deactivation should happen automatically when the comparison ends with a winner.
- [ ] What should the comparison note show in strategy history by default?
- [ ] Should experiment bundles be visible before or after the hard backtest gate lands?
- [ ] Do we want classifier suggestions visible by default, or only as an opt-in assist?
- [ ] Which model providers should be allowed to produce applyable recommendations?

## Recommended First Delivery Slice

If we want the highest value with the lowest risk, the first live implementation slice should be:

- [ ] Regime metadata persistence
- [ ] Regime filters in UI and APIs
- [ ] Entry/exit quality metrics
- [x] Hard validation gate before patch promotion
- [x] Challenger trader compare workflow with timed winner/loser resolution

This slice would already unlock:

- better cohort analysis
- better trade-quality diagnosis
- safer strategy changes
- controlled old-vs-new live comparison before replacing a strategy
- selectable simulation vs live-wallet comparison modes

## Suggested Editing Pass For User

Before implementation starts, review and edit these sections first:

- `Proposed Priority Order`
- `Decisions To Confirm Before Implementation`
- `Recommended First Delivery Slice`

If you want, you can also mark any section as:

- `must-have`
- `nice-to-have`
- `not now`

## Implementation Tracking

- [x] Scope approved
- [x] Scope adjusted after user edits
- [x] Implementation started
- [x] Validation gate, replay gate, challenger launch flow, compare history, compare detail, and protocol timeline are live
- [x] Strategy-history compare deep links are live
- [ ] Broader replay coverage and full backtest gate are still pending
