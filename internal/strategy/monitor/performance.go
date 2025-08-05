package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"okx-market-sentry/internal/strategy/database"
	"okx-market-sentry/internal/strategy/engine"
	"okx-market-sentry/pkg/types"
)

// PerformanceMonitor 策略性能监控器
type PerformanceMonitor struct {
	dbManager *database.Manager
	engine    *engine.DonchianEngine
	config    types.DonchianConfig
	
	ctx       context.Context
	cancel    context.CancelFunc
	
	// 性能指标
	metrics   *PerformanceMetrics
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	StartTime          time.Time            `json:"start_time"`
	TotalSignals       int64                `json:"total_signals"`
	LongSignals        int64                `json:"long_signals"`
	ShortSignals       int64                `json:"short_signals"`
	ProcessedKlines    int64                `json:"processed_klines"`
	AvgSignalStrength  float64              `json:"avg_signal_strength"`
	SignalFrequency    float64              `json:"signal_frequency"` // 信号/小时
	SymbolStats        map[string]*SymbolMetrics `json:"symbol_stats"`
	LastUpdateTime     time.Time            `json:"last_update_time"`
}

// SymbolMetrics 单个交易对的性能指标
type SymbolMetrics struct {
	Symbol            string    `json:"symbol"`
	TotalSignals      int       `json:"total_signals"`
	LongSignals       int       `json:"long_signals"`
	ShortSignals      int       `json:"short_signals"`
	AvgSignalStrength float64   `json:"avg_signal_strength"`
	LastSignalTime    time.Time `json:"last_signal_time"`
	LastSignalType    string    `json:"last_signal_type"`
	LastSignalPrice   float64   `json:"last_signal_price"`
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor(dbManager *database.Manager, engine *engine.DonchianEngine, config types.DonchianConfig) *PerformanceMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &PerformanceMonitor{
		dbManager: dbManager,
		engine:    engine,
		config:    config,
		ctx:       ctx,
		cancel:    cancel,
		metrics: &PerformanceMetrics{
			StartTime:   time.Now(),
			SymbolStats: make(map[string]*SymbolMetrics),
		},
	}
}

// Start 启动性能监控
func (pm *PerformanceMonitor) Start() {
	if !pm.config.Enabled {
		return
	}
	
	zap.L().Info("📊 启动策略性能监控器")
	
	// 初始化交易对指标
	for _, symbol := range pm.config.Symbols {
		pm.metrics.SymbolStats[symbol] = &SymbolMetrics{
			Symbol: symbol,
		}
	}
	
	// 启动监控协程
	go pm.monitorLoop()
	go pm.reportLoop()
}

// monitorLoop 监控循环
func (pm *PerformanceMonitor) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.updateMetrics()
		}
	}
}

// reportLoop 报告循环
func (pm *PerformanceMonitor) reportLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.generateReport()
		}
	}
}

// updateMetrics 更新性能指标
func (pm *PerformanceMonitor) updateMetrics() {
	// 获取引擎统计数据
	engineStats := pm.engine.GetStats()
	
	// 更新基础指标
	if processedKlines, ok := engineStats["processed_klines"].(int64); ok {
		pm.metrics.ProcessedKlines = processedKlines
	}
	
	if detectedSignals, ok := engineStats["detected_signals"].(int64); ok {
		pm.metrics.TotalSignals = detectedSignals
	}
	
	// 计算信号频率（信号/小时）
	runTime := time.Since(pm.metrics.StartTime).Hours()
	if runTime > 0 {
		pm.metrics.SignalFrequency = float64(pm.metrics.TotalSignals) / runTime
	}
	
	// 更新各交易对的详细统计
	pm.updateSymbolMetrics()
	
	pm.metrics.LastUpdateTime = time.Now()
}

