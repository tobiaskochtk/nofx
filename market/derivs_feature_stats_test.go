package market

import (
	"testing"
	"time"

	"nofx/pkg/types"
)

func TestRecordDerivsFeatureStats_FullCoverage(t *testing.T) {
	now := time.Now().UTC()
	data := &Data{Symbol: "BTCUSDT"}
	preferLong := "long"
	avwapBias := "bull"
	volRegime := "expanding"
	one := 1
	floatVal := 1.0

	recordDerivsFeatureStats(data, &types.DerivsFeatures{
		CVDNotionalZ3mShort: &floatVal,
		SlopePrice3mShort:   &floatVal,
		ConfidenceCVD3m:     &floatVal,

		DistUpAtr3m:       &floatVal,
		DistDnAtr3m:       &floatVal,
		PreferDirection3m: &preferLong,
		ConfidenceLiq3m:   &floatVal,

		AVWAPUpName3m:     &preferLong,
		AVWAPBias3m:       &avwapBias,
		ConfidenceAVWAP3m: &floatVal,

		BBW3m:           &floatVal,
		RvRatio3m:       &floatVal,
		VolRegime3m:     &volRegime,
		ConfidenceVol3m: &floatVal,
		SqueezeOn3m:     &one,
	}, now)

	for _, key := range []string{FeatureKeyF4, FeatureKeyF5, FeatureKeyF6, FeatureKeyF7} {
		stat, ok := data.FeatureQuality(key)
		if !ok {
			t.Fatalf("expected feature stat for %s", key)
		}
		if stat.Coverage != 1 {
			t.Fatalf("coverage for %s = %.3f, want 1.000", key, stat.Coverage)
		}
		if !stat.UpdatedAt.Equal(now) {
			t.Fatalf("updatedAt for %s = %v, want %v", key, stat.UpdatedAt, now)
		}
	}
}

func TestRecordDerivsFeatureStats_PartialCoverage(t *testing.T) {
	now := time.Now().UTC()
	data := &Data{Symbol: "BTCUSDT"}
	floatVal := 1.0

	recordDerivsFeatureStats(data, &types.DerivsFeatures{
		CVDNotionalZ3mShort: &floatVal,
	}, now)

	statF4, ok := data.FeatureQuality(FeatureKeyF4)
	if !ok {
		t.Fatalf("expected feature stat for %s", FeatureKeyF4)
	}
	if statF4.Coverage <= 0 || statF4.Coverage >= 1 {
		t.Fatalf("coverage for %s = %.3f, want partial coverage", FeatureKeyF4, statF4.Coverage)
	}

	for _, key := range []string{FeatureKeyF5, FeatureKeyF6, FeatureKeyF7} {
		stat, ok := data.FeatureQuality(key)
		if !ok {
			t.Fatalf("expected feature stat for %s", key)
		}
		if stat.Coverage != 0 {
			t.Fatalf("coverage for %s = %.3f, want 0", key, stat.Coverage)
		}
	}
}

func TestTouchFeatureStats(t *testing.T) {
	oldTs := time.Now().UTC().Add(-5 * time.Minute)
	newTs := time.Now().UTC()
	data := &Data{
		Symbol: "BTCUSDT",
		FeatureStats: map[string]FeatureStat{
			FeatureKeyF4: {Coverage: 1, UpdatedAt: oldTs},
			FeatureKeyF5: {Coverage: 0.5, UpdatedAt: oldTs},
		},
	}

	data.TouchFeatureStats(newTs)

	for key, stat := range data.FeatureStats {
		if !stat.UpdatedAt.Equal(newTs) {
			t.Fatalf("updatedAt for %s = %v, want %v", key, stat.UpdatedAt, newTs)
		}
	}
}
