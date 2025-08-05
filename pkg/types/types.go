package types

// 此文件作为类型定义的入口文件，用于统一导出所有类型
// 具体的类型定义已拆分到不同的文件中：
//
// config.go     - 配置相关类型
// market.go     - 市场数据相关类型
// strategy.go   - 策略配置相关类型
// indicators.go - 技术指标相关类型
//
// 使用方式：
// import "okx-market-sentry/pkg/types"
// config := &types.Config{}
// kline := &types.KLine{}
// signal := &types.TradingSignal{}
