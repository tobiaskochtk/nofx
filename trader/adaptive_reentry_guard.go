package trader

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"nofx/kernel"
	"nofx/store"
)

type recentExecutionRegimeSnapshot struct {
	TradeCount         int
	WinRatePct         float64
	AvgPnLPct          float64
	ConsecutiveLosses  int
	FollowThroughState string
	ChurnRisk          string
}

func buildRecentExecutionRegime(trades []store.RecentTrade) *kernel.RecentExecutionRegime {
	return buildRecentExecutionRegimeWithConfig(trades, store.DefaultAdaptiveReentryGuardConfig())
}

func buildRecentExecutionRegimeWithConfig(trades []store.RecentTrade, cfg store.AdaptiveReentryGuardConfig) *kernel.RecentExecutionRegime {
	snapshot := summarizeRecentExecutionRegimeWithConfig(trades, cfg)
	if snapshot.TradeCount == 0 {
		return nil
	}
	return &kernel.RecentExecutionRegime{
		TradeCount:         snapshot.TradeCount,
		WinRatePct:         snapshot.WinRatePct,
		AvgPnLPct:          snapshot.AvgPnLPct,
		ConsecutiveLosses:  snapshot.ConsecutiveLosses,
		FollowThroughState: snapshot.FollowThroughState,
		ChurnRisk:          snapshot.ChurnRisk,
	}
}

func summarizeRecentExecutionRegime(trades []store.RecentTrade) recentExecutionRegimeSnapshot {
	return summarizeRecentExecutionRegimeWithConfig(trades, store.DefaultAdaptiveReentryGuardConfig())
}

func summarizeRecentExecutionRegimeWithConfig(trades []store.RecentTrade, cfg store.AdaptiveReentryGuardConfig) recentExecutionRegimeSnapshot {
	if len(trades) == 0 {
		return recentExecutionRegimeSnapshot{}
	}

	limit := len(trades)
	if cfg.RecentTradeWindow > 0 && limit > cfg.RecentTradeWindow {
		limit = cfg.RecentTradeWindow
	}
	window := trades[:limit]

	var wins int
	var totalPnLPct float64
	for _, trade := range window {
		totalPnLPct += trade.PnLPct
		if trade.PnLPct > 0 {
			wins++
		}
	}

	winRatePct := 0.0
	if limit > 0 {
		winRatePct = float64(wins) / float64(limit) * 100
	}

	consecutiveLosses := 0
	for _, trade := range window {
		if trade.PnLPct < 0 {
			consecutiveLosses++
			continue
		}
		break
	}

	avgPnLPct := 0.0
	if limit > 0 {
		avgPnLPct = totalPnLPct / float64(limit)
	}

	minTrades := cfg.MinRecentTrades
	if minTrades <= 0 {
		minTrades = store.DefaultAdaptiveReentryMinRecentTrades
	}

	followThroughState := "mixed"
	churnRisk := "low"
	if limit < minTrades {
		followThroughState = "unknown"
	} else {
		switch {
		case winRatePct >= 60 && avgPnLPct >= 0.75:
			followThroughState = "strong"
		case winRatePct <= 45 && avgPnLPct <= 0:
			followThroughState = "weak"
		default:
			followThroughState = "mixed"
		}
	}

	switch {
	case followThroughState == "weak" && (consecutiveLosses >= 2 || avgPnLPct < 0):
		churnRisk = "high"
	case (followThroughState == "weak" && limit >= minTrades) ||
		(followThroughState == "mixed" && consecutiveLosses >= 2):
		churnRisk = "elevated"
	}

	return recentExecutionRegimeSnapshot{
		TradeCount:         limit,
		WinRatePct:         winRatePct,
		AvgPnLPct:          avgPnLPct,
		ConsecutiveLosses:  consecutiveLosses,
		FollowThroughState: followThroughState,
		ChurnRisk:          churnRisk,
	}
}

func adaptiveReentryGuardReason(symbol string, trades []store.RecentTrade, now time.Time) string {
	return adaptiveReentryGuardReasonWithConfig(symbol, trades, now, store.DefaultAdaptiveReentryGuardConfig())
}

