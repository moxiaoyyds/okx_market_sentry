package database

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"okx-market-sentry/pkg/types"
)

// Manager 数据库管理器
type Manager struct {
	db     *gorm.DB
	config types.MySQLConfig
}

// KLine 数据库K线模型
type KLine struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Symbol    string    `gorm:"type:varchar(20);not null;index:idx_symbol_time" json:"symbol"`
	OpenTime  int64     `gorm:"not null;index:idx_symbol_time" json:"open_time"`
	CloseTime int64     `gorm:"not null;index:idx_close_time" json:"close_time"`
	Open      float64   `gorm:"type:decimal(20,8);not null" json:"open"`
	High      float64   `gorm:"type:decimal(20,8);not null" json:"high"`
	Low       float64   `gorm:"type:decimal(20,8);not null" json:"low"`
	Close     float64   `gorm:"type:decimal(20,8);not null" json:"close"`
	Volume    float64   `gorm:"type:decimal(20,8);not null" json:"volume"`
	Interval  string    `gorm:"type:varchar(10);not null;default:'15m'" json:"interval"`
	CreatedAt time.Time `json:"created_at"`
}

// Indicator 技术指标模型
type Indicator struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Symbol            string    `gorm:"type:varchar(20);not null;index:idx_symbol_time" json:"symbol"`
	KlineTime         int64     `gorm:"not null;index:idx_symbol_time" json:"kline_time"`
	DonchianUpper     *float64  `gorm:"type:decimal(20,8)" json:"donchian_upper"`
	DonchianLower     *float64  `gorm:"type:decimal(20,8)" json:"donchian_lower"`
	ATRValue          *float64  `gorm:"type:decimal(20,8)" json:"atr_value"`
	ATRSlope          *float64  `gorm:"type:decimal(10,6)" json:"atr_slope"`
	IsConsolidation   bool      `gorm:"default:false" json:"is_consolidation"`
	ConsolidationBars int       `gorm:"default:0" json:"consolidation_bars"`
	CreatedAt         time.Time `json:"created_at"`
}

// TradingSignal 交易信号模型
type TradingSignal struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Symbol            string    `gorm:"type:varchar(20);not null;index:idx_symbol_time" json:"symbol"`
	SignalTime        int64     `gorm:"not null;index:idx_symbol_time" json:"signal_time"`
	SignalType        string    `gorm:"type:enum('LONG','SHORT','CLOSE');not null" json:"signal_type"`
	Price             float64   `gorm:"type:decimal(20,8);not null" json:"price"`
	Volume            float64   `gorm:"type:decimal(20,8);not null" json:"volume"`
	VolumeRatio       *float64  `gorm:"type:decimal(5,2)" json:"volume_ratio"`
	DonchianUpper     *float64  `gorm:"type:decimal(20,8)" json:"donchian_upper"`
	ATRValue          *float64  `gorm:"type:decimal(20,8)" json:"atr_value"`
	ConsolidationBars *int      `json:"consolidation_bars"`
	SignalStrength    *float64  `gorm:"type:decimal(3,2)" json:"signal_strength"`
	CreatedAt         time.Time `json:"created_at"`
}

