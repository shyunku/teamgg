package platform

import (
	"team.gg-server/service/statistics"
	"time"
)

type GetChampionStatisticsResponseItem struct {
	ChampionId   int    `json:"championId"`
	ChampionName string `json:"championName"`

	Win         int     `json:"win"`
	Total       int     `json:"total"`
	AvgPickRate float64 `json:"avgPickRate"`
	AvgBanRate  float64 `json:"avgBanRate"`
	AvgWinRate  float64 `json:"avgWinRate"`

	ExtraStats statistics.ChampionDetailStatisticsExtraStats `json:"extraStats"`
}

type GetChampionStatisticsResponseDto struct {
	UpdatedAt time.Time                                 `json:"updatedAt"`
	Patches   []string                                  `json:"patches"`
	Data      map[int]GetChampionStatisticsResponseItem `json:"data"`
}

type GetChampionStatisticsDetailRequestDto struct {
	ChampionId int `form:"championId" binding:"required"`
}

type GetCounterStatisticsRequestDto struct {
	ChampionId        int    `form:"championId" binding:"required"`
	TeamPosition      string `form:"teamPosition" binding:"required"`
	CounterChampionId int    `form:"counterChampionId" binding:"required"`
}
