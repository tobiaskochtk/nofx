package store

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const traderOpenExecutionSampleLimit = 12

type traderRegimeAccumulator struct {
	Summary TraderRegimeSummary
}

func enrichDecisionRecordTelemetry(record *DecisionRecord) {
	if record == nil {
		return
	}

	openSymbols := make(map[string]struct{})
	symbolRejectReasons := make(map[string][]string)
	genericRejectReasons := []string{}

	for i := range record.Decisions {
		action := &record.Decisions[i]
		if action == nil {
			continue
		}
		symbol := strings.ToUpper(strings.TrimSpace(action.Symbol))
		if action.Timestamp.IsZero() && !record.Timestamp.IsZero() {
			action.Timestamp = record.Timestamp.UTC()
		}
		if action.MarketContext == nil && symbol != "" {
			if context := buildDealReviewMarketContextFromDecisionRecord(
				record,
				decisionReviewStage(action.Action),
				symbol,
				decisionActionSide(action.Action),
				decisionActionTimestamp(record, action),
			); context != nil {
				action.MarketContext = context
			}
		}
		if action.ExchangeOrderID == "" && action.OrderID > 0 {
			action.ExchangeOrderID = fmt.Sprintf("%d", action.OrderID)
		}
		if isOpenDecisionAction(action.Action) && symbol != "" {
			openSymbols[symbol] = struct{}{}
			if action.Execution == nil {
				action.Execution = buildDecisionExecutionTelemetry(action)
			}
		}
		if !isHoldOrWaitAction(action.Action) {
			continue
		}
		if len(action.RejectReasons) == 0 {
			action.RejectReasons = deriveRejectReasons(action.Reasoning, action.MarketContext)
		}
		if len(action.RejectReasons) == 0 {
			continue
		}
		if symbol == "" {
			genericRejectReasons = mergeRejectReasonLists(genericRejectReasons, action.RejectReasons)
			continue
		}
		symbolRejectReasons[symbol] = mergeRejectReasonLists(symbolRejectReasons[symbol], action.RejectReasons)
	}

	for i := range record.CandidateDetails {
		detail := &record.CandidateDetails[i]
		if detail == nil {
			continue
		}
		symbol := strings.ToUpper(strings.TrimSpace(detail.Symbol))
		if symbol == "" {
			continue
		}
		if detail.MarketContext == nil {
			if context := buildDealReviewMarketContextFromDecisionRecord(
				record,
				DealReviewStageOpen,
				symbol,
				"",
				record.Timestamp.UTC(),
			); context != nil {
				detail.MarketContext = context
			}
		}
		if _, opened := openSymbols[symbol]; opened {
			continue
		}
		if len(detail.RejectReasons) > 0 {
			continue
		}
		reasons := mergeRejectReasonLists(symbolRejectReasons[symbol], genericRejectReasons)
		reasons = mergeRejectReasonLists(reasons, deriveRejectReasons("", detail.MarketContext))
		if len(reasons) == 0 {
			reasons = []string{"unspecified"}
		}
		detail.RejectReasons = reasons
	}
}

func decisionReviewStage(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "close_long", "close_short":
		return DealReviewStageClose
	default:
		return DealReviewStageOpen
	}
}

func decisionActionSide(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_long", "close_long":
		return "LONG"
	case "open_short", "close_short":
		return "SHORT"
	default:
		return ""
	}
}

func decisionActionTimestamp(record *DecisionRecord, action *DecisionAction) time.Time {
	if action != nil && !action.Timestamp.IsZero() {
		return action.Timestamp.UTC()
	}
	if record != nil && !record.Timestamp.IsZero() {
		return record.Timestamp.UTC()
	}
	return time.Now().UTC()
}

func isOpenDecisionAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_long", "open_short":
		return true
	default:
		return false
	}
}

func isHoldOrWaitAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "hold", "wait":
		return true
	default:
		return false
	}
}

