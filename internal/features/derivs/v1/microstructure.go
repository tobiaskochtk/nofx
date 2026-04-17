package derivsv1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	binanceFuturesBase     = "https://fapi.binance.com"
	binanceWSFutures       = "wss://fstream.binance.com/ws"
	bybitWSLinear          = "wss://stream.bybit.com/v5/public/linear"
	bybitLiqTopicPrefix    = "allLiquidation"
	bybitLiqMax            = 200
	takerInterval          = "5m"
	takerLimit             = 200
	liqLimit               = 300
	liqFetchAttempts       = 2
	liqFetchRetryDelay     = 200 * time.Millisecond
	priceKlineLimit        = 200
	microHTTPTimeout       = 4 * time.Second
	zWindowShortBuckets    = 90
	zWindowLongBuckets     = 360
	slopeWindowShort       = 20
	slopeWindowMid         = 60
	atrPeriod              = 14
	liqClusterBandPct      = 0.002
	liqRiskAtrThreshold    = 1.0
	liqFallbackAtrMultiple = 1.5
	liqFallbackConf3m      = 0.1
	liqFallbackConf15m     = 0.08
	confirmationWindowMins = 15
	avwapAnchorLookback    = 60
	bbPeriod               = 20
	rvShortWindow          = 20
	rvLongWindow           = 80
	liqVenueCooldown       = 2 * time.Minute
	liqSymbolCooldown      = 3 * time.Minute
	liqVenueTimeoutLimit   = 2
)

type microFetcher struct {
	client    *http.Client
	apiKey    string
	apiSecret string
}

type liqFetchOutcome struct {
	Attempted       bool
	CooldownSkipped bool
}

type liqFetchCircuit struct {
	mu                sync.Mutex
	symbolCooldowns   map[string]time.Time
	venueCooldowns    map[string]time.Time
	venueTimeoutCount map[string]int
}

var globalLiqFetchCircuit = newLiqFetchCircuit()

func newLiqFetchCircuit() *liqFetchCircuit {
	return &liqFetchCircuit{
		symbolCooldowns:   make(map[string]time.Time),
		venueCooldowns:    make(map[string]time.Time),
		venueTimeoutCount: make(map[string]int),
	}
}

func newMicroFetcher() *microFetcher {
	return newMicroFetcherWithAuth("", "")
}

func newMicroFetcherWithAuth(apiKey, apiSecret string) *microFetcher {
	return &microFetcher{
		client: &http.Client{
			Timeout: microHTTPTimeout,
		},
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}
}

type takerRatio struct {
	BuyVol    string `json:"buyVol"`
	SellVol   string `json:"sellVol"`
	Timestamp int64  `json:"timestamp"`
}

type klineCandle struct {
	OpenTime int64
	Open     float64
	High     float64
	Low      float64
	Close    float64
}

type takerSample struct {
	buy       float64
	sell      float64
	timestamp int64
}

type liqEvent struct {
	price float64
	size  float64
	side  string
	ts    int64
}

type bybitLiqItem struct {
	Symbol      string `json:"symbol"`      // legacy/custom adapters
	Side        string `json:"side"`        // legacy/custom adapters
	Price       string `json:"price"`       // legacy/custom adapters
	Size        string `json:"size"`        // legacy/custom adapters
	UpdatedTime string `json:"updatedTime"` // legacy/custom adapters

	// Bybit V5 allLiquidation payload fields
	SymbolV5 string `json:"s"`
	SideV5   string `json:"S"`
	PriceV5  string `json:"p"`
	SizeV5   string `json:"v"`
	TimeV5   int64  `json:"T"`
}

type bybitLiqWSMessage struct {
	Topic string         `json:"topic"`
	Type  string         `json:"type"`
	Ts    int64          `json:"ts"`
	Data  []bybitLiqItem `json:"data"`
}

type binanceForceOrderMessage struct {
	Event     string                 `json:"e"`
	EventTime int64                  `json:"E"`
	Order     binanceForceOrderEntry `json:"o"`
}

type binanceForceOrderEntry struct {
	Symbol    string `json:"s"`
	Side      string `json:"S"`
	Price     string `json:"p"`
	AvgPrice  string `json:"ap"`
	Qty       string `json:"q"`
	LastQty   string `json:"l"`
	FilledQty string `json:"z"`
	TradeTime int64  `json:"T"`
}

// MicroFeatureConfig bundles network access for microstructure computations.
type MicroFeatureConfig struct {
	Symbol  string
	Fetcher *microFetcher
}

