package store

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	SemanticMemoryBenchmarkScopeSimilarity = "similarity"

	semanticMemoryBenchmarkDefaultSamplePerDocType = 12
	semanticMemoryBenchmarkDefaultTopK             = 5
	semanticMemoryBenchmarkRelevantThreshold       = 0.45
	semanticMemoryBenchmarkStrongThreshold         = 0.70
)

type SemanticMemoryBenchmarkConfig struct {
	DocTypes         []string `json:"doc_types,omitempty"`
	SamplePerDocType int      `json:"sample_per_doc_type"`
	TopK             int      `json:"top_k"`
}

type SemanticMemoryBenchmarkCorpusResult struct {
	DocType           string  `json:"doc_type"`
	DocumentCount     int     `json:"document_count"`
	EvaluatedQueries  int     `json:"evaluated_queries"`
	SkippedQueries    int     `json:"skipped_queries"`
	ZeroHitQueries    int     `json:"zero_hit_queries"`
	AvgTop1Similarity float64 `json:"avg_top1_similarity"`
	AvgTop1Relevance  float64 `json:"avg_top1_relevance"`
	AvgTopKRelevance  float64 `json:"avg_top_k_relevance"`
	HitRateAtK        float64 `json:"hit_rate_at_k"`
	StrongTop1Rate    float64 `json:"strong_top1_rate"`
	AvgReturnedHits   float64 `json:"avg_returned_hits"`
}

type SemanticMemoryBenchmarkRun struct {
	ID                 string    `gorm:"primaryKey" json:"id"`
	UserID             string    `gorm:"column:user_id;not null;index:idx_semantic_memory_benchmark_runs_user_trader_time" json:"user_id"`
	TraderID           string    `gorm:"column:trader_id;not null;index:idx_semantic_memory_benchmark_runs_user_trader_time" json:"trader_id"`
	Scope              string    `gorm:"column:scope;default:similarity" json:"scope"`
	Status             string    `gorm:"column:status;default:running;index:idx_semantic_memory_benchmark_runs_status" json:"status"`
	Summary            string    `gorm:"column:summary;type:text;default:''" json:"summary"`
	CorpusCount        int       `gorm:"column:corpus_count;default:0" json:"corpus_count"`
	QueryCount         int       `gorm:"column:query_count;default:0" json:"query_count"`
	RelevantQueryCount int       `gorm:"column:relevant_query_count;default:0" json:"relevant_query_count"`
	AvgTop1Similarity  float64   `gorm:"column:avg_top1_similarity;default:0" json:"avg_top1_similarity"`
	AvgTop1Relevance   float64   `gorm:"column:avg_top1_relevance;default:0" json:"avg_top1_relevance"`
	AvgTopKRelevance   float64   `gorm:"column:avg_top_k_relevance;default:0" json:"avg_top_k_relevance"`
	HitRateAtK         float64   `gorm:"column:hit_rate_at_k;default:0" json:"hit_rate_at_k"`
	StrongTop1Rate     float64   `gorm:"column:strong_top1_rate;default:0" json:"strong_top1_rate"`
	MetadataJSON       string    `gorm:"column:metadata_json;type:text;default:'{}'" json:"-"`
	ErrorMessage       string    `gorm:"column:error_message;type:text;default:''" json:"error_message,omitempty"`
	StartedAt          time.Time `gorm:"column:started_at;index:idx_semantic_memory_benchmark_runs_started" json:"started_at"`
	CompletedAt        time.Time `gorm:"column:completed_at" json:"completed_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`

	Config        SemanticMemoryBenchmarkConfig         `gorm:"-" json:"config"`
	CorpusResults []SemanticMemoryBenchmarkCorpusResult `gorm:"-" json:"corpus_results,omitempty"`
}

func (SemanticMemoryBenchmarkRun) TableName() string { return "semantic_memory_benchmark_runs" }

