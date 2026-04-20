// Run from the scripts module, for example:
//
//	go run .\compare_signal_provider.go -reference-base-url https://nofxos.ai -candidate-base-url http://127.0.0.1:8081
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"nofx/provider/nofxos"
	"os"
	"sort"
	"strings"
	"time"
)

type providerConfig struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Auth    string `json:"-"`
}

type scalarDiff struct {
	Reference float64 `json:"reference"`
	Candidate float64 `json:"candidate"`
	AbsDiff   float64 `json:"abs_diff"`
}

type rankedSetComparison struct {
	ReferenceCount  int      `json:"reference_count"`
	CandidateCount  int      `json:"candidate_count"`
	SharedCount     int      `json:"shared_count"`
	UnionCount      int      `json:"union_count"`
	OverlapRate     float64  `json:"overlap_rate"`
	JaccardRate     float64  `json:"jaccard_rate"`
	MissingRate     float64  `json:"missing_rate"`
	RankCorrelation *float64 `json:"rank_correlation,omitempty"`
	ReferenceOnly   []string `json:"reference_only,omitempty"`
	CandidateOnly   []string `json:"candidate_only,omitempty"`
	SharedTop       []string `json:"shared_top,omitempty"`
}

type ai500Comparison struct {
	Symbols           rankedSetComparison `json:"symbols"`
	MeanAbsScoreDelta float64             `json:"mean_abs_score_delta"`
}

type rankedPairComparison struct {
	Top rankedSetComparison `json:"top"`
	Low rankedSetComparison `json:"low"`
}

type coinDetailComparison struct {
	Symbol           string                `json:"symbol"`
	ReferencePresent bool                  `json:"reference_present"`
	CandidatePresent bool                  `json:"candidate_present"`
	MissingSections  map[string][]string   `json:"missing_sections,omitempty"`
	Price            *scalarDiff           `json:"price,omitempty"`
	PriceChange      map[string]scalarDiff `json:"price_change,omitempty"`
	AI500            map[string]scalarDiff `json:"ai500,omitempty"`
	OIByExchange     map[string]scalarDiff `json:"oi_current_by_exchange,omitempty"`
	OIDeltaPercent   map[string]scalarDiff `json:"oi_delta_percent,omitempty"`
	NetflowByBucket  map[string]scalarDiff `json:"netflow_by_bucket,omitempty"`
}

type endpointError struct {
	Endpoint string `json:"endpoint"`
	Side     string `json:"side"`
	Message  string `json:"message"`
}

type comparisonReport struct {
	GeneratedAt time.Time                       `json:"generated_at"`
	Reference   providerConfig                  `json:"reference"`
	Candidate   providerConfig                  `json:"candidate"`
	Symbol      string                          `json:"symbol"`
	Settings    map[string]interface{}          `json:"settings"`
	AI500       *ai500Comparison                `json:"ai500,omitempty"`
	OI          *rankedPairComparison           `json:"oi,omitempty"`
	Price       map[string]rankedPairComparison `json:"price,omitempty"`
	Coin        *coinDetailComparison           `json:"coin,omitempty"`
	Errors      []endpointError                 `json:"errors,omitempty"`
}

type scriptConfig struct {
	Reference providerConfig
	Candidate providerConfig
	Symbol    string
	Limit     int
	Duration  string
	Durations string
	Include   string
	Format    string
	Timeout   time.Duration
}

func main() {
	cfg := parseFlags()
	report := runComparison(cfg)

	switch strings.ToLower(cfg.Format) {
	case "json":
		renderJSON(report)
	default:
		renderText(report)
	}

	if len(report.Errors) > 0 {
		os.Exit(1)
	}
}