func buildDecisionExecutionTelemetry(action *DecisionAction) *DecisionExecutionTelemetry {
	if action == nil || !isOpenDecisionAction(action.Action) {
		return nil
	}
	orderID := strings.TrimSpace(action.ExchangeOrderID)
	if orderID == "" && action.OrderID > 0 {
		orderID = fmt.Sprintf("%d", action.OrderID)
	}
	telemetry := &DecisionExecutionTelemetry{
		ExchangeOrderID: orderID,
		StatusDetail:    strings.TrimSpace(action.Error),
	}
	if action.Success {
		telemetry.HandedToExecution = true
		telemetry.OrderSubmitted = true
		telemetry.TerminalStatus = "submitted"
		return telemetry
	}

	lower := strings.ToLower(strings.TrimSpace(action.Error))
	if orderID != "" {
		telemetry.HandedToExecution = true
	}

	switch {
	case strings.Contains(lower, "safe mode"),
		strings.Contains(lower, "risk control"),
		strings.Contains(lower, "max positions"),
		strings.Contains(lower, "position value"),
		strings.Contains(lower, "position size"),
		strings.Contains(lower, "min position"),
		strings.Contains(lower, "insufficient margin"),
		strings.Contains(lower, "remaining"),
		strings.Contains(lower, "paused"):
		telemetry.TerminalStatus = "blocked_by_risk_control"
		telemetry.FailureCategory = "risk_control"
	case strings.Contains(lower, "already has"),
		strings.Contains(lower, "close it first"),
		strings.Contains(lower, "existing position"):
		telemetry.TerminalStatus = "blocked_by_position_state"
		telemetry.FailureCategory = "position_state"
	case strings.Contains(lower, "unsupported"),
		strings.Contains(lower, "not found in meta"),
		strings.Contains(lower, "instrument info"),
		strings.Contains(lower, "format quantity"),
		strings.Contains(lower, "precision"),
		strings.Contains(lower, "no price"),
		strings.Contains(lower, "min notional"),
		strings.Contains(lower, "order book"),
		strings.Contains(lower, "venue"):
		telemetry.TerminalStatus = "failed_exchange_validation"
		telemetry.FailureCategory = "exchange_validation"
	case strings.Contains(lower, "cancel"),
		strings.Contains(lower, "expired"),
		strings.Contains(lower, "reject"):
		telemetry.TerminalStatus = "rejected_or_canceled"
		telemetry.FailureCategory = "execution_response"
	default:
		if telemetry.HandedToExecution {
			telemetry.TerminalStatus = "submit_failed"
		} else {
			telemetry.TerminalStatus = "never_handed_to_execution"
		}
		telemetry.FailureCategory = "execution_handoff"
	}
	return telemetry
}

