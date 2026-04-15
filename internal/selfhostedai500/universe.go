package selfhostedai500

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

var supportedUniverseExchanges = map[string]struct{}{
	"hyperliquid": {},
	"binance":     {},
	"bybit":       {},
}

func normalizeUniverseExchanges(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]bool)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.ToLower(strings.TrimSpace(part))
		if value == "" {
			continue
		}
		if _, ok := supportedUniverseExchanges[value]; !ok {
			continue
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		return []string{"hyperliquid", "binance", "bybit"}
	}
	return result
}

func fetchUniverseSnapshots(ctx context.Context, exchanges []string, limit int) ([]rawUniverseSnapshot, error) {
	enabled := append([]string(nil), exchanges...)
	if len(enabled) == 0 {
		enabled = []string{"hyperliquid", "binance", "bybit"}
	}

	type result struct {
		exchange  string
		snapshots []rawUniverseSnapshot
		err       error
	}

	resultsCh := make(chan result, len(enabled))
	var wg sync.WaitGroup
	for _, exchange := range enabled {
		wg.Add(1)
		go func(exchange string) {
			defer wg.Done()

			var snapshots []rawUniverseSnapshot
			var err error
			switch exchange {
			case "hyperliquid":
				snapshots, err = fetchHyperliquidUniverseSnapshots(ctx, limit)
			case "binance":
				snapshots, err = fetchBinanceUniverseSnapshots(ctx, limit)
			case "bybit":
				snapshots, err = fetchBybitUniverseSnapshots(ctx, limit)
			default:
				err = fmt.Errorf("unsupported universe exchange %q", exchange)
			}
			resultsCh <- result{exchange: exchange, snapshots: snapshots, err: err}
		}(exchange)
	}

	wg.Wait()
	close(resultsCh)

	combined := make([]rawUniverseSnapshot, 0, limit*len(enabled))
	errors := make([]string, 0)
	for item := range resultsCh {
		if item.err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", item.exchange, item.err))
			continue
		}
		log.Printf("[selfhosted-ai500] universe source %s ready: %d symbols", item.exchange, len(item.snapshots))
		combined = append(combined, item.snapshots...)
	}

	if len(combined) == 0 {
		return nil, fmt.Errorf("all universe sources failed: %s", strings.Join(errors, "; "))
	}

	aggregated := aggregateUniverseSnapshots(combined, limit)
	if len(errors) > 0 {
		log.Printf("[selfhosted-ai500] partial universe fetch failure: %s", strings.Join(errors, "; "))
	}
	return aggregated, nil
}

func aggregateUniverseSnapshots(snapshots []rawUniverseSnapshot, limit int) []rawUniverseSnapshot {
	type aggregate struct {
		Symbol        string
		Sources       map[string]bool
		Price         weightedField
		PrevDayPrice  weightedField
		RefPrice1H    weightedField
		RefPrice4H    weightedField
		Funding       weightedField
		Premium       weightedField
		SpreadBps     weightedField
		OpenInterest  float64
		Volume24H     float64
		VolumeBase24H float64
		IsAvailable   bool
	}

	grouped := make(map[string]*aggregate)
	for _, snapshot := range snapshots {
		symbol := normalizePair(snapshot.Symbol)
		item := grouped[symbol]
		if item == nil {
			item = &aggregate{
				Symbol:  symbol,
				Sources: make(map[string]bool),
			}
			grouped[symbol] = item
		}

		for _, source := range snapshot.Sources {
			item.Sources[source] = true
		}

		weight := snapshot.Volume24H
		if weight <= 0 {
			weight = snapshot.OpenInterest * snapshot.Price
		}
		if weight <= 0 {
			weight = 1
		}

		item.Price.Add(snapshot.Price, weight)
		item.PrevDayPrice.Add(snapshot.PrevDayPrice, weight)
		item.RefPrice1H.Add(snapshot.RefPrice1H, weight)
		item.RefPrice4H.Add(snapshot.RefPrice4H, weight)
		item.Funding.Add(snapshot.Funding, weight)
		item.Premium.Add(snapshot.Premium, weight)
		item.SpreadBps.Add(snapshot.SpreadBps, weight)
		item.OpenInterest += snapshot.OpenInterest
		item.Volume24H += snapshot.Volume24H
		item.VolumeBase24H += snapshot.VolumeBase24H
		item.IsAvailable = item.IsAvailable || snapshot.IsAvailable
	}

	result := make([]rawUniverseSnapshot, 0, len(grouped))
	for _, item := range grouped {
		result = append(result, rawUniverseSnapshot{
			Symbol:        item.Symbol,
			Sources:       sortedSourceList(item.Sources),
			Price:         item.Price.Value(),
			PrevDayPrice:  item.PrevDayPrice.Value(),
			RefPrice1H:    item.RefPrice1H.Value(),
			RefPrice4H:    item.RefPrice4H.Value(),
			OpenInterest:  item.OpenInterest,
			Volume24H:     item.Volume24H,
			VolumeBase24H: item.VolumeBase24H,
			Funding:       item.Funding.Value(),
			Premium:       item.Premium.Value(),
			SpreadBps:     item.SpreadBps.Value(),
			IsAvailable:   item.IsAvailable && item.Price.Value() > 0 && item.Volume24H > 0,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Volume24H == result[j].Volume24H {
			return result[i].OpenInterest > result[j].OpenInterest
		}
		return result[i].Volume24H > result[j].Volume24H
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func fetchBootstrapPriceRefForSnapshot(ctx context.Context, snapshot rawUniverseSnapshot, exchanges []string) (bootstrapPriceRef, bool) {
	for _, exchange := range orderedBootstrapExchanges(snapshot.Sources, exchanges) {
		switch exchange {
		case "binance":
			if ref, ok := fetchBinanceBootstrapPriceRef(ctx, snapshot.Symbol); ok {
				return ref, true
			}
		case "bybit":
			if ref, ok := fetchBybitBootstrapPriceRef(ctx, snapshot.Symbol); ok {
				return ref, true
			}
		case "hyperliquid":
			if ref, ok := fetchHyperliquidBootstrapPriceRef(ctx, snapshot.Symbol); ok {
				return ref, true
			}
		}
	}
	return bootstrapPriceRef{}, false
}

func orderedBootstrapExchanges(primary, configured []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(primary)+len(configured))
	for _, list := range [][]string{primary, configured, []string{"hyperliquid", "binance", "bybit"}} {
		for _, exchange := range list {
			exchange = strings.ToLower(strings.TrimSpace(exchange))
			if _, ok := supportedUniverseExchanges[exchange]; !ok {
				continue
			}
			if !seen[exchange] {
				seen[exchange] = true
				result = append(result, exchange)
			}
		}
	}
	return result
}

func sortedSourceList(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func perExchangeSelectionLimit(limit int) int {
	switch {
	case limit <= 0:
		return 120
	case limit < 60:
		return 60
	case limit > 160:
		return 160
	default:
		return limit
	}
}

func httpJSONGet(ctx context.Context, client *http.Client, requestURL string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, trimResponseBody(body))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return err
	}
	return nil
}

func trimResponseBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}

func newUniverseHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

type weightedField struct {
	sum      float64
	weight   float64
	fallback float64
	hasValue bool
}

func (f *weightedField) Add(value, weight float64) {
	if value <= 0 {
		return
	}
	if !f.hasValue {
		f.fallback = value
		f.hasValue = true
	}
	f.sum += value * weight
	f.weight += weight
}

func (f weightedField) Value() float64 {
	if f.weight > 0 {
		return f.sum / f.weight
	}
	return f.fallback
}