// updateSymbolMetrics 更新交易对指标
func (pm *PerformanceMonitor) updateSymbolMetrics() {
	// 检查数据库管理器是否可用
	if pm.dbManager == nil {
		zap.L().Debug("数据库管理器未初始化，跳过符号指标更新")
		return
	}

	for _, symbol := range pm.config.Symbols {
		// 从数据库获取最近的信号数据
		signals, err := pm.dbManager.GetTradingSignals(symbol, 100)
		if err != nil {
			zap.L().Warn("获取交易信号失败", 
				zap.String("symbol", symbol),
				zap.Error(err))
			continue
		}
		
		if len(signals) == 0 {
			continue
		}
		
		symbolMetrics := pm.metrics.SymbolStats[symbol]
		if symbolMetrics == nil {
			symbolMetrics = &SymbolMetrics{Symbol: symbol}
			pm.metrics.SymbolStats[symbol] = symbolMetrics
		}
		
		// 统计信号数量和类型
		symbolMetrics.TotalSignals = len(signals)
		symbolMetrics.LongSignals = 0
		symbolMetrics.ShortSignals = 0
		
		var totalStrength float64
		strengthCount := 0
		
		for _, signal := range signals {
			if signal.SignalType == "LONG" {
				symbolMetrics.LongSignals++
			} else if signal.SignalType == "SHORT" {
				symbolMetrics.ShortSignals++
			}
			
			if signal.SignalStrength != nil {
				totalStrength += *signal.SignalStrength
				strengthCount++
			}
		}
		
		// 计算平均信号强度
		if strengthCount > 0 {
			symbolMetrics.AvgSignalStrength = totalStrength / float64(strengthCount)
		}
		
		// 更新最新信号信息
		if len(signals) > 0 {
			latest := signals[0] // 按时间倒序排列，第一个是最新的
			symbolMetrics.LastSignalTime = time.Unix(latest.SignalTime, 0)
			symbolMetrics.LastSignalType = latest.SignalType
			symbolMetrics.LastSignalPrice = latest.Price
		}
		
		// 更新全局统计
		pm.metrics.LongSignals += int64(symbolMetrics.LongSignals)
		pm.metrics.ShortSignals += int64(symbolMetrics.ShortSignals)
	}
	
	// 计算全局平均信号强度
	if pm.metrics.TotalSignals > 0 {
		totalStrength := 0.0
		count := 0
		
		for _, symbolMetrics := range pm.metrics.SymbolStats {
			if symbolMetrics.AvgSignalStrength > 0 {
				totalStrength += symbolMetrics.AvgSignalStrength * float64(symbolMetrics.TotalSignals)
				count += symbolMetrics.TotalSignals
			}
		}
		
		if count > 0 {
			pm.metrics.AvgSignalStrength = totalStrength / float64(count)
		}
	}
}

// generateReport 生成性能报告
func (pm *PerformanceMonitor) generateReport() {
	runTime := time.Since(pm.metrics.StartTime)
	
	zap.L().Info("📈 策略性能报告",
		zap.Duration("run_time", runTime),
		zap.Int64("total_signals", pm.metrics.TotalSignals),
		zap.Int64("long_signals", pm.metrics.LongSignals),
		zap.Int64("short_signals", pm.metrics.ShortSignals),
		zap.Float64("avg_signal_strength", pm.metrics.AvgSignalStrength),
		zap.Float64("signal_frequency", pm.metrics.SignalFrequency),
		zap.Int64("processed_klines", pm.metrics.ProcessedKlines))
	
	// 输出各交易对的详细报告
	for symbol, metrics := range pm.metrics.SymbolStats {
		if metrics.TotalSignals > 0 {
			zap.L().Info("📊 交易对性能",
				zap.String("symbol", symbol),
				zap.Int("total_signals", metrics.TotalSignals),
				zap.Int("long_signals", metrics.LongSignals),
				zap.Int("short_signals", metrics.ShortSignals),
				zap.Float64("avg_strength", metrics.AvgSignalStrength),
				zap.Time("last_signal_time", metrics.LastSignalTime),
				zap.String("last_signal_type", metrics.LastSignalType),
				zap.Float64("last_signal_price", metrics.LastSignalPrice))
		}
	}
}

