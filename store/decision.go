package store

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	reDecisionInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")
	reDecisionJSONFence      = regexp.MustCompile("(?is)^\\s*```(?:json|jsonc)\\s*([\\s\\S]*?)\\s*```\\s*$")
)

// DecisionStore decision log storage
type DecisionStore struct {
	db *gorm.DB
}

// DecisionRecordDB internal GORM model for decision_records table
type DecisionRecordDB struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement"`
	TraderID            string    `gorm:"column:trader_id;not null;index:idx_decision_records_trader_time"`
	CycleNumber         int       `gorm:"column:cycle_number;not null"`
	Timestamp           time.Time `gorm:"not null;index:idx_decision_records_trader_time,sort:desc;index:idx_decision_records_timestamp,sort:desc"`
	SystemPrompt        string    `gorm:"column:system_prompt;default:''"`
	InputPrompt         string    `gorm:"column:input_prompt;default:''"`
	CoTTrace            string    `gorm:"column:cot_trace;default:''"`
	DecisionJSON        string    `gorm:"column:decision_json;default:''"`
	RawResponse         string    `gorm:"column:raw_response;default:''"`
	AccountStateJSON    string    `gorm:"column:account_state_json;type:text;default:'{}'"`
	PositionsJSON       string    `gorm:"column:positions_json;type:text;default:'[]'"`
	CandidateCoins      string    `gorm:"column:candidate_coins;default:''"`
	CandidateDetails    string    `gorm:"column:candidate_details;type:text;default:'[]'"`
	CandidateMetaVer    int       `gorm:"column:candidate_metadata_version;default:0"`
	ExecutionLog        string    `gorm:"column:execution_log;default:''"`
	Decisions           string    `gorm:"column:decisions;default:'[]'"`
	Success             bool      `gorm:"default:false"`
	ErrorMessage        string    `gorm:"column:error_message;default:''"`
	AIRequestDurationMs int64     `gorm:"column:ai_request_duration_ms;default:0"`
	CreatedAt           time.Time `json:"created_at"`
}

func (DecisionRecordDB) TableName() string { return "decision_records" }

// DecisionRecord decision record (external API struct)
type DecisionRecord struct {
	ID                  int64              `json:"id"`
	TraderID            string             `json:"trader_id"`
	CycleNumber         int                `json:"cycle_number"`
	Timestamp           time.Time          `json:"timestamp"`
	SystemPrompt        string             `json:"system_prompt"`
	InputPrompt         string             `json:"input_prompt"`
	CoTTrace            string             `json:"cot_trace"`
	DecisionJSON        string             `json:"decision_json"`
	RawResponse         string             `json:"raw_response"` // Raw AI response for debugging
	CandidateCoins      []string           `json:"candidate_coins"`
	CandidateDetails    []CandidateDetail  `json:"candidate_details,omitempty"`
	CandidateMetaVer    int                `json:"candidate_metadata_version"`
	ExecutionLog        []string           `json:"execution_log"`
	Success             bool               `json:"success"`
	ErrorMessage        string             `json:"error_message"`
	AIRequestDurationMs int64              `json:"ai_request_duration_ms"`
	AccountState        AccountSnapshot    `json:"account_state"`
	Positions           []PositionSnapshot `json:"positions"`
	Decisions           []DecisionAction   `json:"decisions"`
}

const DecisionCandidateMetadataVersion = 1

// CandidateDetail keeps per-cycle candidate metadata needed for later review.
type CandidateDetail struct {
	Symbol          string   `json:"symbol"`
	Sources         []string `json:"sources,omitempty"`
	SelectionBucket string   `json:"selection_bucket,omitempty"`
}

// TraderBucketReview is a live 24h-style review aggregated from stored decision records.
type TraderBucketReview struct {
	TraderID                string                 `json:"trader_id"`
	WindowHours             int                    `json:"window_hours"`
	WindowStart             time.Time              `json:"window_start"`
	WindowEnd               time.Time              `json:"window_end"`
	GeneratedAt             time.Time              `json:"generated_at"`
	CandidateMetaVer        int                    `json:"candidate_metadata_version"`
	HasFullWindow           bool                   `json:"has_full_window"`
	CoverageHours           float64                `json:"coverage_hours"`
	RecordCount             int                    `json:"record_count"`
	LegacyRecordCount       int                    `json:"legacy_record_count"`
	CyclesWithCandidates    int                    `json:"cycles_with_candidates"`
	CyclesWithoutCandidates int                    `json:"cycles_without_candidates"`
	CyclesWithOpenDecisions int                    `json:"cycles_with_open_decisions"`
	TotalCandidates         int                    `json:"total_candidates"`
	TotalOpenDecisions      int                    `json:"total_open_decisions"`
	HoldDecisionCount       int                    `json:"hold_decision_count"`
	WaitDecisionCount       int                    `json:"wait_decision_count"`
	DecisionConversionRate  float64                `json:"decision_conversion_rate"`
	AvgDecisionConfidence   float64                `json:"avg_decision_confidence"`
	FirstRecordAt           *time.Time             `json:"first_record_at,omitempty"`
	LastRecordAt            *time.Time             `json:"last_record_at,omitempty"`
	Buckets                 []TraderBucketSummary  `json:"buckets"`
	RecentCycles            []TraderBucketCycle    `json:"recent_cycles"`
	RejectReasons           []TraderRejectReason   `json:"reject_reasons,omitempty"`
	ConfidenceBands         []TraderConfidenceBand `json:"confidence_bands,omitempty"`
	OpportunitySessions     []TraderSessionSummary `json:"opportunity_sessions,omitempty"`
	OpportunitySymbols      []TraderSymbolSummary  `json:"opportunity_symbols,omitempty"`
}

