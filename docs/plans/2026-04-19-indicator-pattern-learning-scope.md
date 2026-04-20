# Generalized Indicator Pattern Learning Scope

Date: 2026-04-19
Owner: Codex draft for staged implementation
Status: Phase 1-9 implemented, including lifecycle monitoring/history, analyst controls, rollback attribution, lifecycle trend rollups, pattern-level live-guard attribution rollups, persisted attribution caching, time-slice attribution deltas, analyst workflow action hints, persisted intervention history, direct live-action candidate escalation, Deal Review inline apply-actions, optimizer-visible strong live-action evidence, and dedicated Pattern Lab / Deal Review ranking+filtering for direct live-action candidates; next work should focus on optional automatic follow-through policies and optimizer action consumption

## Goal

Build a separate learning layer that can detect repeatable indicator and market-context patterns from historical decision and deal data, quantify their edge, and make those findings usable in review and optimization workflows.

## Current Implementation Snapshot

The first bounded V1 slice is already in progress and should be treated as the current baseline when continuing this scope later.

Implemented so far:

- persisted learned-pattern feature rows in `deal_review_pattern_features`
- persisted learned patterns in `deal_review_learned_patterns`
- replayable feature extraction from closed `deal_review_cases`
- learned pattern rebuild + stale refresh in store
- first pattern scopes:
  - `global`
  - `trader_local`
  - `regime_local`
  - `symbol`
- first pattern orders:
  - single features
  - feature pairs
  - limited triples
- first validation labels:
  - `confirmed`
  - `candidate`
  - `insufficient_evidence`
  - `false_positive`
  - `reverse_risk`
  - `drifting`
  - `expired`
- first recommended-use labels:
  - `review_hint`
  - `prompt_hint`
  - `config_candidate`
  - `monitoring_rule`
  - `monitor_only`
  - `expired_do_not_use`
- API route:
  - `GET /api/traders/:id/deal-review/learned-patterns`
- deal-detail API enrichment:
  - `GET /api/traders/:id/deal-review/cases/:case_id` now includes matching `learned_patterns`
- review UI additions:
  - deal-detail panel for matching learned patterns
  - anomaly-page summary cards for positive edges, anti-edges, and symbol overrides
  - dedicated `/pattern-lab` page with filters, risk watchlists, and drilldown links into deal review
  - Pattern Lab filter expansion for regime token, minimum confidence, minimum drift, and minimum sample size
  - optimizer run-detail panel for learned-pattern evidence, watchlists, and proposer/critic citations
  - semantic-memory deep links into `/pattern-lab` with direct `pattern_id` focus

Important V1 constraints already chosen:

- V1 started review-first and now supports explicit opt-in live guards
- source data is persisted structured closed-deal review data, not raw prompt mining
- optimizer can now consume learned patterns as review-window evidence
- optimizer can now synthesize backlog findings from learned anti-patterns, drift, and weak validation coverage
- semantic-memory projection for learned patterns is now implemented
- pattern evidence now exists both as embedded JSON on learned-pattern rows for compatibility and as normalized rows in `deal_review_pattern_evidence`
- learned-pattern rebuilds now append `deal_review_pattern_validation_runs`, including label counts and feature-quality metadata
- learned-pattern rebuilds now persist synthesized backlog suggestions in `deal_review_pattern_backlog_candidates`
- learned-pattern listings hydrate evidence from the normalized evidence table when available
- stale-refresh now falls back to validation-run build timestamps, so traders with zero current learned patterns still avoid unnecessary rebuild loops
- global learned patterns are now rebuilt user-wide and attached to the selected trader for review/optimizer consumption
- regime-local learned patterns now use exact persisted regime scope keys for matching and filtering
- triple-order learned patterns are now enabled with capped triple-candidate generation, prioritized feature selection, and stricter support/confidence thresholds before persistence
- learned-pattern live guard can now be enabled per strategy via `risk_control.learned_pattern_live_guard`
- live guard supports `monitor` and `hard_block` modes behind explicit strategy configuration
- live guard now only operationalizes negative patterns whose effective `recommended_use` resolves to `monitoring_rule`
- deal-review now exposes learned-pattern live guard status and recent guard events for analyst inspection
- optimizer run detail now surfaces `monitoring_rule` counts and labels so operationally eligible patterns are visible to analysts
- learned patterns now derive a runtime lifecycle state:
  - `active`
  - `degrading`
  - `rollback_watch`
  - `expired`
