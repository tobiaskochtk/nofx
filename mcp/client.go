package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	ProviderCustom = "custom"
)

var (
	DefaultTimeout = 120 * time.Second
)

// Client AI API配置
type Client struct {
	Provider   string
	APIKey     string
	BaseURL    string
	Model      string
	UseFullURL bool // 是否使用完整URL（不添加/chat/completions）
	MaxTokens  int  // AI响应的最大token数

	httpClient *http.Client
	logger     Logger // 日志器（可替换）
	config     *Config // 配置对象（保存所有配置）

	// hooks 用于实现动态分派（多态）
	// 当 DeepSeekClient 嵌入 Client 时，hooks 指向 DeepSeekClient
	// 这样 call() 中调用的方法会自动分派到子类重写的版本
	hooks clientHooks
}

func New() AIClient {
	// 从环境变量读取 MaxTokens，默认 2000
	maxTokens := 2000
	if envMaxTokens := os.Getenv("AI_MAX_TOKENS"); envMaxTokens != "" {
		if parsed, err := strconv.Atoi(envMaxTokens); err == nil && parsed > 0 {
			maxTokens = parsed
			log.Printf("🔧 [MCP] 使用环境变量 AI_MAX_TOKENS: %d", maxTokens)
		} else {
			log.Printf("⚠️  [MCP] 环境变量 AI_MAX_TOKENS 无效 (%s)，使用默认值: %d", envMaxTokens, maxTokens)
		}
	}

	// 从环境变量读取 Timeout，默认 300 秒 (5分钟，适配 deepseek-reasoner)
	timeout := 300 * time.Second
	if envTimeout := os.Getenv("AI_TIMEOUT_SECONDS"); envTimeout != "" {
		if parsed, err := strconv.Atoi(envTimeout); err == nil && parsed > 0 {
			timeout = time.Duration(parsed) * time.Second
			log.Printf("🔧 [MCP] 使用环境变量 AI_TIMEOUT_SECONDS: %d 秒", parsed)
		} else {
			log.Printf("⚠️  [MCP] 环境变量 AI_TIMEOUT_SECONDS 无效 (%s)，使用默认值: 300 秒", envTimeout)
		}
	}

	// 默认配置
	return &Client{
		Provider:  ProviderDeepSeek,
		BaseURL:   "https://api.deepseek.com/v1",
		Model:     "deepseek-chat",
		Timeout:   timeout, // 默认 300 秒，适配 deepseek-reasoner 等慢速模型
		MaxTokens: maxTokens,
	}
}

// SetCustomAPI 设置自定义OpenAI兼容API
func (client *Client) SetAPIKey(apiKey, apiURL, customModel string) {
	client.Provider = ProviderCustom
	client.APIKey = apiKey

	// 检查URL是否以#结尾，如果是则使用完整URL（不添加/chat/completions）
	if strings.HasSuffix(apiURL, "#") {
		client.BaseURL = strings.TrimSuffix(apiURL, "#")
		client.UseFullURL = true
	} else {
		client.BaseURL = apiURL
		client.UseFullURL = false
	}

	client.Model = customModel
	// 自定义API也使用较长的超时时间，避免因模型推理慢导致超时
	client.Timeout = 300 * time.Second
}

// CallWithMessages 模板方法 - 固定的重试流程（不可重写）
func (client *Client) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	if client.APIKey == "" {
		return "", fmt.Errorf("AI API密钥未设置，请先调用 SetAPIKey")
	}

	// 固定的重试流程
	var lastErr error
	maxRetries := client.config.MaxRetries

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			client.logger.Warnf("⚠️  AI API调用失败，正在重试 (%d/%d)...", attempt, maxRetries)
		}

		// 调用固定的单次调用流程
		result, err := client.hooks.call(systemPrompt, userPrompt)
		if err == nil {
			if attempt > 1 {
				client.logger.Infof("✓ AI API重试成功")
			}
			return result, nil
		}

		lastErr = err
		// 通过 hooks 判断是否可重试（支持子类自定义重试策略）
		if !client.hooks.isRetryableError(err) {
			return "", err
		}

		// 重试前等待
		if attempt < maxRetries {
			waitTime := client.config.RetryWaitBase * time.Duration(attempt)
			client.logger.Infof("⏳ 等待%v后重试...", waitTime)
			time.Sleep(waitTime)
		}
	}

	return "", fmt.Errorf("重试%d次后仍然失败: %w", maxRetries, lastErr)
}

func (client *Client) setAuthHeader(reqHeader http.Header) {
	reqHeader.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
}

