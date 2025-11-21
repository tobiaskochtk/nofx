package trader

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/config"
	"nofx/decision"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
)

// AutoTraderConfig 自动交易配置（简化版 - AI全权决策）
type AutoTraderConfig struct {
	// Trader标识
	ID      string // Trader唯一标识（用于日志目录等）
	Name    string // Trader显示名称
	AIModel string // AI模型: "qwen" 或 "deepseek"

	// 交易平台选择
	Exchange string // "binance", "hyperliquid" 或 "aster"

	// 币安API配置
	BinanceAPIKey    string
	BinanceSecretKey string

	// Hyperliquid配置
	HyperliquidPrivateKey string
	HyperliquidWalletAddr string
	HyperliquidTestnet    bool

	// Aster配置
	AsterUser       string // Aster主钱包地址
	AsterSigner     string // Aster API钱包地址
	AsterPrivateKey string // Aster API钱包私钥

	// AI配置
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string

	// 自定义AI API配置
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// 扫描配置
	ScanInterval time.Duration // 扫描间隔（建议3分钟）

	// 账户配置
	InitialBalance float64 // 初始金额（用于计算盈亏，需手动设置）

	// 杠杆配置
	BTCETHLeverage  int // BTC和ETH的杠杆倍数
	AltcoinLeverage int // 山寨币的杠杆倍数

	// 风险控制（仅作为提示，AI可自主决定）
	MaxDailyLoss    float64       // 最大日亏损百分比（提示）
	MaxDrawdown     float64       // 最大回撤百分比（提示）
	StopTradingTime time.Duration // 触发风控后暂停时长

	// 仓位模式
	IsCrossMargin bool // true=全仓模式, false=逐仓模式

	// 币种配置
	DefaultCoins []string // 默认币种列表（从数据库获取）
	TradingCoins []string // 实际交易币种列表

	// 持仓限制
	MaxPositions int // 最大持仓数量（默认: 3）

	// 系统提示词模板
	SystemPromptTemplate string // 系统提示词模板名称（如 "default", "aggressive"）
}

// TrailingStopConfig 自动追踪止损配置
type TrailingStopConfig struct {
	Enabled            bool               // 是否启用自动追踪止损
	Tiers              []TrailingStopTier // 分级止损配置（按利润阈值升序排列）
	UpdateThresholdPct float64            // 最小更新阈值百分比（防止频繁API调用）
	CheckIntervalSec   int                // 检查间隔（秒）
	AllowAIOverride    bool               // 是否允许AI覆盖自动止损
}

// TrailingStopTier 追踪止损分级配置
type TrailingStopTier struct {
	ProfitThreshold float64 // 利润阈值（%）- 达到此利润时激活此档位
	StopOffset      float64 // 止损偏移量（%）- 负值表示固定利润目标，正值表示当前利润减去此值
}

// AutoTrader 自动交易器
type AutoTrader struct {
	id                    string // Trader唯一标识
	name                  string // Trader显示名称
	aiModel               string // AI模型名称
	exchange              string // 交易平台名称
	config                AutoTraderConfig
	trader                Trader // 使用Trader接口（支持多平台）
	mcpClient             mcp.AIClient
	decisionLogger        logger.IDecisionLogger // 决策日志记录器
	initialBalance        float64
	dailyPnL              float64
	customPrompt          string   // 自定义交易策略prompt
	overrideBasePrompt    bool     // 是否覆盖基础prompt
	systemPromptTemplate  string   // 系统提示词模板名称
	defaultCoins          []string // 默认币种列表（从数据库获取）
	tradingCoins          []string // 实际交易币种列表
	lastResetTime         time.Time
	stopUntil             time.Time
	isRunning             bool
	startTime             time.Time          // 系统启动时间
	callCount             int                // AI调用次数
	positionFirstSeenTime map[string]int64   // 持仓首次出现时间 (symbol_side -> timestamp毫秒)
	stopMonitorCh         chan struct{}      // 用于停止监控goroutine
	monitorWg             sync.WaitGroup     // 用于等待监控goroutine结束
	peakPnLCache          map[string]float64 // 最高收益缓存 (symbol -> 峰值盈亏百分比)
	peakPnLCacheMutex     sync.RWMutex       // 缓存读写锁
	lastBalanceSyncTime   time.Time          // 上次余额同步时间
	database              interface{}        // 数据库引用（用于自动更新余额）
	userID                string             // 用户ID
	// deals persistence helpers
	lastSystemPrompt string
	lastUserPrompt   string
	lastCoTTrace     string
	lastDecisionJSON string
	openDealIDs      map[string]int64 // symbol_side -> dealID
	// Trailing Stop Management
	trailingStopConfig     TrailingStopConfig // 追踪止损配置
	trailingStopManaged    map[string]bool    // AI是否管理此止损 (symbol_side -> true if AI-managed)
	trailingStopLastUpdate map[string]float64 // 上次设置的止损价格 (symbol_side -> stopPrice)
	trailingStopActiveTier map[string]int     // 当前激活的档位 (symbol_side -> tier index, -1 if none)
	trailingStopMutex      sync.RWMutex       // 追踪止损状态锁
	trailingStopMonitorCh  chan struct{}      // 追踪止损监控停止信号
}

const manualDealCloseGrace = 2 * time.Minute

// parseTrailingStopTiers 解析追踪止损分级配置
// 格式: "threshold1:offset1,threshold2:offset2,..."
// 例如: "0.5:-0.2,1.0:0.5,3.0:1.0,10.0:3.0"
// 负offset表示固定利润目标，正offset表示当前利润减去此值
func parseTrailingStopTiers(tiersStr string) []TrailingStopTier {
	if tiersStr == "" {
		// 默认配置
		return []TrailingStopTier{
			{ProfitThreshold: 0.5, StopOffset: -0.2}, // >=0.5% → 止损在0.2%利润
			{ProfitThreshold: 1.0, StopOffset: 0.5},  // >=1.0% → 止损在利润-0.5%
			{ProfitThreshold: 3.0, StopOffset: 1.0},  // >=3.0% → 止损在利润-1.0%
			{ProfitThreshold: 10.0, StopOffset: 3.0}, // >=10.0% → 止损在利润-3.0%
		}
	}

	var tiers []TrailingStopTier
	pairs := strings.Split(tiersStr, ",")
	for _, pair := range pairs {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) != 2 {
			continue
		}
		threshold, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		offset, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil {
			tiers = append(tiers, TrailingStopTier{
				ProfitThreshold: threshold,
				StopOffset:      offset,
			})
		}
	}

	// 按利润阈值升序排序
	sort.Slice(tiers, func(i, j int) bool {
		return tiers[i].ProfitThreshold < tiers[j].ProfitThreshold
	})

	return tiers
}

// getEnvFloat 从环境变量读取浮点数（支持 .env 注入）
func getEnvFloat(name string, def float64) float64 {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return def
}

// NewAutoTrader 创建自动交易器
func NewAutoTrader(config AutoTraderConfig, database interface{}, userID string) (*AutoTrader, error) {
	// 设置默认值
	if config.ID == "" {
		config.ID = "default_trader"
	}
	if config.Name == "" {
		config.Name = "Default Trader"
	}
	if config.AIModel == "" {
		if config.UseQwen {
			config.AIModel = "qwen"
		} else {
			config.AIModel = "deepseek"
		}
	}

	mcpClient := mcp.New()

	// 初始化AI
	if config.AIModel == "custom" {
		// 使用自定义API
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		log.Printf("🤖 [%s] 使用自定义AI API: %s (模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
	} else if config.UseQwen || config.AIModel == "qwen" {
		// 使用Qwen (支持自定义URL和Model)
		mcpClient = mcp.NewQwenClient()
		mcpClient.SetAPIKey(config.QwenKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] 使用阿里云Qwen AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] 使用阿里云Qwen AI", config.Name)
		}
	} else {
		// 默认使用DeepSeek (支持自定义URL和Model)
		mcpClient = mcp.NewDeepSeekClient()
		mcpClient.SetAPIKey(config.DeepSeekKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] 使用DeepSeek AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] 使用DeepSeek AI", config.Name)
		}
	}

	// 设置默认交易平台
	if config.Exchange == "" {
		config.Exchange = "binance"
	}

	// 根据配置创建对应的交易器
	var trader Trader
	var err error

	// 记录仓位模式（通用）
	marginModeStr := "全仓"
	if !config.IsCrossMargin {
		marginModeStr = "逐仓"
	}
	log.Printf("📊 [%s] 仓位模式: %s", config.Name, marginModeStr)

	switch config.Exchange {
	case "binance":
		log.Printf("🏦 [%s] 使用币安合约交易", config.Name)
		trader = NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey, userID)
	case "hyperliquid":
		log.Printf("🏦 [%s] 使用Hyperliquid交易", config.Name)
		trader, err = NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
		if err != nil {
			return nil, fmt.Errorf("初始化Hyperliquid交易器失败: %w", err)
		}
	case "aster":
		log.Printf("🏦 [%s] 使用Aster交易", config.Name)
		trader, err = NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("初始化Aster交易器失败: %w", err)
		}
	default:
		return nil, fmt.Errorf("不支持的交易平台: %s", config.Exchange)
	}

	// 验证初始金额配置
	if config.InitialBalance <= 0 {
		return nil, fmt.Errorf("初始金额必须大于0，请在配置中设置InitialBalance")
	}

	// 初始化决策日志记录器（使用trader ID创建独立目录）
	logDir := fmt.Sprintf("decision_logs/%s", config.ID)
	decisionLogger := logger.NewDecisionLogger(logDir)

	// 设置默认系统提示词模板
	systemPromptTemplate := config.SystemPromptTemplate
	if systemPromptTemplate == "" {
		// feature/partial-close-dynamic-tpsl 分支默认使用 adaptive（支持动态止盈止损）
		systemPromptTemplate = "adaptive"
	}

	// 从数据库加载追踪止损配置（trader-specific），如果数据库中没有则使用环境变量
	var trailingStopConfig TrailingStopConfig
	if db, ok := database.(interface {
		GetSystemConfig(key string) (string, error)
	}); ok {
		enabled, _ := db.GetSystemConfig(fmt.Sprintf("trailing_stop_%s_enabled", config.ID))
		tiers, _ := db.GetSystemConfig(fmt.Sprintf("trailing_stop_%s_tiers", config.ID))
		updateThresholdStr, _ := db.GetSystemConfig(fmt.Sprintf("trailing_stop_%s_update_threshold_pct", config.ID))
		checkIntervalStr, _ := db.GetSystemConfig(fmt.Sprintf("trailing_stop_%s_check_interval_sec", config.ID))
		allowAIOverrideStr, _ := db.GetSystemConfig(fmt.Sprintf("trailing_stop_%s_allow_ai_override", config.ID))

		// 如果数据库中有配置，使用数据库配置
		if enabled != "" {
			trailingStopConfig.Enabled = enabled == "true"
			trailingStopConfig.Tiers = parseTrailingStopTiers(tiers)
			if val, err := strconv.ParseFloat(updateThresholdStr, 64); err == nil && val > 0 {
				trailingStopConfig.UpdateThresholdPct = val
			} else {
				trailingStopConfig.UpdateThresholdPct = 0.3
			}
			if val, err := strconv.Atoi(checkIntervalStr); err == nil && val > 0 {
				trailingStopConfig.CheckIntervalSec = val
			} else {
				trailingStopConfig.CheckIntervalSec = 30
			}
			trailingStopConfig.AllowAIOverride = allowAIOverrideStr == "true"
		} else {
			// 数据库中没有配置，使用环境变量作为后备
			trailingStopConfig = TrailingStopConfig{
				Enabled:            os.Getenv("TRAILING_STOP_ENABLED") == "true",
				Tiers:              parseTrailingStopTiers(os.Getenv("TRAILING_STOP_TIERS")),
				UpdateThresholdPct: getEnvFloat("TRAILING_STOP_UPDATE_THRESHOLD_PCT", 0.3),
				CheckIntervalSec:   int(getEnvFloat("TRAILING_STOP_CHECK_INTERVAL_SEC", 30)),
				AllowAIOverride:    os.Getenv("TRAILING_STOP_ALLOW_AI_OVERRIDE") == "true",
			}
		}
	} else {
		// 如果数据库不支持GetSystemConfig，使用环境变量
		trailingStopConfig = TrailingStopConfig{
			Enabled:            os.Getenv("TRAILING_STOP_ENABLED") == "true",
			Tiers:              parseTrailingStopTiers(os.Getenv("TRAILING_STOP_TIERS")),
			UpdateThresholdPct: getEnvFloat("TRAILING_STOP_UPDATE_THRESHOLD_PCT", 0.3),
			CheckIntervalSec:   int(getEnvFloat("TRAILING_STOP_CHECK_INTERVAL_SEC", 30)),
			AllowAIOverride:    os.Getenv("TRAILING_STOP_ALLOW_AI_OVERRIDE") == "true",
		}
	}

	// 日志输出配置状态
	if trailingStopConfig.Enabled {
		log.Printf("🎯 [%s] 自动追踪止损已启用 (分级系统):", config.Name)
		for i, tier := range trailingStopConfig.Tiers {
			if tier.StopOffset < 0 {
				log.Printf("   档位%d: 利润≥%.1f%% → 止损固定在 %.1f%% 利润",
					i+1, tier.ProfitThreshold, -tier.StopOffset)
			} else {
				log.Printf("   档位%d: 利润≥%.1f%% → 止损在 利润-%.1f%%",
					i+1, tier.ProfitThreshold, tier.StopOffset)
			}
		}
		log.Printf("   更新阈值: %.1f%%, 检查间隔: %d秒, AI覆盖: %v",
			trailingStopConfig.UpdateThresholdPct,
			trailingStopConfig.CheckIntervalSec,
			trailingStopConfig.AllowAIOverride)
	} else {
		log.Printf("⏸️ [%s] 自动追踪止损已禁用", config.Name)
	}

	return &AutoTrader{
		id:                     config.ID,
		name:                   config.Name,
		aiModel:                config.AIModel,
		exchange:               config.Exchange,
		config:                 config,
		trader:                 trader,
		mcpClient:              mcpClient,
		decisionLogger:         decisionLogger,
		initialBalance:         config.InitialBalance,
		systemPromptTemplate:   systemPromptTemplate,
		defaultCoins:           config.DefaultCoins,
		tradingCoins:           config.TradingCoins,
		lastResetTime:          time.Now(),
		startTime:              time.Now(),
		callCount:              0,
		isRunning:              false,
		positionFirstSeenTime:  make(map[string]int64),
		stopMonitorCh:          make(chan struct{}),
		monitorWg:              sync.WaitGroup{},
		peakPnLCache:           make(map[string]float64),
		peakPnLCacheMutex:      sync.RWMutex{},
		lastBalanceSyncTime:    time.Now(), // 初始化为当前时间
		database:               database,
		userID:                 userID,
		openDealIDs:            make(map[string]int64),
		trailingStopConfig:     trailingStopConfig,
		trailingStopManaged:    make(map[string]bool),
		trailingStopLastUpdate: make(map[string]float64),
		trailingStopActiveTier: make(map[string]int),
		trailingStopMutex:      sync.RWMutex{},
	}, nil
}

