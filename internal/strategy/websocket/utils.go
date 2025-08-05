package websocket

import (
	"strconv"
	"time"
)

// parseTimestamp 解析时间戳（毫秒）
func parseTimestamp(ts string) (time.Time, error) {
	timestamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(timestamp/1000, (timestamp%1000)*1000000), nil
}

// parseFloat 解析浮点数
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// getIntervalDuration 获取时间间隔的Duration
func getIntervalDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "3m":
		return 3 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1H":
		return time.Hour
	case "2H":
		return 2 * time.Hour
	case "4H":
		return 4 * time.Hour
	case "6H":
		return 6 * time.Hour
	case "12H":
		return 12 * time.Hour
	case "1D":
		return 24 * time.Hour
	default:
		return 15 * time.Minute // 默认15分钟
	}
}
