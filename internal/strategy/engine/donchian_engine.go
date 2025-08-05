package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"okx-market-sentry/internal/strategy/database"
	"okx-market-sentry/internal/strategy/fetcher"
	"okx-market-sentry/internal/strategy/signals"
	"okx-market-sentry/internal/strategy/websocket"
	"okx-market-sentry/pkg/types"
)

// DonchianEngine 唐奇安通道策略引擎
type DonchianEngine struct {
	config         types.DonchianConfig
	wsClient       *websocket.Client
	signalDetector *signals.DonchianSignalDetector
	dbManager      *database.Manager
	historyFetcher *fetcher.HistoryKlineFetcher

	// 数据管道
	klineBuffer map[string][]*types.KLine
	bufferMutex sync.RWMutex

	// 处理通道
	klineChan  chan *types.KLine
	signalChan chan *types.TradingSignal

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 统计
	processedKlines int64
	detectedSignals int64
	statsMutex      sync.RWMutex
}

// NewDonchianEngine 创建唐奇安通道策略引擎
func NewDonchianEngine(config types.DonchianConfig, wsConfig types.WebSocketConfig, dbConfig types.MySQLConfig, proxy string) (*DonchianEngine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建WebSocket客户端
	wsClient := websocket.NewClient(wsConfig.OKXEndpoint, proxy, wsConfig)

	// 创建信号检测器
	signalDetector := signals.NewDonchianSignalDetector(config)

	// 创建数据库管理器
	dbManager, err := database.NewManager(dbConfig)
	if err != nil {
		cancel()
		return nil, err
	}

	// 创建历史数据获取器
	historyFetcher := fetcher.NewHistoryKlineFetcher(proxy, 30*time.Second)

	engine := &DonchianEngine{
		config:         config,
		wsClient:       wsClient,
		signalDetector: signalDetector,
		dbManager:      dbManager,
		historyFetcher: historyFetcher,
		klineBuffer:    make(map[string][]*types.KLine),
		klineChan:      make(chan *types.KLine, 10000), // 大缓冲区
		signalChan:     make(chan *types.TradingSignal, 1000),
		ctx:            ctx,
		cancel:         cancel,
	}

	return engine, nil
}

// Start 启动策略引擎
func (de *DonchianEngine) Start() error {
	if !de.config.Enabled {
		zap.L().Info("🚫 唐奇安通道策略未启用")
		return nil
	}

	zap.L().Info("🚀 启动唐奇安通道策略引擎",
		zap.Strings("symbols", de.config.Symbols),
		zap.String("interval", de.config.Interval))

	// 1. 初始化历史K线数据
	if err := de.initializeHistoryData(); err != nil {
		return fmt.Errorf("初始化历史数据失败: %v", err)
	}

	// 2. 连接WebSocket
	if err := de.wsClient.Connect(); err != nil {
		return err
	}

	// 3. 订阅K线数据
	if err := de.wsClient.Subscribe(de.config.Symbols, de.config.Interval); err != nil {
		return err
	}

	// 4. 启动各个处理协程
	de.startWorkers()

	zap.L().Info("✅ 唐奇安通道策略引擎启动成功")

	return nil
}

// startWorkers 启动工作协程
func (de *DonchianEngine) startWorkers() {
	// 启动WebSocket数据读取
	de.wsClient.StartReading()

	// 启动K线数据收集器
	de.wg.Add(1)
	go de.klineCollector()

	// 启动K线数据处理器池（多个worker）
	workerCount := 5
	for i := 0; i < workerCount; i++ {
		de.wg.Add(1)
		go de.klineProcessor(i)
	}

	// 启动信号处理器
	de.wg.Add(1)
	go de.signalProcessor()

	// 启动数据库持久化器
	de.wg.Add(1)
	go de.databasePersister()

	// 启动性能监控器
	de.wg.Add(1)
	go de.performanceMonitor()
}