// Run 运行自动交易主循环
func (at *AutoTrader) Run() error {
	log.Printf("🐛 [Run] ENTRY - at==nil: %v", at == nil)
	
	at.isRunning = true
	log.Printf("🐛 [Run] Set isRunning=true")
	
	at.stopMonitorCh = make(chan struct{})
	log.Printf("🐛 [Run] Created stopMonitorCh")
	
	at.startTime = time.Now()
	log.Printf("🐛 [Run] Set startTime")

	log.Println("🚀 AI驱动自动交易系统启动")
	log.Printf("💰 初始余额: %.2f USDT", at.initialBalance)
	log.Printf("⚙️  扫描间隔: %v", at.config.ScanInterval)
	log.Println("🤖 AI将全权决定杠杆、仓位大小、止损止盈等参数")
	at.monitorWg.Add(1)
	defer at.monitorWg.Done()

	// 启动回撤监控
	at.startDrawdownMonitor()

	// 启动追踪止损监控（如果启用）
	if at.trailingStopConfig.Enabled {
		at.startTrailingStopMonitor()
	}

	ticker := time.NewTicker(at.config.ScanInterval)
	defer ticker.Stop()

	// 首次立即执行
	if err := at.runCycle(); err != nil {
		log.Printf("❌ 执行失败: %v", err)
	}

	for at.isRunning {
		select {
		case <-ticker.C:
			if err := at.runCycle(); err != nil {
				log.Printf("❌ 执行失败: %v", err)
			}
		case <-at.stopMonitorCh:
			log.Printf("[%s] ⏹ 收到停止信号，退出自动交易主循环", at.name)
			return nil
		}
	}

	return nil
}

// Stop 停止自动交易
func (at *AutoTrader) Stop() {
	if !at.isRunning {
		return
	}
	at.isRunning = false
	close(at.stopMonitorCh) // 通知监控goroutine停止

	// 等待监控goroutine结束，但最多等待10秒
	done := make(chan struct{})
	go func() {
		at.monitorWg.Wait()
		close(done)
	}()

	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	select {
	case <-done:
		log.Println("⏹ 自动交易系统停止")
	case <-timeout.C:
		log.Println("⚠️  监控goroutine停止超时，强制退出")
	}
}

