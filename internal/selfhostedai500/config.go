package selfhostedai500

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                 string
	AuthToken            string
	DBType               string
	DBPath               string
	DBHost               string
	DBPort               int
	DBUser               string
	DBPassword           string
	DBName               string
	DBSSLMode            string
	Exchanges            []string
	RefreshInterval      time.Duration
	UniverseLimit        int
	ScoreThreshold       float64
	BootstrapConcurrency int
	SnapshotRetention    time.Duration
}

func LoadConfig() Config {
	refreshSecs := envInt("SELFHOSTED_AI500_REFRESH_SECS", 60)
	if refreshSecs < 15 {
		refreshSecs = 15
	}

	universeLimit := envInt("SELFHOSTED_AI500_UNIVERSE_LIMIT", 120)
	if universeLimit < 20 {
		universeLimit = 20
	}

	bootstrapConcurrency := envInt("SELFHOSTED_AI500_BOOTSTRAP_CONCURRENCY", 8)
	if bootstrapConcurrency < 1 {
		bootstrapConcurrency = 1
	}
	if bootstrapConcurrency > 32 {
		bootstrapConcurrency = 32
	}

	scoreThreshold := envFloat("SELFHOSTED_AI500_SCORE_THRESHOLD", 70)
	if scoreThreshold < 0 {
		scoreThreshold = 0
	}
	if scoreThreshold > 100 {
		scoreThreshold = 100
	}

	retentionHours := envInt("SELFHOSTED_AI500_SNAPSHOT_RETENTION_HOURS", 72)
	if retentionHours < 24 {
		retentionHours = 24
	}

	exchanges := normalizeUniverseExchanges(envString("SELFHOSTED_AI500_EXCHANGES", "hyperliquid,binance,bybit"))

	return Config{
		Port:                 envString("SELFHOSTED_AI500_PORT", "8081"),
		AuthToken:            envString("SELFHOSTED_AI500_AUTH_TOKEN", "local-selfhosted-ai500-token"),
		DBType:               envString("SELFHOSTED_AI500_DB_TYPE", "postgres"),
		DBPath:               envString("SELFHOSTED_AI500_DB_PATH", "data/selfhosted-ai500/selfhosted-ai500.db"),
		DBHost:               envString("SELFHOSTED_AI500_DB_HOST", "localhost"),
		DBPort:               envInt("SELFHOSTED_AI500_DB_PORT", 5432),
		DBUser:               envString("SELFHOSTED_AI500_DB_USER", "nofx"),
		DBPassword:           envString("SELFHOSTED_AI500_DB_PASSWORD", ""),
		DBName:               envString("SELFHOSTED_AI500_DB_NAME", "nofx"),
		DBSSLMode:            envString("SELFHOSTED_AI500_DB_SSLMODE", "disable"),
		Exchanges:            exchanges,
		RefreshInterval:      time.Duration(refreshSecs) * time.Second,
		UniverseLimit:        universeLimit,
		ScoreThreshold:       scoreThreshold,
		BootstrapConcurrency: bootstrapConcurrency,
		SnapshotRetention:    time.Duration(retentionHours) * time.Hour,
	}
}

func (c Config) StoreConfig() StoreConfig {
	return StoreConfig{
		Type:     normalizeStoreDBType(c.DBType),
		Path:     c.DBPath,
		Host:     c.DBHost,
		Port:     c.DBPort,
		User:     c.DBUser,
		Password: c.DBPassword,
		DBName:   c.DBName,
		SSLMode:  c.DBSSLMode,
	}
}

func envString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
