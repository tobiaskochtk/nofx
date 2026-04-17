# NOFX Indicator Scope Document

> **Version:** 1.0  
> **Date:** April 16, 2026  
> **Purpose:** Complete inventory of all trading indicators — currently implemented, user-requested, and newly discovered from industry research — with ratings, data sources, and implementation recommendations.

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Currently Implemented Indicators](#2-currently-implemented-indicators)
3. [Missing Indicators — From User Research](#3-missing-indicators--from-user-research)
4. [Missing Indicators — From Internet/Industry Research](#4-missing-indicators--from-internetindustry-research)
5. [Complete Master List (All Indicators)](#5-complete-master-list-all-indicators)
6. [Priority Implementation Roadmap](#6-priority-implementation-roadmap)
7. [Data Source Mapping](#7-data-source-mapping)
8. [Architecture Recommendations](#8-architecture-recommendations)
9. [Appendix: Indicator Reference Links](#9-appendix-indicator-reference-links)

---

## 1. Executive Summary

### Current State
NOFX currently implements **~40 indicators** across 4 categories:
- **7 basic technical indicators** (EMA, MACD, RSI, ATR, Bollinger, Volume, Donchian)
- **~25 derivatives/microstructure features** (F4-F7: CVD, Liquidation, AVWAP, Volatility/Squeeze)
- **~8 macro/flow indicators** (OI, Funding Rate, Basis, NetFlow, AI500)

### Gap Analysis
The system is **strong on short-term microstructure** (orderflow, liquidations, volatility squeezes) but **critically lacks**:
- **On-chain cycle indicators** (MVRV, NUPL, SOPR, Realized Price, HODL Waves)
- **Sentiment data** (Fear & Greed, Google Trends)
- **Market structure indicators** (BTC Dominance, Altcoin Season Index, Total3)
- **Macro liquidity indicators** (Global M2, FED Rate Expectations, Stablecoin Supply Ratio)
- **Miner health indicators** (Hash Ribbons, Puell Multiple)
- **Pricing models** (Terminal Price, Top Cap, Delta Top, Power Law)

### Impact
Adding the top-priority missing indicators would transform NOFX from a **short-term derivatives scalper** into a **cycle-aware, macro-informed** trading system capable of:
- Identifying cycle tops/bottoms with high confidence
- Scaling position sizes based on macro risk
- Timing altcoin rotation accurately
- Avoiding trades during unfavorable macro environments

---

## 2. Currently Implemented Indicators

### 2.1 Basic Technical Indicators

| ID | Indicator | Periods | File | Rating | Usage |
|----|-----------|---------|------|--------|-------|
| T01 | **EMA** (Exponential Moving Average) | 20, 50 | `market/data_indicators.go` | ⭐⭐⭐⭐ | Trend direction across timeframes |
| T02 | **MACD** | 12/26/9 | `market/data_indicators.go` | ⭐⭐⭐⭐ | Momentum + crossover signals |
| T03 | **RSI** (Relative Strength Index) | 7, 14 | `market/data_indicators.go` | ⭐⭐⭐⭐⭐ | Overbought/oversold (>70/<30) |
| T04 | **ATR** (Average True Range) | 14, 3 | `market/data_indicators.go` | ⭐⭐⭐⭐⭐ | Volatility, stop-loss sizing, position sizing |
| T05 | **Bollinger Bands** | 20 (2 std) | `market/data_indicators.go` | ⭐⭐⭐⭐ | Volatility bands, squeeze detection |
| T06 | **Volume** | N/A | `market/types.go` | ⭐⭐⭐⭐ | Volume analysis, confirmation signals |
| T07 | **Donchian Channel** | 72h/240h/500h | `market/data_indicators.go` | ⭐⭐⭐⭐ | Support/resistance, breakout levels |

**Configuration:** `store/strategy.go` — `IndicatorConfig` struct with enable/disable toggles and configurable periods.

### 2.2 Derivatives Microstructure Features (F4–F7)

#### F4 — Orderflow Microstructure (CVD/Taker Analysis)
| ID | Metric | Timeframes | File | Rating |
|----|--------|------------|------|--------|
| F4a | **CVD Z-score** (Cumulative Volume Delta) | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐⭐ |
| F4b | **Order Imbalance Z-score** | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐⭐ |
| F4c | **Taker Buy Ratio (TBR)** | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐ |
| F4d | **Slope** (Price/CVD/Imbalance) | 3m | `pkg/types/features.go` | ⭐⭐⭐⭐ |
| F4e | **Divergence Signals** (Bull/Bear) | 3m | `pkg/types/features.go` | ⭐⭐⭐⭐⭐ |
| F4f | **Confidence CVD** | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐ |

#### F5 — Liquidation Risk Heatmap
| ID | Metric | Timeframes | File | Rating |
|----|--------|------------|------|--------|
| F5a | **Distance to Liquidation** (Up/Down ATR) | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐⭐ |
| F5b | **Liquidation Risk Level** (1-10) | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐ |
| F5c | **Cluster Strength** | 3m | `pkg/types/features.go` | ⭐⭐⭐⭐ |
| F5d | **Preferred Direction** | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐ |

#### F6 — Anchored VWAP (Structure/Levels)
| ID | Metric | Timeframes | File | Rating |
|----|--------|------------|------|--------|
| F6a | **AVWAP Distance** (ATR units) | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐⭐ |
| F6b | **AVWAP Bias** (long/short/neutral) | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐ |
| F6c | **Reclaim/Rejection Flags** | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐ |
| F6d | **Confluence Count** | 3m | `pkg/types/features.go` | ⭐⭐⭐⭐ |

#### F7 — Volatility & Squeeze
| ID | Metric | Timeframes | File | Rating |
|----|--------|------------|------|--------|
| F7a | **BBW** (Bollinger Band Width) | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐⭐ |
| F7b | **BBW Percentile Rank** (0-100) | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐ |
| F7c | **Squeeze On/Release** | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐⭐ |
| F7d | **Realized Vol / RV Ratio** | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐ |
| F7e | **Volatility Regime** | 3m, 15m | `pkg/types/features.go` | ⭐⭐⭐⭐ |

### 2.3 Exchange & Flow Indicators

| ID | Indicator | Source | File | Rating |
|----|-----------|--------|------|--------|
| E01 | **OI Delta (1h)** | Real-time perp data | `provider/nofxos/oi.go` | ⭐⭐⭐⭐⭐ |
| E02 | **OI Z-score (7d)** | Historical OI | `provider/nofxos/oi.go` | ⭐⭐⭐⭐ |
| E03 | **OI/Price Divergence** | OI vs price correlation | `provider/nofxos/oi.go` | ⭐⭐⭐⭐⭐ |
| E04 | **OI/Price Correlation (24h)** | Pearson correlation | `provider/nofxos/oi.go` | ⭐⭐⭐⭐ |
| E05 | **Funding Rate + Z-score (7d)** | Exchange data | `provider/nofxos/ai500.go` | ⭐⭐⭐⭐⭐ |
| E06 | **Funding Dispersion** | Std deviation | `provider/nofxos/ai500.go` | ⭐⭐⭐⭐ |
| E07 | **Basis % + Z-score (14d)** | Spot vs Perp | Decision payloads | ⭐⭐⭐⭐ |
| E08 | **Institution Fund In/Outflow** | Futures flow | `provider/nofxos/netflow.go` | ⭐⭐⭐⭐⭐ |
| E09 | **Retail Fund In/Outflow** | Futures flow | `provider/nofxos/netflow.go` | ⭐⭐⭐⭐ |
| E10 | **AI500 Score + Bucket** | Proprietary | `provider/nofxos/ai500.go` | ⭐⭐⭐⭐ |

### 2.4 Market Rankings

| ID | Indicator | Timeframes | File | Rating |
|----|-----------|------------|------|--------|
| R01 | **OI Top/Low Rankings** | 1h, 4h, 24h | `provider/nofxos/oi.go` | ⭐⭐⭐⭐ |
| R02 | **NetFlow Rankings** | 1h, 4h, 24h | `provider/nofxos/netflow.go` | ⭐⭐⭐⭐ |
| R03 | **Price Leaders/Losers** | 1h, 4h, 24h | `store/strategy.go` | ⭐⭐⭐ |

---

## 3. Missing Indicators — From User Research

These indicators were provided in the user's comprehensive indicator list but are **NOT implemented** in the codebase.

### 3.1 Cycle & Macro Indicators

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| M01 | **Bitcoin AHR999 Index** | Timing strategy for BTC accumulation. Measures short-term returns and deviation from expected valuation. | <0.45 = strong buy, 0.45-1.2 = DCA zone, >1.2 = wait | ⭐⭐⭐ | Medium |
| M02 | **Pi Cycle Top Indicator** | EMA111 crossing EMA350×2 from below. Has historically called every BTC cycle top within 3 days. | EMA111 > EMA350×2 = cycle top signal | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| M03 | **Puell Multiple** | Daily coin issuance value / 365-day MA of daily issuance value. Shows miner selling pressure. | <0.5 = buy zone (miner capitulation), >4 = sell zone | ⭐⭐⭐⭐ | High |
| M04 | **Bitcoin Rainbow Chart** | Logarithmic growth curve with color bands. Shows potential market stages. | Color bands: blue=fire sale → red=maximum bubble territory | ⭐⭐⭐ | Low |
| M05 | **2-Year MA Multiplier** | BTC price relative to 2Y MA and 2Y MA ×5. Golden Ratio Multiplier variant. | Below 2Y MA = buy, above 2Y MA ×5 = sell | ⭐⭐⭐⭐ | High |
| M06 | **MVRV Z-Score** | Market Value vs Realized Value, z-normalized. Best single cycle top/bottom indicator. | Z>7 = market top, Z<0 = generational buy | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| M07 | **Bitcoin Bubble Index** | Composite: price info + 60-day cumulative increase + hot keywords + bubble index. | High composite = bubble territory | ⭐⭐⭐ | Low |
| M08 | **NUPL (Net Unrealized Profit/Loss)** | Difference between unrealized profits and losses / total market cap. | >0.75 = euphoria (red), 0.5-0.75 = belief/denial (yellow), <0 = capitulation | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| M09 | **Bitcoin RHODL Ratio** | Relative wealth distribution between 1-week holders and 1-2 year holders via Realized Value HODL. | Spikes at cycle tops when short-term holders dominate | ⭐⭐⭐⭐ | Medium |
| M10 | **Bitcoin Macro Oscillator (BMO)** | Composite: MVRV Ratio + VWAP Ratio + CVDD Ratio + Sharpe Ratio. | Composite signal: 0-100 scale | ⭐⭐⭐⭐ | Medium |
| M11 | **Bitcoin MVRV Ratio** (raw) | Market Cap / Realized Cap. Raw version without Z-normalization. | >3.5 = overvalued, <1 = undervalued | ⭐⭐⭐⭐ | High (if M06 added, less critical) |
| M12 | **Bitcoin 4-Year Moving Average** | Long-term trend floor. BTC rarely trades below its 4Y MA. | Price < 4Y MA = extreme buy zone | ⭐⭐⭐⭐ | Medium |
| M13 | **CBBI (Bull Run Index)** | Composite of 9 metrics (Pi Cycle, NUPL, RHODL, Puell, 2Y MA, Rainbow, MVRV, Reserve Risk, CVDD). Colin Talks Crypto. | >90 = cycle top zone, <10 = cycle bottom zone | ⭐⭐⭐⭐ | High |
| M14 | **Mayer Multiple** | BTC Price / 200-Day MA. Simple but effective overbought/oversold indicator. | >2.4 = historically overbought, <0.8 = oversold | ⭐⭐⭐⭐ | High |
| M15 | **AHR999x Top Escape Indicator** | Complement to AHR999 focused on sell signals. | Signals when to escape/sell position | ⭐⭐⭐ | Medium |
| M16 | **Bitcoin Reserve Risk** | Long-term holder confidence vs. price. Based on HODL bank (accumulated opportunity cost of holding). | Green zone = accumulate, Red zone = distribute | ⭐⭐⭐⭐ | High |
| M17 | **Bitcoin Terminal Price** | On-chain pricing model for cycle top identification. Uses Coin Days Destroyed weighted price. | Price approaches Terminal Price = extreme cycle top | ⭐⭐⭐⭐ | Medium |
| M18 | **The Golden Ratio Multiplier** | Bitcoin 350DMA × specific Fibonacci multipliers. Explores adoption curve. | Specific multiplier levels as S/R | ⭐⭐⭐⭐ | Medium |
| M19 | **Bitcoin Trend Indicator** | Dynamic momentum signal showing presence, direction, strength of BTC momentum. | Direction + strength → trend confidence | ⭐⭐⭐ | Low |
| M20 | **3-Month Annualized Ratio** | BTC 3-month annualized % change. Peaks at market highs. | Extreme spikes = market high warning | ⭐⭐⭐ | Low |
| M21 | **BTC Top Cap Model** | Market cycle tops model using Delta Cap × 35. | Price near Top Cap = cycle top | ⭐⭐⭐⭐ | Medium |

### 3.2 Sentiment & Market Breadth Indicators

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| S01 | **Fear & Greed Index** | Composite sentiment: volatility, market momentum, social media, surveys, dominance, trends. | <20 = extreme fear (contrarian buy), >80 = extreme greed (risk mgmt) | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| S02 | **Altcoin Season Index** | % of top-50 coins outperforming BTC over 90 days. | >75% = altcoin season, <25% = BTC season | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| S03 | **Bitcoin Dominance** | BTC market cap / total crypto market cap. | >70% = BTC dominance peak (cycle top warning), declining = altseason starts | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| S04 | **Google Trend "Bitcoin"** | Search interest for "Bitcoin" on Google. Retail frenzy indicator. | Spikes at cycle tops. Leading indicator for mania phase. | ⭐⭐⭐ | Medium |
| S05 | **USDT Flexible Savings Rate** | Binance USDT margin borrow interest rate. Demand-for-leverage proxy. | High rates = excessive leverage demand | ⭐⭐⭐ | Low |
| S06 | **Crypto RSI Heatmap** | Visualization of RSI across multiple crypto assets and timeframes. | Bulk overbought/oversold identification | ⭐⭐⭐ | Medium |
| S07 | **Liquidation Heatmap** (CoinGlass) | Macro-level liquidation levels across all major exchanges. | Magnetic price targets, cascade risk zones | ⭐⭐⭐⭐ | Medium |

### 3.3 Market Structure & Rotation Indicators

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| R04 | **Total3 (Altcoin Market Cap excl. BTC & ETH)** | Market cap of all altcoins minus BTC and ETH. | Rising Total3 + falling BTC.D = altcoin season. Death cross of Total3/BTC.D = altcoin top | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| R05 | **Ethereum Market Cap** | ETH market cap as altseason top proxy. | Peak at >$600B historically (user rule) | ⭐⭐⭐⭐ | Medium |
| R06 | **Ethereum Market Dominance** | ETH market cap / total crypto market cap. | Sell signal at >20% (user rule) | ⭐⭐⭐⭐ | Medium |
| R07 | **Bitcoin Market Cap** | BTC market cap for macro top identification. | Bullrun top signals at levels >$2T (historical context) | ⭐⭐⭐ | Low |
| R08 | **Total Crypto Market Cap** (Global) | Sum of all crypto market capitalizations. | Broadest liquidity measure | ⭐⭐⭐⭐ | Medium |
| R09 | **Crypto Daily Volume / Market Cap Share** | Daily volume as % of total market cap. | Volume > market cap = parabolas often in final stage (Crypto Banter thesis) | ⭐⭐⭐ | Medium |
| R10 | **BTC Dominance Change** | Rate of change of BTC dominance. | Acceleration/deceleration of rotation | ⭐⭐⭐ | Low |
| R11 | **DeFi Volume** | Total DeFi protocol volume. | Sector rotation indicator | ⭐⭐⭐ | Low |
| R12 | **ETH vs BTC Relative Strength** | Whether ETH rises significantly stronger than BTC. | ETH >> BTC = BTC ATH signal, altseason start | ⭐⭐⭐⭐ | Medium |

### 3.4 On-Chain Holder Behavior

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| H01 | **Bitcoin LTH Supply** | Total BTC held by entities holding >155 days. | LTH distribution (declining) precedes cycle tops | ⭐⭐⭐⭐ | High |
| H02 | **Bitcoin STH Supply** | Total BTC held by entities holding <155 days. | STH panic selling = bottom signal | ⭐⭐⭐⭐ | High |
| H03 | **Whale Sell Activity** | Large transaction volumes moving to exchanges. | Spike in whale deposits = sell pressure warning | ⭐⭐⭐⭐ | Medium |

### 3.5 Macro & Institutional Indicators

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| L01 | **Global Liquidity Index** | Global M2 money supply + central bank balance sheets. Crypto correlates ~0.85 lagged 3 months. | Rising M2 = bullish macro. >$20T milestone = potential this cycle | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| L02 | **FED Rate Expectations** | CME FedWatch implied rate probabilities. | 2-3 pending cuts = high liquidity inflows expected | ⭐⭐⭐⭐ | High |
| L03 | **ETF Net Outflows** | Bitcoin ETF daily net flow data. Multi-day outflows precede corrections. | Consistent outflows = institutional selling pressure | ⭐⭐⭐⭐ | Medium |
| L04 | **ETF-to-BTC Ratio** | Institutional vs retail investment ratio via ETF flows. | Growing ratio = mainstream adoption accelerating | ⭐⭐⭐ | Medium |
| L05 | **MicroStrategy Avg BTC Cost** | Institutional floor proxy. | BTC < MSTR cost = extreme risk-off | ⭐⭐⭐ | Low |
| L06 | **US Inflation Rate (CPI)** | Consumer Price Index. Impacts Fed policy → liquidity → crypto. | Falling CPI = more room for cuts = bullish | ⭐⭐⭐ | Low |

### 3.6 Coin-Specific Indicators

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| C01 | **Token Unlocks** | Scheduled vesting releases creating sell pressure. | Large upcoming unlock = price suppression risk | ⭐⭐⭐⭐ | High |
| C02 | **In/Outflows per Coin** | Historical buy/sell volumes at major exchanges. | Net outflows = accumulation, inflows = distribution | ⭐⭐⭐⭐ | Partially covered by NetFlow |
| C03 | **Historical Monthly Performance** | Seasonal patterns per coin per month. | Statistical edge from seasonality | ⭐⭐⭐ | Low |
| C04 | **Fibonacci Retracement Levels** | Price retracement zones based on Fibonacci ratios. | Key S/R levels at 0.236, 0.382, 0.5, 0.618, 0.786 | ⭐⭐⭐ | Low (partially covered by AVWAP/Donchian) |
| C05 | **Chart Patterns** | Head-shoulders, pennants, cup-and-handle, etc. | Pattern recognition for future price moves | ⭐⭐⭐ | Low (complex to automate) |
| C06 | **Chande Momentum Oscillator** | ROC-based momentum. >75 or line crossing = signal. | Divergence detection, momentum extremes | ⭐⭐⭐ | Low (RSI covers most) |
| C07 | **RSI 22-Day** | Medium-term RSI complementing 7/14 periods. | Weekly-scale momentum analysis | ⭐⭐⭐⭐ | Medium (easy add) |

### 3.7 Proprietary/Community Indicators

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| P01 | **TJD Jewel Thief** | BD community indicator for exit strategies. | Coin ATH / cycle top timing | ⭐⭐⭐ | Unknown (proprietary) |
| P02 | **VWAP 3.5** | BD community indicator for exit strategies. | Coin ATH / cycle top timing | ⭐⭐⭐ | Unknown (proprietary) |
| P03 | **Market Cipher** | Proprietary TradingView indicator suite. | Multiple confluence signals | N/A | **SKIP** (closed/proprietary, not API accessible) |
| P04 | **Smithson's Forecast** | Smithson Investment Trust stock forecast. | N/A | N/A | **SKIP** (not crypto relevant) |

---

## 4. Missing Indicators — From Internet/Industry Research

These indicators were identified through research of Glassnode, CryptoQuant, Bitcoin Magazine Pro, CoinGlass, and industry best practices. They were **NOT** in the user's original list nor in the codebase.

### 4.1 Critical On-Chain Indicators (Glassnode / CryptoQuant / Bitcoin Magazine Pro)

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| N01 | **SOPR (Spent Output Profit Ratio)** | Tracks profit margin of all moved bitcoins. SOPR > 1 = coins moved at profit. | SOPR < 1 in bull market = buy dip. SOPR resetting to 1 = support in uptrends | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| N02 | **STH-SOPR** (Short-Term Holder SOPR) | SOPR filtered to only short-term holders (<155d). Most reactive version. | STH-SOPR < 1 in bull = local bottom. Sustained < 1 = bear confirmed | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| N03 | **LTH-SOPR** (Long-Term Holder SOPR) | SOPR filtered to long-term holders. Shows when veterans sell at profit/loss. | LTH-SOPR spike = long-term holders distributing = late cycle warning | ⭐⭐⭐⭐ | High |
| N04 | **NVT Signal (Advanced)** | Network Value to Transactions ratio with moving average smoothing. Bitcoin's "PE ratio." | NVT > 150 = overvalued network, NVT < 45 = undervalued | ⭐⭐⭐⭐ | High |
| N05 | **Realized Price** | Average cost basis of all bitcoins (UTXO-weighted). The aggregate "break-even" price. | BTC < Realized Price = capitulation zone (rare, historically best buy zone) | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| N06 | **STH Realized Price** | Average cost basis of short-term holders only. Most dynamic cost basis. | Key support in bull markets. Break below = trend change warning | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| N07 | **LTH Realized Price** | Average cost basis of long-term holders only. Deep cycle floor. | Price near LTH RP = extreme value zone | ⭐⭐⭐⭐ | High |
| N08 | **STH MVRV** | MVRV ratio for short-term holders only. Most sensitive cycle indicator. | STH MVRV < 1 = STH underwater (panic), > 1.4 = STH in significant profit (distribution risk) | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| N09 | **LTH MVRV** | MVRV ratio for long-term holders only. Shows veteran holder profit levels. | LTH MVRV > 3.5 = extreme profit-taking territory | ⭐⭐⭐⭐ | High |
| N10 | **Stablecoin Supply Ratio (SSR)** | BTC market cap / total stablecoin market cap. Measures "dry powder." | Low SSR = high stablecoin supply relative to BTC = buying power available = bullish | ⭐⭐⭐⭐⭐ | **CRITICAL** |
| N11 | **Exchange Reserve (BTC)** | Total BTC held on exchange wallets. | Declining reserve = coins moving to cold storage = bullish. Rising = selling pressure | ⭐⭐⭐⭐ | High |
| N12 | **Exchange Netflow (On-chain)** | Net BTC flowing in/out of exchanges (on-chain, not just futures). | Sustained negative netflow = accumulation. Positive spikes = sell-off risk | ⭐⭐⭐⭐ | High (complements existing futures netflow) |
| N13 | **% Addresses in Profit** | Percentage of all BTC addresses that are "in the money." | >95% in profit = euphoria zone, historically precedes corrections | ⭐⭐⭐⭐ | High |

### 4.2 Mining Health Indicators

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| N14 | **Hash Ribbons** | 30D MA and 60D MA of hashrate. Capitulation when 30D < 60D, recovery when 30D crosses back above. | Hash Ribbon buy signal after capitulation = historically one of the best buy signals | ⭐⭐⭐⭐⭐ | **HIGH** |
| N15 | **Bitcoin Hashrate** | Total network hashrate. Growing hashrate = healthy network, miner confidence. | Declining hashrate = miner stress, potential forced selling | ⭐⭐⭐ | Low |
| N16 | **Hashprice** | Revenue per unit of hashrate. Measures miner profitability directly. | Low hashprice = miner capitulation risk | ⭐⭐⭐ | Low |

### 4.3 HODL Wave / Supply Distribution Indicators

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| N17 | **HODL Waves** | Age distribution of all UTXOs in bands (1d, 1w, 1m, 3m, 6m, 1y, 2y, 3y, 5y+). | Young coins expanding = distribution phase. Old coins expanding = accumulation | ⭐⭐⭐⭐ | Medium |
| N18 | **Realized Cap HODL Waves** | Same as HODL Waves but weighted by realized price at time of acquisition. | More accurate than raw HODL waves for value flow analysis | ⭐⭐⭐⭐ | Medium |
| N19 | **Coin Days Destroyed (CDD)** | Number of coins × days since last moved. Spikes = old coins moving. | Major CDD spikes during price rises = smart money distributing | ⭐⭐⭐⭐ | Medium |
| N20 | **Value Days Destroyed (VDD) Multiple** | CDD weighted by price, compared to yearly average. Identifies cycle highs. | VDD Multiple > 2.0 = high spending velocity = cycle top zone | ⭐⭐⭐⭐ | High |
| N21 | **Whale Shadows (Revived Supply)** | Large amounts of old coins moving again. Tracks old-money movement. | Spikes in revived supply = potential large sell-off incoming | ⭐⭐⭐⭐ | Medium |

### 4.4 Pricing Models

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| N22 | **Top Cap** | Average Cap × 35. Pricing model for bull market highs. | Price approaching Top Cap = extreme cycle top zone | ⭐⭐⭐⭐ | Medium |
| N23 | **Delta Top** | Realized Cap + (Realized Cap - Average Cap). Alternative top model. | Price approaching Delta Top = cycle top zone | ⭐⭐⭐⭐ | Medium |
| N24 | **CVDD (Cumulated Value-Days Destroyed)** | Pricing model for bear market lows. | Price near CVDD = bear market floor | ⭐⭐⭐ | Low |
| N25 | **Balanced Price** | Pricing model combining Realized Price and Transferred Price. | Bear market floor identification | ⭐⭐⭐ | Low |
| N26 | **Power Law (Linear Regression)** | Log-log regression of BTC price over time. Long-term fair value corridor. | Deviation from power law band = over/undervalued | ⭐⭐⭐⭐ | Medium |
| N27 | **Stock-to-Flow (S2F)** | Scarcity model: existing supply / annual production. Debated but influential. | Provides long-term price target based on post-halving scarcity | ⭐⭐⭐ | Low (less reliable post-2022) |
| N28 | **200-Week MA Heatmap** | Color-coded price relative to 200-week moving average. | Buy when price near 200W MA. Color = rate of change | ⭐⭐⭐⭐ | Medium |

### 4.5 Network Activity Indicators

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| N29 | **Bitcoin Active Addresses** | Unique daily active addresses (send + receive). Network adoption proxy. | Growing active addresses + price rise = healthy rally. Divergence = weakness | ⭐⭐⭐⭐ | Medium |
| N30 | **AASI (Active Address Sentiment)** | Compares change in price vs change in active addresses. Short-term valuation. | Price rising faster than addresses = overbought. Vice versa = oversold | ⭐⭐⭐⭐ | Medium |
| N31 | **New Addresses** | First-time addresses appearing on the Bitcoin network. | Spike in new addresses = new retail participants entering | ⭐⭐⭐ | Low |
| N32 | **Everything Indicator** (Bitcoin Magazine Pro) | New composite indicator consolidating multiple on-chain metrics into single score. | Single composite market overview score | ⭐⭐⭐⭐ | Medium |

### 4.6 Narrative/Sector Rotation Indicators

| ID | Indicator | Description | Signal Logic | Rating | Priority |
|----|-----------|-------------|--------------|--------|----------|
| N33 | **Crypto Narrative Index** (Dune/CryptoKoryo) | Tracks performance of 30+ crypto narratives (AI, DeFi, GameFi, RWA, Meme, LSD, etc). | Identifies which narrative is outperforming/rotating. Hot narrative = alpha source | ⭐⭐⭐⭐ | Medium |
| N34 | **DePIN/AI/RWA Sector Trends** (Google Trends) | Relative search interest for emerging crypto sectors. | Rising narrative interest = early entry opportunity | ⭐⭐⭐ | Low |

---

## 5. Complete Master List (All Indicators)

### Summary by Category

| Category | Implemented | Missing (User List) | Missing (Research) | Total |
|----------|-------------|--------------------|--------------------|-------|
| Basic Technical | 7 | 1 (RSI-22) | 0 | 8 |
| Derivatives/Microstructure (F4-F7) | ~25 | 0 | 0 | ~25 |
| Exchange & Flow | 10 | 0 | 3 (SSR, ExReserve, On-chain Netflow) | 13 |
| Market Rankings | 3 | 0 | 0 | 3 |
| Cycle/Macro Indicators | 0 | 21 | 0 | 21 |
| Sentiment/Market Breadth | 0 | 7 | 0 | 7 |
| Market Structure/Rotation | 0 | 12 | 1 (Narrative) | 13 |
| On-Chain Holder Behavior | 0 | 3 | 13 (SOPR, NVT, etc.) | 16 |
| Mining Health | 0 | 0 | 3 | 3 |
| HODL Waves/Supply | 0 | 0 | 5 | 5 |
| Pricing Models | 0 | 0 | 7 | 7 |
| Network Activity | 0 | 0 | 4 | 4 |
| Macro/Institutional | 0 | 6 | 0 | 6 |
| Coin-Specific | 0 | 7 | 0 | 7 |
| Proprietary/Community | 0 | 4 (2 skip) | 0 | 4 |
| **TOTAL** | **~45** | **~61** | **~36** | **~142** |

### Rating Distribution (Missing Indicators Only)

| Rating | Count | Description |
|--------|-------|-------------|
| ⭐⭐⭐⭐⭐ | **16** | Must-have, transformative for decision quality |
| ⭐⭐⭐⭐ | **28** | Strong value-add, clear improvement |
| ⭐⭐⭐ | **25** | Nice-to-have, situational value |
| SKIP | **3** | Not applicable or not accessible |
| Unknown | **2** | Proprietary, cannot evaluate |

---

## 6. Priority Implementation Roadmap

### Phase 1: CRITICAL — Must-Have (16 indicators)
**Impact:** Transforms system from short-term scalper to cycle-aware engine.  
**Estimated effort:** Medium — mostly API integrations + simple calculations.

| # | Indicator | Source API | Effort | Justification |
|---|-----------|-----------|--------|---------------|
| 1 | **Fear & Greed Index** (S01) | alternative.me (free) | Easy | Zero sentiment data today. Single API call. |
| 2 | **Bitcoin Dominance** (S03) | CoinGecko / CMC | Easy | Essential for altcoin rotation — core to trading strategy |
| 3 | **Total3 Altcoin Market Cap** (R04) | CoinGecko / TradingView | Easy | Death cross with BTC.D = altcoin top signal |
| 4 | **Altcoin Season Index** (S02) | blockchaincenter.net / calculate | Easy | Direct signal for altcoin allocation |
| 5 | **MVRV Z-Score** (M06) | Glassnode / CryptoQuant | Medium | Best single cycle top/bottom indicator |
| 6 | **NUPL** (M08) | Glassnode / CryptoQuant | Medium | Market euphoria/capitulation barometer |
| 7 | **Pi Cycle Top** (M02) | Calculate (BTC daily closes) | Easy | Nearly free, historically perfect cycle top caller |
| 8 | **Global Liquidity / M2** (L01) | FRED API + central banks | Medium | Macro ceiling — crypto ~0.85 correlated |
| 9 | **SOPR** (N01) | Glassnode / CryptoQuant | Medium | Key profit-taking metric for all market participants |
| 10 | **STH-SOPR** (N02) | Glassnode / CryptoQuant | Medium | Most reactive dip-buying signal in bull markets |
| 11 | **Realized Price** (N05) | Glassnode / CryptoQuant | Medium | Aggregate break-even — ultimate bear market floor |
| 12 | **STH Realized Price** (N06) | Glassnode / CryptoQuant | Medium | Key dynamic support level in bull markets |
| 13 | **STH MVRV** (N08) | Glassnode / CryptoQuant | Medium | Most sensitive holder profitability metric |
| 14 | **Stablecoin Supply Ratio** (N10) | CryptoQuant | Medium | Measures available buying power ("dry powder") |
| 15 | **Hash Ribbons** (N14) | Calculate (hashrate data) | Medium | Best buy signal after miner capitulation |
| 16 | **Mayer Multiple** (M14) | Calculate (BTC / 200DMA) | Easy | Nearly free overbought/oversold metric |

### Phase 2: HIGH VALUE — Strong Additions (18 indicators)
**Impact:** Completes on-chain coverage, adds miner health, holder cohort analysis.

| # | Indicator | Source API | Effort |
|---|-----------|-----------|--------|
| 1 | Puell Multiple (M03) | Glassnode | Medium |
| 2 | 2-Year MA Multiplier (M05) | Calculate | Easy |
| 3 | Reserve Risk (M16) | Glassnode | Medium |
| 4 | CBBI (M13) | CBBI API | Easy |
| 5 | LTH-SOPR (N03) | Glassnode | Medium |
| 6 | NVT Signal (N04) | Glassnode | Medium |
| 7 | LTH Realized Price (N07) | Glassnode | Medium |
| 8 | LTH MVRV (N09) | Glassnode | Medium |
| 9 | Exchange Reserve (N11) | CryptoQuant | Medium |
| 10 | Exchange Netflow On-chain (N12) | CryptoQuant | Medium |
| 11 | % Addresses in Profit (N13) | Glassnode | Medium |
| 12 | VDD Multiple (N20) | Glassnode | Medium |
| 13 | FED Rate Expectations (L02) | CME FedWatch | Medium |
| 14 | ETF Net Flows (L03) | SoSoValue / Farside | Medium |
| 15 | Token Unlocks (C01) | tokenomist.ai | Medium |
| 16 | LTH Supply (H01) | Glassnode | Medium |
| 17 | STH Supply (H02) | Glassnode | Medium |
| 18 | RSI 22-Day (C07) | Calculate (in-house) | Easy |

### Phase 3: NICE-TO-HAVE — Extended Coverage (20+ indicators)
**Impact:** Adds depth, alternative confirmations, and niche signals.

Includes: RHODL Ratio, BMO, 4Y MA, Rainbow Chart, HODL Waves, CDD, Power Law, 200W MA Heatmap, Active Addresses, AASI, Pricing Models (Top Cap, Delta Top, CVDD), Google Trends, Narrative Index, ETH/BTC Relative Strength, Crypto RSI Heatmap, Historical Monthly Performance, etc.

### Phase 4: SKIP / NOT RECOMMENDED

| Indicator | Reason |
|-----------|--------|
| Smithson's Forecast | Not crypto-relevant |
| Market Cipher | Proprietary TradingView-only, no API |
| Stock-to-Flow (S2F) | Reliability questioned post-2022, PlanB model broken |
| Bitcoin Bubble Index | Mostly overlap with CBBI/NUPL |

---

## 7. Data Source Mapping

### Free APIs (No subscription required)

| Source | Indicators Available | Rate Limits | Notes |
|--------|---------------------|-------------|-------|
| **alternative.me** | Fear & Greed Index | Generous | JSON API, easy integration |
| **CoinGecko** | BTC Dominance, Market Caps, Total3, ETH Dominance | 10-30 calls/min (free) | Comprehensive market data |
| **blockchaincenter.net** | Altcoin Season Index | Scrape or calculate | May need to calculate manually |
| **FRED (Federal Reserve)** | M2 Money Supply, Fed Funds Rate | 120 calls/min | US economic data |
| **CBBI (colintalkscrypto.com)** | CBBI composite score | Open source | 9-metric composite |
| **BTC daily price data** | Pi Cycle, Mayer Multiple, 2Y MA, 4Y MA, Golden Ratio, 200W MA | Via exchange | Calculate in-house |
| **Binance API** | USDT savings rate, klines, volume | Standard rate limits | Already likely integrated |

### Paid APIs (Subscription required)

| Source | Tier | Cost (approx) | Indicators Available |
|--------|------|----------------|---------------------|
| **Glassnode** | Standard | ~$29/mo | MVRV Z-Score, NUPL, SOPR, Realized Price, RHODL, Reserve Risk, HODL Waves, CDD, Active Addresses, NVT, LTH/STH metrics, Hash Ribbons, Puell Multiple |
| **Glassnode** | Professional | ~$799/mo | All Standard + real-time, more granularity, API access |
| **CryptoQuant** | Advanced | ~$29/mo | NUPL, SOPR, Exchange Reserve, Exchange Netflow, SSR, Whale metrics, Miner metrics |
| **CryptoQuant** | Professional | ~$99/mo | All Advanced + alerts, real-time |
| **Bitcoin Magazine Pro** | Pro | ~$30/mo | All on-chain charts, Everything Indicator |
| **SoSoValue** | Free/Pro | Free basic | ETF flow data |
| **tokenomist.ai** | Standard | ~$19/mo | Token unlock schedules |
| **CME FedWatch** | Via CME feeds | Varies | Fed rate probabilities |

### Recommended Minimum: Glassnode Standard ($29/mo) + CryptoQuant Advanced ($29/mo) = **$58/mo**
This combination covers **90%+ of Phase 1 and Phase 2 indicators**.

---

## 8. Architecture Recommendations

### 8.1 Indicator Categorization for Decision Engine

```
┌─────────────────────────────────────────────────────────┐
│                    DECISION ENGINE                        │
│                                                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │  MACRO LAYER │  │  MESO LAYER  │  │  MICRO LAYER │   │
│  │  (Cycle)     │  │  (Daily)     │  │  (Intraday)  │   │
│  ├──────────────┤  ├──────────────┤  ├──────────────┤   │
│  │ MVRV Z-Score │  │ Fear & Greed │  │ CVD/Orderflow│   │
│  │ NUPL         │  │ SOPR/STH-SOPR│  │ Liquidation  │   │
│  │ Pi Cycle     │  │ Funding Rate │  │ AVWAP Levels │   │
│  │ Global M2    │  │ BTC Dominance│  │ Vol/Squeeze  │   │
│  │ Reserve Risk │  │ OI Delta     │  │ EMA/MACD/RSI │   │
│  │ Hash Ribbons │  │ Exchange Flow│  │ ATR/Bollinger│   │
│  │ Puell Mult.  │  │ Altcoin Idx  │  │ Donchian     │   │
│  │ Mayer Mult.  │  │ Realized Pr. │  │ Basis/Fund.  │   │
│  │ Terminal Pr.  │  │ SSR          │  │ Imbalance    │   │
│  │ LTH/STH MVRV│  │ Total3       │  │ TBR          │   │
│  └──────────────┘  └──────────────┘  └──────────────┘   │
│         │                 │                 │            │
│         ▼                 ▼                 ▼            │
│  ┌─────────────────────────────────────────────────┐     │
│  │          POSITION SIZING & RISK FILTER          │     │
│  │                                                  │     │
│  │  Macro Score (0-100) → scales max position size  │     │
│  │  Meso Score → entry/exit timing                  │     │
│  │  Micro Score → execution precision               │     │
│  └─────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
```

### 8.2 Suggested New Provider Structure

```
provider/
├── nofxos/              # Existing: OI, NetFlow, AI500
├── glassnode/           # NEW: On-chain metrics
│   ├── client.go        # API client with rate limiting
│   ├── mvrv.go          # MVRV Z-Score, STH/LTH MVRV
│   ├── nupl.go          # NUPL
│   ├── sopr.go          # SOPR, STH-SOPR, LTH-SOPR
│   ├── realized.go      # Realized Price, STH/LTH RP
│   ├── supply.go        # LTH/STH Supply, Exchange Reserve
│   ├── hodl_waves.go    # HODL Waves, CDD, VDD
│   ├── mining.go        # Hash Ribbons, Puell, Hashrate
│   └── network.go       # Active Addresses, NVT, AASI
├── cryptoquant/         # NEW: Alternative on-chain source
│   ├── client.go
│   ├── exchange_flow.go # Exchange Reserve, Netflow, SSR
│   └── whale.go         # Whale activity metrics
├── macro/               # NEW: Macro indicators
│   ├── fear_greed.go    # Fear & Greed (alternative.me)
│   ├── dominance.go     # BTC Dominance, Total3, Altcoin Season
│   ├── liquidity.go     # Global M2, FED rates
│   └── etf.go           # ETF flows
└── calculated/          # NEW: Self-calculated indicators
    ├── pi_cycle.go      # Pi Cycle Top (EMA111/EMA350×2)
    ├── mayer.go         # Mayer Multiple (Price/200DMA)
    ├── moving_avg.go    # 2Y MA, 4Y MA, 200W MA
    └── cbbi.go          # CBBI composite
```

### 8.3 Refresh Frequency Recommendations

| Layer | Indicators | Refresh Rate | Storage |
|-------|-----------|-------------|---------|
| **Macro** | MVRV, NUPL, Pi Cycle, LTH/STH Supply, Hash Ribbons, M2 | Every 4-24 hours | PostgreSQL + Redis cache |
| **Meso** | Fear & Greed, SOPR, BTC Dominance, SSR, Exchange Reserve, Altcoin Season | Every 1-4 hours | Redis (TTL-based) |
| **Micro** | Existing F4-F7, OI, Funding, Basis, NetFlow | Every 1-15 minutes | Redis (existing) |
| **Static** | Token Unlocks, Monthly Performance, Pricing Models | Daily or on-demand | PostgreSQL |

### 8.4 Integration with LLM Decision Prompts

The macro/cycle indicators should be injected into the LLM decision context as a **Macro Context Block**:

```
## Macro Context (updated every 4h)
- MVRV Z-Score: 3.2 (elevated, approaching distribution zone)
- NUPL: 0.62 (belief phase, not yet euphoria)  
- Pi Cycle: No signal (EMA111 at $X, EMA350×2 at $Y, gap: Z%)
- Fear & Greed: 72 (Greed — cautious)
- BTC Dominance: 58.3% (declining — early altseason signal)
- Altcoin Season Index: 68 (approaching altseason threshold)
- Global M2 (3mo lag): +4.2% YoY (expansionary)
- Stablecoin Supply Ratio: 12.4 (moderate buying power)
- Hash Ribbons: No capitulation signal
- Mayer Multiple: 1.45 (normal range)
- Realized Price: $32,400 | STH RP: $61,200 (current support)
- Macro Risk Score: 65/100 → Max position: 75% of normal

## Meso Context (updated every 1h)
- SOPR: 1.04 (positive, coins moving at profit)
- STH-SOPR: 0.98 (short-term holders slightly underwater — dip buy zone)
- Exchange Reserve: -2.1% (7d) — accumulation signal
- Exchange Netflow: -$142M (24h) — coins leaving exchanges
- OI Delta: +$340M (1h)
- Funding: +8.2 bps
```

---

## 9. Appendix: Indicator Reference Links

### Data Platforms
| Platform | URL | Type |
|----------|-----|------|
| Glassnode Studio | https://studio.glassnode.com/ | On-chain analytics (paid API) |
| CryptoQuant | https://cryptoquant.com/ | On-chain analytics (paid API) |
| Bitcoin Magazine Pro | https://www.bitcoinmagazinepro.com/charts/ | On-chain charts (paid) |
| CoinGlass | https://www.coinglass.com/ | Derivatives data (free/paid) |
| CoinGecko | https://www.coingecko.com/ | Market data (free API) |
| Alternative.me | https://alternative.me/crypto/fear-and-greed-index/ | Fear & Greed (free API) |
| Blockchain Center | https://www.blockchaincenter.net/altcoin-season-index/ | Altcoin Season (free) |
| CBBI | https://colintalkscrypto.com/cbbi/ | Composite index (free, open source) |
| Bitbo | https://charts.bitbo.io/ | Bitcoin dashboard (free/paid) |
| CheckOnChain | https://checkonchain.com/ | On-chain research |
| Dune Analytics | https://dune.com/cryptokoryo/narratives | Narrative tracking (free) |
| FRED | https://fred.stlouisfed.org/ | Macro economic data (free API) |
| SoSoValue | https://sosovalue.com/ | ETF flow data |
| Tokenomist | https://tokenomist.ai/ | Token unlock schedules |
| DefiLlama | https://defillama.com/ | DeFi data (free) |
| Santiment | https://app.santiment.net/ | Social + on-chain (paid) |
| TradingView | https://www.tradingview.com/ | Charting (for reference) |

### Specific Chart References
| Indicator | URL |
|-----------|-----|
| Fear & Greed API | https://api.alternative.me/fng/ |
| MVRV Z-Score | https://www.bitcoinmagazinepro.com/charts/mvrv-zscore/ |
| NUPL | https://www.bitcoinmagazinepro.com/charts/relative-unrealized-profit--loss/ |
| Pi Cycle Top | https://www.bitcoinmagazinepro.com/charts/pi-cycle-top-indicator/ |
| Puell Multiple | https://www.bitcoinmagazinepro.com/charts/puell-multiple/ |
| RHODL Ratio | https://www.bitcoinmagazinepro.com/charts/rhodl-ratio/ |
| Reserve Risk | https://www.bitcoinmagazinepro.com/charts/reserve-risk/ |
| SOPR | https://www.bitcoinmagazinepro.com/charts/sopr-spent-output-profit-ratio/ |
| Hash Ribbons | https://www.bitcoinmagazinepro.com/charts/hash-ribbons/ |
| Realized Price | https://www.bitcoinmagazinepro.com/charts/realized-price/ |
| STH Realized Price | https://www.bitcoinmagazinepro.com/charts/short-term-holder-realized-price/ |
| LTH Realized Price | https://www.bitcoinmagazinepro.com/charts/long-term-holder-realized-price/ |
| HODL Waves | https://www.bitcoinmagazinepro.com/charts/hodl-waves/ |
| VDD Multiple | https://www.bitcoinmagazinepro.com/charts/value-days-destroyed-multiple/ |
| NVT Signal | https://www.bitcoinmagazinepro.com/charts/advanced-nvt-signal/ |
| Everything Indicator | https://www.bitcoinmagazinepro.com/charts/everything-indicator/ |
| 2-Year MA | https://www.bitcoinmagazinepro.com/charts/bitcoin-investor-tool/ |
| Golden Ratio Multiplier | https://www.bitcoinmagazinepro.com/charts/golden-ratio-multiplier/ |
| 200W MA Heatmap | https://www.bitcoinmagazinepro.com/charts/200-week-moving-average-heatmap/ |
| Terminal Price | https://www.bitcoinmagazinepro.com/charts/terminal-price/ |
| Top Cap | https://www.bitcoinmagazinepro.com/charts/top-cap/ |
| Delta Top | https://www.bitcoinmagazinepro.com/charts/delta-top/ |
| Bitcoin Rainbow | https://www.bitcoinmagazinepro.com/charts/bitcoin-rainbow-chart/ |
| Power Law | https://www.bitcoinmagazinepro.com/charts/bitcoin-power-law/ |
| Active Addresses | https://www.bitcoinmagazinepro.com/charts/bitcoin-active-addresses/ |
| RSI Heatmap | https://www.coinglass.com/pro/i/RsiHeatMap |
| Liquidation Heatmap | https://www.coinglass.com/pro/futures/LiquidationHeatMap |
| Crypto Narratives | https://dune.com/cryptokoryo/narratives |
| Token Unlocks | https://tokenomist.ai/ |

---

## Document End

> **Next steps:** Review priority Phase 1 indicators and approve data source subscriptions (Glassnode Standard + CryptoQuant Advanced recommended). Implementation can begin with the free/calculated indicators (Fear & Greed, Pi Cycle, Mayer Multiple, BTC Dominance, Altcoin Season Index) in parallel with API integration work for Glassnode/CryptoQuant.