// runCycle 运行一个交易周期（使用AI全权决策）
func (at *AutoTrader) runCycle() error {
	at.callCount++

	log.Print("\n" + strings.Repeat("=", 70) + "\n")
	log.Printf("⏰ %s - AI决策周期 #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
	log.Println(strings.Repeat("=", 70))

	// 创建决策记录
	record := &logger.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
	}

	// 1. 检查是否需要停止交易
	if time.Now().Before(at.stopUntil) {
		remaining := at.stopUntil.Sub(time.Now())
		log.Printf("⏸ 风险控制：暂停交易中，剩余 %.0f 分钟", remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("风险控制暂停中，剩余 %.0f 分钟", remaining.Minutes())
		at.decisionLogger.LogDecision(record)
		return nil
	}

	// 2. 重置日盈亏（每天重置）
	if time.Since(at.lastResetTime) > 24*time.Hour {
		at.dailyPnL = 0
		at.lastResetTime = time.Now()
		log.Println("📅 日盈亏已重置")
	}

	// 3. 自动同步余额（每10分钟检查一次，充值/提现后自动更新）
	// 暂时禁用：此功能会将初始余额错误地更新为可用余额，导致P&L计算错误
	// TODO: 改进逻辑，使用钱包总余额(totalWalletBalance)而非可用余额进行比较
	// at.autoSyncBalanceIfNeeded()

	// 4. 收集交易上下文
	ctx, err := at.buildTradingContext()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("构建交易上下文失败: %v", err)
		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("构建交易上下文失败: %w", err)
	}

	// 保存账户状态快照
	record.AccountState = logger.AccountSnapshot{
		TotalBalance:          ctx.Account.TotalEquity - ctx.Account.UnrealizedPnL,
		AvailableBalance:      ctx.Account.AvailableBalance,
		TotalUnrealizedProfit: ctx.Account.UnrealizedPnL,
		PositionCount:         ctx.Account.PositionCount,
		MarginUsedPct:         ctx.Account.MarginUsedPct,
		InitialBalance:        at.initialBalance, // 记录当时的初始余额基准
	}

	// 保存持仓快照
	for _, pos := range ctx.Positions {
		record.Positions = append(record.Positions, logger.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        pos.MarkPrice,
			UnrealizedProfit: pos.UnrealizedPnL,
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
		})
	}

	log.Print(strings.Repeat("=", 70))
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	log.Printf("📊 账户净值: %.2f USDT | 可用: %.2f USDT | 持仓: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 5. 调用AI获取完整决策
	log.Printf("🤖 正在请求AI分析并决策... [模板: %s]", at.systemPromptTemplate)
	decision, err := decision.GetFullDecisionWithCustomPrompt(ctx, at.mcpClient, at.customPrompt, at.overrideBasePrompt, at.systemPromptTemplate)

	if decision != nil && decision.AIRequestDurationMs > 0 {
		record.AIRequestDurationMs = decision.AIRequestDurationMs
		log.Printf("⏱️ AI调用耗时: %.2f 秒", float64(record.AIRequestDurationMs)/1000)
		record.ExecutionLog = append(record.ExecutionLog,
			fmt.Sprintf("AI调用耗时: %d ms", record.AIRequestDurationMs))
	}

	// 即使有错误，也保存思维链、决策和输入prompt（用于debug）
	if decision != nil {
		record.SystemPrompt = decision.SystemPrompt // 保存系统提示词
		record.InputPrompt = decision.UserPrompt
		record.CoTTrace = decision.CoTTrace
		if len(decision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(decision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
		// 缓存到AutoTrader，便于持久化到deals
		at.lastSystemPrompt = record.SystemPrompt
		at.lastUserPrompt = record.InputPrompt
		at.lastCoTTrace = record.CoTTrace
		at.lastDecisionJSON = record.DecisionJSON
	}

	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("获取AI决策失败: %v", err)

		// 打印系统提示词和AI思维链（即使有错误，也要输出以便调试）
		if decision != nil {
			log.Print("\n" + strings.Repeat("=", 70) + "\n")
			log.Printf("📋 系统提示词 [模板: %s] (错误情况)", at.systemPromptTemplate)
			log.Println(strings.Repeat("=", 70))
			log.Println(decision.SystemPrompt)
			log.Println(strings.Repeat("=", 70))

			if decision.CoTTrace != "" {
				log.Print("\n" + strings.Repeat("-", 70) + "\n")
				log.Println("💭 AI思维链分析（错误情况）:")
				log.Println(strings.Repeat("-", 70))
				log.Println(decision.CoTTrace)
				log.Println(strings.Repeat("-", 70))
			}
		}

		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("获取AI决策失败: %w", err)
	}

	// // 5. 打印系统提示词
	// log.Printf("\n" + strings.Repeat("=", 70))
	// log.Printf("📋 系统提示词 [模板: %s]", at.systemPromptTemplate)
	// log.Println(strings.Repeat("=", 70))
	// log.Println(decision.SystemPrompt)
	// log.Printf(strings.Repeat("=", 70) + "\n")

	// 6. 打印AI思维链
	// log.Printf("\n" + strings.Repeat("-", 70))
	// log.Println("💭 AI思维链分析:")
	// log.Println(strings.Repeat("-", 70))
	// log.Println(decision.CoTTrace)
	// log.Printf(strings.Repeat("-", 70) + "\n")

	// 7. 打印AI决策
	// log.Printf("📋 AI决策列表 (%d 个):\n", len(decision.Decisions))
	// for i, d := range decision.Decisions {
	//     log.Printf("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
	//     if d.Action == "open_long" || d.Action == "open_short" {
	//        log.Printf("      杠杆: %dx | 仓位: %.2f USDT | 止损: %.4f | 止盈: %.4f",
	//           d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
	//     }
	// }
	log.Println()
	log.Print(strings.Repeat("-", 70))
	// 8. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	log.Print(strings.Repeat("-", 70))

	// 8. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	sortedDecisions := sortDecisionsByPriority(decision.Decisions)

	log.Println("🔄 执行顺序（已优化）: 先平仓→后开仓")
	for i, d := range sortedDecisions {
		log.Printf("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	log.Println()

	// 执行决策并记录结果
	for _, d := range sortedDecisions {
		actionRecord := logger.DecisionAction{
			Action:    d.Action,
			Symbol:    d.Symbol,
			Quantity:  0,
			Leverage:  d.Leverage,
			Price:     0,
			Timestamp: time.Now(),
			Success:   false,
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			log.Printf("❌ 执行决策失败 (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 失败: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 成功", d.Symbol, d.Action))
			// 成功执行后短暂延迟
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 9. 保存决策记录
	if err := at.decisionLogger.LogDecision(record); err != nil {
		log.Printf("⚠ 保存决策记录失败: %v", err)
	}

	return nil
}

// buildTradingContext 构建交易上下文
func (at *AutoTrader) buildTradingContext() (*decision.Context, error) {
	// 1. 获取账户信息
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	// 获取账户字段（只需可用余额）
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity（账户净值）
	// 对于合约账户更贴近实际的口径：可用余额 + 已占用保证金
	// Hyperliquid / 合约场景: accountValue ≈ available + marginUsed
	// 如果没有持仓（marginUsed=0），则退化为 totalEquity = availableBalance
	// 注意：totalWalletBalance = (accountValue - unrealizedPnl) + spotBalance
	// 旧口径（wallet+unrealized）在存在 Spot 余额时会把 Spot 也计入，从而与前端显示不一致
	// 统一改为 available + marginUsed，保持与用户期望一致
	// 先占位，待计算出 totalMarginUsed 后再赋值
	totalEquity := 0.0

	// 2. 获取持仓信息
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0
	totalUnrealizedProfit := 0.0

	// 当前持仓的key集合（用于清理已平仓的记录）
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := strings.ToLower(pos["side"].(string))
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // 空仓数量为负，转为正数
		}

		// 跳过已平仓的持仓（quantity = 0），防止"幽灵持仓"传递给AI
		if quantity == 0 {
			continue
		}

		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)
		totalUnrealizedProfit += unrealizedPnl

		// 计算占用保证金（估算）
		leverage := 10 // 默认值，实际应该从持仓信息获取
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// 计算盈亏百分比（基于保证金，考虑杠杆）
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// 跟踪持仓首次出现时间
		posKey := makePositionKey(symbol, side)
		currentPositionKeys[posKey] = true
		if _, exists := at.positionFirstSeenTime[posKey]; !exists {
			// 新持仓，记录当前时间
			at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
		}
		updateTime := at.positionFirstSeenTime[posKey]

		// 获取该持仓的历史最高收益率
		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		positionInfos = append(positionInfos, decision.PositionInfo{
			Symbol:           symbol,
			Side:             side,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			Quantity:         quantity,
			Leverage:         leverage,
			UnrealizedPnL:    unrealizedPnl,
			UnrealizedPnLPct: pnlPct,
			PeakPnLPct:       peakPnlPct,
			LiquidationPrice: liquidationPrice,
			MarginUsed:       marginUsed,
			UpdateTime:       updateTime,
		})
	}

	// 清理已平仓的持仓记录
	for key := range at.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(at.positionFirstSeenTime, key)
		}
	}

	at.reconcileDealsWithPositions(currentPositionKeys)

	// 3. 获取交易员的候选币种池
	candidateCoins, err := at.getCandidateCoins()
	if err != nil {
		return nil, fmt.Errorf("获取候选币种失败: %w", err)
	}

	// 🔥 新增: 智能候选币种过滤（AI预算优化）
	// 如果已经达到最大持仓数量，则不发送候选币种给AI
	// 只发送持仓信息用于管理决策（平仓、调整止损止盈等）
	maxPositions := at.config.MaxPositions
	if maxPositions <= 0 {
		maxPositions = 3 // 默认最大持仓3个
	}

	currentPositionCount := len(positionInfos)
	if currentPositionCount >= maxPositions {
		log.Printf("⚠️  已达到最大持仓数 (%d/%d)，跳过候选币种分析（AI预算优化）", currentPositionCount, maxPositions)
		log.Printf("    仅分析现有持仓的管理决策（平仓/止损止盈调整）")
		// 清空候选币种列表，只分析现有持仓
		candidateCoins = []decision.CandidateCoin{}
	} else {
		log.Printf("✓ 当前持仓 %d/%d，将分析 %d 个候选币种", currentPositionCount, maxPositions, len(candidateCoins))
	}

	// 4. 计算总盈亏
	// 现在可以计算总权益
	totalEquity = availableBalance + totalMarginUsed

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. 分析历史表现（最近100个周期，避免长期持仓的交易记录丢失）
	// 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
	performance, err := at.decisionLogger.AnalyzePerformance(100)
	if err != nil {
		log.Printf("⚠️  分析历史表现失败: %v", err)
		// 不影响主流程，继续执行（但设置performance为nil以避免传递错误数据）
		performance = nil
	}

	// 6. 构建上下文
	ctx := &decision.Context{
		CurrentTime:     time.Now().Format("2006-01-02 15:04:05"),
		RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
		CallCount:       at.callCount,
		BTCETHLeverage:  at.config.BTCETHLeverage,  // 使用配置的杠杆倍数
		AltcoinLeverage: at.config.AltcoinLeverage, // 使用配置的杠杆倍数
		MaxPositions:    maxPositions,              // 传递最大持仓数给Context
		PayloadVersion:  decision.PayloadSchemaVersion,
		Account: decision.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			UnrealizedPnL:    totalUnrealizedProfit,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
		},
		Positions:      positionInfos,
		CandidateCoins: candidateCoins,
		Performance:    performance, // 添加历史表现分析
	}

	return ctx, nil
}

// executeDecisionWithRecord 执行AI决策并记录详细信息
func (at *AutoTrader) executeDecisionWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	switch decision.Action {
	case "open_long":
		return at.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		return at.executeOpenShortWithRecord(decision, actionRecord)
	case "close_long":
		return at.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		return at.executeCloseShortWithRecord(decision, actionRecord)
	case "update_stop_loss":
		return at.executeUpdateStopLossWithRecord(decision, actionRecord)
	case "update_take_profit":
		return at.executeUpdateTakeProfitWithRecord(decision, actionRecord)
	case "update_sl_tp":
		// 组合动作：先更新止损，再更新止盈；两步任一失败则整体失败
		if err := at.executeUpdateStopLossWithRecord(decision, actionRecord); err != nil {
			return err
		}
		return at.executeUpdateTakeProfitWithRecord(decision, actionRecord)
	case "partial_close":
		return at.executePartialCloseWithRecord(decision, actionRecord)
	case "hold", "wait":
		// 无需执行，仅记录
		return nil
	default:
		return fmt.Errorf("未知的action: %s", decision.Action)
	}
}

// executeOpenLongWithRecord 执行开多仓并记录详细信息
func (at *AutoTrader) executeOpenLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📈 开多仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				return fmt.Errorf("❌ %s 已有多仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_long 决策", decision.Symbol)
			}
		}
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice
	dbSymbol := normalizeDealSymbol(decision.Symbol)

	// ⚠️ 保证金验证：防止保证金不足错误（code=-2019）
	requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// 手续费估算（Taker费率 0.04%）
	estimatedFee := decision.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee

	if totalRequired > availableBalance {
		return fmt.Errorf("❌ 保证金不足: 需要 %.2f USDT（保证金 %.2f + 手续费 %.2f），可用 %.2f USDT",
			totalRequired, requiredMargin, estimatedFee, availableBalance)
	}

	// 设置仓位模式
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		// 继续执行，不影响交易
	}

	// 开仓
	order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	filledQty := quantity
	if v, ok := orderFloatValue(order, "filledSize"); ok && v > 0 {
		filledQty = v
	}
	fillPrice := marketData.CurrentPrice
	if v, ok := orderFloatValue(order, "avgPrice"); ok && v > 0 {
		fillPrice = v
	}
	actionRecord.Quantity = filledQty
	actionRecord.Price = fillPrice

	// 记录订单ID
	if orderID, ok := orderInt64Value(order, "orderId"); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f (成交: %.4f @ %.4f)", order["orderId"], quantity, filledQty, fillPrice)

	// 记录开仓时间
	posKey := makePositionKey(decision.Symbol, "long")
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// 设置止损止盈
	if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, decision.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, decision.TakeProfit); err != nil {
		log.Printf("  ⚠ 设置止盈失败: %v", err)
	}

	// 持久化到数据库（deals）
	if at.database != nil {
		if db, ok := at.database.(interface {
			CreateDeal(deal *config.DealRecord) (int64, error)
		}); ok {
			marketCtx := map[string]interface{}{
				"symbol":          marketData.Symbol,
				"current_price":   marketData.CurrentPrice,
				"price_change_1h": marketData.PriceChange1h,
				"price_change_4h": marketData.PriceChange4h,
				"ema20":           marketData.CurrentEMA20,
				"macd":            marketData.CurrentMACD,
				"rsi7":            marketData.CurrentRSI7,
				"funding_rate":    marketData.FundingRate,
			}
			if marketData.OpenInterest != nil {
				marketCtx["oi_latest"] = marketData.OpenInterest.Latest
				marketCtx["oi_average"] = marketData.OpenInterest.Average
			}
			ctxJSON, _ := json.Marshal(marketCtx)
			openOrderID := fmt.Sprintf("%v", order["orderId"])
			deal := &config.DealRecord{
				UserID:            at.userID,
				TraderID:          at.id,
				Exchange:          at.exchange,
				Symbol:            dbSymbol,
				Side:              "long",
				Leverage:          decision.Leverage,
				PositionSizeUSD:   decision.PositionSizeUSD,
				Quantity:          filledQty,
				OpenPrice:         fillPrice,
				OpenTime:          time.Now(),
				OpenOrderID:       openOrderID,
				SystemPrompt:      at.lastSystemPrompt,
				UserPrompt:        at.lastUserPrompt,
				Reasoning:         decision.Reasoning,
				CoTTrace:          at.lastCoTTrace,
				DecisionJSON:      at.lastDecisionJSON,
				MarketContextJSON: string(ctxJSON),
				StopLoss:          decision.StopLoss,
				TakeProfit:        decision.TakeProfit,
			}
			if id, err := db.CreateDeal(deal); err != nil {
				log.Printf("  ⚠️ 记录交易(deal)失败: %v", err)
			} else {
				at.openDealIDs[posKey] = id
				log.Printf("  📝 已保存交易记录到数据库 (deal_id=%d)", id)
				// 记录事件: open
				if dbEvt, ok := at.database.(interface {
					CreateDealEvent(event *config.DealEvent) error
				}); ok {
					_ = dbEvt.CreateDealEvent(&config.DealEvent{
						UserID: at.userID, TraderID: at.id, DealID: id, Type: "open",
						Symbol: dbSymbol, Side: "long", Quantity: filledQty, Price: fillPrice, OrderID: openOrderID,
					})
				}
			}
		}
	}

	return nil
}

