package v1

import (
	"github.com/gin-gonic/gin"
	log "github.com/shyunku-libraries/go-logger"
	"net/http"
	api2 "team.gg-server/controllers/v1/api"
	"team.gg-server/controllers/v1/platform"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/service"
	"team.gg-server/third_party/riot/api"
	"team.gg-server/types"
	"team.gg-server/util"
	"time"
)

func UseV1Router(r *gin.Engine) {
	g := r.Group("/v1")
	UseIconRouter(g)
	UseAuthRouter(g)
	platform.UsePlatformRouter(g)
	api2.UseApiRouter(g)

	g.GET("/summoner", GetSummonerInfo)
	g.GET("/summoner-by-puuid", GetSummonerInfoByPuuid)
	g.GET("/matches", GetMatches)
	g.GET("/quickSearch", QuickSearchSummoner)
	g.POST("/renewSummoner", RenewSummonerInfo)
	g.POST("/loadMatches", LoadMatches)
	g.GET("/ingame", GetIngameInfo)
}

func GetSummonerInfo(c *gin.Context) {
	var req GetSummonerInfoRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	tagLine := "KR1"
	if req.TagLine != nil {
		tagLine = *req.TagLine
	}

	if req.GameName == "" {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid game name")
		return
	}

	summonerDAO, exists, err := models.GetSummonerDAO_byNameTag(db.Root, req.GameName, tagLine)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		account, status, err := api.GetAccountByRiotId(req.GameName, tagLine)
		if err != nil {
			if status == http.StatusNotFound {
				util.AbortWithStrJson(c, http.StatusNotFound, "invalid game name: "+req.GameName+" "+tagLine)
				return
			}
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusBadRequest, "internal server error")
			return
		}

		if err := service.RenewSummonerTotal(account.Puuid); err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		// retry
		summonerDAO, exists, err = models.GetSummonerDAO_byNameTag(db.Root, req.GameName, tagLine)
		if err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
		if !exists {
			util.AbortWithStrJson(c, http.StatusBadRequest, "invalid summoner name")
			return
		}
	}

	// configure VOs
	summaryVO, err := service.GetSummonerSummaryVO_byPuuid(summonerDAO.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	soloRankVO, err := service.GetSummonerRankVO(summonerDAO.Puuid, types.RankTypeSolo)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	flexRankVO, err := service.GetSummonerRankVO(summonerDAO.Puuid, types.RankTypeFlex)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	masteryVOs, err := service.GetSummonerMasteryVOs(summonerDAO.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	matchesVOs, err := service.GetSummonerRecentMatchSummaryVOs_byQueueId(summonerDAO.Puuid, types.QueueTypeAll, service.GetInitialMatchCount())
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	extraVO, err := service.GetSummonerExtraVO(summonerDAO.Puuid, soloRankVO)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := GetSummonerInfoResponseDto{
		Summary:  *summaryVO,
		SoloRank: soloRankVO,
		FlexRank: flexRankVO,
		Mastery:  masteryVOs,
		Matches:  matchesVOs,
		Extra:    *extraVO,
	}

	c.JSON(http.StatusOK, resp)
}

func GetSummonerInfoByPuuid(c *gin.Context) {
	var req GetSummonerInfoByPuuidRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	summonerDAO, exists, err := models.GetSummonerDAO_byPuuid(db.Root, req.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		// find summoner by puuid on riot
		summonerDAO, exists, err = service.RenewSummonerInfoByPuuid(db.Root, req.Puuid)
		if err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
		if !exists {
			util.AbortWithStrJson(c, http.StatusNotFound, "account/summoner not found")
			return
		}
	}

	c.JSON(http.StatusOK, summonerDAO)
}

func GetMatches(c *gin.Context) {
	var req GetMatchesRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	var queueType int
	if req.QueueId == nil {
		queueType = types.QueueTypeAll
	} else {
		queueType = *req.QueueId
	}

	matchesVOs, err := service.GetSummonerRecentMatchSummaryVOs_byQueueId(req.Puuid, queueType, service.GetInitialMatchCount())
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if len(matchesVOs) < 20 {
		if err := service.RenewSummonerMatches(db.Root, req.Puuid, &api.MatchIdsReqOption{
			Count:   service.GetInitialMatchCount(),
			QueueId: queueType,
		}); err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		matchesVOs, err = service.GetSummonerRecentMatchSummaryVOs_byQueueId(req.Puuid, queueType, service.GetInitialMatchCount())
		if err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	c.JSON(http.StatusOK, matchesVOs)
}

func QuickSearchSummoner(c *gin.Context) {
	var req QuickSearchSummonerRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	summonerDAOs, err := models.FindSummonerDAO_byKeyword(db.Root, req.Keyword, 5)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]service.SummonerSummaryVO, 0)
	for _, summonerDAO := range summonerDAOs {
		summaryVO, err := service.GetSummonerSummaryVO_byPuuid(summonerDAO.Puuid)
		if err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
		resp = append(resp, *summaryVO)
	}

	c.JSON(http.StatusOK, resp)
}

func RenewSummonerInfo(c *gin.Context) {
	var req RenewSummonerInfoRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	if err := service.RenewSummonerTotal(req.Puuid); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, nil)
}

func LoadMatches(c *gin.Context) {
	var req LoadMatchesRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	var queueId int
	if req.QueueId == nil {
		queueId = types.QueueTypeAll
	} else {
		queueId = *req.QueueId
	}

	_, exists, err := models.GetOldestSummonerMatchDAO(db.Root, req.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		// just renew matches (recent)
		if err := service.RenewSummonerMatches(db.Root, req.Puuid, &api.MatchIdsReqOption{
			QueueId: queueId,
		}); err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	} else {
		// renew matches (before requested time)
		beforeTime := time.UnixMilli(*req.Before)
		if err := service.RenewSummonerMatches(db.Root, req.Puuid, &api.MatchIdsReqOption{
			QueueId: queueId,
			Count:   service.GetLoadMoreMatchCount(),
			EndTime: &beforeTime,
		}); err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if req.Before == nil {
		now := time.Now().UnixMilli()
		req.Before = &now
	}

	resp, err := service.GetSummonerMatchSummaryVOs_byQueueId_before(req.Puuid, queueId, *req.Before, service.GetLoadMoreMatchCount())
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, resp)
}

func GetIngameInfo(c *gin.Context) {
	var req GetIngameInfoRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	summonerDAO, exists, err := models.GetSummonerDAO_byPuuid(db.Root, req.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid puuid")
		return
	}

	spectatorInfo, status, err := api.GetSpectatorInfoV5(summonerDAO.Puuid)
	if err != nil {
		if status == http.StatusNotFound {
			util.AbortWithStrJson(c, http.StatusNotFound, "not in game")
			return
		}
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	team1 := make([]service.IngameParticipantVO, 0)
	team2 := make([]service.IngameParticipantVO, 0)
	for _, participant := range spectatorInfo.Participants {
		if participant.TeamId == 100 {
			team1 = append(team1, service.IngameParticipantMixer(participant))
		} else {
			team2 = append(team2, service.IngameParticipantMixer(participant))
		}
	}

	resp := GetIngameInfoResponseDto{
		GameType:          spectatorInfo.GameType,
		MapId:             spectatorInfo.MapId,
		GameStartTime:     spectatorInfo.GameStartTime,
		GameMode:          spectatorInfo.GameMode,
		GameQueueConfigId: spectatorInfo.GameQueueConfigId,
		Team1:             team1,
		Team2:             team2,
	}

	c.JSON(http.StatusOK, resp)
}
