# Symbol Behavior Priors Scope

Date: 2026-04-18
Owner: Codex draft for review before implementation
Status: Review/optimizer slice live, plus opt-in live guard for confirmed negative priors

## Goal

Add a dedicated learning layer that can discover and score symbol-specific behavior patterns which repeatedly contradict generic indicator expectations.

Example target:

- a symbol repeatedly shows a "strong long" indicator cluster
- but realized outcomes for that symbol and regime are often weak or negative
- the system learns that the symbol behaves differently under that setup
- review, optimizer, and later the trading engine can consume that learned prior explicitly

This scope is not only about prompt wording. It is about persistent, testable priors derived from historical evidence.

## Why This Needs Its Own Scope

The current semantic-memory scope gives us:

- searchable historical review cases
- searchable optimizer runs
- searchable curated decision-cycle summaries

That is necessary groundwork, but it is not enough for live "learned priors".

This next layer needs:

- explicit feature/outcome attribution
- symbol-level and regime-level aggregation
- confidence and sample-size handling
- decay and recency logic
- validation gates before live influence

That is broader than the current prompt-optimizer memory scope, so it should stay separate.

## Phase 0 Decisions Locked

The first implementation is intentionally narrow so it can ship quickly and remain explainable:

- first layer is `review-first`
- optimizer consumption is deferred until the prior quality is visible and testable in review
- priors are built from persisted `closed deal_review_cases` first
- `decision_record_summary` remains a next input source, not a day-one dependency
- priors are keyed by:
  - `symbol`
  - `side`
  - compact open-side regime signature
  - selection bucket
- first evidence thresholds:
  - `observed`: at least `3` closed deals
  - `candidate`: at least `5` closed deals
- first recency policy:
  - continuous decay
  - half-life `30 days`
- first live effect:
  - none
  - visibility only in review

## First Implementation Slice

The first slice being implemented now is:

- add a persisted `symbol_behavior_priors` table
- build replayable priors from closed deal-review cases
- compute sample size, win/loss mix, expectancy, MFE/MAE, give-back, recency, confidence, and stability
- store evidence links back to the underlying deal-review cases
- expose priors through deal-review API
- show matching priors in review before any optimizer or live-trading integration

## Core Questions This Scope Should Answer

### A. Symbol-Specific Edge Failure

- Which symbols repeatedly fail under a specific signal cluster?
- Is that effect stable or only visible in one short window?

### B. Regime-Specific Symbol Behavior

- Does the pattern only happen in certain regimes?
- Example:
  - `RAVEUSDT`
  - `long`
  - `uptrend + oi_flat + crowded_longs`
  - frequent give-back or stop-loss despite generic long indicators

### C. Priors For Review And Optimizer

- Can review surface these symbol priors next to similar cases?
- Can the optimizer use them as evidence when proposing prompt or config changes?

### D. Future Live Use

- Can we later gate entries using learned priors?
- Only after validation and with explicit rollback/monitoring.

## Design Principles

### 1. Evidence First

- no prior without sample size, win/loss mix, and regime context
- every prior must be traceable back to underlying deals or decision-cycle summaries

### 2. Separate Observation From Action

- learned observations are one layer
- recommendations are another
- live auto-application is a later, separately gated layer

### 3. Symbol + Regime + Signal Cluster

- do not learn only at the raw symbol level
- include:
  - side
  - regime
  - execution quality
  - signal cluster / reason-code cluster
  - selection bucket

### 4. Recency And Drift Matter

- old behavior should decay
- priors must weaken if recent evidence stops supporting them

## Candidate Outputs

Each learned prior should eventually include:

- `symbol`
- `side`
- `signal_cluster`
- `regime_signature`
- `sample_count`
- `win_rate`
- `avg_pnl`
- `avg_mfe`
- `avg_mae`
- `give_back_rate`
- `exit_mix`
- `confidence_score`
- `recency_weight`
- `stability_score`
- `recommended_action`
- `evidence_links`

## Proposed Signal Inputs

The first implementation should reuse already persisted data where possible:

- `deal_review_cases`
- `decision_record_summary`
- `deal_review_market_context`
- `deal_review_exit evidence`
- `selection_bucket`
- `reject_reasons`
- `reasoning / reason-code clusters`
- `execution quality / spread / slippage / liquidity`
- `optimizer findings`

Later optional sources:

- feature snapshots from raw decision payloads
- normalized indicator cluster vectors
- backtest counterfactual outputs

## Phased Delivery Plan

## Phase 0. Problem Framing

- [x] finalize whether the first prior layer is review-only or also optimizer-facing
- [x] define the minimum evidence threshold for a learned prior
- [x] define whether priors are per-symbol only or symbol+regime from day one
- [x] define decay / lookback windows

## Phase 1. Data Model

- [x] add `symbol_behavior_priors` table
- [x] add supporting evidence link table or evidence JSON schema
- [x] add fields for sample size, confidence, recency, and stability
- [x] add status model such as:
  - `observed`
  - `candidate`
  - `validated`
  - `rejected`
  - `expired`