// klineCollector K线数据收集器
func (de *DonchianEngine) klineCollector() {
	defer de.wg.Done()

	klineSource := de.wsClient.GetKlineChannel()

	for {
		select {
		case <-de.ctx.Done():
			return
		case kline := <-klineSource:
			if kline == nil {
				continue
			}

			// 发送到处理通道
			select {
			case de.klineChan <- kline:
			default:
				zap.L().Warn("K线处理通道满，丢弃数据",
					zap.String("symbol", kline.Symbol))
			}
		}
	}
}

// klineProcessor K线数据处理器
func (de *DonchianEngine) klineProcessor(workerID int) {
	defer de.wg.Done()

	zap.L().Debug("启动K线处理器", zap.Int("worker_id", workerID))

	for {
		select {
		case <-de.ctx.Done():
			return
		case kline := <-de.klineChan:
			if kline == nil {
				continue
			}

			de.processKline(kline, workerID)
		}
	}
}

// processKline 处理单个K线数据
func (de *DonchianEngine) processKline(kline *types.KLine, workerID int) {
	// 更新K线缓冲区
	de.updateKlineBuffer(kline)

	// 获取足够的历史数据
	klines := de.getKlineHistory(kline.Symbol)
	if len(klines) < de.getRequiredBars() {
		zap.L().Debug("历史数据不足，跳过分析",
			zap.String("symbol", kline.Symbol),
			zap.Int("available", len(klines)),
			zap.Int("required", de.getRequiredBars()))
		return
	}

	// 检测交易信号
	signal := de.signalDetector.DetectSignal(kline.Symbol, klines)
	if signal != nil {
		// 发送信号到信号处理通道
		select {
		case de.signalChan <- signal:
			de.incrementSignalCount()
			zap.L().Info("🎯 发现交易信号",
				zap.String("symbol", signal.Symbol),
				zap.String("type", signal.SignalType),
				zap.Float64("strength", signal.SignalStrength),
				zap.Int("worker_id", workerID))
		default:
			zap.L().Warn("信号处理通道满", zap.String("symbol", kline.Symbol))
		}
	}

	de.incrementKlineCount()
}

// updateKlineBuffer 更新K线缓冲区
func (de *DonchianEngine) updateKlineBuffer(kline *types.KLine) {
	de.bufferMutex.Lock()
	defer de.bufferMutex.Unlock()

	symbol := kline.Symbol

	// 初始化缓冲区
	if de.klineBuffer[symbol] == nil {
		de.klineBuffer[symbol] = make([]*types.KLine, 0)
	}

	// 添加新K线
	de.klineBuffer[symbol] = append(de.klineBuffer[symbol], kline)

	// 保持缓冲区大小（保留最近200根K线）
	maxBuffer := 200
	if len(de.klineBuffer[symbol]) > maxBuffer {
		de.klineBuffer[symbol] = de.klineBuffer[symbol][len(de.klineBuffer[symbol])-maxBuffer:]
	}
}

// getKlineHistory 获取K线历史数据
func (de *DonchianEngine) getKlineHistory(symbol string) []*types.KLine {
	de.bufferMutex.RLock()
	defer de.bufferMutex.RUnlock()

	klines := de.klineBuffer[symbol]
	if klines == nil {
		return nil
	}

	// 返回副本避免并发修改
	result := make([]*types.KLine, len(klines))
	copy(result, klines)

	return result
}

// signalProcessor 信号处理器
func (de *DonchianEngine) signalProcessor() {
	defer de.wg.Done()

	zap.L().Debug("启动信号处理器")

	for {
		select {
		case <-de.ctx.Done():
			return
		case signal := <-de.signalChan:
			if signal == nil {
				continue
			}

			de.processSignal(signal)
		}
	}
}

