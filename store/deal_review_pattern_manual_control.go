package store

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DealReviewLearnedPatternManualControlStateLive       = "live"
	DealReviewLearnedPatternManualControlStateSuppressed = "suppressed"
	DealReviewLearnedPatternManualControlStateRetired    = "retired"

	DealReviewLearnedPatternManualControlActionAcknowledgeLive = "acknowledge_live"
	DealReviewLearnedPatternManualControlActionSuppress        = "suppress"
	DealReviewLearnedPatternManualControlActionRetire          = "retire"
	DealReviewLearnedPatternManualControlActionRearm           = "rearm"

	dealReviewLearnedPatternManualControlHistoryLimit = 4
)

type DealReviewLearnedPatternManualControl struct {
	ID                      string    `gorm:"primaryKey" json:"id"`
	UserID                  string    `gorm:"column:user_id;not null;index:idx_pattern_manual_control_user_trader,priority:1" json:"user_id"`
	TraderID                string    `gorm:"column:trader_id;not null;index:idx_pattern_manual_control_user_trader,priority:2;index:idx_pattern_manual_control_ref" json:"trader_id"`
	PatternID               string    `gorm:"column:pattern_id;default:'';index:idx_pattern_manual_control_pattern_id" json:"pattern_id,omitempty"`
	PatternStableKey        string    `gorm:"column:pattern_stable_key;default:'';index:idx_pattern_manual_control_ref" json:"pattern_stable_key,omitempty"`
	PatternSignature        string    `gorm:"column:pattern_signature;type:text;default:''" json:"pattern_signature,omitempty"`
	PatternClass            string    `gorm:"column:pattern_class;default:''" json:"pattern_class,omitempty"`
	ScopeType               string    `gorm:"column:scope_type;default:''" json:"scope_type,omitempty"`
	Symbol                  string    `gorm:"column:symbol;default:''" json:"symbol,omitempty"`
	Side                    string    `gorm:"column:side;default:''" json:"side,omitempty"`
	ControlState            string    `gorm:"column:control_state;default:'';index:idx_pattern_manual_control_state" json:"control_state"`
	LastAction              string    `gorm:"column:last_action;default:''" json:"last_action"`
	Note                    string    `gorm:"column:note;type:text;default:''" json:"note,omitempty"`
	BaseRecommendedUse      string    `gorm:"column:base_recommended_use;default:''" json:"base_recommended_use,omitempty"`
	EffectiveRecommendedUse string    `gorm:"column:effective_recommended_use;default:''" json:"effective_recommended_use,omitempty"`
	LifecycleStatus         string    `gorm:"column:lifecycle_status;default:''" json:"lifecycle_status,omitempty"`
	AppliedAt               time.Time `gorm:"column:applied_at;index:idx_pattern_manual_control_applied" json:"applied_at"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func (DealReviewLearnedPatternManualControl) TableName() string {
	return "deal_review_learned_pattern_manual_controls"
}

type DealReviewLearnedPatternManualControlEvent struct {
	ID                      string    `gorm:"primaryKey" json:"id"`
	UserID                  string    `gorm:"column:user_id;not null;index:idx_pattern_manual_control_event_user_trader,priority:1" json:"user_id"`
	TraderID                string    `gorm:"column:trader_id;not null;index:idx_pattern_manual_control_event_user_trader,priority:2;index:idx_pattern_manual_control_event_ref" json:"trader_id"`
	PatternID               string    `gorm:"column:pattern_id;default:'';index:idx_pattern_manual_control_event_pattern_id" json:"pattern_id,omitempty"`
	PatternStableKey        string    `gorm:"column:pattern_stable_key;default:'';index:idx_pattern_manual_control_event_ref" json:"pattern_stable_key,omitempty"`
	PatternSignature        string    `gorm:"column:pattern_signature;type:text;default:''" json:"pattern_signature,omitempty"`
	PatternClass            string    `gorm:"column:pattern_class;default:''" json:"pattern_class,omitempty"`
	Symbol                  string    `gorm:"column:symbol;default:''" json:"symbol,omitempty"`
	Side                    string    `gorm:"column:side;default:''" json:"side,omitempty"`
	Action                  string    `gorm:"column:action;default:'';index:idx_pattern_manual_control_event_action" json:"action"`
	ControlState            string    `gorm:"column:control_state;default:''" json:"control_state"`
	Note                    string    `gorm:"column:note;type:text;default:''" json:"note,omitempty"`
	BaseRecommendedUse      string    `gorm:"column:base_recommended_use;default:''" json:"base_recommended_use,omitempty"`
	EffectiveRecommendedUse string    `gorm:"column:effective_recommended_use;default:''" json:"effective_recommended_use,omitempty"`
	LifecycleStatus         string    `gorm:"column:lifecycle_status;default:''" json:"lifecycle_status,omitempty"`
	AppliedAt               time.Time `gorm:"column:applied_at;index:idx_pattern_manual_control_event_applied" json:"applied_at"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func (DealReviewLearnedPatternManualControlEvent) TableName() string {
	return "deal_review_learned_pattern_manual_control_events"
}

func normalizeDealReviewLearnedPatternManualControlState(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternManualControlStateLive:
		return DealReviewLearnedPatternManualControlStateLive
	case DealReviewLearnedPatternManualControlStateSuppressed:
		return DealReviewLearnedPatternManualControlStateSuppressed
	case DealReviewLearnedPatternManualControlStateRetired:
		return DealReviewLearnedPatternManualControlStateRetired
	default:
		return ""
	}
}

