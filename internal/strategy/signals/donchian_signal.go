package signals

import (
	"go.uber.org/zap"
	"okx-market-sentry/internal/strategy/indicators"
	"okx-market-sentry/pkg/types"
)

// DonchianSignalDetector 唐奇安通道信号检测器
type DonchianSignalDetector struct {
	donchianCalc *indicators.DonchianCalculator
	atrCalc      *indicators.ATRCalculator
	config       types.DonchianConfig
}

// NewDonchianSignalDetector 创建信号检测器
func NewDonchianSignalDetector(config types.DonchianConfig) *DonchianSignalDetector {
	return &DonchianSignalDetector{
		donchianCalc: indicators.NewDonchianCalculator(config.DonchianLength, config.DonchianOffset),
		atrCalc:      indicators.NewATRCalculator(config.ATRLength),
		config:       config,
	}
}

// DetectSignal 检测交易信号
func (dsd *DonchianSignalDetector) DetectSignal(symbol string, klines []*types.KLine) *types.TradingSignal {
	if len(klines) < dsd.getRequiredBars() {
		return nil
	}

	// 1. 检测盘整阶段
	isConsolidation, consolidationBars := dsd.donchianCalc.DetectConsolidation(klines, dsd.config.ConsolidationBars)
	if !isConsolidation {
		zap.L().Debug("未检测到盘整状态", 
			zap.String("symbol", symbol),
			zap.Int("consolidation_bars", consolidationBars))
		return nil
	}

	// 2. 计算ATR并检查下降趋势
	atrData := dsd.atrCalc.Calculate(klines)
	if atrData == nil {
		return nil
	}

	isATRDecreasing := dsd.atrCalc.IsATRDecreasing(atrData, klines)
	if !isATRDecreasing {
		zap.L().Debug("ATR未呈现下降趋势", 
			zap.String("symbol", symbol),
			zap.Float64("atr_value", atrData.Value),
			zap.Float64("atr_slope", atrData.Slope))
		return nil
	}

	// 3. 计算唐奇安通道
	channel := dsd.donchianCalc.Calculate(klines)
	if channel == nil {
		return nil
	}

	// 4. 检查突破确认
	isBreakout, direction := dsd.donchianCalc.CalculateBreakout(klines, channel)
	if !isBreakout {
		return nil
	}

	// 5. 验证突破有效性（包括成交量确认）
	isValidBreakout := dsd.donchianCalc.IsValidBreakout(klines, channel, dsd.config.VolumeMultiplier)
	if !isValidBreakout {
		zap.L().Debug("突破无效", 
			zap.String("symbol", symbol),
			zap.String("direction", direction))
		return nil
	}

	// 6. 计算信号强度
	signalStrength := dsd.calculateSignalStrength(klines, channel, atrData)
	if signalStrength < dsd.config.MinSignalStrength {
		zap.L().Debug("信号强度不足", 
			zap.String("symbol", symbol),
			zap.Float64("signal_strength", signalStrength),
			zap.Float64("min_required", dsd.config.MinSignalStrength))
		return nil
	}

	// 构建交易信号
	latest := klines[len(klines)-1]
	previous := klines[len(klines)-2]
	
	signal := &types.TradingSignal{
		Symbol:            symbol,
		SignalType:        direction,
		Price:             latest.Close,
		Volume:            latest.Volume,
		VolumeRatio:       latest.Volume / previous.Volume,
		DonchianUpper:     channel.Upper,
		ATRValue:          atrData.Value,
		ConsolidationBars: consolidationBars,
		SignalStrength:    signalStrength,
		SignalTime:        latest.CloseTime,
	}

	zap.L().Info("🎯 检测到交易信号", 
		zap.String("symbol", symbol),
		zap.String("signal_type", direction),
		zap.Float64("price", latest.Close),
		zap.Float64("volume_ratio", signal.VolumeRatio),
		zap.Float64("signal_strength", signalStrength),
		zap.Int("consolidation_bars", consolidationBars))

	return signal
}

// calculateSignalStrength 计算信号强度
func (dsd *DonchianSignalDetector) calculateSignalStrength(klines []*types.KLine, channel *types.DonchianChannel, atrData *types.ATRData) float64 {
	if len(klines) < 2 || channel == nil || atrData == nil {
		return 0
	}

	latest := klines[len(klines)-1]
	previous := klines[len(klines)-2]

	var strength float64

	// 1. 突破幅度权重 (0-30分)
	breakoutStrength := dsd.calculateBreakoutStrength(latest, channel)
	strength += breakoutStrength * 0.3

	// 2. 成交量确认权重 (0-25分)
	volumeStrength := dsd.calculateVolumeStrength(latest, previous)
	strength += volumeStrength * 0.25

	// 3. ATR下降确认权重 (0-20分)
	atrStrength := dsd.calculateATRStrength(atrData, klines)
	strength += atrStrength * 0.2

	// 4. K线形态权重 (0-15分)
	candleStrength := dsd.calculateCandleStrength(latest)
	strength += candleStrength * 0.15

	// 5. 通道位置权重 (0-10分)
	positionStrength := dsd.calculatePositionStrength(latest, channel)
	strength += positionStrength * 0.1

	return strength
}

