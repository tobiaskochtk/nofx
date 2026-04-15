package selfhostedai500

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	binanceFuturesBaseURL = "https://fapi.binance.com"
)

type binanceExchangeInfoResponse struct {
	Symbols []struct {
		Symbol       string `json:"symbol"`
		Status       string `json:"status"`
		ContractType string `json:"contractType"`
		QuoteAsset   string `json:"quoteAsset"`
	} `json:"symbols"`
}

type binancePremiumIndex struct {
	Symbol          string `json:"symbol"`
	MarkPrice       string `json:"markPrice"`
	IndexPrice      string `json:"indexPrice"`
	LastFundingRate string `json:"lastFundingRate"`
}

type binanceTicker24H struct {
	Symbol      string `json:"symbol"`
	LastPrice   string `json:"lastPrice"`
	OpenPrice   string `json:"openPrice"`
	QuoteVolume string `json:"quoteVolume"`
	Volume      string `json:"volume"`
}

type binanceBookTicker struct {
	Symbol   string `json:"symbol"`
	BidPrice string `json:"bidPrice"`
	AskPrice string `json:"askPrice"`
}

type binanceOpenInterestResponse struct {
	Symbol       string `json:"symbol"`
	OpenInterest string `json:"openInterest"`
}

func fetchBinanceUniverseSnapshots(ctx context.Context, limit int) ([]rawUniverseSnapshot, error) {
	client := newUniverseHTTPClient()

	var exchangeInfo binanceExchangeInfoResponse
	if err := httpJSONGet(ctx, client, binanceFuturesBaseURL+"/fapi/v1/exchangeInfo", &exchangeInfo); err != nil {
		return nil, fmt.Errorf("exchange info: %w", err)
	}

	var tickerRows []binanceTicker24H
	if err := httpJSONGet(ctx, client, binanceFuturesBaseURL+"/fapi/v1/ticker/24hr", &tickerRows); err != nil {
		return nil, fmt.Errorf("24h ticker: %w", err)
	}

	var premiumRows []binancePremiumIndex
	if err := httpJSONGet(ctx, client, binanceFuturesBaseURL+"/fapi/v1/premiumIndex", &premiumRows); err != nil {
		premiumRows = nil
	}

	var bookRows []binanceBookTicker
	if err := httpJSONGet(ctx, client, binanceFuturesBaseURL+"/fapi/v1/ticker/bookTicker", &bookRows); err != nil {
		bookRows = nil
	}

	tradable := make(map[string]bool)
	for _, item := range exchangeInfo.Symbols {
		if item.Status != "TRADING" {
			continue
		}
		if item.ContractType != "PERPETUAL" {
			continue
		}
		if item.QuoteAsset != "USDT" {
			continue
		}
		tradable[item.Symbol] = true
	}

	premiumBySymbol := make(map[string]binancePremiumIndex, len(premiumRows))
	for _, item := range premiumRows {
		premiumBySymbol[item.Symbol] = item
	}

	bookBySymbol := make(map[string]binanceBookTicker, len(bookRows))
	for _, item := range bookRows {
		bookBySymbol[item.Symbol] = item
	}

	selected := make([]binanceTicker24H, 0, len(tickerRows))
	for _, item := range tickerRows {
		if !tradable[item.Symbol] {
			continue
		}
		selected = append(selected, item)
	}

	sort.Slice(selected, func(i, j int) bool {
		left := parseFloat(selected[i].QuoteVolume)
		right := parseFloat(selected[j].QuoteVolume)
		if left == right {
			return selected[i].Symbol < selected[j].Symbol
		}
		return left > right
	})

	selectionLimit := perExchangeSelectionLimit(limit)
	if len(selected) > selectionLimit {
		selected = selected[:selectionLimit]
	}

	openInterestBySymbol := fetchBinanceOpenInterestMap(ctx, client, selected, 12)
	result := make([]rawUniverseSnapshot, 0, len(selected))
	for _, ticker := range selected {
		price := parseFloat(ticker.LastPrice)
		if price <= 0 {
			continue
		}

		premium := premiumBySymbol[ticker.Symbol]
		markPrice := parseFloat(premium.MarkPrice)
		indexPrice := parseFloat(premium.IndexPrice)
		premiumRate := 0.0
		if markPrice > 0 && indexPrice > 0 {
			premiumRate = (markPrice - indexPrice) / indexPrice
		}

		book := bookBySymbol[ticker.Symbol]
		spreadBps := computeSpreadBps([]string{book.BidPrice, book.AskPrice}, price)
		openInterest := openInterestBySymbol[ticker.Symbol]

		result = append(result, rawUniverseSnapshot{
			Symbol:        ticker.Symbol,
			Sources:       []string{"binance"},
			Price:         price,
			PrevDayPrice:  parseFloat(ticker.OpenPrice),
			OpenInterest:  openInterest,
			Volume24H:     parseFloat(ticker.QuoteVolume),
			VolumeBase24H: parseFloat(ticker.Volume),
			Funding:       parseFloat(premium.LastFundingRate),
			Premium:       premiumRate,
			SpreadBps:     spreadBps,
			IsAvailable:   price > 0 && parseFloat(ticker.QuoteVolume) > 0,
		})
	}
	return result, nil
}

