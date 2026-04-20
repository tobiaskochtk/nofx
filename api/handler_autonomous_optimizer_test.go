package api

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"nofx/store"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

func TestShouldSelfPauseAutonomousOptimizerAfterThreeLowEvidenceRuns(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "autonomous-optimizer.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root, err := store.NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("store.NewFromGorm() error = %v", err)
	}
	if err := gdb.AutoMigrate(&store.AutonomousOptimizerRun{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	userID := "user-1"
	traderID := "trader-1"
	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		run := &store.AutonomousOptimizerRun{
			UserID:           userID,
			TraderID:         traderID,
			ConfigID:         "cfg-1",
			Trigger:          store.AutonomousOptimizerRunTriggerSchedule,
			Status:           store.AutonomousOptimizerStatusInsufficientEvidence,
			PrimaryModelName: "gpt-5.4",
			CriticModelName:  "gpt-5.4",
			StartedAt:        now.Add(time.Duration(i) * time.Minute),
			CompletedAt:      now.Add(time.Duration(i) * time.Minute),
		}
		if err := root.AutonomousOptimizer().SaveRun(run); err != nil {
			t.Fatalf("SaveRun(%d) error = %v", i, err)
		}
		if i == 2 && run.ID == "" {
			t.Fatalf("SaveRun(%d) did not assign ID", i)
		}
	}

	runs, err := root.AutonomousOptimizer().ListRuns(userID, traderID, 3)
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 3 || runs[0] == nil {
		t.Fatalf("ListRuns() = %#v, want 3 runs", runs)
	}

	if !shouldSelfPauseAutonomousOptimizer(store.AutonomousOptimizerStatusInsufficientEvidence, userID, traderID, runs[0].ID, root) {
		t.Fatalf("shouldSelfPauseAutonomousOptimizer() = false, want true")
	}
}

