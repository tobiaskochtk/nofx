package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DerivsV1Config describes the derivatives flow enrichment knobs loaded from config/derivs_v1.yaml.
type DerivsV1Config struct {
	Enabled            bool              `yaml:"enabled"`
	Symbols            []string          `yaml:"symbols"`
	ExchangesPerp      []string          `yaml:"exchangesPerp"`
	SpotIndexExchanges []string          `yaml:"spotIndexExchanges"`
	Timings            DerivsTimings     `yaml:"timings"`
	SmoothingAlpha     float64           `yaml:"smoothing_alpha"`
	Timeouts           DerivsTimeouts    `yaml:"timeouts_ms"`
	Cache              DerivsCacheConfig `yaml:"cache"`
}

// DerivsTimings captures rolling windows expressed in hours or funding intervals.
type DerivsTimings struct {
	SampleInterval         string `yaml:"sample_interval"`
	OIZWindowHours         int    `yaml:"oi_z_window_hours"`
	BasisZWindowHours      int    `yaml:"basis_z_window_hours"`
	FundingWindowIntervals int    `yaml:"funding_window_intervals"`
	MinHistoryHours        int    `yaml:"min_history_hours"`
}

// DerivsTimeouts exposes REST/WS dial timeouts for the ingest process.
type DerivsTimeouts struct {
	Rest int `yaml:"rest"`
	WS   int `yaml:"ws"`
}

type DerivsCacheConfig struct {
	RedisAddr     string `yaml:"redis_addr"`
	RedisPassword string `yaml:"redis_password"`
	RedisDB       int    `yaml:"redis_db"`
	Keyspace      string `yaml:"keyspace"`
}

// LoadDerivsV1Config parses config/derivs_v1.yaml. If the file is missing we return a disabled config.
func LoadDerivsV1Config(path string) (*DerivsV1Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &DerivsV1Config{}, nil
		}
		return nil, fmt.Errorf("read derivs config: %w", err)
	}

	var wrapper struct {
		DerivsV1 DerivsV1Config `yaml:"derivs_v1"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse derivs config: %w", err)
	}
	cfg := &wrapper.DerivsV1
	if cfg.Cache.RedisAddr == "" {
		cfg.Cache.RedisAddr = "127.0.0.1:6379"
	}
	if cfg.Cache.Keyspace == "" {
		cfg.Cache.Keyspace = "derivs:v1"
	}
	return cfg, nil
}

// HasSymbol reports whether the provided perp or sanitized symbol is configured.
func (c *DerivsV1Config) HasSymbol(symbol string, sanitizer func(string) string) bool {
	if c == nil {
		return false
	}
	sNorm := sanitizer(symbol)
	for _, sym := range c.Symbols {
		if sanitizer(sym) == sNorm {
			return true
		}
	}
	return false
}