func fetchBinanceOpenInterestMap(ctx context.Context, client *http.Client, tickers []binanceTicker24H, concurrency int) map[string]float64 {
	result := make(map[string]float64, len(tickers))
	if len(tickers) == 0 {
		return result
	}
	if concurrency < 1 {
		concurrency = 1
	}

	jobs := make(chan binanceTicker24H)
	var wg sync.WaitGroup
	var mu sync.Mutex

	worker := func() {
		defer wg.Done()
		for ticker := range jobs {
			var response binanceOpenInterestResponse
			requestURL := fmt.Sprintf("%s/fapi/v1/openInterest?symbol=%s", binanceFuturesBaseURL, ticker.Symbol)
			if err := httpJSONGet(ctx, client, requestURL, &response); err != nil {
				continue
			}
			value := parseFloat(response.OpenInterest)
			if value <= 0 {
				continue
			}

			mu.Lock()
			result[ticker.Symbol] = value
			mu.Unlock()
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}
	for _, ticker := range tickers {
		jobs <- ticker
	}
	close(jobs)
	wg.Wait()
	return result
}

func fetchBinanceBootstrapPriceRef(ctx context.Context, symbol string) (bootstrapPriceRef, bool) {
	type klineRow []interface{}

	client := newUniverseHTTPClient()
	requestURL := fmt.Sprintf("%s/fapi/v1/klines?symbol=%s&interval=1h&limit=5", binanceFuturesBaseURL, normalizePair(symbol))
	var rows []klineRow
	if err := httpJSONGet(ctx, client, requestURL, &rows); err != nil {
		return bootstrapPriceRef{}, false
	}
	if len(rows) == 0 {
		return bootstrapPriceRef{}, false
	}

	closeAt := func(row klineRow) float64 {
		if len(row) < 5 {
			return 0
		}
		if value, ok := row[4].(string); ok {
			return parseFloat(value)
		}
		return 0
	}

	latestPrice := closeAt(rows[len(rows)-1])
	if latestPrice <= 0 {
		return bootstrapPriceRef{}, false
	}

	ref := bootstrapPriceRef{FetchedAt: time.Now().UTC()}
	if len(rows) >= 2 {
		ref.Price1H = closeAt(rows[len(rows)-2])
	}
	if len(rows) >= 5 {
		ref.Price4H = closeAt(rows[0])
	} else if len(rows) >= 4 {
		ref.Price4H = closeAt(rows[len(rows)-4])
	}
	if ref.Price1H <= 0 {
		ref.Price1H = latestPrice
	}
	if ref.Price4H <= 0 {
		ref.Price4H = ref.Price1H
	}
	return ref, true
}
