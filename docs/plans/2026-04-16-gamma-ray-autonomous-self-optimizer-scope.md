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

- [ ] Add a one-time bootstrap workflow that copies:
  - strategy config
  - prompt config / prompt bindings where applicable
  - trader risk defaults that are safe to inherit
- [ ] Persist a baseline snapshot so the original `GAMMA-RAY` can always be reconstructed.
- [ ] Mark the bootstrap event in strategy history and optimizer history.
- [ ] Tag the resulting trader/version as `autonomous_optimizer_seed`.
- [ ] Keep the experiment scoped to `GAMMA-RAY` only in v1.

### Acceptance

- [ ] `GAMMA-RAY` can be reseeded from `Aggressive Altcoin Hunter` with one explicit tracked baseline version.

## B. Autonomous Review Loop

### Goal

Make the system review and act on a fixed cadence without waiting for manual operation.

### Draft Scope

- [ ] Add an optimizer schedule record per trader.
- [ ] Default cadence to `12h`, but keep it configurable.
- [ ] On each run, build a review bundle from:
  - newly closed deals
  - still-open deal behavior
  - no-trade / skipped-opportunity windows
  - current config
  - current prompt bundle
  - recent auto-change history
- [ ] Produce one of these run outcomes:
  - `no_change`
  - `config_patch`
  - `prompt_patch`
  - `config_and_prompt_patch`
  - `backlog_only`
  - `pause_optimizer`
- [ ] Persist every run, even when the answer is `no_change`.

### Acceptance

- [ ] Every review window ends in an explicit persisted result, not silent inactivity.

## C. Low-Trade / No-Trade Intelligence Layer

### Goal

Avoid making bad optimization decisions just because `GAMMA-RAY` does not trade often.

### Draft Scope

- [ ] Persist reviewable `no-trade` evidence, not just executed deals.
- [ ] Persist candidate / near-candidate context when the system considered but rejected trades.
- [ ] Track trade starvation metrics such as:
  - candidate count
  - reject count
  - reject reasons
  - confidence distribution
  - symbol/session opportunity density
- [ ] Let AI analyze both:
  - bad executed trades
  - missed or over-filtered opportunities
- [ ] Add a hard `insufficient_evidence` outcome when there is not enough deal or candidate data.

### Why This Is Required

For a trader doing `< 1 trade / 48h`, optimizing only from closed deals is too weak. The system must also learn from what it almost traded and what it refused to trade.

### Acceptance

- [ ] The optimizer can explain both `why trades were bad` and `why the trader stayed too inactive`.

## D. Auto-Apply Without Human Approval

### Goal

Let the optimizer apply changes automatically, but only when machine gates say the change is safe enough.

### Draft Scope

- [ ] Reuse the existing review validation flow as the first machine gate.
- [ ] Extend validation so autonomous applies can be blocked by:
  - insufficient sample size
  - holdout degradation
  - replay degradation where replay is supported
  - oversized patch scope
  - too many consecutive auto-applies
  - active cooldown window
- [ ] Add per-run apply budget controls:
  - max one auto-apply per review window
  - max config paths changed per run
  - max prompt sections changed per run
- [ ] Support automatic outcomes:
  - `auto_applied`
  - `blocked_by_gate`
  - `deferred_for_next_window`
- [ ] Persist the exact gate reasons in optimizer history.

### Acceptance

- [ ] The system can auto-apply without a click, but it can also refuse to apply and explain exactly why.

## E. Prompt Evolution Layer

### Goal

Treat prompts as first-class optimization targets, not only the trader config JSON.

### Draft Scope

- [ ] Persist versioned prompt bundles attached to optimizer runs.
- [ ] Allow AI output to propose prompt diffs for:
  - system prompt
  - market-context prompt
  - review / optimization prompt
  - decision formatting / rubric guidance
- [ ] Validate prompt changes before activation:
  - required placeholders still present
  - output contract still parseable
  - guardrail instructions not removed
- [ ] Record prompt diffs in the same history stream as config diffs.
- [ ] Support rollback for prompt-only and mixed config+prompt changes.

### Acceptance

- [ ] A run can safely auto-apply a prompt improvement and show the exact diff later.

## F. Missing Capability Backlog

### Goal

Turn AI observations like "this needs better OI delta" or "we need a pair residual feature" into structured, ranked work items.

### Draft Scope

- [ ] Add a new optimizer backlog entity.
- [ ] Backlog item types should include:
  - missing indicator / feature
  - missing market data source
  - missing execution telemetry
  - missing regime metadata
  - missing risk control
  - missing prompt instruction
  - missing review metric
  - other system capability gap