// executeOpenShortWithRecord 执行开空仓并记录详细信息
func (at *AutoTrader) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📉 开空仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				return fmt.Errorf("❌ %s 已有空仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_short 决策", decision.Symbol)
			}
		}
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice
	dbSymbol := normalizeDealSymbol(decision.Symbol)

	// ⚠️ 保证金验证：防止保证金不足错误（code=-2019）
	requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// 手续费估算（Taker费率 0.04%）
	estimatedFee := decision.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee

	if totalRequired > availableBalance {
		return fmt.Errorf("❌ 保证金不足: 需要 %.2f USDT（保证金 %.2f + 手续费 %.2f），可用 %.2f USDT",
			totalRequired, requiredMargin, estimatedFee, availableBalance)
	}

	// 设置仓位模式
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		// 继续执行，不影响交易
	}

	// 开仓
	order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	filledQty := quantity
	if v, ok := orderFloatValue(order, "filledSize"); ok && v > 0 {
		filledQty = v
	}
	fillPrice := marketData.CurrentPrice
	if v, ok := orderFloatValue(order, "avgPrice"); ok && v > 0 {
		fillPrice = v
	}
	actionRecord.Quantity = filledQty
	actionRecord.Price = fillPrice

	// 记录订单ID
	if orderID, ok := orderInt64Value(order, "orderId"); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f (成交: %.4f @ %.4f)", order["orderId"], quantity, filledQty, fillPrice)

	// 记录开仓时间
	posKey := makePositionKey(decision.Symbol, "short")
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// 设置止损止盈
	if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, decision.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, decision.TakeProfit); err != nil {
		log.Printf("  ⚠ 设置止盈失败: %v", err)
	}

	// 持久化到数据库（deals）
	if at.database != nil {
		if db, ok := at.database.(interface {
			CreateDeal(deal *config.DealRecord) (int64, error)
		}); ok {
			marketCtx := map[string]interface{}{
				"symbol":          marketData.Symbol,
				"current_price":   marketData.CurrentPrice,
				"price_change_1h": marketData.PriceChange1h,
				"price_change_4h": marketData.PriceChange4h,
				"ema20":           marketData.CurrentEMA20,
				"macd":            marketData.CurrentMACD,
				"rsi7":            marketData.CurrentRSI7,
				"funding_rate":    marketData.FundingRate,
			}
			if marketData.OpenInterest != nil {
				marketCtx["oi_latest"] = marketData.OpenInterest.Latest
				marketCtx["oi_average"] = marketData.OpenInterest.Average
			}
			ctxJSON, _ := json.Marshal(marketCtx)
			openOrderID := fmt.Sprintf("%v", order["orderId"])
			deal := &config.DealRecord{
				UserID:            at.userID,
				TraderID:          at.id,
				Exchange:          at.exchange,
				Symbol:            dbSymbol,
				Side:              "short",
				Leverage:          decision.Leverage,
				PositionSizeUSD:   decision.PositionSizeUSD,
				Quantity:          filledQty,
				OpenPrice:         fillPrice,
				OpenTime:          time.Now(),
				OpenOrderID:       openOrderID,
				SystemPrompt:      at.lastSystemPrompt,
				UserPrompt:        at.lastUserPrompt,
				Reasoning:         decision.Reasoning,
				CoTTrace:          at.lastCoTTrace,
				DecisionJSON:      at.lastDecisionJSON,
				MarketContextJSON: string(ctxJSON),
				StopLoss:          decision.StopLoss,
				TakeProfit:        decision.TakeProfit,
			}
			if id, err := db.CreateDeal(deal); err != nil {
				log.Printf("  ⚠️ 记录交易(deal)失败: %v", err)
			} else {
				posKey := makePositionKey(decision.Symbol, "short")
				at.openDealIDs[posKey] = id
				log.Printf("  📝 已保存交易记录到数据库 (deal_id=%d)", id)
				// 记录事件: open
				if dbEvt, ok := at.database.(interface {
					CreateDealEvent(event *config.DealEvent) error
				}); ok {
					_ = dbEvt.CreateDealEvent(&config.DealEvent{
						UserID: at.userID, TraderID: at.id, DealID: id, Type: "open",
						Symbol: dbSymbol, Side: "short", Quantity: filledQty, Price: fillPrice, OrderID: openOrderID,
					})
				}
			}
		}
	}

	return nil
}

// executeCloseLongWithRecord 执行平多仓并记录详细信息
func (at *AutoTrader) executeCloseLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平多仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 平仓
	order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	fillPrice := marketData.CurrentPrice
	if v, ok := orderFloatValue(order, "avgPrice"); ok && v > 0 {
		fillPrice = v
	}
	actionRecord.Price = fillPrice

	// 记录订单ID
	if orderID, ok := orderInt64Value(order, "orderId"); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")
	at.closeDealInternal(decision.Symbol, "long", fillPrice, fmt.Sprintf("%v", order["orderId"]), false, "ai_close_long")

	// Cleanup Trailing-Stop-State
	posKey := makePositionKey(decision.Symbol, "long")
	at.trailingStopMutex.Lock()
	delete(at.trailingStopManaged, posKey)
	delete(at.trailingStopLastUpdate, posKey)
	at.trailingStopMutex.Unlock()

	return nil
}

func (at *AutoTrader) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平空仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 平仓
	order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	fillPrice := marketData.CurrentPrice
	if v, ok := orderFloatValue(order, "avgPrice"); ok && v > 0 {
		fillPrice = v
	}
	actionRecord.Price = fillPrice

	// 记录订单ID
	if orderID, ok := orderInt64Value(order, "orderId"); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")
	at.closeDealInternal(decision.Symbol, "short", fillPrice, fmt.Sprintf("%v", order["orderId"]), false, "ai_close_short")

	// Cleanup Trailing-Stop-State
	posKey := makePositionKey(decision.Symbol, "short")
	at.trailingStopMutex.Lock()
	delete(at.trailingStopManaged, posKey)
	delete(at.trailingStopLastUpdate, posKey)
	at.trailingStopMutex.Unlock()

	return nil
}

func (at *AutoTrader) executeUpdateStopLossWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 调整止损: %s → %.2f", decision.Symbol, decision.NewStopLoss)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	normalizedSymbol := normalizeDealSymbol(decision.Symbol)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 验证新止损价格合理性
	if positionSide == "LONG" && decision.NewStopLoss >= marketData.CurrentPrice {
		return fmt.Errorf("多单止损必须低于当前价格 (当前: %.2f, 新止损: %.2f)", marketData.CurrentPrice, decision.NewStopLoss)
	}
	if positionSide == "SHORT" && decision.NewStopLoss <= marketData.CurrentPrice {
		return fmt.Errorf("空单止损必须高于当前价格 (当前: %.2f, 新止损: %.2f)", marketData.CurrentPrice, decision.NewStopLoss)
	}

	// ⚠️ 防御性检查：检测是否存在双向持仓（不应该出现，但提供保护）
	var hasOppositePosition bool
	oppositeSide := ""
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posSide, _ := pos["side"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 && strings.ToUpper(posSide) != positionSide {
			hasOppositePosition = true
			oppositeSide = strings.ToUpper(posSide)
			break
		}
	}

	if hasOppositePosition {
		log.Printf("  🚨 警告：检测到 %s 存在双向持仓（%s + %s），这违反了策略规则",
			decision.Symbol, positionSide, oppositeSide)
		log.Printf("  🚨 取消止损单将影响两个方向的订单，请检查是否为用户手动操作导致")
		log.Printf("  🚨 建议：手动平掉其中一个方向的持仓，或检查系统是否有BUG")
	}

	// 取消旧的止损单（只删除止损单，不影响止盈单）
	// 注意：如果存在双向持仓，这会删除两个方向的止损单
	if err := at.trader.CancelStopLossOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止损单失败: %v", err)
		// 不中断执行，继续设置新止损
	}

	// 调用交易所 API 修改止损
	quantity := math.Abs(positionAmt)
	err = at.trader.SetStopLoss(decision.Symbol, positionSide, quantity, decision.NewStopLoss)
	if err != nil {
		return fmt.Errorf("修改止损失败: %w", err)
	}

	log.Printf("  ✓ 止损已调整: %.2f (当前价格: %.2f)", decision.NewStopLoss, marketData.CurrentPrice)

	// 检查是否允许AI覆盖自动追踪止损
	if at.trailingStopConfig.Enabled && !at.trailingStopConfig.AllowAIOverride {
		log.Printf("  🚫 [AI·跳过止损] %s %s 正由自动追踪系统管理（TRAILING_STOP_ALLOW_AI_OVERRIDE=false）", normalizedSymbol, side)
		log.Printf("  🚫 AI决策已忽略，自动追踪将继续按配置工作")
		
		// 回滚：恢复旧的止损单（因为我们已经取消了）
		// 使用追踪系统记录的最后一次止损价格
		posKey := makePositionKey(decision.Symbol, side)
		at.trailingStopMutex.RLock()
		lastStopPrice, hasLastPrice := at.trailingStopLastUpdate[posKey]
		at.trailingStopMutex.RUnlock()
		
		if hasLastPrice && lastStopPrice > 0 {
			// 恢复旧的止损
			log.Printf("  🔄 [恢复止损] 恢复自动追踪止损价格: %.4f", lastStopPrice)
			if err := at.trader.SetStopLoss(decision.Symbol, positionSide, quantity, lastStopPrice); err != nil {
				log.Printf("  ⚠️ [恢复失败] 无法恢复止损单: %v", err)
			}
		}
		
		return fmt.Errorf("AI止损更新已拒绝：自动追踪系统正在管理此持仓")
	}
	
	// 标记为AI管理（如果允许AI覆盖）
	if at.trailingStopConfig.Enabled && at.trailingStopConfig.AllowAIOverride {
		posKey := makePositionKey(decision.Symbol, side)
		at.trailingStopMutex.Lock()
		at.trailingStopManaged[posKey] = true
		at.trailingStopLastUpdate[posKey] = decision.NewStopLoss
		at.trailingStopMutex.Unlock()
		log.Printf("  🤖 [AI接管] %s %s 的止损管理已由AI接管", normalizedSymbol, side)
	}

	// 更新数据库止损
	if at.database != nil {
		// symbol_side -> id
		posKey := makePositionKey(decision.Symbol, positionSide)
		dealID, has := at.openDealIDs[posKey]
		if !has {
			if dbFind, ok := at.database.(interface {
				FindOpenDeal(userID, traderID, symbol, side string) (*config.DealRecord, error)
			}); ok {
				if rec, err := dbFind.FindOpenDeal(at.userID, at.id, normalizedSymbol, strings.ToLower(positionSide)); err == nil && rec != nil {
					dealID = rec.ID
				}
			}
		}
		if dealID != 0 {
			if dbUpd, ok := at.database.(interface {
				UpdateDealStopLoss(userID, traderID string, id int64, stopLoss float64) error
			}); ok {
				_ = dbUpd.UpdateDealStopLoss(at.userID, at.id, dealID, decision.NewStopLoss)
			}
			if dbEvt, ok := at.database.(interface {
				CreateDealEvent(event *config.DealEvent) error
			}); ok {
				_ = dbEvt.CreateDealEvent(&config.DealEvent{UserID: at.userID, TraderID: at.id, DealID: dealID, Type: "update_stop_loss", Symbol: normalizedSymbol, Side: strings.ToLower(positionSide), Price: decision.NewStopLoss})
			}
		}
	}
	return nil
}

