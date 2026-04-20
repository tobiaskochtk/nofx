package market

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	configpkg "nofx/config"
	derivsv1 "nofx/internal/features/derivs/v1"
	"nofx/logger"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	derivsVenueBinanceUSDM  = "binanceusdm"
	derivsHourMillis        = int64(time.Hour / time.Millisecond)
	derivsFundingBucketSize = int64((8 * time.Hour) / time.Millisecond)
)

var (
	derivsSyncOnce       sync.Once
	derivsSyncStore      *derivsv1.RedisStore
	derivsSyncCfg        *configpkg.DerivsV1Config
	derivsSyncInitErr    error
	derivsBootstrapMu    sync.Mutex
	derivsBootstrapped   = map[string]bool{}
	derivsBootstrapTryAt = map[string]time.Time{}
	derivsBootstrapCount = map[string]bootstrapStats{}
)

type bootstrapStats struct {
	oi      int
	funding int
	basis   int
}

const derivsBootstrapRetryInterval = 15 * time.Minute

func syncDerivsBaseCache(symbol string, oi *OIData, premium *premiumIndexData) {
	if symbol == "" || IsXyzDexAsset(symbol) {
		return
	}

	store, cfg, err := ensureDerivsSyncStore()
	if err != nil || store == nil || cfg == nil || !cfg.Enabled {
		return
	}

	norm := derivsv1.SanitizeSymbol(symbol)
	bootstrapDerivsHistoryIfNeeded(store, cfg, norm)

	ts := time.Now().UTC().UnixMilli()
	if premium != nil && premium.Timestamp > 0 {
		ts = premium.Timestamp
	}

	if oi != nil && oi.Latest > 0 && isFinite(oi.Latest) {
		sample := derivsv1.OICacheSample{
			Timestamp: bucketTimestamp(ts, derivsHourMillis),
			Value:     oi.Latest,
		}
		if err := store.UpsertOISamples(norm, derivsVenueBinanceUSDM, []derivsv1.OICacheSample{sample}, "ok"); err != nil {
			logger.Infof("⚠️ [DerivsSync] failed upserting OI sample for %s: %v", norm, err)
		}
	} else {
		_ = store.UpsertOISamples(norm, derivsVenueBinanceUSDM, nil, "miss")
	}

	if premium != nil && isFinite(premium.FundingRate) {
		sample := derivsv1.FundingCacheSample{
			Timestamp: bucketTimestamp(ts, derivsFundingBucketSize),
			Rate:      premium.FundingRate,
		}
		if err := store.UpsertFundingSamples(norm, derivsVenueBinanceUSDM, []derivsv1.FundingCacheSample{sample}, "ok"); err != nil {
			logger.Infof("⚠️ [DerivsSync] failed upserting funding sample for %s: %v", norm, err)
		}
	} else {
		_ = store.UpsertFundingSamples(norm, derivsVenueBinanceUSDM, nil, "miss")
	}

	if premium != nil && premium.MarkPrice > 0 && premium.IndexPrice > 0 && isFinite(premium.MarkPrice) && isFinite(premium.IndexPrice) {
		sample := derivsv1.BasisCacheSample{
			Timestamp: bucketTimestamp(ts, derivsHourMillis),
			Mark:      premium.MarkPrice,
			Index:     premium.IndexPrice,
		}
		if err := store.UpsertBasisSamples(norm, derivsVenueBinanceUSDM, []derivsv1.BasisCacheSample{sample}, "ok"); err != nil {
			logger.Infof("⚠️ [DerivsSync] failed upserting basis sample for %s: %v", norm, err)
		}
	} else {
		_ = store.UpsertBasisSamples(norm, derivsVenueBinanceUSDM, nil, "miss")
	}
}

func ensureDerivsSyncStore() (*derivsv1.RedisStore, *configpkg.DerivsV1Config, error) {
	derivsSyncOnce.Do(func() {
		cfg, err := configpkg.LoadDerivsV1Config("config/derivs_v1.yaml")
		if err != nil {
			derivsSyncInitErr = err
			logger.Infof("⚠️ [DerivsSync] failed loading config: %v", err)
			return
		}
		if cfg == nil || !cfg.Enabled {
			return
		}
		store, err := derivsv1.NewRedisStore(derivsv1.RedisOptions{
			Addr:     cfg.Cache.RedisAddr,
			Password: cfg.Cache.RedisPassword,
			DB:       cfg.Cache.RedisDB,
			Keyspace: cfg.Cache.Keyspace,
		})
		if err != nil {
			derivsSyncInitErr = err
			logger.Infof("⚠️ [DerivsSync] failed connecting redis store: %v", err)
			return
		}
		derivsSyncStore = store
		derivsSyncCfg = cfg
	})
	return derivsSyncStore, derivsSyncCfg, derivsSyncInitErr
}

