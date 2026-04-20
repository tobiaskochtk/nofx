package derivsv1

import "testing"

func TestMergeOICacheSamples_DedupeAndTrim(t *testing.T) {
	existing := []OICacheSample{
		{Timestamp: 1000, Value: 10},
		{Timestamp: 2000, Value: 20},
	}
	incoming := []OICacheSample{
		{Timestamp: 2000, Value: 22},
		{Timestamp: 3000, Value: 30},
	}

	merged := mergeOICacheSamples(existing, incoming, 2)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged samples, got %d", len(merged))
	}
	if merged[0].Timestamp != 2000 || merged[0].Value != 22 {
		t.Fatalf("expected first sample to be deduped ts=2000 value=22, got %+v", merged[0])
	}
	if merged[1].Timestamp != 3000 || merged[1].Value != 30 {
		t.Fatalf("expected second sample ts=3000 value=30, got %+v", merged[1])
	}
}

func TestMergeFundingCacheSamples_Dedupe(t *testing.T) {
	existing := []FundingCacheSample{
		{Timestamp: 1000, Rate: 0.001},
	}
	incoming := []FundingCacheSample{
		{Timestamp: 1000, Rate: 0.002},
		{Timestamp: 2000, Rate: 0.003},
	}

	merged := mergeFundingCacheSamples(existing, incoming, 10)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged samples, got %d", len(merged))
	}
	if merged[0].Rate != 0.002 {
		t.Fatalf("expected deduped rate 0.002 at ts=1000, got %.6f", merged[0].Rate)
	}
}

func TestMergeBasisCacheSamples_Dedupe(t *testing.T) {
	existing := []BasisCacheSample{
		{Timestamp: 1000, Mark: 10, Index: 9},
	}
	incoming := []BasisCacheSample{
		{Timestamp: 1000, Mark: 11, Index: 10},
		{Timestamp: 2000, Mark: 12, Index: 11},
	}

	merged := mergeBasisCacheSamples(existing, incoming, 10)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged samples, got %d", len(merged))
	}
	if merged[0].Mark != 11 || merged[0].Index != 10 {
		t.Fatalf("expected deduped basis at ts=1000 to be replaced, got %+v", merged[0])
	}
}
