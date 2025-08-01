package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	okxcommon "github.com/nntaoli-project/goex/v2/okx/common"
	"okx-market-sentry/internal/storage"
)

// DataFetcher 数据获取器
type DataFetcher struct {
	storage   *storage.StateManager
	interval  time.Duration
	okxClient *okxcommon.OKxV5
}

func NewDataFetcher(stateManager *storage.StateManager) *DataFetcher {
	// 使用goex v2 OKX客户端
	client := okxcommon.New()

	fmt.Println("✅ 初始化goex v2 OKX客户端")

	return &DataFetcher{
		storage:   stateManager,
		interval:  1 * time.Minute,
		okxClient: client,
	}
}

func (f *DataFetcher) Start(ctx context.Context) {
	fmt.Println("🚀 数据获取器启动，开始获取OKX V5真实市场数据...")

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	// 立即执行一次
	f.fetchAndStore()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("📴 数据获取器已停止")
			return
		case <-ticker.C:
			f.fetchAndStore()
		}
	}
}

func (f *DataFetcher) fetchAndStore() {
	fmt.Printf("🔄 正在使用goex v2获取OKX市场数据... [%s]\n", time.Now().Format("15:04:05"))

	// 获取所有现货交易对的ticker数据
	tickers, err := f.getTickers()
	if err != nil {
		fmt.Printf("❌ 获取市场数据失败: %v\n", err)
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

	fmt.Printf("✅ 获取到 %d 个交易对，其中 %d 个USDT交易对已存储\n", count, usdtCount)
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

// getTickers 使用goex v2获取OKX所有现货交易对ticker数据
func (f *DataFetcher) getTickers() ([]Ticker, error) {
	// 使用goex v2的DoNoAuthRequest方法调用OKX tickers API
	params := &url.Values{}
	params.Set("instType", "SPOT")

	data, responseBody, err := f.okxClient.DoNoAuthRequest("GET", f.okxClient.UriOpts.Endpoint+"/api/v5/market/tickers", params)
	if err != nil {
		return nil, fmt.Errorf("goex v2请求失败: %v, response: %s", err, string(responseBody))
	}

	// 解析响应数据
	var tickers []Ticker
	if err := json.Unmarshal(data, &tickers); err != nil {
		return nil, fmt.Errorf("解析ticker数据失败: %v", err)
	}

	// 过滤出USDT交易对
	usdtTickers := make([]Ticker, 0)
	for _, ticker := range tickers {
		if strings.HasSuffix(ticker.InstId, "-USDT") {
			usdtTickers = append(usdtTickers, ticker)
		}
	}

	fmt.Printf("📊 使用goex v2从 %d 个交易对中筛选出 %d 个USDT交易对\n", len(tickers), len(usdtTickers))
	return usdtTickers, nil
}