func parseFlags() scriptConfig {
	referenceBaseURL := firstNonEmpty(os.Getenv("COMPARE_REFERENCE_BASE_URL"), nofxos.DefaultBaseURL)
	referenceAuth := firstNonEmpty(os.Getenv("COMPARE_REFERENCE_AUTH"), nofxos.DefaultAuthKey)
	candidateBaseURL := firstNonEmpty(os.Getenv("COMPARE_CANDIDATE_BASE_URL"), "http://127.0.0.1:8081")
	candidateAuth := firstNonEmpty(os.Getenv("COMPARE_CANDIDATE_AUTH"), os.Getenv("SELFHOSTED_AI500_AUTH_TOKEN"), "local-selfhosted-ai500-token")

	var cfg scriptConfig
	flag.StringVar(&cfg.Reference.Name, "reference-name", "official", "display name of the reference provider")
	flag.StringVar(&cfg.Reference.BaseURL, "reference-base-url", referenceBaseURL, "base URL of the reference provider")
	flag.StringVar(&cfg.Reference.Auth, "reference-auth", referenceAuth, "auth token for the reference provider")
	flag.StringVar(&cfg.Candidate.Name, "candidate-name", "selfhosted", "display name of the candidate provider")
	flag.StringVar(&cfg.Candidate.BaseURL, "candidate-base-url", candidateBaseURL, "base URL of the candidate provider")
	flag.StringVar(&cfg.Candidate.Auth, "candidate-auth", candidateAuth, "auth token for the candidate provider")
	flag.StringVar(&cfg.Symbol, "symbol", "", "symbol used for coin detail comparison; empty means auto-select from ai500 list")
	flag.IntVar(&cfg.Limit, "limit", 10, "ranking comparison limit")
	flag.StringVar(&cfg.Duration, "duration", "1h", "single duration for oi comparison")
	flag.StringVar(&cfg.Durations, "durations", "1h,4h,24h", "durations for price ranking comparison")
	flag.StringVar(&cfg.Include, "include", "netflow,oi,price,ai500", "coin detail include list")
	flag.StringVar(&cfg.Format, "format", "text", "output format: text or json")
	timeoutSeconds := flag.Int("timeout-sec", 20, "HTTP timeout in seconds")
	flag.Parse()

	cfg.Reference.BaseURL = strings.TrimRight(cfg.Reference.BaseURL, "/")
	cfg.Candidate.BaseURL = strings.TrimRight(cfg.Candidate.BaseURL, "/")
	cfg.Symbol = normalizeSymbolInput(cfg.Symbol)
	cfg.Duration = normalizeDuration(cfg.Duration)
	cfg.Durations = strings.Join(normalizeDurationList(cfg.Durations), ",")
	cfg.Timeout = time.Duration(*timeoutSeconds) * time.Second
	return cfg
}

func runComparison(cfg scriptConfig) comparisonReport {
	report := comparisonReport{
		GeneratedAt: time.Now().UTC(),
		Reference: providerConfig{
			Name:    cfg.Reference.Name,
			BaseURL: cfg.Reference.BaseURL,
		},
		Candidate: providerConfig{
			Name:    cfg.Candidate.Name,
			BaseURL: cfg.Candidate.BaseURL,
		},
		Settings: map[string]interface{}{
			"limit":     cfg.Limit,
			"duration":  cfg.Duration,
			"durations": normalizeDurationList(cfg.Durations),
			"include":   cfg.Include,
		},
	}

	client := &http.Client{Timeout: cfg.Timeout}

	refAI500, refAIErr := fetchAI500(client, cfg.Reference)
	if refAIErr != nil {
		report.Errors = append(report.Errors, endpointError{Endpoint: "ai500/list", Side: "reference", Message: refAIErr.Error()})
	}
	candAI500, candAIErr := fetchAI500(client, cfg.Candidate)
	if candAIErr != nil {
		report.Errors = append(report.Errors, endpointError{Endpoint: "ai500/list", Side: "candidate", Message: candAIErr.Error()})
	}
	if refAIErr == nil && candAIErr == nil {
		report.AI500 = compareAI500(refAI500, candAI500)
	}

	if cfg.Symbol == "" {
		cfg.Symbol = chooseSymbol(refAI500, candAI500)
	}
	report.Symbol = cfg.Symbol

	refOI, refOIErr := fetchOI(client, cfg.Reference, cfg.Duration, cfg.Limit)
	if refOIErr != nil {
		report.Errors = append(report.Errors, endpointError{Endpoint: "oi/ranking", Side: "reference", Message: refOIErr.Error()})
	}
	candOI, candOIErr := fetchOI(client, cfg.Candidate, cfg.Duration, cfg.Limit)
	if candOIErr != nil {
		report.Errors = append(report.Errors, endpointError{Endpoint: "oi/ranking", Side: "candidate", Message: candOIErr.Error()})
	}
	if refOIErr == nil && candOIErr == nil {
		report.OI = &rankedPairComparison{
			Top: compareRankedSymbolSets(extractOISymbols(refOI.Data.Data.Positions), extractOISymbols(candOI.Data.Data.Positions)),
			Low: compareRankedSymbolSets(extractOISymbols(refOI.DataLow.Data.Positions), extractOISymbols(candOI.DataLow.Data.Positions)),
		}
	}

	refPrice, refPriceErr := fetchPrice(client, cfg.Reference, cfg.Durations, cfg.Limit)
	if refPriceErr != nil {
		report.Errors = append(report.Errors, endpointError{Endpoint: "price/ranking", Side: "reference", Message: refPriceErr.Error()})
	}
	candPrice, candPriceErr := fetchPrice(client, cfg.Candidate, cfg.Durations, cfg.Limit)
	if candPriceErr != nil {
		report.Errors = append(report.Errors, endpointError{Endpoint: "price/ranking", Side: "candidate", Message: candPriceErr.Error()})
	}
	if refPriceErr == nil && candPriceErr == nil {
		report.Price = comparePriceRanking(refPrice, candPrice, normalizeDurationList(cfg.Durations))
	}

	if cfg.Symbol != "" {
		refCoin, refCoinErr := fetchCoin(client, cfg.Reference, cfg.Symbol, cfg.Include)
		if refCoinErr != nil {
			report.Errors = append(report.Errors, endpointError{Endpoint: "coin/detail", Side: "reference", Message: refCoinErr.Error()})
		}
		candCoin, candCoinErr := fetchCoin(client, cfg.Candidate, cfg.Symbol, cfg.Include)
		if candCoinErr != nil {
			report.Errors = append(report.Errors, endpointError{Endpoint: "coin/detail", Side: "candidate", Message: candCoinErr.Error()})
		}
		report.Coin = compareCoinDetails(cfg.Symbol, refCoin, refCoinErr == nil, candCoin, candCoinErr == nil)
	}

	return report
}