## Phase 2. Feature And Cluster Builder

- [x] define compact signal-cluster extraction from decision summaries and review cases
- [x] define regime-signature extraction
- [x] define symbol-side cluster aggregation jobs
- [x] add replayable builder for priors from historical data

## Phase 3. Scoring

- [x] add confidence scoring based on sample count and consistency
- [x] add recency weighting
- [x] add contradiction scoring:
  - generic signal says long
  - symbol prior says this setup underperforms
- [x] add stability scoring across windows

## Phase 4. Review Integration

- [x] show matching symbol priors in deal-review detail
- [x] add cohort/filter view for symbol priors
- [x] add anomaly card for repeated symbol-specific edge failures
- [x] add links back to source cases and decision-cycle summaries

## Phase 5. Optimizer Integration

- [x] expose validated / candidate priors in optimizer evidence payload
- [x] let optimizer cite priors in proposals and critic feedback
- [x] distinguish between:
  - prompt-only implication
  - config implication
  - missing-data/backlog implication

## Phase 6. Validation Layer

- [x] holdout/backtest evaluation for learned priors
- [x] prior drift monitoring
- [x] rollback / expiration policy when priors stop holding
- [x] report false-positive / false-negative priors

## Phase 7. Optional Live Trading Influence

- [x] define safe first live use:
  - `monitor` mode for review-only runtime observation
  - `hard_block` mode for explicit pre-execution veto
  - first live effect is intentionally narrow:
    - negative priors only
    - confirmed priors only for hard blocks
    - no positive-prior force-entry behavior
- [x] require explicit gate before any hard live trading effect
- [x] add monitoring and rollback hooks

## Phase 8. Normalized Signal / Indicator Cluster Layer

- [x] persist normalized signal-cluster fields on symbol priors
- [x] derive normalized cluster labels from:
  - candidate sources
  - selection bucket
  - AI reasoning tags / phrases
  - persisted market-context indicator fields
- [x] persist normalized cluster labels on decision evidence snapshots
- [x] expose normalized clusters in review and semantic memory
- [x] let priors be regrouped or compared by normalized cluster, not only symbol+regime
- [x] add cluster-level filter controls in review
- [x] add cluster-conditioned performance comparisons for the optimizer
- [x] decide whether mature cluster-conditioned priors should tighten live guard thresholds

## First Recommended Slice

The first safe slice is review-first, not live-trading-first:

- [x] build symbol+regime prior candidates from existing closed review cases
- [x] persist them as `observed` / `candidate`
- [x] surface them in review detail with evidence links
- [x] do not affect live decisions yet

## Known Next Inputs After The First Slice

- `decision_record_summary` should later enrich signal-cluster extraction
- normalized reason-code clusters should later reduce dependency on free-text reasoning
- optimizer integration should only happen after review visibility shows the priors are directionally useful

## Current Implementation Notes

- current priors are outcome-backed by `closed deal_review_cases`
- current priors are additionally enriched by persisted `decision_records` with:
  - repeated open-action counts
  - average decision confidence
  - extracted reasoning signal tags
- autonomous optimizer payloads now include structured `symbol_behavior_priors` evidence with:
  - current-window match type (`closed_case`, `recent_open_execution`, `opportunity_symbol`, `global_background`)
  - implication type (`prompt_only`, `config_candidate`, `missing_data_backlog`)
  - contradiction, sample size, decision confidence, signal tags, and evidence case IDs
- proposal and critic prompts now explicitly instruct the models to cite `symbol_prior_references`
- conversation replay memory now keeps condensed symbol-prior evidence so later runs preserve that context
- anomaly scan now includes a dedicated repeated symbol edge-failure card:
  - only for a concrete trader filter
  - matched against the currently filtered closed-deal slice
  - ranked by slice recurrence, contradiction score, and weak historical expectancy
- explicit column backfill migration for `deal_review_symbol_behavior_priors` is live, so existing PostgreSQL tables pick up:
  - `decision_cycle_count`
  - `decision_open_count`
  - `avg_decision_confidence`
  - `contradiction_score`
  - `signal_tags_json`
  - `decision_evidence_json`
  - compact decision evidence entries
  - contradiction score
- current grouping key is:
  - `symbol`
  - `side`
  - `open_selection_bucket`
  - `open_trend_regime`
  - `open_volatility_regime`
  - `open_oi_regime`
- current evidence is stored as compact evidence JSON with linked `case_id`s
- current scoring includes:
  - sample-size confidence
  - recency decay
  - stability
  - contradiction score between repeated open decisions and realized negative behavior
  - composite score
