# GAMMA-RAY Arbitrage Signal Scope

## Goal

Upgrade `GAMMA-RAY` from an arbitrage-style relative-value trader into a genuinely stronger spread / carry / hedge-aware strategy without adding useless context.

Current live state:
- `GAMMA-RAY` is deployed on the former `BTC Grid Trader` account/trader.
- It trades a fixed liquid Bybit perpetual universe.
- It uses existing NOFX signals plus selfhosted AI500-compatible market context.
- It is **not** a true cross-exchange arbitrage engine yet.

## Current Constraints

What is still missing for real arbitrage quality:
- No explicit pair-spread or residual signal
- No hedge-ratio / beta-neutrality calculation
- No cross-exchange best-bid / best-ask comparison
- No fee-aware edge calculation
- No perp-vs-spot basis signal in the AI payload
- No funding differential carry signal between venues
- No portfolio-level neutrality guardrail for paired trades

## Priority

### P0: Must-Have Before Calling It Real Arbitrage

- [x] `f13_spread_residual`
  - Deliver rolling spread, z-score, residual, and mean-reversion half-life for selected liquid pairs.
  - Examples: `ETH/BTC beta-adjusted residual`, `SOL/ETH residual`, `DOGE/XRP residual`.
  - Why: without this, the AI cannot measure dislocation directly.

- [x] `f14_hedge_ratio`
  - Deliver rolling hedge ratio, correlation stability, and cointegration confidence.
  - Include lookback windows and confidence decay.
  - Why: paired trades without hedge math are directional bets wearing an arbitrage label.

- [x] `f15_fee_slippage_edge`
  - Deliver expected taker/maker fee cost, estimated slippage, and net edge after costs.
  - Include venue-specific cost assumptions for the exact tradable symbol.
  - Why: many apparent spreads disappear after fees.

- [x] `f16_basis_carry`
  - Deliver perp-vs-index / perp-vs-spot basis, basis z-score, and funding carry expectation.
  - Include short-horizon and 8h context.
  - Why: this is the core signal for carry and basis-style arbitrage.

### P1: Strongly Improves Decision Quality

- [ ] `f17_cross_exchange_top_of_book`
  - Deliver best bid/ask across enabled venues for the same symbol, plus spread capture after fees.
  - Include quote timestamp freshness.
  - Why: enables actual cross-venue execution decisions instead of single-venue inference.

- [ ] `f18_pair_liquidity_quality`
  - Deliver paired liquidity score: depth symmetry, top-of-book size, spread stability, and execution quality for both legs.
  - Why: one weak leg breaks the whole trade.

- [ ] `f19_crowding_unwind`
  - Deliver crowding score combining funding extremes, OI velocity, liquidation pressure, and one-sided flow.
  - Why: helps distinguish true edge from crowded traps.

- [ ] `f20_portfolio_neutrality`
  - Deliver current portfolio beta, long/short notional imbalance, concentration, and post-trade neutrality estimate.
  - Why: relative-value should manage portfolio exposure, not only single symbols.

### P2: Useful Refinements

- [ ] `f21_session_regime`
  - Deliver session-aware volatility/liquidity regime for Asia / Europe / US overlaps.
  - Why: arbitrage quality is time-of-day sensitive.

- [ ] `f22_event_risk_blockers`
  - Deliver listing, unlock, macro, funding window, and exchange-maintenance blockers.
  - Why: these invalidate mean-reversion and spread assumptions.

- [ ] `f23_execution_path_confidence`
  - Deliver confidence that both legs can be entered and exited inside a defined time/impact budget.
  - Why: protects against trapped partial fills.

## Payload Guidance

Do not dump raw matrices into the prompt.

Preferred payload style:
- compact per-pair summary
- top 3 strongest spread candidates
- top 3 strongest carry candidates
- explicit freshness and cost-adjusted edge
- portfolio-neutrality estimate before and after trade

## Suggested First Implementation Order

1. `f13_spread_residual`
2. `f14_hedge_ratio`
3. `f16_basis_carry`
4. `f15_fee_slippage_edge`
5. `f20_portfolio_neutrality`
6. `f17_cross_exchange_top_of_book`

## Success Criteria

The arbitrage stack is good enough when:
- the AI can explain **why** a spread is statistically stretched
- the AI sees the required hedge ratio explicitly
- expected edge is shown **after** fees and slippage
- paired trades can be evaluated at portfolio level
- cross-venue and carry trades are measurable rather than inferred