func bootstrapDerivsHistoryIfNeeded(store *derivsv1.RedisStore, cfg *configpkg.DerivsV1Config, symbol string) {
	now := time.Now().UTC()
	derivsBootstrapMu.Lock()
	if derivsBootstrapped[symbol] {
		derivsBootstrapMu.Unlock()
		return
	}
	if lastTry, ok := derivsBootstrapTryAt[symbol]; ok && now.Sub(lastTry) < derivsBootstrapRetryInterval {
		derivsBootstrapMu.Unlock()
		return
	}
	derivsBootstrapTryAt[symbol] = now
	derivsBootstrapMu.Unlock()

	maxHours := cfg.Timings.BasisZWindowHours
	if cfg.Timings.OIZWindowHours > maxHours {
		maxHours = cfg.Timings.OIZWindowHours
	}
	if cfg.Timings.MinHistoryHours > maxHours {
		maxHours = cfg.Timings.MinHistoryHours
	}
	if maxHours < 24 {
		maxHours = 24
	}
	if maxHours > 500 {
		maxHours = 500
	}

	fundingLimit := cfg.Timings.FundingWindowIntervals * 4
	if fundingLimit < 64 {
		fundingLimit = 64
	}
	if fundingLimit > 500 {
		fundingLimit = 500
	}

	var stats bootstrapStats

	oiSamples, err := fetchBinanceOIHistory(symbol, maxHours)
	if err != nil {
		logger.Infof("⚠️ [DerivsSync] OI bootstrap failed for %s: %v", symbol, err)
	}
	oiStatus := "miss"
	if len(oiSamples) > 0 {
		oiStatus = "ok"
		stats.oi = len(oiSamples)
	}
	if err := store.UpsertOISamples(symbol, derivsVenueBinanceUSDM, oiSamples, oiStatus); err != nil {
		logger.Infof("⚠️ [DerivsSync] OI bootstrap upsert failed for %s: %v", symbol, err)
	}

	fundingSamples, err := fetchBinanceFundingHistory(symbol, fundingLimit)
	if err != nil {
		logger.Infof("⚠️ [DerivsSync] funding bootstrap failed for %s: %v", symbol, err)
	}
	fundingStatus := "miss"
	if len(fundingSamples) > 0 {
		fundingStatus = "ok"
		stats.funding = len(fundingSamples)
	}
	if err := store.UpsertFundingSamples(symbol, derivsVenueBinanceUSDM, fundingSamples, fundingStatus); err != nil {
		logger.Infof("⚠️ [DerivsSync] funding bootstrap upsert failed for %s: %v", symbol, err)
	}

	basisSamples, err := fetchBinanceBasisHistory(symbol, maxHours)
	if err != nil {
		logger.Infof("⚠️ [DerivsSync] basis bootstrap failed for %s: %v", symbol, err)
	}
	basisStatus := "miss"
	if len(basisSamples) > 0 {
		basisStatus = "ok"
		stats.basis = len(basisSamples)
	}
	if err := store.UpsertBasisSamples(symbol, derivsVenueBinanceUSDM, basisSamples, basisStatus); err != nil {
		logger.Infof("⚠️ [DerivsSync] basis bootstrap upsert failed for %s: %v", symbol, err)
	}

	derivsBootstrapMu.Lock()
	derivsBootstrapCount[symbol] = stats
	if stats.oi > 0 || stats.funding > 0 || stats.basis > 0 {
		derivsBootstrapped[symbol] = true
	}
	derivsBootstrapMu.Unlock()
	logger.Infof("[DerivsSync] bootstrap %s complete: oi=%d funding=%d basis=%d", symbol, stats.oi, stats.funding, stats.basis)
}

