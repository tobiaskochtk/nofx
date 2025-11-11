package derivsv1

import (
	"log"
	"sort"
	"strings"
	"time"
)

const hourMillis = int64(time.Hour / time.Millisecond)

// OIHistory exposes aligned hourly OI and price series per symbol.
type OIHistory interface {
	GetOHLC(symbol string, hours int) (oi []float64, price []float64, err error)
	SourceStatus(symbol string) string
}

// FundingHistory provides latest funding rates plus historical windows.
type FundingHistory interface {
	LatestRates(symbol string) (map[string]float64, error)
	History(symbol string, intervals int) (map[string][]float64, error)
	SourceStatus(symbol string) string
}

// BasisHistory supplies mark/index history for basis calculations.
type BasisHistory interface {
	Latest(symbol string) (map[string]BasisPoint, error)
	History(symbol string, hours int) (map[string][]BasisPoint, error)
	SourceStatus(symbol string) string
}

// CacheOIHistory implements OIHistory on top of a generic cache store.
type CacheOIHistory struct {
	store Store
}

func NewCacheOIHistory(store Store) *CacheOIHistory {
	return &CacheOIHistory{store: store}
}

// CacheFundingHistory implements FundingHistory.
type CacheFundingHistory struct {
	store Store
}

func NewCacheFundingHistory(store Store) *CacheFundingHistory {
	return &CacheFundingHistory{store: store}
}

// CacheBasisHistory implements BasisHistory.
type CacheBasisHistory struct {
	store Store
}

func NewCacheBasisHistory(store Store) *CacheBasisHistory {
	return &CacheBasisHistory{store: store}
}

func bucketTimestamp(ts int64) int64 {
	if ts <= 0 {
		return ts
	}
	return (ts / hourMillis) * hourMillis
}

func cutoffTime(hours int) time.Time {
	if hours <= 0 {
		return time.Unix(0, 0)
	}
	return time.Now().Add(-time.Duration(hours) * time.Hour)
}

func aggregateMedian(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 0 {
		return (values[mid-1] + values[mid]) / 2, true
	}
	return values[mid], true
}

func aggregateOI(samples map[string][]OICacheSample, cutoff time.Time) map[int64]float64 {
	aggregated := make(map[int64][]float64)
	cutoffMs := cutoff.UnixMilli()
	for _, venueSamples := range samples {
		for _, sample := range venueSamples {
			if sample.Timestamp < cutoffMs {
				continue
			}
			bucket := bucketTimestamp(sample.Timestamp)
			aggregated[bucket] = append(aggregated[bucket], sample.Value)
		}
	}

	result := make(map[int64]float64, len(aggregated))
	for ts, values := range aggregated {
		if median, ok := aggregateMedian(values); ok {
			result[ts] = median
		}
	}
	return result
}

func aggregatePrices(samples map[string][]BasisCacheSample, cutoff time.Time) map[int64]float64 {
	aggregated := make(map[int64][]float64)
	cutoffMs := cutoff.UnixMilli()
	for _, venueSamples := range samples {
		for _, sample := range venueSamples {
			if sample.Timestamp < cutoffMs || sample.Mark <= 0 {
				continue
			}
			bucket := bucketTimestamp(sample.Timestamp)
			aggregated[bucket] = append(aggregated[bucket], sample.Mark)
		}
	}

	result := make(map[int64]float64, len(aggregated))
	for ts, values := range aggregated {
		if median, ok := aggregateMedian(values); ok {
			result[ts] = median
		}
	}
	return result
}

func mapKeysSorted(m map[int64]float64) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func joinStatus(status map[string]string) string {
	if len(status) == 0 {
		return ""
	}
	keys := make([]string, 0, len(status))
	for k := range status {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+":"+status[k])
	}
	return strings.Join(parts, "|")
}