// ComputeMicrostructureFeatures derives Feature 4 (CVD/taker) and Feature 5 (liq heatmap) metrics.
// It intentionally degrades gracefully: transient API errors return partial/empty results without failing the caller.
func ComputeMicrostructureFeatures(cfg MicroFeatureConfig) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	if cfg.Fetcher == nil {
		return result, nil
	}

	// --- Fetch data sources ---
	flows, err := cfg.Fetcher.fetchTakerFlow(cfg.Symbol)
	if err != nil {
		log.Printf("[ComputeMicrostructureFeatures] %s: taker flow fetch failed: %v", cfg.Symbol, err)
		return result, nil
	}
	prices, candles, atr, err := cfg.Fetcher.fetchPriceSeries(cfg.Symbol)
	if err != nil {
		log.Printf("[ComputeMicrostructureFeatures] %s: price series fetch failed: %v", cfg.Symbol, err)
		return result, nil
	}
	liqEvents, liqOutcome, err := cfg.Fetcher.fetchLiquidations(cfg.Symbol)
	if err != nil && !liqOutcome.CooldownSkipped {
		log.Printf("[ComputeMicrostructureFeatures] %s: liq events fetch failed: %v", cfg.Symbol, err)
	}
	if len(liqEvents) == 0 && liqOutcome.Attempted && !liqOutcome.CooldownSkipped {
		log.Printf("[ComputeMicrostructureFeatures] %s: liq fallback metrics engaged (no live liquidation events)", cfg.Symbol)
	}

	if len(flows) == 0 || len(prices) == 0 || len(candles) == 0 {
		return result, nil
	}

	// Align a copy of the price series to flow length to preserve slope windows.
	pricesForFlow := prices
	if len(pricesForFlow) > len(flows) {
		pricesForFlow = pricesForFlow[len(pricesForFlow)-len(flows):]
	}

	// --- Feature 4: CVD / taker imbalance ---
	cvdSeries, imbSeries := buildCVDSeries(flows)
	if len(cvdSeries) > 0 {
		if z := zScoreWindow(cvdSeries, zWindowShortBuckets); z != nil {
			result["cvd_notional_z_3m_short"] = *z
		}
		if z := zScoreWindow(cvdSeries, zWindowLongBuckets); z != nil {
			result["cvd_notional_z_3m_long"] = *z
		}
		if slope, r2 := slopeR2(cvdSeries, slopeWindowShort); slope != nil {
			result["slope_cvdz_3m_short"] = *slope
			result["r2_cvdz_3m_short"] = r2
		}
		if slope, r2 := slopeR2(cvdSeries, slopeWindowMid); slope != nil {
			result["slope_cvdz_3m_mid"] = *slope
			result["r2_cvdz_3m_mid"] = r2
		}
	}

	if len(imbSeries) > 0 {
		if z := zScoreWindow(imbSeries, zWindowShortBuckets); z != nil {
			result["imb_notional_z_3m_short"] = *z
		}
		if z := zScoreWindow(imbSeries, zWindowLongBuckets); z != nil {
			result["imb_notional_z_3m_long"] = *z
		}
		if slope, r2 := slopeR2(imbSeries, slopeWindowShort); slope != nil {
			result["slope_imb_3m_short"] = *slope
			result["r2_imb_3m_short"] = r2
		}
		if slope, r2 := slopeR2(imbSeries, slopeWindowMid); slope != nil {
			result["slope_imb_3m_mid"] = *slope
			result["r2_imb_3m_mid"] = r2
		}
	}

	if slope, r2 := slopeR2(pricesForFlow, slopeWindowShort); slope != nil {
		result["slope_price_3m_short"] = *slope
		result["r2_price_3m_short"] = r2
	}
	if slope, r2 := slopeR2(pricesForFlow, slopeWindowMid); slope != nil {
		result["slope_price_3m_mid"] = *slope
		result["r2_price_3m_mid"] = r2
	}

	if len(flows) > 0 {
		last := flows[len(flows)-1]
		tbr := takerBuyRatio(last.buy, last.sell)
		if !math.IsNaN(tbr) {
			result["tbr_notional_3m"] = tbr
		}
	}

	// Divergences on short/mid windows.
	assignDivergence(result, pricesForFlow, cvdSeries, slopeWindowShort, "div_bull_short_3m", "div_bear_short_3m")
	assignDivergence(result, pricesForFlow, cvdSeries, slopeWindowMid, "div_bull_mid_3m", "div_bear_mid_3m")

	// Confidence = coverage of samples vs long window.
	confCVD := clamp01(float64(len(cvdSeries)) / float64(zWindowLongBuckets))
	if confCVD > 0 {
		result["confidence_cvd_3m"] = confCVD
	}

	// 15m confirmation: reuse freshest slices (≈3 samples on 5m interval).
	if len(cvdSeries) >= 3 {
		sub := cvdSeries[len(cvdSeries)-3:]
		conf15 := clamp01(float64(len(sub)) / 3.0)
		result["confidence_cvd_15m"] = conf15
		if z := zScore(sub); z != nil {
			result["cvd_notional_z_15m_short"] = *z
			result["cvd_notional_z_15m_long"] = *z
		}
		if len(imbSeries) >= 3 {
			if z := zScore(imbSeries[len(imbSeries)-3:]); z != nil {
				result["imb_notional_z_15m_short"] = *z
				result["imb_notional_z_15m_long"] = *z
			}
		}
		if len(flows) >= 1 {
			last := flows[len(flows)-1]
			result["tbr_notional_15m"] = takerBuyRatio(last.buy, last.sell)
		}
		// simple divergence flags on mid window for confirmation
		if slope, _ := slopeR2(pricesForFlow, 3); slope != nil && len(cvdSeries) >= 3 {
			if slopeCVD, _ := slopeR2(cvdSeries, 3); slopeCVD != nil {
				if *slope > 0 && *slopeCVD < 0 {
					result["div_bear_mid_15m"] = 1
				} else if *slope < 0 && *slopeCVD > 0 {
					result["div_bull_mid_15m"] = 1
				}
			}
		}
	}

	// --- Feature 5: Liquidation heatmap ---
	if atr > 0 && len(prices) > 0 {
		currPrice := prices[len(prices)-1]
		liqMetrics := computeLiqMetrics(liqEvents, currPrice, atr)
		for k, v := range liqMetrics {
			result[k] = v
		}
	}

	// --- Feature 6: AVWAP structure ---
	avwapMetrics := computeAVWAPMetrics(candles, atr)
	for k, v := range avwapMetrics {
		result[k] = v
	}

	// --- Feature 7: volatility / squeeze regime ---
	volMetrics := computeVolatilityMetrics(candles, atr)
	for k, v := range volMetrics {
		result[k] = v
	}

	return result, nil
}