func normalizeSemanticMemoryBenchmarkConfig(cfg SemanticMemoryBenchmarkConfig) SemanticMemoryBenchmarkConfig {
	cfg.DocTypes = semanticMemoryNormalizedRequestedDocTypes(cfg.DocTypes)
	if cfg.SamplePerDocType <= 0 || cfg.SamplePerDocType > 100 {
		cfg.SamplePerDocType = semanticMemoryBenchmarkDefaultSamplePerDocType
	}
	if cfg.TopK <= 0 || cfg.TopK > 20 {
		cfg.TopK = semanticMemoryBenchmarkDefaultTopK
	}
	return cfg
}

func hydrateSemanticMemoryBenchmarkRun(run *SemanticMemoryBenchmarkRun) {
	if run == nil {
		return
	}
	meta := semanticMemoryParseJSONObject(run.MetadataJSON)
	if configRaw, ok := meta["config"]; ok {
		run.Config = parseSemanticMemoryBenchmarkConfig(configRaw)
	}
	if resultsRaw, ok := meta["corpus_results"]; ok {
		run.CorpusResults = parseSemanticMemoryBenchmarkCorpusResults(resultsRaw)
	}
}

func parseSemanticMemoryBenchmarkConfig(raw any) SemanticMemoryBenchmarkConfig {
	cfg := SemanticMemoryBenchmarkConfig{}
	object, ok := raw.(map[string]any)
	if !ok {
		return normalizeSemanticMemoryBenchmarkConfig(cfg)
	}
	cfg.DocTypes = semanticMemoryMetadataStringSlice(object, "doc_types")
	cfg.SamplePerDocType = semanticMemoryMetadataInt(object, "sample_per_doc_type")
	cfg.TopK = semanticMemoryMetadataInt(object, "top_k")
	return normalizeSemanticMemoryBenchmarkConfig(cfg)
}

func parseSemanticMemoryBenchmarkCorpusResults(raw any) []SemanticMemoryBenchmarkCorpusResult {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	results := make([]SemanticMemoryBenchmarkCorpusResult, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		results = append(results, SemanticMemoryBenchmarkCorpusResult{
			DocType:           semanticMemoryMetadataString(object, "doc_type"),
			DocumentCount:     semanticMemoryMetadataInt(object, "document_count"),
			EvaluatedQueries:  semanticMemoryMetadataInt(object, "evaluated_queries"),
			SkippedQueries:    semanticMemoryMetadataInt(object, "skipped_queries"),
			ZeroHitQueries:    semanticMemoryMetadataInt(object, "zero_hit_queries"),
			AvgTop1Similarity: semanticMemoryMetadataFloat(object, "avg_top1_similarity"),
			AvgTop1Relevance:  semanticMemoryMetadataFloat(object, "avg_top1_relevance"),
			AvgTopKRelevance:  semanticMemoryMetadataFloat(object, "avg_top_k_relevance"),
			HitRateAtK:        semanticMemoryMetadataFloat(object, "hit_rate_at_k"),
			StrongTop1Rate:    semanticMemoryMetadataFloat(object, "strong_top1_rate"),
			AvgReturnedHits:   semanticMemoryMetadataFloat(object, "avg_returned_hits"),
		})
	}
	return results
}

func (s *SemanticMemoryStore) ListBenchmarkRuns(userID, traderID string, limit int) ([]*SemanticMemoryBenchmarkRun, error) {
	if limit <= 0 || limit > 50 {
		limit = 8
	}
	query := s.db.Model(&SemanticMemoryBenchmarkRun{}).Where("user_id = ?", strings.TrimSpace(userID))
	if trader := strings.TrimSpace(traderID); trader != "" {
		query = query.Where("trader_id = ?", trader)
	}
	var items []*SemanticMemoryBenchmarkRun
	if err := query.Order("started_at DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		hydrateSemanticMemoryBenchmarkRun(item)
	}
	return items, nil
}

