package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ProviderN8n           = "n8n"
	DefaultN8nEndpoint    = "http://localhost:5678/webhook/ai/decision"
	DefaultN8nModel       = "gpt-5-codex"
)

// N8nClient 调用自托管 n8n Webhook 作为 AI 推理端点。
// 该端点遵循简单 JSON 契约: {request_id, prompt, max_runtime_s, model} → {status, decision_raw}。
type N8nClient struct {
	*Client
	maxRuntimeSeconds int
}

// NewN8nClient 创建 n8n 客户端，默认从环境变量读取：
//   - N8N_AI_ENDPOINT (Webhook URL)
//   - N8N_AI_TOKEN (可选 Bearer Token)
//   - N8N_AI_MODEL (model 字段，默认 gpt-5-codex)
//   - N8N_AI_MAX_RUNTIME_S (max_runtime_s，默认 300)
func NewN8nClient(opts ...ClientOption) AIClient {
	baseURL := strings.TrimSpace(getEnvString("N8N_AI_ENDPOINT", DefaultN8nEndpoint))
	model := strings.TrimSpace(getEnvString("N8N_AI_MODEL", DefaultN8nModel))
	maxRuntime := getEnvInt("N8N_AI_MAX_RUNTIME_S", 300)

	n8nOpts := []ClientOption{
		WithProvider(ProviderN8n),
		WithBaseURL(baseURL),
		WithModel(model),
		WithAllowEmptyAPIKey(true),
	}
	allOpts := append(n8nOpts, opts...)
	baseClient := NewClient(allOpts...).(*Client)

	client := &N8nClient{
		Client:            baseClient,
		maxRuntimeSeconds: maxRuntime,
	}

	// 默认 token 从环境变量读取（可被 SetAPIKey 覆盖）
	if baseClient.APIKey == "" {
		baseClient.APIKey = strings.TrimSpace(getEnvString("N8N_AI_TOKEN", ""))
	}

	baseClient.Hooks = client
	return client
}

// SetAPIKey 自定义 n8n 鉴权/地址/模型（API Key 可为空）。
func (n *N8nClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	n.APIKey = strings.TrimSpace(apiKey)
	if n.APIKey == "" {
		n.APIKey = strings.TrimSpace(getEnvString("N8N_AI_TOKEN", ""))
	}

	if strings.TrimSpace(customURL) != "" {
		n.BaseURL = strings.TrimSpace(customURL)
	} else if n.BaseURL == "" {
		n.BaseURL = strings.TrimSpace(getEnvString("N8N_AI_ENDPOINT", DefaultN8nEndpoint))
	}

	if strings.TrimSpace(customModel) != "" {
		n.Model = strings.TrimSpace(customModel)
	} else if n.Model == "" {
		n.Model = strings.TrimSpace(getEnvString("N8N_AI_MODEL", DefaultN8nModel))
	}
}

// CallWithRequest 将高级请求折叠为 system+user 的组合，调用标准消息接口。
func (n *N8nClient) CallWithRequest(req *Request) (string, error) {
	var systemPrompt, userPrompt string
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			if systemPrompt == "" {
				systemPrompt = msg.Content
			}
		case "user":
			// 取最后一条 user 消息作为主体
			userPrompt = msg.Content
		}
	}

	return n.CallWithMessages(systemPrompt, userPrompt)
}

func (n *N8nClient) BuildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	promptParts := []string{}

	if sp := strings.TrimSpace(systemPrompt); sp != "" {
		promptParts = append(promptParts, "System Prompt:\n"+sp)
	}
	if up := strings.TrimSpace(userPrompt); up != "" {
		promptParts = append(promptParts, "User Prompt:\n"+up)
	}

	combinedPrompt := strings.TrimSpace(strings.Join(promptParts, "\n\n"))
	if combinedPrompt == "" {
		combinedPrompt = strings.TrimSpace(userPrompt)
	}

	body := map[string]any{
		"request_id": fmt.Sprintf("nofx-%s", uuid.NewString()),
		"prompt":     combinedPrompt,
	}

	if n.maxRuntimeSeconds > 0 {
		body["max_runtime_s"] = n.maxRuntimeSeconds
	}
	if n.Model != "" {
		body["model"] = n.Model
	}

	return body
}

func (n *N8nClient) BuildUrl() string {
	return n.BaseURL
}

func (n *N8nClient) SetAuthHeader(headers http.Header) {
	if n.APIKey != "" {
		headers.Set("Authorization", fmt.Sprintf("Bearer %s", n.APIKey))
	}
}

func (n *N8nClient) ParseMCPResponse(body []byte) (string, error) {
	var resp struct {
		Status       string `json:"status"`
		DecisionRaw  string `json:"decision_raw"`
		ModelInfo    string `json:"model_info"`
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("解析 n8n 响应失败: %w", err)
	}

	if strings.ToLower(resp.Status) != "ok" {
		msg := strings.TrimSpace(resp.ErrorMessage)
		if msg == "" {
			msg = strings.TrimSpace(resp.Status)
		}
		if resp.ErrorCode != "" {
			msg = fmt.Sprintf("%s (%s)", msg, resp.ErrorCode)
		}
		return "", fmt.Errorf("n8n 返回错误: %s", msg)
	}

	if strings.TrimSpace(resp.DecisionRaw) == "" {
		return "", fmt.Errorf("n8n 响应缺少 decision_raw 字段")
	}

	return resp.DecisionRaw, nil
}

// 保持与其他客户端一致的重试策略
func (n *N8nClient) IsRetryableError(err error) bool {
	// 复用基础客户端的重试关键字（含网络/超时）
	return n.Client.IsRetryableError(err)
}

// 为日志中标记 Provider/Model 提供可读输出
func (n *N8nClient) String() string {
	return fmt.Sprintf("[Provider: %s, Model: %s, MaxRuntime: %ds]", n.Provider, n.Model, n.maxRuntimeSeconds)
}

// 允许在运行时动态调整 max_runtime_s
func (n *N8nClient) SetMaxRuntime(seconds int) {
	if seconds > 0 {
		n.maxRuntimeSeconds = seconds
	}
}

// 再暴露一次超时设置，便于调用方直接复用
func (n *N8nClient) SetTimeout(timeout time.Duration) {
	n.Client.SetTimeout(timeout)
}
