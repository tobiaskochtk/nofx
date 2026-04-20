package store

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	dealReviewLearnedPatternLiveGuardAttributionWindow = 12 * time.Hour

	DealReviewLearnedPatternLiveGuardAttributionStatusCorrectlyBlocked      = "correctly_blocked"
	DealReviewLearnedPatternLiveGuardAttributionStatusOverblocked           = "overblocked"
	DealReviewLearnedPatternLiveGuardAttributionStatusWarningConfirmed      = "warning_confirmed"
	DealReviewLearnedPatternLiveGuardAttributionStatusWarningNotConfirmed   = "warning_not_confirmed"
	DealReviewLearnedPatternLiveGuardAttributionStatusThresholdMissedLoss   = "threshold_missed_loss"
	DealReviewLearnedPatternLiveGuardAttributionStatusThresholdMissedProfit = "threshold_missed_profit"
	DealReviewLearnedPatternLiveGuardAttributionStatusFollowupOpen          = "followup_open"
	DealReviewLearnedPatternLiveGuardAttributionStatusPending               = "pending"
)

type DealReviewLearnedPatternLiveGuardEventAttribution struct {
	Status                     string    `json:"status,omitempty"`
	Summary                    string    `json:"summary,omitempty"`
	Resolved                   bool      `json:"resolved"`
	HorizonHours               int       `json:"horizon_hours,omitempty"`
	NextAttemptCycleNumber     int       `json:"next_attempt_cycle_number,omitempty"`
	NextAttemptAt              time.Time `json:"next_attempt_at,omitempty"`
	NextAttemptAction          string    `json:"next_attempt_action,omitempty"`
	NextAttemptTerminalStatus  string    `json:"next_attempt_terminal_status,omitempty"`
	NextAttemptFailureCategory string    `json:"next_attempt_failure_category,omitempty"`
	FollowupCaseID             string    `json:"followup_case_id,omitempty"`
	FollowupCaseStatus         string    `json:"followup_case_status,omitempty"`
	FollowupCaseOutcome        string    `json:"followup_case_outcome,omitempty"`
	FollowupRealizedPnL        float64   `json:"followup_realized_pnl,omitempty"`
	FollowupRealizedPnLPct     float64   `json:"followup_realized_pnl_pct,omitempty"`
	FollowupEntryDelayMs       int64     `json:"followup_entry_delay_ms,omitempty"`
	FollowupClosedAt           time.Time `json:"followup_closed_at,omitempty"`
}

type dealReviewLearnedPatternLiveGuardFollowupAction struct {
	RecordID        int64
	CycleNumber     int
	Timestamp       time.Time
	Symbol          string
	Side            string
	Action          string
	TerminalStatus  string
	FailureCategory string
}

func (s *DealReviewStore) enrichLearnedPatternLiveGuardEventAttribution(userID, traderID string, items []DealReviewLearnedPatternLiveGuardEvent) error {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if len(items) == 0 || traderID == "" {
		return nil
	}

	var windowStart time.Time
	var windowEnd time.Time
	for _, item := range items {
		ts := dealReviewLearnedPatternLiveGuardEventTime(&item)
		if ts.IsZero() {
			continue
		}
		if windowStart.IsZero() || ts.Before(windowStart) {
			windowStart = ts
		}
		end := ts.Add(dealReviewLearnedPatternLiveGuardAttributionWindow)
		if windowEnd.IsZero() || end.After(windowEnd) {
			windowEnd = end
		}
	}
	if windowStart.IsZero() || windowEnd.IsZero() {
		return nil
	}

	actionsByKey, err := s.loadLearnedPatternLiveGuardFollowupActions(traderID, windowStart, windowEnd)
	if err != nil {
		return err
	}
	casesByKey, err := s.loadLearnedPatternLiveGuardFollowupCases(userID, traderID, windowStart, windowEnd)
	if err != nil {
		return err
	}

	for idx := range items {
		key := dealReviewLearnedPatternLiveGuardEventKey(items[idx].Symbol, items[idx].Side)
		items[idx].Attribution = buildDealReviewLearnedPatternLiveGuardEventAttribution(
			&items[idx],
			actionsByKey[key],
			casesByKey[key],
		)
	}
	return nil
}

