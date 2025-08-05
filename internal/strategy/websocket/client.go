package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"okx-market-sentry/pkg/types"
)

// Client WebSocket客户端
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
		klineChan:     make(chan *types.KLine, 1000), // 缓冲1000个K线数据
		config:        config,
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

	// 建立连接
	conn, _, err := dialer.Dial(c.endpoint, nil)
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %v", err)
	}

	c.conn = conn
	c.isConnected = true

	zap.L().Info("✅ WebSocket连接建立成功",
		zap.String("endpoint", c.endpoint),
		zap.String("proxy", c.proxy))

	return nil
}

// Subscribe 订阅K线数据
func (c *Client) Subscribe(symbols []string, interval string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.isConnected || c.conn == nil {
		return fmt.Errorf("WebSocket未连接")
	}

	// 构建订阅消息
	subscription := OKXSubscription{
		Op: "subscribe",
	}

	for _, symbol := range symbols {
		subscription.Args = append(subscription.Args, struct {
			Channel string `json:"channel"`
			InstID  string `json:"instId"`
		}{
			Channel: fmt.Sprintf("candle%s", interval),
			InstID:  symbol,
		})
	}

	// 发送订阅消息
	if err := c.conn.WriteJSON(subscription); err != nil {
		return fmt.Errorf("发送订阅消息失败: %v", err)
	}

	zap.L().Info("📊 已订阅K线数据",
		zap.Strings("symbols", symbols),
		zap.String("interval", interval))

	return nil
}

// StartReading 开始读取WebSocket数据
func (c *Client) StartReading() {
	go c.readLoop()
	go c.reconnectLoop()
	go c.pingLoop()
}

// readLoop 读取数据循环
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

			// 解析K线数据
			if err := c.parseKlineData(message); err != nil {
				zap.L().Warn("解析K线数据失败", zap.Error(err))
			}
		}
	}
}

// parseKlineData 解析K线数据
func (c *Client) parseKlineData(message []byte) error {
	var response OKXKlineResponse
	if err := json.Unmarshal(message, &response); err != nil {
		return err
	}

	// 检查是否是K线数据
	if !strings.HasPrefix(response.Arg.Channel, "candle") {
		return nil // 忽略非K线数据
	}

	// 解析每条K线数据
	for _, data := range response.Data {
		if len(data) < 7 {
			continue
		}

		kline, err := c.parseOKXKlineData(response.Arg.InstID, data, response.Arg.Channel)
		if err != nil {
			zap.L().Warn("解析单条K线数据失败", zap.Error(err))
			continue
		}

		// 发送到处理通道
		select {
		case c.klineChan <- kline:
		default:
			zap.L().Warn("K线数据通道满，丢弃数据", zap.String("symbol", kline.Symbol))
		}
	}

	return nil
}

// parseOKXKlineData 解析OKX K线数据格式
func (c *Client) parseOKXKlineData(symbol string, data []string, channel string) (*types.KLine, error) {
	if len(data) < 7 {
		return nil, fmt.Errorf("K线数据格式不正确")
	}

	// OKX K线数据格式: [timestamp, open, high, low, close, volume, volumeCcy]
	openTime, err := parseTimestamp(data[0])
	if err != nil {
		return nil, fmt.Errorf("解析开盘时间失败: %v", err)
	}

	open, err := parseFloat(data[1])
	if err != nil {
		return nil, fmt.Errorf("解析开盘价失败: %v", err)
	}

	high, err := parseFloat(data[2])
	if err != nil {
		return nil, fmt.Errorf("解析最高价失败: %v", err)
	}

	low, err := parseFloat(data[3])
	if err != nil {
		return nil, fmt.Errorf("解析最低价失败: %v", err)
	}

	close, err := parseFloat(data[4])
	if err != nil {
		return nil, fmt.Errorf("解析收盘价失败: %v", err)
	}

	volume, err := parseFloat(data[5])
	if err != nil {
		return nil, fmt.Errorf("解析成交量失败: %v", err)
	}

	// 提取时间间隔
	interval := strings.TrimPrefix(channel, "candle")

	return &types.KLine{
		Symbol:    symbol,
		OpenTime:  openTime,
		CloseTime: openTime.Add(getIntervalDuration(interval)),
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
		Interval:  interval,
	}, nil
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

			zap.L().Info("尝试重连WebSocket",
				zap.Int("attempt", reconnectAttempts),
				zap.Int("max_attempts", c.config.MaxReconnectAttempts))

			if err := c.Connect(); err != nil {
				zap.L().Error("重连失败", zap.Error(err))
				time.Sleep(c.config.ReconnectInterval)
				c.reconnectChan <- struct{}{}
				continue
			}

			// 重连成功，重置重连次数
			reconnectAttempts = 0
			zap.L().Info("WebSocket重连成功")
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