func fetchBinanceOIHistory(symbol string, limit int) ([]derivsv1.OICacheSample, error) {
	if limit < 1 {
		limit = 1
	}
	var payload []struct {
		Timestamp       int64  `json:"timestamp"`
		SumOpenInterest string `json:"sumOpenInterest"`
	}
	if err := fetchBinanceJSON("/futures/data/openInterestHist", map[string]string{
		"symbol": symbol,
		"period": "1h",
		"limit":  strconv.Itoa(limit),
	}, &payload); err != nil {
		return nil, err
	}
	samples := make([]derivsv1.OICacheSample, 0, len(payload))
	for _, row := range payload {
		value, err := strconv.ParseFloat(row.SumOpenInterest, 64)
		if err != nil || !isFinite(value) || value <= 0 || row.Timestamp <= 0 {
			continue
		}
		samples = append(samples, derivsv1.OICacheSample{
			Timestamp: bucketTimestamp(row.Timestamp, derivsHourMillis),
			Value:     value,
		})
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].Timestamp < samples[j].Timestamp })
	return samples, nil
}

func fetchBinanceFundingHistory(symbol string, limit int) ([]derivsv1.FundingCacheSample, error) {
	if limit < 1 {
		limit = 1
	}
	var payload []struct {
		FundingTime int64  `json:"fundingTime"`
		FundingRate string `json:"fundingRate"`
	}
	if err := fetchBinanceJSON("/fapi/v1/fundingRate", map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(limit),
	}, &payload); err != nil {
		return nil, err
	}
	samples := make([]derivsv1.FundingCacheSample, 0, len(payload))
	for _, row := range payload {
		rate, err := strconv.ParseFloat(row.FundingRate, 64)
		if err != nil || !isFinite(rate) || row.FundingTime <= 0 {
			continue
		}
		samples = append(samples, derivsv1.FundingCacheSample{
			Timestamp: bucketTimestamp(row.FundingTime, derivsFundingBucketSize),
			Rate:      rate,
		})
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].Timestamp < samples[j].Timestamp })
	return samples, nil
}

func fetchBinanceBasisHistory(symbol string, limit int) ([]derivsv1.BasisCacheSample, error) {
	if limit < 1 {
		limit = 1
	}
	mark, err := fetchBinanceKlineClose("/fapi/v1/markPriceKlines", "symbol", symbol, limit)
	if err != nil {
		return nil, err
	}
	index, err := fetchBinanceKlineClose("/fapi/v1/indexPriceKlines", "pair", symbol, limit)
	if err != nil {
		return nil, err
	}

	samples := make([]derivsv1.BasisCacheSample, 0, len(mark))
	for ts, markPrice := range mark {
		indexPrice, ok := index[ts]
		if !ok || !isFinite(markPrice) || !isFinite(indexPrice) || markPrice <= 0 || indexPrice <= 0 {
			continue
		}
		samples = append(samples, derivsv1.BasisCacheSample{
			Timestamp: bucketTimestamp(ts, derivsHourMillis),
			Mark:      markPrice,
			Index:     indexPrice,
		})
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].Timestamp < samples[j].Timestamp })
	return samples, nil
}

func fetchBinanceKlineClose(path, symbolKey, symbol string, limit int) (map[int64]float64, error) {
	if limit < 1 {
		limit = 1
	}
	var payload [][]interface{}
	if err := fetchBinanceJSON(path, map[string]string{
		symbolKey:  symbol,
		"interval": "1h",
		"limit":    strconv.Itoa(limit),
	}, &payload); err != nil {
		return nil, err
	}
	out := make(map[int64]float64, len(payload))
	for _, row := range payload {
		if len(row) < 5 {
			continue
		}
		ts, ok := parseInt64(row[0])
		if !ok || ts <= 0 {
			continue
		}
		price, ok := parseFloat64(row[4])
		if !ok || !isFinite(price) || price <= 0 {
			continue
		}
		out[ts] = price
	}
	return out, nil
}

func fetchBinanceJSON(path string, query map[string]string, out interface{}) error {
	apiClient := NewAPIClient()
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return err
	}
	q := req.URL.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := apiClient.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, truncateForLog(string(body), 180))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func bucketTimestamp(ts, bucketSize int64) int64 {
	if ts <= 0 || bucketSize <= 0 {
		return ts
	}
	return (ts / bucketSize) * bucketSize
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func parseFloat64(raw interface{}) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func parseInt64(raw interface{}) (int64, bool) {
	switch v := raw.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return i, true
		}
		f, ferr := strconv.ParseFloat(v, 64)
		return int64(f), ferr == nil
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return i, true
		}
		f, ferr := v.Float64()
		return int64(f), ferr == nil
	default:
		return 0, false
	}
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[:max]) + "..."
}