// TraderBucketSummary aggregates usage for a single selection bucket.
type TraderBucketSummary struct {
	Name              string   `json:"name"`
	CandidateCount    int      `json:"candidate_count"`
	UniqueSymbols     int      `json:"unique_symbols"`
	OpenDecisionCount int      `json:"open_decision_count"`
	SymbolSamples     []string `json:"symbol_samples,omitempty"`
}

// TraderBucketCycle captures a single cycle inside the review window.
type TraderBucketCycle struct {
	CycleNumber         int                      `json:"cycle_number"`
	Timestamp           time.Time                `json:"timestamp"`
	CandidateCount      int                      `json:"candidate_count"`
	OpenDecisionSymbols []string                 `json:"open_decision_symbols,omitempty"`
	Buckets             []TraderBucketCycleBreak `json:"buckets"`
}

// TraderBucketCycleBreak contains the bucket mix for one cycle.
type TraderBucketCycleBreak struct {
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	Symbols []string `json:"symbols,omitempty"`
}

type TraderRejectReason struct {
	Reason   string  `json:"reason"`
	Count    int     `json:"count"`
	SharePct float64 `json:"share_pct,omitempty"`
}

type TraderConfidenceBand struct {
	Band              string  `json:"band"`
	DecisionCount     int     `json:"decision_count"`
	OpenDecisionCount int     `json:"open_decision_count"`
	HoldDecisionCount int     `json:"hold_decision_count"`
	WaitDecisionCount int     `json:"wait_decision_count"`
	AvgConfidence     float64 `json:"avg_confidence,omitempty"`
}

type TraderSessionSummary struct {
	Session           string  `json:"session"`
	CycleCount        int     `json:"cycle_count"`
	CandidateCount    int     `json:"candidate_count"`
	OpenDecisionCount int     `json:"open_decision_count"`
	HoldDecisionCount int     `json:"hold_decision_count"`
	WaitDecisionCount int     `json:"wait_decision_count"`
	AvgConfidence     float64 `json:"avg_confidence,omitempty"`
}

type TraderSymbolSummary struct {
	Symbol            string   `json:"symbol"`
	CandidateCount    int      `json:"candidate_count"`
	OpenDecisionCount int      `json:"open_decision_count"`
	HoldDecisionCount int      `json:"hold_decision_count"`
	WaitDecisionCount int      `json:"wait_decision_count"`
	AvgConfidence     float64  `json:"avg_confidence,omitempty"`
	Sessions          []string `json:"sessions,omitempty"`
	SelectionBuckets  []string `json:"selection_buckets,omitempty"`
}

// AccountSnapshot account state snapshot
type AccountSnapshot struct {
	TotalBalance          float64 `json:"total_balance"`
	AvailableBalance      float64 `json:"available_balance"`
	TotalUnrealizedProfit float64 `json:"total_unrealized_profit"`
	PositionCount         int     `json:"position_count"`
	MarginUsedPct         float64 `json:"margin_used_pct"`
	InitialBalance        float64 `json:"initial_balance"`
}

// PositionSnapshot position snapshot
type PositionSnapshot struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"`
	PositionAmt      float64 `json:"position_amt"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	UnrealizedProfit float64 `json:"unrealized_profit"`
	Leverage         float64 `json:"leverage"`
	LiquidationPrice float64 `json:"liquidation_price"`
}

// DecisionAction decision action
type DecisionAction struct {
	Action     string    `json:"action"`
	Symbol     string    `json:"symbol"`
	Quantity   float64   `json:"quantity"`
	Leverage   int       `json:"leverage"`
	Price      float64   `json:"price"`
	StopLoss   float64   `json:"stop_loss,omitempty"`   // Stop loss price
	TakeProfit float64   `json:"take_profit,omitempty"` // Take profit price
	Confidence int       `json:"confidence,omitempty"`  // AI confidence (0-100)
	Reasoning  string    `json:"reasoning,omitempty"`   // Brief reasoning
	OrderID    int64     `json:"order_id"`
	Timestamp  time.Time `json:"timestamp"`
	Success    bool      `json:"success"`
	Error      string    `json:"error"`
}

// Statistics statistics information
type Statistics struct {
	TotalCycles         int `json:"total_cycles"`
	SuccessfulCycles    int `json:"successful_cycles"`
	FailedCycles        int `json:"failed_cycles"`
	TotalOpenPositions  int `json:"total_open_positions"`
	TotalClosePositions int `json:"total_close_positions"`
}

// NewDecisionStore creates a new DecisionStore
func NewDecisionStore(db *gorm.DB) *DecisionStore {
	return &DecisionStore{db: db}
}

// initTables initializes AI decision log tables
func (s *DecisionStore) initTables() error {
	// For PostgreSQL with existing table, skip AutoMigrate
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'decision_records'`).Scan(&tableExists)
		if tableExists == 0 {
			if err := s.db.AutoMigrate(&DecisionRecordDB{}); err != nil {
				return err
			}
		}
		return s.ensureDecisionRecordColumns()
	}
	if err := s.db.AutoMigrate(&DecisionRecordDB{}); err != nil {
		return err
	}
	return s.ensureDecisionRecordColumns()
}

