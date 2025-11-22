package market

import "time"

// FeatureQuality returns quality stats for a feature key if recorded.
func (d *Data) FeatureQuality(key string) (FeatureStat, bool) {
	if d == nil || d.FeatureStats == nil {
		return FeatureStat{}, false
	}
	stat, ok := d.FeatureStats[key]
	return stat, ok
}

// markFeatureFresh records that a feature was updated with the provided coverage.
func (d *Data) markFeatureFresh(key string, coverage float64, ts time.Time) {
	if d == nil {
		return
	}
	if d.FeatureStats == nil {
		d.FeatureStats = make(map[string]FeatureStat)
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	d.FeatureStats[key] = FeatureStat{
		Coverage:  coverage,
		UpdatedAt: ts,
	}
}