func (s *DealReviewStore) loadLearnedPatternLiveGuardFollowupActions(traderID string, start, end time.Time) (map[string][]dealReviewLearnedPatternLiveGuardFollowupAction, error) {
	index := make(map[string][]dealReviewLearnedPatternLiveGuardFollowupAction)
	if strings.TrimSpace(traderID) == "" || start.IsZero() || end.IsZero() || !end.After(start) {
		return index, nil
	}

	var rows []DecisionRecordDB
	if err := s.db.
		Model(&DecisionRecordDB{}).
		Where("trader_id = ? AND timestamp >= ? AND timestamp <= ?", traderID, start.UTC(), end.UTC()).
		Order("timestamp ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		record := row.toRecord()
		if record == nil {
			continue
		}
		for actionIdx := range record.Decisions {
			action := record.Decisions[actionIdx]
			if !isOpenDecisionAction(action.Action) {
				continue
			}
			key := dealReviewLearnedPatternLiveGuardEventKey(action.Symbol, decisionActionSide(action.Action))
			if key == "" {
				continue
			}
			ts := decisionActionTimestamp(record, &action)
			if ts.IsZero() {
				ts = record.Timestamp.UTC()
			}
			terminalStatus := ""
			failureCategory := ""
			if action.Execution != nil {
				terminalStatus = strings.TrimSpace(action.Execution.TerminalStatus)
				failureCategory = strings.TrimSpace(action.Execution.FailureCategory)
			}
			index[key] = append(index[key], dealReviewLearnedPatternLiveGuardFollowupAction{
				RecordID:        record.ID,
				CycleNumber:     record.CycleNumber,
				Timestamp:       ts,
				Symbol:          strings.ToUpper(strings.TrimSpace(action.Symbol)),
				Side:            normalizeDealReviewSide(decisionActionSide(action.Action)),
				Action:          strings.ToLower(strings.TrimSpace(action.Action)),
				TerminalStatus:  terminalStatus,
				FailureCategory: failureCategory,
			})
		}
	}

	for key := range index {
		sort.SliceStable(index[key], func(i, j int) bool {
			left := index[key][i]
			right := index[key][j]
			if !left.Timestamp.Equal(right.Timestamp) {
				return left.Timestamp.Before(right.Timestamp)
			}
			if left.CycleNumber != right.CycleNumber {
				return left.CycleNumber < right.CycleNumber
			}
			return left.RecordID < right.RecordID
		})
	}

	return index, nil
}

