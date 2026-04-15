package selfhostedai500

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/provider/hyperliquid"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const hyperliquidInfoURL = "https://api.hyperliquid.xyz/info"

type universeAsset struct {
	Name       string `json:"name"`
	IsDelisted bool   `json:"isDelisted,omitempty"`
}

type assetContext struct {
	Funding      string   `json:"funding"`
	OpenInterest string   `json:"openInterest"`
	PrevDayPx    string   `json:"prevDayPx"`
	DayNtlVlm    string   `json:"dayNtlVlm"`
	DayBaseVlm   string   `json:"dayBaseVlm"`
	Premium      *string  `json:"premium"`
	OraclePx     string   `json:"oraclePx"`
	MarkPx       string   `json:"markPx"`
	MidPx        *string  `json:"midPx"`
	ImpactPxs    []string `json:"impactPxs"`
}

type rawUniverseSnapshot struct {
	Symbol        string
	Sources       []string
	Price         float64
	PrevDayPrice  float64
	RefPrice1H    float64
	RefPrice4H    float64
	OpenInterest  float64
	Volume24H     float64
	VolumeBase24H float64
	Funding       float64
	Premium       float64
	SpreadBps     float64
	IsAvailable   bool
}

func fetchHyperliquidUniverseSnapshots(ctx context.Context, limit int) ([]rawUniverseSnapshot, error) {
	reqBody := []byte(`{"type":"metaAndAssetCtxs"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hyperliquidInfoURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hyperliquid metaAndAssetCtxs status %d: %s", resp.StatusCode, string(body))
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if len(raw) < 2 {
		return nil, fmt.Errorf("unexpected metaAndAssetCtxs response")
	}

	var meta struct {
		Universe []universeAsset `json:"universe"`
	}
	if err := json.Unmarshal(raw[0], &meta); err != nil {
		return nil, err
	}

	var contexts []assetContext
	if err := json.Unmarshal(raw[1], &contexts); err != nil {
		return nil, err
	}

	snapshots := make([]rawUniverseSnapshot, 0, len(meta.Universe))
	for i, asset := range meta.Universe {
		if i >= len(contexts) {
			break
		}
		if asset.IsDelisted {
			continue
		}

		ctxRow := contexts[i]
		price := firstPositiveFloat(stringValue(ctxRow.MidPx), ctxRow.MarkPx, ctxRow.OraclePx)
		if price <= 0 {
			continue
		}

		prevDayPrice := parseFloat(ctxRow.PrevDayPx)
		openInterest := parseFloat(ctxRow.OpenInterest)
		volume24H := parseFloat(ctxRow.DayNtlVlm)
		volumeBase24H := parseFloat(ctxRow.DayBaseVlm)
		funding := parseFloat(ctxRow.Funding)
		premium := parseFloat(stringValue(ctxRow.Premium))
		spreadBps := computeSpreadBps(ctxRow.ImpactPxs, price)

		snapshots = append(snapshots, rawUniverseSnapshot{
			Symbol:        asset.Name,
			Sources:       []string{"hyperliquid"},
			Price:         price,
			PrevDayPrice:  prevDayPrice,
			OpenInterest:  openInterest,
			Volume24H:     volume24H,
			VolumeBase24H: volumeBase24H,
			Funding:       funding,
			Premium:       premium,
			SpreadBps:     spreadBps,
			IsAvailable:   volume24H > 0 && price > 0,
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Volume24H == snapshots[j].Volume24H {
			return snapshots[i].OpenInterest > snapshots[j].OpenInterest
		}
		return snapshots[i].Volume24H > snapshots[j].Volume24H
	})

	if limit > 0 && len(snapshots) > limit {
		snapshots = snapshots[:limit]
	}
	return snapshots, nil
}

func fetchHyperliquidBootstrapPriceRef(ctx context.Context, symbol string) (bootstrapPriceRef, bool) {
	client := hyperliquid.NewClient()
	coin := strings.TrimSuffix(strings.ToUpper(symbol), "USDT")
	candles, err := client.GetCandles(ctx, coin, "1h", 5)
	if err != nil || len(candles) == 0 {
		return bootstrapPriceRef{}, false
	}

	latestPrice := parseFloat(candles[len(candles)-1].Close)
	if latestPrice <= 0 {
		latestPrice = parseFloat(candles[len(candles)-1].Open)
	}
	if latestPrice <= 0 {
		return bootstrapPriceRef{}, false
	}

	ref := bootstrapPriceRef{FetchedAt: time.Now().UTC()}
	if len(candles) >= 2 {
		ref.Price1H = parseFloat(candles[len(candles)-2].Close)
	}
	if len(candles) >= 5 {
		ref.Price4H = parseFloat(candles[0].Close)
	} else if len(candles) >= 4 {
		ref.Price4H = parseFloat(candles[len(candles)-4].Close)
	}
	if ref.Price1H <= 0 {
		ref.Price1H = latestPrice
	}
	if ref.Price4H <= 0 {
		ref.Price4H = ref.Price1H
	}
	return ref, true
}

func fetchBootstrapPriceRefs(ctx context.Context, snapshots []rawUniverseSnapshot, concurrency int, exchanges []string) map[string]bootstrapPriceRef {
	results := make(map[string]bootstrapPriceRef)
	if len(snapshots) == 0 {
		return results
	}

	jobs := make(chan rawUniverseSnapshot)
	var mu sync.Mutex
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for snapshot := range jobs {
			ref, ok := fetchBootstrapPriceRefForSnapshot(ctx, snapshot, exchanges)
			if !ok {
				continue
			}

			mu.Lock()
			results[snapshot.Symbol] = ref
			mu.Unlock()
		}
	}

	if concurrency < 1 {
		concurrency = 1
	}
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}
	for _, snapshot := range snapshots {
		jobs <- snapshot
	}
	close(jobs)
	wg.Wait()
	return results
}

func computeSpreadBps(impactPxs []string, price float64) float64 {
	if len(impactPxs) < 2 || price <= 0 {
		return 0
	}
	bid := parseFloat(impactPxs[0])
	ask := parseFloat(impactPxs[1])
	if bid <= 0 || ask <= 0 || ask < bid {
		return 0
	}
	return ((ask - bid) / price) * 10000
}

func firstPositiveFloat(values ...string) float64 {
	for _, value := range values {
		parsed := parseFloat(value)
		if parsed > 0 {
			return parsed
		}
	}
	return 0
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func parseFloat(value string) float64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
