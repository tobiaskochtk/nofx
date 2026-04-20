package store

import (
	"fmt"
	"net/url"
	"sort"
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
	candidateLimit := limit
	if normalizeSemanticMemoryDocType(docType) == SemanticMemoryDocTypeAutonomousOptimizerRun {
		candidateLimit = limit * 4
		if candidateLimit < 20 {
			candidateLimit = 20
		}
		if candidateLimit > 50 {
			candidateLimit = 50
		}
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
	args = append(args, candidateLimit)

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

	type rankedHit struct {
		doc        *SemanticMemoryDocument
		score      float64
		distance   float64
		sourceLink string
	}
	result := &SemanticMemorySimilarityResult{
		QueryDocument: doc,
		Items:         make([]*SemanticMemorySearchHit, 0, minInt(limit, len(rows))),
	}
	ranked := make([]rankedHit, 0, len(rows))
	for idx := range rows {
		hitDoc := rows[idx].toDocument()
		score := semanticMemorySearchCompositeScore(doc, hitDoc, rows[idx].SimilarityScore)
		ranked = append(ranked, rankedHit{
			doc:        hitDoc,
			score:      score,
			distance:   rows[idx].Distance,
			sourceLink: semanticMemorySourceLink(hitDoc),
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].doc.UpdatedAt.After(ranked[j].doc.UpdatedAt)
		}
		return ranked[i].score > ranked[j].score
	})
	for idx := 0; idx < len(ranked) && idx < limit; idx++ {
		result.Items = append(result.Items, &SemanticMemorySearchHit{
			Document:        ranked[idx].doc,
			SimilarityScore: ranked[idx].score,
			Distance:        ranked[idx].distance,
			SourceLink:      ranked[idx].sourceLink,
		})
	}
	return result, nil
}

func semanticMemorySearchCompositeScore(queryDoc, hitDoc *SemanticMemoryDocument, vectorScore float64) float64 {
	score := clamp01(vectorScore)
	switch normalizeSemanticMemoryDocType(queryDoc.DocType) {
	case SemanticMemoryDocTypeAutonomousOptimizerRun:
		metaScore := semanticMemorySearchOptimizerRunMetadataScore(queryDoc, hitDoc)
		return clamp01(score*0.60 + metaScore*0.40)
	case SemanticMemoryDocTypeSymbolBehaviorPrior:
		metaScore := semanticMemorySearchSymbolBehaviorPriorMetadataScore(queryDoc, hitDoc)
		return clamp01(score*0.62 + metaScore*0.38)
	case SemanticMemoryDocTypeLearnedPattern:
		metaScore := semanticMemorySearchLearnedPatternMetadataScore(queryDoc, hitDoc)
		return clamp01(score*0.62 + metaScore*0.38)
	default:
		return score
	}
}

func semanticMemorySearchOptimizerRunMetadataScore(queryDoc, hitDoc *SemanticMemoryDocument) float64 {
	if queryDoc == nil || hitDoc == nil {
		return 0
	}
	queryMeta := semanticMemoryParseJSONObject(queryDoc.MetadataJSON)
	hitMeta := semanticMemoryParseJSONObject(hitDoc.MetadataJSON)
	score := 0.10 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "status")
	score += 0.06 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "trigger")
	score += 0.06 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "primary_model_name")
	score += 0.06 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "critic_model_name")
	score += 0.16 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "proposal_type")
	score += 0.14 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "critic_recommended_action")
	score += 0.12 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "config_validation_status")
	score += 0.12 * semanticMemoryBenchmarkArrayOverlap(queryMeta, hitMeta, "gate_reasons")
	score += 0.08 * semanticMemoryBenchmarkArrayOverlap(queryMeta, hitMeta, "deferred_reasons")
	score += 0.06 * semanticMemoryBenchmarkArrayOverlap(queryMeta, hitMeta, "config_patch_paths")
	score += 0.02 * semanticMemoryBenchmarkArrayOverlap(queryMeta, hitMeta, "prompt_patch_keys")
	score += 0.02 * semanticMemoryBenchmarkLexicalOverlap(queryDoc, hitDoc)
	return clamp01(score)
}