// executeUpdateTakeProfitWithRecord 执行调整止盈并记录详细信息
func (at *AutoTrader) executeUpdateTakeProfitWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 调整止盈: %s → %.2f", decision.Symbol, decision.NewTakeProfit)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	normalizedSymbol := normalizeDealSymbol(decision.Symbol)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 验证新止盈价格合理性
	if positionSide == "LONG" && decision.NewTakeProfit <= marketData.CurrentPrice {
		return fmt.Errorf("多单止盈必须高于当前价格 (当前: %.2f, 新止盈: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}
	if positionSide == "SHORT" && decision.NewTakeProfit >= marketData.CurrentPrice {
		return fmt.Errorf("空单止盈必须低于当前价格 (当前: %.2f, 新止盈: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}

	// ⚠️ 防御性检查：检测是否存在双向持仓（不应该出现，但提供保护）
	var hasOppositePosition bool
	oppositeSide := ""
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posSide, _ := pos["side"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 && strings.ToUpper(posSide) != positionSide {
			hasOppositePosition = true
			oppositeSide = strings.ToUpper(posSide)
			break
		}
	}

	if hasOppositePosition {
		log.Printf("  🚨 警告：检测到 %s 存在双向持仓（%s + %s），这违反了策略规则",
			decision.Symbol, positionSide, oppositeSide)
		log.Printf("  🚨 取消止盈单将影响两个方向的订单，请检查是否为用户手动操作导致")
		log.Printf("  🚨 建议：手动平掉其中一个方向的持仓，或检查系统是否有BUG")
	}

	// 取消旧的止盈单（只删除止盈单，不影响止损单）
	// 注意：如果存在双向持仓，这会删除两个方向的止盈单
	if err := at.trader.CancelTakeProfitOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止盈单失败: %v", err)
		// 不中断执行，继续设置新止盈
	}

	// 调用交易所 API 修改止盈
	quantity := math.Abs(positionAmt)
	err = at.trader.SetTakeProfit(decision.Symbol, positionSide, quantity, decision.NewTakeProfit)
	if err != nil {
		return fmt.Errorf("修改止盈失败: %w", err)
	}

	log.Printf("  ✓ 止盈已调整: %.2f (当前价格: %.2f)", decision.NewTakeProfit, marketData.CurrentPrice)
	// 更新数据库止盈 + 记录事件
	if at.database != nil {
		posKey := makePositionKey(decision.Symbol, positionSide)
		dealID, has := at.openDealIDs[posKey]
		if !has {
			if dbFind, ok := at.database.(interface {
				FindOpenDeal(userID, traderID, symbol, side string) (*config.DealRecord, error)
			}); ok {
				if rec, err := dbFind.FindOpenDeal(at.userID, at.id, normalizedSymbol, strings.ToLower(positionSide)); err == nil && rec != nil {
					dealID = rec.ID
				}
			}
		}
		if dealID != 0 {
			if dbUpd, ok := at.database.(interface {
				UpdateDealTakeProfit(userID, traderID string, id int64, takeProfit float64) error
			}); ok {
				_ = dbUpd.UpdateDealTakeProfit(at.userID, at.id, dealID, decision.NewTakeProfit)
			}
			if dbEvt, ok := at.database.(interface {
				CreateDealEvent(event *config.DealEvent) error
			}); ok {
				_ = dbEvt.CreateDealEvent(&config.DealEvent{UserID: at.userID, TraderID: at.id, DealID: dealID, Type: "update_take_profit", Symbol: normalizedSymbol, Side: strings.ToLower(positionSide), Price: decision.NewTakeProfit})
			}
		}
	}
	return nil
}

// executePartialCloseWithRecord 执行部分平仓并记录详细信息
func (at *AutoTrader) executePartialCloseWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📊 部分平仓: %s %.1f%%", decision.Symbol, decision.ClosePercentage)

	// 验证百分比范围
	if decision.ClosePercentage <= 0 || decision.ClosePercentage > 100 {
		return fmt.Errorf("平仓百分比必须在 0-100 之间，当前: %.1f", decision.ClosePercentage)
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	normalizedSymbol := normalizeDealSymbol(decision.Symbol)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 计算平仓数量
	totalQuantity := math.Abs(positionAmt)
	closeQuantity := totalQuantity * (decision.ClosePercentage / 100.0)
	actionRecord.Quantity = closeQuantity

	// ✅ Layer 2: 最小仓位检查（防止产生小额剩余）
	markPrice, ok := targetPosition["markPrice"].(float64)
	if !ok || markPrice <= 0 {
		return fmt.Errorf("无法解析当前价格，无法执行最小仓位检查")
	}

	currentPositionValue := totalQuantity * markPrice
	remainingQuantity := totalQuantity - closeQuantity
	remainingValue := remainingQuantity * markPrice

	const MIN_POSITION_VALUE = 10.0 // 最小持仓价值 10 USDT（對齊交易所底线，小仓位建议直接全平）

	if remainingValue > 0 && remainingValue <= MIN_POSITION_VALUE {
		log.Printf("⚠️ 检测到 partial_close 后剩余仓位 %.2f USDT < %.0f USDT",
			remainingValue, MIN_POSITION_VALUE)
		log.Printf("  → 当前仓位价值: %.2f USDT, 平仓 %.1f%%, 剩余: %.2f USDT",
			currentPositionValue, decision.ClosePercentage, remainingValue)
		log.Printf("  → 自动修正为全部平仓，避免产生无法平仓的小额剩余")

		// 🔄 自动修正为全部平仓
		if positionSide == "LONG" {
			decision.Action = "close_long"
			log.Printf("  ✓ 已修正为: close_long")
			return at.executeCloseLongWithRecord(decision, actionRecord)
		} else {
			decision.Action = "close_short"
			log.Printf("  ✓ 已修正为: close_short")
			return at.executeCloseShortWithRecord(decision, actionRecord)
		}
	}

	// 执行平仓
	var order map[string]interface{}
	if positionSide == "LONG" {
		order, err = at.trader.CloseLong(decision.Symbol, closeQuantity)
	} else {
		order, err = at.trader.CloseShort(decision.Symbol, closeQuantity)
	}

	if err != nil {
		return fmt.Errorf("部分平仓失败: %w", err)
	}

	fillPrice := marketData.CurrentPrice
	if v, ok := orderFloatValue(order, "avgPrice"); ok && v > 0 {
		fillPrice = v
	}
	filledQuantity := closeQuantity
	if v, ok := orderFloatValue(order, "filledSize"); ok && v > 0 {
		filledQuantity = v
	}
	actionRecord.Price = fillPrice
	actionRecord.Quantity = filledQuantity

	// 记录订单ID
	if orderID, ok := orderInt64Value(order, "orderId"); ok {
		actionRecord.OrderID = orderID
	}

	remainingQuantity = totalQuantity - filledQuantity
	log.Printf("  ✓ 部分平仓成功: 平仓 %.4f (%.1f%%), 剩余 %.4f",
		filledQuantity, decision.ClosePercentage, remainingQuantity)

	// 记录部分平仓事件
	if at.database != nil {
		posKey := makePositionKey(decision.Symbol, positionSide)
		dealID, has := at.openDealIDs[posKey]
		if !has {
			if dbFind, ok := at.database.(interface {
				FindOpenDeal(userID, traderID, symbol, side string) (*config.DealRecord, error)
			}); ok {
				if rec, err := dbFind.FindOpenDeal(at.userID, at.id, normalizedSymbol, strings.ToLower(positionSide)); err == nil && rec != nil {
					dealID = rec.ID
				}
			}
		}
		if dealID != 0 {
			if dbEvt, ok := at.database.(interface {
				CreateDealEvent(event *config.DealEvent) error
			}); ok {
				_ = dbEvt.CreateDealEvent(&config.DealEvent{UserID: at.userID, TraderID: at.id, DealID: dealID, Type: "partial_close", Symbol: normalizedSymbol, Side: strings.ToLower(positionSide), Quantity: filledQuantity, Percentage: decision.ClosePercentage, Price: fillPrice, OrderID: fmt.Sprintf("%v", order["orderId"])})
			}
		}
	}

	// ✅ Step 4: 恢复止盈止损（防止剩余仓位裸奔）
	// 重要：币安等交易所在部分平仓后会自动取消原有的 TP/SL 订单（因为数量不匹配）
	// 如果 AI 提供了新的止损止盈价格，则为剩余仓位重新设置保护
	if decision.NewStopLoss > 0 {
		log.Printf("  → 为剩余仓位 %.4f 恢复止损单: %.2f", remainingQuantity, decision.NewStopLoss)
		err = at.trader.SetStopLoss(decision.Symbol, positionSide, remainingQuantity, decision.NewStopLoss)
		if err != nil {
			log.Printf("  ⚠️ 恢复止损失败: %v（不影响平仓结果）", err)
		}
	}

	if decision.NewTakeProfit > 0 {
		log.Printf("  → 为剩余仓位 %.4f 恢复止盈单: %.2f", remainingQuantity, decision.NewTakeProfit)
		err = at.trader.SetTakeProfit(decision.Symbol, positionSide, remainingQuantity, decision.NewTakeProfit)
		if err != nil {
			log.Printf("  ⚠️ 恢复止盈失败: %v（不影响平仓结果）", err)
		}
	}

	// 如果 AI 没有提供新的止盈止损，记录警告
	if decision.NewStopLoss <= 0 && decision.NewTakeProfit <= 0 {
		log.Printf("  ⚠️⚠️⚠️ 警告: 部分平仓后AI未提供新的止盈止损价格")
		log.Printf("  → 剩余仓位 %.4f (价值 %.2f USDT) 目前没有止盈止损保护", remainingQuantity, remainingValue)
		log.Printf("  → 建议: 在 partial_close 决策中包含 new_stop_loss 和 new_take_profit 字段")
	}

	return nil
}

func (at *AutoTrader) closeDealInternal(symbol, side string, closePrice float64, orderID string, wasStopLoss bool, reason string) {
	symbol = normalizeDealSymbol(symbol)
	dealID, openRec := at.getOpenDealRecord(symbol, side)
	at.closeDealInternalWithRecord(symbol, side, dealID, openRec, closePrice, orderID, wasStopLoss, reason)
}

func (at *AutoTrader) closeDealInternalWithRecord(symbol, side string, dealID int64, openRec *config.DealRecord, closePrice float64, orderID string, wasStopLoss bool, reason string) {
	symbol = normalizeDealSymbol(symbol)
	if dealID == 0 || openRec == nil {
		log.Printf("⚠️ [%s] 未找到可关闭的交易记录 (%s %s)", at.name, symbol, side)
		return
	}
	if closePrice <= 0 {
		closePrice = openRec.OpenPrice
	}
	realizedPnL, realizedPnLPct := at.computeDealPnL(openRec, closePrice)
	duration := time.Since(openRec.OpenTime).Seconds()
	if closer, ok := at.database.(interface {
		CloseDeal(userID, traderID string, id int64, closePrice float64, closeOrderID string, realizedPnL, realizedPnLPct float64, durationSeconds int64, wasStopLoss bool, closeReason string) error
	}); ok {
		if err := closer.CloseDeal(at.userID, at.id, dealID, closePrice, orderID, realizedPnL, realizedPnLPct, int64(duration), wasStopLoss, reason); err != nil {
			log.Printf("⚠️ [%s] 更新交易P/L失败 (%s %s): %v", at.name, symbol, side, err)
			return
		}
		posKey := makePositionKey(symbol, side)
		delete(at.openDealIDs, posKey)
		log.Printf("💾 [%s] 已更新交易P/L (deal_id=%d, reason=%s)", at.name, dealID, reason)
		if evt, ok := at.database.(interface {
			CreateDealEvent(event *config.DealEvent) error
		}); ok {
			_ = evt.CreateDealEvent(&config.DealEvent{
				UserID: at.userID, TraderID: at.id, DealID: dealID, Type: "close",
				Symbol: symbol, Side: strings.ToLower(side), Price: closePrice, OrderID: orderID,
			})
		}
	}
}

func orderFloatValue(order map[string]interface{}, key string) (float64, bool) {
	if order == nil {
		return 0, false
	}
	value, ok := order[key]
	if !ok || value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func orderInt64Value(order map[string]interface{}, key string) (int64, bool) {
	if order == nil {
		return 0, false
	}
	value, ok := order[key]
	if !ok || value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i, true
		}
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}

func normalizeDealSymbol(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	switch {
	case strings.HasSuffix(s, "USDC"):
		return s[:len(s)-4] + "USDT"
	case strings.HasSuffix(s, "USD"):
		// Treat bare USD as USDT for consistency
		return s + "T"
	default:
		return s
	}
}

func makePositionKey(symbol, side string) string {
	return normalizeDealSymbol(symbol) + "_" + strings.ToLower(strings.TrimSpace(side))
}

func (at *AutoTrader) getOpenDealRecord(symbol, side string) (int64, *config.DealRecord) {
	symbol = normalizeDealSymbol(symbol)
	side = strings.ToLower(side)
	posKey := makePositionKey(symbol, side)
	if dealID, ok := at.openDealIDs[posKey]; ok {
		if rec := at.getDealByID(dealID); rec != nil {
			return dealID, rec
		}
	}
	if finder, ok := at.database.(interface {
		FindOpenDeal(userID, traderID, symbol, side string) (*config.DealRecord, error)
	}); ok {
		if rec, err := finder.FindOpenDeal(at.userID, at.id, symbol, side); err == nil && rec != nil {
			at.openDealIDs[posKey] = rec.ID
			return rec.ID, rec
		}
	}
	return 0, nil
}

func (at *AutoTrader) getDealByID(id int64) *config.DealRecord {
	if getter, ok := at.database.(interface {
		GetDealByID(userID, traderID string, id int64) (*config.DealRecord, error)
	}); ok {
		if rec, err := getter.GetDealByID(at.userID, at.id, id); err == nil {
			return rec
		}
	}
	return nil
}

func (at *AutoTrader) computeDealPnL(openRec *config.DealRecord, closePrice float64) (float64, float64) {
	if openRec == nil {
		return 0, 0
	}
	side := strings.ToLower(openRec.Side)
	open := openRec.OpenPrice
	slippageRate := getEnvFloat("NOFX_PNL_SLIPPAGE_RATE", 0.0005)
	feeRate := getEnvFloat("NOFX_PNL_TAKER_FEE_RATE", 0.0004)
	effOpen := open
	effClose := closePrice
	if side == "long" {
		effOpen = open * (1 + slippageRate)
		effClose = closePrice * (1 - slippageRate)
	} else {
		effOpen = open * (1 - slippageRate)
		effClose = closePrice * (1 + slippageRate)
	}
	priceChangePct := 0.0
	if effOpen > 0 {
		if side == "long" {
			priceChangePct = (effClose - effOpen) / effOpen
		} else {
			priceChangePct = (effOpen - effClose) / effOpen
		}
	}
	positionValue := openRec.Quantity * effOpen
	// Futures PnL = position notional * price change percentage (already leverage-adjusted in margin),
	// leverage should not be applied again, otherwise the profit/loss is overstated.
	grossPnL := positionValue * priceChangePct
	fees := (openRec.Quantity*effOpen + openRec.Quantity*effClose) * feeRate
	realizedPnL := grossPnL - fees
	marginUsed := 0.0
	if openRec.Leverage > 0 {
		marginUsed = positionValue / float64(openRec.Leverage)
	}
	realizedPnLPct := 0.0
	if marginUsed > 0 {
		realizedPnLPct = (realizedPnL / marginUsed) * 100
	}
	return realizedPnL, realizedPnLPct
}

func (at *AutoTrader) reconcileDealsWithPositions(current map[string]bool) {
	for key := range at.openDealIDs {
		if current[key] {
			continue
		}
		symbol, side, ok := splitSymbolSide(key)
		if !ok {
			continue
		}
		dealID, openRec := at.getOpenDealRecord(symbol, side)
		if openRec == nil {
			continue
		}
		if time.Since(openRec.OpenTime) < manualDealCloseGrace {
			continue
		}
		price := at.getLatestPrice(symbol)
		if price <= 0 {
			log.Printf("⚠️ [%s] 无法获取 %s 的最新价格以强制关闭交易", at.name, symbol)
			continue
		}

		// 确定关闭原因：检查是否是追踪止损触发
		closeReason := "position_missing"
		at.trailingStopMutex.RLock()
		tierIndex, hasTier := at.trailingStopActiveTier[key]
		at.trailingStopMutex.RUnlock()

		if hasTier && tierIndex >= 0 && tierIndex < len(at.trailingStopConfig.Tiers) {
			// 追踪止损触发
			closeReason = fmt.Sprintf("trailing_stop_tier%d", tierIndex+1)
		}

		at.closeDealInternalWithRecord(symbol, side, dealID, openRec, price, "manual_close", false, closeReason)

		// 清理追踪止损状态
		at.trailingStopMutex.Lock()
		delete(at.trailingStopActiveTier, key)
		delete(at.trailingStopLastUpdate, key)
		delete(at.trailingStopManaged, key)
		at.trailingStopMutex.Unlock()
	}
}

func splitSymbolSide(key string) (string, string, bool) {
	idx := strings.LastIndex(key, "_")
	if idx <= 0 || idx >= len(key)-1 {
		return "", "", false
	}
	return key[:idx], key[idx+1:], true
}

func (at *AutoTrader) getLatestPrice(symbol string) float64 {
	data, err := market.Get(symbol)
	if err != nil || data == nil {
		return 0
	}
	return data.CurrentPrice
}

// GetID 获取trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetName 获取trader名称
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetAIModel 获取AI模型
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetExchange 获取交易所
func (at *AutoTrader) GetExchange() string {
	return at.exchange
}

// SetCustomPrompt 设置自定义交易策略prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
	at.customPrompt = prompt
}

// SetOverrideBasePrompt 设置是否覆盖基础prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
	at.overrideBasePrompt = override
}

