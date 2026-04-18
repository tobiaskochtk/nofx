package store

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type SemanticMemoryQueryResult struct {
	Query             string                     `json:"query"`
	EmbeddingProvider string                     `json:"embedding_provider"`
	EmbeddingModel    string                     `json:"embedding_model"`
	Items             []*SemanticMemorySearchHit `json:"items"`
}

type SemanticMemoryCorpusDocTypeStatus struct {
	DocType                       string    `json:"doc_type"`
	TotalDocuments                int       `json:"total_documents"`
	EmbeddedDocuments             int       `json:"embedded_documents"`
	PendingDocuments              int       `json:"pending_documents"`
	FailedDocuments               int       `json:"failed_documents"`
	SkippedDocuments              int       `json:"skipped_documents"`
	TotalTokenEstimate            int       `json:"total_token_estimate"`
	EmbeddedTokenEstimate         int       `json:"embedded_token_estimate"`
	PendingTokenEstimate          int       `json:"pending_token_estimate"`
	FailedTokenEstimate           int       `json:"failed_token_estimate"`
	EstimatedTotalCostUSD         float64   `json:"estimated_total_cost_usd"`
	EstimatedPendingCostUSD       float64   `json:"estimated_pending_cost_usd"`
	EstimatedEmbeddedCostUSD      float64   `json:"estimated_embedded_cost_usd"`
	EstimatedFailedReembedCostUSD float64   `json:"estimated_failed_reembed_cost_usd"`
	LastUpdatedAt                 time.Time `json:"last_updated_at"`
	LastEmbeddedAt                time.Time `json:"last_embedded_at"`
	DominantModel                 string    `json:"dominant_model,omitempty"`
	DominantProvider              string    `json:"dominant_provider,omitempty"`
}

type SemanticMemoryEmbeddingUsageSummary struct {
	RecentRuns              int       `json:"recent_runs"`
	RecentEmbeddedDocuments int       `json:"recent_embedded_documents"`
	RecentFailedDocuments   int       `json:"recent_failed_documents"`
	RecentPromptTokens      int       `json:"recent_prompt_tokens"`
	RecentEstimatedCostUSD  float64   `json:"recent_estimated_cost_usd"`
	LastCompletedAt         time.Time `json:"last_completed_at"`
	LastEmbeddingModel      string    `json:"last_embedding_model,omitempty"`
	LastEmbeddingProvider   string    `json:"last_embedding_provider,omitempty"`
}

type SemanticMemoryFailureSummary struct {
	RecentFailedRuns   int       `json:"recent_failed_runs"`
	LastFailedAt       time.Time `json:"last_failed_at"`
	LastFailureScope   string    `json:"last_failure_scope,omitempty"`
	LastFailureMessage string    `json:"last_failure_message,omitempty"`
}

type SemanticMemoryCostEstimate struct {
	EmbeddingModel                string  `json:"embedding_model"`
	PricePer1MTokensUSD           float64 `json:"price_per_1m_tokens_usd"`
	TotalTokenEstimate            int     `json:"total_token_estimate"`
	EmbeddedTokenEstimate         int     `json:"embedded_token_estimate"`
	PendingTokenEstimate          int     `json:"pending_token_estimate"`
	FailedTokenEstimate           int     `json:"failed_token_estimate"`
	EstimatedTotalCostUSD         float64 `json:"estimated_total_cost_usd"`
	EstimatedEmbeddedCostUSD      float64 `json:"estimated_embedded_cost_usd"`
	EstimatedPendingCostUSD       float64 `json:"estimated_pending_cost_usd"`
	EstimatedFailedReembedCostUSD float64 `json:"estimated_failed_reembed_cost_usd"`
}