- lifecycle state now uses both validation quality and recent live-guard activity, including stale-evidence lag and repeated qualified guard hits
- learned patterns now persist a rebuild-stable `stable_key`, so lifecycle history and future live-guard references can survive learned-pattern rebuilds
- lifecycle snapshots now persist in `deal_review_pattern_lifecycle_snapshots`
- lifecycle snapshots are appended both on learned-pattern rebuilds and on matched learned-pattern live-guard events
- Pattern Lab and Deal Review now surface expiring monitoring rules and rollback-watch lists
- Pattern Lab and Deal Review now surface recent lifecycle trails for matched / listed learned patterns
- optimizer payload and backlog synthesis now include monitoring-rule lifecycle decay and rollback pressure
- monitoring rules now support analyst `keep live`, `suppress`, `retire`, and `re-arm` controls with persisted notes and control history keyed by rebuild-stable `stable_key`
- effective live-guard eligibility now respects analyst suppression / retirement rather than raw recommended-use alone
- learned-pattern live-guard events now derive follow-up attribution from later same-side attempts and realized deal outcomes
- Deal Review now surfaces a rollback-attribution lane for learned-pattern live-guard events, including:
  - `correctly_blocked`
  - `overblocked`
  - `warning_confirmed`
  - `warning_not_confirmed`
  - `threshold_missed_loss`
  - `threshold_missed_profit`
  - pending / open follow-up states
- learned patterns now derive snapshot-based lifecycle trends from the persisted lifecycle trail, including:
  - time spent `active`
  - time spent `degrading`
  - time spent `rollback_watch`
  - time spent `expired`
  - status-change count
  - stale guard-lag counts / shares
  - fragile live-rule classification
- Pattern Lab and Deal Review now surface lifecycle-trend summaries and “fragile monitoring rule” watchlists
- optimizer learned-pattern payloads now include lifecycle-trend evidence and aggregate fragile / lagging live-rule counts
- learned patterns now also derive pattern-level live-guard attribution rollups from recent matched guard events, including:
  - `protective`
  - `overblocking`
  - `mixed`
  - `pending`
- rollups now track recent resolved follow-up outcomes per learned pattern:
  - correctly blocked losers
  - overblocked profitable follow-through
  - warning-confirmed losers
  - warning-not-confirmed winners
  - threshold-missed loss / profit outcomes
- Pattern Lab and Deal Review now surface per-pattern live-guard attribution summaries plus dedicated “overblocking rules” watchlists
- optimizer learned-pattern payloads and conversation replay memory now include aggregate protective / overblocking counts and top overblocking rule evidence
- pattern-level live-guard attribution rollups now persist in `deal_review_pattern_live_guard_rollup_cache`
- cached rollups now reuse persisted JSON summaries until either:
  - the source guard-event count / latest event timestamp changes
  - or the cache refresh window expires
- unresolved rollups now refresh on a short TTL, while fully resolved rollups reuse a longer TTL for UI-heavy reopen workflows
- matched learned-pattern live-guard event writes now invalidate cached rollups immediately for the affected pattern ref
- live-guard attribution caches now also persist recent-vs-prior delta summaries for the same pattern refs
- learned patterns now compare the last `N` guard events vs the prior `N` guard events and derive:
  - `improving`
  - `stable`
  - `degrading`
  - `newly_overblocking`
  - `insufficient_evidence`
- Pattern Lab and Deal Review now surface delta trend bands and dedicated improving / degrading monitoring-rule watchlists
- optimizer learned-pattern payloads and conversation replay memory now include delta trend counts plus top improving / degrading rule evidence
- optimizer learned-pattern implication summaries now downgrade degrading / newly-overblocking monitoring rules away from normal anti-evidence reuse
- optimizer backlog synthesis now emits dedicated rollback / revalidation findings when a monitoring rule turns degrading or newly overblocking versus its prior guard window
- learned patterns now derive analyst action hints from lifecycle state, recent-vs-prior live-guard deltas, and manual-control state:
  - `suppress`
  - `retire`
  - `rearm`