- current normalized signal-cluster layer now adds:
  - persisted `signal_cluster_key`
  - persisted `signal_clusters`
  - decision-evidence level normalized clusters
  - cluster labels derived from:
    - candidate sources
    - selection bucket
    - reasoning phrases / tags
    - indicator/context fields like RSI, MACD, OI delta, price impulse, BTC-relative strength, and funding crowding
  - review UI visibility for both prior-level and decision-evidence-level clusters
  - semantic-memory indexing for cluster key and cluster labels
  - review-level rollups for top normalized clusters:
    - prior count
    - symbol count
    - sample count
    - decision-open count
    - confirmed / false-positive / reverse-edge-risk / drifting counts
    - top symbols
  - review filter support via:
    - `signal_cluster`
  - optimizer payload rollups via:
    - `symbol_behavior_priors.cluster_rollups`
    - per-cluster suggested-use hints
- current validation layer now also adds:
  - temporal train / holdout / recent windows per symbol prior
  - validation support counts and support score
  - recent-window support counts and support score
  - drift score between holdout and recent behavior
  - automatic status promotion / decay into:
    - `validated`
    - `rejected`
    - `expired`
  - human-readable `validation_summary`
  - derived prior-health reporting:
    - `validation_label`
    - `validation_alert`
    - `false_positive_score`
    - `false_negative_score`
  - rollup summary with:
    - confirmed prior count
    - false-positive prior count
    - false-negative-risk count
    - drifting count
    - top false-positive priors
    - top reverse-edge-risk priors
- current UI surfaces:
  - a trader-level learned symbol-prior block on the review page
  - the learned-prior block now also shows live guard status, thresholds, summary counters, and recent guard evaluation events
  - matching priors inside deal detail
  - normalized signal-cluster key and cluster chips on the learned-prior cards
  - normalized cluster chips inside stored decision-evidence rows
  - holdout / recent validation metrics and drift in the learned-prior cards
  - explicit prior-health badges and alerts on the cards
  - top false-positive / reverse-edge-risk watchlists
  - dedicated filter tabs for:
    - `confirmed`
    - `false_positive`
    - `false_negative_risk`
    - `drifting`
- current semantic-memory surfaces:
  - learned symbol priors are now indexed as `symbol_behavior_prior` documents
  - `/memory` can search them with shared semantic filters like symbol, side, status, and regime fields
  - symbol-prior memory docs now also carry:
    - `signal_cluster_key`
    - `signal_clusters`
  - hits can deep-link back into `/deal-review` with focused symbol/side filters and the selected prior highlighted
- current API surfaces:
  - `GET /api/traders/:id/deal-review/symbol-priors`
  - matching priors are also included in `GET /api/traders/:id/deal-review/cases/:caseId`
  - symbol-priors list now also returns a reporting summary block
- current live-guard surfaces:
  - strategy config now includes `risk_control.symbol_behavior_live_guard`
  - modes:
    - `monitor`
    - `hard_block`
  - default remains:
    - disabled
    - conservative thresholds when enabled
  - hard-block eligibility currently requires:
    - negative prior bias
    - confirmed validation label
    - sufficient sample count
    - prior confidence above threshold
    - strong current regime match
    - low false-positive score
    - low drift score
    - high contradiction score
  - every evaluated live open decision now writes an audit row into:
    - `deal_review_symbol_behavior_live_guard_events`
  - hard blocks now also flow into stored decision telemetry via:
    - `reject_reasons`
    - execution log entries
    - risk-control style failure messaging
  - rollback is now implicit and automatic:
    - if a prior drifts, expires, flips to false-positive, or drops below thresholds, it stops qualifying on the next cycle without any separate override step
  - mature normalized clusters now conservatively tighten live-guard thresholds before a hard block is allowed:
    - same-side cluster evidence must span multiple priors, multiple symbols, and enough closed-deal samples
    - the cluster also needs strong validation support, strong contradiction, and limited drift
    - when triggered, the guard raises:
      - minimum prior confidence
      - minimum current match score
      - minimum contradiction score
    - and lowers:
      - maximum false-positive score
      - maximum drift score
    - this does not create new block modes; it only makes borderline blocks fail closed more often when the broader cluster evidence is already negative

## What Still Matters Next

- next sensible expansion is to wire these prior-health outputs into:
  - eventually a safe live-use gate layer
- inside the new normalized-cluster block, the next sensible expansion is:
  - cluster-conditioned regrouping / comparisons so we can answer whether one indicator-cluster fails inside a symbol+regime slice while another still works

## Open Questions

- Should signal clusters remain a secondary explanatory layer, or become part of the primary prior grouping key?
- Should cluster-conditioned priors be allowed to hard-block live entries only after they outperform regime-only priors on holdout / recent windows?
- Should priors be per-symbol, per-symbol+side, or per-symbol+side+regime from day one?
- What minimum sample count is acceptable before a prior becomes visible to the optimizer?
- Should priors decay continuously or on fixed review windows?

## Notes

- This scope depends on the current semantic-memory work, especially `decision_record_summary`.
- The likely next dependency after this is a normalized feature-cluster layer so priors are not tied only to free-text reasoning.
- Do not let this scope silently become a hidden live override system. Observation, recommendation, and action need separate statuses and gates.
