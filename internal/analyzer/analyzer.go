package analyzer

import (
	"fmt"
	"sync"
	"time"

	"okx-market-sentry/internal/notifier"
	"okx-market-sentry/internal/storage"
	"okx-market-sentry/pkg/types"
)

// AnalysisEngine 分析引擎
type AnalysisEngine struct {
	stateManager *storage.StateManager
	notifier     notifier.Interface
	threshold    float64
	alertHistory map[string]time.Time // 防止重复预警
	mutex        sync.RWMutex
}

func NewAnalysisEngine(stateManager *storage.StateManager, notifyService notifier.Interface, threshold float64) *AnalysisEngine {
	return &AnalysisEngine{
		stateManager: stateManager,
		notifier:     notifyService,
		threshold:    threshold,
		alertHistory: make(map[string]time.Time),
	}
}

// AnalyzeAll 分析所有交易对的价格变化
func (ae *AnalysisEngine) AnalyzeAll() {
	symbols := ae.stateManager.GetAllSymbols()
	if len(symbols) == 0 {
		return
	}

	fmt.Printf("开始分析 %d 个交易对的价格变化...\n", len(symbols))

	// 并发分析各个交易对
	var wg sync.WaitGroup
	alertCount := 0
	var alertMutex sync.Mutex

	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			if ae.analyzeSymbol(sym) {
				alertMutex.Lock()
				alertCount++
				alertMutex.Unlock()
			}
		}(symbol)
	}
	wg.Wait()

	if alertCount > 0 {
		fmt.Printf("✅ 分析完成，触发 %d 个预警\n", alertCount)
	} else {
		fmt.Printf("✅ 分析完成，暂无异常波动\n")
	}
}

// analyzeSymbol 分析单个交易对
func (ae *AnalysisEngine) analyzeSymbol(symbol string) bool {
	// 获取价格数据
	current, past := ae.stateManager.GetPriceData(symbol)
	if current == nil || past == nil {
		return false // 数据不足，跳过分析
	}

	// 计算涨幅
	changePercent := ((current.Price - past.Price) / past.Price) * 100

	// 检查是否超过阈值（正负都检查）
	absChange := changePercent
	if absChange < 0 {
		absChange = -absChange
	}

	if absChange > ae.threshold {
		// 检查是否在短时间内已经预警过（避免重复预警）
		if ae.shouldAlert(symbol) {
			alert := &types.AlertData{
				Symbol:        symbol,
				CurrentPrice:  current.Price,
				PastPrice:     past.Price,
				ChangePercent: changePercent,
				AlertTime:     time.Now(),
			}

			// 发送预警
			err := ae.notifier.SendAlert(alert)
			if err != nil {
				fmt.Printf("❌ 发送预警失败: %s - %v\n", symbol, err)
				return false
			}

			// 记录预警历史
			ae.recordAlert(symbol)
			return true
		}
	}

	return false
}

// shouldAlert 检查是否应该发送预警（防止短时间内重复预警）
func (ae *AnalysisEngine) shouldAlert(symbol string) bool {
	ae.mutex.RLock()
	defer ae.mutex.RUnlock()

	lastAlert, exists := ae.alertHistory[symbol]
	if !exists {
		return true
	}

	// 如果距离上次预警超过5分钟，则可以再次预警
	return time.Since(lastAlert) > 5*time.Minute
}

// recordAlert 记录预警历史
func (ae *AnalysisEngine) recordAlert(symbol string) {
	ae.mutex.Lock()
	defer ae.mutex.Unlock()

	ae.alertHistory[symbol] = time.Now()

	// 清理超过1小时的预警历史
	cutoff := time.Now().Add(-1 * time.Hour)
	for sym, alertTime := range ae.alertHistory {
		if alertTime.Before(cutoff) {
			delete(ae.alertHistory, sym)
		}
	}
}