// StrategyPerformance 策略性能模型
type StrategyPerformance struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Symbol            string    `gorm:"type:varchar(20);not null;uniqueIndex:uk_symbol_date" json:"symbol"`
	Date              time.Time `gorm:"type:date;not null;uniqueIndex:uk_symbol_date" json:"date"`
	TotalSignals      int       `gorm:"default:0" json:"total_signals"`
	LongSignals       int       `gorm:"default:0" json:"long_signals"`
	ShortSignals      int       `gorm:"default:0" json:"short_signals"`
	AvgSignalStrength *float64  `gorm:"type:decimal(3,2)" json:"avg_signal_strength"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// NewManager 创建数据库管理器
func NewManager(config types.MySQLConfig) (*Manager, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)

	// 配置GORM日志
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // 生产环境使用Silent
	}

	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("连接MySQL失败: %v", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库实例失败: %v", err)
	}

	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	manager := &Manager{
		db:     db,
		config: config,
	}

	// 自动迁移表结构
	if err := manager.AutoMigrate(); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %v", err)
	}

	zap.L().Info("✅ MySQL数据库连接成功",
		zap.String("host", config.Host),
		zap.Int("port", config.Port),
		zap.String("database", config.Database))

	return manager, nil
}

// AutoMigrate 自动迁移表结构
func (m *Manager) AutoMigrate() error {
	return m.db.AutoMigrate(
		&KLine{},
		&Indicator{},
		&TradingSignal{},
		&StrategyPerformance{},
	)
}

// SaveKLine 保存K线数据
func (m *Manager) SaveKLine(kline *types.KLine) error {
	dbKline := &KLine{
		Symbol:    kline.Symbol,
		OpenTime:  kline.OpenTime.Unix(),
		CloseTime: kline.CloseTime.Unix(),
		Open:      kline.Open,
		High:      kline.High,
		Low:       kline.Low,
		Close:     kline.Close,
		Volume:    kline.Volume,
		Interval:  kline.Interval,
		CreatedAt: time.Now(),
	}

	// 使用ON DUPLICATE KEY UPDATE避免重复插入
	result := m.db.Create(dbKline)
	if result.Error != nil {
		// 如果是重复键错误，尝试更新
		if result.Error.Error() == "UNIQUE constraint failed" {
			return m.db.Where("symbol = ? AND open_time = ? AND interval = ?",
				dbKline.Symbol, dbKline.OpenTime, dbKline.Interval).
				Updates(dbKline).Error
		}
		return result.Error
	}

	return nil
}

// SaveIndicator 保存技术指标数据
func (m *Manager) SaveIndicator(symbol string, klineTime time.Time, donchianChannel *types.DonchianChannel, atrData *types.ATRData, isConsolidation bool, consolidationBars int) error {
	indicator := &Indicator{
		Symbol:            symbol,
		KlineTime:         klineTime.Unix(),
		IsConsolidation:   isConsolidation,
		ConsolidationBars: consolidationBars,
		CreatedAt:         time.Now(),
	}

	if donchianChannel != nil {
		indicator.DonchianUpper = &donchianChannel.Upper
		indicator.DonchianLower = &donchianChannel.Lower
	}

	if atrData != nil {
		indicator.ATRValue = &atrData.Value
		indicator.ATRSlope = &atrData.Slope
	}

	// 使用UPSERT操作
	return m.db.Create(indicator).Error
}

// SaveTradingSignal 保存交易信号
func (m *Manager) SaveTradingSignal(signal *types.TradingSignal) error {
	dbSignal := &TradingSignal{
		Symbol:            signal.Symbol,
		SignalTime:        signal.SignalTime.Unix(),
		SignalType:        signal.SignalType,
		Price:             signal.Price,
		Volume:            signal.Volume,
		VolumeRatio:       &signal.VolumeRatio,
		DonchianUpper:     &signal.DonchianUpper,
		ATRValue:          &signal.ATRValue,
		ConsolidationBars: &signal.ConsolidationBars,
		SignalStrength:    &signal.SignalStrength,
		CreatedAt:         time.Now(),
	}

	return m.db.Create(dbSignal).Error
}

// UpdateStrategyPerformance 更新策略性能统计
func (m *Manager) UpdateStrategyPerformance(symbol string, signalType string, signalStrength float64) error {
	today := time.Now().Truncate(24 * time.Hour)

	var performance StrategyPerformance
	result := m.db.Where("symbol = ? AND date = ?", symbol, today).First(&performance)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 创建新记录
		performance = StrategyPerformance{
			Symbol:            symbol,
			Date:              today,
			TotalSignals:      1,
			AvgSignalStrength: &signalStrength,
		}

		if signalType == "LONG" {
			performance.LongSignals = 1
		} else if signalType == "SHORT" {
			performance.ShortSignals = 1
		}

		return m.db.Create(&performance).Error
	} else if result.Error != nil {
		return result.Error
	} else {
		// 更新现有记录
		updates := map[string]interface{}{
			"total_signals": performance.TotalSignals + 1,
		}

		if signalType == "LONG" {
			updates["long_signals"] = performance.LongSignals + 1
		} else if signalType == "SHORT" {
			updates["short_signals"] = performance.ShortSignals + 1
		}

		// 计算新的平均信号强度
		if performance.AvgSignalStrength != nil {
			newAvg := ((*performance.AvgSignalStrength)*float64(performance.TotalSignals) + signalStrength) / float64(performance.TotalSignals+1)
			updates["avg_signal_strength"] = newAvg
		} else {
			updates["avg_signal_strength"] = signalStrength
		}

		return m.db.Model(&performance).Where("id = ?", performance.ID).Updates(updates).Error
	}
}

// GetKLines 获取K线数据
func (m *Manager) GetKLines(symbol string, interval string, limit int) ([]*types.KLine, error) {
	var dbKlines []KLine
	err := m.db.Where("symbol = ? AND interval = ?", symbol, interval).
		Order("open_time DESC").
		Limit(limit).
		Find(&dbKlines).Error

	if err != nil {
		return nil, err
	}

	var klines []*types.KLine
	for _, dbKline := range dbKlines {
		kline := &types.KLine{
			Symbol:    dbKline.Symbol,
			OpenTime:  time.Unix(dbKline.OpenTime, 0),
			CloseTime: time.Unix(dbKline.CloseTime, 0),
			Open:      dbKline.Open,
			High:      dbKline.High,
			Low:       dbKline.Low,
			Close:     dbKline.Close,
			Volume:    dbKline.Volume,
			Interval:  dbKline.Interval,
		}
		klines = append(klines, kline)
	}

	return klines, nil
}

// GetTradingSignals 获取交易信号
func (m *Manager) GetTradingSignals(symbol string, limit int) ([]TradingSignal, error) {
	var signals []TradingSignal
	err := m.db.Where("symbol = ?", symbol).
		Order("signal_time DESC").
		Limit(limit).
		Find(&signals).Error

	return signals, err
}

// GetStrategyPerformance 获取策略性能数据
func (m *Manager) GetStrategyPerformance(symbol string, days int) ([]StrategyPerformance, error) {
	var performances []StrategyPerformance
	startDate := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)

	err := m.db.Where("symbol = ? AND date >= ?", symbol, startDate).
		Order("date DESC").
		Find(&performances).Error

	return performances, err
}

// BatchSaveKlines 批量保存K线数据
func (m *Manager) BatchSaveKlines(klines []*types.KLine) error {
	if len(klines) == 0 {
		return nil
	}

	// 转换为数据库模型
	dbKlines := make([]KLine, 0, len(klines))
	for _, kline := range klines {
		dbKline := KLine{
			Symbol:    kline.Symbol,
			OpenTime:  kline.OpenTime.Unix(),
			CloseTime: kline.CloseTime.Unix(),
			Open:      kline.Open,
			High:      kline.High,
			Low:       kline.Low,
			Close:     kline.Close,
			Volume:    kline.Volume,
			Interval:  kline.Interval,
			CreatedAt: time.Now(),
		}
		dbKlines = append(dbKlines, dbKline)
	}

	// 批量插入，使用ON CONFLICT处理重复键
	tx := m.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// 分批处理避免单个事务过大
	batchSize := 100
	for i := 0; i < len(dbKlines); i += batchSize {
		end := i + batchSize
		if end > len(dbKlines) {
			end = len(dbKlines)
		}

		batch := dbKlines[i:end]

		// 使用CreateInBatches进行批量插入
		if err := tx.CreateInBatches(batch, len(batch)).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("批量插入K线数据失败: %v", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交批量插入事务失败: %v", err)
	}

	zap.L().Debug("✅ 批量保存K线数据完成",
		zap.Int("count", len(klines)),
		zap.String("first_symbol", klines[0].Symbol))

	return nil
}

// Close 关闭数据库连接
func (m *Manager) Close() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Health 检查数据库连接健康状态
func (m *Manager) Health() error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
