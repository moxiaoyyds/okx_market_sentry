package types

import "time"

// PriceDataPoint 价格数据点
type PriceDataPoint struct {
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

// AlertData 预警数据
type AlertData struct {
	Symbol        string    `json:"symbol"`
	CurrentPrice  float64   `json:"current_price"`
	PastPrice     float64   `json:"past_price"`
	ChangePercent float64   `json:"change_percent"`
	AlertTime     time.Time `json:"alert_time"`
}

// Config 配置结构
type Config struct {
	LogLevel string         `mapstructure:"log_level"`
	Redis    RedisConfig    `mapstructure:"redis"`
	DingTalk DingTalkConfig `mapstructure:"dingtalk"`
	PushPlus PushPlusConfig `mapstructure:"pushplus"`
	Alert    AlertConfig    `mapstructure:"alert"`
	Fetch    FetchConfig    `mapstructure:"fetch"`
	Network  NetworkConfig  `mapstructure:"network"`
}

type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type DingTalkConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
}

type PushPlusConfig struct {
	UserToken string `mapstructure:"user_token"`
	To        string `mapstructure:"to"` // 好友令牌，多人用逗号分隔
}

type AlertConfig struct {
	Threshold float64 `mapstructure:"threshold"`
}

type FetchConfig struct {
	Interval time.Duration `mapstructure:"interval"`
}

type NetworkConfig struct {
	Proxy   string        `mapstructure:"proxy"`   // HTTP代理地址，如 http://127.0.0.1:7890
	Timeout time.Duration `mapstructure:"timeout"` // 网络超时时间
}