type oiFetchResult struct {
	Data    nofxos.OIRankingResponse
	DataLow nofxos.OIRankingResponse
}

func fetchAI500(client *http.Client, provider providerConfig) (nofxos.AI500Response, error) {
	return fetchJSON[nofxos.AI500Response](client, provider, "/api/ai500/list")
}

func fetchOI(client *http.Client, provider providerConfig, duration string, limit int) (oiFetchResult, error) {
	top, err := fetchJSON[nofxos.OIRankingResponse](client, provider, fmt.Sprintf("/api/oi/top-ranking?duration=%s&limit=%d", url.QueryEscape(duration), limit))
	if err != nil {
		return oiFetchResult{}, err
	}
	low, err := fetchJSON[nofxos.OIRankingResponse](client, provider, fmt.Sprintf("/api/oi/low-ranking?duration=%s&limit=%d", url.QueryEscape(duration), limit))
	if err != nil {
		return oiFetchResult{}, err
	}
	return oiFetchResult{Data: top, DataLow: low}, nil
}

func fetchPrice(client *http.Client, provider providerConfig, durations string, limit int) (nofxos.PriceRankingResponse, error) {
	return fetchJSON[nofxos.PriceRankingResponse](client, provider, fmt.Sprintf("/api/price/ranking?duration=%s&limit=%d", url.QueryEscape(durations), limit))
}

func fetchCoin(client *http.Client, provider providerConfig, symbol, include string) (*nofxos.CoinResponse, error) {
	response, err := fetchJSON[nofxos.CoinResponse](client, provider, fmt.Sprintf("/api/coin/%s?include=%s", url.PathEscape(strings.TrimSuffix(symbol, "USDT")), url.QueryEscape(include)))
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func fetchJSON[T any](client *http.Client, provider providerConfig, endpoint string) (T, error) {
	var payload T

	fullURL, err := buildURL(provider.BaseURL, endpoint, provider.Auth)
	if err != nil {
		return payload, err
	}

	resp, err := client.Get(fullURL)
	if err != nil {
		return payload, fmt.Errorf("%s request failed: %w", provider.Name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return payload, fmt.Errorf("%s read failed: %w", provider.Name, err)
	}

	if resp.StatusCode != http.StatusOK {
		return payload, fmt.Errorf("%s returned HTTP %d: %s", provider.Name, resp.StatusCode, trimBody(string(body)))
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return payload, fmt.Errorf("%s JSON parse failed: %w", provider.Name, err)
	}

	return payload, nil
}

func buildURL(baseURL, endpoint, auth string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}
	pathURL, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid endpoint %q: %w", endpoint, err)
	}
	resolved := base.ResolveReference(pathURL)
	query := resolved.Query()
	if auth != "" && query.Get("auth") == "" {
		query.Set("auth", auth)
	}
	resolved.RawQuery = query.Encode()
	return resolved.String(), nil
}