// GetMetrics 获取当前性能指标
func (pm *PerformanceMonitor) GetMetrics() *PerformanceMetrics {
	pm.updateMetrics()
	return pm.metrics
}

// GetMetricsJSON 获取JSON格式的性能指标
func (pm *PerformanceMonitor) GetMetricsJSON() (string, error) {
	metrics := pm.GetMetrics()
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetDailyReport 获取日报告
func (pm *PerformanceMonitor) GetDailyReport(symbol string) (*DailyReport, error) {
	// 获取今日性能数据
	performances, err := pm.dbManager.GetStrategyPerformance(symbol, 1)
	if err != nil {
		return nil, err
	}
	
	if len(performances) == 0 {
		return &DailyReport{
			Symbol: symbol,
			Date:   time.Now().Truncate(24 * time.Hour),
		}, nil
	}
	
	perf := performances[0]
	
	report := &DailyReport{
		Symbol:            symbol,
		Date:              perf.Date,
		TotalSignals:      perf.TotalSignals,
		LongSignals:       perf.LongSignals,
		ShortSignals:      perf.ShortSignals,
		AvgSignalStrength: 0,
	}
	
	if perf.AvgSignalStrength != nil {
		report.AvgSignalStrength = *perf.AvgSignalStrength
	}
	
	// 计算成功率等其他指标
	if report.TotalSignals > 0 {
		report.LongRatio = float64(report.LongSignals) / float64(report.TotalSignals) * 100
		report.ShortRatio = float64(report.ShortSignals) / float64(report.TotalSignals) * 100
	}
	
	return report, nil
}

// DailyReport 日报告
type DailyReport struct {
	Symbol            string    `json:"symbol"`
	Date              time.Time `json:"date"`
	TotalSignals      int       `json:"total_signals"`
	LongSignals       int       `json:"long_signals"`
	ShortSignals      int       `json:"short_signals"`
	AvgSignalStrength float64   `json:"avg_signal_strength"`
	LongRatio         float64   `json:"long_ratio"`
	ShortRatio        float64   `json:"short_ratio"`
}

// PrintFormattedReport 打印格式化报告
func (pm *PerformanceMonitor) PrintFormattedReport() {
	metrics := pm.GetMetrics()
	runTime := time.Since(metrics.StartTime)
	
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📈 唐奇安通道策略性能报告")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("🕐 运行时间: %s\n", runTime.Truncate(time.Second))
	fmt.Printf("📊 处理K线: %d\n", metrics.ProcessedKlines)
	fmt.Printf("🎯 总信号数: %d\n", metrics.TotalSignals)
	fmt.Printf("📈 做多信号: %d\n", metrics.LongSignals)
	fmt.Printf("📉 做空信号: %d\n", metrics.ShortSignals)
	fmt.Printf("⭐ 平均强度: %.2f\n", metrics.AvgSignalStrength)
	fmt.Printf("🔄 信号频率: %.2f信号/小时\n", metrics.SignalFrequency)
	fmt.Println(strings.Repeat("-", 80))
	
	// 交易对详细信息
	for symbol, symbolMetrics := range metrics.SymbolStats {
		if symbolMetrics.TotalSignals > 0 {
			fmt.Printf("💹 %s: %d信号 (%.2f强度) 最近: %s\n",
				symbol,
				symbolMetrics.TotalSignals,
				symbolMetrics.AvgSignalStrength,
				symbolMetrics.LastSignalTime.Format("01-02 15:04"))
		}
	}
	
	fmt.Println(strings.Repeat("=", 80) + "\n")
}

// Stop 停止性能监控
func (pm *PerformanceMonitor) Stop() {
	zap.L().Info("🛑 停止策略性能监控器")
	pm.cancel()
}