// SetSystemPromptTemplate 设置系统提示词模板
func (at *AutoTrader) SetSystemPromptTemplate(templateName string) {
	at.systemPromptTemplate = templateName
}

// GetSystemPromptTemplate 获取当前系统提示词模板名称
func (at *AutoTrader) GetSystemPromptTemplate() string {
	return at.systemPromptTemplate
}

// GetDecisionLogger 获取决策日志记录器
func (at *AutoTrader) GetDecisionLogger() logger.IDecisionLogger {
	return at.decisionLogger
}

// UpdateAIModelConfig 动态更新AI提供商与密钥（热更新，不需重启trader）
func (at *AutoTrader) UpdateAIModelConfig(provider, apiKey, customAPIURL, customModelName string) {
	// 更新内部配置与状态
	at.aiModel = provider
	at.config.CustomAPIURL = customAPIURL
	at.config.CustomModelName = customModelName

	switch strings.ToLower(provider) {
	case "qwen":
		at.config.UseQwen = true
		at.config.QwenKey = apiKey
		at.config.DeepSeekKey = ""
		// 应用到 MCP 客户端
		at.mcpClient.SetAPIKey(apiKey, customAPIURL, customModelName)
		log.Printf("🔄 [%s] 已更新AI配置为 Qwen (自定义URL=%s, 模型=%s)", at.name, customAPIURL, customModelName)
	case "deepseek":
		at.config.UseQwen = false
		at.config.DeepSeekKey = apiKey
		at.config.QwenKey = ""
		at.mcpClient.SetAPIKey(apiKey, customAPIURL, customModelName)
		log.Printf("🔄 [%s] 已更新AI配置为 DeepSeek (自定义URL=%s, 模型=%s)", at.name, customAPIURL, customModelName)
	case "custom":
		at.config.UseQwen = false
		at.config.CustomAPIKey = apiKey
		at.config.DeepSeekKey = ""
		at.config.QwenKey = ""
		at.mcpClient.SetAPIKey(apiKey, customAPIURL, customModelName)
		log.Printf("🔄 [%s] 已更新AI配置为 自定义API (URL=%s, 模型=%s)", at.name, customAPIURL, customModelName)
	default:
		// 未知提供商，默认按 DeepSeek 处理以保持兼容
		at.config.UseQwen = false
		at.config.DeepSeekKey = apiKey
		at.config.QwenKey = ""
		at.mcpClient.SetAPIKey(apiKey, customAPIURL, customModelName)
		log.Printf("⚠️  [%s] 未知AI提供商 '%s'，按 DeepSeek 处理 (URL=%s, 模型=%s)", at.name, provider, customAPIURL, customModelName)
	}
}

// GetBalance 获取账户余额（用于API Balance Check）
func (at *AutoTrader) GetBalance() (map[string]interface{}, error) {
	return at.trader.GetBalance()
}

// GetStatus 获取系统状态（用于API）
func (at *AutoTrader) GetStatus() map[string]interface{} {
	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	// Trailing-Stop-Status sammeln
	tiersInfo := make([]map[string]interface{}, len(at.trailingStopConfig.Tiers))
	for i, tier := range at.trailingStopConfig.Tiers {
		tiersInfo[i] = map[string]interface{}{
			"profit_threshold": tier.ProfitThreshold,
			"stop_offset":      tier.StopOffset,
		}
	}

	trailingStopStatus := map[string]interface{}{
		"enabled":              at.trailingStopConfig.Enabled,
		"tiers":                tiersInfo,
		"update_threshold_pct": at.trailingStopConfig.UpdateThresholdPct,
		"check_interval_sec":   at.trailingStopConfig.CheckIntervalSec,
		"allow_ai_override":    at.trailingStopConfig.AllowAIOverride,
	}

	// Aktive Trailing-Stops (AI-managed vs Auto-managed)
	at.trailingStopMutex.RLock()
	aiManagedPositions := make([]string, 0)
	autoManagedPositions := make([]string, 0)
	for posKey, isAIManaged := range at.trailingStopManaged {
		if isAIManaged {
			aiManagedPositions = append(aiManagedPositions, posKey)
		} else {
			autoManagedPositions = append(autoManagedPositions, posKey)
		}
	}
	at.trailingStopMutex.RUnlock()

	trailingStopStatus["ai_managed_positions"] = aiManagedPositions
	trailingStopStatus["auto_managed_positions"] = autoManagedPositions

	return map[string]interface{}{
		"trader_id":       at.id,
		"trader_name":     at.name,
		"ai_model":        at.aiModel,
		"exchange":        at.exchange,
		"is_running":      at.isRunning,
		"start_time":      at.startTime.Format(time.RFC3339),
		"runtime_minutes": int(time.Since(at.startTime).Minutes()),
		"call_count":      at.callCount,
		"initial_balance": at.initialBalance,
		"scan_interval":   at.config.ScanInterval.String(),
		"stop_until":      at.stopUntil.Format(time.RFC3339),
		"last_reset_time": at.lastResetTime.Format(time.RFC3339),
		"ai_provider":     aiProvider,
		"trailing_stop":   trailingStopStatus,
	}
}

