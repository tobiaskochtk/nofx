package types

// DerivsFeatures captures normalized derivatives flow metrics for a symbol snapshot.
type DerivsFeatures struct {
    OIDelta1hPct        *float64 `json:"oi_delta_1h_pct,omitempty"`
    OIZ7d               *float64 `json:"oi_z_7d,omitempty"`
    OIPriceDiv          *string  `json:"oi_price_div,omitempty"`
    OIPriceCorr24h      *float64 `json:"oi_price_corr_24h,omitempty"`
    OISourceStatus      string   `json:"oi_source_status"`

    FundingLatestBps      *float64 `json:"funding_latest_bps,omitempty"`
    FundingMedianZ7d      *float64 `json:"funding_median_z_7d,omitempty"`
    FundingDispersionBps  *float64 `json:"funding_dispersion_bps,omitempty"`
    FundingSourceStatus   string   `json:"funding_source_status"`

    BasisPct            *float64 `json:"basis_pct,omitempty"`
    BasisZ14d           *float64 `json:"basis_z_14d,omitempty"`
    BasisSourceStatus   string   `json:"basis_source_status"`
}
