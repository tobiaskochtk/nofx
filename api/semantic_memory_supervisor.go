package api

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"nofx/logger"
	"nofx/store"

	"gorm.io/gorm"
)

const (
	semanticMemoryRefreshInterval       = 15 * time.Minute
	semanticMemoryRefreshEmbeddingLimit = 250
	semanticMemoryDecisionSummaryLimit  = 150
)

type semanticMemoryRefreshScope struct {
	UserID   string
	TraderID string
	Name     string
}

func (s *Server) RunSemanticMemoryRefreshSupervisor() {
	if err := s.ProcessPendingSemanticMemoryRefresh(); err != nil {
		logger.Warnf("⚠️ Initial semantic memory refresh failed: %v", err)
	}
	ticker := time.NewTicker(semanticMemoryRefreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		if err := s.ProcessPendingSemanticMemoryRefresh(); err != nil {
			logger.Warnf("⚠️ Semantic memory refresh supervisor failed: %v", err)
		}
	}
}

func (s *Server) ProcessPendingSemanticMemoryRefresh() error {
	if s == nil || s.store == nil {
		return fmt.Errorf("semantic memory refresh store is unavailable")
	}

	traders, err := s.store.Trader().ListAll()
	if err != nil {
		return err
	}
	scopes := collectSemanticMemoryRefreshScopes(traders)
	if len(scopes) == 0 {
		return nil
	}

	canEmbed := false
	if s.store.GormDB() != nil && s.store.GormDB().Dialector.Name() == "postgres" {
		vectorAvailable, _, vectorErr := s.store.SemanticMemory().VectorExtensionStatus()
		if vectorErr != nil {
			return vectorErr
		}
		canEmbed = vectorAvailable
	}

	modelCache := map[string]*store.AIModel{}
	missingModelUsers := map[string]struct{}{}
	for _, scope := range scopes {
		if strings.TrimSpace(scope.UserID) == "" || strings.TrimSpace(scope.TraderID) == "" {
			continue
		}
		coreBackfillResult, coreBackfillErr := s.store.SemanticMemory().BackfillDocuments(scope.UserID, scope.TraderID, nil, 0)
		if coreBackfillErr != nil {
			logger.Warnf("⚠️ Semantic memory core backfill failed for trader %s (%s): %v", fallbackSemanticMemoryTraderName(scope.Name, scope.TraderID), scope.TraderID, coreBackfillErr)
		}

		decisionBackfillResult, decisionBackfillErr := s.store.SemanticMemory().BackfillDocuments(
			scope.UserID,
			scope.TraderID,
			[]string{store.SemanticMemoryDocTypeDecisionRecordSummary},
			semanticMemoryDecisionSummaryLimit,
		)
		if decisionBackfillErr != nil {
			logger.Warnf("⚠️ Semantic memory decision-summary backfill failed for trader %s (%s): %v", fallbackSemanticMemoryTraderName(scope.Name, scope.TraderID), scope.TraderID, decisionBackfillErr)
		}
		if coreBackfillErr != nil && decisionBackfillErr != nil {
			continue
		}
		backfillResult := mergeSemanticMemoryBackfillResults(coreBackfillResult, decisionBackfillResult)

		embeddedDocs := 0
		embeddedTokens := 0
		if canEmbed {
			modelCfg, ok := modelCache[scope.UserID]
			if !ok {
				cfg, modelErr := s.store.AIModel().GetFirstEnabledByProvider(scope.UserID, "openai")
				switch {
				case modelErr == nil:
					modelCache[scope.UserID] = cfg
					modelCfg = cfg
				case errors.Is(modelErr, gorm.ErrRecordNotFound):
					modelCache[scope.UserID] = nil
					modelCfg = nil
				default:
					logger.Warnf("⚠️ Semantic memory model lookup failed for user %s: %v", scope.UserID, modelErr)
					modelCache[scope.UserID] = nil
					modelCfg = nil
				}
			}
			if modelCfg == nil {
				if _, logged := missingModelUsers[scope.UserID]; !logged {
					logger.Infof("ℹ️ Semantic memory embedding skipped for user %s: no enabled OpenAI account configured", scope.UserID)
					missingModelUsers[scope.UserID] = struct{}{}
				}
			} else {
				embedResult, embedErr := s.store.SemanticMemory().EmbedPendingDocuments(
					scope.UserID,
					scope.TraderID,
					modelCfg,
					store.SemanticMemoryDefaultEmbeddingModel,
					semanticMemoryRefreshEmbeddingLimit,
				)
				if embedErr != nil {
					logger.Warnf("⚠️ Semantic memory embedding failed for trader %s (%s): %v", fallbackSemanticMemoryTraderName(scope.Name, scope.TraderID), scope.TraderID, embedErr)
				} else if embedResult != nil {
					embeddedDocs = embedResult.EmbeddedDocuments
					embeddedTokens = embedResult.TotalPromptTokens
				}
			}
		}

		if shouldLogSemanticMemoryRefresh(backfillResult, embeddedDocs, embeddedTokens) {
			logger.Infof(
				"🧠 Semantic memory refreshed for trader %s (%s): docs=%d inserted=%d updated=%d unchanged=%d failed=%d embedded=%d tokens=%d",
				fallbackSemanticMemoryTraderName(scope.Name, scope.TraderID),
				scope.TraderID,
				safeSemanticMemoryRunCount(backfillResult, "total"),
				safeSemanticMemoryRunCount(backfillResult, "inserted"),
				safeSemanticMemoryRunCount(backfillResult, "updated"),
				safeSemanticMemoryRunCount(backfillResult, "unchanged"),
				safeSemanticMemoryRunCount(backfillResult, "failed"),
				embeddedDocs,
				embeddedTokens,
			)
		}
	}

	if !canEmbed {
		logger.Infof("ℹ️ Semantic memory embedding skipped: pgvector is not available in the current runtime")
	}

	return nil
}

