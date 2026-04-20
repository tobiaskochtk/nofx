package derivsv1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultCacheDir = "data/derivs"
const redisDefaultTimeout = 2 * time.Second
const redisMaxSeriesPoints = 400

// Store provides access to cached derivs slices regardless of backing medium.
type Store interface {
	LoadOI(symbol string) (*OICache, error)
	LoadFunding(symbol string) (*FundingCache, error)
	LoadBasis(symbol string) (*BasisCache, error)
}

// SanitizeSymbol normalizes symbols such as LINK/USDT:USDT into LINKUSDT.
func SanitizeSymbol(symbol string) string {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	replacer := strings.NewReplacer("/", "", ":", "", "-", "")
	normalized := replacer.Replace(upper)
	// Hyperliquid uses USDC, but derivs data is indexed by USDT
	normalized = strings.Replace(normalized, "USDC", "USDT", 1)
	return normalized
}

// CacheDir resolves the root directory that stores derivs payloads.
func CacheDir() string {
	if fromEnv := os.Getenv("DERIVS_CACHE_DIR"); fromEnv != "" {
		return fromEnv
	}
	return defaultCacheDir
}

// FileStore reads cached derivs slices from disk.
type FileStore struct {
	root string
}

func NewFileStore(root string) *FileStore {
	if root == "" {
		root = CacheDir()
	}
	return &FileStore{root: root}
}

func (s *FileStore) load(typeDir, symbol string, out any) error {
	file := filepath.Join(s.root, typeDir, fmt.Sprintf("%s.json", SanitizeSymbol(symbol)))
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s cache: %w", typeDir, err)
	}
	return nil
}

// OICache mirrors the JSON payload persisted by the ingest layer.
type OICache struct {
	Symbol    string                     `json:"symbol"`
	UpdatedAt int64                      `json:"updated_at"`
	Samples   map[string][]OICacheSample `json:"samples"`
	Status    map[string]string          `json:"status"`
}

// OICacheSample is a single hourly open-interest observation.
type OICacheSample struct {
	Timestamp int64   `json:"ts"`
	Value     float64 `json:"value"`
}

func (s *FileStore) LoadOI(symbol string) (*OICache, error) {
	var cache OICache
	if err := s.load("oi", symbol, &cache); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &cache, nil
}

// FundingCache stores funding-rate series per venue.
type FundingCache struct {
	Symbol    string                          `json:"symbol"`
	UpdatedAt int64                           `json:"updated_at"`
	Samples   map[string][]FundingCacheSample `json:"samples"`
	Status    map[string]string               `json:"status"`
}

// FundingCacheSample holds per-8h rate observations.
type FundingCacheSample struct {
	Timestamp int64   `json:"ts"`
	Rate      float64 `json:"rate"`
}

func (s *FileStore) LoadFunding(symbol string) (*FundingCache, error) {
	var cache FundingCache
	if err := s.load("funding", symbol, &cache); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &cache, nil
}

// BasisCache stores perp mark/index time-series per venue.
type BasisCache struct {
	Symbol    string                        `json:"symbol"`
	UpdatedAt int64                         `json:"updated_at"`
	Samples   map[string][]BasisCacheSample `json:"samples"`
	Status    map[string]string             `json:"status"`
}

// BasisCacheSample contains mark/index prices for a venue.
type BasisCacheSample struct {
	Timestamp int64   `json:"ts"`
	Mark      float64 `json:"mark"`
	Index     float64 `json:"index"`
}

func (s *FileStore) LoadBasis(symbol string) (*BasisCache, error) {
	var cache BasisCache
	if err := s.load("basis", symbol, &cache); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &cache, nil
}

// RedisOptions defines connection parameters for RedisStore.
type RedisOptions struct {
	Addr     string
	Password string
	DB       int
	Keyspace string
}

// RedisStore fetches cached slices from Redis.
type RedisStore struct {
	client   *redis.Client
	keyspace string
	timeout  time.Duration
}

// NewRedisStore configures a store backed by Redis, returning an error if the connection fails.
func NewRedisStore(cfg RedisOptions) (*RedisStore, error) {
	opts := &redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), redisDefaultTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	keyspace := cfg.Keyspace
	if keyspace == "" {
		keyspace = "derivs:v1"
	}
	return &RedisStore{
		client:   client,
		keyspace: keyspace,
		timeout:  redisDefaultTimeout,
	}, nil
}

func (s *RedisStore) key(kind, symbol string) string {
	return fmt.Sprintf("%s:%s:%s", s.keyspace, kind, SanitizeSymbol(symbol))
}

func (s *RedisStore) load(kind, symbol string, out any) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	data, err := s.client.Get(ctx, s.key(kind, symbol)).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return false, fmt.Errorf("decode %s cache: %w", kind, err)
	}
	return true, nil
}

func (s *RedisStore) set(kind, symbol string, payload any) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key(kind, symbol), body, 0).Err()
}

// LoadOI implements Store for Redis.
func (s *RedisStore) LoadOI(symbol string) (*OICache, error) {
	var cache OICache
	ok, err := s.load("oi", symbol, &cache)
	if err != nil || !ok {
		return nil, err
	}
	return &cache, nil
}

// LoadFunding implements Store for Redis.
func (s *RedisStore) LoadFunding(symbol string) (*FundingCache, error) {
	var cache FundingCache
	ok, err := s.load("funding", symbol, &cache)
	if err != nil || !ok {
		return nil, err
	}
	return &cache, nil
}