- Pattern Lab now surfaces those action hints with priority, summary, auto-note, and one-click analyst application through the existing manual-control path
- Deal Review now surfaces the same action hints on matching learned patterns so case-level review sees the current live-rule recommendation directly
- optimizer learned-pattern payloads now include action-hint metadata so proposer / critic runs can cite the same recommended intervention and recent shift context
- critical learned-pattern action hints now synthesize explicit direct-live-action escalation objects so severe `degrading` / `newly_overblocking` cases are promoted into suppression or rollback-grade candidates
- learned-pattern intervention history now persists rebuild-stable suggestion / trigger rows plus manual analyst outcomes in `deal_review_learned_pattern_interventions`
- intervention history now deduplicates repeated identical suggestions via trigger fingerprints while preserving manual accept / override actions as separate events
- intervention rows now persist direct live-action candidate metadata so later review and optimizer runs can distinguish soft suggestions from strong suppress / rollback candidates
- active suggestions are now resolved as:
  - `accepted`
  - `overridden`
  - `superseded`
  - `cleared`
- Pattern Lab now surfaces the recent intervention history per monitoring rule, including open suggestions, repeated trigger counts, accepted actions, and overrides
- Deal Review now surfaces the same intervention history beside matching learned patterns on individual deal detail views
- Deal Review now also surfaces inline learned-pattern control actions, reusable suggested notes, manual-control history, and direct live-action candidate context without switching back to Pattern Lab
- optimizer learned-pattern payloads now include compact intervention-history evidence so proposer / critic runs can see whether recent suggestions were accepted, overridden, or still open
- optimizer learned-pattern payloads now also include open direct-live-action candidate evidence so proposer / critic runs can explicitly factor suppress / rollback-grade learned-rule pressure into the next window

This is the layer that should eventually answer questions like:

- if indicator cluster `A + B + C` appears, does price rise in a meaningful share of cases
- does that only hold for a specific symbol family, side, session, or volatility regime
- when generic long indicators look strong, which symbols systematically break that expectation
- which patterns are strong enough to inform prompt tuning, config changes, gating, or backlog recommendations

The system is not limited to false positives. It must support:

- positive edge discovery
- negative edge discovery
- reverse-edge discovery
- stability and drift tracking over time

## Why This Needs A Separate Scope

We already have three adjacent systems:

- semantic memory for retrieval
- deal-review symbol priors for symbol+side+regime contradictions
- autonomous optimizer for prompt/config proposals

Those systems provide strong groundwork, but they are not a generalized conditional-pattern miner.

The current symbol-prior layer is intentionally narrower:

- keyed around symbol+side+regime clusters
- derived from closed deal-review cases
- designed to surface learned priors in a review-first way

This new scope is broader:

- it must learn from normalized feature combinations
- it must support both positive and negative patterns
- it must reason about conditional edge strength
- it must separate stable signal from small-sample noise
- it must expose explainable evidence and confidence before any automation consumes it

## Primary Questions

### A. Repeatable Positive Patterns

- which feature combinations repeatedly precede profitable long or short outcomes
- which combinations are stable enough to trust beyond one small sample window

### B. Repeatable Negative Patterns

- which indicator combinations repeatedly lead to failure despite sounding good in isolation
- which combinations should be treated as anti-signals or caution flags

### C. Symbol-Specific Deviations

- which symbols deviate from the generic pattern
- when is the pattern global, and when is it symbol-local or regime-local

### D. Optimizer And Review Utility

- which learned patterns should become review hints
- which should become optimizer evidence
- which should create backlog items because missing features or missing data prevent better decisions

## Example Outputs

The target system should eventually produce statements like:

- `Momentum breakout + rising OI + low spread + Asia open` produced positive follow-through in `72%` of the last `83` validated long cases
- `RAVEUSDT long + crowded longs + flat OI + breakout chase` produced losses or give-back exits in `68%` of recent cases
- `Symbol X` looks generically bullish by current indicators, but its symbol-local pattern under this regime is still negative
- a proposed optimizer patch would increase exposure to a pattern with weak holdout support and rising drift, so it should be blocked