// calculateBreakoutStrength 计算突破强度
func (dsd *DonchianSignalDetector) calculateBreakoutStrength(kline *types.KLine, channel *types.DonchianChannel) float64 {
	if channel.Upper == channel.Lower {
		return 0
	}

	// 计算突破幅度相对于通道宽度的比例
	channelWidth := channel.Upper - channel.Lower
	var breakoutDistance float64

	if kline.Close > channel.Upper {
		breakoutDistance = kline.Close - channel.Upper
	} else if kline.Close < channel.Lower {
		breakoutDistance = channel.Lower - kline.Close
	} else {
		return 0
	}

	// 突破距离相对于通道宽度的比例，最大为100%
	ratio := (breakoutDistance / channelWidth) * 100
	if ratio > 100 {
		ratio = 100
	}

	return ratio
}

// calculateVolumeStrength 计算成交量强度
func (dsd *DonchianSignalDetector) calculateVolumeStrength(current, previous *types.KLine) float64 {
	if previous.Volume == 0 {
		return 0
	}

	volumeRatio := current.Volume / previous.Volume
	
	// 成交量倍数越高，强度越大，最高100分
	if volumeRatio >= dsd.config.VolumeMultiplier*2 {
		return 100
	} else if volumeRatio >= dsd.config.VolumeMultiplier {
		return 80
	} else if volumeRatio >= dsd.config.VolumeMultiplier*0.8 {
		return 60
	} else if volumeRatio >= dsd.config.VolumeMultiplier*0.6 {
		return 40
	} else {
		return 20
	}
}

// calculateATRStrength 计算ATR强度
func (dsd *DonchianSignalDetector) calculateATRStrength(atrData *types.ATRData, klines []*types.KLine) float64 {
	// ATR斜率越负，强度越高
	slopeStrength := 0.0
	if atrData.Slope < 0 {
		// 将负斜率转换为正强度值
		slopeStrength = (-atrData.Slope) * 1000 // 放大斜率值
		if slopeStrength > 50 {
			slopeStrength = 50
		}
	}

	// ATR百分位越低，强度越高
	percentile := dsd.atrCalc.GetATRPercentile(atrData.Value, klines)
	percentileStrength := (100 - percentile) / 2 // 0-50分

	return slopeStrength + percentileStrength
}

// calculateCandleStrength 计算K线形态强度
func (dsd *DonchianSignalDetector) calculateCandleStrength(kline *types.KLine) float64 {
	if kline.High == kline.Low {
		return 0
	}

	// 计算实体大小占整个K线的比例
	bodySize := kline.Close - kline.Open
	if bodySize < 0 {
		bodySize = -bodySize
	}
	
	totalRange := kline.High - kline.Low
	bodyRatio := (bodySize / totalRange) * 100

	// 实体越大，形态越强
	return bodyRatio
}

// calculatePositionStrength 计算通道位置强度
func (dsd *DonchianSignalDetector) calculatePositionStrength(kline *types.KLine, channel *types.DonchianChannel) float64 {
	position := dsd.donchianCalc.GetDonchianPosition(kline.Close, channel)
	
	// 突破上轨时，位置越高强度越大
	if kline.Close > channel.Upper {
		return (position - 1) * 200 // 超过1的部分转换为0-100分
	}
	
	// 突破下轨时，位置越低强度越大
	if kline.Close < channel.Lower {
		return (0 - position) * 200 // 低于0的部分转换为0-100分
	}
	
	return 0
}

// getRequiredBars 获取所需的最小K线数量
func (dsd *DonchianSignalDetector) getRequiredBars() int {
	// 需要足够的数据来计算所有指标
	required := dsd.config.ConsolidationBars + dsd.config.DonchianLength + dsd.config.DonchianOffset + dsd.config.ATRLength + 45
	return required
}

// ValidateSignalConditions 验证信号条件
func (dsd *DonchianSignalDetector) ValidateSignalConditions(symbol string, klines []*types.KLine) map[string]interface{} {
	conditions := make(map[string]interface{})
	
	if len(klines) < dsd.getRequiredBars() {
		conditions["sufficient_data"] = false
		conditions["required_bars"] = dsd.getRequiredBars()
		conditions["available_bars"] = len(klines)
		return conditions
	}
	
	conditions["sufficient_data"] = true
	
	// 检查盘整条件
	isConsolidation, consolidationBars := dsd.donchianCalc.DetectConsolidation(klines, dsd.config.ConsolidationBars)
	conditions["consolidation"] = isConsolidation
	conditions["consolidation_bars"] = consolidationBars
	
	// 检查ATR条件
	atrData := dsd.atrCalc.Calculate(klines)
	if atrData != nil {
		conditions["atr_value"] = atrData.Value
		conditions["atr_slope"] = atrData.Slope
		conditions["atr_decreasing"] = dsd.atrCalc.IsATRDecreasing(atrData, klines)
	}
	
	// 检查唐奇安通道
	channel := dsd.donchianCalc.Calculate(klines)
	if channel != nil {
		conditions["donchian_upper"] = channel.Upper
		conditions["donchian_lower"] = channel.Lower
		conditions["donchian_middle"] = channel.Middle
		
		isBreakout, direction := dsd.donchianCalc.CalculateBreakout(klines, channel)
		conditions["breakout"] = isBreakout
		conditions["breakout_direction"] = direction
		
		if len(klines) > 0 {
			latest := klines[len(klines)-1]
			conditions["current_price"] = latest.Close
			conditions["price_position"] = dsd.donchianCalc.GetDonchianPosition(latest.Close, channel)
		}
	}
	
	return conditions
}