func mergeSemanticMemoryBackfillResults(results ...*store.SemanticMemoryBackfillResult) *store.SemanticMemoryBackfillResult {
	merged := &store.SemanticMemoryBackfillResult{
		Run: &store.SemanticMemorySyncRun{},
	}
	haveAny := false
	for _, result := range results {
		if result == nil || result.Run == nil {
			continue
		}
		haveAny = true
		merged.Run.TotalDocuments += result.Run.TotalDocuments
		merged.Run.InsertedDocuments += result.Run.InsertedDocuments
		merged.Run.UpdatedDocuments += result.Run.UpdatedDocuments
		merged.Run.UnchangedDocuments += result.Run.UnchangedDocuments
		merged.Run.FailedDocuments += result.Run.FailedDocuments
	}
	if !haveAny {
		return nil
	}
	return merged
}

func collectSemanticMemoryRefreshScopes(traders []*store.Trader) []semanticMemoryRefreshScope {
	scopes := make([]semanticMemoryRefreshScope, 0, len(traders))
	seen := map[string]struct{}{}
	for _, trader := range traders {
		if trader == nil {
			continue
		}
		userID := strings.TrimSpace(trader.UserID)
		traderID := strings.TrimSpace(trader.ID)
		if userID == "" || traderID == "" {
			continue
		}
		key := userID + "::" + traderID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		scopes = append(scopes, semanticMemoryRefreshScope{
			UserID:   userID,
			TraderID: traderID,
			Name:     strings.TrimSpace(trader.Name),
		})
	}
	return scopes
}

func shouldLogSemanticMemoryRefresh(result *store.SemanticMemoryBackfillResult, embeddedDocs, embeddedTokens int) bool {
	if embeddedDocs > 0 || embeddedTokens > 0 {
		return true
	}
	if result == nil || result.Run == nil {
		return false
	}
	run := result.Run
	return run.InsertedDocuments > 0 || run.UpdatedDocuments > 0 || run.FailedDocuments > 0
}

func safeSemanticMemoryRunCount(result *store.SemanticMemoryBackfillResult, field string) int {
	if result == nil || result.Run == nil {
		return 0
	}
	switch field {
	case "inserted":
		return result.Run.InsertedDocuments
	case "updated":
		return result.Run.UpdatedDocuments
	case "unchanged":
		return result.Run.UnchangedDocuments
	case "failed":
		return result.Run.FailedDocuments
	default:
		return result.Run.TotalDocuments
	}
}

func fallbackSemanticMemoryTraderName(name, traderID string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(traderID)
}
