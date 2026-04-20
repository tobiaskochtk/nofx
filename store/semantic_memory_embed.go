package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	semanticMemoryMaxEmbeddingBatchInputs            = 128
	semanticMemoryMaxEmbeddingBatchTokens            = 200000
	semanticMemoryTextEmbedding3SmallCostPer1MTokens = 0.02
	semanticMemoryTextEmbedding3LargeCostPer1MTokens = 0.13
)

type SemanticMemoryEmbeddingRunResult struct {
	Run               *SemanticMemorySyncRun `json:"run"`
	EmbeddedDocuments int                    `json:"embedded_documents"`
	TotalPromptTokens int                    `json:"total_prompt_tokens"`
	ModelConfigID     string                 `json:"model_config_id"`
	ModelName         string                 `json:"model_name"`
}

type openAIEmbeddingsRequest struct {
	Input          []string `json:"input"`
	Model          string   `json:"model"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	User           string   `json:"user,omitempty"`
}

type openAIEmbeddingsResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func normalizeSemanticMemoryEmbeddingModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return SemanticMemoryDefaultEmbeddingModel
	}
	return model
}

func semanticMemoryEmbeddingPricePer1MTokensUSD(model string) float64 {
	switch normalizeSemanticMemoryEmbeddingModel(model) {
	case "text-embedding-3-large":
		return semanticMemoryTextEmbedding3LargeCostPer1MTokens
	default:
		return semanticMemoryTextEmbedding3SmallCostPer1MTokens
	}
}

func semanticMemoryEstimatedCostUSD(model string, promptTokens int) float64 {
	if promptTokens <= 0 {
		return 0
	}
	return (float64(promptTokens) / 1_000_000.0) * semanticMemoryEmbeddingPricePer1MTokensUSD(model)
}

func resolveOpenAIEmbeddingsURL(customURL string) string {
	base := strings.TrimSpace(customURL)
	if base == "" {
		return "https://api.openai.com/v1/embeddings"
	}
	base = strings.TrimSuffix(base, "#")
	base = strings.TrimSuffix(base, "/")
	base = strings.TrimSuffix(base, "/embeddings")
	base = strings.TrimSuffix(base, "/chat/completions")
	base = strings.TrimSuffix(base, "/models")
	if strings.HasSuffix(base, "/v1") {
		return base + "/embeddings"
	}
	return base + "/embeddings"
}

func (s *SemanticMemoryStore) ListDocumentsByEmbeddingStatuses(userID, traderID string, statuses []string, limit int) ([]*SemanticMemoryDocument, error) {
	return s.ListDocumentsForEmbedding(userID, traderID, nil, statuses, limit)
}

func (s *SemanticMemoryStore) ListDocumentsForEmbedding(userID, traderID string, docTypes, statuses []string, limit int) ([]*SemanticMemoryDocument, error) {
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	normalized := make([]string, 0, len(statuses))
	for _, status := range statuses {
		normalized = append(normalized, normalizeSemanticMemoryEmbeddingStatus(status))
	}
	if len(normalized) == 0 {
		normalized = append(normalized, SemanticMemoryEmbeddingStatusPending, SemanticMemoryEmbeddingStatusFailed)
	}
	query := s.db.Model(&SemanticMemoryDocument{}).
		Where("user_id = ? AND embedding_status IN ?", userID, normalized)
	if strings.TrimSpace(traderID) != "" {
		query = query.Where("trader_id = ?", traderID)
	}
	if normalizedDocTypes := semanticMemoryNormalizeDocTypes(docTypes); len(normalizedDocTypes) > 0 {
		query = query.Where("doc_type IN ?", normalizedDocTypes)
	}
	var items []*SemanticMemoryDocument
	if err := query.Order("updated_at ASC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *SemanticMemoryStore) MarkDocumentsPendingForEmbedding(userID, traderID string, docTypes []string) (int64, error) {
	query := s.db.Model(&SemanticMemoryDocument{}).
		Where("user_id = ? AND embedding_status <> ?", userID, SemanticMemoryEmbeddingStatusSkipped)
	if strings.TrimSpace(traderID) != "" {
		query = query.Where("trader_id = ?", traderID)
	}
	if normalizedDocTypes := semanticMemoryNormalizeDocTypes(docTypes); len(normalizedDocTypes) > 0 {
		query = query.Where("doc_type IN ?", normalizedDocTypes)
	}
	result := query.Updates(map[string]any{
		"embedding_status":     SemanticMemoryEmbeddingStatusPending,
		"last_embedding_error": "",
	})
	return result.RowsAffected, result.Error
}

func (s *SemanticMemoryStore) EmbedPendingDocuments(userID, traderID string, modelCfg *AIModel, modelName string, limit int) (*SemanticMemoryEmbeddingRunResult, error) {
	return s.EmbedDocuments(userID, traderID, nil, []string{
		SemanticMemoryEmbeddingStatusPending,
		SemanticMemoryEmbeddingStatusFailed,
	}, modelCfg, modelName, limit)
}

func (s *SemanticMemoryStore) EmbedDocuments(userID, traderID string, docTypes, statuses []string, modelCfg *AIModel, modelName string, limit int) (*SemanticMemoryEmbeddingRunResult, error) {
	if modelCfg == nil {
		return nil, fmt.Errorf("embedding model config cannot be nil")
	}
	if strings.TrimSpace(modelCfg.Provider) != "openai" {
		return nil, fmt.Errorf("semantic memory embeddings currently require an enabled OpenAI model config")
	}
	modelName = normalizeSemanticMemoryEmbeddingModel(modelName)
	if modelName != SemanticMemoryDefaultEmbeddingModel {
		return nil, fmt.Errorf("semantic memory embeddings currently support only %s", SemanticMemoryDefaultEmbeddingModel)
	}
	if s.db == nil || s.db.Dialector.Name() != "postgres" {
		return nil, fmt.Errorf("semantic memory embeddings require PostgreSQL with pgvector")
	}
	vectorAvailable, _, err := s.VectorExtensionStatus()
	if err != nil {
		return nil, err
	}
	if !vectorAvailable {
		return nil, fmt.Errorf("pgvector extension is not available in the current PostgreSQL runtime")
	}

	now := time.Now().UTC()
	run := &SemanticMemorySyncRun{
		ID:        uuid.NewString(),
		UserID:    userID,
		TraderID:  traderID,
		Scope:     SemanticMemorySyncScopeEmbedPending,
		Status:    SemanticMemorySyncStatusRunning,
		StartedAt: now,
	}
	if err := s.db.Create(run).Error; err != nil {
		return nil, err
	}

	normalizedDocTypes := semanticMemoryNormalizeDocTypes(docTypes)
	normalizedStatuses := make([]string, 0, len(statuses))
	for _, status := range statuses {
		normalizedStatuses = append(normalizedStatuses, normalizeSemanticMemoryEmbeddingStatus(status))
	}
	if len(normalizedStatuses) == 0 {
		normalizedStatuses = []string{
			SemanticMemoryEmbeddingStatusPending,
			SemanticMemoryEmbeddingStatusFailed,
		}
	}

	docs, err := s.ListDocumentsForEmbedding(userID, traderID, normalizedDocTypes, normalizedStatuses, limit)
	if err != nil {
		run.Status = SemanticMemorySyncStatusFailed
		run.ErrorMessage = err.Error()
		run.CompletedAt = time.Now().UTC()
		run.Summary = "Failed to list pending semantic memory documents."
		_ = s.db.Save(run).Error
		return nil, err
	}

	result := &SemanticMemoryEmbeddingRunResult{
		Run:           run,
		ModelConfigID: modelCfg.ID,
		ModelName:     modelName,
	}
	run.EmbeddingProvider = strings.TrimSpace(modelCfg.Provider)
	run.EmbeddingModel = modelName
	if len(docs) == 0 {
		run.EstimatedCostUSD = 0
		run.Status = SemanticMemorySyncStatusCompleted
		run.CompletedAt = time.Now().UTC()
		run.Summary = "No pending or failed semantic memory documents to embed."
		run.MetadataJSON = semanticMemoryMarshalJSON(map[string]any{
			"embedding_provider": modelCfg.Provider,
			"embedding_model":    modelName,
			"prompt_tokens":      0,
			"estimated_cost_usd": 0,
			"doc_types":          normalizedDocTypes,
			"statuses":           normalizedStatuses,
		})
		if err := s.db.Save(run).Error; err != nil {
			return nil, err
		}
		return result, nil
	}

	client := &http.Client{Timeout: 120 * time.Second}
	embeddingURL := resolveOpenAIEmbeddingsURL(modelCfg.CustomAPIURL)

	for _, batch := range semanticMemorySplitEmbeddingBatches(docs) {
		inputs := make([]string, 0, len(batch))
		for _, doc := range batch {
			inputs = append(inputs, buildSemanticMemoryEmbeddingInput(doc))
		}

		embeddings, promptTokens, err := callOpenAIEmbeddings(client, embeddingURL, string(modelCfg.APIKey), userID, modelName, inputs)
		run.TotalDocuments += len(batch)
		if err != nil {
			run.FailedDocuments += len(batch)
			if updateErr := s.markDocumentsEmbeddingFailed(batch, err.Error()); updateErr != nil && run.ErrorMessage == "" {
				run.ErrorMessage = updateErr.Error()
			}
			if run.ErrorMessage == "" {
				run.ErrorMessage = err.Error()
			}
			continue
		}
		if len(embeddings) != len(batch) {
			err = fmt.Errorf("embedding response count mismatch: got %d embeddings for %d docs", len(embeddings), len(batch))
			run.FailedDocuments += len(batch)
			if updateErr := s.markDocumentsEmbeddingFailed(batch, err.Error()); updateErr != nil && run.ErrorMessage == "" {
				run.ErrorMessage = updateErr.Error()
			}
			if run.ErrorMessage == "" {
				run.ErrorMessage = err.Error()
			}
			continue
		}
		if err := s.storeEmbeddingBatch(batch, embeddings, modelCfg.Provider, modelName); err != nil {
			run.FailedDocuments += len(batch)
			if updateErr := s.markDocumentsEmbeddingFailed(batch, err.Error()); updateErr != nil && run.ErrorMessage == "" {
				run.ErrorMessage = updateErr.Error()
			}
			if run.ErrorMessage == "" {
				run.ErrorMessage = err.Error()
			}
			continue
		}
		run.UpdatedDocuments += len(batch)
		result.EmbeddedDocuments += len(batch)
		result.TotalPromptTokens += promptTokens
	}

	run.CompletedAt = time.Now().UTC()
	run.PromptTokens = result.TotalPromptTokens
	run.EstimatedCostUSD = semanticMemoryEstimatedCostUSD(modelName, result.TotalPromptTokens)
	run.MetadataJSON = semanticMemoryMarshalJSON(map[string]any{
		"embedding_provider": modelCfg.Provider,
		"embedding_model":    modelName,
		"prompt_tokens":      result.TotalPromptTokens,
		"estimated_cost_usd": run.EstimatedCostUSD,
		"doc_types":          normalizedDocTypes,
		"statuses":           normalizedStatuses,
	})
	if run.FailedDocuments > 0 {
		run.Status = SemanticMemorySyncStatusFailed
		if strings.TrimSpace(run.ErrorMessage) == "" {
			run.ErrorMessage = "one or more embedding batches failed"
		}
		run.Summary = fmt.Sprintf("Embedded %d semantic documents, %d failed.", run.UpdatedDocuments, run.FailedDocuments)
	} else {
		run.Status = SemanticMemorySyncStatusCompleted
		run.Summary = fmt.Sprintf("Embedded %d semantic documents.", run.UpdatedDocuments)
	}
	if err := s.db.Save(run).Error; err != nil {
		return nil, err
	}
	return result, nil
}

func buildSemanticMemoryEmbeddingInput(doc *SemanticMemoryDocument) string {
	if doc == nil {
		return ""
	}
	return strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(doc.Title),
		strings.TrimSpace(doc.Summary),
		strings.TrimSpace(doc.Body),
	}, "\n\n"))
}

func semanticMemorySplitEmbeddingBatches(docs []*SemanticMemoryDocument) [][]*SemanticMemoryDocument {
	if len(docs) == 0 {
		return nil
	}
	batches := make([][]*SemanticMemoryDocument, 0, 1)
	current := make([]*SemanticMemoryDocument, 0, semanticMemoryMaxEmbeddingBatchInputs)
	currentTokens := 0
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		docTokens := doc.TokenEstimate
		if docTokens <= 0 {
			docTokens = semanticMemoryApproxTokens(buildSemanticMemoryEmbeddingInput(doc))
		}
		if len(current) > 0 && (len(current) >= semanticMemoryMaxEmbeddingBatchInputs || currentTokens+docTokens > semanticMemoryMaxEmbeddingBatchTokens) {
			batches = append(batches, current)
			current = make([]*SemanticMemoryDocument, 0, semanticMemoryMaxEmbeddingBatchInputs)
			currentTokens = 0
		}
		current = append(current, doc)
		currentTokens += docTokens
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func callOpenAIEmbeddings(client *http.Client, url, apiKey, userID, modelName string, inputs []string) ([][]float64, int, error) {
	requestBody := openAIEmbeddingsRequest{
		Input:          inputs,
		Model:          modelName,
		EncodingFormat: "float",
		User:           strings.TrimSpace(userID),
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("embeddings request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed openAIEmbeddingsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, 0, err
	}
	embeddings := make([][]float64, len(parsed.Data))
	for _, item := range parsed.Data {
		if item.Index < 0 || item.Index >= len(embeddings) {
			return nil, 0, fmt.Errorf("embeddings response index out of range: %d", item.Index)
		}
		embeddings[item.Index] = item.Embedding
	}
	return embeddings, parsed.Usage.PromptTokens, nil
}

func (s *SemanticMemoryStore) markDocumentsEmbeddingFailed(batch []*SemanticMemoryDocument, errMsg string) error {
	ids := make([]string, 0, len(batch))
	for _, doc := range batch {
		if doc == nil || strings.TrimSpace(doc.ID) == "" {
			continue
		}
		ids = append(ids, doc.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	return s.db.Model(&SemanticMemoryDocument{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"embedding_status":     SemanticMemoryEmbeddingStatusFailed,
			"last_embedding_error": strings.TrimSpace(errMsg),
		}).Error
}

func (s *SemanticMemoryStore) storeEmbeddingBatch(batch []*SemanticMemoryDocument, embeddings [][]float64, provider, model string) error {
	now := time.Now().UTC()
	return s.db.Transaction(func(tx *gorm.DB) error {
		for idx, doc := range batch {
			if doc == nil {
				continue
			}
			vector := embeddings[idx]
			if len(vector) != SemanticMemoryDefaultEmbeddingDims {
				return fmt.Errorf("unexpected embedding dimension %d for doc %s", len(vector), doc.ID)
			}
			if err := tx.Exec(`
				INSERT INTO semantic_memory_vectors (
					document_id, embedding, embedding_provider, embedding_model, dimensions, created_at, updated_at
				)
				VALUES (?, CAST(? AS vector), ?, ?, ?, ?, ?)
				ON CONFLICT (document_id) DO UPDATE SET
					embedding = EXCLUDED.embedding,
					embedding_provider = EXCLUDED.embedding_provider,
					embedding_model = EXCLUDED.embedding_model,
					dimensions = EXCLUDED.dimensions,
					updated_at = EXCLUDED.updated_at
			`, doc.ID, semanticMemoryVectorLiteral(vector), provider, model, len(vector), now, now).Error; err != nil {
				return err
			}
			if err := tx.Model(&SemanticMemoryDocument{}).
				Where("id = ?", doc.ID).
				Updates(map[string]any{
					"embedding_status":     SemanticMemoryEmbeddingStatusEmbedded,
					"last_embedding_error": "",
					"last_embedded_at":     now,
					"embedding_provider":   provider,
					"embedding_model":      model,
					"embedding_dimensions": len(vector),
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func semanticMemoryVectorLiteral(values []float64) string {
	var builder strings.Builder
	builder.Grow(len(values) * 12)
	builder.WriteByte('[')
	for idx, value := range values {
		if idx > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
	}
	builder.WriteByte(']')
	return builder.String()
}