func (s *DecisionStore) ensureDecisionRecordColumns() error {
	if !s.db.Migrator().HasColumn(&DecisionRecordDB{}, "AccountStateJSON") {
		if err := s.db.Migrator().AddColumn(&DecisionRecordDB{}, "AccountStateJSON"); err != nil {
			return fmt.Errorf("failed to add account_state_json column: %w", err)
		}
	}
	if !s.db.Migrator().HasColumn(&DecisionRecordDB{}, "PositionsJSON") {
		if err := s.db.Migrator().AddColumn(&DecisionRecordDB{}, "PositionsJSON"); err != nil {
			return fmt.Errorf("failed to add positions_json column: %w", err)
		}
	}
	if !s.db.Migrator().HasColumn(&DecisionRecordDB{}, "CandidateDetails") {
		if err := s.db.Migrator().AddColumn(&DecisionRecordDB{}, "CandidateDetails"); err != nil {
			return fmt.Errorf("failed to add candidate_details column: %w", err)
		}
	}
	if !s.db.Migrator().HasColumn(&DecisionRecordDB{}, "CandidateMetaVer") {
		if err := s.db.Migrator().AddColumn(&DecisionRecordDB{}, "CandidateMetaVer"); err != nil {
			return fmt.Errorf("failed to add candidate_metadata_version column: %w", err)
		}
	}
	return nil
}

// toRecord converts DB model to API struct
func (db *DecisionRecordDB) toRecord() *DecisionRecord {
	record := &DecisionRecord{
		ID:                  db.ID,
		TraderID:            db.TraderID,
		CycleNumber:         db.CycleNumber,
		Timestamp:           db.Timestamp,
		SystemPrompt:        db.SystemPrompt,
		InputPrompt:         db.InputPrompt,
		CoTTrace:            sanitizeDecisionCoTTrace(db.CoTTrace),
		DecisionJSON:        db.DecisionJSON,
		RawResponse:         db.RawResponse,
		CandidateMetaVer:    db.CandidateMetaVer,
		Success:             db.Success,
		ErrorMessage:        db.ErrorMessage,
		AIRequestDurationMs: db.AIRequestDurationMs,
	}
	json.Unmarshal([]byte(db.CandidateCoins), &record.CandidateCoins)
	json.Unmarshal([]byte(db.CandidateDetails), &record.CandidateDetails)
	json.Unmarshal([]byte(db.ExecutionLog), &record.ExecutionLog)
	json.Unmarshal([]byte(db.Decisions), &record.Decisions)
	json.Unmarshal([]byte(db.AccountStateJSON), &record.AccountState)
	json.Unmarshal([]byte(db.PositionsJSON), &record.Positions)
	return record
}

// LogDecision logs decision
func (s *DecisionStore) LogDecision(record *DecisionRecord) error {
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	} else {
		record.Timestamp = record.Timestamp.UTC()
	}

	// Serialize arrays to JSON
	candidateCoinsJSON, _ := json.Marshal(record.CandidateCoins)
	candidateDetailsJSON, _ := json.Marshal(record.CandidateDetails)
	executionLogJSON, _ := json.Marshal(record.ExecutionLog)
	decisionsJSON, _ := json.Marshal(record.Decisions)
	accountStateJSON, _ := json.Marshal(record.AccountState)
	positionsJSON, _ := json.Marshal(record.Positions)
	normalizedCoT := sanitizeDecisionCoTTrace(record.CoTTrace)
	record.CoTTrace = normalizedCoT

	dbRecord := &DecisionRecordDB{
		TraderID:            record.TraderID,
		CycleNumber:         record.CycleNumber,
		Timestamp:           record.Timestamp,
		SystemPrompt:        record.SystemPrompt,
		InputPrompt:         record.InputPrompt,
		CoTTrace:            normalizedCoT,
		DecisionJSON:        record.DecisionJSON,
		RawResponse:         record.RawResponse,
		AccountStateJSON:    string(accountStateJSON),
		PositionsJSON:       string(positionsJSON),
		CandidateCoins:      string(candidateCoinsJSON),
		CandidateDetails:    string(candidateDetailsJSON),
		CandidateMetaVer:    record.CandidateMetaVer,
		ExecutionLog:        string(executionLogJSON),
		Decisions:           string(decisionsJSON),
		Success:             record.Success,
		ErrorMessage:        record.ErrorMessage,
		AIRequestDurationMs: record.AIRequestDurationMs,
	}

	if err := s.db.Create(dbRecord).Error; err != nil {
		return fmt.Errorf("failed to insert decision record: %w", err)
	}
	record.ID = dbRecord.ID
	return nil
}