func (s *SemanticMemoryStore) RunBenchmark(userID, traderID string, cfg SemanticMemoryBenchmarkConfig) (*SemanticMemoryBenchmarkRun, error) {
	if s.db == nil || s.db.Dialector.Name() != "postgres" {
		return nil, fmt.Errorf("semantic memory benchmarks require PostgreSQL with pgvector")
	}
	vectorAvailable, _, err := s.VectorExtensionStatus()
	if err != nil {
		return nil, err
	}
	if !vectorAvailable {
		return nil, fmt.Errorf("pgvector extension is not available in the current PostgreSQL runtime")
	}

	cfg = normalizeSemanticMemoryBenchmarkConfig(cfg)
	now := time.Now().UTC()
	run := &SemanticMemoryBenchmarkRun{
		ID:        uuid.NewString(),
		UserID:    strings.TrimSpace(userID),
		TraderID:  strings.TrimSpace(traderID),
		Scope:     SemanticMemoryBenchmarkScopeSimilarity,
		Status:    SemanticMemorySyncStatusRunning,
		StartedAt: now,
	}
	if err := s.db.Create(run).Error; err != nil {
		return nil, err
	}

	results, err := s.executeSemanticMemoryBenchmark(run.UserID, run.TraderID, cfg)
	if err != nil {
		run.Status = SemanticMemorySyncStatusFailed
		run.ErrorMessage = err.Error()
		run.Summary = "Semantic-memory benchmark failed."
		run.CompletedAt = time.Now().UTC()
		run.MetadataJSON = semanticMemoryMarshalJSON(map[string]any{
			"config": cfg,
		})
		_ = s.db.Save(run).Error
		return nil, err
	}

	summarizeSemanticMemoryBenchmarkRun(run, cfg, results)
	if err := s.db.Save(run).Error; err != nil {
		return nil, err
	}
	hydrateSemanticMemoryBenchmarkRun(run)
	return run, nil
}

