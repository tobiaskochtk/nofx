package market

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// OIRankItem 表示单个币种的 Open Interest 排名数据
type OIRankItem struct {
	Symbol         string  `json:"symbol"`
	TotalOI        float64 `json:"total_oi"`         // 总 OI（Binance + Bybit）
	BinanceOI      float64 `json:"binance_oi"`       // Binance OI
	BybitOI        float64 `json:"bybit_oi"`         // Bybit OI
	BinanceOIValue float64 `json:"binance_oi_value"` // Binance OI 价值（USDT）
	BybitOIValue   float64 `json:"bybit_oi_value"`   // Bybit OI 价值（USDT）
	TotalOIValue   float64 `json:"total_oi_value"`   // 总 OI 价值（USDT）
}

// OIAggregator Open Interest 聚合器
type OIAggregator struct {
	client *http.Client
}

// NewOIAggregator 创建 OI 聚合器
func NewOIAggregator() *OIAggregator {
	return &OIAggregator{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// BinanceOIResponse Binance Open Interest 响应
type BinanceOIResponse struct {
	Symbol               string `json:"symbol"`
	OpenInterest         string `json:"openInterest"`
	Time                 int64  `json:"time"`
	OpenInterestValue    string `json:"openInterestValue,omitempty"`
	SumOpenInterest      string `json:"sumOpenInterest,omitempty"`
	SumOpenInterestValue string `json:"sumOpenInterestValue,omitempty"`
}

// BybitOIResponse Bybit Open Interest 响应
type BybitOIResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []struct {
			Symbol       string `json:"symbol"`
			OpenInterest string `json:"openInterest"`
		} `json:"list"`
	} `json:"result"`
}

// GetOIRanking 获取 Open Interest 排名
// symbols: 要查询的币种列表（如 []string{"BTCUSDT", "ETHUSDT"}）
// 如果 symbols 为空，则返回所有支持的币种
func (oa *OIAggregator) GetOIRanking(symbols []string) ([]OIRankItem, error) {
	// 标准化币种格式（XYZUSDC → XYZUSDT）
	normalizedSymbols := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		// 移除空格
		sym = strings.TrimSpace(sym)
		// 统一大写
		sym = strings.ToUpper(sym)
		// 将 USDC 替换为 USDT（Binance/Bybit OI 主要是 USDT 永续）
		if strings.HasSuffix(sym, "USDC") {
			sym = strings.TrimSuffix(sym, "USDC") + "USDT"
		}
		// 确保以 USDT 结尾
		if !strings.HasSuffix(sym, "USDT") {
			sym += "USDT"
		}
		normalizedSymbols = append(normalizedSymbols, sym)
	}

	// 并行查询 Binance 和 Bybit
	var wg sync.WaitGroup
	var binanceData map[string]*BinanceOIResponse
	var bybitData map[string]string
	var binanceErr, bybitErr error

	wg.Add(2)

	// 查询 Binance
	go func() {
		defer wg.Done()
		binanceData, binanceErr = oa.fetchBinanceOI(normalizedSymbols)
	}()

	// 查询 Bybit
	go func() {
		defer wg.Done()
		bybitData, bybitErr = oa.fetchBybitOI(normalizedSymbols)
	}()

	wg.Wait()

	// 记录错误但不中断（部分数据总比没有好）
	if binanceErr != nil {
		log.Printf("⚠️ Binance OI 查询失败: %v", binanceErr)
	}
	if bybitErr != nil {
		log.Printf("⚠️ Bybit OI 查询失败: %v", bybitErr)
	}

	// 合并数据
	ranking := oa.mergeOIData(normalizedSymbols, binanceData, bybitData)

	// 按总 OI 价值排序（降序）
	sort.Slice(ranking, func(i, j int) bool {
		return ranking[i].TotalOIValue > ranking[j].TotalOIValue
	})

	return ranking, nil
}

// fetchBinanceOI 获取 Binance Open Interest 数据
func (oa *OIAggregator) fetchBinanceOI(symbols []string) (map[string]*BinanceOIResponse, error) {
	result := make(map[string]*BinanceOIResponse)

	// Binance API: 如果不指定 symbol，返回所有币种
	url := "https://fapi.binance.com/fapi/v1/openInterest"
	
	if len(symbols) == 0 {
		// 获取所有币种（通过 exchangeInfo）
		allSymbols, err := oa.getAllBinanceSymbols()
		if err != nil {
			return nil, fmt.Errorf("获取 Binance 所有币种失败: %w", err)
		}
		symbols = allSymbols
	}

	// Binance API 每次只能查询单个币种，需要批量并发查询
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(symbols))

	// 限制并发数量（防止 rate limit）
	semaphore := make(chan struct{}, 10)

	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			semaphore <- struct{}{} // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			reqURL := fmt.Sprintf("%s?symbol=%s", url, sym)
			resp, err := oa.client.Get(reqURL)
			if err != nil {
				errChan <- fmt.Errorf("查询 %s 失败: %w", sym, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				// 跳过不支持的币种（如现货）
				if resp.StatusCode == http.StatusBadRequest {
					return
				}
				errChan <- fmt.Errorf("Binance API 返回错误: %s (status: %d)", sym, resp.StatusCode)
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				errChan <- fmt.Errorf("读取响应失败: %w", err)
				return
			}

			var oiData BinanceOIResponse
			if err := json.Unmarshal(body, &oiData); err != nil {
				errChan <- fmt.Errorf("解析 JSON 失败: %w", err)
				return
			}

			mu.Lock()
			result[sym] = &oiData
			mu.Unlock()
		}(symbol)
	}

	wg.Wait()
	close(errChan)

	// 收集错误（只记录第一个）
	for err := range errChan {
		log.Printf("⚠️ Binance OI 查询部分失败: %v", err)
		break
	}

	return result, nil
}

