package service

import "math"

func PredictMatchMakingRating(ratingPoint float64, avgGGScore float64) float64 {
	// convert rp (0~3700) to 800~3000
	convertedRP := (ratingPoint/3700)*2200 + 800
	// convert ggScore (0~100+) to -x?~+x?
	var ggPoint float64
	convertedGGScore := avgGGScore - 30 // -30~70+
	if convertedGGScore >= 0 {
		ggPoint = math.Pow(convertedGGScore, 1.4) // 70 -> about +382
	} else {
		ggPoint = -math.Pow(-convertedGGScore, 1.5) // -30 -> about -164
	}

	mmr := convertedRP + ggPoint
	return mmr
}

func MMRtoRatingPoint(mmr float64) float64 {
	// convert mmr to 0~3700
	convertedMMR := ((mmr - 800) / 2200) * 3700
	return convertedMMR
}
