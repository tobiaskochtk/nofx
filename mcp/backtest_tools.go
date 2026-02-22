package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ToolCreateStrategyAndBacktest = "create_strategy_and_backtest"
	ToolGetBacktestResults        = "get_backtest_results"
	ToolImproveStrategy           = "improve_strategy"
)

// AgentBacktestToolDefinitions returns MCP tool schemas for agent-driven backtest workflows.
func AgentBacktestToolDefinitions() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        ToolCreateStrategyAndBacktest,
				Description: "Create strategy (optional) and start a backtest run.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"config": map[string]any{
							"type":        "object",
							"description": "Backtest config payload compatible with POST /api/backtest/create",
						},
						"strategy": map[string]any{
							"type":        "object",
							"description": "Optional strategy input ({id} or {natural_language/config})",
						},
						"wait_for_completion": map[string]any{
							"type":        "boolean",
							"description": "Wait until terminal run state before returning",
						},
						"timeout_seconds": map[string]any{
							"type":        "integer",
							"description": "Wait timeout when wait_for_completion=true",
						},
					},
					"required": []string{"config"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        ToolGetBacktestResults,
				Description: "Fetch metadata, status, metrics, and optional series for a backtest run.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "Backtest run ID",
						},
						"include": map[string]any{
							"type":        "string",
							"description": "Optional CSV: equity,trades,decisions,all",
						},
						"equity_limit": map[string]any{
							"type":        "integer",
							"description": "Optional limit for equity points",
						},
						"trade_limit": map[string]any{
							"type":        "integer",
							"description": "Optional limit for trades",
						},
						"decision_limit": map[string]any{
							"type":        "integer",
							"description": "Optional limit for decisions",
						},
					},
					"required": []string{"id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        ToolImproveStrategy,
				Description: "Generate strategy improvements from backtest results, with optional auto-apply and rerun.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "Backtest run ID",
						},
						"goal": map[string]any{
							"type":        "string",
							"description": "Optional optimization objective",
						},
						"max_suggestions": map[string]any{
							"type":        "integer",
							"description": "Maximum suggestions to return/apply",
						},
						"auto_apply": map[string]any{
							"type":        "boolean",
							"description": "Whether to apply suggestions",
						},
						"save_as_strategy": map[string]any{
							"type":        "boolean",
							"description": "Persist improved config as strategy",
						},
						"strategy_name": map[string]any{
							"type":        "string",
							"description": "Name used when saving improved strategy",
						},
						"rerun": map[string]any{
							"type":        "boolean",
							"description": "Start a new backtest after applying changes",
						},
						"wait_for_completion": map[string]any{
							"type":        "boolean",
							"description": "Wait for rerun completion",
						},
						"timeout_seconds": map[string]any{
							"type":        "integer",
							"description": "Wait timeout for rerun completion",
						},
					},
					"required": []string{"id"},
				},
			},
		},
	}
}

// BacktestToolExecutor executes NOFX backtest MCP tools through NOFX HTTP API endpoints.
type BacktestToolExecutor struct {
	BaseURL    string
	AuthToken  string
	HTTPClient *http.Client
}

// NewBacktestToolExecutor creates an executor for NOFX agent backtest tools.
func NewBacktestToolExecutor(baseURL, authToken string) *BacktestToolExecutor {
	return &BacktestToolExecutor{
		BaseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		AuthToken: strings.TrimSpace(authToken),
		HTTPClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (e *BacktestToolExecutor) Tools() []Tool {
	return AgentBacktestToolDefinitions()
}

// Execute routes a tool call to the corresponding NOFX endpoint.
func (e *BacktestToolExecutor) Execute(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
	switch toolName {
	case ToolCreateStrategyAndBacktest:
		return e.CreateStrategyAndBacktest(ctx, args)
	case ToolGetBacktestResults:
		return e.GetBacktestResults(ctx, args)
	case ToolImproveStrategy:
		return e.ImproveStrategy(ctx, args)
	default:
		return nil, fmt.Errorf("unsupported tool: %s", toolName)
	}
}

func (e *BacktestToolExecutor) CreateStrategyAndBacktest(ctx context.Context, args map[string]any) (map[string]any, error) {
	return e.doJSONRequest(ctx, http.MethodPost, "/api/backtest/create", args, nil)
}

func (e *BacktestToolExecutor) GetBacktestResults(ctx context.Context, args map[string]any) (map[string]any, error) {
	runID, err := requiredStringArg(args, "id")
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	if include, ok := optionalStringArg(args, "include"); ok {
		query.Set("include", include)
	}
	if v, ok := optionalIntArg(args, "equity_limit"); ok {
		query.Set("equity_limit", strconv.Itoa(v))
	}
	if v, ok := optionalIntArg(args, "trade_limit"); ok {
		query.Set("trade_limit", strconv.Itoa(v))
	}
	if v, ok := optionalIntArg(args, "decision_limit"); ok {
		query.Set("decision_limit", strconv.Itoa(v))
	}

	path := "/api/backtest/" + url.PathEscape(runID)
	return e.doJSONRequest(ctx, http.MethodGet, path, nil, query)
}

func (e *BacktestToolExecutor) ImproveStrategy(ctx context.Context, args map[string]any) (map[string]any, error) {
	runID, err := requiredStringArg(args, "id")
	if err != nil {
		return nil, err
	}

	payload := copyMap(args)
	delete(payload, "id")

	path := "/api/backtest/" + url.PathEscape(runID) + "/improve"
	return e.doJSONRequest(ctx, http.MethodPost, path, payload, nil)
}

func (e *BacktestToolExecutor) doJSONRequest(ctx context.Context, method, path string, payload map[string]any, query url.Values) (map[string]any, error) {
	if strings.TrimSpace(e.BaseURL) == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	fullURL := e.BaseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if e.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.AuthToken)
	}

	client := e.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(body) == 0 {
			return nil, fmt.Errorf("api error status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("api error status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if len(body) == 0 {
		return map[string]any{}, nil
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response json: %w", err)
	}
	return result, nil
}

func requiredStringArg(args map[string]any, key string) (string, error) {
	raw, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if text == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return text, nil
}

func optionalStringArg(args map[string]any, key string) (string, bool) {
	raw, ok := args[key]
	if !ok {
		return "", false
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if text == "" {
		return "", false
	}
	return text, true
}

func optionalIntArg(args map[string]any, key string) (int, bool) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return int(i), true
		}
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

func copyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
