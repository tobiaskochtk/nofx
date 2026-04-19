package kernel

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
	"regexp"
	"strings"
	"time"
)

const (
	// Keep generous headroom below provider hard-limits.
	defaultPromptInputTokenBudget = 118000
	retryPromptInputTokenBudget   = 90000
	minPromptSectionTokenBudget   = 6000
	tokenCharsEstimate            = 3 // conservative estimate: ~1 token per 3 chars
	promptTruncationNotice        = "\n\n[... prompt truncated due to context budget ...]\n\n"
)

// ============================================================================
// Pre-compiled regular expressions (performance optimization)
// ============================================================================

var (
	// Safe regex: capture fenced JSON content without assuming array vs envelope object.
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(.*?)\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

	// XML tag extraction (supports any characters in reasoning chain)
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
)

// ============================================================================
// Entry Functions - Main API
// ============================================================================

// GetFullDecision gets AI's complete trading decision (batch analysis of all coins and positions)
// Uses default strategy configuration - for production use GetFullDecisionWithStrategy with explicit config
func GetFullDecision(ctx *Context, mcpClient mcp.AIClient) (*FullDecision, error) {
	defaultConfig := store.GetDefaultStrategyConfig("en")
	engine := NewStrategyEngine(&defaultConfig)
	return GetFullDecisionWithStrategy(ctx, mcpClient, engine, "")
}

// GetFullDecisionWithStrategy uses StrategyEngine to get AI decision (unified prompt generation)
func GetFullDecisionWithStrategy(ctx *Context, mcpClient mcp.AIClient, engine *StrategyEngine, variant string) (*FullDecision, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if engine == nil {
		defaultConfig := store.GetDefaultStrategyConfig("en")
		engine = NewStrategyEngine(&defaultConfig)
	}

	// Clamp strategy limits to prevent token overflow
	engineConfig := engine.GetConfig()
	engineConfig.ClampLimits()

	// Token estimation check — block if exceeding the specific model's context limit
	estimate := engineConfig.EstimateTokens()

	// Determine context limit for the specific model being used
	contextLimit := 131072 // safe default (strictest common limit)
	var providerName string
	if embedder, ok := mcpClient.(mcp.ClientEmbedder); ok {
		base := embedder.BaseClient()
		providerName = base.Provider
		contextLimit = store.GetContextLimitForClient(base.Provider, base.Model)
	}

	if estimate.Total > contextLimit {
		logger.Errorf("🚫 Token estimate %d exceeds %s context limit %d — blocking analysis",
			estimate.Total, providerName, contextLimit)
		return nil, fmt.Errorf("estimated %d tokens exceeds model context limit of %d; reduce coins, timeframes, or K-line count",
			estimate.Total, contextLimit)
	}
	if estimate.Total*100/contextLimit >= 80 {
		logger.Infof("⚠️  Token estimate %d — approaching %s context limit %d",
			estimate.Total, providerName, contextLimit)
	}

	// 1. Fetch market data using strategy config
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataWithStrategy(ctx, engine); err != nil {
			return nil, fmt.Errorf("failed to fetch market data: %w", err)
		}
	}

	// Ensure OITopDataMap is initialized
	if ctx.OITopDataMap == nil {
		ctx.OITopDataMap = make(map[string]*OITopData)
		oiPositions, err := engine.nofxosClient.GetOITopPositions()
		if err == nil {
			for _, pos := range oiPositions {
				ctx.OITopDataMap[pos.Symbol] = &OITopData{
					Rank:              pos.Rank,
					OIDeltaPercent:    pos.OIDeltaPercent,
					OIDeltaValue:      pos.OIDeltaValue,
					PriceDeltaPercent: pos.PriceDeltaPercent,
				}
			}
		}
	}

	// 2. Build System Prompt using strategy engine
	riskConfig := engine.GetRiskControlConfig()
	systemPrompt := engine.BuildSystemPrompt(ctx.Account.TotalEquity, variant)

	// 3. Build User Prompt using strategy engine
	userPrompt, err := buildDecisionPayloadPrompt(ctx, riskConfig.MaxPositions)
	if err != nil {
		logger.Warnf("⚠️  Failed to build compact payload prompt, fallback to legacy formatter: %v", err)
		userPrompt = engine.BuildUserPrompt(ctx)
	}

	origInputTokens := estimatePromptTokens(systemPrompt) + estimatePromptTokens(userPrompt)
	systemPrompt, userPrompt, trimmed := fitPromptsToBudget(systemPrompt, userPrompt, defaultPromptInputTokenBudget)
	if trimmed {
		newInputTokens := estimatePromptTokens(systemPrompt) + estimatePromptTokens(userPrompt)
		logger.Warnf("⚠️  Prompt exceeded budget (%d est. tokens), trimmed to %d est. tokens",
			origInputTokens, newInputTokens)
	}

	// 4. Call AI API
	aiCallStart := time.Now()
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		if isContextLengthError(err) {
			retrySystemPrompt, retryUserPrompt, retryTrimmed := fitPromptsToBudget(systemPrompt, userPrompt, retryPromptInputTokenBudget)
			if retryTrimmed {
				logger.Warnf("⚠️  Retrying AI call with aggressive prompt trimming (budget: %d est. tokens)",
					retryPromptInputTokenBudget)
				aiCallStart = time.Now()
				aiResponse, err = mcpClient.CallWithMessages(retrySystemPrompt, retryUserPrompt)
				aiCallDuration = time.Since(aiCallStart)
				if err == nil {
					systemPrompt = retrySystemPrompt
					userPrompt = retryUserPrompt
				}
			}
		}
		if err != nil {
			if isEmptyAIContentError(err) {
				logger.Warnf("⚠️  AI returned empty content, forcing no-trade envelope for cycle %d", ctx.CallCount)
				aiResponse = buildNoTradeDecisionEnvelope(ctx.CallCount)
				err = nil
			}
		}
		if err != nil {
			return nil, fmt.Errorf("AI API call failed: %w", err)
		}
	}
	if strings.TrimSpace(aiResponse) == "" {
		logger.Warnf("⚠️  AI response was blank, forcing no-trade envelope for cycle %d", ctx.CallCount)
		aiResponse = buildNoTradeDecisionEnvelope(ctx.CallCount)
	}

	// 5. Parse AI response
	decision, err := parseFullDecisionResponse(
		aiResponse,
		ctx.Account.TotalEquity,
		riskConfig.BTCETHMaxLeverage,
		riskConfig.AltcoinMaxLeverage,
		riskConfig.BTCETHMaxPositionValueRatio,
		riskConfig.AltcoinMaxPositionValueRatio,
	)

	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt
		decision.UserPrompt = userPrompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
		decision.RawResponse = aiResponse
	}

	if err != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}