func (f *microFetcher) fetchTakerFlow(symbol string) ([]takerSample, error) {
	url := fmt.Sprintf("%s/futures/data/takerlongshortRatio?symbol=%s&period=%s&limit=%d", binanceFuturesBase, symbol, takerInterval, takerLimit)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload []takerRatio
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	samples := make([]takerSample, 0, len(payload))
	for _, p := range payload {
		buy, err1 := strconv.ParseFloat(p.BuyVol, 64)
		sell, err2 := strconv.ParseFloat(p.SellVol, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		samples = append(samples, takerSample{
			buy:       buy,
			sell:      sell,
			timestamp: p.Timestamp,
		})
	}
	return samples, nil
}

func (f *microFetcher) fetchPriceSeries(symbol string) ([]float64, []klineCandle, float64, error) {
	url := fmt.Sprintf("%s/fapi/v1/klines?symbol=%s&interval=%s&limit=%d", binanceFuturesBase, symbol, takerInterval, priceKlineLimit)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, 0, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, nil, 0, err
	}
	defer resp.Body.Close()

	var raw [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, nil, 0, err
	}
	candles := make([]klineCandle, 0, len(raw))
	for _, row := range raw {
		if len(row) < 6 {
			continue
		}
		open, _ := toFloat(row[1])
		high, _ := toFloat(row[2])
		low, _ := toFloat(row[3])
		closeP, _ := toFloat(row[4])
		openTime, _ := toInt64(row[0])
		candles = append(candles, klineCandle{
			OpenTime: openTime,
			Open:     open,
			High:     high,
			Low:      low,
			Close:    closeP,
		})
	}
	if len(candles) == 0 {
		return nil, nil, 0, fmt.Errorf("no klines")
	}
	prices := make([]float64, 0, len(candles))
	for _, c := range candles {
		prices = append(prices, c.Close)
	}
	atr := computeATR(candles, atrPeriod)
	return prices, candles, atr, nil
}

func (f *microFetcher) fetchLiquidations(symbol string) ([]liqEvent, liqFetchOutcome, error) {
	outcome := liqFetchOutcome{}
	limit := liqLimit
	if limit > bybitLiqMax {
		limit = bybitLiqMax
	}
	now := time.Now()
	if globalLiqFetchCircuit.shouldSkipSymbol(symbol, now) {
		outcome.CooldownSkipped = true
		return nil, outcome, nil
	}
	var errors []string
	venuesAttempted := 0

	if globalLiqFetchCircuit.shouldAttemptVenue("bybit", now) {
		venuesAttempted++
		outcome.Attempted = true
		for attempt := 1; attempt <= liqFetchAttempts; attempt++ {
			events, err := f.fetchBybitLiquidations(symbol, limit)
			if len(events) > 0 {
				globalLiqFetchCircuit.clearSymbolCooldown(symbol)
				globalLiqFetchCircuit.recordVenueSuccess("bybit")
				return events, outcome, nil
			}
			if err != nil {
				errors = append(errors, fmt.Sprintf("bybit attempt %d: %v", attempt, err))
				if triggered, until := globalLiqFetchCircuit.recordVenueFailure("bybit", err, time.Now()); triggered {
					log.Printf("[ComputeMicrostructureFeatures] bybit liq stream cooldown engaged until %s after repeated transport timeouts", until.UTC().Format(time.RFC3339))
					break
				}
			}
			if attempt < liqFetchAttempts {
				time.Sleep(liqFetchRetryDelay)
			}
		}
	}

	if globalLiqFetchCircuit.shouldAttemptVenue("binance", now) {
		venuesAttempted++
		outcome.Attempted = true
		for attempt := 1; attempt <= liqFetchAttempts; attempt++ {
			events, err := f.fetchBinanceLiquidations(symbol, limit)
			if len(events) > 0 {
				globalLiqFetchCircuit.clearSymbolCooldown(symbol)
				globalLiqFetchCircuit.recordVenueSuccess("binance")
				return events, outcome, nil
			}
			if err != nil {
				errors = append(errors, fmt.Sprintf("binance attempt %d: %v", attempt, err))
				if triggered, until := globalLiqFetchCircuit.recordVenueFailure("binance", err, time.Now()); triggered {
					log.Printf("[ComputeMicrostructureFeatures] binance liq stream cooldown engaged until %s after repeated transport timeouts", until.UTC().Format(time.RFC3339))
					break
				}
			}
			if attempt < liqFetchAttempts {
				time.Sleep(liqFetchRetryDelay)
			}
		}
	}

	if venuesAttempted == 0 {
		outcome.CooldownSkipped = true
		globalLiqFetchCircuit.markSymbolCooldown(symbol, now)
		return nil, outcome, nil
	}
	globalLiqFetchCircuit.markSymbolCooldown(symbol, time.Now())
	if len(errors) == 0 {
		return nil, outcome, fmt.Errorf("no liquidation events for %s", strings.ToUpper(symbol))
	}
	return nil, outcome, fmt.Errorf("no liquidation events for %s (%s)", strings.ToUpper(symbol), strings.Join(errors, "; "))
}