// LoadBasis implements Store for Redis.
func (s *RedisStore) LoadBasis(symbol string) (*BasisCache, error) {
	var cache BasisCache
	ok, err := s.load("basis", symbol, &cache)
	if err != nil || !ok {
		return nil, err
	}
	return &cache, nil
}

// UpsertOISamples merges venue samples into the existing OI cache payload.
func (s *RedisStore) UpsertOISamples(symbol, venue string, samples []OICacheSample, status string) error {
	cache, err := s.LoadOI(symbol)
	if err != nil {
		return err
	}
	if cache == nil {
		cache = &OICache{
			Symbol:  SanitizeSymbol(symbol),
			Samples: map[string][]OICacheSample{},
			Status:  map[string]string{},
		}
	}
	if cache.Samples == nil {
		cache.Samples = map[string][]OICacheSample{}
	}
	if cache.Status == nil {
		cache.Status = map[string]string{}
	}
	if len(samples) > 0 {
		cache.Samples[venue] = mergeOICacheSamples(cache.Samples[venue], samples, redisMaxSeriesPoints)
	}
	if status != "" {
		cache.Status[venue] = status
	}
	cache.UpdatedAt = time.Now().UnixMilli()
	return s.set("oi", symbol, cache)
}

// UpsertFundingSamples merges venue samples into the existing funding cache payload.
func (s *RedisStore) UpsertFundingSamples(symbol, venue string, samples []FundingCacheSample, status string) error {
	cache, err := s.LoadFunding(symbol)
	if err != nil {
		return err
	}
	if cache == nil {
		cache = &FundingCache{
			Symbol:  SanitizeSymbol(symbol),
			Samples: map[string][]FundingCacheSample{},
			Status:  map[string]string{},
		}
	}
	if cache.Samples == nil {
		cache.Samples = map[string][]FundingCacheSample{}
	}
	if cache.Status == nil {
		cache.Status = map[string]string{}
	}
	if len(samples) > 0 {
		cache.Samples[venue] = mergeFundingCacheSamples(cache.Samples[venue], samples, redisMaxSeriesPoints)
	}
	if status != "" {
		cache.Status[venue] = status
	}
	cache.UpdatedAt = time.Now().UnixMilli()
	return s.set("funding", symbol, cache)
}

// UpsertBasisSamples merges venue samples into the existing basis cache payload.
func (s *RedisStore) UpsertBasisSamples(symbol, venue string, samples []BasisCacheSample, status string) error {
	cache, err := s.LoadBasis(symbol)
	if err != nil {
		return err
	}
	if cache == nil {
		cache = &BasisCache{
			Symbol:  SanitizeSymbol(symbol),
			Samples: map[string][]BasisCacheSample{},
			Status:  map[string]string{},
		}
	}
	if cache.Samples == nil {
		cache.Samples = map[string][]BasisCacheSample{}
	}
	if cache.Status == nil {
		cache.Status = map[string]string{}
	}
	if len(samples) > 0 {
		cache.Samples[venue] = mergeBasisCacheSamples(cache.Samples[venue], samples, redisMaxSeriesPoints)
	}
	if status != "" {
		cache.Status[venue] = status
	}
	cache.UpdatedAt = time.Now().UnixMilli()
	return s.set("basis", symbol, cache)
}

func mergeOICacheSamples(existing, incoming []OICacheSample, maxPoints int) []OICacheSample {
	merged := make([]OICacheSample, 0, len(existing)+len(incoming))
	merged = append(merged, existing...)
	merged = append(merged, incoming...)
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Timestamp < merged[j].Timestamp
	})
	dedup := merged[:0]
	for _, sample := range merged {
		if len(dedup) > 0 && dedup[len(dedup)-1].Timestamp == sample.Timestamp {
			dedup[len(dedup)-1] = sample
			continue
		}
		dedup = append(dedup, sample)
	}
	if maxPoints > 0 && len(dedup) > maxPoints {
		dedup = dedup[len(dedup)-maxPoints:]
	}
	return append([]OICacheSample(nil), dedup...)
}

func mergeFundingCacheSamples(existing, incoming []FundingCacheSample, maxPoints int) []FundingCacheSample {
	merged := make([]FundingCacheSample, 0, len(existing)+len(incoming))
	merged = append(merged, existing...)
	merged = append(merged, incoming...)
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Timestamp < merged[j].Timestamp
	})
	dedup := merged[:0]
	for _, sample := range merged {
		if len(dedup) > 0 && dedup[len(dedup)-1].Timestamp == sample.Timestamp {
			dedup[len(dedup)-1] = sample
			continue
		}
		dedup = append(dedup, sample)
	}
	if maxPoints > 0 && len(dedup) > maxPoints {
		dedup = dedup[len(dedup)-maxPoints:]
	}
	return append([]FundingCacheSample(nil), dedup...)
}

func mergeBasisCacheSamples(existing, incoming []BasisCacheSample, maxPoints int) []BasisCacheSample {
	merged := make([]BasisCacheSample, 0, len(existing)+len(incoming))
	merged = append(merged, existing...)
	merged = append(merged, incoming...)
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Timestamp < merged[j].Timestamp
	})
	dedup := merged[:0]
	for _, sample := range merged {
		if len(dedup) > 0 && dedup[len(dedup)-1].Timestamp == sample.Timestamp {
			dedup[len(dedup)-1] = sample
			continue
		}
		dedup = append(dedup, sample)
	}
	if maxPoints > 0 && len(dedup) > maxPoints {
		dedup = dedup[len(dedup)-maxPoints:]
	}
	return append([]BasisCacheSample(nil), dedup...)
}