func estimatePromptTokens(s string) int {
	if s == "" {
		return 0
	}
	// Conservative estimate to avoid provider-side hard-limit failures.
	return (len(s) + tokenCharsEstimate - 1) / tokenCharsEstimate
}

func fitPromptsToBudget(systemPrompt, userPrompt string, inputTokenBudget int) (string, string, bool) {
	if inputTokenBudget <= 0 {
		return systemPrompt, userPrompt, false
	}

	systemTokens := estimatePromptTokens(systemPrompt)
	userTokens := estimatePromptTokens(userPrompt)
	if systemTokens+userTokens <= inputTokenBudget {
		return systemPrompt, userPrompt, false
	}

	trimmed := false

	maxSystemTokens := inputTokenBudget - minPromptSectionTokenBudget
	if maxSystemTokens < minPromptSectionTokenBudget {
		maxSystemTokens = minPromptSectionTokenBudget
	}
	if systemTokens > maxSystemTokens {
		systemPrompt = trimPromptMiddle(systemPrompt, maxSystemTokens)
		systemTokens = estimatePromptTokens(systemPrompt)
		trimmed = true
	}

	remainingUserTokens := inputTokenBudget - systemTokens
	if remainingUserTokens < minPromptSectionTokenBudget {
		remainingUserTokens = minPromptSectionTokenBudget
	}
	if userTokens > remainingUserTokens {
		userPrompt = trimPromptMiddle(userPrompt, remainingUserTokens)
		trimmed = true
	}

	return systemPrompt, userPrompt, trimmed
}