// processSignal 处理交易信号
func (de *DonchianEngine) processSignal(signal *types.TradingSignal) {
	zap.L().Info("📊 处理交易信号",
		zap.String("symbol", signal.Symbol),
		zap.String("type", signal.SignalType),
		zap.Float64("price", signal.Price),
		zap.Float64("strength", signal.SignalStrength))

	// 这里可以添加信号过滤、风险管理等逻辑

	// 保存信号到数据库（异步）
	go func() {
		if err := de.dbManager.SaveTradingSignal(signal); err != nil {
			zap.L().Error("保存交易信号失败",
				zap.Error(err),
				zap.String("symbol", signal.Symbol))
		}

		// 更新策略性能统计
		if err := de.dbManager.UpdateStrategyPerformance(signal.Symbol, signal.SignalType, signal.SignalStrength); err != nil {
			zap.L().Error("更新策略性能失败",
				zap.Error(err),
				zap.String("symbol", signal.Symbol))
		}
	}()
}

// databasePersister 数据库持久化器
func (de *DonchianEngine) databasePersister() {
	defer de.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // 每30秒持久化一次K线数据
	defer ticker.Stop()

	for {
		select {
		case <-de.ctx.Done():
			return
		case <-ticker.C:
			de.persistKlineData()
		}
	}
}

// persistKlineData 持久化K线数据
func (de *DonchianEngine) persistKlineData() {
	de.bufferMutex.RLock()

	// 获取需要持久化的K线数据
	var klinesToSave []*types.KLine
	for _, klines := range de.klineBuffer {
		if len(klines) > 0 {
			// 只保存最新的几根K线
			start := len(klines) - 5
			if start < 0 {
				start = 0
			}

			for _, kline := range klines[start:] {
				klinesToSave = append(klinesToSave, kline)
			}
		}
	}

	de.bufferMutex.RUnlock()

	// 异步保存
	go func() {
		for _, kline := range klinesToSave {
			if err := de.dbManager.SaveKLine(kline); err != nil {
				zap.L().Debug("保存K线数据失败",
					zap.Error(err),
					zap.String("symbol", kline.Symbol))
			}
		}
	}()
}

// performanceMonitor 性能监控器
func (de *DonchianEngine) performanceMonitor() {
	defer de.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-de.ctx.Done():
			return
		case <-ticker.C:
			de.logPerformanceStats()
		}
	}
}

// logPerformanceStats 记录性能统计
func (de *DonchianEngine) logPerformanceStats() {
	de.statsMutex.RLock()
	processedKlines := de.processedKlines
	detectedSignals := de.detectedSignals
	de.statsMutex.RUnlock()

	de.bufferMutex.RLock()
	bufferSizes := make(map[string]int)
	for symbol, klines := range de.klineBuffer {
		bufferSizes[symbol] = len(klines)
	}
	de.bufferMutex.RUnlock()

	zap.L().Info("📈 策略引擎性能统计",
		zap.Int64("processed_klines", processedKlines),
		zap.Int64("detected_signals", detectedSignals),
		zap.Any("buffer_sizes", bufferSizes),
		zap.Bool("ws_connected", de.wsClient.IsConnected()))
}

// getRequiredBars 获取所需的最小K线数量
func (de *DonchianEngine) getRequiredBars() int {
	return de.config.ConsolidationBars + de.config.DonchianLength + de.config.DonchianOffset + de.config.ATRLength + 45
}

// incrementKlineCount 增加K线计数
func (de *DonchianEngine) incrementKlineCount() {
	de.statsMutex.Lock()
	de.processedKlines++
	de.statsMutex.Unlock()
}

// incrementSignalCount 增加信号计数
func (de *DonchianEngine) incrementSignalCount() {
	de.statsMutex.Lock()
	de.detectedSignals++
	de.statsMutex.Unlock()
}

// GetStats 获取统计信息
func (de *DonchianEngine) GetStats() map[string]interface{} {
	de.statsMutex.RLock()
	defer de.statsMutex.RUnlock()

	de.bufferMutex.RLock()
	defer de.bufferMutex.RUnlock()

	bufferSizes := make(map[string]int)
	for symbol, klines := range de.klineBuffer {
		bufferSizes[symbol] = len(klines)
	}

	return map[string]interface{}{
		"processed_klines": de.processedKlines,
		"detected_signals": de.detectedSignals,
		"buffer_sizes":     bufferSizes,
		"ws_connected":     de.wsClient.IsConnected(),
		"enabled":          de.config.Enabled,
		"symbols":          de.config.Symbols,
		"interval":         de.config.Interval,
	}
}