type SemanticMemoryCorpusStatus struct {
	UserID                string                              `json:"user_id"`
	TraderID              string                              `json:"trader_id"`
	VectorAvailable       bool                                `json:"vector_available"`
	VectorExtensionName   string                              `json:"vector_extension_name,omitempty"`
	TotalDocuments        int                                 `json:"total_documents"`
	EmbeddedDocuments     int                                 `json:"embedded_documents"`
	PendingDocuments      int                                 `json:"pending_documents"`
	FailedDocuments       int                                 `json:"failed_documents"`
	SkippedDocuments      int                                 `json:"skipped_documents"`
	TotalTokenEstimate    int                                 `json:"total_token_estimate"`
	EmbeddedTokenEstimate int                                 `json:"embedded_token_estimate"`
	PendingTokenEstimate  int                                 `json:"pending_token_estimate"`
	FailedTokenEstimate   int                                 `json:"failed_token_estimate"`
	CostEstimate          SemanticMemoryCostEstimate          `json:"cost_estimate"`
	EmbeddingUsage        SemanticMemoryEmbeddingUsageSummary `json:"embedding_usage"`
	FailureSummary        SemanticMemoryFailureSummary        `json:"failure_summary"`
	DocTypes              []SemanticMemoryCorpusDocTypeStatus `json:"doc_types"`
	RecentSyncRuns        []*SemanticMemorySyncRun            `json:"recent_sync_runs,omitempty"`
}

type semanticMemoryCorpusStatusRow struct {
	DocType               string `gorm:"column:doc_type"`
	TotalDocuments        int    `gorm:"column:total_documents"`
	EmbeddedDocuments     int    `gorm:"column:embedded_documents"`
	PendingDocuments      int    `gorm:"column:pending_documents"`
	FailedDocuments       int    `gorm:"column:failed_documents"`
	SkippedDocuments      int    `gorm:"column:skipped_documents"`
	TotalTokenEstimate    int    `gorm:"column:total_token_estimate"`
	EmbeddedTokenEstimate int    `gorm:"column:embedded_token_estimate"`
	PendingTokenEstimate  int    `gorm:"column:pending_token_estimate"`
	FailedTokenEstimate   int    `gorm:"column:failed_token_estimate"`
	LastUpdatedAtRaw      string `gorm:"column:last_updated_at"`
	LastEmbeddedAtRaw     string `gorm:"column:last_embedded_at"`
	DominantModel         string `gorm:"column:dominant_model"`
	DominantProvider      string `gorm:"column:dominant_provider"`
}