func trimPromptMiddle(s string, targetTokens int) string {
	if targetTokens <= 0 || s == "" {
		return s
	}

	targetChars := targetTokens * tokenCharsEstimate
	runes := []rune(s)
	if len(runes) <= targetChars {
		return s
	}

	noticeRunes := []rune(promptTruncationNotice)
	minKeep := 512
	if targetChars <= len(noticeRunes)+minKeep*2 {
		tail := targetChars - len(noticeRunes)
		if tail < minKeep {
			tail = minKeep
		}
		if tail > len(runes) {
			tail = len(runes)
		}
		return string(append(noticeRunes, runes[len(runes)-tail:]...))
	}

	contentBudget := targetChars - len(noticeRunes)
	head := int(float64(contentBudget) * 0.58)
	tail := contentBudget - head
	if head < minKeep {
		head = minKeep
		tail = contentBudget - head
	}
	if tail < minKeep {
		tail = minKeep
		head = contentBudget - tail
	}
	if head < 0 {
		head = 0
	}
	if tail < 0 {
		tail = 0
	}
	if head+tail > len(runes) {
		return s
	}

	var b strings.Builder
	b.Grow(head + len(noticeRunes) + tail)
	b.WriteString(string(runes[:head]))
	b.WriteString(promptTruncationNotice)
	b.WriteString(string(runes[len(runes)-tail:]))
	return b.String()
}

func isContextLengthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "maximum context length") ||
		strings.Contains(msg, "reduce the length of the messages") ||
		(strings.Contains(msg, "requested ") && strings.Contains(msg, "tokens"))
}

func logMarketDataFetchIssue(subject string, err error) {
	if err == nil {
		return
	}
	if market.IsExpectedDataMiss(err) {
		logger.Debugf("Skipping %s due to expected market-data miss: %v", subject, err)
		return
	}
	logger.Infof("⚠️  Failed to fetch %s: %v", subject, err)
}

// ============================================================================
// Market Data Fetching
// ============================================================================

// fetchMarketDataWithStrategy fetches market data using strategy config (multiple timeframes)
func fetchMarketDataWithStrategy(ctx *Context, engine *StrategyEngine) error {
	config := engine.GetConfig()
	ctx.MarketDataMap = make(map[string]*market.Data)

	timeframes := config.Indicators.Klines.SelectedTimeframes
	primaryTimeframe := config.Indicators.Klines.PrimaryTimeframe
	klineCount := config.Indicators.Klines.PrimaryCount

	// Compatible with old configuration
	if len(timeframes) == 0 {
		if primaryTimeframe != "" {
			timeframes = append(timeframes, primaryTimeframe)
		} else {
			timeframes = append(timeframes, "3m")
		}
		if config.Indicators.Klines.LongerTimeframe != "" {
			timeframes = append(timeframes, config.Indicators.Klines.LongerTimeframe)
		}
	}
	if primaryTimeframe == "" {
		primaryTimeframe = timeframes[0]
	}
	if klineCount <= 0 {
		klineCount = 30
	}

	logger.Infof("📊 Strategy timeframes: %v, Primary: %s, Kline count: %d", timeframes, primaryTimeframe, klineCount)

	const benchmarkSymbol = "BTCUSDT"
	if _, exists := ctx.MarketDataMap[benchmarkSymbol]; !exists {
		benchmarkData, err := market.GetWithTimeframes(benchmarkSymbol, timeframes, primaryTimeframe, klineCount)
		if err != nil {
			logMarketDataFetchIssue(fmt.Sprintf("benchmark market data for %s", benchmarkSymbol), err)
		} else {
			ctx.MarketDataMap[benchmarkSymbol] = benchmarkData
		}
	}

	// 1. First fetch data for position coins (must fetch)
	for _, pos := range ctx.Positions {
		data, err := market.GetWithTimeframes(pos.Symbol, timeframes, primaryTimeframe, klineCount)
		if err != nil {
			logMarketDataFetchIssue(fmt.Sprintf("market data for position %s", pos.Symbol), err)
			continue
		}
		ctx.MarketDataMap[pos.Symbol] = data
	}

	// 2. Fetch data for all candidate coins
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	const minOIThresholdMillions = 15.0 // 15M USD minimum open interest value

	for _, coin := range ctx.CandidateCoins {
		if _, exists := ctx.MarketDataMap[coin.Symbol]; exists {
			continue
		}

		data, err := market.GetWithTimeframes(coin.Symbol, timeframes, primaryTimeframe, klineCount)
		if err != nil {
			logMarketDataFetchIssue(fmt.Sprintf("market data for %s", coin.Symbol), err)
			continue
		}

		// Liquidity filter (skip for xyz dex assets - they don't have OI data from Binance)
		isExistingPosition := positionSymbols[coin.Symbol]
		isXyzAsset := market.IsXyzDexAsset(coin.Symbol)
		if !isExistingPosition && !isXyzAsset && data.OpenInterest != nil && data.CurrentPrice > 0 {
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000
			if oiValueInMillions < minOIThresholdMillions {
				logger.Debugf("Skipping %s because OI value is too low (%.2fM USD < %.1fM)",
					coin.Symbol, oiValueInMillions, minOIThresholdMillions)
				continue
			}
		}

		ctx.MarketDataMap[coin.Symbol] = data
	}

	// Align feature freshness across the whole cycle. Snapshot construction is serial,
	// so early symbols can otherwise age out before the prompt is assembled.
	refreshAt := time.Now().UTC()
	for _, data := range ctx.MarketDataMap {
		data.TouchFeatureStats(refreshAt)
	}

	logger.Infof("📊 Successfully fetched multi-timeframe market data for %d coins", len(ctx.MarketDataMap))
	return nil
}

