package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"nofx/codexaudit"
	"nofx/mcp"
)

const (
	DefaultCodexBaseURL = "codex://cli"
	DefaultCodexModel   = "gpt-5.4"
)

func init() {
	mcp.RegisterProvider(mcp.ProviderCodex, func(opts ...mcp.ClientOption) mcp.AIClient {
		return NewCodexClientWithOptions(opts...)
	})
}

type CodexClient struct {
	*mcp.Client
}

func (c *CodexClient) BaseClient() *mcp.Client { return c.Client }

func NewCodexClient() mcp.AIClient {
	return NewCodexClientWithOptions()
}

func NewCodexClientWithOptions(opts ...mcp.ClientOption) mcp.AIClient {
	codexOpts := []mcp.ClientOption{
		mcp.WithProvider(mcp.ProviderCodex),
		mcp.WithModel(DefaultCodexModel),
		mcp.WithBaseURL(DefaultCodexBaseURL),
		mcp.WithAllowEmptyAPIKey(true),
	}

	allOpts := append(codexOpts, opts...)
	baseClient := mcp.NewClient(allOpts...).(*mcp.Client)

	codexClient := &CodexClient{
		Client: baseClient,
	}

	baseClient.Hooks = codexClient
	return codexClient
}

func (c *CodexClient) SetAPIKey(_ string, customURL string, customModel string) {
	c.APIKey = ""
	c.BaseURL = DefaultCodexBaseURL

	if strings.TrimSpace(customURL) != "" {
		c.Log.Warnf("⚠️  [MCP] Codex ignores custom BaseURL %q because requests run through the local Codex CLI", customURL)
	}

	if model := strings.TrimSpace(customModel); model != "" {
		c.Model = model
	} else if strings.TrimSpace(c.Model) == "" {
		c.Model = DefaultCodexModel
	}

	c.Log.Infof("🔧 [MCP] Codex using local CLI auth with model: %s", c.Model)
}

func (c *CodexClient) SetTimeout(timeout time.Duration) {
	c.Client.SetTimeout(timeout)
	if c.Cfg != nil {
		c.Cfg.Timeout = timeout
	}
}

func (c *CodexClient) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	model := strings.TrimSpace(c.Model)
	if model == "" {
		model = DefaultCodexModel
	}
	prompt := buildCodexMessagesPrompt(systemPrompt, userPrompt)
	return c.runPromptWithRetry(prompt, model)
}

func (c *CodexClient) Call(systemPrompt, userPrompt string) (string, error) {
	model := strings.TrimSpace(c.Model)
	if model == "" {
		model = DefaultCodexModel
	}
	prompt := buildCodexMessagesPrompt(systemPrompt, userPrompt)
	return c.runPrompt(prompt, model)
}