func (s *SemanticMemoryStore) executeSemanticMemoryBenchmark(userID, traderID string, cfg SemanticMemoryBenchmarkConfig) ([]SemanticMemoryBenchmarkCorpusResult, error) {
	results := make([]SemanticMemoryBenchmarkCorpusResult, 0, len(cfg.DocTypes))
	for _, docType := range cfg.DocTypes {
		docs, err := s.listBenchmarkDocuments(userID, traderID, docType, cfg.SamplePerDocType)
		if err != nil {
			return nil, err
		}
		result := SemanticMemoryBenchmarkCorpusResult{
			DocType:       docType,
			DocumentCount: len(docs),
		}
		if len(docs) < 2 {
			result.SkippedQueries = len(docs)
			results = append(results, result)
			continue
		}

		totalTop1Similarity := 0.0
		totalTop1Relevance := 0.0
		totalTopKRelevance := 0.0
		totalReturnedHits := 0
		relevantQueries := 0
		strongTop1Queries := 0

		for _, doc := range docs {
			similar, err := s.FindSimilarDocumentsBySource(userID, traderID, docType, doc.SourceID, SemanticMemorySearchFilter{}, cfg.TopK)
			if err != nil {
				return nil, err
			}
			result.EvaluatedQueries++
			if len(similar.Items) == 0 {
				result.ZeroHitQueries++
				continue
			}
			totalReturnedHits += len(similar.Items)

			top1 := similar.Items[0]
			top1Relevance := semanticMemoryBenchmarkRelevance(doc, top1.Document)
			totalTop1Similarity += clamp01(top1.SimilarityScore)
			totalTop1Relevance += top1Relevance

			topKSum := 0.0
			hadRelevantHit := false
			for _, hit := range similar.Items {
				relevance := semanticMemoryBenchmarkRelevance(doc, hit.Document)
				topKSum += relevance
				if relevance >= semanticMemoryBenchmarkRelevantThreshold {
					hadRelevantHit = true
				}
			}
			topKAvg := topKSum / float64(len(similar.Items))
			totalTopKRelevance += topKAvg
			if hadRelevantHit {
				relevantQueries++
			}
			if top1Relevance >= semanticMemoryBenchmarkStrongThreshold {
				strongTop1Queries++
			}
		}

		if result.EvaluatedQueries > 0 {
			queryCount := float64(result.EvaluatedQueries)
			result.AvgTop1Similarity = totalTop1Similarity / queryCount
			result.AvgTop1Relevance = totalTop1Relevance / queryCount
			result.AvgTopKRelevance = totalTopKRelevance / queryCount
			result.HitRateAtK = float64(relevantQueries) / queryCount
			result.StrongTop1Rate = float64(strongTop1Queries) / queryCount
			result.AvgReturnedHits = float64(totalReturnedHits) / queryCount
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *SemanticMemoryStore) listBenchmarkDocuments(userID, traderID, docType string, limit int) ([]*SemanticMemoryDocument, error) {
	if limit <= 0 {
		limit = semanticMemoryBenchmarkDefaultSamplePerDocType
	}
	query := s.db.Model(&SemanticMemoryDocument{}).
		Where("user_id = ? AND trader_id = ? AND doc_type = ? AND embedding_status = ?", strings.TrimSpace(userID), strings.TrimSpace(traderID), normalizeSemanticMemoryDocType(docType), SemanticMemoryEmbeddingStatusEmbedded).
		Order("source_updated_at DESC").
		Limit(limit)
	var docs []*SemanticMemoryDocument
	if err := query.Find(&docs).Error; err != nil {
		return nil, err
	}
	return docs, nil
}

func summarizeSemanticMemoryBenchmarkRun(run *SemanticMemoryBenchmarkRun, cfg SemanticMemoryBenchmarkConfig, results []SemanticMemoryBenchmarkCorpusResult) {
	run.Status = SemanticMemorySyncStatusCompleted
	run.CompletedAt = time.Now().UTC()
	run.CorpusCount = len(results)
	totalQueries := 0
	totalRelevantQueries := 0
	totalTop1Similarity := 0.0
	totalTop1Relevance := 0.0
	totalTopKRelevance := 0.0
	totalStrongTop1 := 0.0
	weightedCorpora := 0.0

	for _, result := range results {
		run.QueryCount += result.EvaluatedQueries
		totalQueries += result.EvaluatedQueries
		if result.EvaluatedQueries <= 0 {
			continue
		}
		weighted := float64(result.EvaluatedQueries)
		totalTop1Similarity += result.AvgTop1Similarity * weighted
		totalTop1Relevance += result.AvgTop1Relevance * weighted
		totalTopKRelevance += result.AvgTopKRelevance * weighted
		totalStrongTop1 += result.StrongTop1Rate * weighted
		totalRelevantQueries += int(math.Round(result.HitRateAtK * weighted))
		weightedCorpora += weighted
	}

	if weightedCorpora > 0 {
		run.AvgTop1Similarity = totalTop1Similarity / weightedCorpora
		run.AvgTop1Relevance = totalTop1Relevance / weightedCorpora
		run.AvgTopKRelevance = totalTopKRelevance / weightedCorpora
		run.HitRateAtK = float64(totalRelevantQueries) / weightedCorpora
		run.StrongTop1Rate = totalStrongTop1 / weightedCorpora
	}
	run.RelevantQueryCount = totalRelevantQueries
	run.MetadataJSON = semanticMemoryMarshalJSON(map[string]any{
		"config":         cfg,
		"corpus_results": results,
	})
	run.Summary = fmt.Sprintf(
		"Benchmarked %d corpora / %d queries. Avg top-1 relevance %.0f%%, hit@%d %.0f%%, strong top-1 %.0f%%.",
		run.CorpusCount,
		run.QueryCount,
		run.AvgTop1Relevance*100,
		cfg.TopK,
		run.HitRateAtK*100,
		run.StrongTop1Rate*100,
	)
}

func semanticMemoryBenchmarkRelevance(queryDoc, hitDoc *SemanticMemoryDocument) float64 {
	if queryDoc == nil || hitDoc == nil {
		return 0
	}
	queryMeta := semanticMemoryParseJSONObject(queryDoc.MetadataJSON)
	hitMeta := semanticMemoryParseJSONObject(hitDoc.MetadataJSON)
	switch normalizeSemanticMemoryDocType(queryDoc.DocType) {
	case SemanticMemoryDocTypeAutonomousOptimizerRun:
		return semanticMemoryBenchmarkOptimizerRunRelevance(queryDoc, hitDoc, queryMeta, hitMeta)
	case SemanticMemoryDocTypeOptimizerBacklogItem:
		return semanticMemoryBenchmarkBacklogRelevance(queryDoc, hitDoc, queryMeta, hitMeta)
	case SemanticMemoryDocTypeStrategyVersion:
		return semanticMemoryBenchmarkStrategyVersionRelevance(queryDoc, hitDoc, queryMeta, hitMeta)
	default:
		return semanticMemoryBenchmarkDealCaseRelevance(queryDoc, hitDoc, queryMeta, hitMeta)
	}
}

func semanticMemoryBenchmarkDealCaseRelevance(queryDoc, hitDoc *SemanticMemoryDocument, queryMeta, hitMeta map[string]any) float64 {
	score := semanticMemoryBenchmarkLexicalOverlap(queryDoc, hitDoc) * 0.20
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "symbol") * 0.20
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "close_reason") * 0.15
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "outcome") * 0.10
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "side") * 0.05
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "exit_origin") * 0.05
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "open_selection_bucket") * 0.05
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "open_trend_regime") * 0.05
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "open_volatility_regime") * 0.05
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "open_oi_regime") * 0.05
	score += semanticMemoryBenchmarkArrayOverlap(queryMeta, hitMeta, "labels") * 0.05
	return clamp01(score)
}

