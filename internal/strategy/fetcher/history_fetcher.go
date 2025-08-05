package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.uber.org/zap"
	"okx-market-sentry/pkg/types"
)

// HistoryKlineFetcher 历史K线数据获取器
type HistoryKlineFetcher struct {
	baseURL    string
	proxy      string
	timeout    time.Duration
	httpClient *http.Client
}

// OKXHistoryKlineResponse OKX历史K线API响应
type OKXHistoryKlineResponse struct {
	Code string     `json:"code"`
	Msg  string     `json:"msg"`
	Data [][]string `json:"data"`
}

// NewHistoryKlineFetcher 创建历史K线获取器
func NewHistoryKlineFetcher(proxy string, timeout time.Duration) *HistoryKlineFetcher {
	client := &http.Client{
		Timeout: timeout,
	}

	// 设置代理
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}

	return &HistoryKlineFetcher{
		baseURL:    "https://www.okx.com/api/v5/market",
		proxy:      proxy,
		timeout:    timeout,
		httpClient: client,
	}
}

// FetchHistoryKlines 获取历史K线数据
func (h *HistoryKlineFetcher) FetchHistoryKlines(symbol, interval string, limit int) ([]*types.KLine, error) {
	// 构建请求URL
	requestURL := fmt.Sprintf("%s/history-index-candles?instId=%s&bar=%s&limit=%d",
		h.baseURL, symbol, interval, limit)

	zap.L().Info("📊 获取历史K线数据",
		zap.String("symbol", symbol),
		zap.String("interval", interval),
		zap.Int("limit", limit),
		zap.String("url", requestURL))

	// 创建请求
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "OKX-Market-Sentry/1.0")
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP响应错误: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %v", err)
	}

	// 解析JSON响应
	var okxResponse OKXHistoryKlineResponse
	if err := json.Unmarshal(body, &okxResponse); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %v", err)
	}

	// 检查OKX API返回码
	if okxResponse.Code != "0" {
		return nil, fmt.Errorf("OKX API返回错误: code=%s, msg=%s", okxResponse.Code, okxResponse.Msg)
	}

	// 转换为内部K线格式
	klines := make([]*types.KLine, 0, len(okxResponse.Data))
	for _, data := range okxResponse.Data {
		if len(data) < 5 {
			continue
		}

		kline, err := h.parseOKXKlineData(symbol, data, interval)
		if err != nil {
			zap.L().Warn("解析历史K线数据失败", zap.Error(err))
			continue
		}

		klines = append(klines, kline)
	}

	zap.L().Info("✅ 历史K线数据获取完成",
		zap.String("symbol", symbol),
		zap.Int("requested", limit),
		zap.Int("received", len(klines)))

	// OKX返回的数据是从新到旧排序，需要反转为从旧到新
	h.reverseKlines(klines)

	return klines, nil
}

// parseOKXKlineData 解析OKX K线数据格式
func (h *HistoryKlineFetcher) parseOKXKlineData(symbol string, data []string, interval string) (*types.KLine, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("K线数据格式不正确")
	}

	// OKX K线数据格式: [timestamp, open, high, low, close, confirm]
	timestamp, err := strconv.ParseInt(data[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("解析时间戳失败: %v", err)
	}

	open, err := strconv.ParseFloat(data[1], 64)
	if err != nil {
		return nil, fmt.Errorf("解析开盘价失败: %v", err)
	}

	high, err := strconv.ParseFloat(data[2], 64)
	if err != nil {
		return nil, fmt.Errorf("解析最高价失败: %v", err)
	}

	low, err := strconv.ParseFloat(data[3], 64)
	if err != nil {
		return nil, fmt.Errorf("解析最低价失败: %v", err)
	}

	close, err := strconv.ParseFloat(data[4], 64)
	if err != nil {
		return nil, fmt.Errorf("解析收盘价失败: %v", err)
	}

	// 成交量在历史API中不提供，设为0
	volume := 0.0

	return &types.KLine{
		Symbol:    symbol,
		OpenTime:  time.Unix(timestamp/1000, (timestamp%1000)*1000000),
		CloseTime: time.Unix(timestamp/1000, (timestamp%1000)*1000000).Add(h.parseIntervalToDuration(interval)),
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
		Interval:  interval,
	}, nil
}

// parseIntervalToDuration 解析时间间隔字符串为Duration
func (h *HistoryKlineFetcher) parseIntervalToDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "3m":
		return 3 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1H", "1h":
		return time.Hour
	case "2H", "2h":
		return 2 * time.Hour
	case "4H", "4h":
		return 4 * time.Hour
	case "6H", "6h":
		return 6 * time.Hour
	case "12H", "12h":
		return 12 * time.Hour
	case "1D", "1d":
		return 24 * time.Hour
	default:
		return 5 * time.Minute
	}
}

// reverseKlines 反转K线数组（从新到旧 → 从旧到新）
func (h *HistoryKlineFetcher) reverseKlines(klines []*types.KLine) {
	for i, j := 0, len(klines)-1; i < j; i, j = i+1, j-1 {
		klines[i], klines[j] = klines[j], klines[i]
	}
}

// FetchMultipleSymbolsHistory 批量获取多个交易对的历史数据
func (h *HistoryKlineFetcher) FetchMultipleSymbolsHistory(symbols []string, interval string, limit int) (map[string][]*types.KLine, error) {
	result := make(map[string][]*types.KLine)

	for i, symbol := range symbols {
		// 限速：10次/2s，所以每个请求间隔200毫秒
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}

		klines, err := h.FetchHistoryKlines(symbol, interval, limit)
		if err != nil {
			zap.L().Error("获取历史K线失败",
				zap.String("symbol", symbol),
				zap.Error(err))
			// 继续处理其他交易对，不中断整个过程
			continue
		}

		result[symbol] = klines

		zap.L().Debug("✅ 完成历史数据获取",
			zap.String("symbol", symbol),
			zap.Int("klines_count", len(klines)))
	}

	return result, nil
}
