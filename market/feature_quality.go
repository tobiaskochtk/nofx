package market

import "time"

const (
	FeatureKeyF4 = "f4"
	FeatureKeyF5 = "f5"
	FeatureKeyF6 = "f6"
	FeatureKeyF7 = "f7"
)

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

// TouchFeatureStats refreshes all recorded feature timestamps to the provided time.
// This is used after a full market-fetch cycle so earlier symbols in the batch do not
// age out while later symbols are still computing.
func (d *Data) TouchFeatureStats(ts time.Time) {
	if d == nil || len(d.FeatureStats) == 0 {
		return
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	for key, stat := range d.FeatureStats {
		stat.UpdatedAt = ts
		d.FeatureStats[key] = stat
	}
}