func semanticMemoryBenchmarkOptimizerRunRelevance(queryDoc, hitDoc *SemanticMemoryDocument, queryMeta, hitMeta map[string]any) float64 {
	score := semanticMemoryBenchmarkLexicalOverlap(queryDoc, hitDoc) * 0.35
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "status") * 0.25
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "trigger") * 0.15
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "primary_model_name") * 0.05
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "critic_model_name") * 0.05
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "applied_strategy_version_id") * 0.05
	return clamp01(score)
}

func semanticMemoryBenchmarkBacklogRelevance(queryDoc, hitDoc *SemanticMemoryDocument, queryMeta, hitMeta map[string]any) float64 {
	score := semanticMemoryBenchmarkLexicalOverlap(queryDoc, hitDoc) * 0.30
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "category") * 0.30
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "status") * 0.10
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "run_id") * 0.05
	score += semanticMemoryBenchmarkNumericCloseness(queryMeta, hitMeta, "composite_score", 0.2) * 0.10
	score += semanticMemoryBenchmarkTitlePrefixMatch(queryDoc, hitDoc) * 0.15
	return clamp01(score)
}

func semanticMemoryBenchmarkStrategyVersionRelevance(queryDoc, hitDoc *SemanticMemoryDocument, queryMeta, hitMeta map[string]any) float64 {
	score := semanticMemoryBenchmarkLexicalOverlap(queryDoc, hitDoc) * 0.25
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "strategy_id") * 0.20
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "source_type") * 0.15
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "observation_ready") * 0.05
	score += semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta, "rollback_suggested") * 0.05
	score += semanticMemoryBenchmarkTargetCohortOverlap(queryMeta, hitMeta) * 0.30
	return clamp01(score)
}

func semanticMemoryBenchmarkLexicalOverlap(queryDoc, hitDoc *SemanticMemoryDocument) float64 {
	queryTokens := semanticMemoryBenchmarkTokenSet(strings.Join([]string{queryDoc.Title, queryDoc.Summary}, " "))
	hitTokens := semanticMemoryBenchmarkTokenSet(strings.Join([]string{hitDoc.Title, hitDoc.Summary}, " "))
	if len(queryTokens) == 0 || len(hitTokens) == 0 {
		return 0
	}
	intersection := 0
	union := map[string]struct{}{}
	for token := range queryTokens {
		union[token] = struct{}{}
		if _, ok := hitTokens[token]; ok {
			intersection++
		}
	}
	for token := range hitTokens {
		union[token] = struct{}{}
	}
	if len(union) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(union))
}