func compareAI500(reference, candidate nofxos.AI500Response) *ai500Comparison {
	report := &ai500Comparison{
		Symbols: compareRankedSymbolSets(extractAI500Symbols(reference), extractAI500Symbols(candidate)),
	}

	refScores := make(map[string]float64, len(reference.Data.Coins))
	for _, coin := range reference.Data.Coins {
		refScores[nofxos.NormalizeSymbol(coin.Pair)] = coin.Score
	}
	candScores := make(map[string]float64, len(candidate.Data.Coins))
	for _, coin := range candidate.Data.Coins {
		candScores[nofxos.NormalizeSymbol(coin.Pair)] = coin.Score
	}

	deltas := 0.0
	shared := 0
	for symbol, refScore := range refScores {
		if candScore, ok := candScores[symbol]; ok {
			deltas += math.Abs(refScore - candScore)
			shared++
		}
	}
	if shared > 0 {
		report.MeanAbsScoreDelta = round(deltas/float64(shared), 4)
	}

	return report
}

func comparePriceRanking(reference, candidate nofxos.PriceRankingResponse, durations []string) map[string]rankedPairComparison {
	report := make(map[string]rankedPairComparison, len(durations))
	for _, duration := range durations {
		refDuration := reference.Data.Data[duration]
		candDuration := candidate.Data.Data[duration]
		report[duration] = rankedPairComparison{
			Top: compareRankedSymbolSets(extractPriceSymbols(refDuration.Top), extractPriceSymbols(candDuration.Top)),
			Low: compareRankedSymbolSets(extractPriceSymbols(refDuration.Low), extractPriceSymbols(candDuration.Low)),
		}
	}
	return report
}

func compareCoinDetails(symbol string, reference *nofxos.CoinResponse, referenceOK bool, candidate *nofxos.CoinResponse, candidateOK bool) *coinDetailComparison {
	result := &coinDetailComparison{
		Symbol:           symbol,
		ReferencePresent: referenceOK && reference != nil && reference.Data != nil,
		CandidatePresent: candidateOK && candidate != nil && candidate.Data != nil,
		MissingSections: map[string][]string{
			"reference": {},
			"candidate": {},
		},
	}

	if !result.ReferencePresent || !result.CandidatePresent {
		return result
	}

	refData := reference.Data
	candData := candidate.Data
	result.Price = diffScalar(refData.Price, candData.Price)
	result.PriceChange = comparePriceChange(refData.PriceChange, candData.PriceChange, &result.MissingSections)
	result.AI500 = compareAI500Fields(refData.AI500, candData.AI500, &result.MissingSections)
	result.OIByExchange, result.OIDeltaPercent = compareOI(refData.OI, candData.OI, &result.MissingSections)
	result.NetflowByBucket = compareNetflow(refData.Netflow, candData.Netflow, &result.MissingSections)

	if len(result.MissingSections["reference"]) == 0 {
		delete(result.MissingSections, "reference")
	}
	if len(result.MissingSections["candidate"]) == 0 {
		delete(result.MissingSections, "candidate")
	}
	if len(result.MissingSections) == 0 {
		result.MissingSections = nil
	}

	return result
}

func comparePriceChange(reference, candidate map[string]float64, missing *map[string][]string) map[string]scalarDiff {
	if len(reference) == 0 {
		(*missing)["reference"] = append((*missing)["reference"], "price_change")
	}
	if len(candidate) == 0 {
		(*missing)["candidate"] = append((*missing)["candidate"], "price_change")
	}
	keys := unionKeysFloatMap(reference, candidate)
	if len(keys) == 0 {
		return nil
	}
	result := make(map[string]scalarDiff, len(keys))
	for _, key := range keys {
		result[key] = *diffScalar(reference[key], candidate[key])
	}
	return result
}