## Design Principles

### 1. Evidence Before Automation

- no learned pattern without support counts, holdout metrics, and recency context
- every learned pattern must link back to underlying deals and decision snapshots

### 2. Normalized Features, Not Only Prompt Text

- use structured features wherever possible
- free-text reasoning can supplement, but not define, the core pattern layer

### 3. Conditional Statistics First

- start with explainable conditional-frequency and expectancy methods
- do not jump straight to opaque ML

### 4. Positive And Negative Learning

- treat strong positive edges and strong anti-edges as first-class outputs
- avoid designing only for false positives

### 5. Regime Separation

- do not flatten all data into one global pool
- every pattern should be analyzable by side, regime, liquidity, session, and symbol scope

### 6. Time-Aware Validation

- training, holdout, and recent windows are mandatory
- drift and decayed relevance matter as much as raw hit rate

## Inputs

### Structured Inputs We Already Have

- `deal_review_cases`
- `deal_review_market_context`
- `decision_records`
- `decision_record_summary`
- `deal_review_price_timeline`
- `deal_review_exit_evidence`
- `deal_review_symbol_behavior_priors`
- `autonomous_optimizer_runs`
- `autonomous_optimizer_backlog_items`

### Likely Feature Families

- symbol and side
- selection bucket
- trend / volatility / OI / funding / BTC-relative-strength regimes
- session / weekday / venue / liquidity / spread / slippage
- open decision confidence
- action type and reject reasons
- stop / target placement quality
- realized outcome
- MFE / MAE / give-back
- exit type and exit-origin quality
- hold duration bucket

### Later Optional Inputs

- richer normalized indicator vectors
- feature snapshots not yet persisted today
- backtest or simulation counterfactual results
- external event or calendar risk flags

## Core Output Model

Each learned pattern record should eventually contain at least:

- `pattern_id`
- `scope_type`
  - `global`
  - `symbol_family`
  - `symbol`
  - `trader_local`
- `side`
- `pattern_signature`
- `feature_set`
- `regime_signature`
- `sample_count`
- `training_sample_count`
- `validation_sample_count`
- `recent_sample_count`
- `support_count`
- `contradict_count`
- `win_rate`
- `avg_pnl`
- `avg_pnl_pct`
- `expectancy`
- `avg_mfe_pct`
- `avg_mae_pct`
- `give_back_rate`
- `confidence_score`
- `stability_score`
- `drift_score`
- `recency_weight`
- `validation_label`
- `recommended_use`
  - `review_hint`
  - `prompt_hint`
  - `config_candidate`
  - `backlog_only`
  - `monitor_only`
- `evidence_links`

## Scope Boundaries

### In Scope

- pattern discovery from existing persisted structured data
- explainable conditional scoring
- positive and negative pattern detection
- validation and drift tracking
- review and optimizer visibility
- backlog generation for missing useful inputs

### Out Of Scope For V1

- direct live hard-veto logic
- opaque end-to-end black-box model replacement
- full tick-level forecasting
- exchange execution micro-optimization models
- unsupervised discovery over raw unnormalized prompt dumps

## Proposed Learning Architecture

### Layer 1. Feature Registry

Create a normalized feature vocabulary for decisions and deals:

- categorical regime dimensions
- bucketed numeric features
- derived execution quality features
- outcome labels

### Layer 2. Pattern Builder

Generate candidate pattern signatures from:

- single features
- feature pairs
- selected higher-order combinations

Keep this constrained to avoid combinatorial explosion.

### Layer 3. Scoring

For each candidate pattern:

- support rate
- contradiction rate
- conditional win rate
- conditional expectancy
- frequency lift against baseline
- sample-size adjusted confidence
- recency weighting
- stability across windows

### Layer 4. Validation

Split history into:

- training
- holdout
- recent

Then derive:

- confirmed
- candidate
- false positive
- false negative risk
- drifting
- expired

### Layer 5. Consumption

Use validated patterns in:

- deal-review UI
- semantic-memory corpus
- optimizer proposal payloads
- backlog suggestions for missing data/features

## Pattern Classes To Support

### Positive Edge Patterns

