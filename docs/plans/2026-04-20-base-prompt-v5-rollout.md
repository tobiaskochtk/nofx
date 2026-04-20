# Base Prompt v5 Rollout

## Goal

Raise the shared trader base prompt so it better reflects the current NOFX stack:

- deal review and reasoning snapshots
- learned pattern analysis
- semantic memory
- optimizer-driven prompt and config iteration
- stronger same-symbol and churn-awareness
- better downstream auditability

The key design constraint is to improve the shared base layer without overwriting strategy-specific prompt sections that were already tuned manually or by the optimizer.

## Rollout Rules

- [x] Keep all existing strategy-specific `prompt_sections` intact.
- [x] Keep all trader-specific `custom_prompt` values intact.
- [x] Introduce a new versioned base prompt template instead of mutating old history in place.
- [x] Move current traders from `v4_2026` to `v5_2026`.
- [x] Change the default template for newly created traders to `v5_2026`.
- [x] Keep `v4_2026` on disk for rollback / comparison.

## What Changed In v5

- [x] stronger recycle-control and same-symbol memory discipline
- [x] clearer handling of churn, recent weak follow-through, and repeat-loss loops
- [x] tighter link between execution feasibility, slippage, and trade validity
- [x] explicit requirement for realistic stop/target placement
- [x] stronger emphasis on concise, stable reason codes for downstream review
- [x] explicit alignment with deal-review, learned-pattern, semantic-memory, and optimizer systems

## Non-Goals

- [x] no destructive overwrite of existing strategy prompt sections
- [x] no forced rewrite of optimizer-generated strategy patches
- [x] no change to per-strategy edge definitions such as aggressive altcoin, conservative majors, or relative-value logic

## Trader / Strategy Notes

- Active traders were all still on trader-level template `v4_2026` at the time of rollout.
- Trader-level custom prompts were empty, so the shared template version is the right rollout surface.
- Strategy-level differences remain in each strategy `prompt_sections` and continue to apply on top of the shared template.

## Follow-Up Recommendation

- [ ] Add a trader-prompt preview that shows the fully effective live prompt after template overlay plus strategy prompt sections plus trader custom prompt.
- [ ] Add a small UI badge that shows the active prompt template version per trader and per optimizer run.