func (f *microFetcher) fetchBybitLiquidations(symbol string, limit int) ([]liqEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*microHTTPTimeout)
	defer cancel()

	wsURL := bybitWSLinear
	dialer := websocket.Dialer{HandshakeTimeout: microHTTPTimeout}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("bybit ws dial: %w", err)
	}
	defer conn.Close()

	sub := map[string]interface{}{
		"op":   "subscribe",
		"args": []string{fmt.Sprintf("%s.%s", bybitLiqTopicPrefix, strings.ToUpper(symbol))},
	}
	if err := conn.WriteJSON(sub); err != nil {
		return nil, fmt.Errorf("bybit ws subscribe: %w", err)
	}

	deadline := time.Now().Add(2 * microHTTPTimeout)
	events := make([]liqEvent, 0, limit)

	for len(events) < limit && time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		_, message, err := conn.ReadMessage()
		if err != nil {
			return events, fmt.Errorf("bybit ws read: %w", err)
		}

		parsed := parseBybitLiqMessage(message, strings.ToUpper(symbol))
		if len(parsed) > 0 {
			events = append(events, parsed...)
		}
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("bybit ws yielded no events for %s", symbol)
	}

	if len(events) > limit {
		events = events[len(events)-limit:]
	}

	return events, nil
}

func (f *microFetcher) fetchBinanceLiquidations(symbol string, limit int) ([]liqEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*microHTTPTimeout)
	defer cancel()

	stream := strings.ToLower(symbol) + "@forceOrder"
	wsURL := fmt.Sprintf("%s/%s", binanceWSFutures, stream)
	dialer := websocket.Dialer{HandshakeTimeout: microHTTPTimeout}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("binance ws dial: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(2 * microHTTPTimeout)
	events := make([]liqEvent, 0, limit)
	want := strings.ToUpper(symbol)

	for len(events) < limit && time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		_, message, err := conn.ReadMessage()
		if err != nil {
			return events, fmt.Errorf("binance ws read: %w", err)
		}
		parsed := parseBinanceForceOrderMessage(message, want)
		if parsed != nil {
			events = append(events, *parsed)
		}
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("binance ws yielded no events for %s", want)
	}
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events, nil
}

func parseBybitLiqMessage(raw []byte, want string) []liqEvent {
	var msg bybitLiqWSMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}
	if len(msg.Data) == 0 {
		return nil
	}
	if want != "" && !strings.Contains(strings.ToUpper(msg.Topic), want) {
		return nil
	}

	return toBybitLiqEvents(msg.Data, want)
}

func toBybitLiqEvents(src []bybitLiqItem, want string) []liqEvent {
	events := make([]liqEvent, 0, len(src))
	for _, item := range src {
		symbol := firstNonEmpty(item.Symbol, item.SymbolV5)
		if want != "" && !strings.EqualFold(symbol, want) {
			continue
		}

		priceRaw := firstNonEmpty(item.Price, item.PriceV5)
		sizeRaw := firstNonEmpty(item.Size, item.SizeV5)
		side := firstNonEmpty(item.Side, item.SideV5)
		price, err1 := strconv.ParseFloat(priceRaw, 64)
		qty, err2 := strconv.ParseFloat(sizeRaw, 64)
		if err1 != nil || err2 != nil || price <= 0 || qty <= 0 {
			continue
		}

		ts := item.TimeV5
		if ts <= 0 {
			parsedTS, err3 := strconv.ParseInt(item.UpdatedTime, 10, 64)
			if err3 == nil {
				ts = parsedTS
			}
		}

		events = append(events, liqEvent{
			price: price,
			size:  price * qty,
			side:  side,
			ts:    ts,
		})
	}
	return events
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseBinanceForceOrderMessage(raw []byte, want string) *liqEvent {
	var msg binanceForceOrderMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}
	if !strings.EqualFold(msg.Event, "forceOrder") {
		return nil
	}
	if want != "" && !strings.EqualFold(msg.Order.Symbol, want) {
		return nil
	}

	price := parsePositiveFloat(msg.Order.AvgPrice)
	if price <= 0 {
		price = parsePositiveFloat(msg.Order.Price)
	}
	qty := parsePositiveFloat(msg.Order.FilledQty)
	if qty <= 0 {
		qty = parsePositiveFloat(msg.Order.LastQty)
	}
	if qty <= 0 {
		qty = parsePositiveFloat(msg.Order.Qty)
	}
	if price <= 0 || qty <= 0 {
		return nil
	}

	ts := msg.Order.TradeTime
	if ts <= 0 {
		ts = msg.EventTime
	}
	return &liqEvent{
		price: price,
		size:  price * qty,
		side:  msg.Order.Side,
		ts:    ts,
	}
}

func (c *liqFetchCircuit) shouldSkipSymbol(symbol string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	until := c.symbolCooldowns[strings.ToUpper(strings.TrimSpace(symbol))]
	return !until.IsZero() && until.After(now)
}

func (c *liqFetchCircuit) markSymbolCooldown(symbol string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.symbolCooldowns[strings.ToUpper(strings.TrimSpace(symbol))] = now.Add(liqSymbolCooldown)
}

func (c *liqFetchCircuit) clearSymbolCooldown(symbol string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.symbolCooldowns, strings.ToUpper(strings.TrimSpace(symbol)))
}

