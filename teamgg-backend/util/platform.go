package util

import (
	"math"
	"strings"
)

func ShortenSummonerName(name string) string {
	// delete space
	name = strings.ReplaceAll(name, " ", "")
	// to lowercase
	name = strings.ToLower(name)
	return name
}

// LogisticNormalize indicates 0.5 when $x is $factor
func LogisticNormalize(x float64, factor float64) float64 {
	return 1 / (1 + (x / factor))
}

func EuclideanDistance(x1, x2 float64) float64 {
	return math.Sqrt(x1*x1 - x2*x2)
}

func EuclideanMean(x1, x2 float64) float64 {
	return math.Sqrt(x1*x1 + x2*x2)
}

// 0~1 -> 0~inf
func PolynomialToInfiniteScale(x float64) float64 {
	return math.Tan(x * math.Pi / 2)
}

// 0~inf -> 0~1, when factor goes high, it goes close to 0
// InfiniteToPolynomialScale indicates 0.5 when $x is $factor
func InfiniteToPolynomialScale(x float64, factor float64) float64 {
	return math.Atan(x/factor) * 2 / math.Pi
}