// GetDatabaseManager 获取数据库管理器
func (de *DonchianEngine) GetDatabaseManager() *database.Manager {
	return de.dbManager
}

// Stop 停止策略引擎
func (de *DonchianEngine) Stop() error {
	zap.L().Info("🛑 停止唐奇安通道策略引擎")

	// 取消上下文
	de.cancel()

	// 关闭WebSocket连接
	if err := de.wsClient.Close(); err != nil {
		zap.L().Error("关闭WebSocket连接失败", zap.Error(err))
	}

	// 等待所有协程结束
	done := make(chan struct{})
	go func() {
		de.wg.Wait()
		close(done)
	}()

	// 设置超时
	select {
	case <-done:
		zap.L().Info("✅ 所有工作协程已停止")
	case <-time.After(30 * time.Second):
		zap.L().Warn("⚠️ 停止超时，强制退出")
	}

	// 关闭数据库连接
	if err := de.dbManager.Close(); err != nil {
		zap.L().Error("关闭数据库连接失败", zap.Error(err))
	}

	zap.L().Info("✅ 唐奇安通道策略引擎已停止")

	return nil
}

// initializeHistoryData 初始化历史K线数据
func (de *DonchianEngine) initializeHistoryData() error {
	zap.L().Info("📚 开始初始化历史K线数据",
		zap.Int("consolidation_bars", de.config.ConsolidationBars),
		zap.Strings("symbols", de.config.Symbols))

	// 计算需要获取的历史K线数量（考虑ATR和Donchian通道计算需要）
	historyLimit := de.config.ConsolidationBars + de.config.DonchianLength + de.config.ATRLength + 10 // 额外10根作为缓冲

	// 批量获取历史数据
	historyData, err := de.historyFetcher.FetchMultipleSymbolsHistory(
		de.config.Symbols,
		de.config.Interval,
		historyLimit,
	)
	if err != nil {
		return fmt.Errorf("获取历史数据失败: %v", err)
	}

	// 初始化K线缓冲区并存储到数据库
	de.bufferMutex.Lock()
	totalKlines := 0
	for symbol, klines := range historyData {
		if len(klines) == 0 {
			zap.L().Warn("⚠️ 历史数据为空",
				zap.String("symbol", symbol))
			continue
		}

		// 存储到内存缓冲区
		de.klineBuffer[symbol] = klines
		totalKlines += len(klines)

		// 批量存储到数据库
		if err := de.batchSaveKlines(klines); err != nil {
			zap.L().Error("批量保存历史K线失败",
				zap.String("symbol", symbol),
				zap.Error(err))
		}

		zap.L().Info("✅ 历史数据初始化完成",
			zap.String("symbol", symbol),
			zap.Int("klines_count", len(klines)),
			zap.Time("oldest", klines[0].OpenTime),
			zap.Time("newest", klines[len(klines)-1].OpenTime))
	}
	de.bufferMutex.Unlock()

	zap.L().Info("🎉 所有历史K线数据初始化完成",
		zap.Int("symbols_count", len(historyData)),
		zap.Int("total_klines", totalKlines))

	return nil
}

// batchSaveKlines 批量保存K线数据到数据库
func (de *DonchianEngine) batchSaveKlines(klines []*types.KLine) error {
	if len(klines) == 0 {
		return nil
	}

	// 分批处理，每批100条
	batchSize := 100
	for i := 0; i < len(klines); i += batchSize {
		end := i + batchSize
		if end > len(klines) {
			end = len(klines)
		}

		batch := klines[i:end]
		if err := de.dbManager.BatchSaveKlines(batch); err != nil {
			return fmt.Errorf("批量保存K线失败 (batch %d-%d): %v", i, end-1, err)
		}
	}

	return nil
}
