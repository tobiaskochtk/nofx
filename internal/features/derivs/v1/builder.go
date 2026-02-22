package derivsv1

import (
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

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
	strictSymbols  bool
	micro          *microFetcher
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

	// Load Binance API credentials from environment
	apiKey := os.Getenv("BINANCE_API_KEY")
	apiSecret := os.Getenv("BINANCE_API_SECRET")

	var microFetcher *microFetcher
	if apiKey != "" && apiSecret != "" {
		log.Printf("[NewBuilder] Initializing microFetcher with Binance API credentials")
		microFetcher = newMicroFetcherWithAuth(apiKey, apiSecret)
	} else {
		log.Printf("[NewBuilder] Initializing microFetcher without API credentials (liquidation data unavailable)")
		microFetcher = newMicroFetcher()
	}

	return &Builder{
		cfg:            cfg,
		oiHistory:      NewCacheOIHistory(store),
		fundingHistory: NewCacheFundingHistory(store),
		basisHistory:   NewCacheBasisHistory(store),
		enabledSymbols: enabled,
		strictSymbols:  derivsStrictSymbolsEnabled(),
		micro:          microFetcher,
	}
}

// Build assembles DerivsFeatures for a normalized symbol (e.g., LINKUSDT).
func (b *Builder) Build(symbol string) (*types.DerivsFeatures, error) {
	log.Printf("[Builder.Build] ENTRY: %s", symbol)
	if b == nil || b.cfg == nil || !b.cfg.Enabled {
		log.Printf("[Builder.Build] %s: early return (nil builder or disabled)", symbol)
		return nil, nil
	}
	norm := SanitizeSymbol(symbol)
	log.Printf("[Builder.Build] %s: normalized to %s", symbol, norm)

	if _, ok := b.enabledSymbols[norm]; !ok && len(b.enabledSymbols) > 0 {
		if b.strictSymbols {
			log.Printf("[Builder.Build] %s: NOT in enabled list (strict mode enabled, skipping)", norm)
			return nil, nil
		}
		log.Printf("[Builder.Build] %s: NOT in enabled list (strict mode off, processing dynamic symbol)", norm)
	}
	log.Printf("[Builder.Build] %s: proceeding with feature computation", norm)

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

	microFeatures, err := ComputeMicrostructureFeatures(MicroFeatureConfig{
		Symbol:  norm,
		Fetcher: b.micro,
	})
	if err != nil {
		log.Printf("[Builder.Build] %s: microstructure compute warning: %v", norm, err)
	}

	// Avoid hammering the upstream APIs: a small delay keeps aggregate rate within limits when iterating symbols.
	time.Sleep(50 * time.Millisecond)

	return mergeFeatures(oiFeatures, fundingFeatures, basisFeatures, microFeatures), nil
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
			case "cvd_notional_z_3m_short":
				feat.CVDNotionalZ3mShort = toFloatPointer(raw)
			case "cvd_notional_z_3m_long":
				feat.CVDNotionalZ3mLong = toFloatPointer(raw)
			case "imb_notional_z_3m_short":
				feat.ImbNotionalZ3mShort = toFloatPointer(raw)
			case "imb_notional_z_3m_long":
				feat.ImbNotionalZ3mLong = toFloatPointer(raw)
			case "tbr_notional_3m":
				feat.TBRNotional3m = toFloatPointer(raw)
			case "slope_price_3m_short":
				feat.SlopePrice3mShort = toFloatPointer(raw)
			case "slope_price_3m_mid":
				feat.SlopePrice3mMid = toFloatPointer(raw)
			case "r2_price_3m_short":
				feat.R2Price3mShort = toFloatPointer(raw)
			case "r2_price_3m_mid":
				feat.R2Price3mMid = toFloatPointer(raw)
			case "slope_cvdz_3m_short":
				feat.SlopeCVDZ3mShort = toFloatPointer(raw)
			case "slope_cvdz_3m_mid":
				feat.SlopeCVDZ3mMid = toFloatPointer(raw)
			case "r2_cvdz_3m_short":
				feat.R2CVDZ3mShort = toFloatPointer(raw)
			case "r2_cvdz_3m_mid":
				feat.R2CVDZ3mMid = toFloatPointer(raw)
			case "slope_imb_3m_short":
				feat.SlopeImb3mShort = toFloatPointer(raw)
			case "slope_imb_3m_mid":
				feat.SlopeImb3mMid = toFloatPointer(raw)
			case "r2_imb_3m_short":
				feat.R2Imb3mShort = toFloatPointer(raw)
			case "r2_imb_3m_mid":
				feat.R2Imb3mMid = toFloatPointer(raw)
			case "div_bear_short_3m":
				feat.DivBearShort3m = toIntPointer(raw)
			case "div_bull_short_3m":
				feat.DivBullShort3m = toIntPointer(raw)
			case "div_bear_mid_3m":
				feat.DivBearMid3m = toIntPointer(raw)
			case "div_bull_mid_3m":
				feat.DivBullMid3m = toIntPointer(raw)
			case "confidence_cvd_3m":
				feat.ConfidenceCVD3m = toFloatPointer(raw)
			case "dist_up_pct_3m":
				feat.DistUpPct3m = toFloatPointer(raw)
			case "dist_up_atr_3m":
				feat.DistUpAtr3m = toFloatPointer(raw)
			case "cluster_strength_up_3m":
				feat.ClusterStrengthUp3m = toFloatPointer(raw)
			case "dist_dn_pct_3m":
				feat.DistDnPct3m = toFloatPointer(raw)
			case "dist_dn_atr_3m":
				feat.DistDnAtr3m = toFloatPointer(raw)
			case "cluster_strength_down_3m":
				feat.ClusterStrengthDown3m = toFloatPointer(raw)
			case "liq_risk_up_3m":
				feat.LiqRiskUp3m = toIntPointer(raw)
			case "liq_risk_down_3m":
				feat.LiqRiskDown3m = toIntPointer(raw)
			case "prefer_direction_3m":
				feat.PreferDirection3m = toStringPointer(raw)
			case "bucket_usd_3m":
				feat.BucketUsd3m = toFloatPointer(raw)
			case "event_count_liq_3m":
				feat.EventCountLiq3m = toIntPointer(raw)
			case "atr_3m":
				feat.Atr3m = toFloatPointer(raw)
			case "confidence_liq_3m":
				feat.ConfidenceLiq3m = toFloatPointer(raw)
			case "avwap_up_name_3m":
				feat.AVWAPUpName3m = toStringPointer(raw)
			case "avwap_up_price_3m":
				feat.AVWAPUpPrice3m = toFloatPointer(raw)
			case "avwap_up_dist_atr_3m":
				feat.AVWAPUpDistAtr3m = toFloatPointer(raw)
			case "avwap_up_band1_dist_atr_3m":
				feat.AVWAPUpBand1DistAtr3m = toFloatPointer(raw)
			case "avwap_dn_name_3m":
				feat.AVWAPDnName3m = toStringPointer(raw)
			case "avwap_dn_price_3m":
				feat.AVWAPDnPrice3m = toFloatPointer(raw)
			case "avwap_dn_dist_atr_3m":
				feat.AVWAPDnDistAtr3m = toFloatPointer(raw)
			case "avwap_dn_band1_dist_atr_3m":
				feat.AVWAPDnBand1DistAtr3m = toFloatPointer(raw)
			case "avwap_reclaim_up_3m":
				feat.AVWAPReclaimUp3m = toIntPointer(raw)
			case "avwap_rejection_down_3m":
				feat.AVWAPRejectionDn3m = toIntPointer(raw)
			case "avwap_confluence_bull_3m":
				feat.AVWAPConfluenceBull3m = toIntPointer(raw)
			case "avwap_confluence_bear_3m":
				feat.AVWAPConfluenceBear3m = toIntPointer(raw)
			case "avwap_bias_3m":
				feat.AVWAPBias3m = toStringPointer(raw)
			case "confidence_avwap_3m":
				feat.ConfidenceAVWAP3m = toFloatPointer(raw)
			case "bbw_3m":
				feat.BBW3m = toFloatPointer(raw)
			case "kc_width_3m":
				feat.KCWidth3m = toFloatPointer(raw)
			case "bbw_pct_rank_3m":
				feat.BBWPctRank3m = toFloatPointer(raw)
			case "squeeze_on_3m":
				feat.SqueezeOn3m = toIntPointer(raw)
			case "squeeze_persistence_3m":
				feat.SqueezePersist3m = toIntPointer(raw)
			case "squeeze_release_3m":
				feat.SqueezeRelease3m = toIntPointer(raw)
			case "bbw_expansion_rate_3m":
				feat.BBWExpansionRate3m = toFloatPointer(raw)
			case "rv_ratio_3m":
				feat.RvRatio3m = toFloatPointer(raw)
			case "rv_ratio_z_3m":
				feat.RvRatioZ3m = toFloatPointer(raw)
			case "vol_regime_3m":
				feat.VolRegime3m = toStringPointer(raw)
			case "confidence_vol_3m":
				feat.ConfidenceVol3m = toFloatPointer(raw)
			case "cvd_notional_z_15m_short":
				feat.CVDNotionalZ15mShort = toFloatPointer(raw)
			case "cvd_notional_z_15m_long":
				feat.CVDNotionalZ15mLong = toFloatPointer(raw)
			case "imb_notional_z_15m_short":
				feat.ImbNotionalZ15mShort = toFloatPointer(raw)
			case "imb_notional_z_15m_long":
				feat.ImbNotionalZ15mLong = toFloatPointer(raw)
			case "tbr_notional_15m":
				feat.TBRNotional15m = toFloatPointer(raw)
			case "div_bear_mid_15m":
				feat.DivBearMid15m = toIntPointer(raw)
			case "div_bull_mid_15m":
				feat.DivBullMid15m = toIntPointer(raw)
			case "dist_up_atr_15m":
				feat.DistUpAtr15m = toFloatPointer(raw)
			case "dist_dn_atr_15m":
				feat.DistDnAtr15m = toFloatPointer(raw)
			case "prefer_direction_15m":
				feat.PreferDirection15m = toStringPointer(raw)
			case "confidence_cvd_15m":
				feat.ConfidenceCVD15m = toFloatPointer(raw)
			case "confidence_liq_15m":
				feat.ConfidenceLiq15m = toFloatPointer(raw)
			case "avwap_up_name_15m":
				feat.AVWAPUpName15m = toStringPointer(raw)
			case "avwap_up_dist_atr_15m":
				feat.AVWAPUpDistAtr15m = toFloatPointer(raw)
			case "avwap_dn_name_15m":
				feat.AVWAPDnName15m = toStringPointer(raw)
			case "avwap_dn_dist_atr_15m":
				feat.AVWAPDnDistAtr15m = toFloatPointer(raw)
			case "avwap_reclaim_up_15m":
				feat.AVWAPReclaimUp15m = toIntPointer(raw)
			case "avwap_rejection_down_15m":
				feat.AVWAPRejectionDn15m = toIntPointer(raw)
			case "avwap_bias_15m":
				feat.AVWAPBias15m = toStringPointer(raw)
			case "confidence_avwap_15m":
				feat.ConfidenceAVWAP15m = toFloatPointer(raw)
			case "squeeze_on_15m":
				feat.SqueezeOn15m = toIntPointer(raw)
			case "squeeze_release_15m":
				feat.SqueezeRelease15m = toIntPointer(raw)
			case "bbw_pct_rank_15m":
				feat.BBWPctRank15m = toFloatPointer(raw)
			case "rv_ratio_15m":
				feat.RvRatio15m = toFloatPointer(raw)
			case "vol_regime_15m":
				feat.VolRegime15m = toStringPointer(raw)
			case "confidence_vol_15m":
				feat.ConfidenceVol15m = toFloatPointer(raw)
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
	case int:
		f := float64(val)
		return &f
	case int64:
		f := float64(val)
		return &f
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

func toIntPointer(v interface{}) *int {
	switch val := v.(type) {
	case int:
		i := val
		return &i
	case *int:
		return val
	case int64:
		i := int(val)
		return &i
	case float64:
		i := int(math.Round(val))
		return &i
	default:
		return nil
	}
}

func derivsStrictSymbolsEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DERIVS_STRICT_SYMBOLS"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
