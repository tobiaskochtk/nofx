package store

import (
	"sort"
	"strings"
)

func extractDealReviewNormalizedSignalClusters(reasoning string, candidate CandidateDetail, actionContext *DealReviewMarketContextSnapshot) []string {
	labels := map[string]struct{}{}
	add := func(value string) {
		normalized := normalizeDealReviewSignalClusterLabel(value)
		if normalized == "" {
			return
		}
		labels[normalized] = struct{}{}
	}

	bucket := normalizeDealReviewSymbolPriorDimension(candidate.SelectionBucket)
	if bucket != "" && bucket != "unknown" {
		add("bucket_" + bucket)
		switch {
		case strings.Contains(bucket, "breakout"):
			add("setup_breakout")
		case strings.Contains(bucket, "mean"), strings.Contains(bucket, "revert"), strings.Contains(bucket, "fade"):
			add("setup_mean_reversion")
		case strings.Contains(bucket, "momentum"):
			add("setup_momentum")
		case strings.Contains(bucket, "trend"), strings.Contains(bucket, "continuation"):
			add("setup_continuation")
		}
	}

	for _, source := range candidate.Sources {
		if normalized := normalizeDealReviewSignalClusterLabel(source); normalized != "" {
			add("src_" + normalized)
		}
	}

	rawTags := extractDealReviewSignalTags(reasoning)
	rawTagSet := make(map[string]struct{}, len(rawTags))
	for _, tag := range rawTags {
		rawTagSet[tag] = struct{}{}
	}
	normalizedReasoning := normalizeReasoningHint(reasoning)

	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "breakout", "follow through", "follow_through") {
		add("setup_breakout")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "mean reversion", "mean_reversion", "fade") {
		add("setup_mean_reversion")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "momentum", "continuation", "trend continuation", "trend_continuation") {
		add("setup_continuation")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "trend align", "trend_align", "ema align", "ema_align", "aligned") {
		add("confirm_alignment")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "relative strength", "rel_strength", "btc strength") {
		add("confirm_relative_strength")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "oi up", "oi_up", "oi rising", "oi_rising", "oi expansion", "oi_up_price_up") {
		add("confirm_oi_expansion")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "oi dislocation", "oi_dislocation") {
		add("confirm_oi_dislocation")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "volume confirm", "volume_confirm", "breadth aligned") {
		add("confirm_volume")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "volume not confirming", "volume_not_confirming", "not confirming") {
		add("warn_volume_non_confirmation")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "oi not confirming", "oi_not_confirming") {
		add("warn_oi_non_confirmation")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "late breakout", "late_breakout") {
		add("warn_late_breakout")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "fresh") {
		add("fresh_setup")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "cooldown", "same symbol cooldown", "same_symbol_cooldown") {
		add("risk_cooldown")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "manage positions first", "manage_positions_first") {
		add("portfolio_priority")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "crowded longs", "crowded_longs") {
		add("crowding_longs")
	}
	if dealReviewSignalClusterContains(normalizedReasoning, rawTagSet, "crowded shorts", "crowded_shorts") {
		add("crowding_shorts")
	}

	context := actionContext
	if (context == nil || !context.HasAny()) && candidate.MarketContext != nil {
		context = candidate.MarketContext
	}
	if context != nil {
		switch normalizeDealReviewSymbolPriorDimension(context.FundingRegime) {
		case "longs_pay", "extreme_longs":
			add("crowding_longs")
		case "shorts_pay", "extreme_shorts":
			add("crowding_shorts")
		}
		switch normalizeDealReviewSymbolPriorDimension(context.OIRegime) {
		case "rising", "oi_rising", "surge", "oi_surge":
			add("indicator_oi_expansion")
		case "falling", "oi_falling", "flush", "oi_flush":
			add("indicator_oi_flush")
		}
		if context.RSI != nil {
			switch {
			case *context.RSI >= 70:
				add("indicator_rsi_overbought")
			case *context.RSI <= 30:
				add("indicator_rsi_oversold")
			}
		}
		if context.MACD != nil {
			switch {
			case *context.MACD > 0:
				add("indicator_macd_positive")
			case *context.MACD < 0:
				add("indicator_macd_negative")
			}
		}
		if context.OIDelta1hPct != nil {
			switch {
			case *context.OIDelta1hPct >= 5:
				add("indicator_oi_expansion")
			case *context.OIDelta1hPct <= -5:
				add("indicator_oi_flush")
			}
		}
		if context.PriceChange1h != nil {
			switch {
			case *context.PriceChange1h >= 2:
				add("indicator_price_impulse_up")
			case *context.PriceChange1h <= -2:
				add("indicator_price_impulse_down")
			}
		}
		if context.BTCRelativeStrengthCtx != nil {
			switch {
			case *context.BTCRelativeStrengthCtx >= 0.75:
				add("indicator_btc_relative_strength")
			case *context.BTCRelativeStrengthCtx <= -0.75:
				add("indicator_btc_relative_weakness")
			}
		}
	}

	if len(labels) == 0 {
		return nil
	}
	out := make([]string, 0, len(labels))
	for label := range labels {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

func dealReviewSignalClusterContains(normalizedReasoning string, rawTagSet map[string]struct{}, phrases ...string) bool {
	for _, phrase := range phrases {
		normalizedPhrase := normalizeReasoningHint(phrase)
		if normalizedPhrase != "" && strings.Contains(normalizedReasoning, normalizedPhrase) {
			return true
		}
		rawToken := normalizeDealReviewSignalClusterLabel(phrase)
		if rawToken == "" {
			continue
		}
		if _, ok := rawTagSet[rawToken]; ok {
			return true
		}
	}
	return false
}

func normalizeDealReviewSignalClusterLabel(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}
