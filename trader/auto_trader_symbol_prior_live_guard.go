package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/store"
	"strings"
)

func (at *AutoTrader) symbolBehaviorLiveGuardConfig() store.SymbolBehaviorLiveGuardConfig {
	if at.config.StrategyConfig == nil {
		return store.DefaultSymbolBehaviorLiveGuardConfig()
	}
	return at.config.StrategyConfig.RiskControl.EffectiveSymbolBehaviorLiveGuard()
}

func (at *AutoTrader) enforceSymbolBehaviorLiveGuard(record *store.DecisionRecord, actionRecord *store.DecisionAction) error {
	if at == nil || at.store == nil || record == nil || actionRecord == nil || !isOpenAction(actionRecord.Action) {
		return nil
	}

	cfg := at.symbolBehaviorLiveGuardConfig()
	if !cfg.Enabled {
		return nil
	}

	probe := store.BuildDealReviewSymbolBehaviorLiveGuardProbeFromDecisionRecord(record, actionRecord)
	if probe == nil {
		return nil
	}

	assessment, err := at.store.DealReview().EvaluateSymbolBehaviorLiveGuard(at.userID, at.id, *probe, cfg)
	if err != nil {
		logger.Warnf("⚠️ [%s] Symbol behavior live guard evaluation failed: %v", at.name, err)
		return nil
	}
	if assessment == nil {
		return nil
	}
	if err := at.store.DealReview().RecordSymbolBehaviorLiveGuardEvent(at.userID, at.id, probe, assessment); err != nil {
		logger.Warnf("⚠️ [%s] Failed to record symbol behavior live guard event: %v", at.name, err)
	}

	switch assessment.Effect {
	case store.DealReviewSymbolBehaviorLiveGuardEffectMonitorOnly:
		if summary := strings.TrimSpace(assessment.Summary); summary != "" {
			record.ExecutionLog = append(record.ExecutionLog, "⚠ symbol prior monitor: "+summary)
			logger.Warnf("⚠️ [%s] %s", at.name, summary)
		}
	case store.DealReviewSymbolBehaviorLiveGuardEffectMatchedUnqualified:
		if summary := strings.TrimSpace(assessment.Summary); summary != "" {
			record.ExecutionLog = append(record.ExecutionLog, "ℹ symbol prior review: "+summary)
		}
	}

	if !assessment.HardBlock {
		return nil
	}

	reason := strings.TrimSpace(assessment.BlockReason)
	if reason == "" {
		reason = strings.TrimSpace(assessment.Summary)
	}
	actionRecord.RejectReasons = appendUniqueRejectReasons(actionRecord.RejectReasons, "risk_control", "symbol_behavior_prior_block")
	record.ExecutionLog = append(record.ExecutionLog, "⚠ symbol prior block: "+reason)
	logger.Warnf("⚠️ [%s] %s", at.name, reason)
	return fmt.Errorf("❌ [RISK CONTROL] symbol prior guard blocked %s %s: %s", probe.Symbol, actionRecord.Action, reason)
}

func isOpenAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_long", "open_short":
		return true
	default:
		return false
	}
}

func appendUniqueRejectReasons(base []string, additions ...string) []string {
	out := append([]string(nil), base...)
	for _, addition := range additions {
		addition = strings.ToLower(strings.TrimSpace(addition))
		if addition == "" {
			continue
		}
		exists := false
		for _, current := range out {
			if strings.EqualFold(strings.TrimSpace(current), addition) {
				exists = true
				break
			}
		}
		if !exists {
			out = append(out, addition)
		}
	}
	return out
}