// callOnce 单次调用AI API（内部使用）
func (client *Client) callOnce(systemPrompt, userPrompt string) (string, error) {
	// 打印当前 AI 配置
	log.Printf("📡 [MCP] AI 请求配置:")
	log.Printf("   Provider: %s", client.Provider)
	log.Printf("   BaseURL: %s", client.BaseURL)
	log.Printf("   Model: %s", client.Model)
	log.Printf("   Timeout: %v", client.Timeout)
	log.Printf("   UseFullURL: %v", client.UseFullURL)
	if len(client.APIKey) > 8 {
		log.Printf("   API Key: %s...%s", client.APIKey[:4], client.APIKey[len(client.APIKey)-4:])
	}

func (client *Client) buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	// 构建 messages 数组
	messages := []map[string]string{}

	// 如果有 system prompt，添加 system message
	if systemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": systemPrompt,
		})
	}
	// 添加 user message
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userPrompt,
	})

	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       client.Model,
		"messages":    messages,
		"temperature": client.config.Temperature, // 使用配置的 temperature
		"max_tokens":  client.MaxTokens,
	}
	return requestBody
}

// can be used to marshal the request body and can be overridden
func (client *Client) marshalRequestBody(requestBody map[string]any) ([]byte, error) {
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}
	return jsonData, nil
}

	// 创建HTTP请求
	var url string
	if client.UseFullURL {
		// 使用完整URL，不添加/chat/completions
		url = client.BaseURL
	} else {
		// 默认行为：添加/chat/completions
		url = fmt.Sprintf("%s/chat/completions", client.BaseURL)
	}
	log.Printf("📡 [MCP] 请求 URL: %s", url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client.setAuthHeader(req.Header)

	// 发送请求
	httpClient := &http.Client{Timeout: client.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误 (status %d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("API返回空响应")
	}

	return result.Choices[0].Message.Content, nil
}

func (client *Client) buildUrl() string {
	if client.UseFullURL {
		return client.BaseURL
	}
	return fmt.Sprintf("%s/chat/completions", client.BaseURL)
}

func (client *Client) buildRequest(url string, jsonData []byte) (*http.Request, error) {
	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("fail to build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 通过 hooks 设置认证头（支持子类重写）
	client.hooks.setAuthHeader(req.Header)

	return req, nil
}

// call 单次调用AI API（固定流程，不可重写）
func (client *Client) call(systemPrompt, userPrompt string) (string, error) {
	// 打印当前 AI 配置
	client.logger.Infof("📡 [%s] Request AI Server: BaseURL: %s", client.String(), client.BaseURL)
	client.logger.Debugf("[%s] UseFullURL: %v", client.String(), client.UseFullURL)
	if len(client.APIKey) > 8 {
		client.logger.Debugf("[%s]   API Key: %s...%s", client.String(), client.APIKey[:4], client.APIKey[len(client.APIKey)-4:])
	}

	// Step 1: 构建请求体（通过 hooks 实现动态分派）
	requestBody := client.hooks.buildMCPRequestBody(systemPrompt, userPrompt)

	// Step 2: 序列化请求体（通过 hooks 实现动态分派）
	jsonData, err := client.hooks.marshalRequestBody(requestBody)
	if err != nil {
		return "", err
	}

	// Step 3: 构建 URL（通过 hooks 实现动态分派）
	url := client.hooks.buildUrl()
	client.logger.Infof("📡 [MCP %s] 请求 URL: %s", client.String(), url)

	// Step 4: 创建 HTTP 请求（固定逻辑）
	req, err := client.hooks.buildRequest(url, jsonData)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// Step 5: 发送 HTTP 请求（固定逻辑）
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// Step 6: 读取响应体（固定逻辑）
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// Step 7: 检查 HTTP 状态码（固定逻辑）
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误 (status %d): %s", resp.StatusCode, string(body))
	}

	// Step 8: 解析响应（通过 hooks 实现动态分派）
	result, err := client.hooks.parseMCPResponse(body)
	if err != nil {
		return "", fmt.Errorf("fail to parse AI server response: %w", err)
	}

	return result, nil
}

func (client *Client) String() string {
	return fmt.Sprintf("[Provider: %s, Model: %s]",
		client.Provider, client.Model)
}

// isRetryableError 判断错误是否可重试（网络错误、超时等）
func (client *Client) isRetryableError(err error) bool {
	errStr := err.Error()
	// 网络错误、超时、EOF等可以重试
	for _, retryable := range client.config.RetryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	return false
}

// ============================================================
// 构建器模式 API（高级功能）
// ============================================================

// CallWithRequest 使用 Request 对象调用 AI API（支持高级功能）
//
// 此方法支持：
// - 多轮对话历史
// - 精细参数控制（temperature、top_p、penalties 等）
// - Function Calling / Tools
// - 流式响应（未来支持）
//
// 使用示例：
//   request := NewRequestBuilder().
//       WithSystemPrompt("You are helpful").
//       WithUserPrompt("Hello").
//       WithTemperature(0.8).
//       Build()
//   result, err := client.CallWithRequest(request)
func (client *Client) CallWithRequest(req *Request) (string, error) {
	if client.APIKey == "" {
		return "", fmt.Errorf("AI API密钥未设置，请先调用 SetAPIKey")
	}

	// 如果 Request 中没有设置 Model，使用 Client 的 Model
	if req.Model == "" {
		req.Model = client.Model
	}

	// 固定的重试流程
	var lastErr error
	maxRetries := client.config.MaxRetries

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			client.logger.Warnf("⚠️  AI API调用失败，正在重试 (%d/%d)...", attempt, maxRetries)
		}

		// 调用单次请求
		result, err := client.callWithRequest(req)
		if err == nil {
			if attempt > 1 {
				client.logger.Infof("✓ AI API重试成功")
			}
			return result, nil
		}

		lastErr = err
		// 判断是否可重试
		if !client.hooks.isRetryableError(err) {
			return "", err
		}

		// 重试前等待
		if attempt < maxRetries {
			waitTime := client.config.RetryWaitBase * time.Duration(attempt)
			client.logger.Infof("⏳ 等待%v后重试...", waitTime)
			time.Sleep(waitTime)
		}
	}

	return "", fmt.Errorf("重试%d次后仍然失败: %w", maxRetries, lastErr)
}

