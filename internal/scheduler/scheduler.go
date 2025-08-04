package scheduler

import (
	"context"
	"fmt"
	"time"

	"okx-market-sentry/internal/analyzer"
	"okx-market-sentry/internal/fetcher"
	"okx-market-sentry/internal/storage"
)

// Scheduler 调度器
type Scheduler struct {
	dataFetcher     *fetcher.DataFetcher
	analysisEngine  *analyzer.AnalysisEngine
	stateManager    *storage.StateManager
	fetchInterval   time.Duration
	analyzeInterval time.Duration
	monitorPeriod   time.Duration // 监控周期
}

func NewScheduler(dataFetcher *fetcher.DataFetcher, analysisEngine *analyzer.AnalysisEngine, stateManager *storage.StateManager, monitorPeriod time.Duration) *Scheduler {
	return &Scheduler{
		dataFetcher:     dataFetcher,
		analysisEngine:  analysisEngine,
		stateManager:    stateManager,
		fetchInterval:   1 * time.Minute, // 每分钟获取数据
		analyzeInterval: 1 * time.Minute, // 每分钟分析一次
		monitorPeriod:   monitorPeriod,   // 监控周期
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	fmt.Println("🚀 调度器启动中...")

	// 启动数据获取器
	go s.dataFetcher.Start(ctx)

	// 计算下一个K线对齐的时间点
	nextKlineTime := s.calculateNextKlineTime()
	waitDuration := time.Until(nextKlineTime)

	fmt.Printf("⏳ 等待同步到下一个K线时间点 %s（等待 %v）...\n",
		nextKlineTime.Format("15:04:05"), waitDuration)

	select {
	case <-ctx.Done():
		return
	case <-time.After(waitDuration):
		fmt.Printf("✅ 已同步到K线时间 %s，开始价格分析和预警监控\n",
			time.Now().Format("15:04:05"))
	}

	// 创建对齐到K线时间的定时器
	s.startKlineAlignedAnalysis(ctx)
}

func (s *Scheduler) runAnalysis() {
	fmt.Printf("\n--- 价格分析任务 [%s] ---\n", time.Now().Format("15:04:05"))

	// 显示存储状态
	stats := s.stateManager.GetRedisStats()
	fmt.Printf("📊 存储状态: 内存中%d个交易对", stats["memory_symbols"])
	if stats["redis_enabled"].(bool) {
		if redisKeys, ok := stats["redis_keys"]; ok {
			fmt.Printf(", Redis中%d个key", redisKeys)
		}
	} else {
		fmt.Printf(", Redis未启用")
	}
	fmt.Println()

	s.analysisEngine.AnalyzeAll()
	fmt.Println("--- 分析任务完成 ---")
}

// calculateNextKlineTime 计算下一个K线对齐的时间点
func (s *Scheduler) calculateNextKlineTime() time.Time {
	now := time.Now()

	// 获取监控周期的分钟数
	periodMinutes := int(s.monitorPeriod.Minutes())

	// 计算当前小时内的分钟数，向上取整到下一个周期倍数
	currentMinute := now.Minute()
	nextAlignedMinute := ((currentMinute / periodMinutes) + 1) * periodMinutes

	// 如果超过60分钟，进入下一小时
	if nextAlignedMinute >= 60 {
		// 进入下一小时的对齐时间点
		nextHour := now.Hour() + 1
		nextAlignedMinute = 0
		return time.Date(now.Year(), now.Month(), now.Day(), nextHour, nextAlignedMinute, 0, 0, now.Location())
	}

	// 同一小时内的对齐时间点
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), nextAlignedMinute, 0, 0, now.Location())
}

// startKlineAlignedAnalysis 启动对齐到K线时间的分析任务
func (s *Scheduler) startKlineAlignedAnalysis(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("📴 调度器已停止")
			return
		default:
			// 运行分析
			s.runAnalysis()

			// 计算下一次分析时间（下一个K线时间点）
			nextAnalysisTime := s.calculateNextKlineTime()
			waitDuration := time.Until(nextAnalysisTime)

			fmt.Printf("⏰ 下次分析时间: %s（等待 %v）\n",
				nextAnalysisTime.Format("15:04:05"), waitDuration)

			// 等待到下一个K线时间点
			select {
			case <-ctx.Done():
				fmt.Println("📴 调度器已停止")
				return
			case <-time.After(waitDuration):
				// 继续下一轮分析
				continue
			}
		}
	}
}
