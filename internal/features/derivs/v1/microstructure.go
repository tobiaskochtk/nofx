package derivsv1

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	binanceFuturesBase     = "https://fapi.binance.com"
	bybitWSLinear          = "wss://stream.bybit.com/v5/public/linear"
	bybitLiqMax            = 200
	takerInterval          = "5m"
	takerLimit             = 200
	liqLimit               = 300
	priceKlineLimit        = 200
	microHTTPTimeout       = 4 * time.Second
	zWindowShortBuckets    = 90
	zWindowLongBuckets     = 360
	slopeWindowShort       = 20
	slopeWindowMid         = 60
	atrPeriod              = 14
	liqClusterBandPct      = 0.002
	liqRiskAtrThreshold    = 1.0
	confirmationWindowMins = 15
)

type microFetcher struct {
	client    *http.Client
	apiKey    string
	apiSecret string
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
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	Price       string `json:"price"`
	Size        string `json:"size"`
	UpdatedTime string `json:"updatedTime"`
}

type bybitLiqWSMessage struct {
	Topic string         `json:"topic"`
	Type  string         `json:"type"`
	Ts    int64          `json:"ts"`
	Data  []bybitLiqItem `json:"data"`
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
	prices, atr, err := cfg.Fetcher.fetchPriceSeries(cfg.Symbol)
	if err != nil {
		log.Printf("[ComputeMicrostructureFeatures] %s: price series fetch failed: %v", cfg.Symbol, err)
		return result, nil
	}
	liqEvents, err := cfg.Fetcher.fetchLiquidations(cfg.Symbol)
	if err != nil {
		log.Printf("[ComputeMicrostructureFeatures] %s: liq events fetch failed: %v", cfg.Symbol, err)
	}

	if len(flows) == 0 || len(prices) == 0 {
		return result, nil
	}

	// Align price series to flow length to preserve slope windows.
	if len(prices) > len(flows) {
		prices = prices[len(prices)-len(flows):]
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

	if slope, r2 := slopeR2(prices, slopeWindowShort); slope != nil {
		result["slope_price_3m_short"] = *slope
		result["r2_price_3m_short"] = r2
	}
	if slope, r2 := slopeR2(prices, slopeWindowMid); slope != nil {
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
	assignDivergence(result, prices, cvdSeries, slopeWindowShort, "div_bull_short_3m", "div_bear_short_3m")
	assignDivergence(result, prices, cvdSeries, slopeWindowMid, "div_bull_mid_3m", "div_bear_mid_3m")

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
		if slope, _ := slopeR2(prices, 3); slope != nil && len(cvdSeries) >= 3 {
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
	if len(liqEvents) > 0 && atr > 0 && len(prices) > 0 {
		currPrice := prices[len(prices)-1]
		liqMetrics := computeLiqMetrics(liqEvents, currPrice, atr)
		for k, v := range liqMetrics {
			result[k] = v
		}
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

func (f *microFetcher) fetchPriceSeries(symbol string) ([]float64, float64, error) {
	url := fmt.Sprintf("%s/fapi/v1/klines?symbol=%s&interval=%s&limit=%d", binanceFuturesBase, symbol, takerInterval, priceKlineLimit)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var raw [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, 0, err
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
		return nil, 0, fmt.Errorf("no klines")
	}
	prices := make([]float64, 0, len(candles))
	for _, c := range candles {
		prices = append(prices, c.Close)
	}
	atr := computeATR(candles, atrPeriod)
	return prices, atr, nil
}

func (f *microFetcher) fetchLiquidations(symbol string) ([]liqEvent, error) {
	limit := liqLimit
	if limit > bybitLiqMax {
		limit = bybitLiqMax
	}

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
		"args": []string{fmt.Sprintf("liquidation.%s", strings.ToUpper(symbol))},
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
		if want != "" && !strings.EqualFold(item.Symbol, want) {
			continue
		}
		price, err1 := strconv.ParseFloat(item.Price, 64)
		qty, err2 := strconv.ParseFloat(item.Size, 64)
		if err1 != nil || err2 != nil || price <= 0 || qty <= 0 {
			continue
		}

		ts, err3 := strconv.ParseInt(item.UpdatedTime, 10, 64)
		if err3 != nil {
			ts = 0
		}

		events = append(events, liqEvent{
			price: price,
			size:  price * qty,
			side:  item.Side,
			ts:    ts,
		})
	}
	return events
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

	switch {
	case distUp == 0 && distDn == 0:
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
	}
	return res
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
