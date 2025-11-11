package derivsv1

import (
    "math"

    "nofx/internal/features/derivs/v1/rolling"
)

const epsilon = 1e-9

// OIFeatureConfig bundles the knobs needed to compute the OI feature set.
type OIFeatureConfig struct {
    Symbol          string
    History         OIHistory
    WindowHours     int
    CorrWindowHours int
    Alpha           float64
    MinSamples      int
}

// ComputeOIFeatures derives normalized OI deltas, divergence state, and OI↔price correlation.
func ComputeOIFeatures(cfg OIFeatureConfig) (map[string]interface{}, error) {
    result := map[string]interface{}{}
    if cfg.History == nil {
        return result, nil
    }

    oiSeries, priceSeries, err := cfg.History.GetOHLC(cfg.Symbol, cfg.WindowHours)
    if err != nil {
        return nil, err
    }
    result["oi_source_status"] = cfg.History.SourceStatus(cfg.Symbol)

    if len(oiSeries) == 0 || len(priceSeries) == 0 || len(oiSeries) < cfg.MinSamples {
        return result, nil
    }

    oiDelta := percentDelta(oiSeries)
    if oiDelta != nil {
        result["oi_delta_1h_pct"] = oiDelta
    }
    priceRet := percentDelta(priceSeries)
    if priceRet != nil {
        state := classifyDivergence(oiDelta, priceRet)
        if state != nil {
            result["oi_price_div"] = state
        }
    }
    if corr := correlation24h(oiSeries, priceSeries, cfg.CorrWindowHours); corr != nil {
        result["oi_price_corr_24h"] = corr
    }
    if z := ewmaZ(oiSeries, cfg.Alpha); z != nil {
        result["oi_z_7d"] = z
    }

    return result, nil
}

func percentDelta(series []float64) *float64 {
    if len(series) < 2 {
        return nil
    }
    prev := series[len(series)-2]
    curr := series[len(series)-1]
    denom := math.Max(math.Abs(prev), epsilon)
    delta := (curr - prev) / denom
    return &delta
}

func classifyDivergence(oiDelta, priceDelta *float64) *string {
    if oiDelta == nil || priceDelta == nil {
        return nil
    }
    product := (*oiDelta) * (*priceDelta)
    var state string
    switch {
    case product > 0:
        state = "confirming"
    case product < 0:
        state = "divergent"
    default:
        state = "flat"
    }
    return &state
}

func correlation24h(oi, price []float64, lookback int) *float64 {
    if len(oi) < 2 || len(price) < 2 {
        return nil
    }
    deltasOI := differences(oi)
    deltasPrice := differences(price)
    if len(deltasOI) < lookback {
        return nil
    }
    start := len(deltasOI) - lookback
    if start < 0 {
        start = 0
    }
    sliceOI := deltasOI[start:]
    slicePrice := deltasPrice[len(deltasPrice)-len(sliceOI):]
    if len(sliceOI) < 2 {
        return nil
    }
    corr := pearson(sliceOI, slicePrice)
    return &corr
}

func differences(series []float64) []float64 {
    out := make([]float64, 0, len(series)-1)
    for i := 1; i < len(series); i++ {
        out = append(out, series[i]-series[i-1])
    }
    return out
}

func pearson(a, b []float64) float64 {
    if len(a) != len(b) || len(a) == 0 {
        return 0
    }
    var sumA, sumB float64
    for i := range a {
        sumA += a[i]
        sumB += b[i]
    }
    meanA := sumA / float64(len(a))
    meanB := sumB / float64(len(b))

    var num, denomA, denomB float64
    for i := range a {
        da := a[i] - meanA
        db := b[i] - meanB
        num += da * db
        denomA += da * da
        denomB += db * db
    }
    denom := math.Sqrt(denomA * denomB)
    if denom == 0 {
        return 0
    }
    return num / denom
}

func ewmaZ(series []float64, alpha float64) *float64 {
    if len(series) == 0 || alpha <= 0 {
        return nil
    }
    stats := &rolling.EWStats{Alpha: alpha}
    for i, value := range series {
        if i == len(series)-1 {
            z := stats.Z(value)
            stats.Update(value)
            return &z
        }
        stats.Update(value)
    }
    zero := 0.0
    return &zero
}