// GetLatestRecords gets the latest N records for specified trader (sorted by time in ascending order: old to new)
func (s *DecisionStore) GetLatestRecords(traderID string, n int) ([]*DecisionRecord, error) {
	var dbRecords []*DecisionRecordDB
	err := s.db.Where("trader_id = ?", traderID).
		Order("timestamp DESC").
		Limit(n).
		Find(&dbRecords).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query decision records: %w", err)
	}

	records := make([]*DecisionRecord, len(dbRecords))
	for i, db := range dbRecords {
		records[i] = db.toRecord()
	}

	// Reverse array to sort time from old to new
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetAllLatestRecords gets the latest N records for all traders
func (s *DecisionStore) GetAllLatestRecords(n int) ([]*DecisionRecord, error) {
	var dbRecords []*DecisionRecordDB
	err := s.db.Order("timestamp DESC").Limit(n).Find(&dbRecords).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query decision records: %w", err)
	}

	records := make([]*DecisionRecord, len(dbRecords))
	for i, db := range dbRecords {
		records[i] = db.toRecord()
	}

	// Reverse array
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetRecordsByDate gets all records for a specified trader on a specified date
func (s *DecisionStore) GetRecordsByDate(traderID string, date time.Time) ([]*DecisionRecord, error) {
	dateStr := date.Format("2006-01-02")

	var dbRecords []*DecisionRecordDB
	err := s.db.Where("trader_id = ? AND DATE(timestamp) = ?", traderID, dateStr).
		Order("timestamp ASC").
		Find(&dbRecords).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query decision records: %w", err)
	}

	records := make([]*DecisionRecord, len(dbRecords))
	for i, db := range dbRecords {
		records[i] = db.toRecord()
	}

	return records, nil
}

// GetByCycleNumber returns one decision record for a trader/cycle pair.
func (s *DecisionStore) GetByCycleNumber(traderID string, cycleNumber int) (*DecisionRecord, error) {
	var dbRecord DecisionRecordDB
	err := s.db.Where("trader_id = ? AND cycle_number = ?", traderID, cycleNumber).
		Order("timestamp DESC").
		First(&dbRecord).Error
	if err != nil {
		return nil, err
	}
	return dbRecord.toRecord(), nil
}

