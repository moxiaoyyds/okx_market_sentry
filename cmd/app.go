package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"okx-market-sentry/internal/analyzer"
	"okx-market-sentry/internal/fetcher"
	"okx-market-sentry/internal/notifier"
	"okx-market-sentry/internal/scheduler"
	"okx-market-sentry/internal/storage"
	"okx-market-sentry/internal/strategy/engine"
	"okx-market-sentry/internal/strategy/monitor"
	"okx-market-sentry/pkg/types"
)

// App 应用程序管理器
type App struct {
	config *types.Config
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewApp 创建应用程序实例
func NewApp(config *types.Config) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动应用程序
func (app *App) Start() {
	zap.L().Info("🚀 OKX Market Sentry 启动中...")

	// 启动原有的价格监控系统（如果需要）
	if app.config.Alert.Threshold > 0 {
		app.wg.Add(1)
		go func() {
			defer app.wg.Done()
			app.startLegacySystem()
		}()
	}

	// 启动唐奇安通道策略引擎
	if app.config.Strategy.Donchian.Enabled {
		app.wg.Add(1)
		go func() {
			defer app.wg.Done()
			app.startDonchianStrategy()
		}()
	}

	zap.L().Info("✅ OKX Market Sentry 已启动")
}

// Stop 停止应用程序
func (app *App) Stop() {
	zap.L().Info("🛑 收到停止信号，正在优雅关闭...")
	app.cancel()

	// 等待所有goroutine结束，最多等待30秒
	done := make(chan struct{})
	go func() {
		app.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		zap.L().Info("✅ OKX Market Sentry 已安全关闭")
	case <-time.After(30 * time.Second):
		zap.L().Warn("⚠️ 强制关闭超时")
	}
}

// WaitForShutdown 等待关闭信号
func (app *App) WaitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

// startLegacySystem 启动原有的价格监控系统
func (app *App) startLegacySystem() {
	zap.L().Info("📊 启动原有价格监控系统")

	// 初始化各模块
	stateManager := storage.NewStateManager(app.config.Redis, app.config.Alert.MonitorPeriod)
	dataFetcher := fetcher.NewDataFetcher(stateManager, app.config.Network)

	// 根据配置选择通知服务（优先级：钉钉 > PushPlus > 控制台）
	var notifyService notifier.Interface
	if app.config.DingTalk.WebhookURL != "" {
		notifyService = notifier.NewDingTalkNotifier(app.config.DingTalk.WebhookURL, app.config.DingTalk.Secret)
	} else if app.config.PushPlus.UserToken != "" {
		notifyService = notifier.NewPushPlusNotifier(app.config.PushPlus.UserToken, app.config.PushPlus.To)
	} else {
		notifyService = notifier.NewConsoleNotifier()
	}

	analysisEngine := analyzer.NewAnalysisEngine(stateManager, notifyService, app.config.Alert.Threshold, app.config.Alert.MonitorPeriod)
	taskScheduler := scheduler.NewScheduler(dataFetcher, analysisEngine, stateManager, app.config.Alert.MonitorPeriod)

	// 启动调度器
	taskScheduler.Start(app.ctx)
}

// startDonchianStrategy 启动唐奇安通道策略引擎
func (app *App) startDonchianStrategy() {
	zap.L().Info("📈 启动唐奇安通道策略引擎")

	// 创建WebSocket配置
	wsConfig := types.WebSocketConfig{
		OKXEndpoint:          "wss://ws.okx.com:8443/ws/v5/public",
		ReconnectInterval:    5 * time.Second,
		PingInterval:         20 * time.Second,
		MaxReconnectAttempts: 10,
	}

	// 创建策略引擎
	strategyEngine, err := engine.NewDonchianEngine(
		app.config.Strategy.Donchian,
		wsConfig,
		app.config.Database.MySQL,
		app.config.Network.Proxy,
	)
	if err != nil {
		zap.L().Error("❌ 创建唐奇安策略引擎失败", zap.Error(err))
		return
	}

	// 启动策略引擎
	if err := strategyEngine.Start(); err != nil {
		zap.L().Error("❌ 启动唐奇安策略引擎失败", zap.Error(err))
		return
	}

	// 创建性能监控器
	performanceMonitor := monitor.NewPerformanceMonitor(strategyEngine.GetDatabaseManager(), strategyEngine, app.config.Strategy.Donchian)
	performanceMonitor.Start()

	// 等待上下文取消
	<-app.ctx.Done()

	zap.L().Info("🛑 停止唐奇安通道策略引擎")

	// 停止性能监控
	performanceMonitor.Stop()

	// 停止策略引擎
	if err := strategyEngine.Stop(); err != nil {
		zap.L().Error("❌ 停止策略引擎失败", zap.Error(err))
	}
}