func normalizeDealReviewLearnedPatternManualControlAction(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DealReviewLearnedPatternManualControlActionAcknowledgeLive:
		return DealReviewLearnedPatternManualControlActionAcknowledgeLive
	case DealReviewLearnedPatternManualControlActionSuppress:
		return DealReviewLearnedPatternManualControlActionSuppress
	case DealReviewLearnedPatternManualControlActionRetire:
		return DealReviewLearnedPatternManualControlActionRetire
	case DealReviewLearnedPatternManualControlActionRearm:
		return DealReviewLearnedPatternManualControlActionRearm
	default:
		return ""
	}
}

func dealReviewLearnedPatternManualControlStateFromAction(action string) string {
	switch normalizeDealReviewLearnedPatternManualControlAction(action) {
	case DealReviewLearnedPatternManualControlActionSuppress:
		return DealReviewLearnedPatternManualControlStateSuppressed
	case DealReviewLearnedPatternManualControlActionRetire:
		return DealReviewLearnedPatternManualControlStateRetired
	case DealReviewLearnedPatternManualControlActionAcknowledgeLive,
		DealReviewLearnedPatternManualControlActionRearm:
		return DealReviewLearnedPatternManualControlStateLive
	default:
		return ""
	}
}

func baseDealReviewLearnedPatternRecommendedUse(item *DealReviewLearnedPattern) string {
	if item == nil {
		return ""
	}
	if base := normalizeDealReviewLearnedPatternRecommendedUse(item.BaseRecommendedUse); base != "" {
		return base
	}
	raw := normalizeDealReviewLearnedPatternRecommendedUse(item.RecommendedUse)
	if raw == "" {
		raw = buildDealReviewLearnedPatternRecommendedUse(
			item.PatternClass,
			item.ValidationLabel,
			item.CompositeScore,
			item.ConfidenceScore,
			item.ValidationSupportScore,
			item.FalsePositiveScore,
			item.DriftScore,
			item.SampleCount,
			math.Abs(item.LiftAvgPnLPct),
		)
	}
	if raw == DealReviewLearnedPatternRecommendedUsePromptHint &&
		isDealReviewLearnedPatternMonitoringRuleCandidate(
			item.PatternClass,
			item.ValidationLabel,
			item.CompositeScore,
			item.ConfidenceScore,
			item.ValidationSupportScore,
			item.FalsePositiveScore,
			item.DriftScore,
			item.SampleCount,
			math.Abs(item.LiftAvgPnLPct),
		) {
		raw = DealReviewLearnedPatternRecommendedUseMonitoringRule
	}
	return raw
}