func deriveRejectReasons(reasoning string, context *DealReviewMarketContextSnapshot) []string {
	reasons := []string{}
	appendReason := func(reason string) {
		reason = normalizeDetailedRejectReason(reason)
		if reason == "" {
			return
		}
		for _, existing := range reasons {
			if existing == reason {
				return
			}
		}
		reasons = append(reasons, reason)
	}

	text := strings.ToLower(strings.TrimSpace(reasoning))
	containsAny := func(parts ...string) bool {
		for _, part := range parts {
			if strings.Contains(text, part) {
				return true
			}
		}
		return false
	}
	switch {
	case strings.Contains(text, "confidence"):
		appendReason("low_confidence")
	case strings.Contains(text, "confirm"):
		appendReason("missing_confirmation")
	}
	if strings.Contains(text, "liquid") {
		appendReason("liquidity_concerns")
	}
	if strings.Contains(text, "spread") || strings.Contains(text, "slippage") {
		appendReason("spread_slippage_concerns")
	}
	if strings.Contains(text, "trend conflict") || strings.Contains(text, "timeframe conflict") || strings.Contains(text, "mixed trend") {
		appendReason("trend_conflict")
	}
	if strings.Contains(text, "regime") || strings.Contains(text, "chop") || strings.Contains(text, "range-bound") {
		appendReason("regime_mismatch")
	}
	if containsAny("overextend", "extended", "stretched", "exhausted", "too far", "too late", "already moved", "chased", "late breakout") {
		appendReason("extended_setup")
	}
	if containsAny("late breakout", "late entry", "already broke out", "already broke", "breakout already happened", "chased move") {
		appendReason("late_breakout")
	}
	if containsAny("volume not confirm", "volume doesn't confirm", "no volume", "weak volume", "volume faded", "volume not supportive", "thin volume") {
		appendReason("volume_not_confirming")
	}
	if containsAny("oi flat", "oi not confirm", "open interest not confirm", "oi faded", "falling oi", "oi weak", "oi not supportive", "open interest weak") {
		appendReason("oi_not_confirming")
	}
	if containsAny("cooldown", "same symbol", "same-symbol", "loss streak", "recent loss", "re-entry guard", "reentry guard") {
		appendReason("same_symbol_cooldown")
	}
	if strings.Contains(text, "venue") || strings.Contains(text, "unsupported") {
		appendReason("venue_unsupported")
	}
	if strings.Contains(text, "risk") || strings.Contains(text, "position limit") {
		appendReason("risk_control")
	}
	if strings.Contains(text, "unclear") || strings.Contains(text, "mixed") {
		appendReason("trend_conflict")
	}

	if context != nil {
		if context.VenueSupported != nil && !*context.VenueSupported {
			appendReason("venue_unsupported")
		}
		switch context.LiquidityTier {
		case "thin", "low", "illiquid":
			appendReason("liquidity_concerns")
		}
		switch context.SpreadBucket {
		case "wide", "very_wide", "extreme":
			appendReason("spread_slippage_concerns")
		}
		switch context.SlippageBucket {
		case "high", "very_high", "extreme":
			appendReason("spread_slippage_concerns")
		}
		switch context.TrendRegime {
		case "mixed", "conflicted":
			appendReason("trend_conflict")
		case "chop", "range", "range_bound":
			appendReason("regime_mismatch")
		}
		switch context.VolatilityRegime {
		case "high_vol", "shock":
			appendReason("regime_mismatch")
		}
		if context.RSI != nil && *context.RSI >= 69 {
			appendReason("extended_setup")
		}
		if context.OIRegime == "flat" || context.OIRegime == "mixed" {
			appendReason("oi_not_confirming")
		}
		if context.OIDelta1hPct != nil && *context.OIDelta1hPct <= 0.2 {
			appendReason("oi_not_confirming")
		}
	}

	return reasons
}

func normalizeDetailedRejectReason(reason string) string {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "", "unknown":
		return ""
	case "low_confidence", "liquidity_concerns", "spread_slippage_concerns", "regime_mismatch", "trend_conflict", "missing_confirmation", "venue_unsupported", "risk_control", "extended_setup", "late_breakout", "volume_not_confirming", "oi_not_confirming", "same_symbol_cooldown", "unspecified":
		return strings.ToLower(strings.TrimSpace(reason))
	default:
		return normalizeTraderRejectReason(reason)
	}
}

