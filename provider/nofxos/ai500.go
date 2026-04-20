package nofxos

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// CoinData represents AI500 coin information
type CoinData struct {
	Pair            string  `json:"pair"`             // Trading pair symbol (e.g.: BTCUSDT)
	Score           float64 `json:"score"`            // Current AI score (0-100)
	StartTime       int64   `json:"start_time"`       // Start time (Unix timestamp)
	StartPrice      float64 `json:"start_price"`      // Start price
	LastScore       float64 `json:"last_score"`       // Latest score
	MaxScore        float64 `json:"max_score"`        // Highest score
	MaxPrice        float64 `json:"max_price"`        // Highest price
	IncreasePercent float64 `json:"increase_percent"` // Increase percentage (already x100)
	SelectionBucket string  `json:"selection_bucket,omitempty"`
	IsAvailable     bool    `json:"-"` // Whether tradable (internal use)
}

// AI500Response is the API response structure
type AI500Response struct {
	Success bool `json:"success"`
	Data    struct {
		Coins []CoinData `json:"coins"`
		Count int        `json:"count"`
	} `json:"data"`
}

// GetAI500List retrieves AI500 coin list with retry mechanism
func (c *Client) GetAI500List() ([]CoinData, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("⚠️  Retry attempt %d of %d to fetch AI500 data...", attempt, maxRetries)
			time.Sleep(2 * time.Second)
		}

		coins, err := c.fetchAI500()
		if err == nil {
			if attempt > 1 {
				log.Printf("✓ Retry attempt %d succeeded", attempt)
			}
			return coins, nil
		}

		lastErr = err
		log.Printf("❌ AI500 request attempt %d failed: %v", attempt, err)
	}

	return nil, fmt.Errorf("all AI500 API requests failed: %w", lastErr)
}

func (c *Client) fetchAI500() ([]CoinData, error) {
	log.Printf("🔄 Requesting AI500 data from %s...", c.GetBaseURL())

	body, err := c.doRequest("/api/ai500/list")
	if err != nil {
		return nil, fmt.Errorf("failed to request AI500 API: %w", err)
	}

	var response AI500Response
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API returned failure status")
	}

	// Empty list is a normal condition, not an error
	if len(response.Data.Coins) == 0 {
		log.Printf("ℹ️  AI500 returned empty coin list (no coins meet criteria currently)")
		return []CoinData{}, nil
	}

	coins, dropped := filterValidAI500Coins(response.Data.Coins)
	if dropped > 0 {
		log.Printf("⚠️  Dropped %d invalid AI500 symbol(s) from feed response", dropped)
	}

	// Set IsAvailable flag
	for i := range coins {
		coins[i].IsAvailable = true
	}

	log.Printf("✓ Successfully fetched %d AI500 coins", len(coins))
	logAI500BucketMix(coins)
	return coins, nil
}

func logAI500BucketMix(coins []CoinData) {
	counts := map[string]int{
		"primary":           0,
		"adaptive":          0,
		"fallback_eligible": 0,
		"exploration":       0,
	}
	hasBucketMetadata := false
	sample := make([]string, 0, minInt(len(coins), 6))
	for idx, coin := range coins {
		bucket := strings.TrimSpace(coin.SelectionBucket)
		if bucket != "" {
			hasBucketMetadata = true
			if _, ok := counts[bucket]; ok {
				counts[bucket]++
			}
		}
		if idx < 6 {
			normalizedSymbol, ok := NormalizeValidSymbol(coin.Pair)
			if !ok {
				continue
			}
			if bucket == "" {
				bucket = "unclassified"
			}
			sample = append(sample, fmt.Sprintf("%s[%s:%.2f]", normalizedSymbol, bucket, coin.Score))
		}
	}
	if !hasBucketMetadata {
		return
	}

	log.Printf(
		"📦 AI500 bucket mix: primary=%d adaptive=%d fallback_eligible=%d exploration=%d sample=%s",
		counts["primary"],
		counts["adaptive"],
		counts["fallback_eligible"],
		counts["exploration"],
		strings.Join(sample, ", "),
	)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetTopRatedCoinData retrieves top N available AI500 coins including metadata.
func (c *Client) GetTopRatedCoinData(limit int) ([]CoinData, error) {
	coins, err := c.GetAI500List()
	if err != nil {
		return nil, err
	}

	var availableCoins []CoinData
	for _, coin := range coins {
		if coin.IsAvailable {
			availableCoins = append(availableCoins, coin)
		}
	}

	if len(availableCoins) == 0 {
		return []CoinData{}, nil
	}

	for i := 0; i < len(availableCoins); i++ {
		for j := i + 1; j < len(availableCoins); j++ {
			if availableCoins[i].Score < availableCoins[j].Score {
				availableCoins[i], availableCoins[j] = availableCoins[j], availableCoins[i]
			}
		}
	}

	maxCount := limit
	if maxCount <= 0 || len(availableCoins) < maxCount {
		maxCount = len(availableCoins)
	}

	return append([]CoinData(nil), availableCoins[:maxCount]...), nil
}

// GetTopRatedCoins retrieves top N coins by score (sorted descending)
func (c *Client) GetTopRatedCoins(limit int) ([]string, error) {
	coins, err := c.GetTopRatedCoinData(limit)
	if err != nil {
		return nil, err
	}

	if len(coins) == 0 {
		return []string{}, nil
	}

	var symbols []string
	for _, coin := range coins {
		symbol, ok := NormalizeValidSymbol(coin.Pair)
		if !ok {
			continue
		}
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// GetAvailableCoins retrieves all available coin symbols
func (c *Client) GetAvailableCoins() ([]string, error) {
	coins, err := c.GetAI500List()
	if err != nil {
		return nil, err
	}

	var symbols []string
	for _, coin := range coins {
		if coin.IsAvailable {
			symbol, ok := NormalizeValidSymbol(coin.Pair)
			if !ok {
				continue
			}
			symbols = append(symbols, symbol)
		}
	}

	// Empty list is normal - just return empty slice, not an error
	return symbols, nil
}

// NormalizeSymbol normalizes coin symbol to XXXUSDT format
func NormalizeSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	symbol = strings.ToUpper(symbol)
	if !strings.HasSuffix(symbol, "USDT") {
		symbol = symbol + "USDT"
	}
	return symbol
}

func NormalizeValidSymbol(symbol string) (string, bool) {
	normalized := NormalizeSymbol(symbol)
	base := strings.TrimSuffix(normalized, "USDT")
	if len(base) < 2 || len(base) > 24 {
		return "", false
	}
	for _, r := range base {
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		return "", false
	}
	return normalized, true
}

func filterValidAI500Coins(coins []CoinData) ([]CoinData, int) {
	filtered := make([]CoinData, 0, len(coins))
	dropped := 0
	for _, coin := range coins {
		normalized, ok := NormalizeValidSymbol(coin.Pair)
		if !ok {
			dropped++
			continue
		}
		coin.Pair = normalized
		filtered = append(filtered, coin)
	}
	return filtered, dropped
}