func (s *SemanticMemoryStore) SearchDocumentsByQuery(userID, traderID, queryText string, docTypes []string, filter SemanticMemorySearchFilter, limit int, modelCfg *AIModel, modelName string) (*SemanticMemoryQueryResult, error) {
	if strings.TrimSpace(queryText) == "" {
		return nil, fmt.Errorf("query text is required")
	}
	if modelCfg == nil {
		return nil, fmt.Errorf("embedding model config cannot be nil")
	}
	if strings.TrimSpace(modelCfg.Provider) != "openai" {
		return nil, fmt.Errorf("semantic memory search currently requires an enabled OpenAI model config")
	}
	if s.db == nil || s.db.Dialector.Name() != "postgres" {
		return nil, fmt.Errorf("semantic similarity search requires PostgreSQL with pgvector")
	}
	vectorAvailable, _, err := s.VectorExtensionStatus()
	if err != nil {
		return nil, err
	}
	if !vectorAvailable {
		return nil, fmt.Errorf("pgvector extension is not available in the current PostgreSQL runtime")
	}

	modelName = normalizeSemanticMemoryEmbeddingModel(modelName)
	if modelName != SemanticMemoryDefaultEmbeddingModel {
		return nil, fmt.Errorf("semantic memory search currently supports only %s", SemanticMemoryDefaultEmbeddingModel)
	}
	if limit <= 0 || limit > 50 {
		limit = 12
	}

	vector, err := s.embedSemanticMemoryQuery(string(modelCfg.APIKey), modelCfg.CustomAPIURL, userID, modelName, queryText)
	if err != nil {
		return nil, err
	}

	normalizedDocTypes := semanticMemoryNormalizeDocTypes(docTypes)
	whereClauses := []string{
		"d.user_id = ?",
		"d.embedding_status = ?",
		"v.dimensions = ?",
	}
	args := []any{
		userID,
		SemanticMemoryEmbeddingStatusEmbedded,
		SemanticMemoryDefaultEmbeddingDims,
	}
	if strings.TrimSpace(traderID) != "" {
		whereClauses = append(whereClauses, "d.trader_id = ?")
		args = append(args, strings.TrimSpace(traderID))
	}
	if len(normalizedDocTypes) > 0 {
		whereClauses = append(whereClauses, "d.doc_type IN ?")
		args = append(args, normalizedDocTypes)
	}
	filterClauses, filterArgs := semanticMemorySearchFilterClauses("d", filter)
	whereClauses = append(whereClauses, filterClauses...)
	args = append(args, filterArgs...)
	vectorLiteral := semanticMemoryVectorLiteral(vector)

	query := fmt.Sprintf(`
		SELECT
			d.id,
			d.user_id,
			d.trader_id,
			d.doc_type,
			d.source_id,
			d.source_updated_at,
			d.title,
			d.summary,
			d.body,
			d.metadata_json,
			d.content_hash,
			d.token_estimate,
			d.embedding_provider,
			d.embedding_model,
			d.embedding_dimensions,
			d.embedding_status,
			d.last_embedding_error,
			d.last_built_at,
			d.last_embedded_at,
			d.created_at,
			d.updated_at,
			COALESCE(1 - (v.embedding <=> CAST(? AS vector)), 0) AS similarity_score,
			COALESCE((v.embedding <=> CAST(? AS vector)), 0) AS distance
		FROM semantic_memory_documents d
		JOIN semantic_memory_vectors v ON v.document_id = d.id
		WHERE %s
		ORDER BY v.embedding <=> CAST(? AS vector) ASC, d.updated_at DESC
		LIMIT ?
	`, strings.Join(whereClauses, " AND "))

	queryArgs := make([]any, 0, len(args)+4)
	queryArgs = append(queryArgs, vectorLiteral, vectorLiteral)
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, vectorLiteral, limit)

	var rows []semanticMemorySearchRow
	if err := s.db.Raw(query, queryArgs...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := &SemanticMemoryQueryResult{
		Query:             strings.TrimSpace(queryText),
		EmbeddingProvider: strings.TrimSpace(modelCfg.Provider),
		EmbeddingModel:    modelName,
		Items:             make([]*SemanticMemorySearchHit, 0, len(rows)),
	}
	for idx := range rows {
		doc := rows[idx].toDocument()
		result.Items = append(result.Items, &SemanticMemorySearchHit{
			Document:        doc,
			SimilarityScore: rows[idx].SimilarityScore,
			Distance:        rows[idx].Distance,
			SourceLink:      semanticMemorySourceLink(doc),
		})
	}
	return result, nil
}