// ============================================================================
// AI Response Parsing
// ============================================================================

func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) (*FullDecision, error) {
	cotTrace := extractCoTTrace(aiResponse)

	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("failed to extract decisions: %w", err)
	}

	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("decision validation failed: %w", err)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

func extractCoTTrace(response string) string {
	response = removeInvisibleRunes(response)
	trimmed := strings.TrimSpace(response)

	if match := reReasoningTag.FindStringSubmatch(response); match != nil && len(match) > 1 {
		logger.Infof("✓ Extracted reasoning chain using <reasoning> tag")
		return strings.TrimSpace(match[1])
	}

	if isJSONOnlyResponse(trimmed) {
		return ""
	}

	if decisionIdx := strings.Index(response, "<decision>"); decisionIdx > 0 {
		logger.Infof("✓ Extracted content before <decision> tag as reasoning chain")
		return strings.TrimSpace(response[:decisionIdx])
	}

	jsonStart := strings.IndexAny(response, "[{")
	if jsonStart > 0 {
		logger.Infof("⚠️  Extracted reasoning chain using legacy JSON separator")
		return strings.TrimSpace(response[:jsonStart])
	}

	return trimmed
}

func extractDecisions(response string) ([]Decision, error) {
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)
	s = fixMissingQuotes(s)

	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); match != nil && len(match) > 1 {
		jsonPart = strings.TrimSpace(match[1])
		logger.Infof("✓ Extracted JSON using <decision> tag")
	} else {
		jsonPart = s
		logger.Infof("⚠️  <decision> tag not found, searching JSON in full text")
	}

	jsonContent, found := extractDecisionJSONFragment(jsonPart)
	if !found || strings.TrimSpace(jsonContent) == "" {
		trimmedPart := strings.TrimSpace(jsonPart)
		if strings.HasPrefix(trimmedPart, "```json") || trimmedPart == "```" || trimmedPart == "```json" {
			logger.Infof("⚠️  Empty fenced JSON artifact detected, treating as no-trade output")
			return []Decision{}, nil
		}
		logger.Infof("⚠️  [SafeFallback] AI didn't output JSON decision, entering safe wait mode")

		cotSummary := jsonPart
		if len(cotSummary) > 240 {
			cotSummary = cotSummary[:240] + "..."
		}

		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("Model didn't output structured JSON decision, entering safe wait; summary: %s", cotSummary),
		}

		return []Decision{fallbackDecision}, nil
	}

	decisions, err := parseDecisionJSONContent(jsonContent)
	if err != nil {
		return nil, fmt.Errorf("%w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
	}
	return decisions, nil
}

