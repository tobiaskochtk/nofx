# GAMMA-RAY Autonomous Self-Optimizer Scope

Date: 2026-04-16
Owner: Codex draft for user review
Status: Reviewed and adjusted after user decisions, ready for implementation

## Goal

Run a fully automatic live optimization loop on `GAMMA-RAY`, seeded from `Aggressive Altcoin Hunter`, that reviews performance every `12h` by default, applies validated improvements without a manual approval click, and keeps a scored backlog of missing capabilities the AI believes would materially improve decisions.

This is intentionally broader than deal review:

- it includes automatic patching
- it includes prompt evolution
- it includes missing-data / missing-capability discovery
- it must handle low-trade regimes, because `GAMMA-RAY` currently trades less than roughly `1 deal / 48h`

## Why This Needs Its Own Scope

The existing review module can already:

- analyze filtered deal cohorts
- validate patches
- apply patches
- launch challenger compares
- track post-apply outcomes

That is not yet the same thing as a live self-optimizing trader.

This scope adds an always-on control loop:

1. observe
2. review
3. propose
4. validate
5. apply automatically
6. monitor outcome
7. rollback or continue
8. create backlog items when better decisions require capabilities the system does not yet have

## User Intent Already Clear

- [x] The first test target is `GAMMA-RAY`.
- [x] The starting base should come from `Aggressive Altcoin Hunter`.
- [x] Review cadence should default to `12h`.
- [x] Improvements should apply automatically after the review window, without waiting for a manual approval click.
- [x] If the AI identifies missing indicators, missing inputs, or other missing system capabilities, those should go into a backlog.
- [x] The backlog should be scored so the next improvements are obvious.
- [x] Prompt updates are in scope, not only strategy-config patches.

## User Decisions Already Confirmed

- [x] Autonomous applies should support both `config_patch` and `prompt_patch` from day one.
- [x] Auto-rollback should be active in the first live pass, not monitoring-only.
- [x] The optimizer may pause itself automatically after repeated bad or low-evidence runs.
- [x] The backlog should be both AI-generated and manually editable, with AI as the primary producer.
- [x] The autonomous loop should use a proposer+critic model setup from day one.
- [x] Default model should be `OpenAI GPT-5.4`.
- [x] The default autonomous model choice should also be configurable in the UI/config surface.

## Core Design Stance

No human approval does **not** mean no safety.

This scope assumes:

- automatic apply is allowed
- hard machine validation remains mandatory
- every applied change must be reversible
- the loop must refuse low-evidence changes rather than forcing an edit every 12h

Without that, low-trade windows would produce noisy self-mutations instead of useful optimization.

## Product Outcome

After this scope, the user should be able to say:

- `GAMMA-RAY` is running an autonomous optimization loop
- every `12h` it either applies a validated change, explicitly does nothing, or creates scored backlog items
- prompt changes and config changes are versioned equally
- low-trade / no-trade windows are analyzed instead of silently ignored
- missing capabilities are turned into a ranked build queue, not lost in scan text
- every auto-change can be traced, measured, and rolled back

## Scope Sections

## A. Bootstrap `GAMMA-RAY` From `Aggressive Altcoin Hunter`

### Goal

Start the experiment from a stronger base than the current inactive `GAMMA-RAY` logic.

### Draft Scope

- [x] Add a one-time bootstrap workflow that copies:
  - strategy config
  - prompt config / prompt bindings where applicable
  - trader risk defaults that are safe to inherit
- [x] Persist a baseline snapshot so the original `GAMMA-RAY` can always be reconstructed.
- [x] Mark the bootstrap event in strategy history and optimizer history.
- [x] Tag the resulting trader/version as `autonomous_optimizer_seed`.
- [ ] Keep the experiment scoped to `GAMMA-RAY` only in v1.

### Acceptance

- [x] `GAMMA-RAY` can be reseeded from `Aggressive Altcoin Hunter` with one explicit tracked baseline version.

## B. Autonomous Review Loop

### Goal

Make the system review and act on a fixed cadence without waiting for manual operation.

### Draft Scope

- [x] Add an optimizer schedule record per trader.
- [x] Default cadence to `12h`, but keep it configurable.
- [x] On each run, build a review bundle from:
  - newly closed deals
  - still-open deal behavior
  - no-trade / skipped-opportunity windows
  - current config
  - current prompt bundle
  - recent auto-change history
- [x] Produce one of these run outcomes:
  - `no_change`
  - `config_patch`
  - `prompt_patch`
  - `config_and_prompt_patch`
  - `backlog_only`
  - `pause_optimizer`