func (c *liqFetchCircuit) shouldAttemptVenue(venue string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	until := c.venueCooldowns[strings.ToLower(strings.TrimSpace(venue))]
	return until.IsZero() || !until.After(now)
}

func (c *liqFetchCircuit) recordVenueSuccess(venue string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	venue = strings.ToLower(strings.TrimSpace(venue))
	c.venueTimeoutCount[venue] = 0
	delete(c.venueCooldowns, venue)
}

func (c *liqFetchCircuit) recordVenueFailure(venue string, err error, now time.Time) (bool, time.Time) {
	if !isLiqTransportTimeout(err) {
		return false, time.Time{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	venue = strings.ToLower(strings.TrimSpace(venue))
	c.venueTimeoutCount[venue]++
	if c.venueTimeoutCount[venue] < liqVenueTimeoutLimit {
		return false, time.Time{}
	}
	until := now.Add(liqVenueCooldown)
	c.venueCooldowns[venue] = until
	c.venueTimeoutCount[venue] = 0
	return true, until
}

func isLiqTransportTimeout(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "timeout")
}

func parsePositiveFloat(raw string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

func buildCVDSeries(samples []takerSample) ([]float64, []float64) {
	cvdSeries := make([]float64, 0, len(samples))
	imbSeries := make([]float64, 0, len(samples))
	var cvd float64
	for _, s := range samples {
		delta := s.buy - s.sell
		total := s.buy + s.sell
		cvd += delta
		cvdSeries = append(cvdSeries, cvd)
		if total > 0 {
			imbSeries = append(imbSeries, delta/total)
		} else {
			imbSeries = append(imbSeries, 0)
		}
	}
	return cvdSeries, imbSeries
}

func takerBuyRatio(buy, sell float64) float64 {
	total := buy + sell
	if total == 0 {
		return math.NaN()
	}
	return buy / total
}

func zScoreWindow(series []float64, window int) *float64 {
	if window <= 0 || len(series) == 0 {
		return nil
	}
	if len(series) > window {
		series = series[len(series)-window:]
	}
	return zScore(series)
}

func zScore(series []float64) *float64 {
	if len(series) == 0 {
		return nil
	}
	meanVal := avg(series)
	std := stdDev(series, meanVal)
	if std == 0 {
		zero := 0.0
		return &zero
	}
	last := series[len(series)-1]
	z := (last - meanVal) / std
	return &z
}

func slopeR2(series []float64, window int) (*float64, float64) {
	if window <= 1 || len(series) < 2 {
		return nil, 0
	}
	if len(series) > window {
		series = series[len(series)-window:]
	}
	n := float64(len(series))
	var sumX, sumY, sumXY, sumXX float64
	for i, y := range series {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return nil, 0
	}
	slope := (n*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / n

	var ssTot, ssRes float64
	meanY := sumY / n
	for i, y := range series {
		x := float64(i)
		pred := slope*x + intercept
		ssRes += (y - pred) * (y - pred)
		ssTot += (y - meanY) * (y - meanY)
	}
	var r2 float64
	if ssTot > 0 {
		r2 = 1 - ssRes/ssTot
	}
	return &slope, r2
}

func avg(series []float64) float64 {
	var sum float64
	for _, v := range series {
		sum += v
	}
	return sum / float64(len(series))
}

func stdDev(series []float64, mean float64) float64 {
	if len(series) == 0 {
		return 0
	}
	var sum float64
	for _, v := range series {
		diff := v - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(series)))
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func computeATR(candles []klineCandle, period int) float64 {
	if len(candles) <= period {
		return 0
	}
	trs := make([]float64, len(candles))
	for i := 1; i < len(candles); i++ {
		high := candles[i].High
		low := candles[i].Low
		prevClose := candles[i-1].Close
		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)
		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}
	var sum float64
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)
	for i := period + 1; i < len(trs); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}
	return atr
}