- repeated profitable outcome under the same normalized setup

### Negative Edge Patterns

- repeated poor outcome under the same normalized setup

### Reverse-Edge Patterns

- the system currently leans one way, but realized outcomes usually move the other way

### Symbol Overrides

- global pattern looks good, but a specific symbol repeatedly contradicts it

### Regime Overrides

- a pattern works in one regime and fails in another

## Recommended Data Strategy

### V1 Start Point

Start from persisted review and decision-summary data, not raw full prompt dumps.

Required initial feature source set:

- `deal_review_cases`
- `deal_review_market_context`
- `decision_record_summary`
- `deal_review_symbol_behavior_priors`

### Why Not Raw Full Prompt Mining First

- too noisy
- too expensive
- hard to normalize
- lower explainability

## Storage Model

### New Core Tables

- [x] `deal_review_pattern_features`
- [x] `deal_review_learned_patterns`
- [x] `deal_review_pattern_evidence`
- [x] `deal_review_pattern_validation_runs`
- [x] `deal_review_pattern_backlog_candidates`
- [x] `deal_review_pattern_lifecycle_snapshots`
- [x] `deal_review_pattern_live_guard_rollup_cache`

### Possible Semantic-Memory Projection

- [x] `semantic_memory` documents for validated patterns and major drifts

## UI Surfaces

### Review Page

- [x] show matching learned patterns beside the deal
- [x] separate positive-edge and anti-edge sections
- [x] expose evidence size and validation status

### Dedicated Pattern Lab

- [x] create a dedicated page for learned patterns
- [x] allow filtering by symbol, side, regime, outcome, label, confidence, drift
  - note: outcome direction is covered by existing positive-edge vs anti-edge pattern-class filters, and minimum sample-size filtering is now included as a support-size control
- [x] show strongest positive patterns
- [x] show strongest anti-patterns
- [x] show symbol-specific overrides
- [x] show recent drifts and reversals

### Optimizer Integration

- [x] expose confirmed patterns into optimizer payload
- [x] expose negative patterns as anti-evidence
- [x] expose missing-input suggestions as backlog items

## Phased Delivery Plan

## Phase 0. Decisions And Boundaries

- [x] lock the first feature families
- [x] lock the first supported pattern order
  - [x] single features
  - [x] pairs
  - [x] limited triples
- [x] lock validation-window policy
- [x] lock whether V1 is review-only or also optimizer-facing

## Phase 1. Feature Registry

- [x] define normalized feature schema for decision and review data
- [x] add replayable feature extraction from persisted rows
- [x] add feature-quality flags for missing or weak inputs
- [x] persist compact feature sets per decision/deal slice

## Phase 2. Candidate Pattern Builder

- [x] generate candidate patterns from normalized features
- [x] prevent combinatorial explosion with capped feature-order rules
- [x] persist pattern signatures and support counts
- [x] separate global, symbol, and regime-local scopes
  - [x] trader-local scope
  - [x] symbol scope
  - [x] global scope
  - [x] regime-local scope

## Phase 3. Scoring And Validation

- [x] compute baseline-aware lift and expectancy
- [x] compute confidence, stability, and recency weighting
- [x] split training / holdout / recent windows
- [x] classify confirmed / false-positive / reverse-risk / drifting / expired

## Phase 4. Review Integration

- [x] surface matching patterns in deal detail
- [x] add dedicated pattern-lab UI
- [x] add watchlists for:
  - note: positive, anti-pattern, symbol-override, and drift watchlists are implemented on the current UI surfaces
  - [x] strongest positive patterns
  - [x] strongest anti-patterns
  - [x] biggest recent drifts
  - [x] symbol-specific overrides

## Phase 5. Optimizer Integration

- [x] inject confirmed patterns into optimizer evidence
- [x] inject anti-patterns as negative evidence
- [x] emit backlog items for missing features or weak data coverage
- [x] track when optimizer proposals cite learned patterns

## Phase 6. Semantic Memory Integration

- [x] index validated patterns into semantic memory
- [x] retrieve similar historical patterns and reversals
- [x] support analyst free-text search over learned pattern corpus

## Phase 7. Controlled Actionability

