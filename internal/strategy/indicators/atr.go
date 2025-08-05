package indicators

import (
	"math"
	"okx-market-sentry/pkg/types"
)

// ATRCalculator ATR指标计算器
type ATRCalculator struct {
	length int
}

// NewATRCalculator 创建ATR计算器
func NewATRCalculator(length int) *ATRCalculator {
	return &ATRCalculator{
		length: length,
	}
}

// Calculate 计算ATR值
func (ac *ATRCalculator) Calculate(klines []*types.KLine) *types.ATRData {
	if len(klines) < ac.length+1 {
		return nil
	}

	// 计算真实波幅序列
	trValues := ac.calculateTrueRange(klines)
	if len(trValues) < ac.length {
		return nil
	}

	// 计算ATR值（真实波幅的移动平均）
	atrValue := ac.calculateSMA(trValues[len(trValues)-ac.length:])

	// 计算ATR斜率（最近45个ATR值的线性回归斜率）
	atrSlope := ac.calculateATRSlope(klines)

	return &types.ATRData{
		Value: atrValue,
		Slope: atrSlope,
	}
}

// calculateTrueRange 计算真实波幅序列
func (ac *ATRCalculator) calculateTrueRange(klines []*types.KLine) []float64 {
	if len(klines) < 2 {
		return nil
	}

	var trValues []float64

	for i := 1; i < len(klines); i++ {
		current := klines[i]
		previous := klines[i-1]

		// 真实波幅 = max(high-low, |high-prevClose|, |low-prevClose|)
		hl := current.High - current.Low
		hc := math.Abs(current.High - previous.Close)
		lc := math.Abs(current.Low - previous.Close)

		tr := math.Max(hl, math.Max(hc, lc))
		trValues = append(trValues, tr)
	}

	return trValues
}

// calculateSMA 计算简单移动平均
func (ac *ATRCalculator) calculateSMA(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, value := range values {
		sum += value
	}

	return sum / float64(len(values))
}

// calculateATRSlope 计算ATR斜率
func (ac *ATRCalculator) calculateATRSlope(klines []*types.KLine) float64 {
	// 需要足够的数据来计算45个ATR值的斜率
	requiredBars := 45 + ac.length
	if len(klines) < requiredBars {
		return 0
	}

	var atrValues []float64

	// 计算最近45个ATR值
	for i := len(klines) - 45; i <= len(klines)-1; i++ {
		if i-ac.length < 0 {
			continue
		}

		// 计算当前位置的ATR值
		trValues := ac.calculateTrueRange(klines[i-ac.length : i+1])
		if len(trValues) >= ac.length {
			atrValue := ac.calculateSMA(trValues[len(trValues)-ac.length:])
			atrValues = append(atrValues, atrValue)
		}
	}

	if len(atrValues) < 10 { // 至少需要10个点来计算斜率
		return 0
	}

	// 使用线性回归计算斜率
	return ac.calculateLinearRegressionSlope(atrValues)
}

// calculateLinearRegressionSlope 计算线性回归斜率
func (ac *ATRCalculator) calculateLinearRegressionSlope(values []float64) float64 {
	n := float64(len(values))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2 float64

	for i, y := range values {
		x := float64(i + 1)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 斜率 = (n*∑xy - ∑x*∑y) / (n*∑x² - (∑x)²)
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	slope := (n*sumXY - sumX*sumY) / denominator
	return slope
}

// IsATRDecreasing 判断ATR是否呈下降趋势
func (ac *ATRCalculator) IsATRDecreasing(atrData *types.ATRData, klines []*types.KLine) bool {
	if atrData == nil {
		return false
	}

	// 方法1：检查斜率是否为负
	if atrData.Slope < 0 {
		return true
	}

	// 方法2：检查当前ATR是否处于最低25%分位
	return ac.isATRInLowestQuartile(atrData.Value, klines)
}

// isATRInLowestQuartile 检查当前ATR是否处于最低25%分位
func (ac *ATRCalculator) isATRInLowestQuartile(currentATR float64, klines []*types.KLine) bool {
	// 需要足够的历史数据
	requiredBars := 45 + ac.length
	if len(klines) < requiredBars {
		return false
	}

	var atrValues []float64

	// 计算最近45个ATR值
	for i := len(klines) - 45; i <= len(klines)-1; i++ {
		if i-ac.length < 0 {
			continue
		}

		trValues := ac.calculateTrueRange(klines[i-ac.length : i+1])
		if len(trValues) >= ac.length {
			atrValue := ac.calculateSMA(trValues[len(trValues)-ac.length:])
			atrValues = append(atrValues, atrValue)
		}
	}

	if len(atrValues) < 4 { // 至少需要4个值来计算分位数
		return false
	}

	// 排序ATR值
	sortedATR := make([]float64, len(atrValues))
	copy(sortedATR, atrValues)
	ac.quickSort(sortedATR, 0, len(sortedATR)-1)

	// 计算25%分位数
	index := int(float64(len(sortedATR)) * 0.25)
	if index >= len(sortedATR) {
		index = len(sortedATR) - 1
	}

	percentile25 := sortedATR[index]

	return currentATR <= percentile25
}

// quickSort 快速排序
func (ac *ATRCalculator) quickSort(arr []float64, low, high int) {
	if low < high {
		pi := ac.partition(arr, low, high)
		ac.quickSort(arr, low, pi-1)
		ac.quickSort(arr, pi+1, high)
	}
}

// partition 分区函数
func (ac *ATRCalculator) partition(arr []float64, low, high int) int {
	pivot := arr[high]
	i := low - 1

	for j := low; j < high; j++ {
		if arr[j] <= pivot {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}
	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}

// GetATRPercentile 获取当前ATR在历史ATR中的百分位
func (ac *ATRCalculator) GetATRPercentile(currentATR float64, klines []*types.KLine) float64 {
	requiredBars := 45 + ac.length
	if len(klines) < requiredBars {
		return 50 // 默认50%分位
	}

	var atrValues []float64

	// 计算历史ATR值
	for i := len(klines) - 45; i <= len(klines)-1; i++ {
		if i-ac.length < 0 {
			continue
		}

		trValues := ac.calculateTrueRange(klines[i-ac.length : i+1])
		if len(trValues) >= ac.length {
			atrValue := ac.calculateSMA(trValues[len(trValues)-ac.length:])
			atrValues = append(atrValues, atrValue)
		}
	}

	if len(atrValues) == 0 {
		return 50
	}

	// 计算当前ATR在历史ATR中的排名
	rank := 0
	for _, atr := range atrValues {
		if currentATR > atr {
			rank++
		}
	}

	return (float64(rank) / float64(len(atrValues))) * 100
}

// CalculateATRNormalized 计算标准化ATR值
func (ac *ATRCalculator) CalculateATRNormalized(atrValue, currentPrice float64) float64 {
	if currentPrice == 0 {
		return 0
	}
	return (atrValue / currentPrice) * 100
}
