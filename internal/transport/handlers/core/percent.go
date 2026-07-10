package core

import (
	"math"
)

// PPMToPercent возвращает float64 с точностью до 3 знаков после запятой.
func PPMToPercent(ppm int64) float64 {
	val := float64(ppm) / 10000.0

	return math.Round(val*1000) / 1000
}

// PercentToPPM конвертирует проценты в PPM
func PercentToPPM(percent float64) int64 {
	return int64(math.Round(percent * 10000.0))
}
