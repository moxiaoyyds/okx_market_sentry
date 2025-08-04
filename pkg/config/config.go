package config

import (
	"errors"
	"time"

	"github.com/spf13/viper"
	"okx-market-sentry/pkg/types"
)

// Load 加载配置
func Load() (*types.Config, error) {
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置默认值
	setDefaults()

	// 读取环境变量
	viper.AutomaticEnv()

	// 优先尝试读取本地配置文件
	viper.SetConfigName("config.local")
	if err := viper.ReadInConfig(); err != nil {
		// 如果本地配置文件不存在，尝试读取默认配置文件
		viper.SetConfigName("config")
		if err := viper.ReadInConfig(); err != nil {
			var configFileNotFoundError viper.ConfigFileNotFoundError
			if !errors.As(err, &configFileNotFoundError) {
				return nil, err
			}
		}
	}

	var config types.Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func setDefaults() {
	viper.SetDefault("log_level", "info") // 兼容保留
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.file_path", "logs")
	viper.SetDefault("log.max_size", 200)
	viper.SetDefault("log.max_age", 30)
	viper.SetDefault("log.max_backups", 7)
	viper.SetDefault("log.compress", false)
	viper.SetDefault("redis.url", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("dingtalk.webhook_url", "")
	viper.SetDefault("dingtalk.secret", "")
	viper.SetDefault("pushplus.user_token", "")
	viper.SetDefault("pushplus.to", "")
	viper.SetDefault("alert.threshold", 3.0)
	viper.SetDefault("alert.monitor_period", 5*time.Minute)
	viper.SetDefault("fetch.interval", time.Minute)
	viper.SetDefault("network.proxy", "")
	viper.SetDefault("network.timeout", 30*time.Second)
}
