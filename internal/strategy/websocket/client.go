package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"okx-market-sentry/pkg/types"
)

// Client WebSocket客户端，按照OKX实际行为优化
type Client struct {
	endpoint      string
	proxy         string
	conn          *websocket.Conn
	mu            sync.RWMutex
	isConnected   bool
	reconnectChan chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
	klineChan     chan *types.KLine
	config        types.WebSocketConfig

	// 新增：存储最新的K线数据
	latestKlines map[string]*types.KLine // symbol -> latest kline
	klinesMutex  sync.RWMutex

	// 定时器配置
	interval string
	symbols  []string
	ticker   *time.Ticker
}

// OKXKlineResponse OKX K线数据响应
type OKXKlineResponse struct {
	Arg struct {
		Channel string `json:"channel"`
		InstID  string `json:"instId"`
	} `json:"arg"`
	Data [][]string `json:"data"`
}

// OKXSubscription OKX订阅消息
type OKXSubscription struct {
	Op   string `json:"op"`
	Args []struct {
		Channel string `json:"channel"`
		InstID  string `json:"instId"`
	} `json:"args"`
}

// NewClient 创建新的WebSocket客户端
func NewClient(endpoint, proxy string, config types.WebSocketConfig) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		endpoint:      endpoint,
		proxy:         proxy,
		reconnectChan: make(chan struct{}, 1),
		ctx:           ctx,
		cancel:        cancel,
		klineChan:     make(chan *types.KLine, 1000),
		config:        config,
		latestKlines:  make(map[string]*types.KLine),
	}
}

// Connect 建立WebSocket连接
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 设置Dialer
	dialer := websocket.DefaultDialer
	if c.proxy != "" {
		proxyURL, err := url.Parse(c.proxy)
		if err != nil {
			return fmt.Errorf("解析代理URL失败: %v", err)
		}
		dialer.Proxy = http.ProxyURL(proxyURL)
	}

	// 建立连接 - 使用正确的OKX WebSocket路径
	wsURL := strings.Replace(c.endpoint, "/ws/v5/public", "/ws/v5/business", 1)
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %v", err)
	}

	c.conn = conn
	c.isConnected = true

	zap.L().Info("🔗 WebSocket连接建立成功",
		zap.String("endpoint", wsURL))

	return nil
}

// Subscribe 订阅K线数据并启动定时读取
func (c *Client) Subscribe(symbols []string, interval string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.isConnected || c.conn == nil {
		return fmt.Errorf("WebSocket未连接")
	}

	// 保存配置
	c.symbols = symbols
	c.interval = interval

	// 根据OKX文档，使用mark-price-candle格式
	channelName := fmt.Sprintf("mark-price-candle%s", interval)

	// 构建订阅消息
	subscription := OKXSubscription{
		Op: "subscribe",
	}

	for _, symbol := range symbols {
		subscription.Args = append(subscription.Args, struct {
			Channel string `json:"channel"`
			InstID  string `json:"instId"`
		}{
			Channel: channelName,
			InstID:  symbol,
		})
	}

	// 发送订阅消息
	if err := c.conn.WriteJSON(subscription); err != nil {
		return fmt.Errorf("发送订阅消息失败: %v", err)
	}

	zap.L().Info("📡 已发送K线订阅请求",
		zap.Strings("symbols", symbols),
		zap.String("channel", channelName))

	// 启动定时处理器
	c.startIntervalProcessor()

	return nil
}

// startIntervalProcessor 启动定时处理器，按我们的时间周期读取数据
func (c *Client) startIntervalProcessor() {
	// 解析时间间隔
	duration := c.parseIntervalToDuration(c.interval)

	// 创建定时器
	c.ticker = time.NewTicker(duration)

	go func() {
		defer c.ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-c.ticker.C:
				// 每个时间周期读取一次最新的完整K线数据
				c.processLatestKlines()
			}
		}
	}()

	zap.L().Info("⏰ 启动K线定时处理器",
		zap.String("interval", c.interval),
		zap.Duration("duration", duration))
}

// processLatestKlines 处理最新的K线数据
func (c *Client) processLatestKlines() {
	c.klinesMutex.RLock()
	defer c.klinesMutex.RUnlock()

	processedCount := 0
	for symbol, kline := range c.latestKlines {
		if kline != nil {
			// 只处理完整的K线（confirm=1）
			select {
			case c.klineChan <- kline:
				processedCount++
				zap.L().Debug("📊 处理K线数据",
					zap.String("symbol", symbol),
					zap.Time("time", kline.OpenTime),
					zap.Float64("close", kline.Close),
					zap.Float64("volume", kline.Volume))
			default:
				zap.L().Warn("K线数据通道满，丢弃数据", zap.String("symbol", symbol))
			}
		}
	}

	if processedCount > 0 {
		zap.L().Info("✅ 定时处理K线数据完成",
			zap.Int("processed_count", processedCount),
			zap.Int("total_symbols", len(c.symbols)))
	}
}

