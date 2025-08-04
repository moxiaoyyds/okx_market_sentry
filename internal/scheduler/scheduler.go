package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"
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
	zap.L().Info("🚀 调度器启动中...")

	// 启动数据获取器
	go s.dataFetcher.Start(ctx)

	// 计算下一个K线对齐的时间点
	nextKlineTime := s.calculateNextKlineTime()
	waitDuration := time.Until(nextKlineTime)

	zap.L().Info("⏳ 等待同步到下一个K线时间点",
		zap.String("next_time", nextKlineTime.Format("15:04:05")),
		zap.Duration("wait_duration", waitDuration))

	select {
	case <-ctx.Done():
		return
	case <-time.After(waitDuration):
		zap.L().Info("✅ 已同步到K线时间，开始价格分析和预警监控",
			zap.String("sync_time", time.Now().Format("15:04:05")))
	}

	// 创建对齐到K线时间的定时器
	s.startKlineAlignedAnalysis(ctx)
}

func (s *Scheduler) runAnalysis() {
	zap.L().Info("--- 价格分析任务开始 ---",
		zap.String("time", time.Now().Format("15:04:05")))

	// 显示存储状态
	stats := s.stateManager.GetRedisStats()
	if stats["redis_enabled"].(bool) {
		if redisKeys, ok := stats["redis_keys"]; ok {
			zap.L().Info("📊 存储状态",
				zap.Int("memory_symbols", stats["memory_symbols"].(int)),
				zap.Int("redis_keys", redisKeys.(int)))
		} else {
			zap.L().Info("📊 存储状态",
				zap.Int("memory_symbols", stats["memory_symbols"].(int)),
				zap.String("redis_status", "已连接但获取key数失败"))
		}
	} else {
		zap.L().Info("📊 存储状态",
			zap.Int("memory_symbols", stats["memory_symbols"].(int)),
			zap.String("redis_status", "未启用"))
	}

	s.analysisEngine.AnalyzeAll()
	zap.L().Info("--- 分析任务完成 ---")
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
			zap.L().Info("📴 调度器已停止")
			return
		default:
			// 运行分析
			s.runAnalysis()

			// 计算下一次分析时间（下一个K线时间点）
			nextAnalysisTime := s.calculateNextKlineTime()
			waitDuration := time.Until(nextAnalysisTime)

			zap.L().Info("⏰ 下次分析时间",
				zap.String("next_time", nextAnalysisTime.Format("15:04:05")),
				zap.Duration("wait_duration", waitDuration))

			// 等待到下一个K线时间点
			select {
			case <-ctx.Done():
				zap.L().Info("📴 调度器已停止")
				return
			case <-time.After(waitDuration):
				// 继续下一轮分析
				continue
			}
		}
	}
}