func TestRecoverStaleAutonomousOptimizerRun(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "autonomous-optimizer-stale-run.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root, err := store.NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("store.NewFromGorm() error = %v", err)
	}
	if err := gdb.AutoMigrate(&store.AutonomousOptimizerConfig{}, &store.AutonomousOptimizerRun{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	server := &Server{store: root}
	now := time.Now().UTC()
	cfg := &store.AutonomousOptimizerConfig{
		ID:                  "cfg-1",
		UserID:              "user-1",
		TraderID:            "trader-1",
		Enabled:             true,
		Status:              store.AutonomousOptimizerStatusRunning,
		ReviewIntervalHours: 12,
		PrimaryModelName:    "gpt-5.4",
		CriticModelName:     "gpt-5.4",
	}
	if err := root.AutonomousOptimizer().SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	run := &store.AutonomousOptimizerRun{
		ID:               "run-stale",
		UserID:           cfg.UserID,
		TraderID:         cfg.TraderID,
		ConfigID:         cfg.ID,
		Trigger:          store.AutonomousOptimizerRunTriggerSchedule,
		Status:           store.AutonomousOptimizerStatusRunning,
		PrimaryModelName: "gpt-5.4",
		CriticModelName:  "gpt-5.4",
		StartedAt:        now.Add(-(autonomousOptimizerStaleRunTimeout + 5*time.Minute)),
	}
	if err := root.AutonomousOptimizer().SaveRun(run); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	if err := server.recoverStaleAutonomousOptimizerState(now); err != nil {
		t.Fatalf("recoverStaleAutonomousOptimizerState() error = %v", err)
	}

	recoveredRun, err := root.AutonomousOptimizer().GetRun(cfg.UserID, cfg.TraderID, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if recoveredRun.Status != store.AutonomousOptimizerStatusFailed {
		t.Fatalf("recovered run status = %q, want failed", recoveredRun.Status)
	}
	if recoveredRun.CompletedAt.IsZero() {
		t.Fatal("recovered run completed_at is zero, want recovery timestamp")
	}
	metadata := parseAutonomousOptimizerJSONObject(recoveredRun.MetadataJSON)
	if recovered, _ := metadata["recovered_stale_run"].(bool); !recovered {
		t.Fatalf("recovered_stale_run = %#v, want true", metadata["recovered_stale_run"])
	}

	recoveredCfg, err := root.AutonomousOptimizer().GetConfig(cfg.UserID, cfg.TraderID)
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if recoveredCfg.Status != store.AutonomousOptimizerStatusFailed {
		t.Fatalf("config status = %q, want failed", recoveredCfg.Status)
	}
	if recoveredCfg.LastRunID != run.ID {
		t.Fatalf("LastRunID = %q, want %q", recoveredCfg.LastRunID, run.ID)
	}
	if recoveredCfg.NextRunAt.IsZero() || !recoveredCfg.NextRunAt.After(now) {
		t.Fatalf("NextRunAt = %v, want future retry time", recoveredCfg.NextRunAt)
	}
}

func TestRecoverOrphanAutonomousOptimizerConfig(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "autonomous-optimizer-orphan-config.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root, err := store.NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("store.NewFromGorm() error = %v", err)
	}
	if err := gdb.AutoMigrate(&store.AutonomousOptimizerConfig{}, &store.AutonomousOptimizerRun{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	server := &Server{store: root}
	now := time.Now().UTC()
	cfg := &store.AutonomousOptimizerConfig{
		ID:                  "cfg-orphan",
		UserID:              "user-1",
		TraderID:            "trader-1",
		Enabled:             true,
		Status:              store.AutonomousOptimizerStatusRunning,
		ReviewIntervalHours: 12,
		LastRunAt:           now.Add(-2 * time.Hour),
		PrimaryModelName:    "gpt-5.4",
		CriticModelName:     "gpt-5.4",
	}
	if err := root.AutonomousOptimizer().SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	if err := server.recoverStaleAutonomousOptimizerState(now); err != nil {
		t.Fatalf("recoverStaleAutonomousOptimizerState() error = %v", err)
	}

	recoveredCfg, err := root.AutonomousOptimizer().GetConfig(cfg.UserID, cfg.TraderID)
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if recoveredCfg.Status != store.AutonomousOptimizerStatusFailed {
		t.Fatalf("config status = %q, want failed", recoveredCfg.Status)
	}
	if recoveredCfg.NextRunAt.IsZero() || !recoveredCfg.NextRunAt.After(now) {
		t.Fatalf("NextRunAt = %v, want future retry time", recoveredCfg.NextRunAt)
	}
}

func TestEvaluateAutonomousOptimizerMonitoringReturnsRollbackPending(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "autonomous-optimizer-rollback-pending.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root, err := store.NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("store.NewFromGorm() error = %v", err)
	}
	if err := gdb.AutoMigrate(&store.AutonomousOptimizerConfig{}, &store.AutonomousOptimizerRun{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	server := &Server{store: root}
	now := time.Now().UTC()
	cfg := &store.AutonomousOptimizerConfig{
		ID:                  "cfg-pending",
		UserID:              "user-1",
		TraderID:            "trader-1",
		Enabled:             true,
		Status:              store.AutonomousOptimizerStatusMonitoring,
		ReviewIntervalHours: 12,
		AutoRollbackEnabled: true,
		PrimaryModelName:    "gpt-5.4",
		CriticModelName:     "gpt-5.4",
	}
	if err := root.AutonomousOptimizer().SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	sourceRun := &store.AutonomousOptimizerRun{
		ID:               "run-source",
		UserID:           cfg.UserID,
		TraderID:         cfg.TraderID,
		ConfigID:         cfg.ID,
		Trigger:          store.AutonomousOptimizerRunTriggerSchedule,
		Status:           store.AutonomousOptimizerStatusAutoApplied,
		PrimaryModelName: "gpt-5.4",
		CriticModelName:  "gpt-5.4",
		StartedAt:        now.Add(-2 * time.Hour),
		CompletedAt:      now.Add(-2 * time.Hour),
		MetadataJSON:     `{"monitoring_snapshot":{"closed_deals":2,"winning_deals":2,"losing_deals":0,"net_pnl":1.2,"avg_pnl":0.6,"win_rate":100,"expectancy":0.6,"profit_factor":3.0,"max_drawdown_pct":1.2,"decision_record_count":14,"decision_candidate_count":80,"open_decision_count":4,"hold_decision_count":2,"wait_decision_count":1,"decision_conversion_rate":5.0,"avg_decision_confidence":72,"avg_exit_efficiency_score":62,"avg_profit_given_back_pct":14}}`,
	}
	if err := root.AutonomousOptimizer().SaveRun(sourceRun); err != nil {
		t.Fatalf("SaveRun(sourceRun) error = %v", err)
	}

	bundle := &autonomousOptimizerWindowBundle{
		WindowEnd: now,
		Metadata: autonomousOptimizerRunMetadata{
			ClosedDeals:            2,
			WinningDeals:           0,
			LosingDeals:            2,
			NetPnL:                 -0.45,
			AvgPnL:                 -0.22,
			WinRate:                0,
			Expectancy:             -0.22,
			ProfitFactor:           0.2,
			MaxDrawdownPct:         4.6,
			DecisionRecordCount:    18,
			DecisionCandidateCount: 84,
			OpenDecisionCount:      1,
			HoldDecisionCount:      6,
			WaitDecisionCount:      5,
			DecisionConversionRate: 1.2,
			AvgDecisionConfidence:  49,
			AvgExitEfficiencyScore: 29,
			AvgProfitGivenBackPct:  61,
		},
	}

	result, handled, err := server.evaluateAutonomousOptimizerMonitoring(cfg, sourceRun, nil, nil, nil, bundle, map[string]any{})
	if err != nil {
		t.Fatalf("evaluateAutonomousOptimizerMonitoring() error = %v", err)
	}
	if !handled {
		t.Fatal("evaluateAutonomousOptimizerMonitoring() handled = false, want true")
	}
	if result == nil {
		t.Fatal("evaluateAutonomousOptimizerMonitoring() result = nil")
	}
	if result.RunStatus != store.AutonomousOptimizerStatusRollbackPending {
		t.Fatalf("RunStatus = %q, want rollback_pending", result.RunStatus)
	}
	if result.ConfigStatus != store.AutonomousOptimizerStatusRollbackPending {
		t.Fatalf("ConfigStatus = %q, want rollback_pending", result.ConfigStatus)
	}
	if result.NextRunAt.IsZero() || !result.NextRunAt.After(now) {
		t.Fatalf("NextRunAt = %v, want follow-up rollback time", result.NextRunAt)
	}
	validationRollbackPending, _ := result.Validation["rollback_pending"].(bool)
	if !validationRollbackPending {
		t.Fatalf("rollback_pending validation = %#v, want true", result.Validation["rollback_pending"])
	}
	if verdict := autonomousOptimizerString(result.Metadata["monitoring_verdict"]); verdict != "rollback_pending" {
		t.Fatalf("monitoring_verdict = %q, want rollback_pending", verdict)
	}
}

func TestAutonomousOptimizerHasProposalEvidenceForLowTradeWindow(t *testing.T) {
	bundle := &autonomousOptimizerWindowBundle{
		Metadata: autonomousOptimizerRunMetadata{
			ClosedDeals:            0,
			DecisionRecordCount:    18,
			DecisionCandidateCount: 160,
		},
	}
	if !autonomousOptimizerHasProposalEvidence(bundle) {
		t.Fatalf("autonomousOptimizerHasProposalEvidence() = false, want true for strong low-trade telemetry")
	}
}

func TestValidateAutonomousOptimizerPromptPatchBlocksOverrideBaseEnable(t *testing.T) {
	cfg := &store.AutonomousOptimizerConfig{}
	traderCfg := &store.Trader{
		ID:                   "trader-1",
		AIModelID:            "model-1",
		CustomPrompt:         "existing",
		OverrideBasePrompt:   false,
		SystemPromptTemplate: "v4_2026",
	}
	defaultCfg := store.GetDefaultStrategyConfig("en")
	strategyCfg := &defaultCfg
	patch := &autonomousOptimizerPromptPatch{
		Trader: &autonomousOptimizerTraderPromptPatch{
			OverrideBasePrompt: boolPtr(true),
		},
	}
	modelCfg := &store.AIModel{Provider: "openai"}

	_, _, _, validation, err := validateAutonomousOptimizerPromptPatch(cfg, traderCfg, strategyCfg, modelCfg, "gpt-5.4", patch)
	if err != nil {
		t.Fatalf("validateAutonomousOptimizerPromptPatch() error = %v", err)
	}
	if validation == nil || len(validation.BlockingIssues) == 0 {
		t.Fatalf("validateAutonomousOptimizerPromptPatch() blocking issues = %#v, want non-empty", validation)
	}
}

func TestValidateAutonomousOptimizerPromptPatchSupportsBroaderPromptSurfaces(t *testing.T) {
	cfg := &store.AutonomousOptimizerConfig{
		ProposalPromptInstructions: "keep proposals narrow",
		CriticPromptInstructions:   "block overreach",
	}
	traderCfg := &store.Trader{
		ID:                   "trader-1",
		AIModelID:            "model-1",
		CustomPrompt:         "existing",
		OverrideBasePrompt:   false,
		SystemPromptTemplate: "v4_2026",
	}
	defaultCfg := store.GetDefaultStrategyConfig("en")
	strategyCfg := &defaultCfg
	patch := &autonomousOptimizerPromptPatch{
		Strategy: &autonomousOptimizerStrategyPromptPatch{
			PromptSections: &autonomousOptimizerPromptSectionsPatch{
				MarketContext:  strPtr("Use market microstructure and BTC-relative strength before entering."),
				DecisionFormat: strPtr("Keep XML output machine-parseable and only issue enter when edge is real."),
			},
		},
		Optimizer: &autonomousOptimizerOptimizerPromptPatch{
			ProposalInstructions: strPtr("Propose only the smallest viable live patch."),
			CriticInstructions:   strPtr("Reject patches that degrade inactivity telemetry quality."),
		},
	}
	modelCfg := &store.AIModel{Provider: "openai"}

	mergedOptimizer, _, mergedStrategy, validation, err := validateAutonomousOptimizerPromptPatch(cfg, traderCfg, strategyCfg, modelCfg, "gpt-5.4", patch)
	if err != nil {
		t.Fatalf("validateAutonomousOptimizerPromptPatch() error = %v", err)
	}
	if validation == nil || len(validation.BlockingIssues) > 0 {
		t.Fatalf("validation blocking issues = %#v, want none", validation)
	}
	if validation.ChangedFieldCount != 4 {
		t.Fatalf("ChangedFieldCount = %d, want 4", validation.ChangedFieldCount)
	}
	changed := map[string]struct{}{}
	for _, field := range validation.ChangedFields {
		changed[field] = struct{}{}
	}
	for _, field := range []string{
		"strategy.prompt_sections.market_context",
		"strategy.prompt_sections.decision_format",
		"optimizer.proposal_instructions",
		"optimizer.critic_instructions",
	} {
		if _, ok := changed[field]; !ok {
			t.Fatalf("ChangedFields missing %q: %#v", field, validation.ChangedFields)
		}
	}
	if mergedStrategy == nil || mergedStrategy.PromptSections.MarketContext == "" || mergedStrategy.PromptSections.DecisionFormat == "" {
		t.Fatalf("mergedStrategy prompt sections not updated: %#v", mergedStrategy)
	}
	if mergedOptimizer == nil || mergedOptimizer.ProposalPromptInstructions == "" || mergedOptimizer.CriticPromptInstructions == "" {
		t.Fatalf("mergedOptimizer prompt instructions not updated: %#v", mergedOptimizer)
	}
}

func TestValidateAutonomousOptimizerPromptPatchAutoTrimsToTopPriorityFields(t *testing.T) {
	cfg := &store.AutonomousOptimizerConfig{
		ProposalPromptInstructions: "keep proposals narrow",
		CriticPromptInstructions:   "block overreach",
	}
	traderCfg := &store.Trader{
		ID:                   "trader-1",
		AIModelID:            "model-1",
		CustomPrompt:         "existing",
		OverrideBasePrompt:   false,
		SystemPromptTemplate: "v4_2026",
	}
	defaultCfg := store.GetDefaultStrategyConfig("en")
	strategyCfg := &defaultCfg
	patch := &autonomousOptimizerPromptPatch{
		Strategy: &autonomousOptimizerStrategyPromptPatch{
			PromptSections: &autonomousOptimizerPromptSectionsPatch{
				RoleDefinition:   strPtr("New role"),
				TradingFrequency: strPtr("New frequency"),
				EntryStandards:   strPtr("New entry standards"),
				MarketContext:    strPtr("New market context"),
				DecisionProcess:  strPtr("New decision process"),
				DecisionFormat:   strPtr("Keep XML output machine-parseable with <decision> tags."),
			},
		},
		Optimizer: &autonomousOptimizerOptimizerPromptPatch{
			ProposalInstructions: strPtr("Optimizer proposal overlay"),
			CriticInstructions:   strPtr("Optimizer critic overlay"),
		},
	}
	modelCfg := &store.AIModel{Provider: "openai"}

	mergedOptimizer, _, mergedStrategy, validation, err := validateAutonomousOptimizerPromptPatch(cfg, traderCfg, strategyCfg, modelCfg, "gpt-5.4", patch)
	if err != nil {
		t.Fatalf("validateAutonomousOptimizerPromptPatch() error = %v", err)
	}
	if validation == nil {
		t.Fatal("validation = nil, want prompt validation details")
	}
	if len(validation.BlockingIssues) > 0 {
		t.Fatalf("validation blocking issues = %#v, want none", validation.BlockingIssues)
	}
	if !validation.AutoTrimmed {
		t.Fatalf("AutoTrimmed = false, want true")
	}
	if validation.RequestedFieldCount != 8 {
		t.Fatalf("RequestedFieldCount = %d, want 8", validation.RequestedFieldCount)
	}
	if validation.ChangedFieldCount != autonomousOptimizerMaxPromptPatchFields {
		t.Fatalf("ChangedFieldCount = %d, want %d", validation.ChangedFieldCount, autonomousOptimizerMaxPromptPatchFields)
	}
	if len(validation.DeferredFields) != 2 {
		t.Fatalf("DeferredFields = %#v, want 2 deferred prompt fields", validation.DeferredFields)
	}

	applied := map[string]struct{}{}
	for _, field := range validation.ChangedFields {
		applied[field] = struct{}{}
	}
	for _, field := range []string{
		"strategy.prompt_sections.entry_standards",
		"strategy.prompt_sections.market_context",
		"strategy.prompt_sections.decision_process",
		"strategy.prompt_sections.trading_frequency",
		"strategy.prompt_sections.role_definition",
		"strategy.prompt_sections.decision_format",
	} {
		if _, ok := applied[field]; !ok {
			t.Fatalf("ChangedFields missing %q after trim: %#v", field, validation.ChangedFields)
		}
	}
	deferred := map[string]struct{}{}
	for _, field := range validation.DeferredFields {
		deferred[field] = struct{}{}
	}
	for _, field := range []string{
		"optimizer.proposal_instructions",
		"optimizer.critic_instructions",
	} {
		if _, ok := deferred[field]; !ok {
			t.Fatalf("DeferredFields missing %q: %#v", field, validation.DeferredFields)
		}
	}
	if mergedStrategy == nil {
		t.Fatal("mergedStrategy = nil, want trimmed strategy prompt changes")
	}
	if mergedStrategy.PromptSections.EntryStandards != "New entry standards" {
		t.Fatalf("EntryStandards = %q, want trimmed strategy change applied", mergedStrategy.PromptSections.EntryStandards)
	}
	if mergedOptimizer == nil {
		t.Fatal("mergedOptimizer = nil, want optimizer snapshot")
	}
	if mergedOptimizer.ProposalPromptInstructions != cfg.ProposalPromptInstructions {
		t.Fatalf("ProposalPromptInstructions = %q, want original value retained after defer", mergedOptimizer.ProposalPromptInstructions)
	}
	if mergedOptimizer.CriticPromptInstructions != cfg.CriticPromptInstructions {
		t.Fatalf("CriticPromptInstructions = %q, want original value retained after defer", mergedOptimizer.CriticPromptInstructions)
	}
}

func TestValidateAutonomousOptimizerPromptPatchIgnoresBlankPlaceholders(t *testing.T) {
	cfg := &store.AutonomousOptimizerConfig{
		ProposalPromptInstructions: "keep proposals narrow",
		CriticPromptInstructions:   "block overreach",
	}
	traderCfg := &store.Trader{
		ID:                   "trader-1",
		AIModelID:            "model-1",
		CustomPrompt:         "existing trader prompt",
		OverrideBasePrompt:   false,
		SystemPromptTemplate: "v4_2026",
	}
	defaultCfg := store.GetDefaultStrategyConfig("en")
	strategyCfg := &defaultCfg
	patch := &autonomousOptimizerPromptPatch{
		Strategy: &autonomousOptimizerStrategyPromptPatch{
			PromptSections: &autonomousOptimizerPromptSectionsPatch{
				EntryStandards:  strPtr(""),
				DecisionProcess: strPtr(""),
			},
		},
		Trader: &autonomousOptimizerTraderPromptPatch{
			CustomPrompt:         strPtr(""),
			SystemPromptTemplate: strPtr(""),
		},
		Optimizer: &autonomousOptimizerOptimizerPromptPatch{
			ProposalInstructions: strPtr(""),
			CriticInstructions:   strPtr(""),
		},
	}
	modelCfg := &store.AIModel{Provider: "openai"}

	mergedOptimizer, mergedTrader, mergedStrategy, validation, err := validateAutonomousOptimizerPromptPatch(cfg, traderCfg, strategyCfg, modelCfg, "gpt-5.4", patch)
	if err != nil {
		t.Fatalf("validateAutonomousOptimizerPromptPatch() error = %v", err)
	}
	if validation == nil {
		t.Fatal("validation = nil, want prompt validation details")
	}
	if len(validation.ChangedFields) != 0 {
		t.Fatalf("ChangedFields = %#v, want no-op for blank placeholders", validation.ChangedFields)
	}
	if len(validation.BlockingIssues) == 0 {
		t.Fatalf("BlockingIssues = %#v, want empty-patch block after blank placeholders are ignored", validation.BlockingIssues)
	}
	if mergedStrategy.PromptSections.EntryStandards != strategyCfg.PromptSections.EntryStandards {
		t.Fatalf("EntryStandards changed unexpectedly to %q", mergedStrategy.PromptSections.EntryStandards)
	}
	if mergedTrader.CustomPrompt != traderCfg.CustomPrompt {
		t.Fatalf("CustomPrompt changed unexpectedly to %q", mergedTrader.CustomPrompt)
	}
	if mergedOptimizer.ProposalPromptInstructions != cfg.ProposalPromptInstructions {
		t.Fatalf("ProposalPromptInstructions changed unexpectedly to %q", mergedOptimizer.ProposalPromptInstructions)
	}
}

func TestBeginAutonomousOptimizerRunUsesManualTrigger(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "autonomous-optimizer-manual-run.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root, err := store.NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("store.NewFromGorm() error = %v", err)
	}
	if err := gdb.AutoMigrate(&store.AutonomousOptimizerConfig{}, &store.AutonomousOptimizerRun{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	server := &Server{store: root}
	now := time.Now().UTC()
	cfg := &store.AutonomousOptimizerConfig{
		ID:                  "cfg-manual",
		UserID:              "user-1",
		TraderID:            "trader-1",
		Enabled:             true,
		Status:              store.AutonomousOptimizerStatusScheduled,
		ReviewIntervalHours: 12,
		PrimaryModelName:    "gpt-5.4",
		CriticModelName:     "gpt-5.4",
	}
	if err := root.AutonomousOptimizer().SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	run, err := server.beginAutonomousOptimizerRun(cfg, now, store.AutonomousOptimizerRunTriggerManual)
	if err != nil {
		t.Fatalf("beginAutonomousOptimizerRun() error = %v", err)
	}
	if run == nil {
		t.Fatal("beginAutonomousOptimizerRun() = nil, want run")
	}
	if run.Trigger != store.AutonomousOptimizerRunTriggerManual {
		t.Fatalf("Trigger = %q, want manual", run.Trigger)
	}

	storedRun, err := root.AutonomousOptimizer().GetRun(cfg.UserID, cfg.TraderID, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if storedRun.Trigger != store.AutonomousOptimizerRunTriggerManual {
		t.Fatalf("stored run trigger = %q, want manual", storedRun.Trigger)
	}
}

func TestBuildAutonomousOptimizerConversationRequestUsesReplayHistory(t *testing.T) {
	history := []*store.AutonomousOptimizerConversationMessage{
		{Role: store.AutonomousOptimizerConversationRoleSystem, ReplayContent: ""},
		{Role: store.AutonomousOptimizerConversationRoleUser, ReplayContent: "older-user"},
		{Role: store.AutonomousOptimizerConversationRoleAssistant, ReplayContent: "older-assistant"},
		{Role: store.AutonomousOptimizerConversationRoleUser, ReplayContent: "newer-user"},
		{Role: store.AutonomousOptimizerConversationRoleAssistant, ReplayContent: "newer-assistant"},
	}

	req, replayed, err := buildAutonomousOptimizerConversationRequest(history, 2, "system-now", "current-user")
	if err != nil {
		t.Fatalf("buildAutonomousOptimizerConversationRequest() error = %v", err)
	}
	if replayed != 2 {
		t.Fatalf("replayed = %d, want 2", replayed)
	}
	if len(req.Messages) != 4 {
		t.Fatalf("len(Messages) = %d, want 4", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content != "system-now" {
		t.Fatalf("Messages[0] = %#v, want latest system prompt first", req.Messages[0])
	}
	if req.Messages[1].Content != "newer-user" || req.Messages[2].Content != "newer-assistant" {
		t.Fatalf("replayed messages = %#v, want newest replay history only", req.Messages[1:3])
	}
	if req.Messages[3].Role != "user" || req.Messages[3].Content != "current-user" {
		t.Fatalf("Messages[3] = %#v, want current user prompt last", req.Messages[3])
	}
}

func TestBuildAutonomousOptimizerStarvationMetrics(t *testing.T) {
	review := &store.TraderBucketReview{
		RecordCount:             18,
		TotalCandidates:         120,
		TotalOpenDecisions:      5,
		HoldDecisionCount:       9,
		WaitDecisionCount:       4,
		DecisionConversionRate:  4.2,
		AvgDecisionConfidence:   68.5,
		RejectReasons:           []store.TraderRejectReason{{Reason: "low_confidence", Count: 6}},
		ConfidenceBands:         []store.TraderConfidenceBand{{Band: "60-74", DecisionCount: 7}},
		OpportunitySessions:     []store.TraderSessionSummary{{Session: "us", CandidateCount: 44}},
		OpportunitySymbols:      []store.TraderSymbolSummary{{Symbol: "DOGEUSDT", CandidateCount: 12}},
		CyclesWithCandidates:    12,
		CyclesWithOpenDecisions: 4,
	}

	metrics := buildAutonomousOptimizerStarvationMetrics(review)
	if got, _ := metrics["candidate_count"].(int); got != 120 {
		t.Fatalf("candidate_count = %v, want 120", metrics["candidate_count"])
	}
	if got, _ := metrics["hold_decision_count"].(int); got != 9 {
		t.Fatalf("hold_decision_count = %v, want 9", metrics["hold_decision_count"])
	}
	reasons, ok := metrics["reject_reasons"].([]store.TraderRejectReason)
	if !ok || len(reasons) != 1 || reasons[0].Reason != "low_confidence" {
		t.Fatalf("reject_reasons = %#v, want low_confidence entry", metrics["reject_reasons"])
	}
}

func TestBuildAutonomousOptimizerSymbolBehaviorPriorPayload(t *testing.T) {
	base := time.Now().UTC().Add(-3 * time.Hour)
	bundle := &autonomousOptimizerWindowBundle{
		Cases: []store.DealReviewCaseDetail{
			{
				Case: store.DealReviewCase{
					Status:               store.DealReviewCaseStatusClosed,
					Symbol:               "RAVEUSDT",
					Side:                 "LONG",
					OpenSelectionBucket:  "breakout",
					OpenTrendRegime:      "uptrend",
					OpenVolatilityRegime: "high",
					OpenOIRegime:         "rising",
					RealizedPnL:          -1.4,
					ExitTimeMs:           base.UnixMilli(),
				},
			},
		},
		BucketReview: &store.TraderBucketReview{
			RecentOpenExecutions: []store.TraderOpenExecution{
				{
					Symbol:           "SOONUSDT",
					Side:             "LONG",
					TrendRegime:      "uptrend",
					VolatilityRegime: "high",
					OIRegime:         "rising",
				},
				{
					Symbol:           "FLIPUSDT",
					Side:             "LONG",
					TrendRegime:      "uptrend",
					VolatilityRegime: "high",
					OIRegime:         "rising",
				},
			},
			OpportunitySymbols: []store.TraderSymbolSummary{
				{Symbol: "DUSTUSDT", CandidateCount: 4},
			},
		},
	}

	priors := []store.DealReviewSymbolBehaviorPrior{
		{
			Symbol:                "RAVEUSDT",
			Side:                  "LONG",
			Status:                store.DealReviewSymbolBehaviorPriorStatusCandidate,
			BehaviorBias:          store.DealReviewSymbolBehaviorBiasNegative,
			ValidationLabel:       store.DealReviewSymbolBehaviorValidationLabelConfirmed,
			ValidationAlert:       "Still confirmed.",
			RecommendedAction:     "penalize_setup",
			OpenSelectionBucket:   "breakout",
			OpenTrendRegime:       "uptrend",
			OpenVolatilityRegime:  "high",
			OpenOIRegime:          "rising",
			RegimeSignature:       "bucket=breakout | trend=uptrend | volatility=high | oi=rising",
			SampleCount:           9,
			WinRate:               0.22,
			LossRate:              0.78,
			AvgPnLPct:             -1.4,
			CompositeScore:        0.74,
			ContradictionScore:    0.79,
			DecisionOpenCount:     4,
			DecisionCycleCount:    3,
			AvgDecisionConfidence: 76,
			SignalTags:            []string{"oi_up", "ema_align"},
			Evidence: []store.DealReviewSymbolBehaviorPriorEvidence{
				{CaseID: "case-rave-1"},
				{CaseID: "case-rave-2"},
			},
			Summary: "RAVEUSDT long underperformed.",
		},
		{
			Symbol:                "SOONUSDT",
			Side:                  "LONG",
			Status:                store.DealReviewSymbolBehaviorPriorStatusCandidate,
			BehaviorBias:          store.DealReviewSymbolBehaviorBiasNegative,
			ValidationLabel:       store.DealReviewSymbolBehaviorValidationLabelFalsePositive,
			ValidationAlert:       "Old edge looks wrong now.",
			RecommendedAction:     "penalize_setup",
			OpenSelectionBucket:   "breakout",
			OpenTrendRegime:       "uptrend",
			OpenVolatilityRegime:  "high",
			OpenOIRegime:          "rising",
			RegimeSignature:       "bucket=breakout | trend=uptrend | volatility=high | oi=rising",
			SampleCount:           6,
			WinRate:               0.35,
			LossRate:              0.65,
			AvgPnLPct:             -0.6,
			CompositeScore:        0.58,
			ContradictionScore:    0.52,
			DecisionOpenCount:     3,
			DecisionCycleCount:    2,
			AvgDecisionConfidence: 71,
			FalsePositiveScore:    0.84,
			SignalTags:            []string{"fresh", "breakout"},
			Summary:               "SOONUSDT long is noisy.",
		},
		{
			Symbol:               "DUSTUSDT",
			Side:                 "SHORT",
			Status:               store.DealReviewSymbolBehaviorPriorStatusCandidate,
			BehaviorBias:         store.DealReviewSymbolBehaviorBiasNegative,
			RecommendedAction:    "observe",
			OpenSelectionBucket:  "mean_revert",
			OpenTrendRegime:      "sideways",
			OpenVolatilityRegime: "medium",
			OpenOIRegime:         "flat",
			RegimeSignature:      "bucket=mean_revert | trend=sideways | volatility=medium | oi=flat",
			SampleCount:          5,
			WinRate:              0.4,
			LossRate:             0.6,
			AvgPnLPct:            -0.2,
			CompositeScore:       0.42,
			ContradictionScore:   0.22,
			DecisionOpenCount:    0,
			DecisionCycleCount:   0,
			Summary:              "DUSTUSDT short lacks detail.",
		},
		{
			Symbol:                 "FLIPUSDT",
			Side:                   "LONG",
			Status:                 store.DealReviewSymbolBehaviorPriorStatusExpired,
			BehaviorBias:           store.DealReviewSymbolBehaviorBiasNegative,
			ValidationLabel:        store.DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk,
			ValidationAlert:        "Recent window points the other way.",
			RecommendedAction:      "observe",
			OpenSelectionBucket:    "breakout",
			OpenTrendRegime:        "uptrend",
			OpenVolatilityRegime:   "high",
			OpenOIRegime:           "rising",
			RegimeSignature:        "bucket=breakout | trend=uptrend | volatility=high | oi=rising",
			SampleCount:            8,
			WinRate:                0.25,
			LossRate:               0.75,
			AvgPnLPct:              -0.9,
			CompositeScore:         0.66,
			ContradictionScore:     0.61,
			DecisionOpenCount:      2,
			DecisionCycleCount:     2,
			AvgDecisionConfidence:  73,
			ValidationSampleCount:  2,
			ValidationSupportCount: 2,
			ValidationSupportScore: 0.82,
			RecentSampleCount:      2,
			RecentSupportCount:     0,
			RecentSupportScore:     0.0,
			RecentAvgPnLPct:        0.95,
			DriftScore:             0.88,
			FalseNegativeScore:     0.79,
			SignalTags:             []string{"breakout", "crowded"},
			Summary:                "FLIPUSDT long recently reversed.",
		},
	}

	payload := buildAutonomousOptimizerSymbolBehaviorPriorPayload(priors, bundle)
	if payload == nil {
		t.Fatal("payload = nil, want summary")
	}
	if payload.AvailableCount != 4 || payload.RelevantCount != 4 {
		t.Fatalf("payload counts = %#v, want 4 available and 4 relevant", payload)
	}
	if payload.ConfigCandidateCount != 1 || payload.PromptOnlyCount != 2 || payload.BacklogCandidateCount != 1 {
		t.Fatalf("implication counts = %#v, want 1 config / 2 prompt / 1 backlog", payload)
	}
	if payload.ConfirmedCount != 1 || payload.FalsePositiveCount != 1 || payload.FalseNegativeRiskCount != 1 {
		t.Fatalf("payload health counts = %#v, want one confirmed / false positive / false negative risk", payload)
	}
	if len(payload.Items) != 4 {
		t.Fatalf("len(payload.Items) = %d, want 4", len(payload.Items))
	}
	if payload.Items[0].Symbol != "FLIPUSDT" || payload.Items[0].ValidationLabel != store.DealReviewSymbolBehaviorValidationLabelFalseNegativeRisk {
		t.Fatalf("payload.Items[0] = %#v, want FLIPUSDT reverse-risk alert first", payload.Items[0])
	}
	if payload.Items[0].ImplicationType != "prompt_only" || payload.Items[0].RecentExecutionMatches != 1 {
		t.Fatalf("payload.Items[0] health implication = %#v, want prompt_only with recent execution relevance", payload.Items[0])
	}
	if payload.Items[1].Symbol != "SOONUSDT" || payload.Items[1].ValidationLabel != store.DealReviewSymbolBehaviorValidationLabelFalsePositive {
		t.Fatalf("payload.Items[1] = %#v, want SOONUSDT false-positive alert second", payload.Items[1])
	}
	if payload.Items[1].ImplicationType != "prompt_only" {
		t.Fatalf("payload.Items[1] implication = %#v, want prompt_only false-positive anti-evidence", payload.Items[1])
	}
	if payload.Items[2].Symbol != "RAVEUSDT" || payload.Items[2].ImplicationType != "config_candidate" {
		t.Fatalf("payload.Items[2] = %#v, want RAVEUSDT config candidate after health alerts", payload.Items[2])
	}
	if payload.Items[2].CurrentWindowMatchType != "closed_case" || payload.Items[2].ClosedCaseMatchCount != 1 {
		t.Fatalf("payload.Items[2] match = %#v, want one closed-case match", payload.Items[2])
	}
	if payload.Items[3].Symbol != "DUSTUSDT" || payload.Items[3].ImplicationType != "missing_data_backlog" {
		t.Fatalf("payload.Items[3] = %#v, want DUSTUSDT backlog last", payload.Items[3])
	}
	if payload.Items[3].CurrentWindowMatchType != "opportunity_symbol" || payload.Items[3].OpportunitySymbolHits != 4 {
		t.Fatalf("payload.Items[3] match = %#v, want opportunity-symbol evidence", payload.Items[3])
	}
	if len(payload.Notes) == 0 {
		t.Fatalf("payload.Notes = %#v, want non-empty notes", payload.Notes)
	}
}

func TestBuildAutonomousOptimizerLearnedPatternPayload(t *testing.T) {
	patterns := []store.DealReviewLearnedPattern{
		{
			ID:                     "pattern-rave",
			ScopeType:              store.DealReviewLearnedPatternScopeSymbol,
			Symbol:                 "RAVEUSDT",
			Side:                   "LONG",
			PatternClass:           store.DealReviewLearnedPatternClassPositiveEdge,
			Status:                 "validated",
			ValidationLabel:        store.DealReviewLearnedPatternValidationLabelConfirmed,
			RecommendedUse:         store.DealReviewLearnedPatternRecommendedUseConfigCand,
			PatternSignature:       "bucket:breakout + trend:uptrend",
			FeatureCount:           2,
			FeatureSet:             []string{"bucket:breakout", "trend:uptrend"},
			SampleCount:            8,
			SupportCount:           6,
			ContradictCount:        2,
			WinRate:                0.75,
			LossRate:               0.25,
			AvgPnLPct:              1.4,
			LiftAvgPnLPct:          0.8,
			Expectancy:             0.52,
			ConfidenceScore:        0.81,
			StabilityScore:         0.74,
			CompositeScore:         0.84,
			TrainingSampleCount:    5,
			ValidationSampleCount:  2,
			ValidationSupportCount: 2,
			ValidationSupportScore: 0.72,
			RecentSampleCount:      3,
			RecentSupportCount:     2,
			RecentSupportScore:     0.68,
			Summary:                "RAVEUSDT breakout longs kept following through.",
			Evidence: []store.DealReviewLearnedPatternEvidence{
				{CaseID: "case-rave-1"},
			},
		},
		{
			ID:                     "pattern-soon",
			ScopeType:              store.DealReviewLearnedPatternScopeSymbol,
			Symbol:                 "SOONUSDT",
			Side:                   "LONG",
			PatternClass:           store.DealReviewLearnedPatternClassNegativeEdge,
			Status:                 "validated",
			ValidationLabel:        store.DealReviewLearnedPatternValidationLabelFalsePositive,
			RecommendedUse:         store.DealReviewLearnedPatternRecommendedUseMonitorOnly,
			PatternSignature:       "bucket:breakout + trend:uptrend + oi:falling",
			FeatureCount:           3,
			FeatureSet:             []string{"bucket:breakout", "oi:falling", "trend:uptrend"},
			SampleCount:            7,
			SupportCount:           5,
			ContradictCount:        2,
			WinRate:                0.29,
			LossRate:               0.71,
			AvgPnLPct:              -1.1,
			LiftAvgPnLPct:          -0.7,
			Expectancy:             -0.44,
			ConfidenceScore:        0.77,
			StabilityScore:         0.69,
			DriftScore:             0.21,
			CompositeScore:         0.79,
			FalsePositiveScore:     0.83,
			TrainingSampleCount:    4,
			ValidationSampleCount:  2,
			ValidationSupportCount: 2,
			ValidationSupportScore: 0.76,
			RecentSampleCount:      3,
			RecentSupportCount:     2,
			RecentSupportScore:     0.72,
			Summary:                "SOONUSDT breakout longs repeatedly failed.",
			ValidationAlert:        "Recent holdout still flags this as a false positive.",
			Evidence: []store.DealReviewLearnedPatternEvidence{
				{CaseID: "case-soon-1"},
			},
		},
		{
			ID:                     "pattern-asia",
			ScopeType:              store.DealReviewLearnedPatternScopeTraderLocal,
			Side:                   "LONG",
			PatternClass:           store.DealReviewLearnedPatternClassNegativeEdge,
			Status:                 "validated",
			ValidationLabel:        store.DealReviewLearnedPatternValidationLabelReverseRisk,
			RecommendedUse:         store.DealReviewLearnedPatternRecommendedUseReviewHint,
			PatternSignature:       "session:asia + trend:sideways",
			FeatureCount:           2,
			FeatureSet:             []string{"session:asia", "trend:sideways"},
			SampleCount:            6,
			SupportCount:           4,
			ContradictCount:        2,
			WinRate:                0.33,
			LossRate:               0.67,
			AvgPnLPct:              -0.6,
			LiftAvgPnLPct:          -0.3,
			Expectancy:             -0.2,
			ConfidenceScore:        0.64,
			StabilityScore:         0.61,
			CompositeScore:         0.71,
			ReverseRiskScore:       0.88,
			TrainingSampleCount:    4,
			ValidationSampleCount:  1,
			ValidationSupportCount: 1,
			ValidationSupportScore: 0.57,
			RecentSampleCount:      2,
			RecentSupportCount:     1,
			RecentSupportScore:     0.51,
			Summary:                "Asia sideways longs keep flipping into losses.",
		},
		{
			ID:                     "pattern-eth-short",
			ScopeType:              store.DealReviewLearnedPatternScopeSymbol,
			Symbol:                 "ETHUSDT",
			Side:                   "SHORT",
			PatternClass:           store.DealReviewLearnedPatternClassPositiveEdge,
			Status:                 "observed",
			ValidationLabel:        store.DealReviewLearnedPatternValidationLabelCandidate,
			RecommendedUse:         store.DealReviewLearnedPatternRecommendedUseReviewHint,
			PatternSignature:       "bucket:mean_revert + trend:downtrend",
			FeatureCount:           2,
			FeatureSet:             []string{"bucket:mean_revert", "trend:downtrend"},
			SampleCount:            4,
			SupportCount:           3,
			ContradictCount:        1,
			WinRate:                0.75,
			LossRate:               0.25,
			AvgPnLPct:              0.8,
			LiftAvgPnLPct:          0.4,
			Expectancy:             0.22,
			ConfidenceScore:        0.55,
			StabilityScore:         0.48,
			CompositeScore:         0.58,
			TrainingSampleCount:    3,
			ValidationSampleCount:  1,
			ValidationSupportCount: 1,
			ValidationSupportScore: 0.50,
			RecentSampleCount:      1,
			RecentSupportCount:     1,
			RecentSupportScore:     0.50,
			Summary:                "ETH mean reversion shorts look promising but still need evidence.",
		},
	}

	bundle := &autonomousOptimizerWindowBundle{
		Cases: []store.DealReviewCaseDetail{
			{
				Case: store.DealReviewCase{
					Status:               store.DealReviewCaseStatusClosed,
					Symbol:               "RAVEUSDT",
					Side:                 "LONG",
					RealizedPnL:          1.12,
					OpenSelectionBucket:  "breakout",
					OpenTrendRegime:      "uptrend",
					OpenVolatilityRegime: "high",
				},
			},
			{
				Case: store.DealReviewCase{
					Status:               store.DealReviewCaseStatusClosed,
					Symbol:               "SOONUSDT",
					Side:                 "LONG",
					RealizedPnL:          -0.92,
					OpenSelectionBucket:  "breakout",
					OpenTrendRegime:      "uptrend",
					OpenVolatilityRegime: "high",
					OpenOIRegime:         "falling",
				},
			},
		},
		BucketReview: &store.TraderBucketReview{
			RecentOpenExecutions: []store.TraderOpenExecution{
				{
					Symbol:           "TAOUSDT",
					Side:             "LONG",
					TrendRegime:      "sideways",
					SessionBucket:    "asia",
					TerminalStatus:   "submitted",
					VolatilityRegime: "medium",
				},
			},
			OpportunitySymbols: []store.TraderSymbolSummary{
				{Symbol: "SOONUSDT", CandidateCount: 4},
				{Symbol: "ETHUSDT", CandidateCount: 2},
			},
		},
	}

	payload := buildAutonomousOptimizerLearnedPatternPayload(patterns, bundle)
	if payload == nil {
		t.Fatal("payload = nil, want summary")
	}
	if payload.AvailableCount != 4 {
		t.Fatalf("payload.AvailableCount = %d, want 4", payload.AvailableCount)
	}
	if payload.PositiveCount != 2 || payload.NegativeCount != 2 {
		t.Fatalf("payload polarity counts = %#v, want 2 positive / 2 negative", payload)
	}
	if payload.ConfigCandidateCount != 1 || payload.MonitorOnlyCount != 1 || payload.ReviewHintCount != 2 {
		t.Fatalf("payload use counts = %#v, want 1 config / 1 monitor / 2 review", payload)
	}
	if payload.FalsePositiveCount != 1 || payload.ReverseRiskCount != 1 || payload.ConfirmedCount != 1 {
		t.Fatalf("payload health counts = %#v, want one false positive / reverse risk / confirmed", payload)
	}
	if payload.RelevantCount != 4 {
		t.Fatalf("payload.RelevantCount = %d, want 4", payload.RelevantCount)
	}
	if len(payload.Items) != 4 {
		t.Fatalf("len(payload.Items) = %d, want 4", len(payload.Items))
	}
	if payload.Items[0].PatternID != "pattern-asia" || payload.Items[0].ImplicationType != "anti_evidence" {
		t.Fatalf("payload.Items[0] = %#v, want reverse-risk anti-evidence first", payload.Items[0])
	}
	if payload.Items[0].CurrentWindowMatchType != "recent_open_execution" || payload.Items[0].RecentExecutionMatchCount != 1 {
		t.Fatalf("payload.Items[0] match = %#v, want one recent execution match", payload.Items[0])
	}
	if payload.Items[1].PatternID != "pattern-soon" || payload.Items[1].ImplicationType != "anti_evidence" {
		t.Fatalf("payload.Items[1] = %#v, want false-positive anti-pattern second", payload.Items[1])
	}
	if payload.Items[1].CurrentWindowMatchType != "closed_case" || payload.Items[1].ClosedCaseMatchCount != 1 {
		t.Fatalf("payload.Items[1] match = %#v, want one closed-case match", payload.Items[1])
	}
	if payload.Items[2].PatternID != "pattern-rave" || payload.Items[2].ImplicationType != "config_candidate" {
		t.Fatalf("payload.Items[2] = %#v, want RAVEUSDT config candidate third", payload.Items[2])
	}
	if payload.Items[2].CurrentWindowNetPnL != 1.12 {
		t.Fatalf("payload.Items[2].CurrentWindowNetPnL = %.2f, want 1.12", payload.Items[2].CurrentWindowNetPnL)
	}
	if payload.Items[3].PatternID != "pattern-eth-short" || payload.Items[3].CurrentWindowMatchType != "opportunity_symbol" {
		t.Fatalf("payload.Items[3] = %#v, want ETH opportunity-symbol background item last", payload.Items[3])
	}
	if len(payload.TopPositivePatterns) == 0 || len(payload.TopNegativePatterns) == 0 || len(payload.TopSymbolOverrides) == 0 {
		t.Fatalf("payload top watchlists = %#v, want non-empty summary watchlists", payload)
	}
	if len(payload.Notes) == 0 {
		t.Fatalf("payload.Notes = %#v, want non-empty notes", payload.Notes)
	}
}

func TestBuildAutonomousOptimizerTrailingStopTelemetry(t *testing.T) {
	base := time.Now().UTC().Add(-2 * time.Hour)
	strategyCfg := store.GetDefaultStrategyConfig("en")
	cases := []store.DealReviewCaseDetail{
		{
			Case: store.DealReviewCase{
				Status:         store.DealReviewCaseStatusClosed,
				Symbol:         "ALPHAUSDT",
				Side:           "LONG",
				EntryPrice:     100,
				PositionID:     1,
				EntryTimeMs:    base.UnixMilli(),
				ExitTimeMs:     base.Add(20 * time.Minute).UnixMilli(),
				CloseReason:    "trailing_stop",
				RealizedPnLPct: -0.8,
			},
		},
		{
			Case: store.DealReviewCase{
				Status:         store.DealReviewCaseStatusClosed,
				Symbol:         "BETAUSDT",
				Side:           "LONG",
				EntryPrice:     200,
				PositionID:     2,
				EntryTimeMs:    base.Add(30 * time.Minute).UnixMilli(),
				ExitTimeMs:     base.Add(70 * time.Minute).UnixMilli(),
				CloseReason:    "trailing_stop",
				RealizedPnLPct: 1.5,
			},
		},
		{
			Case: store.DealReviewCase{
				Status:         store.DealReviewCaseStatusClosed,
				Symbol:         "GAMMAUSDT",
				Side:           "LONG",
				EntryPrice:     100,
				PositionID:     3,
				EntryTimeMs:    base.Add(90 * time.Minute).UnixMilli(),
				ExitTimeMs:     base.Add(100*time.Minute + 20*time.Second).UnixMilli(),
				CloseReason:    "stop_loss",
				RealizedPnLPct: -1.2,
				ExitEvidence: &store.DealReviewExitEvidence{
					PreviousStopPrice:         99.2,
					NewStopPrice:              100.05,
					TrailingMode:              "lock_profit",
					TrailingTriggerProfitPct:  0.5,
					TrailingUpdatedAtMs:       base.Add(100 * time.Minute).UnixMilli(),
					TrailingUnrealizedPnL:     0.45,
					TrailingUnrealizedPnLPct:  0.45,
					TrailingStopProfitPct:     0.1,
					TrailingProtectsBreakeven: true,
				},
			},
		},
	}

	updates := map[int64]store.DealReviewTrailingUpdateRecord{
		1: {
			PositionID:           1,
			EntryPrice:           100,
			PreviousStopPrice:    98.5,
			NewStopPrice:         99.8,
			TimestampMs:          base.Add(5 * time.Minute).UnixMilli(),
			UnrealizedPnL:        0.3,
			UnrealizedPnLPct:     0.3,
			StopProfitPct:        -0.2,
			ProtectsBreakeven:    false,
			TierTriggerProfitPct: 0.5,
			TrailingMode:         "lock_profit",
		},
		2: {
			PositionID:           2,
			EntryPrice:           200,
			PreviousStopPrice:    200.2,
			NewStopPrice:         201,
			TimestampMs:          base.Add(55 * time.Minute).UnixMilli(),
			UnrealizedPnL:        1.1,
			UnrealizedPnLPct:     1.1,
			StopProfitPct:        0.5,
			ProtectsBreakeven:    true,
			TierTriggerProfitPct: 1.0,
			TrailingMode:         "trail_offset",
		},
	}

	telemetry := buildAutonomousOptimizerTrailingStopTelemetry(&strategyCfg, cases, updates)
	if telemetry == nil {
		t.Fatal("telemetry = nil, want trailing-stop summary")
	}
	if telemetry.TrailingExitCount != 2 || telemetry.TrailingProfitExitCount != 1 || telemetry.TrailingLossExitCount != 1 {
		t.Fatalf("telemetry = %#v, want 2 trailing exits split 1 profit / 1 loss", telemetry)
	}
	if telemetry.InitialStopLossCount != 1 {
		t.Fatalf("InitialStopLossCount = %d, want 1", telemetry.InitialStopLossCount)
	}
	if telemetry.EarlyTighteningCount != 2 || telemetry.EarlyTighteningLossCount != 2 {
		t.Fatalf("telemetry = %#v, want two early-tightening losses", telemetry)
	}
	if telemetry.EarlyTighteningBelowEntryCount != 1 || telemetry.EarlyTighteningProtectedCount != 1 {
		t.Fatalf("telemetry = %#v, want one below-entry and one protected early tightening", telemetry)
	}
	if telemetry.OneCycleExitCount != 1 || telemetry.EarlyTighteningOneCycleExitCount != 1 {
		t.Fatalf("telemetry = %#v, want one one-cycle exit and it should be early-tightened", telemetry)
	}
	if telemetry.OneCycleExitThresholdSec != 30 {
		t.Fatalf("OneCycleExitThresholdSec = %d, want 30", telemetry.OneCycleExitThresholdSec)
	}
	if telemetry.FirstUpdateAuditCount != 3 || telemetry.BreakevenProtectedCount != 2 {
		t.Fatalf("telemetry = %#v, want 3 audited first updates with 2 breakeven-protected stops", telemetry)
	}
	if telemetry.AvgMinutesToFirstUpdate != 15 {
		t.Fatalf("AvgMinutesToFirstUpdate = %.1f, want 15.0", telemetry.AvgMinutesToFirstUpdate)
	}
	if telemetry.AvgMinutesFromFirstUpdateToExit != 15 {
		t.Fatalf("AvgMinutesFromFirstUpdateToExit = %.1f, want 15.0", telemetry.AvgMinutesFromFirstUpdateToExit)
	}
	if telemetry.TrailingExitAvgPnLPct != 0.35 {
		t.Fatalf("TrailingExitAvgPnLPct = %.2f, want 0.35", telemetry.TrailingExitAvgPnLPct)
	}
	if telemetry.InitialStopLossAvgPnLPct != -1.2 {
		t.Fatalf("InitialStopLossAvgPnLPct = %.2f, want -1.20", telemetry.InitialStopLossAvgPnLPct)
	}
	if len(telemetry.TierBreakdown) != 2 {
		t.Fatalf("TierBreakdown len = %d, want 2", len(telemetry.TierBreakdown))
	}
	if telemetry.TierBreakdown[0].TierTriggerProfitPct != 0.5 || telemetry.TierBreakdown[0].AuditCount != 2 || telemetry.TierBreakdown[0].EarlyTighteningLossCount != 2 {
		t.Fatalf("TierBreakdown[0] = %#v, want 0.5%% tier with two harmful early tightenings", telemetry.TierBreakdown[0])
	}
	if telemetry.TierBreakdown[0].BelowEntryCount != 1 || telemetry.TierBreakdown[0].BreakevenOrBetterCount != 1 || telemetry.TierBreakdown[0].OneCycleExitCount != 1 {
		t.Fatalf("TierBreakdown[0] = %#v, want split entry protection and one one-cycle exit", telemetry.TierBreakdown[0])
	}
	if len(telemetry.ProfitBandBreakdown) != 2 {
		t.Fatalf("ProfitBandBreakdown len = %d, want 2", len(telemetry.ProfitBandBreakdown))
	}
	if telemetry.ProfitBandBreakdown[0].ProfitBand != "0.25-0.5%" || telemetry.ProfitBandBreakdown[0].AuditCount != 2 || telemetry.ProfitBandBreakdown[0].EarlyTighteningLossCount != 2 {
		t.Fatalf("ProfitBandBreakdown[0] = %#v, want two harmful updates in the 0.25-0.5%% band", telemetry.ProfitBandBreakdown[0])
	}
	if len(telemetry.EntryProtectionBreakdown) != 2 {
		t.Fatalf("EntryProtectionBreakdown len = %d, want 2", len(telemetry.EntryProtectionBreakdown))
	}
	if telemetry.EntryProtectionBreakdown[0].EntryProtectionState != "below_entry" || telemetry.EntryProtectionBreakdown[0].AuditCount != 1 {
		t.Fatalf("EntryProtectionBreakdown[0] = %#v, want one below-entry audit", telemetry.EntryProtectionBreakdown[0])
	}
	if telemetry.EntryProtectionBreakdown[1].EntryProtectionState != "breakeven_or_better" || telemetry.EntryProtectionBreakdown[1].OneCycleExitCount != 1 {
		t.Fatalf("EntryProtectionBreakdown[1] = %#v, want protected state to include the one-cycle exit", telemetry.EntryProtectionBreakdown[1])
	}
	if len(telemetry.SampleUpdates) != 3 {
		t.Fatalf("SampleUpdates len = %d, want 3", len(telemetry.SampleUpdates))
	}
	byPosition := make(map[int64]autonomousOptimizerTrailingStopUpdateAuditItem, len(telemetry.SampleUpdates))
	for _, item := range telemetry.SampleUpdates {
		byPosition[item.PositionID] = item
	}
	if item, ok := byPosition[3]; !ok {
		t.Fatalf("SampleUpdates = %#v, want position 3 exit-evidence audit", telemetry.SampleUpdates)
	} else {
		if item.UpdateSource != "exit_evidence" || !item.ExitWithinOneCycle {
			t.Fatalf("position 3 sample = %#v, want exit_evidence one-cycle audit", item)
		}
		if item.EntryProtectionState != "breakeven_or_better" || item.PreUpdateProfitBand != "0.25-0.5%" {
			t.Fatalf("position 3 sample = %#v, want protected 0.25-0.5%% band", item)
		}
	}
}

func TestBuildAutonomousOptimizerLearnedPatternBacklogProposalsSynthesizesActionableFindings(t *testing.T) {
	payload := &autonomousOptimizerLearnedPatternPayload{
		Items: []autonomousOptimizerLearnedPatternEvidence{
			{
				PatternID:                   "pattern-guard",
				ScopeType:                   store.DealReviewLearnedPatternScopeSymbol,
				Symbol:                      "RAVEUSDT",
				Side:                        "LONG",
				PatternClass:                store.DealReviewLearnedPatternClassNegativeEdge,
				ValidationLabel:             store.DealReviewLearnedPatternValidationLabelReverseRisk,
				ImplicationType:             "anti_evidence",
				CurrentWindowMatchType:      "recent_open_execution",
				RecentExecutionMatchCount:   1,
				OpportunitySymbolMatchCount: 2,
				ReverseRiskScore:            0.91,
				CompositeScore:              0.74,
				FeatureSet:                  []string{"bucket:breakout", "oi:flat"},
				EvidenceCaseIDs:             []string{"case-1"},
				Summary:                     "Breakout longs on RAVEUSDT reversed despite strong prompt confidence.",
			},
			{
				PatternID:              "pattern-drift",
				ScopeType:              store.DealReviewLearnedPatternScopeTraderLocal,
				Side:                   "SHORT",
				PatternClass:           store.DealReviewLearnedPatternClassPositiveEdge,
				ValidationLabel:        store.DealReviewLearnedPatternValidationLabelDrifting,
				ImplicationType:        "monitor_only",
				CurrentWindowMatchType: "closed_case",
				ClosedCaseMatchCount:   1,
				DriftScore:             0.83,
				CompositeScore:         0.61,
				FeatureSet:             []string{"bucket:mean_revert", "session:asia"},
				ValidationAlert:        "Recent windows no longer confirm the older short edge.",
			},
			{
				PatternID:              "pattern-coverage",
				ScopeType:              store.DealReviewLearnedPatternScopeTraderLocal,
				Side:                   "LONG",
				PatternClass:           store.DealReviewLearnedPatternClassPositiveEdge,
				ValidationLabel:        store.DealReviewLearnedPatternValidationLabelCandidate,
				ImplicationType:        "review_hint",
				CurrentWindowMatchType: "opportunity_symbol",
				SampleCount:            6,
				SupportCount:           5,
				CompositeScore:         0.67,
				ConfidenceScore:        0.63,
				ValidationSampleCount:  1,
				RecentSampleCount:      0,
				FeatureSet:             []string{"bucket:momentum", "trend:uptrend"},
				Summary:                "Momentum longs look promising but holdout coverage is still thin.",
			},
			{
				PatternID:                                "pattern-delta",
				ScopeType:                                store.DealReviewLearnedPatternScopeSymbol,
				Symbol:                                   "SPACEUSDT",
				Side:                                     "SHORT",
				PatternClass:                             store.DealReviewLearnedPatternClassNegativeEdge,
				ValidationLabel:                          store.DealReviewLearnedPatternValidationLabelConfirmed,
				RecommendedUse:                           store.DealReviewLearnedPatternRecommendedUseMonitoringRule,
				ImplicationType:                          "monitor_only",
				LiveGuardAttributionDeltaTrendLabel:      store.DealReviewLearnedPatternLiveGuardAttributionTrendNewlyOverblocking,
				LiveGuardAttributionDeltaConfidenceScore: 0.88,
				LiveGuardAttributionDeltaRecentResolvedCount:    3,
				LiveGuardAttributionDeltaPriorResolvedCount:     3,
				LiveGuardAttributionDeltaRecentOverblockingRate: 1,
				LiveGuardAttributionDeltaPriorOverblockingRate:  0.33,
				LiveGuardAttributionDeltaRecentProtectiveRate:   0,
				LiveGuardAttributionDeltaPriorProtectiveRate:    0.67,
				LiveGuardAttributionDeltaOverblockingRateDelta:  0.67,
				LiveGuardAttributionDeltaSummary:                "Recent live-guard follow-up newly looks overblocking.",
				LifecycleRollbackScore:                          0.72,
				FeatureSet:                                      []string{"bucket:fade", "oi:rising"},
				Summary:                                         "The live monitoring rule flipped from protective to overblocking.",
			},
		},
	}

	proposals := buildAutonomousOptimizerLearnedPatternBacklogProposals(payload)
	if len(proposals) != 4 {
		t.Fatalf("len(proposals) = %d, want 4", len(proposals))
	}

	byTitle := map[string]autonomousOptimizerBacklogProposal{}
	for _, proposal := range proposals {
		byTitle[proposal.Title] = proposal
		if len(proposal.Evidence) == 0 {
			t.Fatalf("proposal %q evidence = %#v, want synthesized evidence", proposal.Title, proposal.Evidence)
		}
		if len(proposal.Metadata) == 0 {
			t.Fatalf("proposal %q metadata = %#v, want synthesized metadata", proposal.Title, proposal.Metadata)
		}
	}
	if guard, ok := byTitle["Encode RAVEUSDT LONG learned anti-pattern guard"]; !ok {
		t.Fatalf("guard proposal not found in %#v", proposals)
	} else if guard.Metadata["pattern_id"] != "pattern-guard" {
		t.Fatalf("guard metadata pattern_id = %#v, want pattern-guard", guard.Metadata["pattern_id"])
	}
	if delta, ok := byTitle["Add auto-rollback trigger for SPACEUSDT SHORT monitoring rule"]; !ok {
		t.Fatalf("delta rollback proposal not found in %#v", proposals)
	} else if delta.Metadata["pattern_id"] != "pattern-delta" {
		t.Fatalf("delta metadata pattern_id = %#v, want pattern-delta", delta.Metadata["pattern_id"])
	}
	if drift, ok := byTitle["Add drift expiry monitor for trader local SHORT learned pattern"]; !ok {
		t.Fatalf("drift proposal not found in %#v", proposals)
	} else if drift.Metadata["derived_goal"] == nil {
		t.Fatalf("drift proposal metadata = %#v, want derived_goal", drift.Metadata)
	}
}

func TestSaveAutonomousOptimizerBacklogFindingsMergesLearnedPatternEvidence(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "autonomous-optimizer-backlog-merge.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}

	root, err := store.NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("store.NewFromGorm() error = %v", err)
	}
	if err := gdb.AutoMigrate(&store.AutonomousOptimizerBacklogItem{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	server := &Server{store: root}
	cfg := &store.AutonomousOptimizerConfig{UserID: "user-1", TraderID: "trader-1"}

	first := []autonomousOptimizerBacklogProposal{
		{
			Title:              "Encode symbol anti-pattern guard",
			Category:           "missing_prompt_instruction",
			Description:        "Teach the optimizer to avoid the repeated anti-pattern.",
			ExpectedImpact:     "Reduce repeated exposure.",
			Confidence:         0.72,
			ImplementationCost: 0.30,
			Urgency:            0.65,
			RecurrenceCount:    1,
			Evidence: []map[string]any{
				{"pattern_id": "pattern-a", "source": "learned_pattern"},
			},
			Metadata: map[string]any{
				"source_kind":  "learned_pattern_synthesis",
				"derived_goal": "encode_guard",
			},
		},
	}
	created, err := server.saveAutonomousOptimizerBacklogFindings(cfg, "run-a", first, "first summary")
	if err != nil {
		t.Fatalf("saveAutonomousOptimizerBacklogFindings(first) error = %v", err)
	}
	if created != 1 {
		t.Fatalf("created(first) = %d, want 1", created)
	}

	second := []autonomousOptimizerBacklogProposal{
		{
			Title:              "Encode symbol anti-pattern guard",
			Category:           "missing_prompt_instruction",
			Description:        "Refine the same guard with the newest evidence.",
			ExpectedImpact:     "Reduce repeated exposure.",
			Confidence:         0.84,
			ImplementationCost: 0.32,
			Urgency:            0.78,
			RecurrenceCount:    2,
			Evidence: []map[string]any{
				{"pattern_id": "pattern-b", "source": "learned_pattern"},
			},
			Metadata: map[string]any{
				"source_kind":  "learned_pattern_synthesis",
				"derived_goal": "encode_guard",
				"pattern_id":   "pattern-b",
			},
		},
	}
	created, err = server.saveAutonomousOptimizerBacklogFindings(cfg, "run-b", second, "second summary")
	if err != nil {
		t.Fatalf("saveAutonomousOptimizerBacklogFindings(second) error = %v", err)
	}
	if created != 0 {
		t.Fatalf("created(second) = %d, want 0 merged into existing backlog item", created)
	}

	items, err := root.AutonomousOptimizer().ListBacklog(cfg.UserID, cfg.TraderID, 10)
	if err != nil {
		t.Fatalf("ListBacklog() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1 merged backlog item", len(items))
	}
	item := items[0]
	if item.MergedFindingCount != 2 {
		t.Fatalf("MergedFindingCount = %d, want 2", item.MergedFindingCount)
	}
	if item.RecurrenceCount != 3 {
		t.Fatalf("RecurrenceCount = %d, want 3", item.RecurrenceCount)
	}

	evidence := parseAutonomousOptimizerJSONArray(item.EvidenceJSON)
	if len(evidence) != 2 {
		t.Fatalf("len(evidence) = %d, want 2 merged evidence items", len(evidence))
	}
	metadata := parseAutonomousOptimizerJSONObject(item.MetadataJSON)
	if autonomousOptimizerString(metadata["run_id"]) != "run-b" {
		t.Fatalf("metadata run_id = %#v, want run-b", metadata["run_id"])
	}
	if autonomousOptimizerString(metadata["derived_goal"]) != "encode_guard" {
		t.Fatalf("metadata derived_goal = %#v, want encode_guard", metadata["derived_goal"])
	}
	if autonomousOptimizerString(metadata["pattern_id"]) != "pattern-b" {
		t.Fatalf("metadata pattern_id = %#v, want pattern-b", metadata["pattern_id"])
	}
}

func TestBuildAutonomousOptimizerAdaptiveCooldownTelemetry(t *testing.T) {
	strategyCfg := store.GetDefaultStrategyConfig("en")
	base := time.Now().UTC().Add(-30 * time.Hour)
	cases := []store.DealReviewCaseDetail{
		{
			Case: store.DealReviewCase{
				Status:               store.DealReviewCaseStatusClosed,
				Symbol:               "ENAUSDT",
				EntryTimeMs:          base.UnixMilli(),
				ExitTimeMs:           base.Add(30 * time.Minute).UnixMilli(),
				OpenSessionBucket:    "us",
				OpenTrendRegime:      "uptrend",
				OpenVolatilityRegime: "high_vol",
				OpenOIRegime:         "flat",
				RealizedPnLPct:       -1.1,
			},
		},
		{
			Case: store.DealReviewCase{
				Status:               store.DealReviewCaseStatusClosed,
				Symbol:               "ENAUSDT",
				EntryTimeMs:          base.Add(90 * time.Minute).UnixMilli(),
				ExitTimeMs:           base.Add(120 * time.Minute).UnixMilli(),
				OpenSessionBucket:    "us",
				OpenTrendRegime:      "uptrend",
				OpenVolatilityRegime: "high_vol",
				OpenOIRegime:         "flat",
				RealizedPnLPct:       -0.6,
			},
		},
		{
			Case: store.DealReviewCase{
				Status:               store.DealReviewCaseStatusClosed,
				Symbol:               "EDGEUSDT",
				EntryTimeMs:          base.Add(150 * time.Minute).UnixMilli(),
				ExitTimeMs:           base.Add(180 * time.Minute).UnixMilli(),
				OpenSessionBucket:    "us",
				OpenTrendRegime:      "uptrend",
				OpenVolatilityRegime: "high_vol",
				OpenOIRegime:         "flat",
				RealizedPnLPct:       -0.4,
			},
		},
		{
			Case: store.DealReviewCase{
				Status:               store.DealReviewCaseStatusClosed,
				Symbol:               "ENAUSDT",
				EntryTimeMs:          base.Add(16 * time.Hour).UnixMilli(),
				ExitTimeMs:           base.Add(16*time.Hour + 30*time.Minute).UnixMilli(),
				OpenSessionBucket:    "asia",
				OpenTrendRegime:      "uptrend",
				OpenVolatilityRegime: "high_vol",
				OpenOIRegime:         "flat",
				RealizedPnLPct:       1.4,
			},
		},
	}

	telemetry := buildAutonomousOptimizerAdaptiveCooldownTelemetry(&strategyCfg, cases)
	if telemetry == nil {
		t.Fatal("telemetry = nil, want cooldown summary")
	}
	if !telemetry.Config.Enabled || !telemetry.Config.RequireWeakExecutionRegime {
		t.Fatalf("Config = %#v, want default adaptive guard snapshot", telemetry.Config)
	}
	if telemetry.CooldownCandidateCount != 1 {
		t.Fatalf("CooldownCandidateCount = %d, want 1", telemetry.CooldownCandidateCount)
	}
	if telemetry.RepeatAfterLossCount != 1 || telemetry.SameSessionReentryCount != 1 {
		t.Fatalf("telemetry = %#v, want one same-session repeat-after-loss reentry", telemetry)
	}
	if telemetry.RegimeRepeatLossCount != 2 {
		t.Fatalf("RegimeRepeatLossCount = %d, want 2", telemetry.RegimeRepeatLossCount)
	}
	if telemetry.BlockedSymbolReentryOutcomes.TradeCount != 1 ||
		telemetry.BlockedSymbolReentryOutcomes.LossCount != 1 ||
		telemetry.BlockedSymbolReentryOutcomes.NetPnLPct != -0.6 {
		t.Fatalf("BlockedSymbolReentryOutcomes = %#v, want one blocked same-symbol loss at -0.6%%", telemetry.BlockedSymbolReentryOutcomes)
	}
	if telemetry.PostSymbolCooldownOutcomes.TradeCount != 1 ||
		telemetry.PostSymbolCooldownOutcomes.WinCount != 1 ||
		telemetry.PostSymbolCooldownOutcomes.NetPnLPct != 1.4 {
		t.Fatalf("PostSymbolCooldownOutcomes = %#v, want one post-cooldown same-symbol winner at +1.4%%", telemetry.PostSymbolCooldownOutcomes)
	}
	if telemetry.BlockedRegimeReentryOutcomes.TradeCount != 2 ||
		telemetry.BlockedRegimeReentryOutcomes.LossCount != 2 ||
		telemetry.BlockedRegimeReentryOutcomes.NetPnLPct != -1.0 {
		t.Fatalf("BlockedRegimeReentryOutcomes = %#v, want two blocked regime repeats totaling -1.0%%", telemetry.BlockedRegimeReentryOutcomes)
	}
	if telemetry.PostRegimeCooldownOutcomes.TradeCount != 1 ||
		telemetry.PostRegimeCooldownOutcomes.WinCount != 1 ||
		telemetry.PostRegimeCooldownOutcomes.NetPnLPct != 1.4 {
		t.Fatalf("PostRegimeCooldownOutcomes = %#v, want one post-cooldown regime follow-up winner at +1.4%%", telemetry.PostRegimeCooldownOutcomes)
	}
	if len(telemetry.TopSymbols) == 0 || telemetry.TopSymbols[0].Symbol != "ENAUSDT" {
		t.Fatalf("TopSymbols = %#v, want ENAUSDT first", telemetry.TopSymbols)
	}
	if telemetry.TopSymbols[0].BlockedOutcomes.TradeCount != 1 || telemetry.TopSymbols[0].PostCooldownOutcomes.TradeCount != 1 {
		t.Fatalf("TopSymbols = %#v, want ENAUSDT blocked/post outcome breakdown", telemetry.TopSymbols)
	}
	if len(telemetry.TopRegimes) == 0 || telemetry.TopRegimes[0].RepeatAfterLossCount != 2 {
		t.Fatalf("TopRegimes = %#v, want repeated uptrend/high_vol/flat regime", telemetry.TopRegimes)
	}
	if telemetry.TopRegimes[0].BlockedOutcomes.TradeCount != 2 || telemetry.TopRegimes[0].PostCooldownOutcomes.TradeCount != 1 {
		t.Fatalf("TopRegimes = %#v, want blocked/post regime outcome breakdown", telemetry.TopRegimes)
	}
}

func TestAutonomousOptimizerCooldownEnd(t *testing.T) {
	now := time.Now().UTC()
	cfg := &store.AutonomousOptimizerConfig{
		AutoApplyCooldownHours: 24,
	}
	runs := []*store.AutonomousOptimizerRun{
		{
			ID:          "run-1",
			Status:      store.AutonomousOptimizerStatusAutoApplied,
			CompletedAt: now.Add(-2 * time.Hour),
		},
	}

	cooldownEnd, sourceRunID := autonomousOptimizerCooldownEnd(cfg, runs, now)
	if sourceRunID != "run-1" {
		t.Fatalf("sourceRunID = %q, want run-1", sourceRunID)
	}
	if cooldownEnd.IsZero() {
		t.Fatal("cooldownEnd is zero, want active cooldown")
	}
	if remaining := cooldownEnd.Sub(now).Hours(); remaining < 21.9 || remaining > 22.1 {
		t.Fatalf("remaining cooldown = %.2fh, want about 22h", remaining)
	}
}

func TestCountConsecutiveAutonomousAppliesStopsAfterNonApply(t *testing.T) {
	runs := []*store.AutonomousOptimizerRun{
		{Status: store.AutonomousOptimizerStatusAutoApplied},
		{Status: store.AutonomousOptimizerStatusMonitoring},
		{Status: store.AutonomousOptimizerStatusDeferredForNextWindow},
		{Status: store.AutonomousOptimizerStatusAutoApplied},
	}

	if got := countConsecutiveAutonomousApplies(runs); got != 2 {
		t.Fatalf("countConsecutiveAutonomousApplies() = %d, want 2", got)
	}
}

func TestAssessAutonomousOptimizerRollbackRepeatedNegativeWindows(t *testing.T) {
	source := autonomousOptimizerMonitoringSnapshot{
		ClosedDeals:            3,
		NetPnL:                 1.4,
		AvgPnL:                 0.47,
		WinRate:                66.7,
		AvgExitEfficiencyScore: 61,
		AvgProfitGivenBackPct:  18,
		MaxDrawdownPct:         2.1,
	}
	current := autonomousOptimizerMonitoringSnapshot{
		ClosedDeals:            1,
		NetPnL:                 -0.35,
		AvgPnL:                 -0.35,
		WinRate:                0,
		AvgExitEfficiencyScore: 28,
		AvgProfitGivenBackPct:  61,
		MaxDrawdownPct:         4.7,
	}
	chain := autonomousOptimizerMonitoringChainStats{
		RootRunID:        "run-apply",
		WindowsObserved:  1,
		NegativeWindows:  1,
		TotalClosedDeals: 1,
		TotalNetPnL:      -0.22,
	}

	assessment := assessAutonomousOptimizerRollback(source, current, chain)
	if !assessment.ShouldRollback {
		t.Fatalf("ShouldRollback = false, want true")
	}
	codeSet := make(map[string]struct{}, len(assessment.TriggerCodes))
	for _, code := range assessment.TriggerCodes {
		codeSet[code] = struct{}{}
	}
	if _, ok := codeSet["repeated_losing_windows"]; !ok {
		t.Fatalf("TriggerCodes = %#v, want repeated_losing_windows", assessment.TriggerCodes)
	}
	if assessment.ObservedClosedDeals != 2 {
		t.Fatalf("ObservedClosedDeals = %d, want 2", assessment.ObservedClosedDeals)
	}
}

func TestAssessAutonomousOptimizerRollbackInactivityDrift(t *testing.T) {
	source := autonomousOptimizerMonitoringSnapshot{
		DecisionCandidateCount: 100,
		DecisionRecordCount:    20,
		OpenDecisionCount:      10,
		HoldDecisionCount:      7,
		WaitDecisionCount:      5,
		DecisionConversionRate: 10,
	}
	current := autonomousOptimizerMonitoringSnapshot{
		DecisionCandidateCount: 124,
		DecisionRecordCount:    22,
		OpenDecisionCount:      2,
		HoldDecisionCount:      15,
		WaitDecisionCount:      8,
		DecisionConversionRate: 2.1,
		NetPnL:                 0,
	}

	assessment := assessAutonomousOptimizerRollback(source, current, autonomousOptimizerMonitoringChainStats{RootRunID: "run-apply"})
	if !assessment.ShouldRollback {
		t.Fatalf("ShouldRollback = false, want true")
	}
	codeSet := make(map[string]struct{}, len(assessment.TriggerCodes))
	for _, code := range assessment.TriggerCodes {
		codeSet[code] = struct{}{}
	}
	if _, ok := codeSet["inactivity_drift"]; !ok {
		t.Fatalf("TriggerCodes = %#v, want inactivity_drift", assessment.TriggerCodes)
	}
}

func TestResolveAutonomousOptimizerMonitoringSourceRun(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "autonomous-optimizer-monitoring-source.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(gormsqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	root, err := store.NewFromGorm(gdb)
	if err != nil {
		t.Fatalf("store.NewFromGorm() error = %v", err)
	}
	if err := gdb.AutoMigrate(&store.AutonomousOptimizerRun{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	server := &Server{store: root}
	sourceRun := &store.AutonomousOptimizerRun{
		UserID:           "user-1",
		TraderID:         "trader-1",
		Status:           store.AutonomousOptimizerStatusAutoApplied,
		PrimaryModelName: "gpt-5.4",
		CriticModelName:  "gpt-5.4",
		StartedAt:        time.Now().UTC().Add(-2 * time.Hour),
		CompletedAt:      time.Now().UTC().Add(-2 * time.Hour),
	}
	if err := root.AutonomousOptimizer().SaveRun(sourceRun); err != nil {
		t.Fatalf("SaveRun(sourceRun) error = %v", err)
	}
	metadata := map[string]any{
		"monitoring_root_run_id":   sourceRun.ID,
		"monitoring_source_run_id": sourceRun.ID,
	}
	body, _ := json.Marshal(metadata)
	monitoringRun := &store.AutonomousOptimizerRun{
		UserID:           "user-1",
		TraderID:         "trader-1",
		Status:           store.AutonomousOptimizerStatusMonitoring,
		PrimaryModelName: "gpt-5.4",
		CriticModelName:  "gpt-5.4",
		MetadataJSON:     string(body),
		StartedAt:        time.Now().UTC().Add(-1 * time.Hour),
		CompletedAt:      time.Now().UTC().Add(-1 * time.Hour),
	}
	if err := root.AutonomousOptimizer().SaveRun(monitoringRun); err != nil {
		t.Fatalf("SaveRun(monitoringRun) error = %v", err)
	}

	resolved, err := server.resolveAutonomousOptimizerMonitoringSourceRun("user-1", "trader-1", monitoringRun)
	if err != nil {
		t.Fatalf("resolveAutonomousOptimizerMonitoringSourceRun() error = %v", err)
	}
	if resolved == nil || resolved.ID != sourceRun.ID {
		t.Fatalf("resolved run = %#v, want %s", resolved, sourceRun.ID)
	}
}

func TestMergeAutonomousOptimizerBacklogUpdatePreservesAIFields(t *testing.T) {
	confidence := 82.0
	cost := 26.0
	urgency := 71.0
	recurrence := 5
	existing := &store.AutonomousOptimizerBacklogItem{
		ID:                 "item-1",
		UserID:             "user-1",
		TraderID:           "trader-1",
		RunID:              "run-keep",
		Title:              "Old title",
		Category:           "missing_indicator",
		Description:        "old description",
		ExpectedImpact:     "old impact",
		Status:             store.AutonomousOptimizerBacklogStatusNew,
		AIGenerated:        true,
		UserEdited:         false,
		MergedFindingCount: 3,
		EvidenceJSON:       `[{"kind":"scan"}]`,
		MetadataJSON:       `{"run_id":"run-keep"}`,
	}

	updated := mergeAutonomousOptimizerBacklogUpdate(existing, autonomousOptimizerBacklogRequest{
		ID:                 existing.ID,
		Title:              "Need better OI delta context",
		Category:           "missing_market_data",
		Description:        "updated description",
		ExpectedImpact:     "updated impact",
		Confidence:         &confidence,
		ImplementationCost: &cost,
		Urgency:            &urgency,
		RecurrenceCount:    &recurrence,
		Status:             store.AutonomousOptimizerBacklogStatusPlanned,
	})
	if updated == nil {
		t.Fatal("mergeAutonomousOptimizerBacklogUpdate() = nil")
	}
	if updated.RunID != "run-keep" {
		t.Fatalf("RunID = %q, want run-keep", updated.RunID)
	}
	if updated.MergedFindingCount != 3 {
		t.Fatalf("MergedFindingCount = %d, want 3", updated.MergedFindingCount)
	}
	if updated.MetadataJSON != `{"run_id":"run-keep"}` {
		t.Fatalf("MetadataJSON = %q, want preserved metadata", updated.MetadataJSON)
	}
	if updated.EvidenceJSON != `[{"kind":"scan"}]` {
		t.Fatalf("EvidenceJSON = %q, want preserved evidence", updated.EvidenceJSON)
	}
	if !updated.AIGenerated {
		t.Fatal("AIGenerated = false, want true")
	}
	if !updated.UserEdited {
		t.Fatal("UserEdited = false, want true")
	}
	if updated.Title != "Need better OI delta context" || updated.Category != "missing_market_data" {
		t.Fatalf("updated fields were not applied: %#v", updated)
	}
}

func TestBuildAutonomousOptimizerModelOutcomes(t *testing.T) {
	now := time.Now().UTC()
	runs := []*store.AutonomousOptimizerRun{
		{
			ID:                   "run-apply-a",
			PrimaryModelConfigID: "openai_a",
			PrimaryModelName:     "gpt-5.4",
			CriticModelConfigID:  "openai_a",
			CriticModelName:      "gpt-5.4",
			Status:               store.AutonomousOptimizerStatusAutoApplied,
			CompletedAt:          now.Add(-6 * time.Hour),
		},
		{
			ID:                   "run-kept-a",
			PrimaryModelConfigID: "openai_a",
			PrimaryModelName:     "gpt-5.4",
			CriticModelConfigID:  "openai_a",
			CriticModelName:      "gpt-5.4",
			Status:               store.AutonomousOptimizerStatusKept,
			CompletedAt:          now.Add(-5 * time.Hour),
			MetadataJSON:         `{"monitoring_source_run_id":"run-apply-a","rollback_analysis":{"observed_net_pnl":1.25}}`,
		},
		{
			ID:                   "run-backlog-a",
			PrimaryModelConfigID: "openai_a",
			PrimaryModelName:     "gpt-5.4",
			CriticModelConfigID:  "openai_a",
			CriticModelName:      "gpt-5.4",
			Status:               store.AutonomousOptimizerStatusBacklogOnly,
			CompletedAt:          now.Add(-4 * time.Hour),
		},
		{
			ID:                   "run-apply-b",
			PrimaryModelConfigID: "openai_b",
			PrimaryModelName:     "gpt-5.4-mini",
			CriticModelConfigID:  "openai_b",
			CriticModelName:      "gpt-5.4",
			Status:               store.AutonomousOptimizerStatusAutoApplied,
			CompletedAt:          now.Add(-3 * time.Hour),
		},
		{
			ID:                   "run-rollback-b",
			PrimaryModelConfigID: "openai_b",
			PrimaryModelName:     "gpt-5.4-mini",
			CriticModelConfigID:  "openai_b",
			CriticModelName:      "gpt-5.4",
			Status:               store.AutonomousOptimizerStatusRolledBack,
			CompletedAt:          now.Add(-2 * time.Hour),
			MetadataJSON:         `{"rollback_source_run_id":"run-apply-b","rollback_analysis":{"observed_net_pnl":-0.75}}`,
		},
		{
			ID:                   "run-failed-b",
			PrimaryModelConfigID: "openai_b",
			PrimaryModelName:     "gpt-5.4-mini",
			CriticModelConfigID:  "openai_b",
			CriticModelName:      "gpt-5.4",
			Status:               store.AutonomousOptimizerStatusFailed,
			CompletedAt:          now.Add(-90 * time.Minute),
			MetadataJSON:         `{"recovered_stale_run":true}`,
		},
		{
			ID:                   "run-evidence-b",
			PrimaryModelConfigID: "openai_b",
			PrimaryModelName:     "gpt-5.4-mini",
			CriticModelConfigID:  "openai_b",
			CriticModelName:      "gpt-5.4",
			Status:               store.AutonomousOptimizerStatusInsufficientEvidence,
			CompletedAt:          now.Add(-60 * time.Minute),
		},
	}
	backlog := []*store.AutonomousOptimizerBacklogItem{
		{RunID: "run-backlog-a", Status: store.AutonomousOptimizerBacklogStatusDone},
		{RunID: "run-backlog-a", Status: store.AutonomousOptimizerBacklogStatusPlanned},
		{RunID: "run-apply-b", Status: store.AutonomousOptimizerBacklogStatusRejected},
	}
	models := []*store.AIModel{
		{ID: "openai_a", Name: "OpenAI Primary", CustomModelName: "Test_openai"},
		{ID: "openai_b", Name: "OpenAI Secondary", CustomModelName: "Test_openai_mini"},
	}

	outcomes := buildAutonomousOptimizerModelOutcomes(runs, backlog, models)
	if len(outcomes) != 2 {
		t.Fatalf("len(outcomes) = %d, want 2", len(outcomes))
	}

	first := outcomes[0]
	if first.PrimaryModelConfigID != "openai_a" {
		t.Fatalf("first.PrimaryModelConfigID = %q, want openai_a", first.PrimaryModelConfigID)
	}
	if first.ApplyCount != 1 || first.KeptCount != 1 || first.KeptWinCount != 1 {
		t.Fatalf("first kept/apply stats = %#v, want 1/1/1", first)
	}
	if first.RollbackCount != 0 {
		t.Fatalf("first.RollbackCount = %d, want 0", first.RollbackCount)
	}
	if first.BacklogItemCount != 2 || first.UsefulBacklogCount != 2 || first.DoneBacklogCount != 1 {
		t.Fatalf("first backlog stats = %#v, want useful backlog counts", first)
	}
	if first.KeptWinRate < 99 || first.BacklogUsefulness < 80 {
		t.Fatalf("first rates = %#v, want strong kept win and backlog usefulness", first)
	}

	second := outcomes[1]
	if second.PrimaryModelConfigID != "openai_b" {
		t.Fatalf("second.PrimaryModelConfigID = %q, want openai_b", second.PrimaryModelConfigID)
	}
	if second.ApplyCount != 1 || second.RollbackCount != 1 {
		t.Fatalf("second apply/rollback stats = %#v, want 1 apply and 1 rollback", second)
	}
	if second.RollbackRate < 99 {
		t.Fatalf("second.RollbackRate = %.2f, want ~100", second.RollbackRate)
	}
	if second.FailedCount != 1 || second.StaleRecoveryCount != 1 {
		t.Fatalf("second failure stats = %#v, want one failed stale-recovery run", second)
	}
	if second.InsufficientEvidence != 1 || second.FailureEvidenceGaps != 1 {
		t.Fatalf("second evidence-gap stats = %#v, want one insufficient-evidence overlap", second)
	}
	if second.OperationalHealth >= 100 {
		t.Fatalf("second.OperationalHealth = %.2f, want degraded score", second.OperationalHealth)
	}
	if second.BacklogItemCount != 1 || second.RejectedBacklogCount != 1 {
		t.Fatalf("second backlog stats = %#v, want one rejected item", second)
	}
}

func TestBuildAutonomousOptimizerLatestGateFeedback(t *testing.T) {
	validation, _ := json.Marshal(map[string]any{
		"gate_reasons": []string{
			"Latest proposal repeated the same unsupported config patch.",
			"Need a narrower subset than last run.",
		},
		"config_patch_paths": []string{
			"risk_control.adaptive_reentry_guard.enabled",
			"risk_control.adaptive_reentry_guard.same_symbol_loss_cooldown_minutes",
		},
		"critic": map[string]any{
			"approved":           false,
			"recommended_action": "block_apply",
			"summary":            "The proposal is directionally valid but still too broad.",
			"symbol_prior_references": []string{
				"RAVEUSDT|LONG|breakout|uptrend|high|rising",
			},
			"blocking_issues": []string{
				"Evidence is still mixed across symbols.",
			},
		},
		"prompt_validation": map[string]any{
			"requested_field_count": 7,
			"changed_field_count":   6,
			"deferred_fields": []string{
				"strategy.prompt_sections.market_context",
			},
		},
	})
	metadata, _ := json.Marshal(map[string]any{
		"proposal": map[string]any{
			"proposal_type":   "config_patch",
			"expected_effect": "Reduce repeated same-symbol loss loops.",
			"symbol_prior_references": []string{
				"RAVEUSDT|LONG|breakout|uptrend|high|rising",
			},
			"rationale": []string{
				"Repeated losses cluster on the same symbols after prior exits.",
			},
		},
	})
	runs := []*store.AutonomousOptimizerRun{
		{
			ID:             "run-blocked",
			Status:         store.AutonomousOptimizerStatusBlockedByGate,
			Trigger:        store.AutonomousOptimizerRunTriggerSchedule,
			Summary:        "Blocked run summary",
			ValidationJSON: string(validation),
			MetadataJSON:   string(metadata),
			StartedAt:      time.Now().UTC().Add(-5 * time.Minute),
			CompletedAt:    time.Now().UTC().Add(-4 * time.Minute),
		},
	}

	feedback := buildAutonomousOptimizerLatestGateFeedback(runs)
	if got := autonomousOptimizerString(feedback["run_id"]); got != "run-blocked" {
		t.Fatalf("feedback.run_id = %q, want run-blocked", got)
	}
	if reasons := autonomousOptimizerStringSlice(feedback["gate_reasons"]); len(reasons) != 2 {
		t.Fatalf("feedback.gate_reasons = %#v, want 2 items", reasons)
	}
	critic := parseAutonomousOptimizerNestedObject(feedback, "critic")
	if got := autonomousOptimizerString(critic["recommended_action"]); got != "block_apply" {
		t.Fatalf("feedback.critic.recommended_action = %q, want block_apply", got)
	}
	if refs := autonomousOptimizerStringSlice(critic["symbol_prior_references"]); len(refs) != 1 {
		t.Fatalf("feedback.critic.symbol_prior_references = %#v, want 1 item", refs)
	}
	proposal := parseAutonomousOptimizerNestedObject(feedback, "proposal")
	if got := autonomousOptimizerString(proposal["proposal_type"]); got != "config_patch" {
		t.Fatalf("feedback.proposal.proposal_type = %q, want config_patch", got)
	}
	if refs := autonomousOptimizerStringSlice(proposal["symbol_prior_references"]); len(refs) != 1 {
		t.Fatalf("feedback.proposal.symbol_prior_references = %#v, want 1 item", refs)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func strPtr(value string) *string {
	return &value
}