func (s *SemanticMemoryStore) GetCorpusStatus(userID, traderID string, syncRunLimit int) (*SemanticMemoryCorpusStatus, error) {
	status := &SemanticMemoryCorpusStatus{
		UserID:   strings.TrimSpace(userID),
		TraderID: strings.TrimSpace(traderID),
		DocTypes: []SemanticMemoryCorpusDocTypeStatus{},
	}
	available, extName, err := s.VectorExtensionStatus()
	if err != nil {
		return nil, err
	}
	status.VectorAvailable = available
	status.VectorExtensionName = extName

	rawFilter := "user_id = ?"
	rawArgs := []any{userID}
	if status.TraderID != "" {
		rawFilter += " AND trader_id = ?"
		rawArgs = append(rawArgs, status.TraderID)
	}

	var rows []semanticMemoryCorpusStatusRow
	rawSQL := fmt.Sprintf(`
		SELECT
			doc_type,
			COUNT(*) AS total_documents,
			SUM(CASE WHEN embedding_status = '%s' THEN 1 ELSE 0 END) AS embedded_documents,
			SUM(CASE WHEN embedding_status = '%s' THEN 1 ELSE 0 END) AS pending_documents,
			SUM(CASE WHEN embedding_status = '%s' THEN 1 ELSE 0 END) AS failed_documents,
			SUM(CASE WHEN embedding_status = '%s' THEN 1 ELSE 0 END) AS skipped_documents,
			SUM(COALESCE(token_estimate, 0)) AS total_token_estimate,
			SUM(CASE WHEN embedding_status = '%s' THEN COALESCE(token_estimate, 0) ELSE 0 END) AS embedded_token_estimate,
			SUM(CASE WHEN embedding_status = '%s' THEN COALESCE(token_estimate, 0) ELSE 0 END) AS pending_token_estimate,
			SUM(CASE WHEN embedding_status = '%s' THEN COALESCE(token_estimate, 0) ELSE 0 END) AS failed_token_estimate,
			CAST(MAX(updated_at) AS TEXT) AS last_updated_at,
			CAST(MAX(last_embedded_at) AS TEXT) AS last_embedded_at,
			MAX(COALESCE(embedding_model, '')) AS dominant_model,
			MAX(COALESCE(embedding_provider, '')) AS dominant_provider
		FROM semantic_memory_documents
		WHERE %s
		GROUP BY doc_type
		ORDER BY doc_type ASC
	`, SemanticMemoryEmbeddingStatusEmbedded, SemanticMemoryEmbeddingStatusPending, SemanticMemoryEmbeddingStatusFailed, SemanticMemoryEmbeddingStatusSkipped, SemanticMemoryEmbeddingStatusEmbedded, SemanticMemoryEmbeddingStatusPending, SemanticMemoryEmbeddingStatusFailed, rawFilter)
	if err := s.db.Raw(rawSQL, rawArgs...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		modelName := blankToValue(row.DominantModel, SemanticMemoryDefaultEmbeddingModel)
		status.DocTypes = append(status.DocTypes, SemanticMemoryCorpusDocTypeStatus{
			DocType:                       row.DocType,
			TotalDocuments:                row.TotalDocuments,
			EmbeddedDocuments:             row.EmbeddedDocuments,
			PendingDocuments:              row.PendingDocuments,
			FailedDocuments:               row.FailedDocuments,
			SkippedDocuments:              row.SkippedDocuments,
			TotalTokenEstimate:            row.TotalTokenEstimate,
			EmbeddedTokenEstimate:         row.EmbeddedTokenEstimate,
			PendingTokenEstimate:          row.PendingTokenEstimate,
			FailedTokenEstimate:           row.FailedTokenEstimate,
			EstimatedTotalCostUSD:         semanticMemoryEstimatedCostUSD(modelName, row.TotalTokenEstimate),
			EstimatedPendingCostUSD:       semanticMemoryEstimatedCostUSD(modelName, row.PendingTokenEstimate),
			EstimatedEmbeddedCostUSD:      semanticMemoryEstimatedCostUSD(modelName, row.EmbeddedTokenEstimate),
			EstimatedFailedReembedCostUSD: semanticMemoryEstimatedCostUSD(modelName, row.FailedTokenEstimate),
			LastUpdatedAt:                 parseSemanticMemoryStatusTime(row.LastUpdatedAtRaw),
			LastEmbeddedAt:                parseSemanticMemoryStatusTime(row.LastEmbeddedAtRaw),
			DominantModel:                 row.DominantModel,
			DominantProvider:              row.DominantProvider,
		})
		status.TotalDocuments += row.TotalDocuments
		status.EmbeddedDocuments += row.EmbeddedDocuments
		status.PendingDocuments += row.PendingDocuments
		status.FailedDocuments += row.FailedDocuments
		status.SkippedDocuments += row.SkippedDocuments
		status.TotalTokenEstimate += row.TotalTokenEstimate
		status.EmbeddedTokenEstimate += row.EmbeddedTokenEstimate
		status.PendingTokenEstimate += row.PendingTokenEstimate
		status.FailedTokenEstimate += row.FailedTokenEstimate
	}
	if syncRunLimit <= 0 || syncRunLimit > 20 {
		syncRunLimit = 8
	}
	runs, err := s.ListSyncRuns(userID, traderID, syncRunLimit)
	if err != nil {
		return nil, err
	}
	status.RecentSyncRuns = runs
	costModel := semanticMemoryCorpusCostModel(status.DocTypes, runs)
	status.CostEstimate = SemanticMemoryCostEstimate{
		EmbeddingModel:                costModel,
		PricePer1MTokensUSD:           semanticMemoryEmbeddingPricePer1MTokensUSD(costModel),
		TotalTokenEstimate:            status.TotalTokenEstimate,
		EmbeddedTokenEstimate:         status.EmbeddedTokenEstimate,
		PendingTokenEstimate:          status.PendingTokenEstimate,
		FailedTokenEstimate:           status.FailedTokenEstimate,
		EstimatedTotalCostUSD:         semanticMemoryEstimatedCostUSD(costModel, status.TotalTokenEstimate),
		EstimatedEmbeddedCostUSD:      semanticMemoryEstimatedCostUSD(costModel, status.EmbeddedTokenEstimate),
		EstimatedPendingCostUSD:       semanticMemoryEstimatedCostUSD(costModel, status.PendingTokenEstimate),
		EstimatedFailedReembedCostUSD: semanticMemoryEstimatedCostUSD(costModel, status.FailedTokenEstimate),
	}
	status.EmbeddingUsage = summarizeSemanticMemoryEmbeddingUsage(runs)
	status.FailureSummary = summarizeSemanticMemoryFailures(runs)
	return status, nil
}

