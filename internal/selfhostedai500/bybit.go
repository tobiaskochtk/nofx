package selfhostedai500

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"
)

const bybitMarketBaseURL = "https://api.bybit.com"

type bybitInstrumentsInfoResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Category       string `json:"category"`
		NextPageCursor string `json:"nextPageCursor"`
		List           []struct {
			Symbol       string `json:"symbol"`
			Status       string `json:"status"`
			ContractType string `json:"contractType"`
			QuoteCoin    string `json:"quoteCoin"`
		} `json:"list"`
	} `json:"result"`
}

type bybitTickersResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Category string           `json:"category"`
		List     []bybitTickerRow `json:"list"`
	} `json:"result"`
}

type bybitTickerRow struct {
	Symbol            string `json:"symbol"`
	LastPrice         string `json:"lastPrice"`
	IndexPrice        string `json:"indexPrice"`
	MarkPrice         string `json:"markPrice"`
	PrevPrice24H      string `json:"prevPrice24h"`
	PrevPrice1H       string `json:"prevPrice1h"`
	OpenInterest      string `json:"openInterest"`
	OpenInterestValue string `json:"openInterestValue"`
	Turnover24H       string `json:"turnover24h"`
	Volume24H         string `json:"volume24h"`
	FundingRate       string `json:"fundingRate"`
	Bid1Price         string `json:"bid1Price"`
	Ask1Price         string `json:"ask1Price"`
}

type bybitKlineResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Symbol   string     `json:"symbol"`
		Category string     `json:"category"`
		List     [][]string `json:"list"`
	} `json:"result"`
}

func fetchBybitUniverseSnapshots(ctx context.Context, limit int) ([]rawUniverseSnapshot, error) {
	client := newUniverseHTTPClient()
	tradable, err := fetchBybitTradableLinearSymbols(ctx, client)
	if err != nil {
		return nil, err
	}

	var tickerResponse bybitTickersResponse
	if err := httpJSONGet(ctx, client, bybitMarketBaseURL+"/v5/market/tickers?category=linear", &tickerResponse); err != nil {
		return nil, fmt.Errorf("tickers: %w", err)
	}
	if tickerResponse.RetCode != 0 {
		return nil, fmt.Errorf("tickers retCode=%d retMsg=%s", tickerResponse.RetCode, tickerResponse.RetMsg)
	}

	selected := make([]bybitTickerRow, 0, len(tickerResponse.Result.List))
	for _, item := range tickerResponse.Result.List {
		if !tradable[item.Symbol] {
			continue
		}
		selected = append(selected, item)
	}

	sort.Slice(selected, func(i, j int) bool {
		left := parseFloat(selected[i].Turnover24H)
		right := parseFloat(selected[j].Turnover24H)
		if left == right {
			return selected[i].Symbol < selected[j].Symbol
		}
		return left > right
	})

	selectionLimit := perExchangeSelectionLimit(limit)
	if len(selected) > selectionLimit {
		selected = selected[:selectionLimit]
	}

	result := make([]rawUniverseSnapshot, 0, len(selected))
	for _, ticker := range selected {
		price := firstPositiveFloat(ticker.LastPrice, ticker.MarkPrice, ticker.IndexPrice)
		if price <= 0 {
			continue
		}

		openInterest := parseFloat(ticker.OpenInterest)
		openInterestValue := parseFloat(ticker.OpenInterestValue)
		if openInterestValue > 0 && price > 0 {
			openInterest = openInterestValue / price
		}

		indexPrice := parseFloat(ticker.IndexPrice)
		markPrice := parseFloat(ticker.MarkPrice)
		premium := 0.0
		if markPrice > 0 && indexPrice > 0 {
			premium = (markPrice - indexPrice) / indexPrice
		}

		spreadBps := computeSpreadBps([]string{ticker.Bid1Price, ticker.Ask1Price}, price)
		result = append(result, rawUniverseSnapshot{
			Symbol:        ticker.Symbol,
			Sources:       []string{"bybit"},
			Price:         price,
			PrevDayPrice:  parseFloat(ticker.PrevPrice24H),
			RefPrice1H:    parseFloat(ticker.PrevPrice1H),
			OpenInterest:  openInterest,
			Volume24H:     parseFloat(ticker.Turnover24H),
			VolumeBase24H: parseFloat(ticker.Volume24H),
			Funding:       parseFloat(ticker.FundingRate),
			Premium:       premium,
			SpreadBps:     spreadBps,
			IsAvailable:   price > 0 && parseFloat(ticker.Turnover24H) > 0,
		})
	}
	return result, nil
}

func fetchBybitTradableLinearSymbols(ctx context.Context, client *http.Client) (map[string]bool, error) {
	result := make(map[string]bool)
	cursor := ""
	for {
		requestURL := bybitMarketBaseURL + "/v5/market/instruments-info?category=linear&limit=1000"
		if cursor != "" {
			requestURL += "&cursor=" + cursor
		}

		var response bybitInstrumentsInfoResponse
		if err := httpJSONGet(ctx, client, requestURL, &response); err != nil {
			return nil, fmt.Errorf("instruments-info: %w", err)
		}
		if response.RetCode != 0 {
			return nil, fmt.Errorf("instruments-info retCode=%d retMsg=%s", response.RetCode, response.RetMsg)
		}

		for _, item := range response.Result.List {
			if item.Status != "Trading" {
				continue
			}
			if item.ContractType != "LinearPerpetual" {
				continue
			}
			if item.QuoteCoin != "USDT" {
				continue
			}
			result[item.Symbol] = true
		}

		cursor = response.Result.NextPageCursor
		if cursor == "" {
			break
		}
	}
	return result, nil
}

func fetchBybitBootstrapPriceRef(ctx context.Context, symbol string) (bootstrapPriceRef, bool) {
	client := newUniverseHTTPClient()
	requestURL := fmt.Sprintf("%s/v5/market/kline?category=linear&symbol=%s&interval=60&limit=5", bybitMarketBaseURL, normalizePair(symbol))
	var response bybitKlineResponse
	if err := httpJSONGet(ctx, client, requestURL, &response); err != nil {
		return bootstrapPriceRef{}, false
	}
	if response.RetCode != 0 || len(response.Result.List) == 0 {
		return bootstrapPriceRef{}, false
	}

	rows := append([][]string(nil), response.Result.List...)
	sort.Slice(rows, func(i, j int) bool {
		return parseFloat(rows[i][0]) < parseFloat(rows[j][0])
	})

	closeAt := func(row []string) float64 {
		if len(row) < 5 {
			return 0
		}
		return parseFloat(row[4])
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