func computeLiqMetrics(events []liqEvent, currentPrice float64, atr float64) map[string]interface{} {
	res := make(map[string]interface{})
	if currentPrice <= 0 || atr <= 0 {
		return res
	}

	var upDistances, dnDistances []float64
	var upStrength, dnStrength float64
	var upPriceClosest, dnPriceClosest float64

	band := math.Max(currentPrice*liqClusterBandPct, atr*0.5)
	now := time.Now().UnixMilli()
	cutoff15m := now - int64(confirmationWindowMins)*int64(time.Minute/time.Millisecond)
	var confirmUp, confirmDn []float64

	for _, ev := range events {
		dist := ev.price - currentPrice
		absDist := math.Abs(dist)
		if absDist == 0 {
			continue
		}
		if dist > 0 {
			upDistances = append(upDistances, dist)
			if upPriceClosest == 0 || dist < upPriceClosest-currentPrice {
				upPriceClosest = ev.price
			}
		} else {
			dnDistances = append(dnDistances, -dist)
			if dnPriceClosest == 0 || -dist < currentPrice-dnPriceClosest {
				dnPriceClosest = ev.price
			}
		}
		if upPriceClosest > 0 && math.Abs(ev.price-upPriceClosest) <= band {
			upStrength += ev.size
		}
		if dnPriceClosest > 0 && math.Abs(ev.price-dnPriceClosest) <= band {
			dnStrength += ev.size
		}
		if ev.ts >= cutoff15m {
			if dist > 0 {
				confirmUp = append(confirmUp, dist)
			} else {
				confirmDn = append(confirmDn, -dist)
			}
		}
	}

	var distUp, distDn float64
	if len(upDistances) > 0 {
		distUp = minFloat(upDistances...)
		atrVal := distUp / atr
		res["dist_up_pct_3m"] = (distUp / currentPrice) * 100
		res["dist_up_atr_3m"] = atrVal
		res["cluster_strength_up_3m"] = upStrength
		if atrVal < liqRiskAtrThreshold {
			res["liq_risk_up_3m"] = 1
		} else {
			res["liq_risk_up_3m"] = 0
		}
	}
	if len(dnDistances) > 0 {
		distDn = minFloat(dnDistances...)
		atrVal := distDn / atr
		res["dist_dn_pct_3m"] = (distDn / currentPrice) * 100
		res["dist_dn_atr_3m"] = atrVal
		res["cluster_strength_down_3m"] = dnStrength
		if atrVal < liqRiskAtrThreshold {
			res["liq_risk_down_3m"] = 1
		} else {
			res["liq_risk_down_3m"] = 0
		}
	}

	// Ensure both sides of F5 remain populated even when one/both event buckets are empty.
	// This keeps prompt structure stable under transient WS outages and low-event regimes.
	distUp, distDn = ensureLiqFallback3m(res, currentPrice, atr, distUp, distDn)

	switch {
	case distUp == 0 && distDn == 0:
		res["prefer_direction_3m"] = "balanced"
	case distUp == 0:
		res["prefer_direction_3m"] = "prefer_longs"
	case distDn == 0:
		res["prefer_direction_3m"] = "prefer_shorts"
	case distUp < distDn:
		res["prefer_direction_3m"] = "prefer_shorts"
	case distDn < distUp:
		res["prefer_direction_3m"] = "prefer_longs"
	}

	res["bucket_usd_3m"] = upStrength + dnStrength
	res["event_count_liq_3m"] = len(events)
	res["atr_3m"] = atr
	conf := clamp01(math.Sqrt(float64(len(events))) / 10)
	if len(events) == 0 && conf < liqFallbackConf3m {
		conf = liqFallbackConf3m
	}
	res["confidence_liq_3m"] = conf

	// Confirmation (15m slice)
	if len(confirmUp) > 0 {
		res["dist_up_atr_15m"] = (minFloat(confirmUp...) / atr)
	}
	if len(confirmDn) > 0 {
		res["dist_dn_atr_15m"] = (minFloat(confirmDn...) / atr)
	}
	if len(confirmUp) > 0 || len(confirmDn) > 0 {
		switch {
		case len(confirmUp) == 0:
			res["prefer_direction_15m"] = "prefer_shorts"
		case len(confirmDn) == 0:
			res["prefer_direction_15m"] = "prefer_longs"
		case minFloat(confirmUp...) < minFloat(confirmDn...):
			res["prefer_direction_15m"] = "prefer_shorts"
		default:
			res["prefer_direction_15m"] = "prefer_longs"
		}
		res["confidence_liq_15m"] = clamp01(math.Sqrt(float64(len(confirmUp)+len(confirmDn))) / 5)
	} else {
		applyLiqFallback15m(res, atr)
	}
	return res
}

func ensureLiqFallback3m(res map[string]interface{}, currentPrice, atr, distUp, distDn float64) (float64, float64) {
	fallbackDist := atr * liqFallbackAtrMultiple
	if fallbackDist <= 0 {
		return distUp, distDn
	}
	if distUp <= 0 {
		distUp = fallbackDist
		atrVal := distUp / atr
		res["dist_up_pct_3m"] = (distUp / currentPrice) * 100
		res["dist_up_atr_3m"] = atrVal
		if _, exists := res["cluster_strength_up_3m"]; !exists {
			res["cluster_strength_up_3m"] = 0.0
		}
		if atrVal < liqRiskAtrThreshold {
			res["liq_risk_up_3m"] = 1
		} else {
			res["liq_risk_up_3m"] = 0
		}
	}
	if distDn <= 0 {
		distDn = fallbackDist
		atrVal := distDn / atr
		res["dist_dn_pct_3m"] = (distDn / currentPrice) * 100
		res["dist_dn_atr_3m"] = atrVal
		if _, exists := res["cluster_strength_down_3m"]; !exists {
			res["cluster_strength_down_3m"] = 0.0
		}
		if atrVal < liqRiskAtrThreshold {
			res["liq_risk_down_3m"] = 1
		} else {
			res["liq_risk_down_3m"] = 0
		}
	}
	return distUp, distDn
}

func applyLiqFallback15m(res map[string]interface{}, atr float64) {
	if atr <= 0 {
		return
	}
	atrVal := liqFallbackAtrMultiple
	if _, exists := res["dist_up_atr_15m"]; !exists {
		res["dist_up_atr_15m"] = atrVal
	}
	if _, exists := res["dist_dn_atr_15m"]; !exists {
		res["dist_dn_atr_15m"] = atrVal
	}
	if _, exists := res["prefer_direction_15m"]; !exists {
		res["prefer_direction_15m"] = "balanced"
	}
	if _, exists := res["confidence_liq_15m"]; !exists {
		res["confidence_liq_15m"] = liqFallbackConf15m
	}
}

