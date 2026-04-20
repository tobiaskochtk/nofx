package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	SemanticMemoryDocTypeDealReviewCase         = "deal_review_case"
	SemanticMemoryDocTypeAutonomousOptimizerRun = "autonomous_optimizer_run"
	SemanticMemoryDocTypeOptimizerBacklogItem   = "autonomous_optimizer_backlog_item"
	SemanticMemoryDocTypeStrategyVersion        = "strategy_version"
	SemanticMemoryDocTypeDecisionRecordSummary  = "decision_record_summary"
	SemanticMemoryDocTypeSymbolBehaviorPrior    = "symbol_behavior_prior"
	SemanticMemoryDocTypeLearnedPattern         = "learned_pattern"

	SemanticMemoryEmbeddingStatusPending  = "pending_embedding"
	SemanticMemoryEmbeddingStatusEmbedded = "embedded"
	SemanticMemoryEmbeddingStatusFailed   = "failed"
	SemanticMemoryEmbeddingStatusSkipped  = "skipped"

	SemanticMemorySyncScopeCoreKnowledge = "core_knowledge"
	SemanticMemorySyncScopeEmbedPending  = "embed_pending"

	SemanticMemorySyncStatusRunning   = "running"
	SemanticMemorySyncStatusCompleted = "completed"
	SemanticMemorySyncStatusFailed    = "failed"

	SemanticMemoryDefaultEmbeddingProvider = "openai"
	SemanticMemoryDefaultEmbeddingModel    = "text-embedding-3-small"
	SemanticMemoryDefaultEmbeddingDims     = 1536
)

type SemanticMemoryStore struct {
	db *gorm.DB
}

type SemanticMemoryDocument struct {
	ID                  string    `gorm:"primaryKey" json:"id"`
	UserID              string    `gorm:"column:user_id;not null;index:idx_semantic_memory_documents_scope_source,unique;index:idx_semantic_memory_documents_user_trader" json:"user_id"`
	TraderID            string    `gorm:"column:trader_id;default:'';index:idx_semantic_memory_documents_user_trader" json:"trader_id"`
	DocType             string    `gorm:"column:doc_type;not null;index:idx_semantic_memory_documents_scope_source,unique;index:idx_semantic_memory_documents_doc_type" json:"doc_type"`
	SourceID            string    `gorm:"column:source_id;not null;index:idx_semantic_memory_documents_scope_source,unique" json:"source_id"`
	SourceUpdatedAt     time.Time `gorm:"column:source_updated_at;index:idx_semantic_memory_documents_source_updated" json:"source_updated_at"`
	Title               string    `gorm:"column:title;type:text;default:''" json:"title"`
	Summary             string    `gorm:"column:summary;type:text;default:''" json:"summary"`
	Body                string    `gorm:"column:body;type:text;default:''" json:"body"`
	MetadataJSON        string    `gorm:"column:metadata_json;type:text;default:'{}'" json:"-"`
	ContentHash         string    `gorm:"column:content_hash;default:'';index:idx_semantic_memory_documents_hash" json:"content_hash"`
	TokenEstimate       int       `gorm:"column:token_estimate;default:0" json:"token_estimate"`
	EmbeddingProvider   string    `gorm:"column:embedding_provider;default:''" json:"embedding_provider"`
	EmbeddingModel      string    `gorm:"column:embedding_model;default:''" json:"embedding_model"`
	EmbeddingDimensions int       `gorm:"column:embedding_dimensions;default:0" json:"embedding_dimensions"`
	EmbeddingStatus     string    `gorm:"column:embedding_status;default:'pending_embedding';index:idx_semantic_memory_documents_status" json:"embedding_status"`
	LastEmbeddingError  string    `gorm:"column:last_embedding_error;type:text;default:''" json:"last_embedding_error"`
	LastBuiltAt         time.Time `gorm:"column:last_built_at;index:idx_semantic_memory_documents_last_built" json:"last_built_at"`
	LastEmbeddedAt      time.Time `gorm:"column:last_embedded_at" json:"last_embedded_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (SemanticMemoryDocument) TableName() string { return "semantic_memory_documents" }

type SemanticMemorySyncRun struct {
	ID                 string    `gorm:"primaryKey" json:"id"`
	UserID             string    `gorm:"column:user_id;not null;index:idx_semantic_memory_sync_runs_scope_time" json:"user_id"`
	TraderID           string    `gorm:"column:trader_id;default:'';index:idx_semantic_memory_sync_runs_scope_time" json:"trader_id"`
	Scope              string    `gorm:"column:scope;not null;index:idx_semantic_memory_sync_runs_scope_time" json:"scope"`
	Status             string    `gorm:"column:status;default:'running';index:idx_semantic_memory_sync_runs_status" json:"status"`
	Summary            string    `gorm:"column:summary;type:text;default:''" json:"summary"`
	TotalDocuments     int       `gorm:"column:total_documents;default:0" json:"total_documents"`
	InsertedDocuments  int       `gorm:"column:inserted_documents;default:0" json:"inserted_documents"`
	UpdatedDocuments   int       `gorm:"column:updated_documents;default:0" json:"updated_documents"`
	UnchangedDocuments int       `gorm:"column:unchanged_documents;default:0" json:"unchanged_documents"`
	FailedDocuments    int       `gorm:"column:failed_documents;default:0" json:"failed_documents"`
	ErrorMessage       string    `gorm:"column:error_message;type:text;default:''" json:"error_message"`
	MetadataJSON       string    `gorm:"column:metadata_json;type:text;default:'{}'" json:"-"`
	StartedAt          time.Time `gorm:"column:started_at;index:idx_semantic_memory_sync_runs_started" json:"started_at"`
	CompletedAt        time.Time `gorm:"column:completed_at" json:"completed_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`

	EmbeddingProvider string   `gorm:"-" json:"embedding_provider,omitempty"`
	EmbeddingModel    string   `gorm:"-" json:"embedding_model,omitempty"`
	PromptTokens      int      `gorm:"-" json:"prompt_tokens,omitempty"`
	EstimatedCostUSD  float64  `gorm:"-" json:"estimated_cost_usd,omitempty"`
	DocTypes          []string `gorm:"-" json:"doc_types,omitempty"`
}

func (SemanticMemorySyncRun) TableName() string { return "semantic_memory_sync_runs" }

type SemanticMemoryBackfillResult struct {
	Run       *SemanticMemorySyncRun `json:"run"`
	ByDocType map[string]int         `json:"by_doc_type,omitempty"`
}

type semanticMemoryUpsertOutcome string

const (
	semanticMemoryUpsertInserted  semanticMemoryUpsertOutcome = "inserted"
	semanticMemoryUpsertUpdated   semanticMemoryUpsertOutcome = "updated"
	semanticMemoryUpsertUnchanged semanticMemoryUpsertOutcome = "unchanged"
)

func NewSemanticMemoryStore(db *gorm.DB) *SemanticMemoryStore {
	return &SemanticMemoryStore{db: db}
}

func (s *SemanticMemoryStore) initTables() error {
	if err := s.db.AutoMigrate(
		&SemanticMemoryDocument{},
		&SemanticMemorySyncRun{},
		&SemanticMemorySearchPreset{},
		&SemanticMemoryBenchmarkRun{},
	); err != nil {
		return err
	}
	return s.initVectorTables()
}

func (s *SemanticMemoryStore) initVectorTables() error {
	if s.db == nil || s.db.Dialector.Name() != "postgres" {
		return nil
	}
	if err := s.EnsureVectorExtension(); err != nil {
		return err
	}
	if err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS semantic_memory_vectors (
			document_id TEXT PRIMARY KEY REFERENCES semantic_memory_documents(id) ON DELETE CASCADE,
			embedding vector(1536) NOT NULL,
			embedding_provider TEXT NOT NULL DEFAULT 'openai',
			embedding_model TEXT NOT NULL DEFAULT 'text-embedding-3-small',
			dimensions INTEGER NOT NULL DEFAULT 1536,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		return err
	}
	if err := s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_semantic_memory_vectors_embedding_hnsw
		ON semantic_memory_vectors
		USING hnsw (embedding vector_cosine_ops)
	`).Error; err != nil {
		return err
	}
	return nil
}

func (s *SemanticMemoryStore) VectorExtensionStatus() (bool, string, error) {
	if s.db == nil || s.db.Dialector.Name() != "postgres" {
		return false, "", nil
	}
	row := s.db.Raw(`SELECT extname FROM pg_extension WHERE extname = 'vector'`).Row()
	var extName string
	err := row.Scan(&extName)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no rows") {
			return false, "", nil
		}
		return false, "", err
	}
	return extName == "vector", extName, nil
}

func (s *SemanticMemoryStore) EnsureVectorExtension() error {
	if s.db == nil || s.db.Dialector.Name() != "postgres" {
		return nil
	}
	return s.db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`).Error
}

