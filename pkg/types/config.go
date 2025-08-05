package types

import "time"

// Config 主配置结构
type Config struct {
	LogLevel string         `mapstructure:"log_level"` // 兼容保留
	Log      LogConfig      `mapstructure:"log"`
	Redis    RedisConfig    `mapstructure:"redis"`
	DingTalk DingTalkConfig `mapstructure:"dingtalk"`
	PushPlus PushPlusConfig `mapstructure:"pushplus"`
	Alert    AlertConfig    `mapstructure:"alert"`
	Fetch    FetchConfig    `mapstructure:"fetch"`
	Network  NetworkConfig  `mapstructure:"network"`
	Strategy StrategyConfig `mapstructure:"strategy"` // 新增策略配置
	Database DatabaseConfig `mapstructure:"database"` // 新增数据库配置
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`       // 日志级别
	FilePath   string `mapstructure:"file_path"`   // 日志输出路径名
	MaxSize    int    `mapstructure:"max_size"`    // 日志文件大小 单位：MB，超限后会自动切割
	MaxAge     int    `mapstructure:"max_age"`     // 日志文件存放时间 单位：天
	MaxBackups int    `mapstructure:"max_backups"` // 日志文件备份数量
	Compress   bool   `mapstructure:"compress"`    // 日志文件压缩
}

// RedisConfig Redis配置
type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
	Secret     string `mapstructure:"secret"`
}

// PushPlusConfig PushPlus配置
type PushPlusConfig struct {
	UserToken string `mapstructure:"user_token"`
	To        string `mapstructure:"to"` // 好友令牌，多人用逗号分隔
}

// AlertConfig 预警配置
type AlertConfig struct {
	Threshold     float64       `mapstructure:"threshold"`
	MonitorPeriod time.Duration `mapstructure:"monitor_period"` // 监控周期，用于价格对比
}

// FetchConfig 数据获取配置
type FetchConfig struct {
	Interval time.Duration `mapstructure:"interval"`
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	Proxy   string        `mapstructure:"proxy"`   // HTTP代理地址，如 http://127.0.0.1:7890
	Timeout time.Duration `mapstructure:"timeout"` // 网络超时时间
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	MySQL MySQLConfig `mapstructure:"mysql"`
}

// MySQLConfig MySQL配置
type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	OKXEndpoint          string        `mapstructure:"okx_endpoint"`
	ReconnectInterval    time.Duration `mapstructure:"reconnect_interval"`
	PingInterval         time.Duration `mapstructure:"ping_interval"`
	MaxReconnectAttempts int           `mapstructure:"max_reconnect_attempts"`
}