// fetchBybitOI 获取 Bybit Open Interest 数据
func (oa *OIAggregator) fetchBybitOI(symbols []string) (map[string]string, error) {
	result := make(map[string]string)

	// Bybit API v5: 获取所有 USDT 永续合约的 OI
	// 文档: https://bybit-exchange.github.io/docs/v5/market/open-interest
	url := "https://api.bybit.com/v5/market/tickers?category=linear"

	resp, err := oa.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Bybit API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bybit API 返回错误: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Bybit 响应失败: %w", err)
	}

	var oiResp struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []struct {
				Symbol       string `json:"symbol"`
				OpenInterest string `json:"openInterest"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &oiResp); err != nil {
		return nil, fmt.Errorf("解析 Bybit JSON 失败: %w", err)
	}

	if oiResp.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API 错误: %s", oiResp.RetMsg)
	}

	// 如果指定了 symbols，只返回这些币种
	symbolSet := make(map[string]bool)
	if len(symbols) > 0 {
		for _, sym := range symbols {
			symbolSet[sym] = true
		}
	}

	// 提取 OI 数据
	for _, item := range oiResp.Result.List {
		// 跳过非 USDT 合约
		if !strings.HasSuffix(item.Symbol, "USDT") {
			continue
		}

		// 如果指定了 symbols，过滤
		if len(symbolSet) > 0 && !symbolSet[item.Symbol] {
			continue
		}

		result[item.Symbol] = item.OpenInterest
	}

	return result, nil
}

// getAllBinanceSymbols 获取所有 Binance USDT 永续合约币种
func (oa *OIAggregator) getAllBinanceSymbols() ([]string, error) {
	url := "https://fapi.binance.com/fapi/v1/exchangeInfo"
	resp, err := oa.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var exchangeInfo struct {
		Symbols []struct {
			Symbol string `json:"symbol"`
			Status string `json:"status"`
		} `json:"symbols"`
	}

	if err := json.Unmarshal(body, &exchangeInfo); err != nil {
		return nil, err
	}

	var symbols []string
	for _, s := range exchangeInfo.Symbols {
		// 只保留 USDT 永续合约且状态为 TRADING
		if strings.HasSuffix(s.Symbol, "USDT") && s.Status == "TRADING" {
			symbols = append(symbols, s.Symbol)
		}
	}

	return symbols, nil
}

// mergeOIData 合并 Binance 和 Bybit 的 OI 数据
func (oa *OIAggregator) mergeOIData(
	symbols []string,
	binanceData map[string]*BinanceOIResponse,
	bybitData map[string]string,
) []OIRankItem {
	// 使用 map 去重（支持来自两个交易所的不同币种）
	mergedMap := make(map[string]*OIRankItem)

	// 添加 Binance 数据
	if binanceData != nil {
		for symbol, data := range binanceData {
			oi, _ := strconv.ParseFloat(data.OpenInterest, 64)
			oiValue, _ := strconv.ParseFloat(data.OpenInterestValue, 64)
			if oiValue == 0 {
				oiValue, _ = strconv.ParseFloat(data.SumOpenInterestValue, 64)
			}

			mergedMap[symbol] = &OIRankItem{
				Symbol:         symbol,
				BinanceOI:      oi,
				BinanceOIValue: oiValue,
				TotalOI:        oi,
				TotalOIValue:   oiValue,
			}
		}
	}

	// 添加 Bybit 数据
	if bybitData != nil {
		for symbol, oi := range bybitData {
			oiVal, _ := strconv.ParseFloat(oi, 64)

			if existing, ok := mergedMap[symbol]; ok {
				// 币种已存在（来自 Binance），累加
				existing.BybitOI = oiVal
				existing.TotalOI += oiVal
				// Bybit 的 openInterest 已经是 USDT 价值
				existing.BybitOIValue = oiVal
				existing.TotalOIValue += oiVal
			} else {
				// 新币种（只在 Bybit 上）
				mergedMap[symbol] = &OIRankItem{
					Symbol:       symbol,
					BybitOI:      oiVal,
					BybitOIValue: oiVal,
					TotalOI:      oiVal,
					TotalOIValue: oiVal,
				}
			}
		}
	}

	// 如果指定了 symbols，只返回这些币种
	var result []OIRankItem
	if len(symbols) > 0 {
		symbolSet := make(map[string]bool)
		for _, sym := range symbols {
			symbolSet[sym] = true
		}

		for symbol, item := range mergedMap {
			if symbolSet[symbol] {
				result = append(result, *item)
			}
		}
	} else {
		// 返回所有币种
		for _, item := range mergedMap {
			result = append(result, *item)
		}
	}

	return result
}