// GetAccountInfo 获取账户信息（用于API）
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取余额失败: %w", err)
	}

	// 获取账户字段
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity 账户净值：在合约账户口径下使用 可用余额 + 已占用保证金
	// 注意：这里先占位，待计算出 totalMarginUsed 后再赋值
	totalEquity := 0.0

	// 获取持仓计算总保证金
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnLCalculated := 0.0
	for _, pos := range positions {
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		totalUnrealizedPnLCalculated += unrealizedPnl

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	// 计算总权益（available + marginUsed）
	totalEquity = availableBalance + totalMarginUsed

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	} else {
		log.Printf("⚠️ Initial Balance异常: %.2f，无法计算PNL百分比", at.initialBalance)
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	return map[string]interface{}{
		// 核心字段
		"total_equity":      totalEquity,           // 账户净值 = available + margin_used（合约账户）
		"wallet_balance":    totalWalletBalance,    // 钱包余额（不含未实现盈亏）
		"unrealized_profit": totalUnrealizedProfit, // 未实现盈亏（交易所API官方值）
		"available_balance": availableBalance,      // 可用余额

		// 盈亏统计
		"total_pnl":       totalPnL,          // 总盈亏 = equity - initial
		"total_pnl_pct":   totalPnLPct,       // 总盈亏百分比
		"initial_balance": at.initialBalance, // 初始余额
		"daily_pnl":       at.dailyPnL,       // 日盈亏

		// 持仓信息
		"position_count":  len(positions),  // 持仓数量
		"margin_used":     totalMarginUsed, // 保证金占用
		"margin_used_pct": marginUsedPct,   // 保证金使用率
	}, nil
}

// GetPositions 获取持仓列表（用于API）
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		// 计算占用保证金
		marginUsed := (quantity * markPrice) / float64(leverage)

		// 计算盈亏百分比（基于保证金）
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		result = append(result, map[string]interface{}{
			"symbol":             symbol,
			"side":               side,
			"entry_price":        entryPrice,
			"mark_price":         markPrice,
			"quantity":           quantity,
			"leverage":           leverage,
			"unrealized_pnl":     unrealizedPnl,
			"unrealized_pnl_pct": pnlPct,
			"liquidation_price":  liquidationPrice,
			"margin_used":        marginUsed,
		})
	}

	return result, nil
}

// calculatePnLPercentage 计算盈亏百分比（基于保证金，自动考虑杠杆）
// 收益率 = 未实现盈亏 / 保证金 × 100%
func calculatePnLPercentage(unrealizedPnl, marginUsed float64) float64 {
	if marginUsed > 0 {
		return (unrealizedPnl / marginUsed) * 100
	}
	return 0.0
}

// sortDecisionsByPriority 对决策排序：先平仓，再开仓，最后hold/wait
// 这样可以避免换仓时仓位叠加超限
func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// 定义优先级
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short", "partial_close":
			return 1 // 最高优先级：先平仓（包括部分平仓）
		case "update_stop_loss", "update_take_profit", "update_sl_tp":
			return 2 // 调整持仓止盈止损
		case "open_long", "open_short":
			return 3 // 次优先级：后开仓
		case "hold", "wait":
			return 4 // 最低优先级：观望
		default:
			return 999 // 未知动作放最后
		}
	}

	// 复制决策列表
	sorted := make([]decision.Decision, len(decisions))
	copy(sorted, decisions)

	// 按优先级排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// getCandidateCoins 获取交易员的候选币种列表
func (at *AutoTrader) getCandidateCoins() ([]decision.CandidateCoin, error) {
	if len(at.tradingCoins) == 0 {
		// 使用数据库配置的默认币种列表
		var candidateCoins []decision.CandidateCoin

		if len(at.defaultCoins) > 0 {
			// 使用数据库中配置的默认币种
			for _, coin := range at.defaultCoins {
				symbol := normalizeSymbol(coin)
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"default"}, // 标记为数据库默认币种
				})
			}
			log.Printf("📋 [%s] 使用数据库默认币种: %d个币种 %v",
				at.name, len(candidateCoins), at.defaultCoins)
			return candidateCoins, nil
		} else {
			// 如果数据库中没有配置默认币种，则使用AI500+OI Top作为fallback
			const ai500Limit = 20 // AI500取前20个评分最高的币种

			mergedPool, err := pool.GetMergedCoinPool(ai500Limit)
			if err != nil {
				return nil, fmt.Errorf("获取合并币种池失败: %w", err)
			}

			// 构建候选币种列表（包含来源信息）
			for _, symbol := range mergedPool.AllSymbols {
				sources := mergedPool.SymbolSources[symbol]
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: sources, // "ai500" 和/或 "oi_top"
				})
			}

			log.Printf("📋 [%s] 数据库无默认币种配置，使用AI500+OI Top: AI500前%d + OI_Top20 = 总计%d个候选币种",
				at.name, ai500Limit, len(candidateCoins))
			return candidateCoins, nil
		}
	} else {
		// 使用自定义币种列表
		var candidateCoins []decision.CandidateCoin
		for _, coin := range at.tradingCoins {
			// 确保币种格式正确（转为大写USDT交易对）
			symbol := normalizeSymbol(coin)
			candidateCoins = append(candidateCoins, decision.CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"custom"}, // 标记为自定义来源
			})
		}

		log.Printf("📋 [%s] 使用自定义币种: %d个币种 %v",
			at.name, len(candidateCoins), at.tradingCoins)
		return candidateCoins, nil
	}
}

// normalizeSymbol 标准化币种符号（确保以USDT结尾）
func normalizeSymbol(symbol string) string {
	// 统一归一化：USDC → USDT，其余无后缀追加USDT
	return market.Normalize(symbol)
}

// 启动回撤监控
func (at *AutoTrader) startDrawdownMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(1 * time.Minute) // 每分钟检查一次
		defer ticker.Stop()

		log.Println("📊 启动持仓回撤监控（每分钟检查一次）")

		for {
			select {
			case <-ticker.C:
				at.checkPositionDrawdown()
			case <-at.stopMonitorCh:
				log.Println("⏹ 停止持仓回撤监控")
				return
			}
		}
	}()
}

// 检查持仓回撤情况
func (at *AutoTrader) checkPositionDrawdown() {
	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("❌ 回撤监控：获取持仓失败: %v", err)
		return
	}

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		logSymbol := normalizeDealSymbol(symbol)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // 空仓数量为负，转为正数
		}

		// 计算当前盈亏百分比
		leverage := 10 // 默认值
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// 构造持仓唯一标识（区分多空）
		posKey := makePositionKey(symbol, side)

		// 获取该持仓的历史最高收益
		at.peakPnLCacheMutex.RLock()
		peakPnLPct, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		if !exists {
			// 如果没有历史最高记录，使用当前盈亏作为初始值
			peakPnLPct = currentPnLPct
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		} else {
			// 更新峰值缓存
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		}

		// 计算回撤（从最高点下跌的幅度）
		var drawdownPct float64
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}

		// 检查平仓条件：收益大于5%且回撤超过40%
		if currentPnLPct > 5.0 && drawdownPct >= 40.0 {
			log.Printf("🚨 触发回撤平仓条件: %s %s | 当前收益: %.2f%% | 最高收益: %.2f%% | 回撤: %.2f%%",
				logSymbol, side, currentPnLPct, peakPnLPct, drawdownPct)

			// 执行平仓
			if err := at.emergencyClosePosition(symbol, side); err != nil {
				log.Printf("❌ 回撤平仓失败 (%s %s): %v", symbol, side, err)
			} else {
				log.Printf("✅ 回撤平仓成功: %s %s", logSymbol, side)
				// 平仓后清理该持仓的缓存
				at.ClearPeakPnLCache(symbol, side)
			}
		} else if currentPnLPct > 5.0 {
			// 记录接近平仓条件的情况（用于调试）
			log.Printf("📊 回撤监控: %s %s | 收益: %.2f%% | 最高: %.2f%% | 回撤: %.2f%%",
				logSymbol, side, currentPnLPct, peakPnLPct, drawdownPct)
		}
	}
}

// 紧急平仓函数
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := at.trader.CloseLong(symbol, 0) // 0 = 全部平仓
		if err != nil {
			return err
		}
		log.Printf("✅ 紧急平多仓成功，订单ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, 0) // 0 = 全部平仓
		if err != nil {
			return err
		}
		log.Printf("✅ 紧急平空仓成功，订单ID: %v", order["orderId"])
	default:
		return fmt.Errorf("未知的持仓方向: %s", side)
	}

	return nil
}

// GetPeakPnLCache 获取最高收益缓存
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// 返回缓存的副本
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL 更新最高收益缓存
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := makePositionKey(symbol, side)
	if peak, exists := at.peakPnLCache[posKey]; exists {
		// 更新峰值（如果是多头，取较大值；如果是空头，currentPnLPct为负，也要比较）
		if currentPnLPct > peak {
			at.peakPnLCache[posKey] = currentPnLPct
		}
	} else {
		// 首次记录
		at.peakPnLCache[posKey] = currentPnLPct
	}
}

// ClearPeakPnLCache 清除指定持仓的峰值缓存
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := makePositionKey(symbol, side)
	delete(at.peakPnLCache, posKey)
}

// startTrailingStopMonitor 启动追踪止损监控
func (at *AutoTrader) startTrailingStopMonitor() {
	at.monitorWg.Add(1)
	at.trailingStopMonitorCh = make(chan struct{})

	go func() {
		defer at.monitorWg.Done()

		interval := time.Duration(at.trailingStopConfig.CheckIntervalSec) * time.Second
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Printf("🎯 启动自动追踪止损监控（每%d秒检查一次）", at.trailingStopConfig.CheckIntervalSec)

		for {
			select {
			case <-ticker.C:
				at.updateTrailingStops()
			case <-at.trailingStopMonitorCh:
				log.Println("⏹ 停止追踪止损监控")
				return
			case <-at.stopMonitorCh:
				log.Println("⏹ 停止追踪止损监控（主循环停止）")
				return
			}
		}
	}()
}