func (c *CodexClient) CallWithRequest(req *mcp.Request) (string, error) {
	resp, err := c.callRequest(req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (c *CodexClient) CallWithRequestFull(req *mcp.Request) (*mcp.LLMResponse, error) {
	return c.callRequest(req)
}

func (c *CodexClient) CallWithRequestStream(req *mcp.Request, onChunk func(string)) (string, error) {
	resp, err := c.callRequest(req)
	if err != nil {
		return "", err
	}
	if onChunk != nil && strings.TrimSpace(resp.Content) != "" {
		onChunk(resp.Content)
	}
	return resp.Content, nil
}

func (c *CodexClient) callRequest(req *mcp.Request) (*mcp.LLMResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if len(req.Tools) > 0 {
		return nil, fmt.Errorf("Codex CLI provider does not support NOFX tool-calling requests")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(c.Model)
	}
	if model == "" {
		model = DefaultCodexModel
	}

	prompt := buildCodexRequestPrompt(req)
	content, err := c.runPromptWithRetry(prompt, model)
	if err != nil {
		return nil, err
	}

	return &mcp.LLMResponse{Content: content}, nil
}

func (c *CodexClient) runPromptWithRetry(prompt, model string) (string, error) {
	maxRetries := 1
	if c.Cfg != nil && c.Cfg.MaxRetries > 0 {
		maxRetries = c.Cfg.MaxRetries
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			c.Log.Warnf("⚠️  Codex CLI call failed, retrying (%d/%d)...", attempt, maxRetries)
		}

		result, err := c.runPrompt(prompt, model)
		if err == nil {
			if attempt > 1 {
				c.Log.Infof("✓ Codex CLI retry succeeded")
			}
			return result, nil
		}

		lastErr = err
		if !c.IsRetryableError(err) {
			return "", err
		}
		if attempt >= maxRetries {
			break
		}

		waitTime := time.Duration(0)
		if c.Cfg != nil {
			waitTime = c.Cfg.RetryWaitBase * time.Duration(attempt)
		}
		if waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("Codex CLI request failed")
	}
	if maxRetries <= 1 {
		return "", lastErr
	}
	return "", fmt.Errorf("still failed after %d retries: %w", maxRetries, lastErr)
}

func (c *CodexClient) runPrompt(prompt, model string) (result string, err error) {
	startedAt := time.Now().UTC()
	var (
		codexBinary string
		args        []string
		outputFile  string
		exitCode    int
		combined    bytes.Buffer
	)
	defer func() {
		c.recordCodexInvocation(startedAt, time.Now().UTC(), codexBinary, args, prompt, result, combined.String(), exitCode, model, err)
	}()

	codexBinary, err = findCodexExecutable()
	if err != nil {
		exitCode = -1
		return "", err
	}

	tempRoot, err := os.MkdirTemp("", "nofx-codex-")
	if err != nil {
		exitCode = -1
		return "", fmt.Errorf("failed to create Codex workspace: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	workspaceDir := filepath.Join(tempRoot, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		exitCode = -1
		return "", fmt.Errorf("failed to create Codex workspace directory: %w", err)
	}

	homeDir := filepath.Join(tempRoot, "home")
	if err := prepareCodexHome(homeDir); err != nil {
		exitCode = -1
		return "", err
	}

	outputFile = filepath.Join(workspaceDir, "last-message.txt")
	timeout := 120 * time.Second
	if c.Cfg != nil && c.Cfg.Timeout > 0 {
		timeout = c.Cfg.Timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args = []string{
		"exec",
		"-m", model,
		"-s", "read-only",
		"-c", `ask_for_approval="never"`,
		"-c", "search=false",
		"-c", "mcp_servers={}",
		"-C", workspaceDir,
		"--skip-git-repo-check",
		"--color", "never",
		"--output-last-message", outputFile,
		"-",
	}

	cmd := exec.CommandContext(ctx, codexBinary, args...)
	cmd.Env = codexCommandEnv(os.Environ(), homeDir)
	cmd.Stdin = strings.NewReader(prompt)

	cmd.Stdout = &combined
	cmd.Stderr = &combined

	c.Log.Infof("📡 [MCP %s] Requesting local Codex CLI model: %s", c.String(), model)
	if err := cmd.Run(); err != nil {
		exitCode = codexExitCode(err)
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("Codex CLI request timed out after %s", timeout)
		}

		details := strings.TrimSpace(combined.String())
		if details != "" {
			return "", fmt.Errorf("Codex CLI request failed: %w: %s", err, details)
		}
		return "", fmt.Errorf("Codex CLI request failed: %w", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		details := strings.TrimSpace(combined.String())
		if details != "" {
			return "", fmt.Errorf("failed to read Codex CLI output: %w: %s", err, details)
		}
		return "", fmt.Errorf("failed to read Codex CLI output: %w", err)
	}

	result = strings.TrimSpace(string(content))
	if result == "" {
		details := strings.TrimSpace(combined.String())
		if details != "" {
			return "", fmt.Errorf("Codex CLI returned an empty response: %s", details)
		}
		return "", fmt.Errorf("Codex CLI returned an empty response")
	}

	return result, nil
}

func (c *CodexClient) recordCodexInvocation(startedAt, finishedAt time.Time, codexBinary string, args []string, prompt, responseText, combinedOutput string, exitCode int, model string, runErr error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		argsJSON = []byte("[]")
	}

	caller := c.Caller.Normalized()
	entry := &codexaudit.LogEntry{
		RequestStartedAt:   startedAt,
		ResponseFinishedAt: &finishedAt,
		CallerType:         caller.CallerType,
		CallerID:           caller.CallerID,
		CallerName:         caller.CallerName,
		UserID:             caller.UserID,
		TraderID:           caller.TraderID,
		TraderName:         caller.TraderName,
		Component:          caller.Component,
		CycleNumber:        caller.CycleNumber,
		Provider:           mcp.ProviderCodex,
		Model:              strings.TrimSpace(model),
		Transport:          "codex_cli",
		CodexBinary:        strings.TrimSpace(codexBinary),
		CommandArgsJSON:    string(argsJSON),
		RequestPrompt:      prompt,
		ResponseText:       responseText,
		CombinedOutput:     combinedOutput,
		Success:            runErr == nil,
		ExitCode:           exitCode,
		DurationMs:         finishedAt.Sub(startedAt).Milliseconds(),
	}
	if runErr != nil {
		entry.ErrorMessage = runErr.Error()
	}

	if err := codexaudit.RecordGlobal(entry); err != nil {
		c.Log.Warnf("⚠️  Failed to persist Codex CLI call log: %v", err)
	}
}

func codexExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func findCodexExecutable() (string, error) {
	candidates := []string{}
	if configured := strings.TrimSpace(os.Getenv("CODEX_CLI_PATH")); configured != "" {
		candidates = append(candidates, configured)
	}
	candidates = append(candidates, "codex", "codex.cmd")

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("Codex CLI executable not found in PATH")
}

func codexCommandEnv(base []string, homeDir string) []string {
	env := append([]string(nil), base...)
	env = setCommandEnv(env, "HOME", homeDir)
	env = setCommandEnv(env, "USERPROFILE", homeDir)
	return env
}

func resolveCodexSourceConfigDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("CODEX_CONFIG_DIR")); configured != "" {
		return filepath.Clean(configured), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve Codex config directory: %w", err)
	}
	if strings.TrimSpace(homeDir) == "" {
		return "", fmt.Errorf("failed to resolve Codex config directory: user home is empty")
	}
	return filepath.Join(homeDir, ".codex"), nil
}