func (s *DealReviewStore) loadLearnedPatternLiveGuardFollowupCases(userID, traderID string, start, end time.Time) (map[string][]DealReviewCase, error) {
	index := make(map[string][]DealReviewCase)
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	if userID == "" || traderID == "" || start.IsZero() || end.IsZero() || !end.After(start) {
		return index, nil
	}

	var items []DealReviewCase
	if err := s.db.
		Model(&DealReviewCase{}).
		Where(
			"user_id = ? AND trader_id = ? AND entry_time_ms > 0 AND entry_time_ms >= ? AND entry_time_ms <= ?",
			userID,
			traderID,
			start.UTC().UnixMilli(),
			end.UTC().UnixMilli(),
		).
		Order("entry_time_ms ASC, updated_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}

	for _, item := range items {
		key := dealReviewLearnedPatternLiveGuardEventKey(item.Symbol, item.Side)
		if key == "" {
			continue
		}
		index[key] = append(index[key], item)
	}

	return index, nil
}

func buildDealReviewLearnedPatternLiveGuardEventAttribution(
	event *DealReviewLearnedPatternLiveGuardEvent,
	actions []dealReviewLearnedPatternLiveGuardFollowupAction,
	cases []DealReviewCase,
) *DealReviewLearnedPatternLiveGuardEventAttribution {
	if event == nil {
		return nil
	}

	relevantEffect := event.Effect == DealReviewLearnedPatternLiveGuardEffectHardBlocked ||
		event.Effect == DealReviewLearnedPatternLiveGuardEffectMonitorOnly ||
		event.Effect == DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified
	if !relevantEffect {
		return nil
	}

	eventTime := dealReviewLearnedPatternLiveGuardEventTime(event)
	if eventTime.IsZero() {
		return nil
	}
	horizonEnd := eventTime.Add(dealReviewLearnedPatternLiveGuardAttributionWindow)
	nextAction := selectDealReviewLearnedPatternLiveGuardNextAction(event, actions, eventTime, horizonEnd)
	followupCase := selectDealReviewLearnedPatternLiveGuardFollowupCase(eventTime, horizonEnd, cases)

	attr := &DealReviewLearnedPatternLiveGuardEventAttribution{
		HorizonHours: int(dealReviewLearnedPatternLiveGuardAttributionWindow.Hours()),
	}
	applyDealReviewLearnedPatternLiveGuardAttributionMeta(attr, eventTime, nextAction, followupCase)

	switch event.Effect {
	case DealReviewLearnedPatternLiveGuardEffectHardBlocked:
		switch {
		case followupCase != nil && isClosedDealReviewCase(followupCase):
			attr.Resolved = true
			if dealReviewLearnedPatternLiveGuardCaseProfited(followupCase) {
				attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusOverblocked
				attr.Summary = fmt.Sprintf(
					"Hard block looks overblocking so far: the first same-side follow-up deal opened %s later and closed %s.",
					formatDealReviewDurationShort(attr.FollowupEntryDelayMs),
					formatDealReviewLearnedPatternLiveGuardCaseOutcome(followupCase),
				)
			} else {
				attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusCorrectlyBlocked
				attr.Summary = fmt.Sprintf(
					"Hard block looks correct so far: the first same-side follow-up deal opened %s later and closed %s.",
					formatDealReviewDurationShort(attr.FollowupEntryDelayMs),
					formatDealReviewLearnedPatternLiveGuardCaseOutcome(followupCase),
				)
			}
		case followupCase != nil:
			attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusFollowupOpen
			attr.Summary = fmt.Sprintf(
				"A same-side follow-up deal opened %s later and is still open.",
				formatDealReviewDurationShort(attr.FollowupEntryDelayMs),
			)
		default:
			attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusPending
			attr.Summary = buildDealReviewLearnedPatternLiveGuardPendingSummary(nextAction, attr.HorizonHours)
		}
	case DealReviewLearnedPatternLiveGuardEffectMonitorOnly:
		switch {
		case followupCase != nil && isClosedDealReviewCase(followupCase):
			attr.Resolved = true
			if dealReviewLearnedPatternLiveGuardCaseProfited(followupCase) {
				attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusWarningNotConfirmed
				attr.Summary = fmt.Sprintf(
					"Monitor-only warning did not confirm: the realized deal closed %s.",
					formatDealReviewLearnedPatternLiveGuardCaseOutcome(followupCase),
				)
			} else {
				attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusWarningConfirmed
				attr.Summary = fmt.Sprintf(
					"Monitor-only warning confirmed: the realized deal closed %s.",
					formatDealReviewLearnedPatternLiveGuardCaseOutcome(followupCase),
				)
			}
		case followupCase != nil:
			attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusFollowupOpen
			attr.Summary = "The monitored deal is still open."
		default:
			attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusPending
			attr.Summary = buildDealReviewLearnedPatternLiveGuardPendingSummary(nextAction, attr.HorizonHours)
		}
	case DealReviewLearnedPatternLiveGuardEffectMatchedUnqualified:
		switch {
		case followupCase != nil && isClosedDealReviewCase(followupCase):
			attr.Resolved = true
			if dealReviewLearnedPatternLiveGuardCaseProfited(followupCase) {
				attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusThresholdMissedProfit
				attr.Summary = fmt.Sprintf(
					"Matched-but-not-qualified setup resolved profitably: the realized deal closed %s.",
					formatDealReviewLearnedPatternLiveGuardCaseOutcome(followupCase),
				)
			} else {
				attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusThresholdMissedLoss
				attr.Summary = fmt.Sprintf(
					"Matched-but-not-qualified setup still closed negative: the realized deal closed %s.",
					formatDealReviewLearnedPatternLiveGuardCaseOutcome(followupCase),
				)
			}
		case followupCase != nil:
			attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusFollowupOpen
			attr.Summary = "The matched-but-not-qualified follow-up deal is still open."
		default:
			attr.Status = DealReviewLearnedPatternLiveGuardAttributionStatusPending
			attr.Summary = buildDealReviewLearnedPatternLiveGuardPendingSummary(nextAction, attr.HorizonHours)
		}
	}

	return attr
}

func selectDealReviewLearnedPatternLiveGuardNextAction(
	event *DealReviewLearnedPatternLiveGuardEvent,
	actions []dealReviewLearnedPatternLiveGuardFollowupAction,
	eventTime time.Time,
	horizonEnd time.Time,
) *dealReviewLearnedPatternLiveGuardFollowupAction {
	if event == nil || len(actions) == 0 {
		return nil
	}
	for idx := range actions {
		action := actions[idx]
		if action.Timestamp.Before(eventTime) || action.Timestamp.After(horizonEnd) {
			continue
		}
		if action.Timestamp.Equal(eventTime) && action.CycleNumber <= event.CycleNumber {
			continue
		}
		return &action
	}
	return nil
}

func selectDealReviewLearnedPatternLiveGuardFollowupCase(eventTime, horizonEnd time.Time, cases []DealReviewCase) *DealReviewCase {
	if eventTime.IsZero() || len(cases) == 0 {
		return nil
	}
	startMs := eventTime.UTC().UnixMilli()
	endMs := horizonEnd.UTC().UnixMilli()
	for idx := range cases {
		caseRec := cases[idx]
		if caseRec.EntryTimeMs <= 0 {
			continue
		}
		if caseRec.EntryTimeMs < startMs || caseRec.EntryTimeMs > endMs {
			continue
		}
		return &caseRec
	}
	return nil
}

func applyDealReviewLearnedPatternLiveGuardAttributionMeta(
	attr *DealReviewLearnedPatternLiveGuardEventAttribution,
	eventTime time.Time,
	nextAction *dealReviewLearnedPatternLiveGuardFollowupAction,
	followupCase *DealReviewCase,
) {
	if attr == nil {
		return
	}
	if nextAction != nil {
		attr.NextAttemptCycleNumber = nextAction.CycleNumber
		attr.NextAttemptAt = nextAction.Timestamp
		attr.NextAttemptAction = nextAction.Action
		attr.NextAttemptTerminalStatus = nextAction.TerminalStatus
		attr.NextAttemptFailureCategory = nextAction.FailureCategory
	}
	if followupCase != nil {
		attr.FollowupCaseID = followupCase.ID
		attr.FollowupCaseStatus = strings.TrimSpace(followupCase.Status)
		attr.FollowupCaseOutcome = strings.TrimSpace(followupCase.Outcome)
		attr.FollowupRealizedPnL = followupCase.RealizedPnL
		attr.FollowupRealizedPnLPct = followupCase.RealizedPnLPct
		attr.FollowupEntryDelayMs = maxInt64(0, followupCase.EntryTimeMs-eventTime.UTC().UnixMilli())
		if followupCase.ExitTimeMs > 0 {
			attr.FollowupClosedAt = time.UnixMilli(followupCase.ExitTimeMs).UTC()
		}
	}
}

func buildDealReviewLearnedPatternLiveGuardPendingSummary(nextAction *dealReviewLearnedPatternLiveGuardFollowupAction, horizonHours int) string {
	if nextAction != nil {
		status := blankToValue(nextAction.TerminalStatus, "unknown")
		return fmt.Sprintf(
			"No closed same-side follow-up deal was observed within %dh yet. The next attempt reached %s on cycle %d.",
			horizonHours,
			status,
			nextAction.CycleNumber,
		)
	}
	return fmt.Sprintf(
		"No same-side follow-up deal was observed within %dh yet.",
		horizonHours,
	)
}

func dealReviewLearnedPatternLiveGuardEventKey(symbol, side string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	side = normalizeDealReviewSide(side)
	if symbol == "" || side == "" {
		return ""
	}
	return symbol + "|" + side
}

func dealReviewLearnedPatternLiveGuardEventTime(event *DealReviewLearnedPatternLiveGuardEvent) time.Time {
	if event == nil {
		return time.Time{}
	}
	if !event.DecisionTimestamp.IsZero() {
		return event.DecisionTimestamp.UTC()
	}
	if !event.CreatedAt.IsZero() {
		return event.CreatedAt.UTC()
	}
	return time.Time{}
}

func dealReviewLearnedPatternLiveGuardCaseProfited(caseRec *DealReviewCase) bool {
	if caseRec == nil {
		return false
	}
	outcome := strings.ToLower(strings.TrimSpace(caseRec.Outcome))
	switch outcome {
	case "profit", "win":
		return true
	case "loss":
		return false
	}
	return caseRec.RealizedPnL > 0 || caseRec.RealizedPnLPct > 0
}

func isClosedDealReviewCase(caseRec *DealReviewCase) bool {
	if caseRec == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(caseRec.Status), DealReviewCaseStatusClosed) ||
		caseRec.ExitTimeMs > 0
}

func formatDealReviewLearnedPatternLiveGuardCaseOutcome(caseRec *DealReviewCase) string {
	if caseRec == nil {
		return "with unknown outcome"
	}
	return fmt.Sprintf(
		"%s (%+.2f / %+.2f%%)",
		blankToValue(strings.ToLower(strings.TrimSpace(caseRec.Outcome)), classifyDealOutcome(caseRec.RealizedPnL)),
		caseRec.RealizedPnL,
		caseRec.RealizedPnLPct,
	)
}

func formatDealReviewDurationShort(ms int64) string {
	if ms <= 0 {
		return "0m"
	}
	minutes := ms / int64(time.Minute)
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	remainder := minutes % 60
	if hours < 24 {
		if remainder == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, remainder)
	}
	days := hours / 24
	hourRemainder := hours % 24
	if hourRemainder == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd %dh", days, hourRemainder)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