- [ ] Persist for each backlog item:
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
- [ ] Support backlog statuses:
  - `new`
  - `confirmed`
  - `planned`
  - `in_progress`
  - `done`
  - `rejected`
- [ ] Make backlog items editable from the UI, while preserving AI-origin metadata and source evidence.
- [ ] Show whether an item was created by AI, edited by user, or merged from repeated findings.
- [ ] Deduplicate repeated AI findings into the same backlog theme where possible.

### Proposed Score Inputs

- evidence strength
- expected PnL impact
- frequency / recurrence
- whether it blocks autonomous improvement
- implementation cost
- breadth across traders

### Acceptance

- [ ] Missing capabilities no longer disappear inside scan prose; they become ranked work items.

## G. Auto-Rollback And Drift Control

### Goal

Prevent the optimizer from walking itself into a degraded state.

### Draft Scope

- [ ] Add post-apply monitoring windows for each autonomous change.
- [ ] Add automatic rollback triggers such as:
  - target cohort degradation
  - repeated losing windows after apply
  - sharp drop in trade quality
  - sharp increase in inactivity without better outcome
- [ ] Add `rollback_pending`, `rolled_back`, and `kept` states.
- [ ] Persist rollback rationale and link it to the source optimizer run.
- [ ] Prevent repeated flip-flopping by using rollback cooldowns.

### Acceptance

- [ ] Every automatic change is either kept, rolled back, or still in monitored observation.

## H. Operator UI / Observability

### Goal

Make the autonomous loop inspectable instead of opaque.

### Draft Scope

- [ ] Add an optimizer overview page or section showing:
  - current state
  - next scheduled run
  - last run result
  - current prompt/config version
  - current backlog top items
- [ ] Add run history with filters for:
  - applied
  - blocked
  - backlog-only
  - no-change
  - rolled-back
- [ ] Add a diff viewer for:
  - config changes
  - prompt changes
  - gate results
- [ ] Add backlog board with score sorting and status updates.
- [ ] Add links from optimizer runs into deal-review cohorts and strategy-version details.

### Acceptance

- [ ] The user can reconstruct exactly why the optimizer changed or did not change `GAMMA-RAY`.

## I. Model Strategy

### Goal

Allow the autonomous loop to evolve safely across available ChatGPT models instead of assuming one model is always best.

### Draft Scope

- [ ] Let the optimizer choose from configured available models.
- [ ] Default the proposer model to `OpenAI GPT-5.4`.
- [ ] Default the critic model to `OpenAI GPT-5.4` unless explicitly changed.
- [ ] Add configurable optimizer model selection in the config UI so the default can be changed without code edits.
- [ ] Persist which model authored each autonomous run.
- [ ] Support:
  - primary model for proposal
  - secondary model for critique / veto
- [ ] Add per-model optimizer outcome tracking:
  - apply rate
  - rollback rate
  - kept-win rate
  - backlog usefulness

### Acceptance

- [ ] The system can later show which model produced the safest or most useful autonomous changes.

## Proposed Status Model

- [ ] `scheduled`
- [ ] `running`
- [ ] `insufficient_evidence`
- [ ] `no_change`
- [ ] `backlog_only`
- [ ] `blocked_by_gate`
- [ ] `auto_applied`
- [ ] `monitoring`
- [ ] `rolled_back`
- [ ] `kept`
- [ ] `paused`
- [ ] `failed`

## Suggested First Delivery Slice

If we want the safest useful first live version, the first slice should be:

- [ ] Bootstrap `GAMMA-RAY` from `Aggressive Altcoin Hunter`
- [ ] 12h autonomous review scheduler
- [ ] no-trade / inactivity intelligence
- [ ] automatic config + prompt patch apply with hard machine validation
- [ ] active auto-rollback
- [ ] optimizer run history
- [ ] scored missing-capability backlog
- [ ] proposer+critic model execution with `GPT-5.4` as the default pair

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
- [ ] Bootstrap flow live
- [ ] Autonomous scheduler live
- [ ] Auto-apply gate live
- [ ] Backlog scoring live
- [ ] Prompt evolution live
- [ ] Rollback control live

## Current Progress Notes

- [x] Backend persistence scaffolding is live for autonomous optimizer config, run history, and scored backlog items.
- [x] Per-trader optimizer defaults now initialize with `GPT-5.4` as proposer and critic, preferring the configured OpenAI account when available.
- [x] Backend APIs are live for:
  - get/update optimizer config
  - list optimizer runs
  - list/create backlog items
  - bootstrap a target trader from a source trader
- [ ] The scheduled autonomous review executor has not been wired yet.
- [ ] The auto-apply decision loop has not been wired yet.
- [ ] Prompt-diff generation/validation has not been wired yet.
- [ ] UI pages for this module are still pending.