- [x] Persist every run, even when the answer is `no_change`.

### Acceptance

- [x] Every review window ends in an explicit persisted result, not silent inactivity.

## C. Low-Trade / No-Trade Intelligence Layer

### Goal

Avoid making bad optimization decisions just because `GAMMA-RAY` does not trade often.

### Draft Scope

- [x] Persist reviewable `no-trade` evidence, not just executed deals.
- [x] Persist candidate / near-candidate context when the system considered but rejected trades.
- [x] Track trade starvation metrics such as:
  - candidate count
  - reject count
  - reject reasons
  - confidence distribution
  - symbol/session opportunity density
- [x] Let AI analyze both:
  - bad executed trades
  - missed or over-filtered opportunities
- [x] Add a hard `insufficient_evidence` outcome when there is not enough deal or candidate data.

### Why This Is Required

For a trader doing `< 1 trade / 48h`, optimizing only from closed deals is too weak. The system must also learn from what it almost traded and what it refused to trade.

### Acceptance

- [x] The optimizer can explain both `why trades were bad` and `why the trader stayed too inactive`.

## D. Auto-Apply Without Human Approval

### Goal

Let the optimizer apply changes automatically, but only when machine gates say the change is safe enough.

### Draft Scope

- [x] Reuse the existing review validation flow as the first machine gate.
- [x] Extend validation so autonomous applies can be blocked by:
  - insufficient sample size
  - holdout degradation
  - replay degradation where replay is supported
  - oversized patch scope
  - too many consecutive auto-applies
  - active cooldown window
- [x] Add per-run apply budget controls:
  - max one auto-apply per review window
  - max config paths changed per run
  - max prompt sections changed per run
- [x] Support automatic outcomes:
  - `auto_applied`
  - `blocked_by_gate`
  - `deferred_for_next_window`
- [x] Persist the exact gate reasons in optimizer history.

### Acceptance

- [x] The system can auto-apply without a click, but it can also refuse to apply and explain exactly why.

## E. Prompt Evolution Layer

### Goal

Treat prompts as first-class optimization targets, not only the trader config JSON.

### Draft Scope

- [x] Persist versioned prompt bundles attached to optimizer runs.
- [x] Allow AI output to propose prompt diffs for:
  - system prompt
  - market-context prompt
  - review / optimization prompt
  - decision formatting / rubric guidance
- [x] Validate prompt changes before activation:
  - required placeholders still present
  - output contract still parseable
  - guardrail instructions not removed
- [x] Record prompt diffs in the same history stream as config diffs.
- [x] Support rollback for prompt-only and mixed config+prompt changes.

### Acceptance

- [x] A run can safely auto-apply a prompt improvement and show the exact diff later.

## F. Missing Capability Backlog

### Goal

Turn AI observations like "this needs better OI delta" or "we need a pair residual feature" into structured, ranked work items.

### Draft Scope

- [x] Add a new optimizer backlog entity.
- [x] Backlog item types should include:
  - missing indicator / feature
  - missing market data source
  - missing execution telemetry
  - missing regime metadata
  - missing risk control
  - missing prompt instruction
  - missing review metric
  - other system capability gap
- [x] Persist for each backlog item:
  - title
  - category
  - description
  - evidence excerpts / related runs
  - affected trader(s)
  - expected impact
  - confidence
  - implementation cost estimate
  - urgency
  - recurrence count
  - composite score
  - status
- [x] Support backlog statuses:
  - `new`
  - `confirmed`
  - `planned`
  - `in_progress`
  - `done`
  - `rejected`
- [x] Make backlog items editable from the UI, while preserving AI-origin metadata and source evidence.
- [x] Show whether an item was created by AI, edited by user, or merged from repeated findings.
- [x] Deduplicate repeated AI findings into the same backlog theme where possible.

### Proposed Score Inputs

- evidence strength
- expected PnL impact
- frequency / recurrence
- whether it blocks autonomous improvement
- implementation cost
- breadth across traders

### Acceptance

- [x] Missing capabilities no longer disappear inside scan prose; they become ranked work items.

## G. Auto-Rollback And Drift Control

### Goal

Prevent the optimizer from walking itself into a degraded state.

### Draft Scope

- [x] Add post-apply monitoring windows for each autonomous change.
- [x] Add automatic rollback triggers such as:
  - target cohort degradation
  - repeated losing windows after apply
  - sharp drop in trade quality
  - sharp increase in inactivity without better outcome