func (s *SemanticMemoryStore) ListDocuments(userID, traderID, docType string, limit int) ([]*SemanticMemoryDocument, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := s.db.Model(&SemanticMemoryDocument{}).Where("user_id = ?", userID)
	if strings.TrimSpace(traderID) != "" {
		query = query.Where("trader_id = ?", traderID)
	}
	if strings.TrimSpace(docType) != "" {
		query = query.Where("doc_type = ?", docType)
	}
	var items []*SemanticMemoryDocument
	if err := query.Order("updated_at DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *SemanticMemoryStore) ListSyncRuns(userID, traderID string, limit int) ([]*SemanticMemorySyncRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	query := s.db.Model(&SemanticMemorySyncRun{}).Where("user_id = ?", userID)
	if strings.TrimSpace(traderID) != "" {
		query = query.Where("trader_id = ?", traderID)
	}
	var items []*SemanticMemorySyncRun
	if err := query.Order("started_at DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		hydrateSemanticMemorySyncRun(item)
	}
	return items, nil
}

func hydrateSemanticMemorySyncRun(run *SemanticMemorySyncRun) {
	if run == nil {
		return
	}
	meta := semanticMemoryParseJSONObject(run.MetadataJSON)
	run.EmbeddingProvider = semanticMemoryMetadataString(meta, "embedding_provider")
	run.EmbeddingModel = semanticMemoryMetadataString(meta, "embedding_model")
	run.PromptTokens = semanticMemoryMetadataInt(meta, "prompt_tokens")
	run.EstimatedCostUSD = semanticMemoryMetadataFloat(meta, "estimated_cost_usd")
	run.DocTypes = semanticMemoryMetadataStringSlice(meta, "doc_types")
	if run.EstimatedCostUSD <= 0 && run.PromptTokens > 0 && run.EmbeddingModel != "" {
		run.EstimatedCostUSD = semanticMemoryEstimatedCostUSD(run.EmbeddingModel, run.PromptTokens)
	}
}

func semanticMemoryNormalizedRequestedDocTypes(docTypes []string) []string {
	normalized := semanticMemoryNormalizeDocTypes(docTypes)
	if len(normalized) > 0 {
		return normalized
	}
	return []string{
		SemanticMemoryDocTypeDealReviewCase,
		SemanticMemoryDocTypeAutonomousOptimizerRun,
		SemanticMemoryDocTypeOptimizerBacklogItem,
		SemanticMemoryDocTypeStrategyVersion,
		SemanticMemoryDocTypeSymbolBehaviorPrior,
		SemanticMemoryDocTypeLearnedPattern,
	}
}

func semanticMemoryRequestedDocTypeSet(docTypes []string) map[string]struct{} {
	items := semanticMemoryNormalizedRequestedDocTypes(docTypes)
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		result[item] = struct{}{}
	}
	return result
}

func (s *SemanticMemoryStore) BackfillCoreDocuments(userID, traderID string, perTypeLimit int) (*SemanticMemoryBackfillResult, error) {
	return s.BackfillDocuments(userID, traderID, nil, perTypeLimit)
}

func (s *SemanticMemoryStore) BackfillDocuments(userID, traderID string, docTypes []string, perTypeLimit int) (*SemanticMemoryBackfillResult, error) {
	now := time.Now().UTC()
	run := &SemanticMemorySyncRun{
		ID:        uuid.NewString(),
		UserID:    userID,
		TraderID:  traderID,
		Scope:     SemanticMemorySyncScopeCoreKnowledge,
		Status:    SemanticMemorySyncStatusRunning,
		StartedAt: now,
	}
	if err := s.db.Create(run).Error; err != nil {
		return nil, err
	}

	byType := map[string]int{}
	var firstErr error
	requested := semanticMemoryRequestedDocTypeSet(docTypes)

	process := func(docType string, docs []*SemanticMemoryDocument) {
		for _, doc := range docs {
			if doc == nil {
				continue
			}
			run.TotalDocuments++
			byType[docType]++
			outcome, err := s.UpsertDocument(doc)
			if err != nil {
				run.FailedDocuments++
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			switch outcome {
			case semanticMemoryUpsertInserted:
				run.InsertedDocuments++
			case semanticMemoryUpsertUpdated:
				run.UpdatedDocuments++
			default:
				run.UnchangedDocuments++
			}
		}
	}

	if _, ok := requested[SemanticMemoryDocTypeDealReviewCase]; ok {
		caseDocs, err := s.buildDealReviewCaseDocuments(userID, traderID, perTypeLimit)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		process(SemanticMemoryDocTypeDealReviewCase, caseDocs)
	}

	if _, ok := requested[SemanticMemoryDocTypeAutonomousOptimizerRun]; ok {
		runDocs, err := s.buildAutonomousOptimizerRunDocuments(userID, traderID, perTypeLimit)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		process(SemanticMemoryDocTypeAutonomousOptimizerRun, runDocs)
	}

	if _, ok := requested[SemanticMemoryDocTypeOptimizerBacklogItem]; ok {
		backlogDocs, err := s.buildAutonomousOptimizerBacklogDocuments(userID, traderID, perTypeLimit)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		process(SemanticMemoryDocTypeOptimizerBacklogItem, backlogDocs)
	}

	if _, ok := requested[SemanticMemoryDocTypeStrategyVersion]; ok {
		versionDocs, err := s.buildStrategyVersionDocuments(userID, traderID, perTypeLimit)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		process(SemanticMemoryDocTypeStrategyVersion, versionDocs)
	}

	if _, ok := requested[SemanticMemoryDocTypeDecisionRecordSummary]; ok {
		decisionDocs, err := s.buildDecisionRecordSummaryDocuments(userID, traderID, perTypeLimit)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		process(SemanticMemoryDocTypeDecisionRecordSummary, decisionDocs)
	}

	if _, ok := requested[SemanticMemoryDocTypeSymbolBehaviorPrior]; ok {
		priorDocs, err := s.buildSymbolBehaviorPriorDocuments(userID, traderID, perTypeLimit)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		process(SemanticMemoryDocTypeSymbolBehaviorPrior, priorDocs)
	}

	if _, ok := requested[SemanticMemoryDocTypeLearnedPattern]; ok {
		patternDocs, err := s.buildLearnedPatternDocuments(userID, traderID, perTypeLimit)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		process(SemanticMemoryDocTypeLearnedPattern, patternDocs)
	}

	run.CompletedAt = time.Now().UTC()
	run.MetadataJSON = semanticMemoryMarshalJSON(map[string]any{
		"doc_type_counts":      byType,
		"per_type_limit":       perTypeLimit,
		"requested_doc_types":  semanticMemoryNormalizedRequestedDocTypes(docTypes),
		"default_doc_type_set": len(docTypes) == 0,
	})
	if firstErr != nil {
		run.Status = SemanticMemorySyncStatusFailed
		run.ErrorMessage = firstErr.Error()
		run.Summary = fmt.Sprintf("Backfill finished with failures: %d docs failed, %d inserted, %d updated, %d unchanged.", run.FailedDocuments, run.InsertedDocuments, run.UpdatedDocuments, run.UnchangedDocuments)
	} else {
		run.Status = SemanticMemorySyncStatusCompleted
		run.Summary = fmt.Sprintf("Backfill completed: %d inserted, %d updated, %d unchanged across %d docs.", run.InsertedDocuments, run.UpdatedDocuments, run.UnchangedDocuments, run.TotalDocuments)
	}
	if err := s.db.Save(run).Error; err != nil {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, err
	}
	if firstErr != nil {
		return &SemanticMemoryBackfillResult{Run: run, ByDocType: byType}, firstErr
	}
	return &SemanticMemoryBackfillResult{Run: run, ByDocType: byType}, nil
}

func (s *SemanticMemoryStore) UpsertDocument(doc *SemanticMemoryDocument) (semanticMemoryUpsertOutcome, error) {
	if doc == nil {
		return semanticMemoryUpsertUnchanged, fmt.Errorf("semantic memory document cannot be nil")
	}
	doc.DocType = normalizeSemanticMemoryDocType(doc.DocType)
	doc.EmbeddingStatus = normalizeSemanticMemoryEmbeddingStatus(doc.EmbeddingStatus)
	doc.EmbeddingProvider = strings.TrimSpace(doc.EmbeddingProvider)
	doc.EmbeddingModel = strings.TrimSpace(doc.EmbeddingModel)
	if doc.EmbeddingProvider == "" {
		doc.EmbeddingProvider = SemanticMemoryDefaultEmbeddingProvider
	}
	if doc.EmbeddingModel == "" {
		doc.EmbeddingModel = SemanticMemoryDefaultEmbeddingModel
	}
	if doc.ID == "" {
		doc.ID = uuid.NewString()
	}
	doc.Title = strings.TrimSpace(doc.Title)
	doc.Summary = strings.TrimSpace(doc.Summary)
	doc.Body = strings.TrimSpace(doc.Body)
	doc.MetadataJSON = semanticMemoryNormalizeJSON(doc.MetadataJSON, "{}")
	doc.ContentHash = semanticMemoryHash(doc.Title + "\n" + doc.Summary + "\n" + doc.Body + "\n" + doc.MetadataJSON)
	doc.TokenEstimate = semanticMemoryApproxTokens(doc.Title + "\n" + doc.Summary + "\n" + doc.Body)
	doc.LastBuiltAt = time.Now().UTC()

	var existing SemanticMemoryDocument
	err := s.db.Where("user_id = ? AND doc_type = ? AND source_id = ?", doc.UserID, doc.DocType, doc.SourceID).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return semanticMemoryUpsertUnchanged, err
	}
	if err == gorm.ErrRecordNotFound {
		if doc.EmbeddingStatus == "" {
			doc.EmbeddingStatus = SemanticMemoryEmbeddingStatusPending
		}
		return semanticMemoryUpsertInserted, s.db.Create(doc).Error
	}

	if existing.ContentHash == doc.ContentHash &&
		existing.SourceUpdatedAt.Equal(doc.SourceUpdatedAt) &&
		existing.Title == doc.Title &&
		existing.Summary == doc.Summary &&
		existing.Body == doc.Body &&
		existing.MetadataJSON == doc.MetadataJSON {
		return semanticMemoryUpsertUnchanged, nil
	}

	updates := map[string]any{
		"trader_id":          doc.TraderID,
		"source_updated_at":  doc.SourceUpdatedAt,
		"title":              doc.Title,
		"summary":            doc.Summary,
		"body":               doc.Body,
		"metadata_json":      doc.MetadataJSON,
		"content_hash":       doc.ContentHash,
		"token_estimate":     doc.TokenEstimate,
		"last_built_at":      doc.LastBuiltAt,
		"embedding_provider": doc.EmbeddingProvider,
		"embedding_model":    doc.EmbeddingModel,
	}
	if existing.ContentHash != doc.ContentHash {
		updates["embedding_status"] = SemanticMemoryEmbeddingStatusPending
		updates["last_embedding_error"] = ""
	}
	return semanticMemoryUpsertUpdated, s.db.Model(&existing).Updates(updates).Error
}

