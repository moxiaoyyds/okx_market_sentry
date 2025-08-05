package types

// StrategyConfig 策略配置总入口
type StrategyConfig struct {
	Donchian DonchianConfig `mapstructure:"donchian"`
	// 未来可以添加其他策略配置
	// MACD    MACDConfig    `mapstructure:"macd"`
	// RSI     RSIConfig     `mapstructure:"rsi"`
}

// DonchianConfig 唐奇安通道策略配置
type DonchianConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	Symbols           []string `mapstructure:"symbols"`
	Interval          string   `mapstructure:"interval"`            // K线周期，如 15m
	DonchianLength    int      `mapstructure:"donchian_length"`     // 唐奇安通道长度，默认30
	DonchianOffset    int      `mapstructure:"donchian_offset"`     // 唐奇安通道偏移，默认1
	ATRLength         int      `mapstructure:"atr_length"`          // ATR长度，默认14
	ConsolidationBars int      `mapstructure:"consolidation_bars"`  // 盘整检测K线数，默认45
	VolumeMultiplier  float64  `mapstructure:"volume_multiplier"`   // 成交量倍数，默认3.0
	MinSignalStrength float64  `mapstructure:"min_signal_strength"` // 最小信号强度，默认0.7
}
