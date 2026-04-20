package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentBacktestToolDefinitions(t *testing.T) {
	tools := AgentBacktestToolDefinitions()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Function.Name] = true
	}

	for _, required := range []string{
		ToolCreateStrategyAndBacktest,
		ToolGetBacktestResults,
		ToolImproveStrategy,
	} {
		if !names[required] {
			t.Fatalf("missing tool definition: %s", required)
		}
	}
}

func TestBacktestToolExecutorCreateStrategyAndBacktest(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run_id":"bt_123"}`))
	}))
	defer server.Close()

	exec := NewBacktestToolExecutor(server.URL, "token-123")
	resp, err := exec.Execute(context.Background(), ToolCreateStrategyAndBacktest, map[string]any{
		"config": map[string]any{"symbols": []string{"BTCUSDT"}},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if resp["run_id"] != "bt_123" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/backtest/create" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if gotAuth != "Bearer token-123" {
		t.Fatalf("unexpected auth header: %s", gotAuth)
	}
	if _, ok := gotBody["config"]; !ok {
		t.Fatalf("expected config in payload, got %+v", gotBody)
	}
}

func TestBacktestToolExecutorGetBacktestResults(t *testing.T) {
	var gotMethod, gotPath, include, eqLimit, tradeLimit string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		include = r.URL.Query().Get("include")
		eqLimit = r.URL.Query().Get("equity_limit")
		tradeLimit = r.URL.Query().Get("trade_limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run_id":"bt_abc","metrics_ready":true}`))
	}))
	defer server.Close()

	exec := NewBacktestToolExecutor(server.URL, "")
	resp, err := exec.Execute(context.Background(), ToolGetBacktestResults, map[string]any{
		"id":           "bt_abc",
		"include":      "equity,trades",
		"equity_limit": 100,
		"trade_limit":  25,
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if resp["run_id"] != "bt_abc" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/api/backtest/bt_abc" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if include != "equity,trades" {
		t.Fatalf("unexpected include query: %s", include)
	}
	if eqLimit != "100" {
		t.Fatalf("unexpected equity_limit: %s", eqLimit)
	}
	if tradeLimit != "25" {
		t.Fatalf("unexpected trade_limit: %s", tradeLimit)
	}
}

func TestBacktestToolExecutorImproveStrategy(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"auto_applied":true}`))
	}))
	defer server.Close()

	exec := NewBacktestToolExecutor(server.URL, "")
	resp, err := exec.Execute(context.Background(), ToolImproveStrategy, map[string]any{
		"id":         "bt_42",
		"goal":       "reduce drawdown",
		"auto_apply": true,
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if resp["auto_applied"] != true {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/backtest/bt_42/improve" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if _, ok := gotBody["id"]; ok {
		t.Fatalf("id should not be present in improve payload body: %+v", gotBody)
	}
	if gotBody["goal"] != "reduce drawdown" {
		t.Fatalf("unexpected goal: %+v", gotBody)
	}
}

func TestBacktestToolExecutorRequiresID(t *testing.T) {
	exec := NewBacktestToolExecutor("http://example.com", "")
	_, err := exec.Execute(context.Background(), ToolGetBacktestResults, map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("expected id required error, got %v", err)
	}
}