// GetOHLC implements OIHistory.
func (h *CacheOIHistory) GetOHLC(symbol string, hours int) ([]float64, []float64, error) {
	oiCache, err := h.store.LoadOI(symbol)
	if err != nil || oiCache == nil {
		log.Printf("[GetOHLC] %s: LoadOI returned err=%v, cache=nil=%v", symbol, err, oiCache == nil)
		return nil, nil, err
	}
	log.Printf("[GetOHLC] %s: Loaded OI cache with %d venue samples", symbol, len(oiCache.Samples))
	
	basisCache, err := h.store.LoadBasis(symbol)
	if err != nil || basisCache == nil {
		log.Printf("[GetOHLC] %s: LoadBasis returned err=%v, cache=nil=%v", symbol, err, basisCache == nil)
		return nil, nil, err
	}
	log.Printf("[GetOHLC] %s: Loaded Basis cache with %d samples", symbol, len(basisCache.Samples))

	cutoff := cutoffTime(hours)
	log.Printf("[GetOHLC] %s: cutoff=%v (hours=%d)", symbol, cutoff, hours)
	
	oiSeries := aggregateOI(oiCache.Samples, cutoff)
	log.Printf("[GetOHLC] %s: aggregateOI returned %d hourly buckets", symbol, len(oiSeries))
	
	priceSeries := aggregatePrices(basisCache.Samples, cutoff)
	log.Printf("[GetOHLC] %s: aggregatePrices returned %d hourly buckets", symbol, len(priceSeries))

	keys := mapKeysSorted(oiSeries)
	var oiValues []float64
	var priceValues []float64
	for _, ts := range keys {
		price, ok := priceSeries[ts]
		if !ok {
			continue
		}
		oiValues = append(oiValues, oiSeries[ts])
		priceValues = append(priceValues, price)
	}
	log.Printf("[GetOHLC] %s: aligned %d timestamps (out of %d OI buckets)", symbol, len(oiValues), len(oiSeries))
	return oiValues, priceValues, nil
}

// SourceStatus returns the concatenated venue states for OI caches.
func (h *CacheOIHistory) SourceStatus(symbol string) string {
	if cache, err := h.store.LoadOI(symbol); err == nil && cache != nil {
		return joinStatus(cache.Status)
	}
	return ""
}

// LatestRates implements FundingHistory.LatestRates.
func (h *CacheFundingHistory) LatestRates(symbol string) (map[string]float64, error) {
	cache, err := h.store.LoadFunding(symbol)
	if err != nil || cache == nil {
		return nil, err
	}
	latest := make(map[string]float64)
	for venue, samples := range cache.Samples {
		if len(samples) == 0 {
			continue
		}
		latest[venue] = samples[len(samples)-1].Rate
	}
	return latest, nil
}

// History implements FundingHistory.History.
func (h *CacheFundingHistory) History(symbol string, intervals int) (map[string][]float64, error) {
	cache, err := h.store.LoadFunding(symbol)
	if err != nil || cache == nil {
		return nil, err
	}
	history := make(map[string][]float64)
	for venue, samples := range cache.Samples {
		if len(samples) == 0 {
			continue
		}
		start := len(samples) - intervals
		if start < 0 {
			start = 0
		}
		slice := samples[start:]
		seq := make([]float64, 0, len(slice))
		for _, sample := range slice {
			seq = append(seq, sample.Rate)
		}
		if len(seq) > 0 {
			history[venue] = seq
		}
	}
	return history, nil
}

// SourceStatus returns funding venue statuses.
func (h *CacheFundingHistory) SourceStatus(symbol string) string {
	if cache, err := h.store.LoadFunding(symbol); err == nil && cache != nil {
		return joinStatus(cache.Status)
	}
	return ""
}

// Latest implements BasisHistory.Latest.
func (h *CacheBasisHistory) Latest(symbol string) (map[string]BasisPoint, error) {
	cache, err := h.store.LoadBasis(symbol)
	if err != nil || cache == nil {
		return nil, err
	}
	latest := make(map[string]BasisPoint)
	for venue, samples := range cache.Samples {
		if len(samples) == 0 {
			continue
		}
		sample := samples[len(samples)-1]
		latest[venue] = BasisPoint{
			Timestamp: time.UnixMilli(sample.Timestamp),
			Mark:      sample.Mark,
			Index:     sample.Index,
		}
	}
	return latest, nil
}

// History implements BasisHistory.History.
func (h *CacheBasisHistory) History(symbol string, hours int) (map[string][]BasisPoint, error) {
	cache, err := h.store.LoadBasis(symbol)
	if err != nil || cache == nil {
		return nil, err
	}
	cutoff := cutoffTime(hours).UnixMilli()
	history := make(map[string][]BasisPoint)
	for venue, samples := range cache.Samples {
		filtered := make([]BasisPoint, 0, len(samples))
		for _, sample := range samples {
			if hours > 0 && sample.Timestamp < cutoff {
				continue
			}
			filtered = append(filtered, BasisPoint{
				Timestamp: time.UnixMilli(sample.Timestamp),
				Mark:      sample.Mark,
				Index:     sample.Index,
			})
		}
		if len(filtered) > 0 {
			history[venue] = filtered
		}
	}
	return history, nil
}

// SourceStatus returns basis venue statuses.
func (h *CacheBasisHistory) SourceStatus(symbol string) string {
	if cache, err := h.store.LoadBasis(symbol); err == nil && cache != nil {
		return joinStatus(cache.Status)
	}
	return ""
}
