package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/store"
	"strings"
)

func (at *AutoTrader) learnedPatternLiveGuardConfig() store.LearnedPatternLiveGuardConfig {
	if at.config.StrategyConfig == nil {
		return store.DefaultLearnedPatternLiveGuardConfig()
	}
	return at.config.StrategyConfig.RiskControl.EffectiveLearnedPatternLiveGuard()
}

func (at *AutoTrader) enforceLearnedPatternLiveGuard(record *store.DecisionRecord, actionRecord *store.DecisionAction) error {
	if at == nil || at.store == nil || record == nil || actionRecord == nil || !isOpenAction(actionRecord.Action) {
		return nil
	}

	cfg := at.learnedPatternLiveGuardConfig()
	if !cfg.Enabled {
		return nil
	}

	probe := store.BuildDealReviewLearnedPatternLiveGuardProbeFromDecisionRecord(record, actionRecord)
	if probe == nil {
		return nil
	}

	assessment, err := at.store.DealReview().EvaluateLearnedPatternLiveGuard(at.userID, at.id, *probe, cfg)
	if err != nil {
		logger.Warnf("⚠️ [%s] Learned-pattern live guard evaluation failed: %v", at.name, err)
		return nil
	}
	if assessment == nil {
		return nil
	}
	if err := at.store.DealReview().RecordLearnedPatternLiveGuardEvent(at.userID, at.id, probe, assessment); err != nil {
		logger.Warnf("⚠️ [%s] Failed to record learned-pattern live guard event: %v", at.name, err)
	}

	switch assessment.Effect {
	case store.DealReviewLearnedPatternLiveGuardEffectMonitorOnly:
		if summary := strings.TrimSpace(assessment.Summary); summary != "" {
			record.ExecutionLog = append(record.ExecutionLog, "⚠ learned pattern monitor: "+summary)
			logger.Warnf("⚠️ [%s] %s", at.name, summary)
		}
	case store.DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified:
		if summary := strings.TrimSpace(assessment.Summary); summary != "" {
			record.ExecutionLog = append(record.ExecutionLog, "ℹ learned pattern review: "+summary)
		}
	}

	if !assessment.HardBlock {
		return nil
	}

	reason := strings.TrimSpace(assessment.BlockReason)
	if reason == "" {
		reason = strings.TrimSpace(assessment.Summary)
	}
	actionRecord.RejectReasons = appendUniqueRejectReasons(actionRecord.RejectReasons, "risk_control", "learned_pattern_block")
	record.ExecutionLog = append(record.ExecutionLog, "⚠ learned pattern block: "+reason)
	logger.Warnf("⚠️ [%s] %s", at.name, reason)
	return fmt.Errorf("❌ [RISK CONTROL] learned-pattern guard blocked %s %s: %s", probe.Symbol, actionRecord.Action, reason)
}
