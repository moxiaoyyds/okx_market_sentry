package types

import "time"

// PriceDataPoint 价格数据点（原有价格监控系统使用）
type PriceDataPoint struct {
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

// AlertData 预警数据（原有价格监控系统使用）
type AlertData struct {
	Symbol        string        `json:"symbol"`
	CurrentPrice  float64       `json:"current_price"`
	PastPrice     float64       `json:"past_price"`
	ChangePercent float64       `json:"change_percent"`
	AlertTime     time.Time     `json:"alert_time"`
	MonitorPeriod time.Duration `json:"monitor_period"` // 监控周期
}

// KLine K线数据结构（通用市场数据）
type KLine struct {
	Symbol    string    `json:"symbol"`
	OpenTime  time.Time `json:"open_time"`
	CloseTime time.Time `json:"close_time"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	Interval  string    `json:"interval"` // 15m
}