// CleanOldRecords cleans old records from N days ago
func (s *DecisionStore) CleanOldRecords(traderID string, days int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -days)

	result := s.db.Where("trader_id = ? AND timestamp < ?", traderID, cutoffTime).
		Delete(&DecisionRecordDB{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to clean old records: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// GetStatistics gets statistics information for specified trader
func (s *DecisionStore) GetStatistics(traderID string) (*Statistics, error) {
	stats := &Statistics{}

	var totalCount, successCount int64
	s.db.Model(&DecisionRecordDB{}).Where("trader_id = ?", traderID).Count(&totalCount)
	s.db.Model(&DecisionRecordDB{}).Where("trader_id = ? AND success = ?", traderID, true).Count(&successCount)

	stats.TotalCycles = int(totalCount)
	stats.SuccessfulCycles = int(successCount)
	stats.FailedCycles = stats.TotalCycles - stats.SuccessfulCycles

	// Count from trader_positions table using raw query for cross-table
	s.db.Raw("SELECT COUNT(*) FROM trader_positions WHERE trader_id = ?", traderID).Scan(&stats.TotalOpenPositions)
	s.db.Raw("SELECT COUNT(*) FROM trader_positions WHERE trader_id = ? AND status = 'CLOSED'", traderID).Scan(&stats.TotalClosePositions)

	return stats, nil
}

// GetAllStatistics gets statistics information for all traders
func (s *DecisionStore) GetAllStatistics() (*Statistics, error) {
	stats := &Statistics{}

	var totalCount, successCount int64
	s.db.Model(&DecisionRecordDB{}).Count(&totalCount)
	s.db.Model(&DecisionRecordDB{}).Where("success = ?", true).Count(&successCount)

	stats.TotalCycles = int(totalCount)
	stats.SuccessfulCycles = int(successCount)
	stats.FailedCycles = stats.TotalCycles - stats.SuccessfulCycles

	// Count from trader_positions table
	s.db.Raw("SELECT COUNT(*) FROM trader_positions").Scan(&stats.TotalOpenPositions)
	s.db.Raw("SELECT COUNT(*) FROM trader_positions WHERE status = 'CLOSED'").Scan(&stats.TotalClosePositions)

	return stats, nil
}

// GetLastCycleNumber gets the last cycle number for specified trader
func (s *DecisionStore) GetLastCycleNumber(traderID string) (int, error) {
	var cycleNumber *int
	err := s.db.Model(&DecisionRecordDB{}).
		Where("trader_id = ?", traderID).
		Select("MAX(cycle_number)").
		Scan(&cycleNumber).Error
	if err != nil {
		return 0, err
	}
	if cycleNumber == nil {
		return 0, nil
	}
	return *cycleNumber, nil
}

type traderBucketAccumulator struct {
	CandidateCount    int
	OpenDecisionCount int
	UniqueSymbols     map[string]struct{}
}

type traderConfidenceAccumulator struct {
	DecisionCount     int
	OpenDecisionCount int
	HoldDecisionCount int
	WaitDecisionCount int
	ConfidenceTotal   int
	ConfidenceSamples int
}

type traderSessionAccumulator struct {
	CycleCount        int
	CandidateCount    int
	OpenDecisionCount int
	HoldDecisionCount int
	WaitDecisionCount int
	ConfidenceTotal   int
	ConfidenceSamples int
}

type traderSymbolAccumulator struct {
	CandidateCount    int
	OpenDecisionCount int
	HoldDecisionCount int
	WaitDecisionCount int
	ConfidenceTotal   int
	ConfidenceSamples int
	Sessions          map[string]struct{}
	SelectionBuckets  map[string]struct{}
}

var traderBucketOrder = []string{
	"primary",
	"adaptive",
	"fallback_eligible",
	"exploration",
	"unclassified",
}

// GetBucketReview aggregates selection-bucket telemetry from recent decision records.
func (s *DecisionStore) GetBucketReview(traderID string, window time.Duration, cycleLimit int) (*TraderBucketReview, error) {
	if window <= 0 {
		window = 24 * time.Hour
	}
	if cycleLimit <= 0 {
		cycleLimit = 20
	}
	if cycleLimit > 100 {
		cycleLimit = 100
	}

	now := time.Now().UTC()
	cutoff := now.Add(-window)

	var dbRecords []*DecisionRecordDB
	if err := s.db.Where("trader_id = ? AND timestamp >= ?", traderID, cutoff).
		Order("timestamp ASC").
		Find(&dbRecords).Error; err != nil {
		return nil, fmt.Errorf("failed to query bucket review records: %w", err)
	}

	report := &TraderBucketReview{
		TraderID:         traderID,
		WindowHours:      int(window.Hours()),
		WindowStart:      cutoff,
		WindowEnd:        now,
		GeneratedAt:      now,
		CandidateMetaVer: DecisionCandidateMetadataVersion,
		Buckets:          []TraderBucketSummary{},
		RecentCycles:     []TraderBucketCycle{},
	}

	accumulators := make(map[string]*traderBucketAccumulator)
	rejectReasons := make(map[string]int)
	confidenceBands := make(map[string]*traderConfidenceAccumulator)
	sessionAccumulators := make(map[string]*traderSessionAccumulator)
	symbolAccumulators := make(map[string]*traderSymbolAccumulator)
	recentCycles := make([]TraderBucketCycle, 0, len(dbRecords))
	var firstRecordAt *time.Time
	var lastRecordAt *time.Time
	totalDecisionConfidence := 0
	totalDecisionConfidenceSamples := 0

	for _, dbRecord := range dbRecords {
		record := dbRecord.toRecord()
		if record.CandidateMetaVer < DecisionCandidateMetadataVersion {
			report.LegacyRecordCount++
			continue
		}

		report.RecordCount++

		ts := record.Timestamp.UTC()
		sessionBucket := deriveDealReviewSessionBucket(ts)
		sessionAcc := ensureTraderSessionAccumulator(sessionAccumulators, sessionBucket)
		sessionAcc.CycleCount++
		if firstRecordAt == nil {
			firstRecordAt = &ts
		}
		lastTs := ts
		lastRecordAt = &lastTs

		if len(record.CandidateDetails) > 0 {
			report.CyclesWithCandidates++
		} else {
			report.CyclesWithoutCandidates++
		}

		report.TotalCandidates += len(record.CandidateDetails)

		symbolToBucket := make(map[string]string, len(record.CandidateDetails))
		cycleBucketSymbols := make(map[string][]string)

		for _, detail := range record.CandidateDetails {
			symbol := strings.ToUpper(strings.TrimSpace(detail.Symbol))
			if symbol == "" {
				continue
			}
			bucket := normalizeTraderBucket(detail.SelectionBucket)
			symbolToBucket[symbol] = bucket

			acc := ensureTraderBucketAccumulator(accumulators, bucket)
			acc.CandidateCount++
			acc.UniqueSymbols[symbol] = struct{}{}
			cycleBucketSymbols[bucket] = appendUniqueSortedSymbol(cycleBucketSymbols[bucket], symbol)

			symbolAcc := ensureTraderSymbolAccumulator(symbolAccumulators, symbol)
			symbolAcc.CandidateCount++
			symbolAcc.Sessions[sessionBucket] = struct{}{}
			symbolAcc.SelectionBuckets[bucket] = struct{}{}
		}
		sessionAcc.CandidateCount += len(record.CandidateDetails)

		openDecisionSymbols := []string{}
		hasOpenDecision := false
		for _, action := range record.Decisions {
			actionType := strings.ToLower(strings.TrimSpace(action.Action))
			symbol := strings.ToUpper(strings.TrimSpace(action.Symbol))
			isOpenDecision := strings.HasPrefix(actionType, "open_")
			isHoldDecision := actionType == "hold"
			isWaitDecision := actionType == "wait"
			if !isOpenDecision && !isHoldDecision && !isWaitDecision {
				continue
			}

			if action.Confidence > 0 {
				totalDecisionConfidence += action.Confidence
				totalDecisionConfidenceSamples++
				sessionAcc.ConfidenceTotal += action.Confidence
				sessionAcc.ConfidenceSamples++
				bandAcc := ensureTraderConfidenceAccumulator(confidenceBands, traderConfidenceBand(action.Confidence))
				bandAcc.DecisionCount++
				bandAcc.ConfidenceTotal += action.Confidence
				bandAcc.ConfidenceSamples++
				if isOpenDecision {
					bandAcc.OpenDecisionCount++
				} else if isHoldDecision {
					bandAcc.HoldDecisionCount++
				} else {
					bandAcc.WaitDecisionCount++
				}
				if symbol != "" {
					symbolAcc := ensureTraderSymbolAccumulator(symbolAccumulators, symbol)
					symbolAcc.ConfidenceTotal += action.Confidence
					symbolAcc.ConfidenceSamples++
				}
			}

			if isOpenDecision {
				if symbol == "" {
					continue
				}
				hasOpenDecision = true
				report.TotalOpenDecisions++
				sessionAcc.OpenDecisionCount++
				openDecisionSymbols = appendUniqueSortedSymbol(openDecisionSymbols, symbol)

				bucket := normalizeTraderBucket(symbolToBucket[symbol])
				acc := ensureTraderBucketAccumulator(accumulators, bucket)
				acc.OpenDecisionCount++

				symbolAcc := ensureTraderSymbolAccumulator(symbolAccumulators, symbol)
				symbolAcc.OpenDecisionCount++
				symbolAcc.Sessions[sessionBucket] = struct{}{}
				symbolAcc.SelectionBuckets[bucket] = struct{}{}
				continue
			}

			rejectReasons[normalizeTraderRejectReason(action.Reasoning)]++
			if isHoldDecision {
				report.HoldDecisionCount++
				sessionAcc.HoldDecisionCount++
				if symbol != "" {
					symbolAcc := ensureTraderSymbolAccumulator(symbolAccumulators, symbol)
					symbolAcc.HoldDecisionCount++
					symbolAcc.Sessions[sessionBucket] = struct{}{}
				}
			} else {
				report.WaitDecisionCount++
				sessionAcc.WaitDecisionCount++
				if symbol != "" {
					symbolAcc := ensureTraderSymbolAccumulator(symbolAccumulators, symbol)
					symbolAcc.WaitDecisionCount++
					symbolAcc.Sessions[sessionBucket] = struct{}{}
				}
			}
		}
		if hasOpenDecision {
			report.CyclesWithOpenDecisions++
		}

		recentCycles = append(recentCycles, TraderBucketCycle{
			CycleNumber:         record.CycleNumber,
			Timestamp:           ts,
			CandidateCount:      len(record.CandidateDetails),
			OpenDecisionSymbols: openDecisionSymbols,
			Buckets:             buildTraderCycleBreaks(cycleBucketSymbols),
		})
	}

	if firstRecordAt != nil {
		report.FirstRecordAt = firstRecordAt
	}
	if lastRecordAt != nil {
		report.LastRecordAt = lastRecordAt
	}
	if firstRecordAt != nil {
		report.CoverageHours = roundFloat(minFloat(now.Sub(*firstRecordAt).Hours(), window.Hours()), 1)
		report.HasFullWindow = report.CoverageHours >= window.Hours()-0.25
	}

	report.Buckets = buildTraderBucketSummaries(accumulators)
	report.RecentCycles = selectRecentTraderCycles(recentCycles, cycleLimit)
	if report.TotalCandidates > 0 {
		report.DecisionConversionRate = roundFloat(float64(report.TotalOpenDecisions)/float64(report.TotalCandidates)*100, 1)
	}
	if totalDecisionConfidenceSamples > 0 {
		report.AvgDecisionConfidence = roundFloat(float64(totalDecisionConfidence)/float64(totalDecisionConfidenceSamples), 1)
	}
	report.RejectReasons = buildTraderRejectReasons(rejectReasons, report.HoldDecisionCount+report.WaitDecisionCount, 6)
	report.ConfidenceBands = buildTraderConfidenceBands(confidenceBands)
	report.OpportunitySessions = buildTraderSessionSummaries(sessionAccumulators, 4)
	report.OpportunitySymbols = buildTraderSymbolSummaries(symbolAccumulators, 6)

	return report, nil
}

func normalizeTraderBucket(bucket string) string {
	switch strings.ToLower(strings.TrimSpace(bucket)) {
	case "primary", "adaptive", "fallback_eligible", "exploration":
		return strings.ToLower(strings.TrimSpace(bucket))
	default:
		return "unclassified"
	}
}

func ensureTraderBucketAccumulator(accumulators map[string]*traderBucketAccumulator, bucket string) *traderBucketAccumulator {
	bucket = normalizeTraderBucket(bucket)
	if accumulators[bucket] == nil {
		accumulators[bucket] = &traderBucketAccumulator{
			UniqueSymbols: make(map[string]struct{}),
		}
	}
	return accumulators[bucket]
}

func ensureTraderConfidenceAccumulator(accumulators map[string]*traderConfidenceAccumulator, band string) *traderConfidenceAccumulator {
	if accumulators[band] == nil {
		accumulators[band] = &traderConfidenceAccumulator{}
	}
	return accumulators[band]
}

func ensureTraderSessionAccumulator(accumulators map[string]*traderSessionAccumulator, session string) *traderSessionAccumulator {
	session = strings.TrimSpace(session)
	if session == "" {
		session = "other"
	}
	if accumulators[session] == nil {
		accumulators[session] = &traderSessionAccumulator{}
	}
	return accumulators[session]
}

func ensureTraderSymbolAccumulator(accumulators map[string]*traderSymbolAccumulator, symbol string) *traderSymbolAccumulator {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		symbol = "UNKNOWN"
	}
	if accumulators[symbol] == nil {
		accumulators[symbol] = &traderSymbolAccumulator{
			Sessions:         make(map[string]struct{}),
			SelectionBuckets: make(map[string]struct{}),
		}
	}
	return accumulators[symbol]
}

func appendUniqueSortedSymbol(symbols []string, symbol string) []string {
	for _, existing := range symbols {
		if existing == symbol {
			return symbols
		}
	}
	symbols = append(symbols, symbol)
	sort.Strings(symbols)
	return symbols
}

func buildTraderCycleBreaks(bucketSymbols map[string][]string) []TraderBucketCycleBreak {
	parts := make([]TraderBucketCycleBreak, 0, len(bucketSymbols))
	for _, bucket := range traderBucketOrder {
		symbols := bucketSymbols[bucket]
		if len(symbols) == 0 {
			continue
		}
		parts = append(parts, TraderBucketCycleBreak{
			Name:    bucket,
			Count:   len(symbols),
			Symbols: append([]string(nil), symbols...),
		})
	}
	return parts
}

func buildTraderBucketSummaries(accumulators map[string]*traderBucketAccumulator) []TraderBucketSummary {
	summaries := make([]TraderBucketSummary, 0, len(accumulators))
	for _, bucket := range traderBucketOrder {
		acc := accumulators[bucket]
		if acc == nil {
			continue
		}
		symbols := make([]string, 0, len(acc.UniqueSymbols))
		for symbol := range acc.UniqueSymbols {
			symbols = append(symbols, symbol)
		}
		sort.Strings(symbols)
		if len(symbols) > 6 {
			symbols = symbols[:6]
		}
		summaries = append(summaries, TraderBucketSummary{
			Name:              bucket,
			CandidateCount:    acc.CandidateCount,
			UniqueSymbols:     len(acc.UniqueSymbols),
			OpenDecisionCount: acc.OpenDecisionCount,
			SymbolSamples:     symbols,
		})
	}
	return summaries
}

func buildTraderRejectReasons(counts map[string]int, totalRejects int, limit int) []TraderRejectReason {
	if len(counts) == 0 {
		return []TraderRejectReason{}
	}
	items := make([]TraderRejectReason, 0, len(counts))
	for reason, count := range counts {
		entry := TraderRejectReason{
			Reason: reason,
			Count:  count,
		}
		if totalRejects > 0 {
			entry.SharePct = roundFloat(float64(count)/float64(totalRejects)*100, 1)
		}
		items = append(items, entry)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Reason < items[j].Reason
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func buildTraderConfidenceBands(accumulators map[string]*traderConfidenceAccumulator) []TraderConfidenceBand {
	order := []string{"0-39", "40-59", "60-74", "75-89", "90-100"}
	result := make([]TraderConfidenceBand, 0, len(order))
	for _, band := range order {
		acc := accumulators[band]
		if acc == nil {
			continue
		}
		entry := TraderConfidenceBand{
			Band:              band,
			DecisionCount:     acc.DecisionCount,
			OpenDecisionCount: acc.OpenDecisionCount,
			HoldDecisionCount: acc.HoldDecisionCount,
			WaitDecisionCount: acc.WaitDecisionCount,
		}
		if acc.ConfidenceSamples > 0 {
			entry.AvgConfidence = roundFloat(float64(acc.ConfidenceTotal)/float64(acc.ConfidenceSamples), 1)
		}
		result = append(result, entry)
	}
	return result
}

func buildTraderSessionSummaries(accumulators map[string]*traderSessionAccumulator, limit int) []TraderSessionSummary {
	if len(accumulators) == 0 {
		return []TraderSessionSummary{}
	}
	result := make([]TraderSessionSummary, 0, len(accumulators))
	for session, acc := range accumulators {
		entry := TraderSessionSummary{
			Session:           session,
			CycleCount:        acc.CycleCount,
			CandidateCount:    acc.CandidateCount,
			OpenDecisionCount: acc.OpenDecisionCount,
			HoldDecisionCount: acc.HoldDecisionCount,
			WaitDecisionCount: acc.WaitDecisionCount,
		}
		if acc.ConfidenceSamples > 0 {
			entry.AvgConfidence = roundFloat(float64(acc.ConfidenceTotal)/float64(acc.ConfidenceSamples), 1)
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CandidateCount == result[j].CandidateCount {
			return result[i].Session < result[j].Session
		}
		return result[i].CandidateCount > result[j].CandidateCount
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func buildTraderSymbolSummaries(accumulators map[string]*traderSymbolAccumulator, limit int) []TraderSymbolSummary {
	if len(accumulators) == 0 {
		return []TraderSymbolSummary{}
	}
	result := make([]TraderSymbolSummary, 0, len(accumulators))
	for symbol, acc := range accumulators {
		entry := TraderSymbolSummary{
			Symbol:            symbol,
			CandidateCount:    acc.CandidateCount,
			OpenDecisionCount: acc.OpenDecisionCount,
			HoldDecisionCount: acc.HoldDecisionCount,
			WaitDecisionCount: acc.WaitDecisionCount,
			Sessions:          mapKeysSorted(acc.Sessions),
			SelectionBuckets:  mapKeysSorted(acc.SelectionBuckets),
		}
		if acc.ConfidenceSamples > 0 {
			entry.AvgConfidence = roundFloat(float64(acc.ConfidenceTotal)/float64(acc.ConfidenceSamples), 1)
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CandidateCount == result[j].CandidateCount {
			if result[i].OpenDecisionCount == result[j].OpenDecisionCount {
				return result[i].Symbol < result[j].Symbol
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

func selectRecentTraderCycles(cycles []TraderBucketCycle, limit int) []TraderBucketCycle {
	if len(cycles) == 0 {
		return []TraderBucketCycle{}
	}
	if len(cycles) > limit {
		cycles = cycles[len(cycles)-limit:]
	}
	result := make([]TraderBucketCycle, 0, len(cycles))
	for i := len(cycles) - 1; i >= 0; i-- {
		result = append(result, cycles[i])
	}
	return result
}

func traderConfidenceBand(confidence int) string {
	switch {
	case confidence < 40:
		return "0-39"
	case confidence < 60:
		return "40-59"
	case confidence < 75:
		return "60-74"
	case confidence < 90:
		return "75-89"
	default:
		return "90-100"
	}
}

func normalizeTraderRejectReason(reasoning string) string {
	value := strings.ToLower(strings.TrimSpace(reasoning))
	switch {
	case value == "":
		return "unspecified"
	case strings.Contains(value, "confidence"):
		return "low_confidence"
	case strings.Contains(value, "position") && strings.Contains(value, "limit"):
		return "position_limit"
	case strings.Contains(value, "risk"):
		return "risk_budget"
	case strings.Contains(value, "liquid"), strings.Contains(value, "spread"), strings.Contains(value, "slippage"):
		return "execution_quality"
	case strings.Contains(value, "volatil"):
		return "volatility"
	case strings.Contains(value, "trend"), strings.Contains(value, "regime"):
		return "regime_mismatch"
	case strings.Contains(value, "funding"):
		return "funding_constraint"
	case strings.Contains(value, "oi"), strings.Contains(value, "open interest"):
		return "oi_constraint"
	case strings.Contains(value, "btc"):
		return "btc_relative_strength"
	case strings.Contains(value, "conflict"), strings.Contains(value, "mixed"), strings.Contains(value, "unclear"):
		return "signal_conflict"
	default:
		value = strings.Split(value, "\n")[0]
		for _, splitter := range []string{";", ".", ","} {
			if idx := strings.Index(value, splitter); idx > 0 {
				value = value[:idx]
				break
			}
		}
		value = strings.TrimSpace(value)
		if len(value) > 48 {
			value = value[:48]
		}
		if value == "" {
			return "unspecified"
		}
		return value
	}
}

func mapKeysSorted(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for key := range values {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	sort.Strings(result)
	return result
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func roundFloat(value float64, digits int) float64 {
	pow := 1.0
	for i := 0; i < digits; i++ {
		pow *= 10
	}
	if pow == 0 {
		return value
	}
	return float64(int(value*pow+0.5)) / pow
}

func sanitizeDecisionCoTTrace(input string) string {
	raw := strings.TrimSpace(reDecisionInvisibleRunes.ReplaceAllString(input, ""))
	if raw == "" {
		return ""
	}

	lower := strings.ToLower(raw)
	if lower == "```" || lower == "```json" || lower == "```jsonc" || lower == "json" || lower == "jsonc" {
		return ""
	}
	if isFenceLinesOnly(raw) {
		return ""
	}

	if match := reDecisionJSONFence.FindStringSubmatch(raw); match != nil && len(match) > 1 {
		inner := strings.TrimSpace(match[1])
		if inner == "" {
			return ""
		}
		if json.Valid([]byte(inner)) {
			return ""
		}
	}

	if (strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[")) && json.Valid([]byte(raw)) {
		return ""
	}

	return raw
}

func isFenceLinesOnly(input string) bool {
	lines := strings.Split(strings.TrimSpace(input), "\n")
	if len(lines) == 0 {
		return true
	}

	seenNonEmpty := false
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		seenNonEmpty = true
		lower := strings.ToLower(l)
		if strings.HasPrefix(lower, "```") || lower == "json" || lower == "jsonc" {
			continue
		}
		return false
	}

	return seenNonEmpty
}