func (s *SemanticMemoryStore) buildDealReviewCaseDocuments(userID, traderID string, limit int) ([]*SemanticMemoryDocument, error) {
	query := s.db.Model(&DealReviewCase{}).Where("user_id = ?", userID)
	if strings.TrimSpace(traderID) != "" {
		query = query.Where("trader_id = ?", traderID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	var items []DealReviewCase
	if err := query.Order("updated_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	docs := make([]*SemanticMemoryDocument, 0, len(items))
	for idx := range items {
		docs = append(docs, buildSemanticMemoryDealReviewCaseDocument(&items[idx]))
	}
	return docs, nil
}

func (s *SemanticMemoryStore) buildAutonomousOptimizerRunDocuments(userID, traderID string, limit int) ([]*SemanticMemoryDocument, error) {
	query := s.db.Model(&AutonomousOptimizerRun{}).Where("user_id = ?", userID)
	if strings.TrimSpace(traderID) != "" {
		query = query.Where("trader_id = ?", traderID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	var items []AutonomousOptimizerRun
	if err := query.Order("updated_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	docs := make([]*SemanticMemoryDocument, 0, len(items))
	for idx := range items {
		docs = append(docs, buildSemanticMemoryAutonomousOptimizerRunDocument(&items[idx]))
	}
	return docs, nil
}

func (s *SemanticMemoryStore) buildAutonomousOptimizerBacklogDocuments(userID, traderID string, limit int) ([]*SemanticMemoryDocument, error) {
	query := s.db.Model(&AutonomousOptimizerBacklogItem{}).Where("user_id = ?", userID)
	if strings.TrimSpace(traderID) != "" {
		query = query.Where("trader_id = ?", traderID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	var items []AutonomousOptimizerBacklogItem
	if err := query.Order("updated_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	docs := make([]*SemanticMemoryDocument, 0, len(items))
	for idx := range items {
		docs = append(docs, buildSemanticMemoryAutonomousOptimizerBacklogDocument(&items[idx]))
	}
	return docs, nil
}

func (s *SemanticMemoryStore) buildDecisionRecordSummaryDocuments(userID, traderID string, limit int) ([]*SemanticMemoryDocument, error) {
	if strings.TrimSpace(traderID) == "" {
		return nil, nil
	}
	query := s.db.Model(&DecisionRecordDB{}).
		Where("trader_id = ? AND candidate_metadata_version > 0 AND decisions IS NOT NULL AND decisions <> '[]' AND LOWER(TRIM(decisions)) <> 'null'", strings.TrimSpace(traderID))
	if limit > 0 {
		query = query.Limit(limit)
	}
	var items []DecisionRecordDB
	if err := query.Order("timestamp DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	docs := make([]*SemanticMemoryDocument, 0, len(items))
	for idx := range items {
		record := items[idx].toRecord()
		if !semanticMemoryShouldIndexDecisionRecordSummary(record) {
			continue
		}
		docs = append(docs, buildSemanticMemoryDecisionRecordSummaryDocument(userID, record))
	}
	return docs, nil
}

func (s *SemanticMemoryStore) buildStrategyVersionDocuments(userID, traderID string, limit int) ([]*SemanticMemoryDocument, error) {
	dealReview := NewDealReviewStore(s.db)
	query := s.db.Model(&DealReviewStrategyVersion{}).Where("user_id = ?", userID)
	if strings.TrimSpace(traderID) != "" {
		query = query.Where("trader_id = ?", traderID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	var items []DealReviewStrategyVersion
	if err := query.Order("applied_at DESC, created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	docs := make([]*SemanticMemoryDocument, 0, len(items))
	for idx := range items {
		detail, err := dealReview.GetStrategyVersion(items[idx].UserID, items[idx].TraderID, items[idx].ID)
		if err != nil {
			return nil, err
		}
		docs = append(docs, buildSemanticMemoryStrategyVersionDocument(detail))
	}
	return docs, nil
}

func (s *SemanticMemoryStore) buildSymbolBehaviorPriorDocuments(userID, traderID string, limit int) ([]*SemanticMemoryDocument, error) {
	dealReview := NewDealReviewStore(s.db)
	if _, err := dealReview.RefreshSymbolBehaviorPriorsIfStale(userID, traderID); err != nil {
		return nil, err
	}

	query := s.db.Model(&DealReviewSymbolBehaviorPrior{}).Where("user_id = ?", userID)
	if strings.TrimSpace(traderID) != "" {
		query = query.Where("trader_id = ?", traderID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var items []DealReviewSymbolBehaviorPrior
	if err := query.Order("composite_score DESC, sample_count DESC, last_observed_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}

	docs := make([]*SemanticMemoryDocument, 0, len(items))
	for idx := range items {
		hydrateDealReviewSymbolBehaviorPrior(&items[idx])
		docs = append(docs, buildSemanticMemorySymbolBehaviorPriorDocument(&items[idx]))
	}
	return docs, nil
}

func (s *SemanticMemoryStore) buildLearnedPatternDocuments(userID, traderID string, limit int) ([]*SemanticMemoryDocument, error) {
	dealReview := NewDealReviewStore(s.db)
	if _, err := dealReview.RefreshLearnedPatternsIfStale(userID, traderID); err != nil {
		return nil, err
	}

	filter := DealReviewLearnedPatternFilter{Limit: limit}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	items, err := dealReview.ListLearnedPatterns(userID, traderID, filter)
	if err != nil {
		return nil, err
	}
	docs := make([]*SemanticMemoryDocument, 0, len(items))
	for idx := range items {
		docs = append(docs, buildSemanticMemoryLearnedPatternDocument(&items[idx]))
	}
	return docs, nil
}

func buildSemanticMemoryDealReviewCaseDocument(item *DealReviewCase) *SemanticMemoryDocument {
	if item == nil {
		return nil
	}
	labels := semanticMemoryNormalizeJSONArray(item.LabelsJSON)
	candidates := semanticMemoryNormalizeJSONArray(item.OpenCandidateSourcesJSON)
	title := fmt.Sprintf("Deal review case %s %s %s", strings.ToUpper(strings.TrimSpace(item.Symbol)), strings.ToLower(strings.TrimSpace(item.Side)), strings.ToLower(strings.TrimSpace(item.Outcome)))
	summary := fmt.Sprintf("Outcome %s with close_reason %s and exit_origin %s. Realized PnL %.4f (%.2f%%).", blankToValue(item.Outcome, "unknown"), blankToValue(item.CloseReason, "unknown"), blankToValue(item.ExitOrigin, "unknown"), item.RealizedPnL, item.RealizedPnLPct)
	body := strings.TrimSpace(fmt.Sprintf(`
Trader: %s
Position: %s %s
Status: %s
Outcome: %s
Entry: price %.8f qty %.8f leverage %d
Exit: price %.8f qty %.8f
PnL: %.8f (%.2f%%)
Close reason: %s
Exit origin: %s
Exit reason quality: %s
Exit evidence summary: %s
Open selection bucket: %s
Open regimes: trend=%s volatility=%s btc_strength=%s funding=%s oi=%s session=%s weekday=%s venue=%s liquidity=%s spread=%s slippage=%s
Close regimes: trend=%s volatility=%s btc_strength=%s funding=%s oi=%s session=%s weekday=%s venue=%s liquidity=%s spread=%s slippage=%s
Scores: entry_timing=%.2f exit_efficiency=%.2f risk_sizing=%.2f
MFE/MAE: mfe=%.8f (%.2f%%) mae=%.8f (%.2f%%)
Profit give-back: %.8f (%.2f%%)
Time to first profit ms: %d
Time to max drawdown ms: %d
Candidate sources: %s
Labels: %s
Analyst note: %s
`, item.TraderID, strings.ToUpper(strings.TrimSpace(item.Symbol)), strings.ToUpper(strings.TrimSpace(item.Side)), blankToValue(item.Status, "unknown"), blankToValue(item.Outcome, "unknown"), item.EntryPrice, item.EntryQuantity, item.Leverage, item.ExitPrice, item.ExitQuantity, item.RealizedPnL, item.RealizedPnLPct, blankToValue(item.CloseReason, "unknown"), blankToValue(item.ExitOrigin, "unknown"), blankToValue(item.ExitReasonQuality, "unknown"), blankToValue(item.ExitEvidenceSummary, "n/a"), blankToValue(item.OpenSelectionBucket, "n/a"), blankToValue(item.OpenTrendRegime, "n/a"), blankToValue(item.OpenVolatilityRegime, "n/a"), blankToValue(item.OpenBTCStrengthRegime, "n/a"), blankToValue(item.OpenFundingRegime, "n/a"), blankToValue(item.OpenOIRegime, "n/a"), blankToValue(item.OpenSessionBucket, "n/a"), blankToValue(item.OpenWeekdayBucket, "n/a"), blankToValue(item.OpenVenueTier, "n/a"), blankToValue(item.OpenLiquidityTier, "n/a"), blankToValue(item.OpenSpreadBucket, "n/a"), blankToValue(item.OpenSlippageBucket, "n/a"), blankToValue(item.CloseTrendRegime, "n/a"), blankToValue(item.CloseVolatilityRegime, "n/a"), blankToValue(item.CloseBTCStrengthRegime, "n/a"), blankToValue(item.CloseFundingRegime, "n/a"), blankToValue(item.CloseOIRegime, "n/a"), blankToValue(item.CloseSessionBucket, "n/a"), blankToValue(item.CloseWeekdayBucket, "n/a"), blankToValue(item.CloseVenueTier, "n/a"), blankToValue(item.CloseLiquidityTier, "n/a"), blankToValue(item.CloseSpreadBucket, "n/a"), blankToValue(item.CloseSlippageBucket, "n/a"), item.EntryTimingScore, item.ExitEfficiencyScore, item.RiskSizingScore, item.MaxFavorableExcursion, item.MaxFavorableExcursionPct, item.MaxAdverseExcursion, item.MaxAdverseExcursionPct, item.ProfitGivenBack, item.ProfitGivenBackPct, item.TimeToFirstProfitMs, item.TimeToMaxDrawdownMs, strings.Join(candidates, ", "), strings.Join(labels, ", "), strings.TrimSpace(item.AnalystNote)))
	metadata := semanticMemoryMarshalJSON(map[string]any{
		"symbol":                  strings.ToUpper(strings.TrimSpace(item.Symbol)),
		"side":                    strings.ToUpper(strings.TrimSpace(item.Side)),
		"status":                  item.Status,
		"outcome":                 item.Outcome,
		"close_reason":            item.CloseReason,
		"exit_origin":             item.ExitOrigin,
		"exit_reason_quality":     item.ExitReasonQuality,
		"open_selection_bucket":   item.OpenSelectionBucket,
		"open_trend_regime":       item.OpenTrendRegime,
		"open_volatility_regime":  item.OpenVolatilityRegime,
		"open_oi_regime":          item.OpenOIRegime,
		"close_trend_regime":      item.CloseTrendRegime,
		"close_volatility_regime": item.CloseVolatilityRegime,
		"close_oi_regime":         item.CloseOIRegime,
		"labels":                  labels,
	})
	return &SemanticMemoryDocument{
		UserID:            item.UserID,
		TraderID:          item.TraderID,
		DocType:           SemanticMemoryDocTypeDealReviewCase,
		SourceID:          item.ID,
		SourceUpdatedAt:   item.UpdatedAt,
		Title:             title,
		Summary:           summary,
		Body:              body,
		MetadataJSON:      metadata,
		EmbeddingProvider: SemanticMemoryDefaultEmbeddingProvider,
		EmbeddingModel:    SemanticMemoryDefaultEmbeddingModel,
		EmbeddingStatus:   SemanticMemoryEmbeddingStatusPending,
	}
}

func buildSemanticMemoryAutonomousOptimizerRunDocument(item *AutonomousOptimizerRun) *SemanticMemoryDocument {
	if item == nil {
		return nil
	}
	configPatch := semanticMemoryParseJSONObject(item.ConfigPatchJSON)
	promptPatch := semanticMemoryParseJSONObject(item.PromptPatchJSON)
	validation := semanticMemoryParseJSONObject(item.ValidationJSON)
	metadataObject := semanticMemoryParseJSONObject(item.MetadataJSON)
	proposal := semanticMemoryNestedJSONObject(metadataObject, "proposal")
	critic := semanticMemoryNestedJSONObject(validation, "critic")
	configValidation := semanticMemoryNestedJSONObject(validation, "config_validation")

	proposalType := semanticMemoryMetadataString(proposal, "proposal_type")
	if proposalType == "" {
		proposalType = semanticMemoryMetadataString(metadataObject, "proposal_type")
	}
	criticAction := semanticMemoryMetadataString(critic, "recommended_action")
	configValidationStatus := semanticMemoryMetadataString(configValidation, "status")
	if configValidationStatus == "" {
		configValidationStatus = semanticMemoryMetadataString(validation, "status")
	}

	gateReasons := semanticMemoryUniqueSortedStrings(append(
		semanticMemoryStringSliceValue(validation["gate_reasons"]),
		semanticMemoryStringSliceValue(metadataObject["gate_reasons"])...,
	))
	deferredReasons := semanticMemoryUniqueSortedStrings(append(
		semanticMemoryStringSliceValue(validation["deferred_reasons"]),
		semanticMemoryStringSliceValue(metadataObject["deferred_reasons"])...,
	))
	configPatchKeys := semanticMemoryFlattenJSONPaths(configPatch, "")
	configPatchPaths := semanticMemoryUniqueSortedStrings(append(
		semanticMemoryStringSliceValue(validation["config_patch_paths"]),
		configPatchKeys...,
	))
	promptPatchKeys := semanticMemoryFlattenJSONPaths(promptPatch, "")
	criticBlockingIssues := semanticMemoryStringSliceValue(critic["blocking_issues"])
	proposalRationale := semanticMemoryStringSliceValue(proposal["rationale"])
	backlogItemsCreated := semanticMemoryMetadataInt(metadataObject, "backlog_items_created")

	title := fmt.Sprintf("Optimizer run %s %s", blankToValue(item.Status, "unknown"), item.ID)
	summaryParts := []string{
		fmt.Sprintf("Trigger %s", blankToValue(item.Trigger, "unknown")),
		fmt.Sprintf("status %s", blankToValue(item.Status, "unknown")),
		fmt.Sprintf("proposer %s", blankToValue(item.PrimaryModelName, AutonomousOptimizerDefaultModelName)),
		fmt.Sprintf("critic %s", blankToValue(item.CriticModelName, AutonomousOptimizerDefaultModelName)),
	}
	if proposalType != "" {
		summaryParts = append(summaryParts, fmt.Sprintf("proposal %s", proposalType))
	}
	if criticAction != "" {
		summaryParts = append(summaryParts, fmt.Sprintf("critic_action %s", criticAction))
	}
	if configValidationStatus != "" {
		summaryParts = append(summaryParts, fmt.Sprintf("config_validation %s", configValidationStatus))
	}
	if len(gateReasons) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("gate_reasons %d", len(gateReasons)))
	}
	if len(deferredReasons) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("deferred %d", len(deferredReasons)))
	}
	summary := fmt.Sprintf("%s. %s", strings.Join(summaryParts, ", "), strings.TrimSpace(item.Summary))
	body := strings.TrimSpace(fmt.Sprintf(`
Trader: %s
Run ID: %s
Trigger: %s
Status: %s
Primary model: %s
Critic model: %s
Source trader: %s
Source strategy: %s
Applied strategy version: %s
Proposal type: %s
Critic recommended action: %s
Config validation status: %s
Backlog items created: %d
Summary: %s
Proposal rationale:
%s

Gate reasons:
%s

Deferred reasons:
%s

Critic blocking issues:
%s

Config patch paths:
%s

Prompt patch paths:
%s

Config patch:
%s

Prompt patch:
%s

Validation:
%s

Proposal:
%s
`, item.TraderID, item.ID, blankToValue(item.Trigger, "unknown"), blankToValue(item.Status, "unknown"), blankToValue(item.PrimaryModelName, AutonomousOptimizerDefaultModelName), blankToValue(item.CriticModelName, AutonomousOptimizerDefaultModelName), blankToValue(item.SourceTraderID, "n/a"), blankToValue(item.SourceStrategyID, "n/a"), blankToValue(item.AppliedStrategyVersionID, "n/a"), blankToValue(proposalType, "n/a"), blankToValue(criticAction, "n/a"), blankToValue(configValidationStatus, "n/a"), backlogItemsCreated, strings.TrimSpace(item.Summary), semanticMemoryJoinStringsForBody(proposalRationale, "n/a", 6), semanticMemoryJoinStringsForBody(gateReasons, "n/a", 6), semanticMemoryJoinStringsForBody(deferredReasons, "n/a", 4), semanticMemoryJoinStringsForBody(criticBlockingIssues, "n/a", 6), semanticMemoryJoinStringsForBody(configPatchPaths, "n/a", 12), semanticMemoryJoinStringsForBody(promptPatchKeys, "n/a", 12), semanticMemoryCompactJSON(item.ConfigPatchJSON, 1600), semanticMemoryCompactJSON(item.PromptPatchJSON, 1600), semanticMemoryCompactJSON(item.ValidationJSON, 1600), semanticMemoryCompactJSON(semanticMemoryMarshalJSON(proposal), 1600)))
	metadata := semanticMemoryMarshalJSON(map[string]any{
		"status":                      item.Status,
		"trigger":                     item.Trigger,
		"primary_model_name":          item.PrimaryModelName,
		"critic_model_name":           item.CriticModelName,
		"applied_strategy_version_id": item.AppliedStrategyVersionID,
		"proposal_type":               proposalType,
		"critic_recommended_action":   criticAction,
		"config_validation_status":    configValidationStatus,
		"gate_reasons":                gateReasons,
		"deferred_reasons":            deferredReasons,
		"config_patch_paths":          configPatchPaths,
		"config_patch_keys":           configPatchKeys,
		"prompt_patch_keys":           promptPatchKeys,
		"backlog_items_created":       backlogItemsCreated,
	})
	return &SemanticMemoryDocument{
		UserID:            item.UserID,
		TraderID:          item.TraderID,
		DocType:           SemanticMemoryDocTypeAutonomousOptimizerRun,
		SourceID:          item.ID,
		SourceUpdatedAt:   item.UpdatedAt,
		Title:             title,
		Summary:           summary,
		Body:              body,
		MetadataJSON:      metadata,
		EmbeddingProvider: SemanticMemoryDefaultEmbeddingProvider,
		EmbeddingModel:    SemanticMemoryDefaultEmbeddingModel,
		EmbeddingStatus:   SemanticMemoryEmbeddingStatusPending,
	}
}

func buildSemanticMemoryAutonomousOptimizerBacklogDocument(item *AutonomousOptimizerBacklogItem) *SemanticMemoryDocument {
	if item == nil {
		return nil
	}
	title := fmt.Sprintf("Optimizer backlog %s", strings.TrimSpace(item.Title))
	summary := fmt.Sprintf("Category %s, status %s, score %.2f, recurrence %d.", blankToValue(item.Category, "uncategorized"), blankToValue(item.Status, "unknown"), item.CompositeScore, item.RecurrenceCount)
	body := strings.TrimSpace(fmt.Sprintf(`
Trader: %s
Run ID: %s
Title: %s
Category: %s
Status: %s
Confidence: %.2f
Implementation cost: %.2f
Urgency: %.2f
Composite score: %.2f
Recurrence count: %d
AI generated: %t
User edited: %t
Expected impact: %s
Description: %s
Evidence:
%s

Metadata:
%s
`, item.TraderID, blankToValue(item.RunID, "n/a"), strings.TrimSpace(item.Title), blankToValue(item.Category, "uncategorized"), blankToValue(item.Status, "unknown"), item.Confidence, item.ImplementationCost, item.Urgency, item.CompositeScore, item.RecurrenceCount, item.AIGenerated, item.UserEdited, strings.TrimSpace(item.ExpectedImpact), strings.TrimSpace(item.Description), semanticMemoryCompactJSON(item.EvidenceJSON, 2400), semanticMemoryCompactJSON(item.MetadataJSON, 1600)))
	metadata := semanticMemoryMarshalJSON(map[string]any{
		"category":        item.Category,
		"status":          item.Status,
		"composite_score": item.CompositeScore,
		"run_id":          item.RunID,
	})
	return &SemanticMemoryDocument{
		UserID:            item.UserID,
		TraderID:          item.TraderID,
		DocType:           SemanticMemoryDocTypeOptimizerBacklogItem,
		SourceID:          item.ID,
		SourceUpdatedAt:   item.UpdatedAt,
		Title:             title,
		Summary:           summary,
		Body:              body,
		MetadataJSON:      metadata,
		EmbeddingProvider: SemanticMemoryDefaultEmbeddingProvider,
		EmbeddingModel:    SemanticMemoryDefaultEmbeddingModel,
		EmbeddingStatus:   SemanticMemoryEmbeddingStatusPending,
	}
}

func buildSemanticMemoryStrategyVersionDocument(detail *DealReviewStrategyVersionDetail) *SemanticMemoryDocument {
	if detail == nil {
		return nil
	}
	version := detail.Version
	appliedAt := effectiveDealReviewStrategyVersionTime(version)
	if appliedAt.IsZero() {
		appliedAt = version.UpdatedAt
	}
	attribution := detail.Attribution
	observationReady := attribution != nil && attribution.ObservationReady
	rollbackSuggested := attribution != nil && attribution.RollbackSuggested
	title := fmt.Sprintf("Strategy version %s %s", blankToValue(version.SourceType, "unknown"), version.ID)
	summary := fmt.Sprintf(
		"Strategy %s, source %s, observation_ready=%t, rollback_suggested=%t. %s",
		blankToValue(version.StrategyID, "unknown"),
		blankToValue(version.SourceType, "unknown"),
		observationReady,
		rollbackSuggested,
		strings.TrimSpace(version.Summary),
	)

	warnings := ""
	note := ""
	fullBefore := "{}"
	fullAfter := "{}"
	targetBefore := "{}"
	targetAfter := "{}"
	nonTargetBefore := "{}"
	nonTargetAfter := "{}"
	if attribution != nil {
		if len(attribution.Warnings) > 0 {
			warnings = strings.Join(attribution.Warnings, "; ")
		}
		note = strings.TrimSpace(attribution.Note)
		fullBefore = semanticMemoryMarshalJSON(attribution.FullBeforeSummary)
		fullAfter = semanticMemoryMarshalJSON(attribution.FullAfterSummary)
		targetBefore = semanticMemoryMarshalJSON(attribution.TargetBeforeSummary)
		targetAfter = semanticMemoryMarshalJSON(attribution.TargetAfterSummary)
		nonTargetBefore = semanticMemoryMarshalJSON(attribution.NonTargetBeforeSummary)
		nonTargetAfter = semanticMemoryMarshalJSON(attribution.NonTargetAfterSummary)
	}

	body := strings.TrimSpace(fmt.Sprintf(`
Trader: %s
Strategy ID: %s
Version ID: %s
Source type: %s
Source scan ID: %s
Source compare ID: %s
Applied at: %s
Summary: %s
Expected effect: %s
Source scan summary: %s
Compare summary: %s
Target cohort:
%s

Previous config:
%s

Next config:
%s

Observation ready: %t
Rollback suggested: %t
Warnings: %s
Attribution note: %s

Attribution full before:
%s

Attribution full after:
%s

Attribution target before:
%s

Attribution target after:
%s

Attribution non-target before:
%s

Attribution non-target after:
%s
`,
		version.TraderID,
		blankToValue(version.StrategyID, "unknown"),
		version.ID,
		blankToValue(version.SourceType, "unknown"),
		blankToValue(version.SourceScanID, "n/a"),
		blankToValue(version.SourceCompareID, "n/a"),
		appliedAt.UTC().Format(time.RFC3339),
		strings.TrimSpace(version.Summary),
		strings.TrimSpace(version.ExpectedEffect),
		strings.TrimSpace(detail.SourceScanSummary),
		strings.TrimSpace(detail.CompareSummary),
		semanticMemoryCompactJSON(semanticMemoryMarshalJSON(detail.TargetCohort), 1600),
		semanticMemoryCompactJSON(semanticMemoryMarshalJSON(detail.PreviousConfig), 2400),
		semanticMemoryCompactJSON(semanticMemoryMarshalJSON(detail.NextConfig), 2400),
		observationReady,
		rollbackSuggested,
		blankToValue(warnings, "none"),
		blankToValue(note, "n/a"),
		semanticMemoryCompactJSON(fullBefore, 1200),
		semanticMemoryCompactJSON(fullAfter, 1200),
		semanticMemoryCompactJSON(targetBefore, 1200),
		semanticMemoryCompactJSON(targetAfter, 1200),
		semanticMemoryCompactJSON(nonTargetBefore, 1200),
		semanticMemoryCompactJSON(nonTargetAfter, 1200),
	))

	metadata := semanticMemoryMarshalJSON(map[string]any{
		"strategy_id":         version.StrategyID,
		"source_type":         version.SourceType,
		"source_scan_id":      version.SourceScanID,
		"source_compare_id":   version.SourceCompareID,
		"observation_ready":   observationReady,
		"rollback_suggested":  rollbackSuggested,
		"target_cohort":       detail.TargetCohort,
		"source_scan_summary": detail.SourceScanSummary,
		"compare_summary":     detail.CompareSummary,
	})
	return &SemanticMemoryDocument{
		UserID:            version.UserID,
		TraderID:          version.TraderID,
		DocType:           SemanticMemoryDocTypeStrategyVersion,
		SourceID:          version.ID,
		SourceUpdatedAt:   version.UpdatedAt,
		Title:             title,
		Summary:           summary,
		Body:              body,
		MetadataJSON:      metadata,
		EmbeddingProvider: SemanticMemoryDefaultEmbeddingProvider,
		EmbeddingModel:    SemanticMemoryDefaultEmbeddingModel,
		EmbeddingStatus:   SemanticMemoryEmbeddingStatusPending,
	}
}

func buildSemanticMemoryDecisionRecordSummaryDocument(userID string, record *DecisionRecord) *SemanticMemoryDocument {
	if record == nil {
		return nil
	}
	candidateSymbols := make([]string, 0, len(record.CandidateDetails))
	selectionBuckets := make([]string, 0, len(record.CandidateDetails))
	rejectReasons := make([]string, 0, len(record.CandidateDetails))
	candidateSources := make([]string, 0, len(record.CandidateDetails))
	sessions := []string{}
	trendRegimes := []string{}
	volatilityRegimes := []string{}
	oiRegimes := []string{}
	actionTypes := make([]string, 0, len(record.Decisions))
	actionSymbols := make([]string, 0, len(record.Decisions))
	terminalStatuses := make([]string, 0, len(record.Decisions))
	actionLines := make([]string, 0, len(record.Decisions))
	openCount := 0
	closeCount := 0
	holdCount := 0
	waitCount := 0

	for _, candidate := range record.CandidateDetails {
		if symbol := strings.ToUpper(strings.TrimSpace(candidate.Symbol)); symbol != "" {
			candidateSymbols = append(candidateSymbols, symbol)
		}
		if bucket := strings.TrimSpace(candidate.SelectionBucket); bucket != "" {
			selectionBuckets = append(selectionBuckets, bucket)
		}
		rejectReasons = append(rejectReasons, candidate.RejectReasons...)
		candidateSources = append(candidateSources, candidate.Sources...)
		if candidate.MarketContext != nil {
			if value := strings.TrimSpace(candidate.MarketContext.SessionBucket); value != "" {
				sessions = append(sessions, value)
			}
			if value := strings.TrimSpace(candidate.MarketContext.TrendRegime); value != "" {
				trendRegimes = append(trendRegimes, value)
			}
			if value := strings.TrimSpace(candidate.MarketContext.VolatilityRegime); value != "" {
				volatilityRegimes = append(volatilityRegimes, value)
			}
			if value := strings.TrimSpace(candidate.MarketContext.OIRegime); value != "" {
				oiRegimes = append(oiRegimes, value)
			}
		}
	}

	for _, action := range record.Decisions {
		actionType := strings.TrimSpace(action.Action)
		if actionType == "" {
			continue
		}
		actionTypes = append(actionTypes, actionType)
		if symbol := strings.ToUpper(strings.TrimSpace(action.Symbol)); symbol != "" {
			actionSymbols = append(actionSymbols, symbol)
		}
		rejectReasons = append(rejectReasons, action.RejectReasons...)
		if action.Execution != nil {
			if value := strings.TrimSpace(action.Execution.TerminalStatus); value != "" {
				terminalStatuses = append(terminalStatuses, value)
			}
		}
		if action.MarketContext != nil {
			if value := strings.TrimSpace(action.MarketContext.SessionBucket); value != "" {
				sessions = append(sessions, value)
			}
			if value := strings.TrimSpace(action.MarketContext.TrendRegime); value != "" {
				trendRegimes = append(trendRegimes, value)
			}
			if value := strings.TrimSpace(action.MarketContext.VolatilityRegime); value != "" {
				volatilityRegimes = append(volatilityRegimes, value)
			}
			if value := strings.TrimSpace(action.MarketContext.OIRegime); value != "" {
				oiRegimes = append(oiRegimes, value)
			}
		}
		switch {
		case strings.HasPrefix(actionType, "open_"):
			openCount++
		case strings.HasPrefix(actionType, "close_"):
			closeCount++
		case actionType == "hold":
			holdCount++
		case actionType == "wait":
			waitCount++
		}
		line := fmt.Sprintf("%s %s conf=%d", strings.ToUpper(actionType), strings.ToUpper(strings.TrimSpace(action.Symbol)), action.Confidence)
		if reasoning := strings.TrimSpace(action.Reasoning); reasoning != "" {
			line += " reason=" + reasoning
		}
		if reasons := semanticMemoryJoinStringsForBody(action.RejectReasons, "", 3); reasons != "" {
			line += " rejects=" + reasons
		}
		if action.Execution != nil && strings.TrimSpace(action.Execution.TerminalStatus) != "" {
			line += " exec=" + strings.TrimSpace(action.Execution.TerminalStatus)
		}
		actionLines = append(actionLines, line)
	}

	candidateSymbols = semanticMemoryUniqueSortedStrings(candidateSymbols)
	selectionBuckets = semanticMemoryUniqueSortedStrings(selectionBuckets)
	rejectReasons = semanticMemoryUniqueSortedStrings(rejectReasons)
	candidateSources = semanticMemoryUniqueSortedStrings(candidateSources)
	sessions = semanticMemoryUniqueSortedStrings(sessions)
	trendRegimes = semanticMemoryUniqueSortedStrings(trendRegimes)
	volatilityRegimes = semanticMemoryUniqueSortedStrings(volatilityRegimes)
	oiRegimes = semanticMemoryUniqueSortedStrings(oiRegimes)
	actionTypes = semanticMemoryUniqueSortedStrings(actionTypes)
	actionSymbols = semanticMemoryUniqueSortedStrings(actionSymbols)
	terminalStatuses = semanticMemoryUniqueSortedStrings(terminalStatuses)

	title := fmt.Sprintf("Decision cycle %d %s", record.CycleNumber, blankToValue(strings.Join(actionTypes, "/"), "no_action"))
	summary := fmt.Sprintf(
		"Trader %s cycle %d with %d candidates, %d opens, %d holds, %d waits, %d closes. Buckets %s. Reject reasons %s.",
		record.TraderID,
		record.CycleNumber,
		len(record.CandidateDetails),
		openCount,
		holdCount,
		waitCount,
		closeCount,
		semanticMemoryJoinStringsForBody(selectionBuckets, "n/a", 4),
		semanticMemoryJoinStringsForBody(rejectReasons, "none", 4),
	)
	body := strings.TrimSpace(fmt.Sprintf(`
Trader: %s
Cycle number: %d
Timestamp: %s
Candidate metadata version: %d
Candidate count: %d
Decision count: %d
Action mix: open=%d hold=%d wait=%d close=%d
Candidate symbols: %s
Action symbols: %s
Selection buckets: %s
Reject reasons: %s
Candidate sources: %s
Sessions: %s
Trend regimes: %s
Volatility regimes: %s
OI regimes: %s
Terminal statuses: %s
Account snapshot: balance=%.2f available=%.2f positions=%d margin_used_pct=%.2f

Actions:
%s
`,
		record.TraderID,
		record.CycleNumber,
		record.Timestamp.UTC().Format(time.RFC3339),
		record.CandidateMetaVer,
		len(record.CandidateDetails),
		len(record.Decisions),
		openCount,
		holdCount,
		waitCount,
		closeCount,
		semanticMemoryJoinStringsForBody(candidateSymbols, "n/a", 10),
		semanticMemoryJoinStringsForBody(actionSymbols, "n/a", 10),
		semanticMemoryJoinStringsForBody(selectionBuckets, "n/a", 8),
		semanticMemoryJoinStringsForBody(rejectReasons, "none", 8),
		semanticMemoryJoinStringsForBody(candidateSources, "n/a", 6),
		semanticMemoryJoinStringsForBody(sessions, "n/a", 4),
		semanticMemoryJoinStringsForBody(trendRegimes, "n/a", 4),
		semanticMemoryJoinStringsForBody(volatilityRegimes, "n/a", 4),
		semanticMemoryJoinStringsForBody(oiRegimes, "n/a", 4),
		semanticMemoryJoinStringsForBody(terminalStatuses, "n/a", 4),
		record.AccountState.TotalBalance,
		record.AccountState.AvailableBalance,
		record.AccountState.PositionCount,
		record.AccountState.MarginUsedPct,
		semanticMemoryJoinStringsForBody(actionLines, "n/a", 8),
	))
	metadata := semanticMemoryMarshalJSON(map[string]any{
		"cycle_number":         record.CycleNumber,
		"action_types":         actionTypes,
		"symbols":              semanticMemoryUniqueSortedStrings(append(append([]string{}, candidateSymbols...), actionSymbols...)),
		"selection_buckets":    selectionBuckets,
		"reject_reasons":       rejectReasons,
		"candidate_sources":    candidateSources,
		"sessions":             sessions,
		"trend_regimes":        trendRegimes,
		"volatility_regimes":   volatilityRegimes,
		"oi_regimes":           oiRegimes,
		"terminal_statuses":    terminalStatuses,
		"open_decision_count":  openCount,
		"hold_decision_count":  holdCount,
		"wait_decision_count":  waitCount,
		"close_decision_count": closeCount,
	})
	return &SemanticMemoryDocument{
		UserID:            strings.TrimSpace(userID),
		TraderID:          record.TraderID,
		DocType:           SemanticMemoryDocTypeDecisionRecordSummary,
		SourceID:          fmt.Sprintf("%s:%d", record.TraderID, record.CycleNumber),
		SourceUpdatedAt:   record.Timestamp.UTC(),
		Title:             title,
		Summary:           summary,
		Body:              body,
		MetadataJSON:      metadata,
		EmbeddingProvider: SemanticMemoryDefaultEmbeddingProvider,
		EmbeddingModel:    SemanticMemoryDefaultEmbeddingModel,
		EmbeddingStatus:   SemanticMemoryEmbeddingStatusPending,
	}
}

func buildSemanticMemorySymbolBehaviorPriorDocument(item *DealReviewSymbolBehaviorPrior) *SemanticMemoryDocument {
	if item == nil {
		return nil
	}

	evidenceCaseIDs := make([]string, 0, len(item.Evidence))
	for _, evidence := range item.Evidence {
		caseID := strings.TrimSpace(evidence.CaseID)
		if caseID == "" {
			continue
		}
		evidenceCaseIDs = append(evidenceCaseIDs, caseID)
	}
	decisionCycles := make([]string, 0, len(item.DecisionEvidence))
	for _, evidence := range item.DecisionEvidence {
		if evidence.CycleNumber <= 0 {
			continue
		}
		decisionCycles = append(decisionCycles, strconv.Itoa(evidence.CycleNumber))
	}

	evidenceCaseIDs = semanticMemoryUniqueSortedStrings(evidenceCaseIDs)
	decisionCycles = semanticMemoryUniqueSortedStrings(decisionCycles)
	sourceUpdatedAt := item.BuiltAt.UTC()
	if sourceUpdatedAt.IsZero() {
		sourceUpdatedAt = item.UpdatedAt.UTC()
	}
	if sourceUpdatedAt.IsZero() {
		sourceUpdatedAt = item.LastObservedAt.UTC()
	}

	title := fmt.Sprintf(
		"Symbol prior %s %s %s",
		strings.ToUpper(strings.TrimSpace(item.Symbol)),
		strings.ToLower(strings.TrimSpace(item.Side)),
		blankToValue(item.ValidationLabel, item.Status),
	)
	summary := fmt.Sprintf(
		"Bias %s, status %s, validation %s. Avg PnL %+.4f (%+.2f%%) across %d deals with contradiction %.0f%%.",
		blankToValue(item.BehaviorBias, "mixed"),
		blankToValue(item.Status, "unknown"),
		blankToValue(item.ValidationLabel, "n/a"),
		item.AvgPnL,
		item.AvgPnLPct,
		item.SampleCount,
		item.ContradictionScore*100,
	)
	body := strings.TrimSpace(fmt.Sprintf(`
Trader: %s
Prior ID: %s
Symbol: %s
Side: %s
Behavior bias: %s
Recommended action: %s
Status: %s
Validation label: %s
Validation alert: %s
Regime signature: %s
Open selection bucket: %s
Open regimes: trend=%s volatility=%s oi=%s
Samples: total=%d training=%d holdout=%d recent=%d
Win/loss/flat: %d / %d / %d
PnL: net=%+.8f avg=%+.8f avg_pct=%+.2f%% expectancy=%+.8f
MFE/MAE: avg_mfe_pct=%+.2f%% avg_mae_pct=%+.2f%%
Give-back: rate=%.2f%% avg=%.2f%%
Decision evidence: %d opens across %d cycles, avg confidence %.2f%%
Scores: contradiction=%.0f%% confidence=%.0f%% stability=%.0f%% recency=%.0f%% composite=%.0f%% drift=%.0f%%
False-positive score: %.0f%%
False-negative score: %.0f%%
Validation summary: %s
Signal cluster key: %s
Signal clusters: %s
Signal tags: %s
Evidence case IDs: %s
Decision cycles: %s
Summary: %s
`,
		item.TraderID,
		item.ID,
		strings.ToUpper(strings.TrimSpace(item.Symbol)),
		strings.ToUpper(strings.TrimSpace(item.Side)),
		blankToValue(item.BehaviorBias, "mixed"),
		blankToValue(item.RecommendedAction, "observe"),
		blankToValue(item.Status, "unknown"),
		blankToValue(item.ValidationLabel, "n/a"),
		blankToValue(item.ValidationAlert, "n/a"),
		blankToValue(item.RegimeSignature, "n/a"),
		blankToValue(item.OpenSelectionBucket, "n/a"),
		blankToValue(item.OpenTrendRegime, "n/a"),
		blankToValue(item.OpenVolatilityRegime, "n/a"),
		blankToValue(item.OpenOIRegime, "n/a"),
		item.SampleCount,
		item.TrainingSampleCount,
		item.ValidationSampleCount,
		item.RecentSampleCount,
		item.WinningDeals,
		item.LosingDeals,
		item.FlatDeals,
		item.NetPnL,
		item.AvgPnL,
		item.AvgPnLPct,
		item.Expectancy,
		item.AvgMFEPct,
		item.AvgMAEPct,
		item.GiveBackRate*100,
		item.AvgGiveBackPct,
		item.DecisionOpenCount,
		item.DecisionCycleCount,
		item.AvgDecisionConfidence,
		item.ContradictionScore*100,
		item.ConfidenceScore*100,
		item.StabilityScore*100,
		item.RecencyWeight*100,
		item.CompositeScore*100,
		item.DriftScore*100,
		item.FalsePositiveScore*100,
		item.FalseNegativeScore*100,
		blankToValue(item.ValidationSummary, "n/a"),
		blankToValue(item.SignalClusterKey, "n/a"),
		semanticMemoryJoinStringsForBody(item.SignalClusters, "none", 8),
		semanticMemoryJoinStringsForBody(item.SignalTags, "none", 8),
		semanticMemoryJoinStringsForBody(evidenceCaseIDs, "none", 8),
		semanticMemoryJoinStringsForBody(decisionCycles, "none", 8),
		blankToValue(item.Summary, "n/a"),
	))
	metadata := semanticMemoryMarshalJSON(map[string]any{
		"prior_id":               item.ID,
		"symbol":                 strings.ToUpper(strings.TrimSpace(item.Symbol)),
		"side":                   strings.ToUpper(strings.TrimSpace(item.Side)),
		"status":                 item.Status,
		"validation_label":       item.ValidationLabel,
		"behavior_bias":          item.BehaviorBias,
		"recommended_action":     item.RecommendedAction,
		"open_selection_bucket":  item.OpenSelectionBucket,
		"open_trend_regime":      item.OpenTrendRegime,
		"open_volatility_regime": item.OpenVolatilityRegime,
		"open_oi_regime":         item.OpenOIRegime,
		"regime_signature":       item.RegimeSignature,
		"signal_cluster_key":     item.SignalClusterKey,
		"signal_clusters":        item.SignalClusters,
		"signal_tags":            item.SignalTags,
		"evidence_case_ids":      evidenceCaseIDs,
		"decision_cycles":        decisionCycles,
		"sample_count":           item.SampleCount,
		"decision_cycle_count":   item.DecisionCycleCount,
		"decision_open_count":    item.DecisionOpenCount,
		"false_positive_score":   item.FalsePositiveScore,
		"false_negative_score":   item.FalseNegativeScore,
	})
	return &SemanticMemoryDocument{
		UserID:            item.UserID,
		TraderID:          item.TraderID,
		DocType:           SemanticMemoryDocTypeSymbolBehaviorPrior,
		SourceID:          buildSemanticMemorySymbolBehaviorPriorSourceID(item),
		SourceUpdatedAt:   sourceUpdatedAt,
		Title:             title,
		Summary:           summary,
		Body:              body,
		MetadataJSON:      metadata,
		EmbeddingProvider: SemanticMemoryDefaultEmbeddingProvider,
		EmbeddingModel:    SemanticMemoryDefaultEmbeddingModel,
		EmbeddingStatus:   SemanticMemoryEmbeddingStatusPending,
	}
}

func buildSemanticMemoryLearnedPatternDocument(item *DealReviewLearnedPattern) *SemanticMemoryDocument {
	if item == nil {
		return nil
	}

	evidenceCaseIDs := make([]string, 0, len(item.Evidence))
	for _, evidence := range item.Evidence {
		caseID := strings.TrimSpace(evidence.CaseID)
		if caseID == "" {
			continue
		}
		evidenceCaseIDs = append(evidenceCaseIDs, caseID)
	}
	evidenceCaseIDs = semanticMemoryUniqueSortedStrings(evidenceCaseIDs)

	sourceUpdatedAt := item.BuiltAt.UTC()
	if sourceUpdatedAt.IsZero() {
		sourceUpdatedAt = item.UpdatedAt.UTC()
	}
	if sourceUpdatedAt.IsZero() {
		sourceUpdatedAt = item.LastObservedAt.UTC()
	}

	scopeLabel := strings.ReplaceAll(blankToValue(item.ScopeType, "pattern"), "_", " ")
	recommendedUse := effectiveDealReviewLearnedPatternRecommendedUse(item)
	title := fmt.Sprintf(
		"Learned pattern %s %s %s",
		scopeLabel,
		strings.ToLower(strings.TrimSpace(item.PatternClass)),
		blankToValue(item.ValidationLabel, item.Status),
	)
	summary := fmt.Sprintf(
		"%s %s pattern on %s. Validation %s, recommended use %s. Avg PnL %+.2f%% across %d samples.",
		blankToValue(item.ScopeType, "pattern"),
		blankToValue(item.PatternClass, "edge"),
		blankToValue(strings.ToUpper(strings.TrimSpace(item.Side)), "unknown side"),
		blankToValue(item.ValidationLabel, "n/a"),
		blankToValue(recommendedUse, "review_hint"),
		item.AvgPnLPct,
		item.SampleCount,
	)
	body := strings.TrimSpace(fmt.Sprintf(`
Trader: %s
Pattern ID: %s
Stable source ID: %s
Scope: %s
Scope key: %s
Symbol: %s
Side: %s
Pattern class: %s
Status: %s
Validation label: %s
Recommended use: %s
Validation alert: %s
Pattern signature: %s
Regime signature: %s
Feature set: %s
Pattern order: %d
Feature count: %d
Samples: total=%d training=%d holdout=%d recent=%d
Support / contradict: %d / %d
Win/loss/flat: %d / %d / %d
PnL: net=%+.8f avg=%+.8f avg_pct=%+.2f%% expectancy=%+.8f
Lift: win_rate=%+.2f%% avg_pct=%+.2f%%
MFE/MAE: avg_mfe_pct=%+.2f%% avg_mae_pct=%+.2f%%
Give-back: rate=%.2f%% avg=%.2f%%
Scores: confidence=%.0f%% stability=%.0f%% drift=%.0f%% recency=%.0f%% composite=%.0f%%
False-positive score: %.0f%%
Reverse-risk score: %.0f%%
Evidence case IDs: %s
Summary: %s
`,
		item.TraderID,
		item.ID,
		buildSemanticMemoryLearnedPatternSourceID(item),
		blankToValue(item.ScopeType, "n/a"),
		blankToValue(item.ScopeKey, "n/a"),
		blankToValue(strings.ToUpper(strings.TrimSpace(item.Symbol)), "n/a"),
		blankToValue(strings.ToUpper(strings.TrimSpace(item.Side)), "n/a"),
		blankToValue(item.PatternClass, "n/a"),
		blankToValue(item.Status, "n/a"),
		blankToValue(item.ValidationLabel, "n/a"),
		blankToValue(recommendedUse, "n/a"),
		blankToValue(item.ValidationAlert, "n/a"),
		blankToValue(item.PatternSignature, "n/a"),
		blankToValue(item.RegimeSignature, "n/a"),
		semanticMemoryJoinStringsForBody(item.FeatureSet, "none", 12),
		item.PatternOrder,
		item.FeatureCount,
		item.SampleCount,
		item.TrainingSampleCount,
		item.ValidationSampleCount,
		item.RecentSampleCount,
		item.SupportCount,
		item.ContradictCount,
		item.WinningDeals,
		item.LosingDeals,
		item.FlatDeals,
		item.NetPnL,
		item.AvgPnL,
		item.AvgPnLPct,
		item.Expectancy,
		item.LiftWinRate*100,
		item.LiftAvgPnLPct,
		item.AvgMFEPct,
		item.AvgMAEPct,
		item.GiveBackRate*100,
		item.AvgGiveBackPct,
		item.ConfidenceScore*100,
		item.StabilityScore*100,
		item.DriftScore*100,
		item.RecencyWeight*100,
		item.CompositeScore*100,
		item.FalsePositiveScore*100,
		item.ReverseRiskScore*100,
		semanticMemoryJoinStringsForBody(evidenceCaseIDs, "none", 8),
		blankToValue(item.Summary, "n/a"),
	))
	metadata := semanticMemoryMarshalJSON(map[string]any{
		"pattern_id":              item.ID,
		"scope_type":              item.ScopeType,
		"scope_key":               item.ScopeKey,
		"symbol":                  strings.ToUpper(strings.TrimSpace(item.Symbol)),
		"side":                    strings.ToUpper(strings.TrimSpace(item.Side)),
		"status":                  item.Status,
		"validation_label":        item.ValidationLabel,
		"pattern_class":           item.PatternClass,
		"recommended_use":         recommendedUse,
		"pattern_signature":       item.PatternSignature,
		"regime_signature":        item.RegimeSignature,
		"feature_set":             item.FeatureSet,
		"feature_count":           item.FeatureCount,
		"sample_count":            item.SampleCount,
		"support_count":           item.SupportCount,
		"validation_sample_count": item.ValidationSampleCount,
		"recent_sample_count":     item.RecentSampleCount,
		"composite_score":         item.CompositeScore,
		"confidence_score":        item.ConfidenceScore,
		"drift_score":             item.DriftScore,
		"false_positive_score":    item.FalsePositiveScore,
		"reverse_risk_score":      item.ReverseRiskScore,
		"evidence_case_ids":       evidenceCaseIDs,
	})
	return &SemanticMemoryDocument{
		UserID:            item.UserID,
		TraderID:          item.TraderID,
		DocType:           SemanticMemoryDocTypeLearnedPattern,
		SourceID:          buildSemanticMemoryLearnedPatternSourceID(item),
		SourceUpdatedAt:   sourceUpdatedAt,
		Title:             title,
		Summary:           summary,
		Body:              body,
		MetadataJSON:      metadata,
		EmbeddingProvider: SemanticMemoryDefaultEmbeddingProvider,
		EmbeddingModel:    SemanticMemoryDefaultEmbeddingModel,
		EmbeddingStatus:   SemanticMemoryEmbeddingStatusPending,
	}
}

func buildSemanticMemorySymbolBehaviorPriorSourceID(item *DealReviewSymbolBehaviorPrior) string {
	if item == nil {
		return ""
	}
	parts := []string{
		strings.ToUpper(strings.TrimSpace(item.Symbol)),
		strings.ToUpper(strings.TrimSpace(item.Side)),
		normalizeDealReviewSymbolPriorDimension(item.OpenSelectionBucket),
		normalizeDealReviewSymbolPriorDimension(item.OpenTrendRegime),
		normalizeDealReviewSymbolPriorDimension(item.OpenVolatilityRegime),
		normalizeDealReviewSymbolPriorDimension(item.OpenOIRegime),
	}
	return strings.Join(parts, "::")
}

func buildSemanticMemoryLearnedPatternSourceID(item *DealReviewLearnedPattern) string {
	if item == nil {
		return ""
	}
	parts := []string{
		strings.TrimSpace(item.ScopeType),
		strings.TrimSpace(item.ScopeKey),
		strings.ToUpper(strings.TrimSpace(item.Symbol)),
		strings.ToUpper(strings.TrimSpace(item.Side)),
		strings.TrimSpace(item.PatternClass),
		strings.TrimSpace(item.PatternSignature),
	}
	return strings.Join(parts, "::")
}

func normalizeSemanticMemoryDocType(value string) string {
	switch strings.TrimSpace(value) {
	case SemanticMemoryDocTypeAutonomousOptimizerRun:
		return SemanticMemoryDocTypeAutonomousOptimizerRun
	case SemanticMemoryDocTypeOptimizerBacklogItem:
		return SemanticMemoryDocTypeOptimizerBacklogItem
	case SemanticMemoryDocTypeStrategyVersion:
		return SemanticMemoryDocTypeStrategyVersion
	case SemanticMemoryDocTypeDecisionRecordSummary:
		return SemanticMemoryDocTypeDecisionRecordSummary
	case SemanticMemoryDocTypeSymbolBehaviorPrior:
		return SemanticMemoryDocTypeSymbolBehaviorPrior
	case SemanticMemoryDocTypeLearnedPattern:
		return SemanticMemoryDocTypeLearnedPattern
	default:
		return SemanticMemoryDocTypeDealReviewCase
	}
}

func normalizeSemanticMemoryEmbeddingStatus(value string) string {
	switch strings.TrimSpace(value) {
	case SemanticMemoryEmbeddingStatusEmbedded:
		return SemanticMemoryEmbeddingStatusEmbedded
	case SemanticMemoryEmbeddingStatusFailed:
		return SemanticMemoryEmbeddingStatusFailed
	case SemanticMemoryEmbeddingStatusSkipped:
		return SemanticMemoryEmbeddingStatusSkipped
	default:
		return SemanticMemoryEmbeddingStatusPending
	}
}

func semanticMemoryApproxTokens(value string) int {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	charCount := utf8.RuneCountInString(value)
	if charCount <= 0 {
		return 0
	}
	tokens := charCount / 4
	if charCount%4 != 0 {
		tokens++
	}
	if tokens < 1 {
		return 1
	}
	return tokens
}

func semanticMemoryHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func semanticMemoryNormalizeJSON(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(trimmed)); err != nil {
		return fallback
	}
	return compact.String()
}

func semanticMemoryCompactJSON(value string, limit int) string {
	normalized := semanticMemoryNormalizeJSON(value, "{}")
	if limit <= 0 || len(normalized) <= limit {
		return normalized
	}
	if limit < 4 {
		return normalized[:limit]
	}
	return normalized[:limit-3] + "..."
}

func semanticMemoryMarshalJSON(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func semanticMemoryParseJSONObject(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" {
		return map[string]any{}
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return map[string]any{}
	}
	return parsed
}

func semanticMemoryNestedJSONObject(raw map[string]any, path ...string) map[string]any {
	current := raw
	for _, segment := range path {
		if len(current) == 0 {
			return map[string]any{}
		}
		value, ok := current[segment]
		if !ok || value == nil {
			return map[string]any{}
		}
		next, ok := value.(map[string]any)
		if ok {
			current = next
			continue
		}
		if text, ok := value.(string); ok {
			current = semanticMemoryParseJSONObject(text)
			continue
		}
		return map[string]any{}
	}
	return current
}

func semanticMemoryMetadataString(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func semanticMemoryMetadataInt(raw map[string]any, key string) int {
	if raw == nil {
		return 0
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
		if parsed, err := typed.Float64(); err == nil {
			return int(parsed)
		}
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return 0
		}
		if parsed, err := strconv.Atoi(typed); err == nil {
			return parsed
		}
	}
	return 0
}

func semanticMemoryMetadataFloat(raw map[string]any, key string) float64 {
	if raw == nil {
		return 0
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float32:
		return float64(typed)
	case float64:
		return typed
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed
		}
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return 0
		}
		if parsed, err := strconv.ParseFloat(typed, 64); err == nil {
			return parsed
		}
	}
	return 0
}

func semanticMemoryMetadataStringSlice(raw map[string]any, key string) []string {
	if raw == nil {
		return nil
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			result = append(result, item)
		}
		return result
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text == "" {
				continue
			}
			result = append(result, text)
		}
		return result
	default:
		return nil
	}
}

func semanticMemoryStringSliceValue(raw any) []string {
	switch typed := raw.(type) {
	case nil:
		return nil
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			result = append(result, item)
		}
		return semanticMemoryUniqueSortedStrings(result)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text == "" || text == "<nil>" {
				continue
			}
			result = append(result, text)
		}
		return semanticMemoryUniqueSortedStrings(result)
	case string:
		if items := semanticMemoryNormalizeJSONArray(typed); len(items) > 0 {
			return semanticMemoryUniqueSortedStrings(items)
		}
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return nil
		}
		return []string{typed}
	default:
		text := strings.TrimSpace(fmt.Sprintf("%v", typed))
		if text == "" || text == "<nil>" {
			return nil
		}
		return []string{text}
	}
}

func semanticMemoryUniqueSortedStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]string, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = item
	}
	if len(seen) == 0 {
		return nil
	}
	result := make([]string, 0, len(seen))
	for _, item := range seen {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func semanticMemoryFlattenJSONPaths(value any, prefix string) []string {
	paths := semanticMemoryFlattenJSONPathsInto(nil, value, strings.TrimSpace(prefix))
	return semanticMemoryUniqueSortedStrings(paths)
}

func semanticMemoryFlattenJSONPathsInto(paths []string, value any, prefix string) []string {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			if prefix != "" {
				paths = append(paths, prefix)
			}
			return paths
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			paths = semanticMemoryFlattenJSONPathsInto(paths, typed[key], next)
		}
		return paths
	case []any:
		if prefix == "" {
			return paths
		}
		if len(typed) == 0 {
			return append(paths, prefix+"[]")
		}
		hasObject := false
		for _, item := range typed {
			if _, ok := item.(map[string]any); ok {
				hasObject = true
			}
			paths = semanticMemoryFlattenJSONPathsInto(paths, item, prefix+"[]")
		}
		if !hasObject {
			paths = append(paths, prefix+"[]")
		}
		return paths
	default:
		if prefix != "" {
			paths = append(paths, prefix)
		}
		return paths
	}
}

func semanticMemoryJoinStringsForBody(items []string, fallback string, limit int) string {
	items = semanticMemoryUniqueSortedStrings(items)
	if len(items) == 0 {
		return fallback
	}
	if limit > 0 && len(items) > limit {
		clipped := append([]string{}, items[:limit]...)
		clipped = append(clipped, fmt.Sprintf("+%d more", len(items)-limit))
		return strings.Join(clipped, "; ")
	}
	return strings.Join(items, "; ")
}

func semanticMemoryNormalizeJSONArray(value string) []string {
	var raw []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &raw); err != nil {
		return nil
	}
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		items = append(items, item)
	}
	return items
}

func blankToValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func semanticMemoryShouldIndexDecisionRecordSummary(record *DecisionRecord) bool {
	if record == nil || record.CandidateMetaVer <= 0 {
		return false
	}
	if len(record.Decisions) == 0 {
		return false
	}
	meaningful := false
	for _, action := range record.Decisions {
		actionType := strings.TrimSpace(action.Action)
		if actionType == "" {
			continue
		}
		meaningful = true
		if strings.HasPrefix(actionType, "open_") || strings.HasPrefix(actionType, "close_") || actionType == "hold" || actionType == "wait" {
			return true
		}
	}
	return meaningful
}