func effectiveDealReviewLearnedPatternRecommendedUse(item *DealReviewLearnedPattern) string {
	if item == nil {
		return ""
	}
	base := baseDealReviewLearnedPatternRecommendedUse(item)
	switch normalizeDealReviewLearnedPatternManualControlState(dealReviewLearnedPatternManualControlState(item.ManualControl)) {
	case DealReviewLearnedPatternManualControlStateSuppressed:
		return DealReviewLearnedPatternRecommendedUseMonitorOnly
	case DealReviewLearnedPatternManualControlStateRetired:
		return DealReviewLearnedPatternRecommendedUseExpiredIgnore
	default:
		return base
	}
}

func hydrateDealReviewLearnedPatternManualControls(item *DealReviewLearnedPattern) {
	if item == nil {
		return
	}
	item.BaseRecommendedUse = baseDealReviewLearnedPatternRecommendedUse(item)
	item.RecommendedUse = effectiveDealReviewLearnedPatternRecommendedUse(item)
}

func dealReviewLearnedPatternManualControlState(control *DealReviewLearnedPatternManualControl) string {
	if control == nil {
		return ""
	}
	return strings.TrimSpace(control.ControlState)
}

func (s *DealReviewStore) hydrateLearnedPatternManualControls(
	userID string,
	traderID string,
	items []DealReviewLearnedPattern,
) error {
	return s.hydrateLearnedPatternManualControlsWithDB(
		s.db,
		userID,
		traderID,
		items,
		dealReviewLearnedPatternManualControlHistoryLimit,
	)
}

func (s *DealReviewStore) hydrateLearnedPatternManualControlsWithDB(
	db *gorm.DB,
	userID string,
	traderID string,
	items []DealReviewLearnedPattern,
	historyLimit int,
) error {
	for idx := range items {
		hydrateDealReviewLearnedPattern(&items[idx])
	}
	if db == nil || len(items) == 0 {
		return nil
	}
	if historyLimit <= 0 || historyLimit > 12 {
		historyLimit = dealReviewLearnedPatternManualControlHistoryLimit
	}

	refs := make([]string, 0, len(items))
	patternIDs := make([]string, 0, len(items))
	seenRefs := make(map[string]struct{}, len(items))
	seenIDs := make(map[string]struct{}, len(items))
	for idx := range items {
		ref := dealReviewLearnedPatternLifecycleReference(&items[idx])
		if ref != "" {
			if _, ok := seenRefs[ref]; !ok {
				seenRefs[ref] = struct{}{}
				refs = append(refs, ref)
			}
		}
		patternID := strings.TrimSpace(items[idx].ID)
		if patternID != "" {
			if _, ok := seenIDs[patternID]; !ok {
				seenIDs[patternID] = struct{}{}
				patternIDs = append(patternIDs, patternID)
			}
		}
	}
	if len(refs) == 0 && len(patternIDs) == 0 {
		for idx := range items {
			hydrateDealReviewLearnedPatternManualControls(&items[idx])
		}
		return nil
	}

	var controls []DealReviewLearnedPatternManualControl
	if err := db.Model(&DealReviewLearnedPatternManualControl{}).
		Where(
			"user_id = ? AND trader_id = ? AND ((pattern_stable_key <> '' AND pattern_stable_key IN ?) OR ((pattern_stable_key = '' OR pattern_stable_key IS NULL) AND pattern_id IN ?))",
			strings.TrimSpace(userID),
			strings.TrimSpace(traderID),
			refs,
			patternIDs,
		).
		Order("applied_at DESC, created_at DESC").
		Find(&controls).Error; err != nil {
		return err
	}

	controlByRef := make(map[string]DealReviewLearnedPatternManualControl, len(controls))
	for _, control := range controls {
		ref := strings.TrimSpace(control.PatternStableKey)
		if ref == "" {
			ref = strings.TrimSpace(control.PatternID)
		}
		if ref == "" {
			continue
		}
		if _, ok := controlByRef[ref]; ok {
			continue
		}
		controlByRef[ref] = control
	}

	var events []DealReviewLearnedPatternManualControlEvent
	if err := db.Model(&DealReviewLearnedPatternManualControlEvent{}).
		Where(
			"user_id = ? AND trader_id = ? AND ((pattern_stable_key <> '' AND pattern_stable_key IN ?) OR ((pattern_stable_key = '' OR pattern_stable_key IS NULL) AND pattern_id IN ?))",
			strings.TrimSpace(userID),
			strings.TrimSpace(traderID),
			refs,
			patternIDs,
		).
		Order("applied_at DESC, created_at DESC").
		Find(&events).Error; err != nil {
		return err
	}
	historyByRef := make(map[string][]DealReviewLearnedPatternManualControlEvent, len(refs))
	for _, event := range events {
		ref := strings.TrimSpace(event.PatternStableKey)
		if ref == "" {
			ref = strings.TrimSpace(event.PatternID)
		}
		if ref == "" || len(historyByRef[ref]) >= historyLimit {
			continue
		}
		historyByRef[ref] = append(historyByRef[ref], event)
	}

	for idx := range items {
		ref := dealReviewLearnedPatternLifecycleReference(&items[idx])
		if control, ok := controlByRef[ref]; ok {
			controlCopy := control
			items[idx].ManualControl = &controlCopy
		}
		if history, ok := historyByRef[ref]; ok && len(history) > 0 {
			items[idx].ManualControlHistory = append(
				[]DealReviewLearnedPatternManualControlEvent(nil),
				history...,
			)
		}
		hydrateDealReviewLearnedPatternManualControls(&items[idx])
	}
	return nil
}