- [x] define safe first operational uses:
  - [x] review-only hint
  - [x] prompt-only bias
  - [x] config candidate
  - [x] monitoring rule
- [x] add explicit approval gates before any direct live effect
  - note: direct live blocking is only active when `risk_control.learned_pattern_live_guard.enabled` is set on the strategy and the configured mode allows it
- [x] add rollback and expiry monitoring
  - note: lifecycle state is currently derived at read-time from learned-pattern metrics plus recent learned-pattern live-guard events; this avoids new persistence complexity while keeping the decay signal visible to review and optimizer consumers

## Scoring Ideas

Patterns should not be ranked by win rate alone.

Candidate scoring dimensions:

- conditional hit rate
- conditional expectancy
- support size
- support-vs-contradict ratio
- baseline lift
- regime concentration
- drift penalty
- recency weight

## Recommended Use Labels

Each pattern should eventually recommend one of:

- `review_hint`
- `prompt_hint`
- `config_candidate`
- `monitoring_rule`
- `backlog_missing_feature`
- `monitor_only`
- `expired_do_not_use`

## Backlog Generation

The AI should be able to create backlog items when it detects that better learning needs missing inputs, such as:

- missing indicator snapshots
- missing venue-quality features
- missing trigger-order history
- missing post-entry path descriptors
- missing volume or microstructure context

Each backlog item should include:

- title
- rationale
- expected usefulness
- estimated implementation value
- confidence

## Risks

### Overfitting

- small-sample patterns that look great but collapse in holdout

### Regime Leakage

- patterns that only work in one narrow regime but get generalized too broadly

### Feature Explosion

- too many combinations create noise and compute cost

### Hidden Data Quality Problems

- missing or stale feature capture produces false edges

## Acceptance Criteria For V1

- [x] learned patterns are built from persisted normalized features
- [x] both positive and negative patterns are visible
- [x] patterns show support counts, validation label, and drift
- [x] review can show matching learned patterns for a deal
- [x] optimizer can consume confirmed patterns and anti-patterns as evidence
- [x] backlog can capture missing-data requests suggested by the AI

## Relation To Existing Scopes

- semantic-memory scope:
  - retrieval substrate and analyst search
- symbol-behavior-priors scope:
  - first narrow learned-prior layer
- autonomous optimizer scope:
  - consumer of learned evidence, not the source of truth

This new scope should build on those systems rather than replacing them.

## Notes For Later Implementation

- start with explainable conditional stats before introducing heavier ML
- treat symbol priors as one input into this layer, not the final form
- keep pattern generation computationally bounded
- keep UI separated from the already dense review page once the pattern corpus grows

## Next Recommended Build Order

When resuming this scope later, the next sensible sequence is:

1. persist trend-trigger intervention history:
   - [x] track when delta-based rollback pressure fired, was acknowledged, auto-suggested, manually applied, or overridden so later optimizer runs can learn from those interventions too
2. promote severe delta trends into stronger live actionability controls:
   - [x] let high-confidence newly-overblocking rules raise direct rollback / suppression candidates rather than only backlog findings
3. extend actionability across review surfaces:
   - [x] add direct Deal Review apply-actions and deeper intervention-history drilldown so analysts can act on and inspect rule suppress / re-arm flows without switching surfaces

## Recommended Next Steps

The latest continuation closed the review-surface ranking/filtering gap:

1. ranking and filtering:
   - [x] add dedicated Pattern Lab / Deal Review filters for `live_action_hint.candidate_kind`, `direct_live_action_candidate`, and open-vs-resolved intervention state so the riskiest live rules can be reviewed first
   - [x] rank persisted direct-live-action candidates ahead of softer review hints in Pattern Lab summary cards, top-candidate panels, and Deal Review matched-pattern detail lists

The next highest-value continuation is now:

2. optional automatic follow-through policy:
   - [ ] add an opt-in policy layer that can auto-suppress rollback-grade learned rules once confidence / evidence thresholds are met, while still recording the action through the same intervention + manual-control audit trail
3. optimizer action consumption:
   - [ ] let proposer / critic distinguish between soft review hints and hard live-action candidates when deciding whether to patch prompts/config, defer, or recommend rollback
