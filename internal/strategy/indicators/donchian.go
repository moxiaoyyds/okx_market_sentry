package indicators

import (
	"okx-market-sentry/pkg/types"
)

// DonchianCalculator 唐奇安通道计算器
type DonchianCalculator struct {
	length int
	offset int
}

// NewDonchianCalculator 创建唐奇安通道计算器
func NewDonchianCalculator(length, offset int) *DonchianCalculator {
	return &DonchianCalculator{
		length: length,
		offset: offset,
	}
}

// Calculate 计算唐奇安通道
func (dc *DonchianCalculator) Calculate(klines []*types.KLine) *types.DonchianChannel {
	if len(klines) < dc.length+dc.offset {
		return nil
	}

	// 计算范围：从offset开始，长度为length的K线数据
	start := len(klines) - dc.length - dc.offset
	end := len(klines) - dc.offset

	if start < 0 {
		start = 0
	}

	var highest, lowest float64
	first := true

	// 找出指定范围内的最高价和最低价
	for i := start; i < end; i++ {
		if first {
			highest = klines[i].High
			lowest = klines[i].Low
			first = false
			continue
		}

		if klines[i].High > highest {
			highest = klines[i].High
		}
		if klines[i].Low < lowest {
			lowest = klines[i].Low
		}
	}

	middle := (highest + lowest) / 2

	return &types.DonchianChannel{
		Upper:  highest,
		Lower:  lowest,
		Middle: middle,
	}
}

// CalculateBreakout 计算是否发生突破
func (dc *DonchianCalculator) CalculateBreakout(klines []*types.KLine, channel *types.DonchianChannel) (bool, string) {
	if len(klines) == 0 || channel == nil {
		return false, ""
	}

	// 获取最新K线
	latest := klines[len(klines)-1]

	// 检查上轨突破（做多信号）
	if latest.Close > channel.Upper && latest.Close > latest.Open {
		return true, "LONG"
	}

	// 检查下轨突破（做空信号）
	if latest.Close < channel.Lower && latest.Close < latest.Open {
		return true, "SHORT"
	}

	return false, ""
}

// DetectConsolidation 检测盘整状态
func (dc *DonchianCalculator) DetectConsolidation(klines []*types.KLine, consolidationBars int) (bool, int) {
	if len(klines) < consolidationBars {
		return false, len(klines)
	}

	// 检查最近consolidationBars根K线的价格区间
	start := len(klines) - consolidationBars
	var highest, lowest float64
	first := true

	for i := start; i < len(klines); i++ {
		if first {
			highest = klines[i].High
			lowest = klines[i].Low
			first = false
			continue
		}

		if klines[i].High > highest {
			highest = klines[i].High
		}
		if klines[i].Low < lowest {
			lowest = klines[i].Low
		}
	}

	// 计算价格区间
	priceRange := highest - lowest
	avgPrice := (highest + lowest) / 2

	// 判断是否为盘整：价格区间相对较小
	// 如果价格区间小于平均价格的5%，认为是盘整
	consolidationThreshold := avgPrice * 0.05
	isConsolidation := priceRange <= consolidationThreshold

	return isConsolidation, consolidationBars
}

// CalculatePriceRange 计算价格区间百分比
func (dc *DonchianCalculator) CalculatePriceRange(klines []*types.KLine, bars int) float64 {
	if len(klines) < bars {
		return 0
	}

	start := len(klines) - bars
	var highest, lowest float64
	first := true

	for i := start; i < len(klines); i++ {
		if first {
			highest = klines[i].High
			lowest = klines[i].Low
			first = false
			continue
		}

		if klines[i].High > highest {
			highest = klines[i].High
		}
		if klines[i].Low < lowest {
			lowest = klines[i].Low
		}
	}

	if lowest == 0 {
		return 0
	}

	return ((highest - lowest) / lowest) * 100
}

// GetDonchianPosition 获取当前价格在通道中的位置（0-1）
func (dc *DonchianCalculator) GetDonchianPosition(price float64, channel *types.DonchianChannel) float64 {
	if channel == nil || channel.Upper == channel.Lower {
		return 0.5
	}

	position := (price - channel.Lower) / (channel.Upper - channel.Lower)

	// 限制在0-1范围内
	if position < 0 {
		position = 0
	} else if position > 1 {
		position = 1
	}

	return position
}

// CalculateChannelWidth 计算通道宽度百分比
func (dc *DonchianCalculator) CalculateChannelWidth(channel *types.DonchianChannel) float64 {
	if channel == nil || channel.Middle == 0 {
		return 0
	}

	return ((channel.Upper - channel.Lower) / channel.Middle) * 100
}

// IsValidBreakout 验证是否为有效突破
func (dc *DonchianCalculator) IsValidBreakout(klines []*types.KLine, channel *types.DonchianChannel, volumeMultiplier float64) bool {
	if len(klines) < 2 || channel == nil {
		return false
	}

	current := klines[len(klines)-1]
	previous := klines[len(klines)-2]

	// 检查突破条件
	isBreakout, direction := dc.CalculateBreakout(klines, channel)
	if !isBreakout {
		return false
	}

	// 检查成交量确认
	volumeConfirmed := current.Volume >= previous.Volume*volumeMultiplier

	// 检查K线形态确认
	var candleConfirmed bool
	if direction == "LONG" {
		// 做多信号：阳线突破上轨
		candleConfirmed = current.Close > current.Open && current.Close > channel.Upper
	} else if direction == "SHORT" {
		// 做空信号：阴线突破下轨
		candleConfirmed = current.Close < current.Open && current.Close < channel.Lower
	}

	return volumeConfirmed && candleConfirmed
}