func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")

	jsonStr = strings.ReplaceAll(jsonStr, "［", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{")
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}")
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":")
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",")

	jsonStr = strings.ReplaceAll(jsonStr, "【", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",")

	jsonStr = strings.ReplaceAll(jsonStr, "　", " ")

	return jsonStr
}

func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)
	if trimmed == "" {
		return fmt.Errorf("JSON content is empty")
	}
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") && strings.TrimSpace(trimmed[1:len(trimmed)-1]) == "" {
		return nil
	}

	if !reArrayHead.MatchString(trimmed) {
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	if strings.Contains(jsonStr, "~") {
		return fmt.Errorf("JSON cannot contain range symbol ~, all numbers must be precise single values")
	}

	for i := 0; i < len(jsonStr)-4; i++ {
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			return fmt.Errorf("JSON numbers cannot contain thousand separator comma, found: %s", jsonStr[i:min(i+10, len(jsonStr))])
		}
	}

	return nil
}

func parseDecisionJSONContent(jsonContent string) ([]Decision, error) {
	trimmed := strings.TrimSpace(fixMissingQuotes(jsonContent))
	if trimmed == "" {
		return nil, fmt.Errorf("JSON parsing failed: decision content empty")
	}

	switch trimmed[0] {
	case '[':
		return parseDecisionArray(trimmed)
	case '{':
		type decisionEnvelope struct {
			Decisions json.RawMessage `json:"decisions"`
		}
		var envelope decisionEnvelope
		if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
			return nil, fmt.Errorf("JSON parsing failed: %w", err)
		}
		if len(envelope.Decisions) == 0 {
			return nil, fmt.Errorf("JSON parsing failed: decision envelope missing decisions field")
		}
		inner := strings.TrimSpace(string(envelope.Decisions))
		if inner == "" || inner == "null" {
			return []Decision{}, nil
		}
		return parseDecisionArray(inner)
	default:
		return nil, fmt.Errorf("JSON must start with [ or {, actual: %s", trimmed[:min(20, len(trimmed))])
	}
}

func parseDecisionArray(jsonContent string) ([]Decision, error) {
	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent)
	if err := validateJSONFormat(jsonContent); err != nil {
		return nil, fmt.Errorf("JSON format validation failed: %w", err)
	}
	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w", err)
	}
	return decisions, nil
}

func extractDecisionJSONFragment(s string) (string, bool) {
	s = fixMissingQuotes(strings.TrimSpace(s))
	if s == "" {
		return "", false
	}
	if match := reJSONFence.FindStringSubmatch(s); match != nil && len(match) > 1 {
		return strings.TrimSpace(match[1]), true
	}
	if fragment, ok := extractBalancedJSON(s); ok {
		return fragment, true
	}
	if array := strings.TrimSpace(reJSONArray.FindString(s)); array != "" {
		return array, true
	}
	return "", false
}

func extractBalancedJSON(s string) (string, bool) {
	start := -1
	for i, r := range s {
		if r == '{' || r == '[' {
			start = i
			break
		}
	}
	if start < 0 {
		return "", false
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[start : i+1]), true
			}
			if depth < 0 {
				return "", false
			}
		}
	}
	return "", false
}

func isJSONOnlyResponse(s string) bool {
	if s == "" {
		return false
	}
	if match := reJSONFence.FindStringSubmatch(s); match != nil && len(match) > 1 && strings.TrimSpace(match[0]) == s {
		return true
	}
	if fragment, ok := extractDecisionJSONFragment(s); ok && strings.TrimSpace(fragment) == s {
		return true
	}
	return false
}

func isEmptyAIContentError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "empty content") ||
		strings.Contains(msg, "returned empty") ||
		strings.Contains(msg, "no content") ||
		strings.Contains(msg, "response body is empty")
}

func buildNoTradeDecisionEnvelope(cycle int) string {
	type noTradeEnvelope struct {
		TimestampUTC string     `json:"ts_utc"`
		Cycle        int        `json:"cycle"`
		Decisions    []Decision `json:"decisions"`
	}
	payload := noTradeEnvelope{
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
		Cycle:        cycle,
		Decisions:    []Decision{},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return `{"ts_utc":"","cycle":0,"decisions":[]}`
	}
	return string(encoded)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}
