package store

import (
	"fmt"
	"strings"
	"time"
)

type SemanticMemorySearchFilter struct {
	Symbol                string     `json:"symbol,omitempty"`
	Side                  string     `json:"side,omitempty"`
	Outcome               string     `json:"outcome,omitempty"`
	CloseReason           string     `json:"close_reason,omitempty"`
	ExitOrigin            string     `json:"exit_origin,omitempty"`
	ExitReasonQuality     string     `json:"exit_reason_quality,omitempty"`
	Status                string     `json:"status,omitempty"`
	Trigger               string     `json:"trigger,omitempty"`
	Category              string     `json:"category,omitempty"`
	OpenTrendRegime       string     `json:"open_trend_regime,omitempty"`
	OpenVolatilityRegime  string     `json:"open_volatility_regime,omitempty"`
	OpenOIRegime          string     `json:"open_oi_regime,omitempty"`
	CloseTrendRegime      string     `json:"close_trend_regime,omitempty"`
	CloseVolatilityRegime string     `json:"close_volatility_regime,omitempty"`
	CloseOIRegime         string     `json:"close_oi_regime,omitempty"`
	FromTime              *time.Time `json:"from_time,omitempty"`
	ToTime                *time.Time `json:"to_time,omitempty"`
}

type SemanticMemorySearchHit struct {
	Document        *SemanticMemoryDocument `json:"document"`
	SimilarityScore float64                 `json:"similarity_score"`
	Distance        float64                 `json:"distance"`
	SourceLink      string                  `json:"source_link,omitempty"`
}

type SemanticMemorySimilarityResult struct {
	QueryDocument *SemanticMemoryDocument    `json:"query_document"`
	Items         []*SemanticMemorySearchHit `json:"items"`
}

type semanticMemorySearchRow struct {
	ID                  string    `gorm:"column:id"`
	UserID              string    `gorm:"column:user_id"`
	TraderID            string    `gorm:"column:trader_id"`
	DocType             string    `gorm:"column:doc_type"`
	SourceID            string    `gorm:"column:source_id"`
	SourceUpdatedAt     time.Time `gorm:"column:source_updated_at"`
	Title               string    `gorm:"column:title"`
	Summary             string    `gorm:"column:summary"`
	Body                string    `gorm:"column:body"`
	MetadataJSON        string    `gorm:"column:metadata_json"`
	ContentHash         string    `gorm:"column:content_hash"`
	TokenEstimate       int       `gorm:"column:token_estimate"`
	EmbeddingProvider   string    `gorm:"column:embedding_provider"`
	EmbeddingModel      string    `gorm:"column:embedding_model"`
	EmbeddingDimensions int       `gorm:"column:embedding_dimensions"`
	EmbeddingStatus     string    `gorm:"column:embedding_status"`
	LastEmbeddingError  string    `gorm:"column:last_embedding_error"`
	LastBuiltAt         time.Time `gorm:"column:last_built_at"`
	LastEmbeddedAt      time.Time `gorm:"column:last_embedded_at"`
	CreatedAt           time.Time `gorm:"column:created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at"`
	SimilarityScore     float64   `gorm:"column:similarity_score"`
	Distance            float64   `gorm:"column:distance"`
}

