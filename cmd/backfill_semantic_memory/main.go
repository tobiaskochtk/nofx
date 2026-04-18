package main

import (
	"flag"
	"fmt"
	"log"
	"nofx/crypto"
	"os"
	"strings"

	"nofx/store"

	"github.com/joho/godotenv"
)

type config struct {
	userID                string
	traderID              string
	docTypeCSV            string
	limit                 int
	ensureVectorExtension bool
	embedPending          bool
	onlyPending           bool
	reEmbed               bool
	embeddingConfigID     string
	embeddingModel        string
	embedLimit            int
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		log.Fatalf("semantic memory backfill failed: %v", err)
	}
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.userID, "user-id", envOrDefault("SEMANTIC_MEMORY_USER_ID", "default"), "user id to backfill")
	flag.StringVar(&cfg.traderID, "trader-id", envOrDefault("SEMANTIC_MEMORY_TRADER_ID", ""), "optional trader id filter")
	flag.StringVar(&cfg.docTypeCSV, "doc-type", envOrDefault("SEMANTIC_MEMORY_DOC_TYPE", ""), "optional semantic-memory doc type filter (single value or comma-separated)")
	flag.IntVar(&cfg.limit, "limit", envIntOrDefault("SEMANTIC_MEMORY_LIMIT", 0), "optional per-doc-type limit (0 = all)")
	flag.BoolVar(&cfg.ensureVectorExtension, "ensure-vector-extension", false, "try to create pgvector extension before backfill")
	flag.BoolVar(&cfg.embedPending, "embed-pending", false, "embed pending semantic memory documents after backfill")
	flag.BoolVar(&cfg.onlyPending, "only-pending", false, "when embedding, process only pending docs and skip previously failed docs")
	flag.BoolVar(&cfg.reEmbed, "re-embed", false, "reset selected docs back to pending before embedding")
	flag.StringVar(&cfg.embeddingConfigID, "embedding-config-id", envOrDefault("SEMANTIC_MEMORY_EMBEDDING_CONFIG_ID", ""), "optional AI model config id to use for embeddings")
	flag.StringVar(&cfg.embeddingModel, "embedding-model", envOrDefault("SEMANTIC_MEMORY_EMBEDDING_MODEL", store.SemanticMemoryDefaultEmbeddingModel), "embedding model to use")
	flag.IntVar(&cfg.embedLimit, "embed-limit", envIntOrDefault("SEMANTIC_MEMORY_EMBED_LIMIT", 250), "max pending docs to embed in one run")
	flag.Parse()
	return cfg
}

func run(cfg config) error {
	_ = godotenv.Load()
	cryptoService, err := crypto.NewCryptoService()
	if err != nil {
		return fmt.Errorf("initialize encryption service: %w", err)
	}
	crypto.SetGlobalCryptoService(cryptoService)

	dbCfg := store.DBConfig{
		Type:     store.DBType(strings.ToLower(envOrDefault("DB_TYPE", "postgres"))),
		Path:     envOrDefault("DB_PATH", "data/data.db"),
		Host:     envOrDefault("DB_HOST", "localhost"),
		Port:     envIntOrDefault("DB_PORT", 5432),
		User:     envOrDefault("DB_USER", "nofx"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   envOrDefault("DB_NAME", "nofx"),
		SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
	}
	root, err := store.NewWithConfig(dbCfg)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer root.Close()

	if cfg.ensureVectorExtension {
		if err := root.SemanticMemory().EnsureVectorExtension(); err != nil {
			return fmt.Errorf("ensure vector extension: %w", err)
		}
	}

	available, extName, err := root.SemanticMemory().VectorExtensionStatus()
	if err != nil {
		return fmt.Errorf("check vector extension status: %w", err)
	}
	log.Printf("vector extension available=%t name=%q", available, extName)

	docTypes := parseDocTypes(cfg.docTypeCSV)
	result, err := root.SemanticMemory().BackfillDocuments(strings.TrimSpace(cfg.userID), strings.TrimSpace(cfg.traderID), docTypes, cfg.limit)
	if result != nil && result.Run != nil {
		log.Printf("sync run %s status=%s total=%d inserted=%d updated=%d unchanged=%d failed=%d",
			result.Run.ID,
			result.Run.Status,
			result.Run.TotalDocuments,
			result.Run.InsertedDocuments,
			result.Run.UpdatedDocuments,
			result.Run.UnchangedDocuments,
			result.Run.FailedDocuments,
		)
		log.Printf("summary: %s", result.Run.Summary)
		for docType, count := range result.ByDocType {
			log.Printf("doc_type %s = %d", docType, count)
		}
	}
	if err != nil {
		return err
	}

	if cfg.embedPending {
		modelCfg, err := resolveEmbeddingModelConfig(root, strings.TrimSpace(cfg.userID), strings.TrimSpace(cfg.embeddingConfigID))
		if err != nil {
			return err
		}
		if cfg.reEmbed {
			queued, err := root.SemanticMemory().MarkDocumentsPendingForEmbedding(strings.TrimSpace(cfg.userID), strings.TrimSpace(cfg.traderID), docTypes)
			if err != nil {
				return err
			}
			log.Printf("re-embed queued %d documents back to pending state", queued)
		}
		statuses := []string{
			store.SemanticMemoryEmbeddingStatusPending,
			store.SemanticMemoryEmbeddingStatusFailed,
		}
		if cfg.onlyPending {
			statuses = []string{store.SemanticMemoryEmbeddingStatusPending}
		}
		embedResult, err := root.SemanticMemory().EmbedDocuments(
			strings.TrimSpace(cfg.userID),
			strings.TrimSpace(cfg.traderID),
			docTypes,
			statuses,
			modelCfg,
			strings.TrimSpace(cfg.embeddingModel),
			cfg.embedLimit,
		)
		if embedResult != nil && embedResult.Run != nil {
			log.Printf("embedding run %s status=%s embedded=%d failed=%d prompt_tokens=%d",
				embedResult.Run.ID,
				embedResult.Run.Status,
				embedResult.EmbeddedDocuments,
				embedResult.Run.FailedDocuments,
				embedResult.TotalPromptTokens,
			)
			log.Printf("embedding summary: %s", embedResult.Run.Summary)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func resolveEmbeddingModelConfig(root *store.Store, userID, explicitConfigID string) (*store.AIModel, error) {
	if root == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}
	if explicitConfigID != "" {
		modelCfg, err := root.AIModel().Get(userID, explicitConfigID)
		if err != nil {
			return nil, fmt.Errorf("load embedding model config %s: %w", explicitConfigID, err)
		}
		if !modelCfg.Enabled || strings.TrimSpace(modelCfg.Provider) != "openai" {
			return nil, fmt.Errorf("embedding model config %s must be an enabled OpenAI config", explicitConfigID)
		}
		return modelCfg, nil
	}
	modelCfg, err := root.AIModel().GetFirstEnabledByProvider(userID, "openai")
	if err != nil {
		return nil, fmt.Errorf("resolve enabled OpenAI embedding config: %w", err)
	}
	return modelCfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func parseDocTypes(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