func computeAVWAPMetrics(candles []klineCandle, atr float64) map[string]interface{} {
	res := make(map[string]interface{})
	if len(candles) < 3 || atr <= 0 {
		return res
	}
	curr := candles[len(candles)-1].Close
	prev := candles[len(candles)-2].Close
	if curr <= 0 {
		return res
	}

	lookback := minInt(avwapAnchorLookback, len(candles)-1)
	if lookback < 3 {
		return res
	}
	start := len(candles) - lookback
	hiIdx := start
	loIdx := start
	for i := start + 1; i < len(candles); i++ {
		if candles[i].High > candles[hiIdx].High {
			hiIdx = i
		}
		if candles[i].Low < candles[loIdx].Low {
			loIdx = i
		}
	}

	upPrice := anchoredTypicalPrice(candles, hiIdx)
	dnPrice := anchoredTypicalPrice(candles, loIdx)
	if upPrice <= curr {
		upPrice = curr + atr*0.5
	}
	if dnPrice >= curr {
		dnPrice = curr - atr*0.5
	}
	if upPrice <= curr || dnPrice >= curr {
		return res
	}

	upDistAtr := (upPrice - curr) / atr
	dnDistAtr := (curr - dnPrice) / atr
	reclaim := 0
	reject := 0
	if prev < upPrice && curr >= upPrice {
		reclaim = 1
	}
	if prev > dnPrice && curr <= dnPrice {
		reject = 1
	}
	bias := classifyAVWAPBias(curr, upPrice, dnPrice, atr, reclaim, reject)
	conf := clamp01(float64(lookback) / float64(avwapAnchorLookback))

	res["avwap_up_name_3m"] = "swing_high"
	res["avwap_up_price_3m"] = upPrice
	res["avwap_up_dist_atr_3m"] = upDistAtr
	res["avwap_up_band1_dist_atr_3m"] = upDistAtr * 0.5
	res["avwap_dn_name_3m"] = "swing_low"
	res["avwap_dn_price_3m"] = dnPrice
	res["avwap_dn_dist_atr_3m"] = dnDistAtr
	res["avwap_dn_band1_dist_atr_3m"] = dnDistAtr * 0.5
	res["avwap_reclaim_up_3m"] = reclaim
	res["avwap_rejection_down_3m"] = reject
	res["avwap_confluence_bull_3m"] = boolToInt(reclaim == 1 && bias == "bullish")
	res["avwap_confluence_bear_3m"] = boolToInt(reject == 1 && bias == "bearish")
	res["avwap_bias_3m"] = bias
	res["confidence_avwap_3m"] = conf

	res["avwap_up_name_15m"] = "swing_high"
	res["avwap_up_dist_atr_15m"] = upDistAtr
	res["avwap_dn_name_15m"] = "swing_low"
	res["avwap_dn_dist_atr_15m"] = dnDistAtr
	res["avwap_reclaim_up_15m"] = reclaim
	res["avwap_rejection_down_15m"] = reject
	res["avwap_bias_15m"] = bias
	res["confidence_avwap_15m"] = clamp01(conf * 0.9)

	return res
}

func computeVolatilityMetrics(candles []klineCandle, atr float64) map[string]interface{} {
	res := make(map[string]interface{})
	if len(candles) < bbPeriod+2 || atr <= 0 {
		return res
	}
	closes := make([]float64, 0, len(candles))
	for _, c := range candles {
		closes = append(closes, c.Close)
	}
	if len(closes) < bbPeriod+2 {
		return res
	}
	ma := meanWindow(closes, bbPeriod)
	if ma <= 0 {
		return res
	}

	bbwSeries := rollingBBW(closes, bbPeriod)
	if len(bbwSeries) == 0 {
		return res
	}
	bbwCurr := bbwSeries[len(bbwSeries)-1]
	kcWidth := (3.0 * atr) / ma
	bbwRank := percentileRank(bbwSeries, bbwCurr)
	sqOn := boolToInt(bbwCurr <= kcWidth)
	sqPersist := squeezePersistence(bbwSeries, kcWidth)
	sqRelease := 0
	if len(bbwSeries) >= 2 && bbwSeries[len(bbwSeries)-2] <= kcWidth && sqOn == 0 {
		sqRelease = 1
	}
	expansion := 0.0
	if len(bbwSeries) >= 2 && bbwSeries[len(bbwSeries)-2] > 0 {
		expansion = (bbwCurr - bbwSeries[len(bbwSeries)-2]) / bbwSeries[len(bbwSeries)-2]
	}

	rets := percentReturns(closes)
	rvShort := realizedVol(rets, rvShortWindow)
	rvLong := realizedVol(rets, rvLongWindow)
	rvRatio := 0.0
	if rvLong > 0 {
		rvRatio = rvShort / rvLong
	}
	rvRatioZ := 0.0
	rvSeries := rollingRVRatio(rets, rvShortWindow, rvLongWindow)
	if z := zScore(rvSeries); z != nil {
		rvRatioZ = *z
	}
	regime := classifyVolRegime(rvRatio, sqOn, expansion)
	conf := clamp01(float64(len(rets)) / float64(rvLongWindow*2))
	if len(rvSeries) < 10 {
		conf *= 0.7
	}

	res["bbw_3m"] = bbwCurr
	res["kc_width_3m"] = kcWidth
	res["bbw_pct_rank_3m"] = bbwRank
	res["squeeze_on_3m"] = sqOn
	res["squeeze_persistence_3m"] = sqPersist
	res["squeeze_release_3m"] = sqRelease
	res["bbw_expansion_rate_3m"] = expansion
	res["rv_ratio_3m"] = rvRatio
	res["rv_ratio_z_3m"] = rvRatioZ
	res["vol_regime_3m"] = regime
	res["confidence_vol_3m"] = conf

	res["squeeze_on_15m"] = sqOn
	res["squeeze_release_15m"] = sqRelease
	res["bbw_pct_rank_15m"] = percentileRank(lastWindow(bbwSeries, 120), bbwCurr)
	res["rv_ratio_15m"] = rvRatio
	res["vol_regime_15m"] = regime
	res["confidence_vol_15m"] = clamp01(conf * 0.9)

	return res
}

