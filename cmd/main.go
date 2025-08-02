package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"okx-market-sentry/internal/analyzer"
	"okx-market-sentry/internal/fetcher"
	"okx-market-sentry/internal/notifier"
	"okx-market-sentry/internal/scheduler"
	"okx-market-sentry/internal/storage"
	"okx-market-sentry/pkg/config"
	"okx-market-sentry/pkg/logger"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("加载配置失败:", err)
	}

	// 初始化日志
	appLogger := logger.New(cfg.LogLevel)
	appLogger.Info("OKX Market Sentry 启动中...")

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化各模块
	stateManager := storage.NewStateManager(cfg.Redis, cfg.Alert.MonitorPeriod)
	dataFetcher := fetcher.NewDataFetcher(stateManager, cfg.Network)

	// 根据配置选择通知服务（优先级：钉钉 > PushPlus > 控制台）
	var notifyService notifier.Interface
	if cfg.DingTalk.WebhookURL != "" {
		notifyService = notifier.NewDingTalkNotifier(cfg.DingTalk.WebhookURL, cfg.DingTalk.Secret)
	} else if cfg.PushPlus.UserToken != "" {
		notifyService = notifier.NewPushPlusNotifier(cfg.PushPlus.UserToken, cfg.PushPlus.To)
	} else {
		notifyService = notifier.NewConsoleNotifier()
	}

	analysisEngine := analyzer.NewAnalysisEngine(stateManager, notifyService, cfg.Alert.Threshold, cfg.Alert.MonitorPeriod)
	taskScheduler := scheduler.NewScheduler(dataFetcher, analysisEngine, stateManager, cfg.Alert.MonitorPeriod)

	// 启动服务
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		taskScheduler.Start(ctx)
	}()

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	appLogger.Info("OKX Market Sentry 已启动")
	<-sigCh

	appLogger.Info("收到停止信号，正在优雅关闭...")
	cancel()

	// 等待所有goroutine结束，最多等待30秒
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		appLogger.Info("OKX Market Sentry 已安全关闭")
	case <-time.After(30 * time.Second):
		appLogger.Warn("强制关闭超时")
	}
}