func compareAI500Fields(reference, candidate *nofxos.AI500QuantData, missing *map[string][]string) map[string]scalarDiff {
	if reference == nil {
		(*missing)["reference"] = append((*missing)["reference"], "ai500")
	}
	if candidate == nil {
		(*missing)["candidate"] = append((*missing)["candidate"], "ai500")
	}
	if reference == nil || candidate == nil {
		return nil
	}

	return map[string]scalarDiff{
		"score":            *diffScalar(reference.Score, candidate.Score),
		"rank":             *diffScalar(float64(reference.Rank), float64(candidate.Rank)),
		"start_price":      *diffScalar(reference.StartPrice, candidate.StartPrice),
		"last_score":       *diffScalar(reference.LastScore, candidate.LastScore),
		"max_score":        *diffScalar(reference.MaxScore, candidate.MaxScore),
		"max_price":        *diffScalar(reference.MaxPrice, candidate.MaxPrice),
		"increase_percent": *diffScalar(reference.IncreasePercent, candidate.IncreasePercent),
	}
}

func compareOI(reference, candidate map[string]*nofxos.OIData, missing *map[string][]string) (map[string]scalarDiff, map[string]scalarDiff) {
	if len(reference) == 0 {
		(*missing)["reference"] = append((*missing)["reference"], "oi")
	}
	if len(candidate) == 0 {
		(*missing)["candidate"] = append((*missing)["candidate"], "oi")
	}
	if len(reference) == 0 || len(candidate) == 0 {
		return nil, nil
	}

	currentOI := make(map[string]scalarDiff)
	deltaPercent := make(map[string]scalarDiff)
	for _, exchange := range unionKeysOIMap(reference, candidate) {
		refData := reference[exchange]
		candData := candidate[exchange]
		if refData == nil || candData == nil {
			continue
		}
		currentOI[exchange] = *diffScalar(refData.CurrentOI, candData.CurrentOI)

		refDurations := refData.Delta
		candDurations := candData.Delta
		for _, duration := range unionKeysOIDeltaMap(refDurations, candDurations) {
			refDelta := refDurations[duration]
			candDelta := candDurations[duration]
			if refDelta == nil || candDelta == nil {
				continue
			}
			key := exchange + "." + duration
			deltaPercent[key] = *diffScalar(refDelta.OIDeltaPercent, candDelta.OIDeltaPercent)
		}
	}

	if len(currentOI) == 0 {
		currentOI = nil
	}
	if len(deltaPercent) == 0 {
		deltaPercent = nil
	}
	return currentOI, deltaPercent
}

func compareNetflow(reference, candidate *nofxos.NetflowData, missing *map[string][]string) map[string]scalarDiff {
	if reference == nil {
		(*missing)["reference"] = append((*missing)["reference"], "netflow")
	}
	if candidate == nil {
		(*missing)["candidate"] = append((*missing)["candidate"], "netflow")
	}
	if reference == nil || candidate == nil {
		return nil
	}

	result := make(map[string]scalarDiff)
	flattenFlow("institution.future", getFlowMap(reference.Institution, "future"), getFlowMap(candidate.Institution, "future"), result)
	flattenFlow("institution.spot", getFlowMap(reference.Institution, "spot"), getFlowMap(candidate.Institution, "spot"), result)
	flattenFlow("personal.future", getFlowMap(reference.Personal, "future"), getFlowMap(candidate.Personal, "future"), result)
	flattenFlow("personal.spot", getFlowMap(reference.Personal, "spot"), getFlowMap(candidate.Personal, "spot"), result)
	if len(result) == 0 {
		return nil
	}
	return result
}

func flattenFlow(prefix string, reference, candidate map[string]float64, target map[string]scalarDiff) {
	for _, duration := range unionKeysFloatMap(reference, candidate) {
		target[prefix+"."+duration] = *diffScalar(reference[duration], candidate[duration])
	}
}

func diffScalar(reference, candidate float64) *scalarDiff {
	return &scalarDiff{
		Reference: round(reference, 6),
		Candidate: round(candidate, 6),
		AbsDiff:   round(math.Abs(reference-candidate), 6),
	}
}