func semanticMemoryCorpusCostModel(docTypes []SemanticMemoryCorpusDocTypeStatus, runs []*SemanticMemorySyncRun) string {
	for _, run := range runs {
		if run == nil {
			continue
		}
		if model := strings.TrimSpace(run.EmbeddingModel); model != "" {
			return model
		}
	}
	for _, docType := range docTypes {
		if model := strings.TrimSpace(docType.DominantModel); model != "" {
			return model
		}
	}
	return SemanticMemoryDefaultEmbeddingModel
}

func summarizeSemanticMemoryEmbeddingUsage(runs []*SemanticMemorySyncRun) SemanticMemoryEmbeddingUsageSummary {
	summary := SemanticMemoryEmbeddingUsageSummary{}
	for _, run := range runs {
		if run == nil || run.Scope != SemanticMemorySyncScopeEmbedPending {
			continue
		}
		summary.RecentRuns++
		summary.RecentEmbeddedDocuments += run.UpdatedDocuments
		summary.RecentFailedDocuments += run.FailedDocuments
		summary.RecentPromptTokens += run.PromptTokens
		summary.RecentEstimatedCostUSD += run.EstimatedCostUSD
		if summary.LastCompletedAt.IsZero() || run.CompletedAt.After(summary.LastCompletedAt) {
			summary.LastCompletedAt = run.CompletedAt
			summary.LastEmbeddingModel = run.EmbeddingModel
			summary.LastEmbeddingProvider = run.EmbeddingProvider
		}
	}
	return summary
}

func summarizeSemanticMemoryFailures(runs []*SemanticMemorySyncRun) SemanticMemoryFailureSummary {
	summary := SemanticMemoryFailureSummary{}
	for _, run := range runs {
		if run == nil || run.Status != SemanticMemorySyncStatusFailed {
			continue
		}
		summary.RecentFailedRuns++
		if summary.LastFailedAt.IsZero() || run.UpdatedAt.After(summary.LastFailedAt) {
			summary.LastFailedAt = run.UpdatedAt
			summary.LastFailureScope = run.Scope
			summary.LastFailureMessage = blankToValue(run.ErrorMessage, run.Summary)
		}
	}
	return summary
}

func (s *SemanticMemoryStore) embedSemanticMemoryQuery(apiKey, customAPIURL, userID, modelName, queryText string) ([]float64, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	embeddings, _, err := callOpenAIEmbeddings(
		client,
		resolveOpenAIEmbeddingsURL(customAPIURL),
		apiKey,
		userID,
		modelName,
		[]string{strings.TrimSpace(queryText)},
	)
	if err != nil {
		return nil, err
	}
	if len(embeddings) != 1 {
		return nil, fmt.Errorf("query embedding response count mismatch: got %d", len(embeddings))
	}
	if len(embeddings[0]) != SemanticMemoryDefaultEmbeddingDims {
		return nil, fmt.Errorf("unexpected query embedding dimension %d", len(embeddings[0]))
	}
	return embeddings[0], nil
}

func semanticMemoryNormalizeDocTypes(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, item := range values {
		normalized := normalizeSemanticMemoryDocType(item)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func parseSemanticMemoryStatusTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}