func (s *SemanticMemoryStore) GetDocumentBySource(userID, traderID, docType, sourceID string) (*SemanticMemoryDocument, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	docType = normalizeSemanticMemoryDocType(docType)
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return nil, fmt.Errorf("source_id is required")
	}
	query := s.db.Model(&SemanticMemoryDocument{}).
		Where("user_id = ? AND doc_type = ? AND source_id = ?", userID, docType, sourceID)
	if strings.TrimSpace(traderID) != "" {
		query = query.Where("trader_id = ?", strings.TrimSpace(traderID))
	}
	var doc SemanticMemoryDocument
	if err := query.First(&doc).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *SemanticMemoryStore) FindSimilarDocumentsBySource(userID, traderID, docType, sourceID string, filter SemanticMemorySearchFilter, limit int) (*SemanticMemorySimilarityResult, error) {
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

	doc, err := s.GetDocumentBySource(userID, traderID, docType, sourceID)
	if err != nil {
		return nil, err
	}
	if normalizeSemanticMemoryEmbeddingStatus(doc.EmbeddingStatus) != SemanticMemoryEmbeddingStatusEmbedded {
		return nil, fmt.Errorf("semantic memory source document is not embedded yet")
	}
	if limit <= 0 || limit > 50 {
		limit = 8
	}

	whereClauses := []string{
		"d.user_id = ?",
		"d.trader_id = ?",
		"d.doc_type = ?",
		"d.id <> ?",
		"d.embedding_status = ?",
		"v.dimensions = src.dimensions",
	}
	args := []any{
		userID,
		strings.TrimSpace(traderID),
		normalizeSemanticMemoryDocType(docType),
		doc.ID,
		SemanticMemoryEmbeddingStatusEmbedded,
	}
	filterClauses, filterArgs := semanticMemorySearchFilterClauses("d", filter)
	whereClauses = append(whereClauses, filterClauses...)
	args = append(args, filterArgs...)
	args = append([]any{doc.ID}, args...)
	args = append(args, limit)

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
			COALESCE(1 - (v.embedding <=> src.embedding), 0) AS similarity_score,
			COALESCE((v.embedding <=> src.embedding), 0) AS distance
		FROM semantic_memory_documents d
		JOIN semantic_memory_vectors v ON v.document_id = d.id
		JOIN semantic_memory_vectors src ON src.document_id = ?
		WHERE %s
		ORDER BY v.embedding <=> src.embedding ASC, d.updated_at DESC
		LIMIT ?
	`, strings.Join(whereClauses, " AND "))

	var rows []semanticMemorySearchRow
	if err := s.db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := &SemanticMemorySimilarityResult{
		QueryDocument: doc,
		Items:         make([]*SemanticMemorySearchHit, 0, len(rows)),
	}
	for idx := range rows {
		hitDoc := rows[idx].toDocument()
		result.Items = append(result.Items, &SemanticMemorySearchHit{
			Document:        hitDoc,
			SimilarityScore: rows[idx].SimilarityScore,
			Distance:        rows[idx].Distance,
			SourceLink:      semanticMemorySourceLink(hitDoc),
		})
	}
	return result, nil
}

func (r semanticMemorySearchRow) toDocument() *SemanticMemoryDocument {
	return &SemanticMemoryDocument{
		ID:                  r.ID,
		UserID:              r.UserID,
		TraderID:            r.TraderID,
		DocType:             r.DocType,
		SourceID:            r.SourceID,
		SourceUpdatedAt:     r.SourceUpdatedAt,
		Title:               r.Title,
		Summary:             r.Summary,
		Body:                r.Body,
		MetadataJSON:        r.MetadataJSON,
		ContentHash:         r.ContentHash,
		TokenEstimate:       r.TokenEstimate,
		EmbeddingProvider:   r.EmbeddingProvider,
		EmbeddingModel:      r.EmbeddingModel,
		EmbeddingDimensions: r.EmbeddingDimensions,
		EmbeddingStatus:     r.EmbeddingStatus,
		LastEmbeddingError:  r.LastEmbeddingError,
		LastBuiltAt:         r.LastBuiltAt,
		LastEmbeddedAt:      r.LastEmbeddedAt,
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
	}
}

func semanticMemorySearchFilterClauses(alias string, filter SemanticMemorySearchFilter) ([]string, []any) {
	clauses := make([]string, 0, 12)
	args := make([]any, 0, 12)
	appendTextFilter := func(key, value, transform string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		switch transform {
		case "upper":
			clauses = append(clauses, fmt.Sprintf("UPPER(COALESCE((%s.metadata_json::jsonb ->> '%s'), '')) = ?", alias, key))
			args = append(args, strings.ToUpper(value))
		default:
			clauses = append(clauses, fmt.Sprintf("LOWER(COALESCE((%s.metadata_json::jsonb ->> '%s'), '')) = ?", alias, key))
			args = append(args, strings.ToLower(value))
		}
	}

	appendTextFilter("symbol", filter.Symbol, "upper")
	appendTextFilter("side", filter.Side, "upper")
	appendTextFilter("outcome", filter.Outcome, "lower")
	appendTextFilter("close_reason", filter.CloseReason, "lower")
	appendTextFilter("exit_origin", filter.ExitOrigin, "lower")
	appendTextFilter("exit_reason_quality", filter.ExitReasonQuality, "lower")
	appendTextFilter("status", filter.Status, "lower")
	appendTextFilter("trigger", filter.Trigger, "lower")
	appendTextFilter("category", filter.Category, "lower")
	appendTextFilter("open_trend_regime", filter.OpenTrendRegime, "lower")
	appendTextFilter("open_volatility_regime", filter.OpenVolatilityRegime, "lower")
	appendTextFilter("open_oi_regime", filter.OpenOIRegime, "lower")
	appendTextFilter("close_trend_regime", filter.CloseTrendRegime, "lower")
	appendTextFilter("close_volatility_regime", filter.CloseVolatilityRegime, "lower")
	appendTextFilter("close_oi_regime", filter.CloseOIRegime, "lower")

	if filter.FromTime != nil && !filter.FromTime.IsZero() {
		clauses = append(clauses, fmt.Sprintf("%s.source_updated_at >= ?", alias))
		args = append(args, filter.FromTime.UTC())
	}
	if filter.ToTime != nil && !filter.ToTime.IsZero() {
		clauses = append(clauses, fmt.Sprintf("%s.source_updated_at <= ?", alias))
		args = append(args, filter.ToTime.UTC())
	}
	return clauses, args
}

func semanticMemorySourceLink(doc *SemanticMemoryDocument) string {
	if doc == nil {
		return ""
	}
	switch normalizeSemanticMemoryDocType(doc.DocType) {
	case SemanticMemoryDocTypeAutonomousOptimizerRun:
		return fmt.Sprintf("/optimizer?run_id=%s", strings.TrimSpace(doc.SourceID))
	case SemanticMemoryDocTypeOptimizerBacklogItem:
		return fmt.Sprintf("/optimizer?backlog_id=%s", strings.TrimSpace(doc.SourceID))
	case SemanticMemoryDocTypeStrategyVersion:
		return fmt.Sprintf("/deal-review?strategy_version_id=%s", strings.TrimSpace(doc.SourceID))
	default:
		return fmt.Sprintf("/deal-review?case_id=%s", strings.TrimSpace(doc.SourceID))
	}
}