- [x] Add `rollback_pending`, `rolled_back`, and `kept` states.
- [x] Persist rollback rationale and link it to the source optimizer run.
- [x] Prevent repeated flip-flopping by using rollback cooldowns.

### Acceptance

- [x] Every automatic change is either kept, rolled back, or still in monitored observation.

## H. Operator UI / Observability

### Goal

Make the autonomous loop inspectable instead of opaque.

### Draft Scope

- [x] Add an optimizer overview page or section showing:
  - current state
  - next scheduled run
  - last run result
  - current prompt/config version
  - current backlog top items
- [x] Add run history with filters for:
  - applied
  - blocked
  - backlog-only
  - no-change
  - rolled-back
- [x] Add a diff viewer for:
  - config changes
  - prompt changes
  - gate results
- [x] Add backlog board with score sorting and status updates.
- [x] Add links from optimizer runs into deal-review cohorts and strategy-version details.

### Acceptance

- [x] The user can reconstruct exactly why the optimizer changed or did not change `GAMMA-RAY`.

## I. Model Strategy

### Goal

Allow the autonomous loop to evolve safely across available ChatGPT models instead of assuming one model is always best.

### Draft Scope

- [x] Let the optimizer choose from configured available models.
- [x] Default the proposer model to `OpenAI GPT-5.4`.
- [x] Default the critic model to `OpenAI GPT-5.4` unless explicitly changed.
- [x] Add configurable optimizer model selection in the config UI so the default can be changed without code edits.
- [x] Persist which model authored each autonomous run.
- [x] Support:
  - primary model for proposal
  - secondary model for critique / veto
- [x] Add per-model optimizer outcome tracking:
  - apply rate
  - rollback rate
  - kept-win rate
  - backlog usefulness

### Acceptance

- [x] The system can later show which model produced the safest or most useful autonomous changes.

## Proposed Status Model

- [x] `scheduled`
- [x] `running`
- [x] `insufficient_evidence`
- [x] `no_change`
- [x] `backlog_only`
- [x] `blocked_by_gate`
- [x] `deferred_for_next_window`
- [x] `auto_applied`
- [x] `monitoring`
- [x] `rollback_pending`
- [x] `rolled_back`
- [x] `kept`
- [x] `paused`
- [x] `failed`

## Suggested First Delivery Slice

If we want the safest useful first live version, the first slice should be:

- [x] Bootstrap `GAMMA-RAY` from `Aggressive Altcoin Hunter`
- [x] 12h autonomous review scheduler
- [x] no-trade / inactivity intelligence
- [x] automatic config + prompt patch apply with hard machine validation
- [x] active auto-rollback
- [x] optimizer run history
- [x] scored missing-capability backlog
- [x] proposer+critic model execution with `GPT-5.4` as the default pair

This first slice would already prove the core loop without yet requiring the full prompt-evolution and rollback stack.

## Recommended Delivery Order

1. Bootstrap + optimizer run records
2. 12h scheduler + explicit run outcomes
3. no-trade / candidate-rejection dataset
4. auto-apply gate on config + prompt patches
5. backlog entity + scoring
6. operator UI
7. prompt evolution
8. auto-rollback and drift control
9. proposer / critic model controls and scoring

## Key Risks

- Low trade count can trick the optimizer into overfitting tiny samples.
- Automatic prompt mutation can silently break output structure if not validated strictly.
- Frequent auto-applies can create config drift that becomes hard to reason about.
- Missing candidate / rejection telemetry would make inactivity optimization mostly guesswork.
- A backlog without dedupe/scoring will quickly become noisy.

## Decisions Confirmed For Implementation

- [x] `GAMMA-RAY` should auto-apply both `config_patch` and `prompt_patch` in the first implementation pass.
- [x] Auto-rollback should be enabled in the first live pass.
- [x] The optimizer may pause itself automatically after repeated bad runs.
- [x] Backlog items should be manually editable in the UI, while remaining AI-first.
- [x] The autonomous loop should support proposer+critic from day one.
- [x] `OpenAI GPT-5.4` should be the default autonomous model pair.
- [x] The default optimizer model should be configurable from the config UI.

## Initial Implementation Tracking

- [x] Scope drafted
- [x] Scope reviewed with user
- [x] Scope adjusted after user edits
- [x] Implementation started
- [x] Bootstrap flow live
- [x] Autonomous scheduler live
- [x] Auto-apply gate live
- [x] Backlog scoring live
- [x] Prompt evolution live
- [x] Rollback control live

## Current Progress Notes

