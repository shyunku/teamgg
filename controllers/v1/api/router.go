package api

import (
	"github.com/gin-gonic/gin"
	log "github.com/shyunku-libraries/go-logger"
	"net/http"
	"team.gg-server/controllers/socket"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/service"
	"team.gg-server/third_party/riot/api"
	"team.gg-server/util"
)

func UseApiRouter(r *gin.RouterGroup) {
	g := r.Group("/api")

	g.GET("/summonerPuuid", getSummonerPuuid)
	g.POST("/summonerLineFavor", setSummonerLineFavor)
	g.GET("/discordIntegrations", getDiscordIntegrations)
}

func getSummonerPuuid(c *gin.Context) {
	var req GetSummonerPuuidRequestDto
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

	var puuid string
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
				util.AbortWithStrJson(c, http.StatusNotFound, "summoner not found")
			} else {
				log.Error(err)
				util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			}
			return
		}
		puuid = account.Puuid
	} else {
		puuid = summonerDAO.Puuid
	}

	c.JSON(http.StatusOK, puuid)
}

func setSummonerLineFavor(c *gin.Context) {
	var req SetSummonerLineFavorRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	lockedConfigs := make(map[string]bool)
	defer func() {
		for configId := range lockedConfigs {
			service.EndCustomGameConfigurationMutation(configId)
		}
	}()

	if len(req.Strengths) != 5 {
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid line favor length")
		return
	}

	for _, strength := range req.Strengths {
		if strength < -1 || strength > 2 {
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusBadRequest, "invalid strength")
			return
		}
	}

	// get integrations
	discordIntegrations, err := models.GetThirdPartyIntegrationDAOs_byPlatformAndToken(db.Root, "discord", req.UserId)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	customGameConfigIdList := make([]string, 0)
	for _, discordIntegration := range discordIntegrations {
		candidateDAOs, err := models.GetCustomGameCandidateDAOs_byPuuid(tx, discordIntegration.Puuid)
		if err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		for _, candidateDAO := range candidateDAOs {
			if !lockedConfigs[candidateDAO.CustomGameConfigId] {
				if !service.TryBeginCustomGameConfigurationMutation(candidateDAO.CustomGameConfigId) {
					_ = tx.Rollback()
					util.AbortWithStrJson(c, http.StatusLocked, "최적의 조합을 계산 중이라 내전 구성을 변경할 수 없습니다.")
					return
				}
				lockedConfigs[candidateDAO.CustomGameConfigId] = true
			}
			// update candidate
			candidateDAO.FlavorTop = req.Strengths[0]
			candidateDAO.FlavorJungle = req.Strengths[1]
			candidateDAO.FlavorMid = req.Strengths[2]
			candidateDAO.FlavorAdc = req.Strengths[3]
			candidateDAO.FlavorSupport = req.Strengths[4]

			if err := candidateDAO.Upsert(tx); err != nil {
				log.Error(err)
				_ = tx.Rollback()
				util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
				return
			}

			if err := service.RecalculateCustomGameBalance(tx, candidateDAO.CustomGameConfigId); err != nil {
				log.Error(err)
				_ = tx.Rollback()
				util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
				return
			}

			customGameConfigIdList = append(customGameConfigIdList, candidateDAO.CustomGameConfigId)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	for _, customGameConfigId := range customGameConfigIdList {
		socket.SocketIO.BroadcastToCustomConfigRoom(customGameConfigId, socket.EventCustomConfigUpdated, nil)
	}
	c.JSON(http.StatusOK, nil)
}

func getDiscordIntegrations(c *gin.Context) {
	var req GetDiscordIntegrationsRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Token == "" {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid token")
		return
	}

	discordIntegrations, err := models.GetThirdPartyIntegrationDAOs_byPlatformAndToken(db.Root, "discord", req.Token)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, discordIntegrations)
}