func compareRankedSymbolSets(reference, candidate []string) rankedSetComparison {
	refRanks := make(map[string]int, len(reference))
	for idx, symbol := range reference {
		refRanks[symbol] = idx + 1
	}
	candRanks := make(map[string]int, len(candidate))
	for idx, symbol := range candidate {
		candRanks[symbol] = idx + 1
	}

	shared := make([]string, 0)
	referenceOnly := make([]string, 0)
	for _, symbol := range reference {
		if _, ok := candRanks[symbol]; ok {
			shared = append(shared, symbol)
		} else {
			referenceOnly = append(referenceOnly, symbol)
		}
	}
	candidateOnly := make([]string, 0)
	for _, symbol := range candidate {
		if _, ok := refRanks[symbol]; !ok {
			candidateOnly = append(candidateOnly, symbol)
		}
	}

	sharedCount := len(shared)
	unionCount := len(referenceOnly) + len(candidateOnly) + sharedCount
	overlapRate := ratio(sharedCount, len(reference))
	jaccardRate := ratio(sharedCount, unionCount)
	missingRate := ratio(len(referenceOnly), len(reference))

	result := rankedSetComparison{
		ReferenceCount: len(reference),
		CandidateCount: len(candidate),
		SharedCount:    sharedCount,
		UnionCount:     unionCount,
		OverlapRate:    round(overlapRate, 4),
		JaccardRate:    round(jaccardRate, 4),
		MissingRate:    round(missingRate, 4),
		ReferenceOnly:  truncateList(referenceOnly, 10),
		CandidateOnly:  truncateList(candidateOnly, 10),
		SharedTop:      truncateList(shared, 10),
	}

	if correlation := spearmanRankCorrelation(shared, refRanks, candRanks); correlation != nil {
		value := round(*correlation, 4)
		result.RankCorrelation = &value
	}

	return result
}

func spearmanRankCorrelation(shared []string, referenceRanks, candidateRanks map[string]int) *float64 {
	if len(shared) < 2 {
		return nil
	}

	sumSquares := 0.0
	for _, symbol := range shared {
		diff := float64(referenceRanks[symbol] - candidateRanks[symbol])
		sumSquares += diff * diff
	}

	n := float64(len(shared))
	if n <= 1 {
		return nil
	}
	value := 1 - ((6 * sumSquares) / (n * (n*n - 1)))
	return &value
}

func extractAI500Symbols(response nofxos.AI500Response) []string {
	symbols := make([]string, 0, len(response.Data.Coins))
	for _, coin := range response.Data.Coins {
		symbols = append(symbols, nofxos.NormalizeSymbol(coin.Pair))
	}
	return symbols
}

func extractOISymbols(positions []nofxos.OIPosition) []string {
	symbols := make([]string, 0, len(positions))
	for _, position := range positions {
		symbols = append(symbols, nofxos.NormalizeSymbol(position.Symbol))
	}
	return symbols
}

func extractPriceSymbols(items []nofxos.PriceRankingItem) []string {
	symbols := make([]string, 0, len(items))
	for _, item := range items {
		symbol := item.Symbol
		if symbol == "" {
			symbol = item.Pair
		}
		symbols = append(symbols, nofxos.NormalizeSymbol(symbol))
	}
	return symbols
}

func chooseSymbol(reference, candidate nofxos.AI500Response) string {
	if len(reference.Data.Coins) > 0 {
		return nofxos.NormalizeSymbol(reference.Data.Coins[0].Pair)
	}
	if len(candidate.Data.Coins) > 0 {
		return nofxos.NormalizeSymbol(candidate.Data.Coins[0].Pair)
	}
	return "BTCUSDT"
}

func normalizeSymbolInput(symbol string) string {
	if strings.TrimSpace(symbol) == "" {
		return ""
	}
	return nofxos.NormalizeSymbol(symbol)
}

func normalizeDuration(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "4h":
		return "4h"
	case "24h":
		return "24h"
	default:
		return "1h"
	}
}

func normalizeDurationList(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]bool)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		duration := normalizeDuration(part)
		if !seen[duration] {
			seen[duration] = true
			result = append(result, duration)
		}
	}
	if len(result) == 0 {
		return []string{"1h"}
	}
	return result
}

func getFlowMap(data *nofxos.FlowTypeData, trade string) map[string]float64 {
	if data == nil {
		return nil
	}
	if trade == "spot" {
		return data.Spot
	}
	return data.Future
}

