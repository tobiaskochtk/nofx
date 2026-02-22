package mcp

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"nofx/logger"
)

// Config client configuration (centralized management of all configurations)
type Config struct {
	// Provider configuration
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
	// 某些提供商不需要 API Key（例如本地或受信任的代理）
	AllowEmptyAPIKey bool

	// Behavior configuration
	MaxTokens   int
	Temperature float64
	UseFullURL  bool

	// Retry configuration
	MaxRetries      int
	RetryWaitBase   time.Duration
	RetryableErrors []string

	// Timeout configuration
	Timeout time.Duration

	// Dependency injection
	Logger     Logger
	HTTPClient *http.Client
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	timeout := getEnvDurationSeconds("AI_HTTP_TIMEOUT_S", DefaultTimeout)

	return &Config{
		// Default values
		MaxTokens:        getEnvInt("AI_MAX_TOKENS", 2000),
		Temperature:      MCPClientTemperature,
		MaxRetries:       MaxRetryTimes,
		RetryWaitBase:    2 * time.Second,
		Timeout:          timeout,
		RetryableErrors:  retryableErrors,
		AllowEmptyAPIKey: false,

		// Default dependencies (use global logger)
		Logger:     logger.NewMCPLogger(),
		HTTPClient: &http.Client{Timeout: timeout},
	}
}

// getEnvInt reads integer from environment variable, returns default value if failed
func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultValue
}

// getEnvString reads string from environment variable, returns default value if empty
func getEnvString(key string, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// getEnvDurationSeconds reads timeout seconds from env and returns defaultValue if missing/invalid.
func getEnvDurationSeconds(key string, defaultValue time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if sec, err := strconv.Atoi(val); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return defaultValue
}
