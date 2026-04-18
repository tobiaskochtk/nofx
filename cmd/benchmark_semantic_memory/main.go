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
	userID        string
	traderID      string
	docTypeCSV    string
	samplePerType int
	topK          int
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		log.Fatalf("semantic memory benchmark failed: %v", err)
	}
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.userID, "user-id", envOrDefault("SEMANTIC_MEMORY_USER_ID", "default"), "user id to benchmark")
	flag.StringVar(&cfg.traderID, "trader-id", envOrDefault("SEMANTIC_MEMORY_TRADER_ID", ""), "trader id to benchmark")
	flag.StringVar(&cfg.docTypeCSV, "doc-type", envOrDefault("SEMANTIC_MEMORY_DOC_TYPE", ""), "optional semantic-memory doc type filter (single value or comma-separated)")
	flag.IntVar(&cfg.samplePerType, "sample-per-type", envIntOrDefault("SEMANTIC_MEMORY_BENCHMARK_SAMPLE_PER_TYPE", 12), "number of query documents per doc type")
	flag.IntVar(&cfg.topK, "top-k", envIntOrDefault("SEMANTIC_MEMORY_BENCHMARK_TOP_K", 5), "similarity hit depth per query")
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

	run, err := root.SemanticMemory().RunBenchmark(strings.TrimSpace(cfg.userID), strings.TrimSpace(cfg.traderID), store.SemanticMemoryBenchmarkConfig{
		DocTypes:         parseDocTypes(cfg.docTypeCSV),
		SamplePerDocType: cfg.samplePerType,
		TopK:             cfg.topK,
	})
	if err != nil {
		return err
	}

	log.Printf("benchmark run %s status=%s corpora=%d queries=%d", run.ID, run.Status, run.CorpusCount, run.QueryCount)
	log.Printf("summary: %s", run.Summary)
	for _, result := range run.CorpusResults {
		log.Printf(
			"doc_type=%s docs=%d queries=%d skipped=%d zero_hits=%d top1_relevance=%.2f hit@%d=%.2f strong_top1=%.2f",
			result.DocType,
			result.DocumentCount,
			result.EvaluatedQueries,
			result.SkippedQueries,
			result.ZeroHitQueries,
			result.AvgTop1Relevance,
			run.Config.TopK,
			result.HitRateAtK,
			result.StrongTop1Rate,
		)
	}
	return nil
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