- [x] Backend persistence scaffolding is live for autonomous optimizer config, run history, and scored backlog items.
- [x] Per-trader optimizer defaults now initialize with `GPT-5.4` as proposer and critic, preferring the configured OpenAI account when available.
- [x] Backend APIs are live for:
  - get/update optimizer config
  - list optimizer runs
  - list/create backlog items
  - bootstrap a target trader from a source trader
- [x] `GAMMA-RAY` has been bootstrapped live from `Aggressive Altcoin Hunter`, with a dedicated seed strategy version and optimizer bootstrap run persisted.
- [x] The scheduled autonomous review executor is live and now writes explicit run outcomes on due windows.
- [x] The first live scheduled `GAMMA-RAY` run already recorded `insufficient_evidence` instead of mutating config blindly:
  - `0` closed deals
  - `109` decision cycles
  - `1090` candidate observations
- [x] The auto-apply decision loop now runs proposer + critic and can auto-apply validated `config_patch` and `prompt_patch` outcomes.
- [x] Low-trade windows can now still generate `prompt_patch` or `backlog_only` outcomes from decision-cycle telemetry, while `config_patch` stays blocked until closed-deal validation is strong enough.
- [x] Prompt patches now cover the broader live prompt surfaces and are validated for scope, template existence, token budget, and output-contract safety before activation:
  - strategy market-context guidance
  - strategy decision-format / rubric guidance
  - optimizer proposer instructions
  - optimizer critic instructions
- [x] Applied autonomous runs now enter monitored state and can auto-rollback on a negative monitored post-apply window, with rollback linked back to the source run.
- [x] The deal-review UI now includes a live Autonomous Optimizer section with config controls, scored improvement steps, and run history for the selected trader.
- [x] Optimizer runs now expose a detail inspector with gate reasons, config/trader/optimizer diffs, raw patch payloads, and direct links into the exact review cohort or linked strategy version.
- [x] The optimizer UI has been separated onto its own `/optimizer` page so the deal-review page stays focused on deal analysis.
- [x] The optimizer now receives explicit low-trade/starvation telemetry from decision records:
  - hold vs wait counts
  - normalized reject reasons
  - confidence-band distributions
  - opportunity density by session
  - opportunity density by symbol
- [x] Run detail now makes the learning context visible:
  - recent optimizer runs used as context for proposer/critic
  - low-trade telemetry summary per window
  - reject reasons, session density, symbol density, and confidence bands directly in the UI
- [x] Autonomous gate cooldown and `deferred_for_next_window` handling are now live:
  - configurable cooldown hours
  - configurable max consecutive auto-applies
  - deferred status instead of hard-block when a patch is valid but must wait
  - stored next-eligible apply time visible in the optimizer UI
- [x] Broader monitoring and rollback drift control is now live:
  - monitoring windows now resolve back to the original applied run even after multiple monitoring passes
  - rollback can trigger on repeated losing windows, target cohort degradation, trade quality drop, or inactivity drift
  - rollback analysis, baseline snapshot, and cumulative monitored metrics are visible in optimizer run detail
  - rolled-back runs participate in cooldown handling so the loop does not immediately flip back
- [x] Backlog items are now fully editable in the optimizer UI:
  - title, category, description, expected impact, urgency, confidence, implementation cost, recurrence, and status
  - backend updates preserve AI origin, source run linkage, merged finding count, and stored evidence/metadata
- [x] Optimizer config UI now exposes the live proposer/critic instruction overlays, so the broader optimizer-review prompt surfaces are inspectable and editable without leaving `/optimizer`.
- [x] `/optimizer` now exposes per-model outcome tracking for proposer/critic pairs:
  - apply rate
  - rollback rate
  - kept-win rate
  - backlog usefulness
  - ranked outcome score with current active pair highlighted
- [x] Stale autonomous optimizer state is now auto-recovered:
  - stale `running` runs are marked `failed` with explicit recovery metadata
  - orphan `running` configs without an active run are reset automatically
  - `next review window` is re-populated instead of staying blank after an interrupted run
- [x] `rollback_pending` is now used as a real transitional state:
  - degrading monitoring first marks rollback as pending
  - the next due optimizer window executes the actual rollback
  - run history and current state now show the intermediate governance step explicitly
- [x] Oversized prompt patches are now auto-sliced instead of hard-blocked on field count:
  - if a prompt proposal touches more than 6 fields, the highest-priority 6 prompt surfaces are applied first
  - deferred prompt fields are stored in run validation and visible in the optimizer UI
  - recent optimizer context now carries prompt requested/applied/deferred counts forward into later runs
