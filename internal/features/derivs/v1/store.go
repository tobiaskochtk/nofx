package derivsv1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultCacheDir = "data/derivs"
const redisDefaultTimeout = 2 * time.Second

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
	return replacer.Replace(upper)
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