func mergeRejectReasonLists(base []string, additions []string) []string {
	if len(additions) == 0 {
		return base
	}
	out := append([]string(nil), base...)
	for _, item := range additions {
		normalized := normalizeDetailedRejectReason(item)
		if normalized == "" {
			continue
		}
		alreadyExists := false
		for _, existing := range out {
			if existing == normalized {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			out = append(out, normalized)
		}
	}
	return out
}

func buildTraderExecutionStatuses(counts map[string]int, total int, limit int) []TraderExecutionStatus {
	if len(counts) == 0 {
		return []TraderExecutionStatus{}
	}
	items := make([]TraderExecutionStatus, 0, len(counts))
	for status, count := range counts {
		entry := TraderExecutionStatus{Status: status, Count: count}
		if total > 0 {
			entry.SharePct = roundFloat(float64(count)/float64(total)*100, 1)
		}
		items = append(items, entry)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Status < items[j].Status
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func buildTraderOpenExecution(record *DecisionRecord, action *DecisionAction) TraderOpenExecution {
	entry := TraderOpenExecution{
		CycleNumber:     0,
		Timestamp:       decisionActionTimestamp(record, action),
		Symbol:          strings.ToUpper(strings.TrimSpace(action.Symbol)),
		Side:            strings.ToLower(strings.TrimSpace(decisionActionSide(action.Action))),
		Confidence:      action.Confidence,
		TerminalStatus:  "unknown",
		ExchangeOrderID: strings.TrimSpace(action.ExchangeOrderID),
		Reasoning:       strings.TrimSpace(action.Reasoning),
	}
	if record != nil {
		entry.CycleNumber = record.CycleNumber
	}
	if action != nil && action.Execution != nil {
		entry.TerminalStatus = strings.TrimSpace(action.Execution.TerminalStatus)
		entry.FailureCategory = strings.TrimSpace(action.Execution.FailureCategory)
		if entry.ExchangeOrderID == "" {
			entry.ExchangeOrderID = strings.TrimSpace(action.Execution.ExchangeOrderID)
		}
	}
	if action != nil && action.MarketContext != nil {
		entry.TrendRegime = action.MarketContext.TrendRegime
		entry.VolatilityRegime = action.MarketContext.VolatilityRegime
		entry.OIRegime = action.MarketContext.OIRegime
		entry.FundingRegime = action.MarketContext.FundingRegime
		entry.SessionBucket = action.MarketContext.SessionBucket
		entry.LiquidityTier = action.MarketContext.LiquidityTier
		entry.SpreadBucket = action.MarketContext.SpreadBucket
		entry.SlippageBucket = action.MarketContext.SlippageBucket
	}
	if entry.TerminalStatus == "" {
		entry.TerminalStatus = "unknown"
	}
	return entry
}

func selectRecentTraderOpenExecutions(items []TraderOpenExecution, limit int) []TraderOpenExecution {
	if len(items) == 0 {
		return []TraderOpenExecution{}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Timestamp.Equal(items[j].Timestamp) {
			if items[i].CycleNumber == items[j].CycleNumber {
				return items[i].Symbol < items[j].Symbol
			}
			return items[i].CycleNumber > items[j].CycleNumber
		}
		return items[i].Timestamp.After(items[j].Timestamp)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func ensureTraderRegimeAccumulator(accumulators map[string]*traderRegimeAccumulator, context *DealReviewMarketContextSnapshot) *traderRegimeAccumulator {
	if context == nil {
		return nil
	}
	key := strings.Join([]string{
		strings.TrimSpace(context.TrendRegime),
		strings.TrimSpace(context.VolatilityRegime),
		strings.TrimSpace(context.OIRegime),
		strings.TrimSpace(context.FundingRegime),
		strings.TrimSpace(context.LiquidityTier),
		strings.TrimSpace(context.SessionBucket),
	}, "|")
	if strings.Trim(strings.ReplaceAll(key, "|", ""), " ") == "" {
		key = "unknown"
	}
	if accumulators[key] == nil {
		accumulators[key] = &traderRegimeAccumulator{
			Summary: TraderRegimeSummary{
				Key:              key,
				TrendRegime:      strings.TrimSpace(context.TrendRegime),
				VolatilityRegime: strings.TrimSpace(context.VolatilityRegime),
				OIRegime:         strings.TrimSpace(context.OIRegime),
				FundingRegime:    strings.TrimSpace(context.FundingRegime),
				LiquidityTier:    strings.TrimSpace(context.LiquidityTier),
				SessionBucket:    strings.TrimSpace(context.SessionBucket),
			},
		}
	}
	return accumulators[key]
}

func buildTraderRegimeSummaries(accumulators map[string]*traderRegimeAccumulator, limit int) []TraderRegimeSummary {
	if len(accumulators) == 0 {
		return []TraderRegimeSummary{}
	}
	result := make([]TraderRegimeSummary, 0, len(accumulators))
	for _, acc := range accumulators {
		if acc == nil {
			continue
		}
		result = append(result, acc.Summary)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CandidateCount == result[j].CandidateCount {
			if result[i].OpenDecisionCount == result[j].OpenDecisionCount {
				return result[i].Key < result[j].Key
			}
			return result[i].OpenDecisionCount > result[j].OpenDecisionCount
		}
		return result[i].CandidateCount > result[j].CandidateCount
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}
