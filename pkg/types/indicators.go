package types

import "time"

// DonchianChannel 唐奇安通道数据
type DonchianChannel struct {
	Upper  float64 `json:"upper"`  // 上轨
	Lower  float64 `json:"lower"`  // 下轨
	Middle float64 `json:"middle"` // 中轨
}

// ATRData ATR指标数据
type ATRData struct {
	Value float64 `json:"value"` // ATR值
	Slope float64 `json:"slope"` // ATR斜率
}

// TradingSignal 交易信号
type TradingSignal struct {
	Symbol            string    `json:"symbol"`
	SignalType        string    `json:"signal_type"`        // LONG, SHORT, CLOSE
	Price             float64   `json:"price"`              // 信号价格
	Volume            float64   `json:"volume"`             // 成交量
	VolumeRatio       float64   `json:"volume_ratio"`       // 成交量倍数
	DonchianUpper     float64   `json:"donchian_upper"`     // 唐奇安上轨
	ATRValue          float64   `json:"atr_value"`          // ATR值
	ConsolidationBars int       `json:"consolidation_bars"` // 盘整K线数
	SignalStrength    float64   `json:"signal_strength"`    // 信号强度
	SignalTime        time.Time `json:"signal_time"`        // 信号时间
}

// TODO: 未来可以添加其他技术指标
// MACDData MACD指标数据
// type MACDData struct {
//     DIF    float64 `json:"dif"`    // 差离值
//     DEA    float64 `json:"dea"`    // 信号线
//     MACD   float64 `json:"macd"`   // MACD柱状图
// }

// RSIData RSI指标数据
// type RSIData struct {
//     Value float64 `json:"value"`  // RSI值
// }