func (s *DealReviewStore) ApplyLearnedPatternManualControl(
	userID string,
	traderID string,
	patternID string,
	stableKey string,
	action string,
	note string,
) (*DealReviewLearnedPatternManualControl, error) {
	userID = strings.TrimSpace(userID)
	traderID = strings.TrimSpace(traderID)
	patternID = strings.TrimSpace(patternID)
	stableKey = strings.TrimSpace(stableKey)
	action = normalizeDealReviewLearnedPatternManualControlAction(action)
	note = strings.TrimSpace(note)
	if userID == "" || traderID == "" {
		return nil, fmt.Errorf("missing user or trader")
	}
	if action == "" {
		return nil, fmt.Errorf("invalid control action")
	}
	if note == "" {
		return nil, fmt.Errorf("note is required")
	}

	var saved DealReviewLearnedPatternManualControl
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var pattern DealReviewLearnedPattern
		query := tx.Where("user_id = ? AND trader_id = ?", userID, traderID)
		switch {
		case stableKey != "" && patternID != "":
			query = query.Where("(stable_key = ? OR id = ?)", stableKey, patternID)
		case stableKey != "":
			query = query.Where("stable_key = ?", stableKey)
		case patternID != "":
			query = query.Where("id = ?", patternID)
		default:
			return fmt.Errorf("pattern reference is required")
		}
		if err := query.First(&pattern).Error; err != nil {
			return err
		}
		hydrateDealReviewLearnedPattern(&pattern)
		if strings.TrimSpace(pattern.StableKey) == "" {
			pattern.StableKey = dealReviewLearnedPatternStableKey(&pattern)
			if pattern.StableKey != "" {
				if err := tx.Model(&DealReviewLearnedPattern{}).
					Where("id = ?", pattern.ID).
					Update("stable_key", pattern.StableKey).Error; err != nil {
					return err
				}
			}
		}
		working := []DealReviewLearnedPattern{pattern}
		if err := s.hydrateLearnedPatternManualControlsWithDB(tx, userID, traderID, working, 1); err != nil {
			return err
		}
		pattern = working[0]
		working = []DealReviewLearnedPattern{pattern}
		if err := s.enrichLearnedPatternLifecycleWithDB(tx, userID, traderID, working); err != nil {
			return err
		}
		pattern = working[0]
		working = []DealReviewLearnedPattern{pattern}
		if err := s.hydrateLearnedPatternLiveGuardAttributionWithDB(
			tx,
			userID,
			traderID,
			working,
			dealReviewLearnedPatternLiveGuardRollupEventLimit,
		); err != nil {
			return err
		}
		pattern = working[0]
		pattern.ActionHint = buildDealReviewLearnedPatternActionHint(&pattern)
		pattern.LiveActionHint = buildDealReviewLearnedPatternLiveActionHint(&pattern)
		baseUse := baseDealReviewLearnedPatternRecommendedUse(&pattern)
		currentActionHint := pattern.ActionHint
		currentLiveActionHint := pattern.LiveActionHint
		controlState := dealReviewLearnedPatternManualControlStateFromAction(action)
		pattern.ManualControl = &DealReviewLearnedPatternManualControl{ControlState: controlState}
		effectiveUse := effectiveDealReviewLearnedPatternRecommendedUse(&pattern)
		lifecycleStatus := ""
		if pattern.Lifecycle != nil {
			lifecycleStatus = strings.TrimSpace(pattern.Lifecycle.Status)
		}

		var existing DealReviewLearnedPatternManualControl
		existingFound := false
		if err := tx.Where(
			"user_id = ? AND trader_id = ? AND pattern_stable_key = ?",
			userID,
			traderID,
			strings.TrimSpace(pattern.StableKey),
		).First(&existing).Error; err != nil {
			if !errorsIsRecordNotFound(err) {
				return err
			}
		} else {
			existingFound = true
		}

		now := time.Now().UTC()
		current := DealReviewLearnedPatternManualControl{
			ID:                      uuid.NewString(),
			UserID:                  userID,
			TraderID:                traderID,
			PatternID:               strings.TrimSpace(pattern.ID),
			PatternStableKey:        strings.TrimSpace(pattern.StableKey),
			PatternSignature:        strings.TrimSpace(pattern.PatternSignature),
			PatternClass:            strings.TrimSpace(pattern.PatternClass),
			ScopeType:               strings.TrimSpace(pattern.ScopeType),
			Symbol:                  strings.TrimSpace(pattern.Symbol),
			Side:                    strings.TrimSpace(pattern.Side),
			ControlState:            controlState,
			LastAction:              action,
			Note:                    note,
			BaseRecommendedUse:      baseUse,
			EffectiveRecommendedUse: effectiveUse,
			LifecycleStatus:         lifecycleStatus,
			AppliedAt:               now,
		}
		if existingFound {
			current.ID = existing.ID
			if err := tx.Model(&DealReviewLearnedPatternManualControl{}).
				Where("id = ?", existing.ID).
				Updates(map[string]any{
					"pattern_id":                current.PatternID,
					"pattern_stable_key":        current.PatternStableKey,
					"pattern_signature":         current.PatternSignature,
					"pattern_class":             current.PatternClass,
					"scope_type":                current.ScopeType,
					"symbol":                    current.Symbol,
					"side":                      current.Side,
					"control_state":             current.ControlState,
					"last_action":               current.LastAction,
					"note":                      current.Note,
					"base_recommended_use":      current.BaseRecommendedUse,
					"effective_recommended_use": current.EffectiveRecommendedUse,
					"lifecycle_status":          current.LifecycleStatus,
					"applied_at":                current.AppliedAt,
				}).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Create(&current).Error; err != nil {
				return err
			}
		}

		event := DealReviewLearnedPatternManualControlEvent{
			ID:                      uuid.NewString(),
			UserID:                  userID,
			TraderID:                traderID,
			PatternID:               current.PatternID,
			PatternStableKey:        current.PatternStableKey,
			PatternSignature:        current.PatternSignature,
			PatternClass:            current.PatternClass,
			Symbol:                  current.Symbol,
			Side:                    current.Side,
			Action:                  action,
			ControlState:            controlState,
			Note:                    note,
			BaseRecommendedUse:      baseUse,
			EffectiveRecommendedUse: effectiveUse,
			LifecycleStatus:         lifecycleStatus,
			AppliedAt:               now,
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		pattern.ActionHint = currentActionHint
		pattern.LiveActionHint = currentLiveActionHint
		if err := s.recordLearnedPatternManualInterventionTx(
			tx,
			userID,
			traderID,
			&pattern,
			action,
			note,
			event.ID,
		); err != nil {
			return err
		}
		saved = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func errorsIsRecordNotFound(err error) bool {
	return err != nil && errors.Is(err, gorm.ErrRecordNotFound)
}