func unionKeysFloatMap(left, right map[string]float64) []string {
	seen := make(map[string]bool)
	keys := make([]string, 0, len(left)+len(right))
	for key := range left {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range right {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func unionKeysOIMap(left, right map[string]*nofxos.OIData) []string {
	seen := make(map[string]bool)
	keys := make([]string, 0, len(left)+len(right))
	for key := range left {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range right {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func unionKeysOIDeltaMap(left, right map[string]*nofxos.OIDeltaData) []string {
	seen := make(map[string]bool)
	keys := make([]string, 0, len(left)+len(right))
	for key := range left {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range right {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func truncateList(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	return append([]string{}, items[:max]...)
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func round(value float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(value*pow) / pow
}

func trimBody(body string) string {
	body = strings.TrimSpace(body)
	if len(body) > 240 {
		return body[:240] + "..."
	}
	return body
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func renderJSON(report comparisonReport) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode report: %v\n", err)
		os.Exit(1)
	}
}

func renderText(report comparisonReport) {
	fmt.Printf("Signal Provider Comparison\n")
	fmt.Printf("Reference: %s (%s)\n", report.Reference.Name, report.Reference.BaseURL)
	fmt.Printf("Candidate: %s (%s)\n", report.Candidate.Name, report.Candidate.BaseURL)
	fmt.Printf("Generated: %s\n", report.GeneratedAt.Format(time.RFC3339))
	fmt.Printf("Symbol: %s\n\n", firstNonEmpty(report.Symbol, "<none>"))

	if report.AI500 != nil {
		fmt.Printf("AI500 list\n")
		printRankedComparison("  symbols", report.AI500.Symbols)
		fmt.Printf("  mean_abs_score_delta: %.4f\n\n", report.AI500.MeanAbsScoreDelta)
	}

	if report.OI != nil {
		fmt.Printf("OI ranking\n")
		printRankedComparison("  top", report.OI.Top)
		printRankedComparison("  low", report.OI.Low)
		fmt.Println()
	}

	if len(report.Price) > 0 {
		fmt.Printf("Price ranking\n")
		durations := make([]string, 0, len(report.Price))
		for duration := range report.Price {
			durations = append(durations, duration)
		}
		sort.Strings(durations)
		for _, duration := range durations {
			fmt.Printf("  %s\n", duration)
			printRankedComparison("    top", report.Price[duration].Top)
			printRankedComparison("    low", report.Price[duration].Low)
		}
		fmt.Println()
	}

	if report.Coin != nil {
		fmt.Printf("Coin detail\n")
		fmt.Printf("  reference_present: %v\n", report.Coin.ReferencePresent)
		fmt.Printf("  candidate_present: %v\n", report.Coin.CandidatePresent)
		if report.Coin.Price != nil {
			fmt.Printf("  price_abs_diff: %.6f\n", report.Coin.Price.AbsDiff)
		}
		if len(report.Coin.AI500) > 0 {
			if diff, ok := report.Coin.AI500["score"]; ok {
				fmt.Printf("  ai500_score_abs_diff: %.6f\n", diff.AbsDiff)
			}
			if diff, ok := report.Coin.AI500["rank"]; ok {
				fmt.Printf("  ai500_rank_abs_diff: %.0f\n", diff.AbsDiff)
			}
		}
		if len(report.Coin.PriceChange) > 0 {
			keys := make([]string, 0, len(report.Coin.PriceChange))
			for key := range report.Coin.PriceChange {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Printf("  price_change.%s abs_diff: %.6f\n", key, report.Coin.PriceChange[key].AbsDiff)
			}
		}
		if len(report.Coin.MissingSections) > 0 {
			for side, sections := range report.Coin.MissingSections {
				fmt.Printf("  missing_%s_sections: %s\n", side, strings.Join(sections, ", "))
			}
		}
		fmt.Println()
	}

	if len(report.Errors) > 0 {
		fmt.Printf("Errors\n")
		for _, item := range report.Errors {
			fmt.Printf("  - %s %s: %s\n", item.Side, item.Endpoint, item.Message)
		}
	}
}

func printRankedComparison(label string, report rankedSetComparison) {
	fmt.Printf("%s overlap=%.2f%% missing=%.2f%% jaccard=%.2f%% shared=%d/%d vs %d",
		label,
		report.OverlapRate*100,
		report.MissingRate*100,
		report.JaccardRate*100,
		report.SharedCount,
		report.ReferenceCount,
		report.CandidateCount,
	)
	if report.RankCorrelation != nil {
		fmt.Printf(" rank_corr=%.4f", *report.RankCorrelation)
	}
	fmt.Println()
	if len(report.ReferenceOnly) > 0 {
		fmt.Printf("%s reference_only: %s\n", label, strings.Join(report.ReferenceOnly, ", "))
	}
	if len(report.CandidateOnly) > 0 {
		fmt.Printf("%s candidate_only: %s\n", label, strings.Join(report.CandidateOnly, ", "))
	}
}