func prepareCodexHome(homeDir string) error {
	sourceConfigDir, err := resolveCodexSourceConfigDir()
	if err != nil {
		return err
	}

	targetConfigDir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(targetConfigDir, 0o755); err != nil {
		return fmt.Errorf("failed to prepare Codex home directory: %w", err)
	}

	for _, filename := range []string{"auth.json", "models_cache.json"} {
		source := filepath.Join(sourceConfigDir, filename)
		target := filepath.Join(targetConfigDir, filename)
		if err := copyCodexFile(source, target); err != nil {
			return err
		}
	}

	return nil
}

func copyCodexFile(sourcePath, targetPath string) error {
	body, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read Codex file %s: %w", sourcePath, err)
	}
	if err := os.WriteFile(targetPath, body, 0o600); err != nil {
		return fmt.Errorf("failed to write Codex file %s: %w", targetPath, err)
	}
	return nil
}

func setCommandEnv(env []string, key, value string) []string {
	prefix := key + "="
	for index, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[index] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func buildCodexMessagesPrompt(systemPrompt, userPrompt string) string {
	messages := make([]mcp.Message, 0, 2)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, mcp.NewSystemMessage(systemPrompt))
	}
	messages = append(messages, mcp.NewUserMessage(userPrompt))
	return buildCodexConversationPrompt(messages)
}

func buildCodexRequestPrompt(req *mcp.Request) string {
	if req == nil {
		return buildCodexConversationPrompt(nil)
	}
	return buildCodexConversationPrompt(req.Messages)
}

func buildCodexConversationPrompt(messages []mcp.Message) string {
	var builder strings.Builder
	builder.WriteString("You are answering a request inside the NOFX application.\n")
	builder.WriteString("Do not run shell commands, inspect the working directory, use web search, or use any external tools.\n")
	builder.WriteString("Reply only with the assistant message for the conversation below.\n\n")
	builder.WriteString("Conversation:\n")

	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "user"
		}
		builder.WriteString("[")
		builder.WriteString(role)
		builder.WriteString("]\n")
		builder.WriteString(buildCodexMessageContent(msg))
		builder.WriteString("\n\n")
	}

	builder.WriteString("Assistant reply:\n")
	return builder.String()
}

func buildCodexMessageContent(msg mcp.Message) string {
	content := strings.TrimSpace(msg.Content)
	if len(msg.ToolCalls) == 0 && msg.ToolCallID == "" {
		if content == "" {
			return "(empty)"
		}
		return content
	}

	var builder strings.Builder
	if content != "" {
		builder.WriteString(content)
		builder.WriteString("\n")
	}

	if msg.ToolCallID != "" {
		builder.WriteString("Tool response ID: ")
		builder.WriteString(strings.TrimSpace(msg.ToolCallID))
		builder.WriteString("\n")
	}

	for _, toolCall := range msg.ToolCalls {
		builder.WriteString("Tool call: ")
		builder.WriteString(strings.TrimSpace(toolCall.Function.Name))
		args := strings.TrimSpace(toolCall.Function.Arguments)
		if args != "" {
			builder.WriteString(" ")
			builder.WriteString(args)
		}
		builder.WriteString("\n")
	}

	result := strings.TrimSpace(builder.String())
	if result == "" {
		return "(empty)"
	}
	return result
}
