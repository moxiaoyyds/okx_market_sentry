package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"okx-market-sentry/pkg/types"
)

// CircularQueue 循环队列实现滑动窗口
type CircularQueue struct {
	data   []types.PriceDataPoint
	maxAge time.Duration
	mutex  sync.RWMutex
}

func NewCircularQueue(maxAge time.Duration) *CircularQueue {
	return &CircularQueue{
		data:   make([]types.PriceDataPoint, 0, 10),
		maxAge: maxAge,
	}
}

func (cq *CircularQueue) Add(point types.PriceDataPoint) {
	cq.mutex.Lock()
	defer cq.mutex.Unlock()

	// 添加新数据点
	cq.data = append(cq.data, point)

	// 清理超过maxAge的旧数据
	cutoff := time.Now().Add(-cq.maxAge)
	newStart := 0
	for i, p := range cq.data {
		if p.Timestamp.After(cutoff) {
			newStart = i
			break
		}
	}
	if newStart > 0 {
		cq.data = cq.data[newStart:]
	}
}

func (cq *CircularQueue) GetOldest() *types.PriceDataPoint {
	cq.mutex.RLock()
	defer cq.mutex.RUnlock()

	if len(cq.data) == 0 {
		return nil
	}
	return &cq.data[0]
}

func (cq *CircularQueue) GetLatest() *types.PriceDataPoint {
	cq.mutex.RLock()
	defer cq.mutex.RUnlock()

	if len(cq.data) == 0 {
		return nil
	}
	return &cq.data[len(cq.data)-1]
}

func (cq *CircularQueue) FindPriceAroundTime(targetTime time.Time) *types.PriceDataPoint {
	cq.mutex.RLock()
	defer cq.mutex.RUnlock()

	if len(cq.data) < 2 {
		return nil
	}

	var closest *types.PriceDataPoint
	minDiff := time.Duration(math.MaxInt64)

	for i := range cq.data {
		diff := targetTime.Sub(cq.data[i].Timestamp)
		if diff < 0 {
			diff = -diff
		}

		if diff < minDiff {
			minDiff = diff
			closest = &cq.data[i]
		}
	}

	// 如果最接近的数据点与目标时间相差超过2分钟，认为数据不足
	if minDiff > 2*time.Minute {
		return nil
	}

	return closest
}

func (cq *CircularQueue) Length() int {
	cq.mutex.RLock()
	defer cq.mutex.RUnlock()
	return len(cq.data)
}

// StateManager 状态管理器
type StateManager struct {
	priceHistory map[string]*CircularQueue
	mutex        sync.RWMutex
	windowSize   time.Duration
	redisClient  *redis.Client
	useRedis     bool
}

func NewStateManager(redisConfig types.RedisConfig) *StateManager {
	sm := &StateManager{
		priceHistory: make(map[string]*CircularQueue),
		windowSize:   5 * time.Minute,
	}

	// 尝试连接Redis
	if redisConfig.URL != "" {
		sm.redisClient = redis.NewClient(&redis.Options{
			Addr:     redisConfig.URL,
			Password: redisConfig.Password,
			DB:       redisConfig.DB,
		})

		// 测试连接
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		_, err := sm.redisClient.Ping(ctx).Result()
		if err != nil {
			fmt.Printf("⚠️  Redis连接失败，使用纯内存模式: %v\n", err)
			sm.useRedis = false
		} else {
			fmt.Println("✅ Redis连接成功")
			sm.useRedis = true
		}
	} else {
		fmt.Println("🔧 未配置Redis，使用纯内存模式")
		sm.useRedis = false
	}

	return sm
}

func (sm *StateManager) Store(symbol string, price float64, timestamp time.Time) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 获取或创建队列
	if sm.priceHistory[symbol] == nil {
		sm.priceHistory[symbol] = NewCircularQueue(sm.windowSize)
	}

	// 添加新数据点
	dataPoint := types.PriceDataPoint{
		Price:     price,
		Timestamp: timestamp,
	}
	sm.priceHistory[symbol].Add(dataPoint)

	// 异步备份到Redis
	if sm.useRedis {
		go sm.backupToRedis(symbol, dataPoint)
	}
}

// backupToRedis 备份数据到Redis
func (sm *StateManager) backupToRedis(symbol string, point types.PriceDataPoint) {
	if !sm.useRedis {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := fmt.Sprintf("okx:price:%s", symbol)
	value, err := json.Marshal(point)
	if err != nil {
		fmt.Printf("序列化价格数据失败: %v\n", err)
		return
	}

	// 使用Redis Sorted Set存储，以时间戳为分数
	err = sm.redisClient.ZAdd(ctx, key, &redis.Z{
		Score:  float64(point.Timestamp.Unix()),
		Member: value,
	}).Err()

	if err != nil {
		fmt.Printf("Redis存储失败 %s: %v\n", symbol, err)
		return
	}

	// 设置过期时间，只保留10分钟数据
	sm.redisClient.Expire(ctx, key, 10*time.Minute)
	
	// 清理旧数据，只保留最近10分钟
	cutoff := float64(time.Now().Add(-10 * time.Minute).Unix())
	sm.redisClient.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%.0f", cutoff))
}

func (sm *StateManager) GetPriceData(symbol string) (*types.PriceDataPoint, *types.PriceDataPoint) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	queue := sm.priceHistory[symbol]
	if queue == nil {
		return nil, nil
	}

	// 获取最新价格
	current := queue.GetLatest()
	if current == nil {
		return nil, nil
	}

	// 获取5分钟前的价格
	past := queue.FindPriceAroundTime(time.Now().Add(-sm.windowSize))

	return current, past
}

func (sm *StateManager) GetAllSymbols() []string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	symbols := make([]string, 0, len(sm.priceHistory))
	for symbol := range sm.priceHistory {
		symbols = append(symbols, symbol)
	}
	return symbols
}

// GetRedisStats 获取Redis统计信息
func (sm *StateManager) GetRedisStats() map[string]interface{} {
	stats := map[string]interface{}{
		"redis_enabled": sm.useRedis,
		"memory_symbols": len(sm.priceHistory),
	}

	if sm.useRedis {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// 获取Redis中的key数量
		keys, err := sm.redisClient.Keys(ctx, "okx:price:*").Result()
		if err == nil {
			stats["redis_keys"] = len(keys)
		} else {
			stats["redis_error"] = err.Error()
		}
	}

	return stats
}