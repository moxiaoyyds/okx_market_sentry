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

	// 并发分析各个交易对，收集预警
	var wg sync.WaitGroup
	var alertMutex sync.Mutex
	alerts := make([]*types.AlertData, 0)

	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			if alert := ae.analyzeSymbol(sym); alert != nil {
				alertMutex.Lock()
				alerts = append(alerts, alert)
				alertMutex.Unlock()
			}
		}(symbol)
	}
	wg.Wait()

	// 批量发送预警
	if len(alerts) > 0 {
		ae.sendBatchAlerts(alerts)
		fmt.Printf("✅ 分析完成，触发 %d 个预警\n", len(alerts))
	} else {
		fmt.Printf("✅ 分析完成，暂无异常波动\n")
	}
}

// analyzeSymbol 分析单个交易对，返回预警数据或nil
func (ae *AnalysisEngine) analyzeSymbol(symbol string) *types.AlertData {
	// 获取价格数据
	current, past := ae.stateManager.GetPriceData(symbol)
	if current == nil || past == nil {
		return nil // 数据不足，跳过分析
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

			// 记录预警历史
			ae.recordAlert(symbol)
			return alert
		}
	}

	return nil
}

// sendBatchAlerts 批量发送预警
func (ae *AnalysisEngine) sendBatchAlerts(alerts []*types.AlertData) {
	if len(alerts) == 0 {
		return
	}

	// 如果只有一个预警，使用单个发送
	if len(alerts) == 1 {
		err := ae.notifier.SendAlert(alerts[0])
		if err != nil {
			fmt.Printf("❌ 发送预警失败: %s - %v\n", alerts[0].Symbol, err)
		}
		return
	}

	// 批量发送多个预警
	err := ae.notifier.SendBatchAlerts(alerts)
	if err != nil {
		fmt.Printf("❌ 批量发送预警失败: %v\n", err)
		// 降级为单个发送
		for _, alert := range alerts {
			if singleErr := ae.notifier.SendAlert(alert); singleErr != nil {
				fmt.Printf("❌ 单个预警发送失败: %s - %v\n", alert.Symbol, singleErr)
			}
		}
	}
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
