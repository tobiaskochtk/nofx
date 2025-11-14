package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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
	Timeout    time.Duration
	UseFullURL bool // 是否使用完整URL（不添加/chat/completions）
	MaxTokens  int  // AI响应的最大token数
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

	client.Model = modelName
	// 自定义API也使用较长的超时时间，避免因模型推理慢导致超时
	client.Timeout = 300 * time.Second
}

// CallWithMessages 使用 system + user prompt 调用AI API（推荐）
func (client *Client) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	if client.APIKey == "" {
		return "", fmt.Errorf("AI API密钥未设置，请先调用 SetAPIKey")
	}

	// 重试配置
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("⚠️  AI API调用失败，正在重试 (%d/%d)...\n", attempt, maxRetries)
		}

		result, err := client.callOnce(systemPrompt, userPrompt)
		if err == nil {
			if attempt > 1 {
				fmt.Printf("✓ AI API重试成功\n")
			}
			return result, nil
		}

		lastErr = err
		// 如果不是网络错误，不重试
		if !isRetryableError(err) {
			return "", err
		}

		// 重试前等待
		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * 2 * time.Second
			fmt.Printf("⏳ 等待%v后重试...\n", waitTime)
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
		"temperature": 0.5, // 降低temperature以提高JSON格式稳定性
		"max_tokens":  client.MaxTokens,
	}

	// 注意：response_format 参数仅 OpenAI 支持，DeepSeek/Qwen 不支持
	// 我们通过强化 prompt 和后处理来确保 JSON 格式正确

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
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

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	errStr := err.Error()
	// 网络错误、超时、EOF等可以重试
	retryableErrors := []string{
		"EOF",
		"timeout",
		"connection reset",
		"connection refused",
		"temporary failure",
		"no such host",
		"stream error",   // HTTP/2 stream 错误
		"INTERNAL_ERROR", // 服务端内部错误
	}
	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	return false
}
