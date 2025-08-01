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
	dataFetcher    *fetcher.DataFetcher
	analysisEngine *analyzer.AnalysisEngine
	stateManager   *storage.StateManager
	fetchInterval  time.Duration
	analyzeInterval time.Duration
}

func NewScheduler(dataFetcher *fetcher.DataFetcher, analysisEngine *analyzer.AnalysisEngine, stateManager *storage.StateManager) *Scheduler {
	return &Scheduler{
		dataFetcher:     dataFetcher,
		analysisEngine:  analysisEngine,
		stateManager:    stateManager,
		fetchInterval:   1 * time.Minute,  // 每分钟获取数据
		analyzeInterval: 1 * time.Minute,  // 每分钟分析一次
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	fmt.Println("🚀 调度器启动中...")
	
	// 启动数据获取器
	go s.dataFetcher.Start(ctx)
	
	// 等待一些数据积累后再开始分析
	fmt.Println("⏳ 等待数据积累中，5分钟后开始价格分析...")
	
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Minute):
		fmt.Println("✅ 开始价格分析和预警监控")
	}
	
	// 启动分析任务
	analyzeTicker := time.NewTicker(s.analyzeInterval)
	defer analyzeTicker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			fmt.Println("📴 调度器已停止")
			return
		case <-analyzeTicker.C:
			s.runAnalysis()
		}
	}
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