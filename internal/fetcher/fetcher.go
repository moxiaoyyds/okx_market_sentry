package fetcher

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	okxcommon "github.com/nntaoli-project/goex/v2/okx/common"
	"go.uber.org/zap"
	"okx-market-sentr
	"okx-market-sentry/internal/storage"
	"okx-market-sentry/pkg/types"
)

// DataFetcher 数据获取器
type DataFetcher struct {
	storage    *storage.StateManager
	interval   time.Duration
	okxClient  *okxcommon.OKxV5
	httpClient *http.Client // 自定义HTTP客户端
}

func NewDataFetcher(stateManager *storage.StateManager, networkConfig types.NetworkConfig) *DataFetcher {
	// 使用goex v2 OKX客户端
	client := okxcommon.New()

	// 设置超时时间
	timeout := networkConfig.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// 创建自定义HTTP客户端
	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	// 如果配置了代理，则使用代理
	if networkConfig.Proxy != "" {
		proxyURL, err := url.Parse(networkConfig.Proxy)
		if err == nil {
			httpClient.Transport.(*http.Transport).Proxy = http.ProxyURL(proxyURL)
			zap.L().Info("✅ 已配置HTTP代理", zap.String("proxy", networkConfig.Proxy))
		} else {
			zap.L().Warn("⚠️ 代理地址格式错误", zap.Error(err))
		}
	}

	// 通过反射或其他方式设置HTTP客户端（goex v2可能需要不同的方法）
	// 暂时先创建基础客户端，后续在请求中使用自定义HTTP客户端

	zap.L().Info("✅ 初始化goex v2 OKX客户端", zap.Duration("timeout", timeout))

	return &DataFetcher{
		storage:    stateManager,
		interval:   1 * time.Minute,
		okxClient:  client,
		httpClient: httpClient, // 保存自定义HTTP客户端供后续使用
	}
}

func (f *DataFetcher) Start(ctx context.Context) {
	zap.L().Info("🚀 数据获取器启动，开始获取OKX V5真实市场数据...")

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	// 立即执行一次
	f.fetchAndStore()

	for {
		select {
		case <-ctx.Done():
			zap.L().Info("📴 数据获取器已停止")
			return
		case <-ticker.C:
			f.fetchAndStore()
		}
	}
}

	zap.L().Info("🔄 正在使用goex v2获取OKX市场数据...",
	zap.L().Info("🔄 正在使用goex v2获取OKX市场数据...", 
		zap.String("time", time.Now().Format("15:04:05")))

	// 获取所有现货交易对的ticker数据
	tickers, err := f.getTickers()
	if err != nil {
		zap.L().Error("❌ 获取市场数据失败", zap.Error(err))
		return
	}

	count := len(tickers)
	usdtCount := 0

	for _, ticker := range tickers {
		// 检查是否为USDT交易对并存储价格数据
		if strings.HasSuffix(ticker.InstId, "-USDT") {
			// 解析价格字符串为float64
			if price, err := strconv.ParseFloat(ticker.Last, 64); err == nil && price > 0 {
				f.storage.Store(ticker.InstId, price, time.Now())
				usdtCount++
			}
		}
	}
	zap.L().Info("✅ 获取到交易对数据",
	zap.L().Info("✅ 获取到交易对数据", 
		zap.Int("total_count", count),
		zap.Int("usdt_count", usdtCount))
}

// Ticker 定义ticker响应结构
type Ticker struct {
	InstId    string `json:"instId"`
	Last      string `json:"last"`
	Open24h   string `json:"open24h"`
	High24h   string `json:"high24h"`
	Low24h    string `json:"low24h"`
	Vol24h    string `json:"vol24h"`
	VolCcy24h string `json:"volCcy24h"`
	Ts        string `json:"ts"`
}

// getTickers 使用自定义HTTP客户端直接获取OKX ticker数据（支持代理）
func (f *DataFetcher) getTickers() ([]Ticker, error) {
	// 重试机制：最多重试3次
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			zap.L().Info("🔄 重试获取数据", zap.Int("attempt", attempt))
			time.Sleep(time.Duration(attempt) * time.Second) // 指数退避
		}

		// 直接使用自定义HTTP客户端发送请求，绕过goex库的限制
		apiURL := "https://www.okx.com/api/v5/market/tickers?instType=SPOT"

		resp, err := f.httpClient.Get(apiURL)
		if err != nil {
			lastErr = fmt.Errorf("HTTP请求失败(第%d次尝试): %v", attempt, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("HTTP状态码错误(第%d次尝试): %d", attempt, resp.StatusCode)
			continue
		}

		// 读取响应体
		var body bytes.Buffer
		_, err = body.ReadFrom(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("读取响应失败(第%d次尝试): %v", attempt, err)
			continue
		}

		// 解析OKX API响应格式
		var apiResp struct {
			Code string   `json:"code"`
			Msg  string   `json:"msg"`
			Data []Ticker `json:"data"`
		}

		if err := json.Unmarshal(body.Bytes(), &apiResp); err != nil {
			lastErr = fmt.Errorf("解析API响应失败(第%d次尝试): %v", attempt, err)
			continue
		}

		if apiResp.Code != "0" {
			lastErr = fmt.Errorf("API返回错误(第%d次尝试): %s - %s", attempt, apiResp.Code, apiResp.Msg)
			continue
		}

		// 过滤出USDT交易对
		usdtTickers := make([]Ticker, 0)
		for _, ticker := range apiResp.Data {
			if strings.HasSuffix(ticker.InstId, "-USDT") {
				usdtTickers = append(usdtTickers, ticker)
			}
		}
		zap.L().Info("📊 使用代理从交易对中筛选出USDT交易对",
		zap.L().Info("📊 使用代理从交易对中筛选出USDT交易对", 
			zap.Int("total_pairs", len(apiResp.Data)),
			zap.Int("usdt_pairs", len(usdtTickers)))
		return usdtTickers, nil
	}

	return nil, lastErr
}
