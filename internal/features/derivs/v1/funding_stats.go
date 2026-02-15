package derivsv1

import "math"

// FundingFeatureConfig defines calculation knobs for funding metrics.
type FundingFeatureConfig struct {
    Symbol     string
    History    FundingHistory
    WindowSize int
}

// ComputeFundingFeatures returns funding median, z-scores and dispersion in basis points.
func ComputeFundingFeatures(cfg FundingFeatureConfig) (map[string]interface{}, error) {
    result := map[string]interface{}{}
    if cfg.History == nil {
        return result, nil
    }

    latest, err := cfg.History.LatestRates(cfg.Symbol)
    if err != nil {
        return nil, err
    }
    history, err := cfg.History.History(cfg.Symbol, cfg.WindowSize)
    if err != nil {
        return nil, err
    }
    result["funding_source_status"] = cfg.History.SourceStatus(cfg.Symbol)

    if len(latest) == 0 {
        return result, nil
    }

    latestBps := make([]float64, 0, len(latest))
    zValues := make([]float64, 0, len(history))
    for venue, rate := range latest {
        latestBps = append(latestBps, rate*1e4)
        if seq, ok := history[venue]; ok {
            if z, ok := fundingZ(seq); ok {
                zValues = append(zValues, z)
            }
        }
    }

    if median, ok := aggregateMedian(latestBps); ok {
        result["funding_latest_bps"] = median
    }
    if medianZ, ok := aggregateMedian(zValues); ok {
        result["funding_median_z_7d"] = medianZ
    }
    if dispersion := stddev(latestBps); dispersion != nil {
        result["funding_dispersion_bps"] = dispersion
    }

    return result, nil
}

func fundingZ(history []float64) (float64, bool) {
    if len(history) == 0 {
        return 0, false
    }
    mean := mean(history)
    variance := 0.0
    for _, value := range history {
        diff := value - mean
        variance += diff * diff
    }
    variance /= float64(len(history))
    if variance == 0 {
        return 0, true
    }
    latest := history[len(history)-1]
    return (latest - mean) / math.Sqrt(variance), true
}

func mean(values []float64) float64 {
    if len(values) == 0 {
        return 0
    }
    sum := 0.0
    for _, v := range values {
        sum += v
    }
    return sum / float64(len(values))
}

func stddev(values []float64) *float64 {
    if len(values) == 0 {
        return nil
    }
    mu := mean(values)
    var sum float64
    for _, v := range values {
        diff := v - mu
        sum += diff * diff
    }
    variance := sum / float64(len(values))
    result := math.Sqrt(variance)
    return &result
}