func anchoredTypicalPrice(candles []klineCandle, start int) float64 {
	if len(candles) == 0 {
		return 0
	}
	if start < 0 {
		start = 0
	}
	if start >= len(candles) {
		start = len(candles) - 1
	}
	sum := 0.0
	count := 0.0
	for i := start; i < len(candles); i++ {
		sum += (candles[i].High + candles[i].Low + candles[i].Close) / 3.0
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / count
}

func classifyAVWAPBias(curr, up, down, atr float64, reclaim, reject int) string {
	if reclaim == 1 {
		return "bullish"
	}
	if reject == 1 {
		return "bearish"
	}
	mid := (up + down) / 2
	switch {
	case curr > mid+0.1*atr:
		return "bullish"
	case curr < mid-0.1*atr:
		return "bearish"
	default:
		return "neutral"
	}
}

func rollingBBW(closes []float64, period int) []float64 {
	if period <= 1 || len(closes) < period {
		return nil
	}
	out := make([]float64, 0, len(closes)-period+1)
	for i := period; i <= len(closes); i++ {
		win := closes[i-period : i]
		mu := avg(win)
		if mu <= 0 {
			continue
		}
		std := stdDev(win, mu)
		upper := mu + 2*std
		lower := mu - 2*std
		bbw := (upper - lower) / mu
		out = append(out, bbw)
	}
	return out
}

func squeezePersistence(bbwSeries []float64, kcWidth float64) int {
	count := 0
	for i := len(bbwSeries) - 1; i >= 0; i-- {
		if bbwSeries[i] <= kcWidth {
			count++
			continue
		}
		break
	}
	return count
}

func percentileRank(series []float64, value float64) float64 {
	if len(series) == 0 {
		return 0
	}
	lessEq := 0
	for _, v := range series {
		if v <= value {
			lessEq++
		}
	}
	return clamp01(float64(lessEq) / float64(len(series)))
}

func percentReturns(series []float64) []float64 {
	if len(series) < 2 {
		return nil
	}
	out := make([]float64, 0, len(series)-1)
	for i := 1; i < len(series); i++ {
		prev := series[i-1]
		if prev == 0 {
			continue
		}
		out = append(out, (series[i]-prev)/prev)
	}
	return out
}

func realizedVol(returns []float64, window int) float64 {
	if window <= 1 || len(returns) < window {
		return 0
	}
	win := returns[len(returns)-window:]
	sum := 0.0
	for _, r := range win {
		sum += r * r
	}
	return math.Sqrt(sum / float64(len(win)))
}

func rollingRVRatio(returns []float64, shortWin, longWin int) []float64 {
	if shortWin <= 1 || longWin <= shortWin || len(returns) < longWin {
		return nil
	}
	out := make([]float64, 0, len(returns)-longWin+1)
	for i := longWin; i <= len(returns); i++ {
		win := returns[i-longWin : i]
		rvL := realizedVol(win, longWin)
		rvS := realizedVol(win, shortWin)
		if rvL <= 0 {
			continue
		}
		out = append(out, rvS/rvL)
	}
	return out
}

func classifyVolRegime(rvRatio float64, squeezeOn int, expansion float64) string {
	if squeezeOn == 1 {
		return "compressed"
	}
	if rvRatio >= 1.25 || (rvRatio >= 1.0 && expansion > 0.1) {
		return "expanding"
	}
	if rvRatio > 0 && rvRatio <= 0.8 {
		return "contracting"
	}
	return "normal"
}

func meanWindow(series []float64, window int) float64 {
	if len(series) == 0 || window <= 0 {
		return 0
	}
	if len(series) > window {
		series = series[len(series)-window:]
	}
	return avg(series)
}

func lastWindow(series []float64, window int) []float64 {
	if window <= 0 || len(series) == 0 {
		return series
	}
	if len(series) <= window {
		return series
	}
	return series[len(series)-window:]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func assignDivergence(result map[string]interface{}, priceSeries, cvdSeries []float64, window int, bullKey, bearKey string) {
	if len(priceSeries) < 2 || len(cvdSeries) < 2 {
		return
	}
	if len(priceSeries) > len(cvdSeries) {
		priceSeries = priceSeries[len(priceSeries)-len(cvdSeries):]
	}
	priceSlope, _ := slopeR2(priceSeries, window)
	cvdSlope, _ := slopeR2(cvdSeries, window)
	if priceSlope == nil || cvdSlope == nil {
		return
	}
	if *priceSlope > 0 && *cvdSlope < 0 {
		result[bearKey] = 1
		result[bullKey] = 0
	} else if *priceSlope < 0 && *cvdSlope > 0 {
		result[bullKey] = 1
		result[bearKey] = 0
	}
}

func minFloat(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func toFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case json.Number:
		return val.Float64()
	default:
		return 0, fmt.Errorf("unsupported type")
	}
}

func toInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case float64:
		return int64(val), nil
	case json.Number:
		return val.Int64()
	default:
		return 0, fmt.Errorf("unsupported type")
	}
}
