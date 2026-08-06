package riot

import (
	"os"
	"time"
)

var (
	apiKey          string
	ApiCalls        int
	LastApiCallTime time.Time
)

func Init() {
	apiKey = os.Getenv("RIOT_API_KEY")
	ApiCalls = 0
	LastApiCallTime = time.Now().Add(-24 * time.Hour)
}

func UpdateRiotApiCalls() {
	ApiCalls++
	LastApiCallTime = time.Now()
}