// updateTrailingStops 更新所有符合条件的持仓的追踪止损
func (at *AutoTrader) updateTrailingStops() {
	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("⚠️ 追踪止损：获取持仓失败: %v", err)
		return
	}

	if len(positions) == 0 {
		return // 无持仓，不需要处理
	}

	for _, pos := range positions {
		symbol, ok := pos["symbol"].(string)
		if !ok {
			continue
		}

		side, ok := pos["side"].(string)
		if !ok {
			continue
		}

		entryPrice, ok := pos["entryPrice"].(float64)
		if !ok || entryPrice <= 0 {
			continue
		}

		markPrice, ok := pos["markPrice"].(float64)
		if !ok || markPrice <= 0 {
			continue
		}

		quantity, ok := pos["positionAmt"].(float64)
		if !ok {
			continue
		}
		quantity = math.Abs(quantity)

		leverage := 10.0
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = lev
		}

		posKey := makePositionKey(symbol, side)

		// 检查AI是否管理此止损
		at.trailingStopMutex.RLock()
		aiManaged := at.trailingStopManaged[posKey]
		at.trailingStopMutex.RUnlock()

		if aiManaged {
			// AI已接管，跳过自动追踪 - Log it
			shouldUpdateFalse := false
			at.logTrailingStopAction(symbol, side, "skip", markPrice, entryPrice, 0, -1, nil, nil, nil, nil, nil, nil, &shouldUpdateFalse, nil, nil, "AI已接管止损管理")
			continue
		}

		// 计算当前盈亏百分比（考虑杠杆）
		var profitPct float64
		if side == "long" {
			profitPct = ((markPrice - entryPrice) / entryPrice) * leverage * 100
		} else {
			profitPct = ((entryPrice - markPrice) / entryPrice) * leverage * 100
		}

		// 查找基于当前利润的档位
		var currentTierIndex int = -1
		for i := len(at.trailingStopConfig.Tiers) - 1; i >= 0; i-- {
			tier := &at.trailingStopConfig.Tiers[i]
			if profitPct >= tier.ProfitThreshold {
				currentTierIndex = i
				break
			}
		}

		// 获取历史最高档位（Ratchet-Only-Up原则：只升不降）
		at.trailingStopMutex.RLock()
		previousTierIndex, hasPreviousTier := at.trailingStopActiveTier[posKey]
		at.trailingStopMutex.RUnlock()

		// 选择较高档位（确保不会降级）
		var activeTierIndex int
		if hasPreviousTier && previousTierIndex > currentTierIndex {
			// 已经达到过更高档位，保持不降级
			activeTierIndex = previousTierIndex
		} else {
			// 使用当前档位（可能是首次激活或升级）
			activeTierIndex = currentTierIndex
		}

		// 如果没有达到任何档位的触发阈值，跳过
		if activeTierIndex < 0 {
			skipReason := fmt.Sprintf("利润%.2f%%未达到任何档位阈值", profitPct)
			shouldUpdateFalse := false
			at.logTrailingStopAction(symbol, side, "skip", markPrice, entryPrice, profitPct, -1, nil, nil, nil, nil, nil, nil, &shouldUpdateFalse, nil, nil, skipReason)
			continue
		}

		// 获取激活的档位配置
		activeTier := &at.trailingStopConfig.Tiers[activeTierIndex]

		// 更新存储的档位（可能升级，但不会降级）
		at.trailingStopMutex.Lock()
		at.trailingStopActiveTier[posKey] = activeTierIndex
		at.trailingStopMutex.Unlock()

		// 根据档位配置计算目标止损利润百分比
		var targetStopProfitPct float64
		if activeTier.StopOffset < 0 {
			// 负值表示固定利润目标
			targetStopProfitPct = -activeTier.StopOffset
		} else {
			// 正值表示当前利润减去偏移量
			targetStopProfitPct = profitPct - activeTier.StopOffset
			// 确保不低于0（不能设置为亏损）
			if targetStopProfitPct < 0 {
				targetStopProfitPct = 0
			}
		}

		// 将目标止损利润百分比转换为价格
		// WICHTIG: profitPct ist bereits MIT Leverage multipliziert (Zeile 2563)!
		// targetStopProfitPct ist z.B. 7% (bei 10% Profit - 3% Offset)
		// Wir müssen zurück zum realen Preis-Profit rechnen: 7% / leverage = 0.7% realer Preis-Move
		// Dann: entryPrice * (1 + 0.007) für Long
		var newStopPrice float64
		stopProfitRatio := targetStopProfitPct / (leverage * 100)

		if side == "long" {
			newStopPrice = entryPrice * (1 + stopProfitRatio)
			
			// Stop darf nicht über aktuellem Preis sein (würde sofort triggern)
			if newStopPrice >= markPrice {
				log.Printf("⚠️ Long Stop %.4f >= Mark %.4f für %s! Setze auf Mark - 0.1%%", 
					newStopPrice, markPrice, symbol)
				newStopPrice = markPrice * 0.999
			}
		} else {
			newStopPrice = entryPrice * (1 - stopProfitRatio)
			
			// Stop darf nicht unter aktuellem Preis sein (würde sofort triggern)
			if newStopPrice <= markPrice {
				log.Printf("⚠️ Short Stop %.4f <= Mark %.4f für %s! Setze auf Mark + 0.1%%", 
					newStopPrice, markPrice, symbol)
				newStopPrice = markPrice * 1.001
			}
		}

		// 检查是否需要更新（避免频繁API调用）
		at.trailingStopMutex.RLock()
		lastStopPrice, hasLastPrice := at.trailingStopLastUpdate[posKey]
		at.trailingStopMutex.RUnlock()

		var priceChangePct float64
		shouldUpdate := false
		skipReason := ""
		
		if !hasLastPrice {
			// 第一次设置
			shouldUpdate = true
		} else {
			// 计算价格变化百分比
			priceChangePct = math.Abs((newStopPrice-lastStopPrice)/lastStopPrice) * 100

			// 只有止损价格向有利方向移动超过阈值时才更新
			if side == "long" {
				if newStopPrice > lastStopPrice {
					if priceChangePct >= at.trailingStopConfig.UpdateThresholdPct {
						shouldUpdate = true
					} else {
						skipReason = fmt.Sprintf("价格变化%.4f%%未达到更新阈值%.2f%%", priceChangePct, at.trailingStopConfig.UpdateThresholdPct)
					}
				} else {
					skipReason = fmt.Sprintf("新止损价%.4f <= 旧止损价%.4f，不向有利方向移动", newStopPrice, lastStopPrice)
				}
			} else if side == "short" {
				if newStopPrice < lastStopPrice {
					if priceChangePct >= at.trailingStopConfig.UpdateThresholdPct {
						shouldUpdate = true
					} else {
						skipReason = fmt.Sprintf("价格变化%.4f%%未达到更新阈值%.2f%%", priceChangePct, at.trailingStopConfig.UpdateThresholdPct)
					}
				} else {
					skipReason = fmt.Sprintf("新止损价%.4f >= 旧止损价%.4f，不向有利方向移动", newStopPrice, lastStopPrice)
				}
			}
		}

		// Log the check action
		oldStopPtr := &lastStopPrice
		if !hasLastPrice {
			oldStopPtr = nil
		}
		priceChangePctPtr := &priceChangePct
		updateThresholdPtr := &at.trailingStopConfig.UpdateThresholdPct
		
		at.logTrailingStopAction(symbol, side, "check", markPrice, entryPrice, profitPct, activeTierIndex, 
			&activeTier.ProfitThreshold, oldStopPtr, &newStopPrice, &targetStopProfitPct, 
			priceChangePctPtr, updateThresholdPtr, &shouldUpdate, nil, nil, skipReason)

		if !shouldUpdate {
			continue
		}

		// 执行止损更新
		normalizedSymbol := normalizeDealSymbol(symbol)
		positionSide := strings.ToUpper(side)

		log.Printf("🎯 [自动追踪·档位%.1f%%] %s %s | 当前利润: %.2f%% | 目标止损利润: %.2f%% | 止损价: %.4f",
			activeTier.ProfitThreshold, normalizedSymbol, side, profitPct, targetStopProfitPct, newStopPrice)

		// 取消旧的止损单
		if err := at.trader.CancelStopLossOrders(symbol); err != nil {
			log.Printf("⚠️ [自动追踪] 取消旧止损单失败 (%s %s): %v", normalizedSymbol, side, err)
			apiError := fmt.Sprintf("取消旧止损单失败: %v", err)
			at.logTrailingStopAction(symbol, side, "error", markPrice, entryPrice, profitPct, activeTierIndex,
				&activeTier.ProfitThreshold, oldStopPtr, &newStopPrice, &targetStopProfitPct,
				priceChangePctPtr, updateThresholdPtr, &shouldUpdate, nil, &apiError, "")
			continue
		}

		// 设置新的止损单
		err = at.trader.SetStopLoss(symbol, positionSide, quantity, newStopPrice)
		if err != nil {
			log.Printf("⚠️ [自动追踪] 设置止损失败 (%s %s): %v", normalizedSymbol, side, err)
			apiError := fmt.Sprintf("设置止损失败: %v", err)
			apiSuccess := false
			at.logTrailingStopAction(symbol, side, "update", markPrice, entryPrice, profitPct, activeTierIndex,
				&activeTier.ProfitThreshold, oldStopPtr, &newStopPrice, &targetStopProfitPct,
				priceChangePctPtr, updateThresholdPtr, &shouldUpdate, &apiSuccess, &apiError, "")
			continue
		}

		// 更新最后设置的止损价格
		at.trailingStopMutex.Lock()
		at.trailingStopLastUpdate[posKey] = newStopPrice
		at.trailingStopMutex.Unlock()

		log.Printf("✅ [自动追踪] 止损已更新: %s %s → %.4f", normalizedSymbol, side, newStopPrice)
		
		// Log successful update
		apiSuccess := true
		at.logTrailingStopAction(symbol, side, "update", markPrice, entryPrice, profitPct, activeTierIndex,
			&activeTier.ProfitThreshold, oldStopPtr, &newStopPrice, &targetStopProfitPct,
			priceChangePctPtr, updateThresholdPtr, &shouldUpdate, &apiSuccess, nil, "")
	}
}

// logTrailingStopAction logs trailing stop actions to database
func (at *AutoTrader) logTrailingStopAction(symbol, side, action string, currentPrice, entryPrice, profitPct float64,
	tierIndex int, tierThreshold, oldStopPrice, newStopPrice, targetStopProfitPct, priceChangePct, updateThresholdPct *float64,
	shouldUpdate, apiSuccess *bool, apiError *string, skipReason string) {
	
	// Get deal ID for this position
	dealID, openRec := at.getOpenDealRecord(symbol, side)
	if openRec == nil {
		return // No deal found, skip logging
	}

	logger, ok := at.database.(interface {
		LogTrailingStopAction(log *config.TrailingStopLog) error
	})
	if !ok {
		return // Database doesn't support trailing stop logging
	}

	logEntry := &config.TrailingStopLog{
		UserID:       at.userID,
		TraderID:     at.id,
		DealID:       dealID,
		Symbol:       normalizeDealSymbol(symbol),
		Side:         side,
		Action:       action,
		CurrentPrice: currentPrice,
		EntryPrice:   entryPrice,
		ProfitPct:    profitPct,
		TierIndex:    tierIndex,
	}

	if tierThreshold != nil {
		logEntry.TierThreshold = sql.NullFloat64{Float64: *tierThreshold, Valid: true}
	}
	if oldStopPrice != nil {
		logEntry.OldStopPrice = sql.NullFloat64{Float64: *oldStopPrice, Valid: true}
	}
	if newStopPrice != nil {
		logEntry.NewStopPrice = sql.NullFloat64{Float64: *newStopPrice, Valid: true}
	}
	if targetStopProfitPct != nil {
		logEntry.TargetStopProfitPct = sql.NullFloat64{Float64: *targetStopProfitPct, Valid: true}
	}
	if priceChangePct != nil {
		logEntry.PriceChangePct = sql.NullFloat64{Float64: *priceChangePct, Valid: true}
	}
	if updateThresholdPct != nil {
		logEntry.UpdateThresholdPct = sql.NullFloat64{Float64: *updateThresholdPct, Valid: true}
	}
	if shouldUpdate != nil {
		logEntry.ShouldUpdate = sql.NullBool{Bool: *shouldUpdate, Valid: true}
	}
	if apiSuccess != nil {
		logEntry.APISuccess = sql.NullBool{Bool: *apiSuccess, Valid: true}
	}
	if apiError != nil && *apiError != "" {
		logEntry.APIError = sql.NullString{String: *apiError, Valid: true}
	}
	if skipReason != "" {
		logEntry.SkipReason = sql.NullString{String: skipReason, Valid: true}
	}

	if err := logger.LogTrailingStopAction(logEntry); err != nil {
		log.Printf("⚠️ 记录追踪止损日志失败: %v", err)
	}
}

