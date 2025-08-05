package main

import (
	"log"

	"okx-market-sentry/pkg/config"
	"okx-market-sentry/pkg/logger"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("加载配置失败:", err)
	}

	// 初始化zap日志系统
	logger.InitLogger(cfg.Log)

	// 创建应用程序实例
	app := NewApp(cfg)

	// 启动应用程序
	app.Start()

	// 等待关闭信号
	app.WaitForShutdown()

	// 停止应用程序
	app.Stop()
}
