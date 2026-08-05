package platform

import (
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/shyunku-libraries/go-logger"
	"net/http"
	"team.gg-server/service/statistics"
	"team.gg-server/types"
	"team.gg-server/util"
	"time"
)

func setStatisticsCacheHeaders(c *gin.Context, key string, updatedAt time.Time) bool {
	etag := fmt.Sprintf("%q", fmt.Sprintf("%s-%d", key, updatedAt.Unix()))
	c.Header("Cache-Control", "public, max-age=300, s-maxage=300, stale-while-revalidate=3600")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return true
	}
	return false
}

func UseStatisticsRouter(r *gin.RouterGroup) {
	g := r.Group("/statistics")

	g.GET("/champion", GetChampionStatistics)
	g.GET("/champion-detail", GetChampionStatisticsDetail)
	g.GET("/meta", GetMetaStatistics)
	g.GET("/meta-summary", GetMetaSummaryStatistics)
	g.GET("/counter", GetCounterStatistics)
	g.GET("/tier", GetTierStatistics)
	g.GET("/mastery", GetMasteryStatistics)
}

func GetChampionStatistics(c *gin.Context) {
	data, err := statistics.ChampionDetailStatisticsRepo.Load()
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if data == nil {
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "not found")
		return
	}
	if setStatisticsCacheHeaders(c, "champion", data.UpdatedAt) {
		return
	}

	innerData := make(map[int]GetChampionStatisticsResponseItem)
	if data.Data != nil {
		for k, v := range data.Data {
			innerData[k] = GetChampionStatisticsResponseItem{
				ChampionId:   k,
				ChampionName: v.ChampionName,
				Win:          v.Win,
				Total:        v.Total,
				AvgPickRate:  v.AvgPickRate,
				AvgBanRate:   v.AvgBanRate,
				AvgWinRate:   v.AvgWinRate,
				ExtraStats:   v.ExtraStats,
			}
		}
	}

	c.JSON(http.StatusOK, GetChampionStatisticsResponseDto{
		UpdatedAt: data.UpdatedAt,
		Patches:   data.Patches,
		Data:      innerData,
	})
}

func GetChampionStatisticsDetail(c *gin.Context) {
	var req GetChampionStatisticsDetailRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	data, err := statistics.ChampionDetailStatisticsRepo.Load()
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if data == nil {
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "statistics cache is not ready")
		return
	}

	championDetail, exists := data.Data[req.ChampionId]
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "champion not found")
		return
	}

	c.JSON(http.StatusOK, championDetail)
}

func GetMetaStatistics(c *gin.Context) {
	data, err := statistics.ChampionDetailStatisticsRepo.Load()
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if data == nil {
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "statistics cache is not ready")
		return
	}
	if setStatisticsCacheHeaders(c, "meta", data.UpdatedAt) {
		return
	}
	c.JSON(http.StatusOK, data)
}

func GetMetaSummaryStatistics(c *gin.Context) {
	meta, err := statistics.ChampionDetailStatisticsRepo.LoadMeta()
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if meta == nil {
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "statistics cache is not ready")
		return
	}
	if setStatisticsCacheHeaders(c, "meta-summary", meta.UpdatedAt) {
		return
	}
	c.JSON(http.StatusOK, meta)
}

func GetCounterStatistics(c *gin.Context) {
	var req GetCounterStatisticsRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	data, err := statistics.ChampionDetailStatisticsRepo.Load()
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if data == nil {
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "cache not found")
		return
	}
	if data.Data == nil {
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "data not found")
		return
	}

	championData, exists := data.Data[req.ChampionId]
	if !exists {
		util.AbortWithStrJson(c, http.StatusInternalServerError, "champion data not found")
		return
	}

	var stat *statistics.ChampionDetailStatisticsMetaTree
	if req.TeamPosition == types.TeamPositionTop {
		stat = championData.MetaTree.Top
	} else if req.TeamPosition == types.TeamPositionJungle {
		stat = championData.MetaTree.Jungle
	} else if req.TeamPosition == types.TeamPositionMid {
		stat = championData.MetaTree.Mid
	} else if req.TeamPosition == types.TeamPositionAdc {
		stat = championData.MetaTree.Adc
	} else if req.TeamPosition == types.TeamPositionSupport {
		stat = championData.MetaTree.Support
	}

	if stat == nil {
		util.AbortWithStrJson(c, http.StatusNotFound, "stat not found")
		return
	}

	counter, exists := stat.CounterMap[req.CounterChampionId]
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "counter not found")
		return
	}

	c.JSON(http.StatusOK, counter)
}

func GetTierStatistics(c *gin.Context) {
	statistics, err := statistics.TierStatisticsRepo.Load()
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if statistics == nil {
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "statistics cache is not ready")
		return
	}

	c.JSON(http.StatusOK, statistics)
}

func GetMasteryStatistics(c *gin.Context) {
	statistics, err := statistics.MasteryStatisticsRepo.Load()
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if statistics == nil {
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "statistics cache is not ready")
		return
	}

	c.JSON(http.StatusOK, statistics)
}