// callWithRequest 单次调用 AI API（使用 Request 对象）
func (client *Client) callWithRequest(req *Request) (string, error) {
	// 打印当前 AI 配置
	client.logger.Infof("📡 [%s] Request AI Server with Builder: BaseURL: %s", client.String(), client.BaseURL)
	client.logger.Debugf("[%s] Messages count: %d", client.String(), len(req.Messages))

	// 构建请求体（从 Request 对象）
	requestBody := client.buildRequestBodyFromRequest(req)

	// 序列化请求体
	jsonData, err := client.hooks.marshalRequestBody(requestBody)
	if err != nil {
		return "", err
	}

	// 构建 URL
	url := client.hooks.buildUrl()
	client.logger.Infof("📡 [MCP %s] 请求 URL: %s", client.String(), url)

	// 创建 HTTP 请求
	httpReq, err := client.hooks.buildRequest(url, jsonData)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 发送 HTTP 请求
	resp, err := client.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误 (status %d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	result, err := client.hooks.parseMCPResponse(body)
	if err != nil {
		return "", fmt.Errorf("fail to parse AI server response: %w", err)
	}

	return result, nil
}

// buildRequestBodyFromRequest 从 Request 对象构建请求体
func (client *Client) buildRequestBodyFromRequest(req *Request) map[string]any {
	// 转换 Message 为 API 格式
	messages := make([]map[string]string, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	// 构建基础请求体
	requestBody := map[string]interface{}{
		"model":    req.Model,
		"messages": messages,
	}

	// 添加可选参数（只添加非 nil 的参数）
	if req.Temperature != nil {
		requestBody["temperature"] = *req.Temperature
	} else {
		// 如果 Request 中没有设置，使用 Client 的配置
		requestBody["temperature"] = client.config.Temperature
	}

	if req.MaxTokens != nil {
		requestBody["max_tokens"] = *req.MaxTokens
	} else {
		// 如果 Request 中没有设置，使用 Client 的 MaxTokens
		requestBody["max_tokens"] = client.MaxTokens
	}

	if req.TopP != nil {
		requestBody["top_p"] = *req.TopP
	}

	if req.FrequencyPenalty != nil {
		requestBody["frequency_penalty"] = *req.FrequencyPenalty
	}

	if req.PresencePenalty != nil {
		requestBody["presence_penalty"] = *req.PresencePenalty
	}

	if len(req.Stop) > 0 {
		requestBody["stop"] = req.Stop
	}

	if len(req.Tools) > 0 {
		requestBody["tools"] = req.Tools
	}

	if req.ToolChoice != "" {
		requestBody["tool_choice"] = req.ToolChoice
	}

	if req.Stream {
		requestBody["stream"] = true
	}

	return requestBody
}
