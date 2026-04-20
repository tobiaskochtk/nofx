# Decision Signal Scope

## Current live baseline

The compact payload now contains and is live in trading:

- account state
- open positions
- candidate list
- `ctx` snapshot per symbol
- `f4` orderflow
- `f5` risk/liquidation context
- `f6` levels/AVWAP context
- `f7` volatility regime
- `f8` quant flow
- `f9` market leadership
- `f10` execution quality
- `f11` volume participation
- `f12` symbol memory
- `f13` venue tradability
- `f14` feature availability
- `f15` relative strength vs BTC
- compact multi-timeframe summaries per symbol
- BTC benchmark regime block
- trader performance summary
- recent trade memory
- candidate source tags

## Live probe notes

Forced live decision after deploy on `2026-04-03` produced record `3310`.

Observed:

- backend health was `200`
- trader `2026` auto-started and ran a fresh cycle immediately after deploy
- payload size increased again:
  - prior verified live record `3306`: `input_prompt=8155`
  - current live record `3310`: `input_prompt=8878`
- the new blocks were present in the real payload:
  - `venue_tradability`
  - `feature_availability`
  - `relative_strength`
- the model explicitly used the new signals in reasoning:
  - `FETUSDT`: "Venue unsupported (cannot trade)"
  - `FETUSDT`: "Strong outperformance vs BTC"
  - `MUSDT`: "Relative strength vs BTC negative"
  - both symbols: stale-data awareness
- output contract stayed clean:
  - `<reasoning>` and `<decision>` both present
  - decision envelope was valid JSON with `decisions: []`

Concrete live examples from `3310`:

- `MUSDT`
  - `venue_tradability.supported=true`
  - `execution_quality.liq_score=0.708`
  - `feature_availability.f10=ok`
  - `relative_strength.state=neutral`
- `FETUSDT`
  - `venue_tradability.supported=false`
  - `venue_tradability.book=venue_unsupported`
  - `feature_availability.f10=venue_unsupported`
  - `relative_strength.state=outperform`

## What still matters most

The latest live cycle also made the remaining gaps clearer:

- core derivatives context is still often missing:
  - `ctx.oi_d1h_pct=null`
  - `ctx.fund_bps=null`
  - `ctx.basis_pct=null`
  - `ctx.src.oi/fund/basis=unknown`
- `f4-f7` were all `missing` for both live candidates
- `score` and `confidence` still arrive as opaque scalars
- cross-timeframe conflict is visible, but there is still no compact "fresh trend vs exhausted move" signal

## Priority

### `P0` Must add next

| Fn | Name | Why it matters | Source | Payload shape |
|---|---|---|---|---|
| `f16` | `context_source_health` | The live cycle showed that core `ctx` derivatives fields were missing on both candidates and even BTC. The AI can see `null`, but not whether that means stale source, unsupported symbol, or collection failure. | `ctx.src.*`, feature freshness, provider status | Per candidate/position block |

### `P1` Very useful

| Fn | Name | Why it matters | Source | Payload shape |
|---|---|---|---|---|
| `f17` | `confidence_decomposition` | Live reasoning still leaned heavily on low `score/confidence`, but those numbers are opaque. The AI still has to infer whether weakness came from trend conflict, missing microstructure, or poor data quality. | existing score components + QoS + execution state | Per candidate block |
| `f18` | `trend_persistence` | `MUSDT` had a classic short-term-down / long-term-up conflict. We still need a compact measure for fresh impulse vs late extension/exhaustion. | current MTF summaries + rolling returns | Per candidate block |

### `P2` Nice to have

| Fn | Name | Why it matters | Source | Payload shape |
|---|---|---|---|---|
| `f19` | `source_rank_context` | Source tags are present, but the AI still does not know whether a candidate is top-ranked in AI500/OI selection or just part of a shallow tail. Helpful when the pool is small. | candidate order in source pools | Per candidate block |

## Recommended definitions

### `f16_context_source_health`

Scope:

- per symbol
- only for the core `ctx` derivatives fields
- no verbose diagnostics

Recommended fields:

- `oi_state`
- `funding_state`
- `basis_state`
- `snapshot_state`

Reason:

- the live payload still had missing core context on both candidates and benchmark
- this is now the biggest remaining ambiguity in what the AI actually knows

### `f17_confidence_decomposition`

Scope:

- per candidate
- derived only from existing score inputs
- should stay compact and numeric/categorical

Recommended fields:

- `trend_component`
- `regime_component`
- `execution_component`
- `data_quality_component`

Reason:

- the model already respects low confidence
- it still cannot see why confidence is low without reverse-engineering multiple blocks

### `f18_trend_persistence`

Scope:

- per candidate
- primary timeframe plus confirmation frames only
- derived from existing MTF summaries

Recommended fields:

- `trend_consistency`
- `pullback_depth_pct`
- `late_extension_flag`
- `tf_conflict_flag`

Reason:

- helps the AI distinguish "good pullback in trend" from "late chase" or "timeframe conflict"
- especially relevant after the live `MUSDT` example

### `f19_source_rank_context`

Scope:

- per candidate
- source-selection metadata only
- no extra market fetches

Recommended fields:

- `ai500_rank`
- `oi_rank`
- `source_priority`

Reason:

- current `src_tags` tell the AI where the candidate came from
- they do not tell the AI how strong that source endorsement actually is

## Explicitly out of scope

Do not add these to the compact payload unless we move to a different prompt budget:

- raw OHLCV arrays
- full ranking tables
- all quant durations
- verbose natural-language summaries generated in code
- deep order book ladders

The compact payload should stay feature-dense, not text-heavy.