func adaptiveReentryGuardReasonWithConfig(symbol string, trades []store.RecentTrade, now time.Time, cfg store.AdaptiveReentryGuardConfig) string {
	normalizedSymbol := normalizeTradeSymbol(symbol)
	if normalizedSymbol == "" {
		return ""
	}
	if !cfg.Enabled {
		return ""
	}

	if cfg.RequireWeakExecutionRegime {
		regime := summarizeRecentExecutionRegimeWithConfig(trades, cfg)
		if regime.TradeCount < cfg.MinRecentTrades {
			return ""
		}
		if regime.FollowThroughState != "weak" && regime.ChurnRisk != "high" {
			return ""
		}
	}

	sameSymbolTrades := filterRecentTradesBySymbol(trades, normalizedSymbol)
	if len(sameSymbolTrades) == 0 {
		return ""
	}

	latest := sameSymbolTrades[0]
	if latest.PnLPct < 0 {
		if minutesSinceClose, ok := tradeMinutesSinceClose(latest, now); ok && minutesSinceClose < float64(cfg.SameSymbolLossCooldownMinutes) {
			return fmt.Sprintf("adaptive re-entry guard: %s last closed %.0f min ago at %+.2f%% while recent follow-through is %s",
				normalizedSymbol, minutesSinceClose, latest.PnLPct, summarizeRecentExecutionRegimeWithConfig(trades, cfg).FollowThroughState)
		}
	}

	if latestTwoSameSymbolTradesAreLosses(sameSymbolTrades, now, cfg.PairLossLookbackHours) {
		return fmt.Sprintf("adaptive re-entry guard: %s has two recent losses while churn risk is %s",
			normalizedSymbol, summarizeRecentExecutionRegimeWithConfig(trades, cfg).ChurnRisk)
	}

	return ""
}

func filterRecentTradesBySymbol(trades []store.RecentTrade, symbol string) []store.RecentTrade {
	if symbol == "" {
		return nil
	}
	filtered := make([]store.RecentTrade, 0, len(trades))
	for _, trade := range trades {
		if normalizeTradeSymbol(trade.Symbol) != symbol {
			continue
		}
		filtered = append(filtered, trade)
	}
	return filtered
}

func latestTwoSameSymbolTradesAreLosses(trades []store.RecentTrade, now time.Time, lookbackHours int) bool {
	if len(trades) < 2 {
		return false
	}
	if lookbackHours <= 0 {
		lookbackHours = store.DefaultAdaptiveReentryPairLossLookbackHours
	}
	lookback := time.Duration(lookbackHours) * time.Hour
	for i := 0; i < 2; i++ {
		trade := trades[i]
		if trade.PnLPct >= 0 {
			return false
		}
		if trade.ExitTime <= 0 {
			return false
		}
		exitTime := time.Unix(trade.ExitTime, 0).UTC()
		if now.UTC().Sub(exitTime) > lookback {
			return false
		}
	}
	return true
}

func tradeMinutesSinceClose(trade store.RecentTrade, now time.Time) (float64, bool) {
	if trade.ExitTime <= 0 {
		return 0, false
	}
	exitTime := time.Unix(trade.ExitTime, 0).UTC()
	if now.UTC().Before(exitTime) {
		return 0, false
	}
	return now.UTC().Sub(exitTime).Minutes(), true
}

func normalizeTradeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func (at *AutoTrader) adaptiveReentryGuardConfig() store.AdaptiveReentryGuardConfig {
	if at == nil || at.config.StrategyConfig == nil {
		return store.DefaultAdaptiveReentryGuardConfig()
	}
	return at.config.StrategyConfig.RiskControl.EffectiveAdaptiveReentryGuard()
}

func (at *AutoTrader) enforceAdaptiveSameSymbolReentry(symbol string) error {
	if at == nil || at.store == nil {
		return nil
	}
	cfg := at.adaptiveReentryGuardConfig()
	if !cfg.Enabled {
		return nil
	}
	recentTradeLimit := cfg.RecentTradeWindow * 2
	if recentTradeLimit < 20 {
		recentTradeLimit = 20
	}
	recentTrades, err := at.store.Position().GetRecentTrades(at.id, recentTradeLimit)
	if err != nil {
		return nil
	}
	if reason := adaptiveReentryGuardReasonWithConfig(symbol, recentTrades, time.Now().UTC(), cfg); reason != "" {
		return errors.New(reason)
	}
	return nil
}
