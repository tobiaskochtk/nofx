package derivsv1

// BasisFeatureConfig aggregates dependencies for basis normalization.
type BasisFeatureConfig struct {
	Symbol      string
	History     BasisHistory
	WindowHours int
	Alpha       float64
}

// ComputeBasisFeatures derives latest median basis and a 14d rolling z-score.
func ComputeBasisFeatures(cfg BasisFeatureConfig) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	if cfg.History == nil {
		return result, nil
	}

	latest, err := cfg.History.Latest(cfg.Symbol)
	if err != nil {
		return nil, err
	}
	history, err := cfg.History.History(cfg.Symbol, cfg.WindowHours)
	if err != nil {
		return nil, err
	}
	result["basis_source_status"] = cfg.History.SourceStatus(cfg.Symbol)

	if len(latest) == 0 {
		return result, nil
	}

	latestPct := make([]float64, 0, len(latest))
	for _, point := range latest {
		if pct, ok := basisPct(point.Mark, point.Index); ok {
			latestPct = append(latestPct, pct)
		}
	}
	if median, ok := aggregateMedian(latestPct); ok {
		result["basis_pct"] = median
	}

	zValues := make([]float64, 0, len(history))
	for _, seq := range history {
		series := make([]float64, 0, len(seq))
		for _, point := range seq {
			if pct, ok := basisPct(point.Mark, point.Index); ok {
				series = append(series, pct)
			}
		}
		if len(series) == 0 {
			continue
		}
		if z := ewmaZ(series, cfg.Alpha); z != nil {
			zValues = append(zValues, *z)
		}
	}
	if medianZ, ok := aggregateMedian(zValues); ok {
		result["basis_z_14d"] = medianZ
	}

	return result, nil
}

func basisPct(mark, index float64) (float64, bool) {
	if index <= 0 {
		return 0, false
	}
	return (mark - index) / index, true
}