func semanticMemoryBenchmarkTokenSet(value string) map[string]struct{} {
	value = strings.ToLower(value)
	replacer := strings.NewReplacer(
		",", " ", ".", " ", ":", " ", ";", " ", "/", " ", "\\", " ", "-", " ", "_", " ", "(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ", "\n", " ", "\t", " ",
	)
	parts := strings.Fields(replacer.Replace(value))
	result := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) < 4 {
			continue
		}
		result[part] = struct{}{}
	}
	return result
}

func semanticMemoryBenchmarkExactMatch(queryMeta, hitMeta map[string]any, key string) float64 {
	queryValue := strings.TrimSpace(strings.ToLower(fmt.Sprint(queryMeta[key])))
	hitValue := strings.TrimSpace(strings.ToLower(fmt.Sprint(hitMeta[key])))
	if queryValue == "" || hitValue == "" || queryValue == "<nil>" || hitValue == "<nil>" {
		return 0
	}
	if queryValue == hitValue {
		return 1
	}
	return 0
}

func semanticMemoryBenchmarkArrayOverlap(queryMeta, hitMeta map[string]any, key string) float64 {
	queryItems := semanticMemoryAnyStringSlice(queryMeta[key])
	hitItems := semanticMemoryAnyStringSlice(hitMeta[key])
	if len(queryItems) == 0 || len(hitItems) == 0 {
		return 0
	}
	querySet := make(map[string]struct{}, len(queryItems))
	for _, item := range queryItems {
		querySet[strings.ToLower(strings.TrimSpace(item))] = struct{}{}
	}
	intersection := 0
	union := make(map[string]struct{}, len(queryItems)+len(hitItems))
	for _, item := range queryItems {
		union[strings.ToLower(strings.TrimSpace(item))] = struct{}{}
	}
	for _, item := range hitItems {
		key := strings.ToLower(strings.TrimSpace(item))
		union[key] = struct{}{}
		if _, ok := querySet[key]; ok {
			intersection++
		}
	}
	if len(union) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(union))
}

func semanticMemoryBenchmarkNumericCloseness(queryMeta, hitMeta map[string]any, key string, tolerance float64) float64 {
	queryValue := semanticMemoryMetadataFloat(queryMeta, key)
	hitValue := semanticMemoryMetadataFloat(hitMeta, key)
	if tolerance <= 0 {
		return 0
	}
	diff := math.Abs(queryValue - hitValue)
	if diff >= tolerance {
		return 0
	}
	return 1 - (diff / tolerance)
}

func semanticMemoryBenchmarkTitlePrefixMatch(queryDoc, hitDoc *SemanticMemoryDocument) float64 {
	queryTitle := strings.ToLower(strings.TrimSpace(queryDoc.Title))
	hitTitle := strings.ToLower(strings.TrimSpace(hitDoc.Title))
	if queryTitle == "" || hitTitle == "" {
		return 0
	}
	queryPrefix := queryTitle
	if idx := strings.Index(queryTitle, " "); idx > 0 {
		queryPrefix = queryTitle[:idx]
	}
	if strings.HasPrefix(hitTitle, queryPrefix) {
		return 1
	}
	return 0
}

func semanticMemoryBenchmarkTargetCohortOverlap(queryMeta, hitMeta map[string]any) float64 {
	queryCohort, _ := queryMeta["target_cohort"].(map[string]any)
	hitCohort, _ := hitMeta["target_cohort"].(map[string]any)
	if len(queryCohort) == 0 || len(hitCohort) == 0 {
		return 0
	}
	keys := []string{"symbol", "side", "outcome", "close_reason"}
	matches := 0.0
	total := 0.0
	for _, key := range keys {
		queryValue := strings.TrimSpace(strings.ToLower(fmt.Sprint(queryCohort[key])))
		hitValue := strings.TrimSpace(strings.ToLower(fmt.Sprint(hitCohort[key])))
		if queryValue == "" || hitValue == "" || queryValue == "<nil>" || hitValue == "<nil>" {
			continue
		}
		total++
		if queryValue == hitValue {
			matches++
		}
	}
	if total == 0 {
		return 0
	}
	return matches / total
}

func semanticMemoryAnyStringSlice(raw any) []string {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(fmt.Sprint(item))
		if value == "" || value == "<nil>" {
			continue
		}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