// StartReading 开始读取WebSocket数据
func (c *Client) StartReading() {
	go c.readLoop()
	go c.reconnectLoop()
	go c.pingLoop()
}

// readLoop 读取数据循环 - 持续接收OKX推送的数据
func (c *Client) readLoop() {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("WebSocket读取panic", zap.Any("error", r))
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				time.Sleep(time.Second)
				continue
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				zap.L().Error("WebSocket读取消息失败", zap.Error(err))
				c.handleDisconnect()
				continue
			}

			// 解析并缓存K线数据，但不立即发送到处理通道
			if err := c.cacheKlineData(message); err != nil {
				zap.L().Debug("解析K线数据失败", zap.Error(err))
			}
		}
	}
}

// cacheKlineData 缓存K线数据，只保存最新的完整K线
func (c *Client) cacheKlineData(message []byte) error {
	var response OKXKlineResponse
	if err := json.Unmarshal(message, &response); err != nil {
		return err
	}

	// 检查是否是K线数据
	if !strings.HasPrefix(response.Arg.Channel, "mark-price-candle") {
		return nil // 忽略非K线数据
	}

	// 解析每条K线数据
	for _, data := range response.Data {
		if len(data) < 6 {
			continue
		}

		// 检查K线是否完结 (confirm字段)
		if len(data) >= 6 && data[5] != "1" {
			continue // 只处理完结的K线
		}

		kline, err := c.parseOKXKlineData(response.Arg.InstID, data, response.Arg.Channel)
		if err != nil {
			continue
		}

		// 缓存最新的完整K线数据
		c.klinesMutex.Lock()
		c.latestKlines[kline.Symbol] = kline
		c.klinesMutex.Unlock()

		zap.L().Debug("💾 缓存完整K线数据",
			zap.String("symbol", kline.Symbol),
			zap.Time("time", kline.OpenTime),
			zap.Float64("close", kline.Close))
	}

	return nil
}

// parseOKXKlineData 解析OKX K线数据格式
func (c *Client) parseOKXKlineData(symbol string, data []string, channel string) (*types.KLine, error) {
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

	// 成交量可能不在mark-price-candle中，设为0
	volume := 0.0

	// 提取时间间隔
	interval := strings.TrimPrefix(channel, "mark-price-candle")

	return &types.KLine{
		Symbol:    symbol,
		OpenTime:  time.Unix(timestamp/1000, (timestamp%1000)*1000000),
		CloseTime: time.Unix(timestamp/1000, (timestamp%1000)*1000000).Add(c.parseIntervalToDuration(interval)),
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
		Interval:  interval,
	}, nil
}

// parseIntervalToDuration 解析时间间隔字符串为Duration
func (c *Client) parseIntervalToDuration(interval string) time.Duration {
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
		return 5 * time.Minute // 默认5分钟
	}
}

// reconnectLoop 重连循环
func (c *Client) reconnectLoop() {
	ticker := time.NewTicker(c.config.ReconnectInterval)
	defer ticker.Stop()

	reconnectAttempts := 0

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.reconnectChan:
			reconnectAttempts++
			if reconnectAttempts > c.config.MaxReconnectAttempts {
				zap.L().Error("达到最大重连次数，停止重连",
					zap.Int("max_attempts", c.config.MaxReconnectAttempts))
				return
			}

			zap.L().Info("🔄 尝试重连WebSocket",
				zap.Int("attempt", reconnectAttempts),
				zap.Int("max_attempts", c.config.MaxReconnectAttempts))

			if err := c.Connect(); err != nil {
				zap.L().Error("重连失败", zap.Error(err))
				time.Sleep(c.config.ReconnectInterval)
				c.reconnectChan <- struct{}{}
				continue
			}

			// 重连成功后重新订阅
			if len(c.symbols) > 0 {
				if err := c.Subscribe(c.symbols, c.interval); err != nil {
					zap.L().Error("重连后重新订阅失败", zap.Error(err))
				}
			}

			// 重连成功，重置重连次数
			reconnectAttempts = 0
			zap.L().Info("✅ WebSocket重连成功")
		}
	}
}

// pingLoop 心跳循环
func (c *Client) pingLoop() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			isConnected := c.isConnected
			c.mu.RUnlock()

			if !isConnected || conn == nil {
				continue
			}

			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				zap.L().Error("发送心跳失败", zap.Error(err))
				c.handleDisconnect()
			}
		}
	}
}

// handleDisconnect 处理断线
func (c *Client) handleDisconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.isConnected = false

	// 触发重连
	select {
	case c.reconnectChan <- struct{}{}:
	default:
	}
}

// GetKlineChannel 获取K线数据通道
func (c *Client) GetKlineChannel() <-chan *types.KLine {
	return c.klineChan
}

// Close 关闭WebSocket连接
func (c *Client) Close() error {
	c.cancel()

	if c.ticker != nil {
		c.ticker.Stop()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.isConnected = false
		return err
	}

	return nil
}

// IsConnected 检查连接状态
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected
}
