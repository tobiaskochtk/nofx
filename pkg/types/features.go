package types

// DerivsFeatures captures normalized derivatives flow metrics for a symbol snapshot.
type DerivsFeatures struct {
	OIDelta1hPct   *float64 `json:"oi_delta_1h_pct,omitempty"`
	OIZ7d          *float64 `json:"oi_z_7d,omitempty"`
	OIPriceDiv     *string  `json:"oi_price_div,omitempty"`
	OIPriceCorr24h *float64 `json:"oi_price_corr_24h,omitempty"`
	OISourceStatus string   `json:"oi_source_status"`

	FundingLatestBps     *float64 `json:"funding_latest_bps,omitempty"`
	FundingMedianZ7d     *float64 `json:"funding_median_z_7d,omitempty"`
	FundingDispersionBps *float64 `json:"funding_dispersion_bps,omitempty"`
	FundingSourceStatus  string   `json:"funding_source_status"`

	BasisPct          *float64 `json:"basis_pct,omitempty"`
	BasisZ14d         *float64 `json:"basis_z_14d,omitempty"`
	BasisSourceStatus string   `json:"basis_source_status"`

	// Microstructure Feature 4 (CVD/Taker) - 3m
	CVDNotionalZ3mShort *float64 `json:"cvd_notional_z_3m_short,omitempty"`
	CVDNotionalZ3mLong  *float64 `json:"cvd_notional_z_3m_long,omitempty"`
	ImbNotionalZ3mShort *float64 `json:"imb_notional_z_3m_short,omitempty"`
	ImbNotionalZ3mLong  *float64 `json:"imb_notional_z_3m_long,omitempty"`
	TBRNotional3m       *float64 `json:"tbr_notional_3m,omitempty"`

	SlopePrice3mShort *float64 `json:"slope_price_3m_short,omitempty"`
	SlopePrice3mMid   *float64 `json:"slope_price_3m_mid,omitempty"`
	R2Price3mShort    *float64 `json:"r2_price_3m_short,omitempty"`
	R2Price3mMid      *float64 `json:"r2_price_3m_mid,omitempty"`

	SlopeCVDZ3mShort *float64 `json:"slope_cvdz_3m_short,omitempty"`
	SlopeCVDZ3mMid   *float64 `json:"slope_cvdz_3m_mid,omitempty"`
	R2CVDZ3mShort    *float64 `json:"r2_cvdz_3m_short,omitempty"`
	R2CVDZ3mMid      *float64 `json:"r2_cvdz_3m_mid,omitempty"`

	SlopeImb3mShort *float64 `json:"slope_imb_3m_short,omitempty"`
	SlopeImb3mMid   *float64 `json:"slope_imb_3m_mid,omitempty"`
	R2Imb3mShort    *float64 `json:"r2_imb_3m_short,omitempty"`
	R2Imb3mMid      *float64 `json:"r2_imb_3m_mid,omitempty"`

	DivBearShort3m  *int     `json:"div_bear_short_3m,omitempty"`
	DivBullShort3m  *int     `json:"div_bull_short_3m,omitempty"`
	DivBearMid3m    *int     `json:"div_bear_mid_3m,omitempty"`
	DivBullMid3m    *int     `json:"div_bull_mid_3m,omitempty"`
	ConfidenceCVD3m *float64 `json:"confidence_cvd_3m,omitempty"`

	// Microstructure Feature 5 (Liquidation heatmap) - 3m
	DistUpPct3m           *float64 `json:"dist_up_pct_3m,omitempty"`
	DistUpAtr3m           *float64 `json:"dist_up_atr_3m,omitempty"`
	ClusterStrengthUp3m   *float64 `json:"cluster_strength_up_3m,omitempty"`
	DistDnPct3m           *float64 `json:"dist_dn_pct_3m,omitempty"`
	DistDnAtr3m           *float64 `json:"dist_dn_atr_3m,omitempty"`
	ClusterStrengthDown3m *float64 `json:"cluster_strength_down_3m,omitempty"`
	LiqRiskUp3m           *int     `json:"liq_risk_up_3m,omitempty"`
	LiqRiskDown3m         *int     `json:"liq_risk_down_3m,omitempty"`
	PreferDirection3m     *string  `json:"prefer_direction_3m,omitempty"`
	BucketUsd3m           *float64 `json:"bucket_usd_3m,omitempty"`
	EventCountLiq3m       *int     `json:"event_count_liq_3m,omitempty"`
	Atr3m                 *float64 `json:"atr_3m,omitempty"`
	ConfidenceLiq3m       *float64 `json:"confidence_liq_3m,omitempty"`

	// Optional confirmation TF (15m)
	CVDNotionalZ15mShort *float64 `json:"cvd_notional_z_15m_short,omitempty"`
	CVDNotionalZ15mLong  *float64 `json:"cvd_notional_z_15m_long,omitempty"`
	ImbNotionalZ15mShort *float64 `json:"imb_notional_z_15m_short,omitempty"`
	ImbNotionalZ15mLong  *float64 `json:"imb_notional_z_15m_long,omitempty"`
	TBRNotional15m       *float64 `json:"tbr_notional_15m,omitempty"`
	DivBearMid15m        *int     `json:"div_bear_mid_15m,omitempty"`
	DivBullMid15m        *int     `json:"div_bull_mid_15m,omitempty"`
	DistUpAtr15m         *float64 `json:"dist_up_atr_15m,omitempty"`
	DistDnAtr15m         *float64 `json:"dist_dn_atr_15m,omitempty"`
	PreferDirection15m   *string  `json:"prefer_direction_15m,omitempty"`
	ConfidenceCVD15m     *float64 `json:"confidence_cvd_15m,omitempty"`
	ConfidenceLiq15m     *float64 `json:"confidence_liq_15m,omitempty"`
}