func semanticMemorySearchSymbolBehaviorPriorMetadataScore(queryDoc, hitDoc *SemanticMemoryDocument) float64 {
	if queryDoc == nil || hitDoc == nil {
		return 0
	}
	queryMeta := semanticMemoryParseJSONObject(queryDoc.MetadataJSON)
	hitMeta := semanticMemoryParseJSONObject(hitDoc.MetadataJSON)
	score := 0.18 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "symbol")
	score += 0.08 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "side")
	score += 0.08 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "status")
	score += 0.12 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "validation_label")
	score += 0.10 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "behavior_bias")
	score += 0.08 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "recommended_action")
	score += 0.08 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "open_selection_bucket")
	score += 0.07 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "open_trend_regime")
	score += 0.05 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "open_volatility_regime")
	score += 0.05 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "open_oi_regime")
	score += 0.09 * semanticMemoryBenchmarkArrayOverlap(queryMeta, hitMeta, "signal_tags")
	score += 0.02 * semanticMemoryBenchmarkLexicalOverlap(queryDoc, hitDoc)
	return clamp01(score)
}

func semanticMemorySearchLearnedPatternMetadataScore(queryDoc, hitDoc *SemanticMemoryDocument) float64 {
	if queryDoc == nil || hitDoc == nil {
		return 0
	}
	queryMeta := semanticMemoryParseJSONObject(queryDoc.MetadataJSON)
	hitMeta := semanticMemoryParseJSONObject(hitDoc.MetadataJSON)
	score := 0.14 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "symbol")
	score += 0.08 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "side")
	score += 0.08 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "scope_type")
	score += 0.12 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "pattern_class")
	score += 0.12 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "validation_label")
	score += 0.08 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "recommended_use")
	score += 0.12 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "pattern_signature")
	score += 0.06 * semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "regime_signature")
	score += 0.14 * semanticMemoryBenchmarkArrayOverlap(queryMeta, hitMeta, "feature_set")
	score += 0.04 * semanticMemoryBenchmarkArrayOverlap(queryMeta, hitMeta, "evidence_case_ids")
	score += 0.02 * semanticMemoryBenchmarkLexicalOverlap(queryDoc, hitDoc)
	return clamp01(score)
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
	case SemanticMemoryDocTypeDecisionRecordSummary:
		return "/dashboard"
	case SemanticMemoryDocTypeSymbolBehaviorPrior:
		meta := semanticMemoryParseJSONObject(doc.MetadataJSON)
		priorID := semanticMemoryMetadataString(meta, "prior_id")
		symbol := semanticMemoryMetadataString(meta, "symbol")
		side := semanticMemoryMetadataString(meta, "side")
		if priorID == "" {
			priorID = strings.TrimSpace(doc.SourceID)
		}
		path := fmt.Sprintf("/deal-review?symbol_prior_id=%s", priorID)
		if symbol != "" {
			path += "&symbol=" + strings.ToUpper(strings.TrimSpace(symbol))
		}
		if side != "" {
			path += "&side=" + strings.ToUpper(strings.TrimSpace(side))
		}
		return path
	case SemanticMemoryDocTypeLearnedPattern:
		meta := semanticMemoryParseJSONObject(doc.MetadataJSON)
		params := url.Values{}
		if patternID := semanticMemoryMetadataString(meta, "pattern_id"); patternID != "" {
			params.Set("pattern_id", patternID)
		}
		if scopeType := semanticMemoryMetadataString(meta, "scope_type"); scopeType != "" {
			params.Set("scope_type", scopeType)
		}
		if patternClass := semanticMemoryMetadataString(meta, "pattern_class"); patternClass != "" {
			params.Set("pattern_class", patternClass)
		}
		if validationLabel := semanticMemoryMetadataString(meta, "validation_label"); validationLabel != "" {
			params.Set("validation_label", validationLabel)
		}
		if symbol := semanticMemoryMetadataString(meta, "symbol"); symbol != "" {
			params.Set("symbol", strings.ToUpper(strings.TrimSpace(symbol)))
		}
		if side := semanticMemoryMetadataString(meta, "side"); side != "" {
			params.Set("side", strings.ToUpper(strings.TrimSpace(side)))
		}
		if encoded := params.Encode(); encoded != "" {
			return "/pattern-lab?" + encoded
		}
		return "/pattern-lab"
	default:
		return fmt.Sprintf("/deal-review?case_id=%s", strings.TrimSpace(doc.SourceID))
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
