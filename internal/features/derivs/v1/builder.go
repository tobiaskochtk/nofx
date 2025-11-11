package derivsv1

import (
	"fmt"

	"nofx/config"
	"nofx/pkg/types"
)

// Builder orchestrates derivatives feature computations for a symbol snapshot.
type Builder struct {
	cfg            *config.DerivsV1Config
	oiHistory      OIHistory
	fundingHistory FundingHistory
	basisHistory   BasisHistory
	enabledSymbols map[string]struct{}
}

// NewBuilder wires history readers using the provided store.
func NewBuilder(cfg *config.DerivsV1Config, store Store) *Builder {
	if cfg == nil || !cfg.Enabled || store == nil {
		return &Builder{}
	}
	enabled := make(map[string]struct{}, len(cfg.Symbols))
	for _, sym := range cfg.Symbols {
		enabled[SanitizeSymbol(sym)] = struct{}{}
	}
	return &Builder{
		cfg:            cfg,
		oiHistory:      NewCacheOIHistory(store),
		fundingHistory: NewCacheFundingHistory(store),
		basisHistory:   NewCacheBasisHistory(store),
		enabledSymbols: enabled,
	}
}

// Build assembles DerivsFeatures for a normalized symbol (e.g., LINKUSDT).
func (b *Builder) Build(symbol string) (*types.DerivsFeatures, error) {
	if b == nil || b.cfg == nil || !b.cfg.Enabled {
		return nil, nil
	}
	norm := SanitizeSymbol(symbol)
	if _, ok := b.enabledSymbols[norm]; !ok {
		return nil, nil
	}

	oiFeatures, err := ComputeOIFeatures(OIFeatureConfig{
		Symbol:          norm,
		History:         b.oiHistory,
		WindowHours:     b.cfg.Timings.OIZWindowHours,
		CorrWindowHours: 24,
		Alpha:           b.cfg.SmoothingAlpha,
		MinSamples:      b.cfg.Timings.MinHistoryHours,
	})
	if err != nil {
		return nil, fmt.Errorf("compute oi features: %w", err)
	}

	fundingFeatures, err := ComputeFundingFeatures(FundingFeatureConfig{
		Symbol:     norm,
		History:    b.fundingHistory,
		WindowSize: b.cfg.Timings.FundingWindowIntervals,
	})
	if err != nil {
		return nil, fmt.Errorf("compute funding features: %w", err)
	}

	basisFeatures, err := ComputeBasisFeatures(BasisFeatureConfig{
		Symbol:      norm,
		History:     b.basisHistory,
		WindowHours: b.cfg.Timings.BasisZWindowHours,
		Alpha:       b.cfg.SmoothingAlpha,
	})
	if err != nil {
		return nil, fmt.Errorf("compute basis features: %w", err)
	}

	return mergeFeatures(oiFeatures, fundingFeatures, basisFeatures), nil
}

func mergeFeatures(parts ...map[string]interface{}) *types.DerivsFeatures {
	feat := &types.DerivsFeatures{}
	for _, part := range parts {
		for key, raw := range part {
			switch key {
			case "oi_delta_1h_pct":
				feat.OIDelta1hPct = toFloatPointer(raw)
			case "oi_z_7d":
				feat.OIZ7d = toFloatPointer(raw)
			case "oi_price_div":
				feat.OIPriceDiv = toStringPointer(raw)
			case "oi_price_corr_24h":
				feat.OIPriceCorr24h = toFloatPointer(raw)
			case "oi_source_status":
				feat.OISourceStatus = toString(raw)
			case "funding_latest_bps":
				feat.FundingLatestBps = toFloatPointer(raw)
			case "funding_median_z_7d":
				feat.FundingMedianZ7d = toFloatPointer(raw)
			case "funding_dispersion_bps":
				feat.FundingDispersionBps = toFloatPointer(raw)
			case "funding_source_status":
				feat.FundingSourceStatus = toString(raw)
			case "basis_pct":
				feat.BasisPct = toFloatPointer(raw)
			case "basis_z_14d":
				feat.BasisZ14d = toFloatPointer(raw)
			case "basis_source_status":
				feat.BasisSourceStatus = toString(raw)
			}
		}
	}

	// Ensure status fields are non-empty strings to satisfy the contract.
	if feat.OISourceStatus == "" {
		feat.OISourceStatus = ""
	}
	if feat.FundingSourceStatus == "" {
		feat.FundingSourceStatus = ""
	}
	if feat.BasisSourceStatus == "" {
		feat.BasisSourceStatus = ""
	}
	return feat
}

func toFloatPointer(v interface{}) *float64 {
	switch val := v.(type) {
	case float64:
		return &val
	case *float64:
		return val
	}
	return nil
}

func toStringPointer(v interface{}) *string {
	switch val := v.(type) {
	case string:
		return &val
	case *string:
		return val
	}
	return nil
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case *string:
		if val == nil {
			return ""
		}
		return *val
	default:
		return ""
	}
}
