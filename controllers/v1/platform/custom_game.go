package platform

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/shyunku-libraries/go-logger"
	"math/rand"
	"net/http"
	"sort"
	"team.gg-server/controllers/socket"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/service"
	"team.gg-server/third_party/riot/api"
	"team.gg-server/types"
	"team.gg-server/util"
	"time"
)

func UseCustomGameRouter(r *gin.RouterGroup) {
	g := r.Group("/custom-game")

	g.GET("/list", GetCustomGameConfigurationList)
	g.GET("/info", GetCustomGameConfiguration)
	g.POST("/create", CreateCustomGameConfiguration)

	g.GET("/tier-rank", GetTierRank)
	g.GET("/balance", GetCustomConfigurationBalance)

	g.PUT("/candidate", AddCandidateToCustomGameConfiguration)
	g.DELETE("/candidate", DeleteCandidateFromCustomGameConfiguration)

	g.POST("/arrange", ArrangeCustomGameParticipant)
	g.POST("/unarrange", UnarrangeCustomGameParticipant)
	g.POST("/favor-position", SetCustomGameParticipantFavorPosition)
	g.POST("/custom-tier-rank", SetCustomGameCandidateCustomTierRank)
	g.POST("/custom-color-label", SetCustomGameCandidateCustomColorLabel)
	g.DELETE("/custom-color-label", DeleteCustomGameCandidateCustomColorLabel)
	g.POST("/optimize", OptimizeCustomGameConfiguration)

	g.POST("/arrange-all", SelectMaxCandidates)
	g.POST("/unarrange-all", UnarrangeAllParticipants)
	g.POST("/swap-team", SwapTeam)
	g.POST("/shuffle", ShuffleTeam)
	g.POST("/renew-ranks", RenewRanks)
}

func GetCustomGameConfigurationList(c *gin.Context) {
	uid := c.GetString("uid")

	if uid == "" {
		log.Warn("uid is empty")
		util.AbortWithStrJson(c, http.StatusUnauthorized, "user not found")
		return
	}

	// get all custom games from db
	resp, err := service.GetCustomGameConfigurationVOs(uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusOK, resp)
}

func GetCustomGameConfiguration(c *gin.Context) {
	var req GetCustomGameConfigurationRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	resp, err := service.GetCustomGameConfigurationVO(req.Id)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, resp)
}

func CreateCustomGameConfiguration(c *gin.Context) {
	uid := c.GetString("uid")

	// get all custom games from db
	customGameConfigurationDAOs, err := models.GetCustomGameDAOs_byCreatorUid(db.Root, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	userDAO, exists, err := models.GetUserDAO_byUid(db.Root, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		log.Errorf("user not found: %s", uid)
		util.AbortWithStrJson(c, http.StatusForbidden, "user not found")
		return
	}

	var name string
	namePrefix := fmt.Sprintf("%s의 내전 팀 구성", userDAO.UserId)
	nameSuffix := 1

	configMapByName := make(map[string]bool)
	for _, customGameConfigurationDAO := range customGameConfigurationDAOs {
		configMapByName[customGameConfigurationDAO.Name] = true
	}

	for {
		name = fmt.Sprintf("%s %d", namePrefix, nameSuffix)
		if _, exists := configMapByName[name]; !exists {
			break
		}
		nameSuffix++
	}

	// create custom game configuration
	newId := uuid.New().String()
	now := time.Now()
	newCustomGameConfigurationDAO := models.CustomGameConfigurationDAO{
		Id:                     newId,
		Name:                   name,
		CreatorUid:             uid,
		CreatedAt:              now,
		LastUpdatedAt:          now,
		Fairness:               0,
		LineFairness:           0,
		TierFairness:           0,
		LineSatisfaction:       0,
		LineFairnessWeight:     types.WeightLineFairness,
		TierFairnessWeight:     types.WeightTierFairness,
		LineSatisfactionWeight: types.WeightLineSatisfaction,
		TopInfluenceWeight:     types.WeightTopInfluence,
		JungleInfluenceWeight:  types.WeightJungleInfluence,
		MidInfluenceWeight:     types.WeightMidInfluence,
		AdcInfluenceWeight:     types.WeightAdcInfluence,
		SupportInfluenceWeight: types.WeightSupportInfluence,
	}
	if err := newCustomGameConfigurationDAO.Upsert(db.Root); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, newId)
}

func GetTierRank(c *gin.Context) {
	var req GetTierRankRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	if req.RatingPoint == nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid rating point")
		return
	}

	tier, rank, lp, err := service.CalculateTierRank(*req.RatingPoint)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, GetTierRankResponseDto{
		Tier: string(tier),
		Rank: string(rank),
		Lp:   int64(lp),
	})
}

func GetCustomConfigurationBalance(c *gin.Context) {
	var req GetCustomConfigurationBalanceRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	resp, err := service.GetCustomGameConfigurationBalanceVO(req.Id)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, resp)
}

func AddCandidateToCustomGameConfiguration(c *gin.Context) {
	var req AddCandidateToCustomGameRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	customGameConfigurationDAO, exists, err := models.GetCustomGameDAO_byId(db.Root, req.CustomGameConfigId)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "custom game configuration not found")
		return
	}
	if customGameConfigurationDAO.CreatorUid != uid {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	// get summoner
	summonerDAO, exists, err := models.GetSummonerDAO_byNameTag(db.Root, req.Name, req.TagLine)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		// need to renew summoner
		tx, err := db.Root.BeginTxx(c, nil)
		if err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		account, status, err := api.GetAccountByRiotId(req.Name, req.TagLine)
		if err != nil {
			if status == http.StatusNotFound {
				util.AbortWithStrJson(c, http.StatusNotFound, "invalid game name")
				return
			}
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusBadRequest, "internal server error")
			return
		}

		if err := service.RenewSummonerTotal(tx, account.Puuid); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		if err := tx.Commit(); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		// retry
		summonerDAO, exists, err = models.GetSummonerDAO_byNameTag(db.Root, req.Name, req.TagLine)
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

	// load summoner completed
	// check if candidate already exists
	candidateDAOs, err := models.GetCustomGameCandidateDAOs_byCustomGameConfigId(db.Root, req.CustomGameConfigId)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	for _, candidateDAO := range candidateDAOs {
		if candidateDAO.Puuid == summonerDAO.Puuid {
			util.AbortWithStrJson(c, http.StatusConflict, "candidate already exists")
			return
		}
	}

	// add candidate
	newCandidateDAO := models.CustomGameCandidateDAO{
		CustomGameConfigId: req.CustomGameConfigId,
		Puuid:              summonerDAO.Puuid,
		CustomTier:         nil,
		CustomRank:         nil,
		FlavorTop:          0,
		FlavorJungle:       0,
		FlavorMid:          0,
		FlavorAdc:          0,
		FlavorSupport:      0,
	}
	if err := newCandidateDAO.Upsert(db.Root); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	candidateVO, err := service.GetCustomGameCandidateVO(newCandidateDAO)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.CustomGameConfigId, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, candidateVO)
}

func DeleteCandidateFromCustomGameConfiguration(c *gin.Context) {
	var req DeleteCandidateFromCustomGameRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	customGameConfigurationDAO, exists, err := models.GetCustomGameDAO_byId(db.Root, req.CustomGameConfigId)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "custom game configuration not found")
		return
	}
	if customGameConfigurationDAO.CreatorUid != uid {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	// delete candidate
	if err := models.DeleteCustomGameCandidateDAO_byPuuid(db.Root, req.CustomGameConfigId, req.Puuid); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.BroadcastToCustomConfigRoom(req.CustomGameConfigId, socket.EventCustomConfigUpdated, nil)

	c.JSON(http.StatusOK, nil)
}

func ArrangeCustomGameParticipant(c *gin.Context) {
	var req ArrangeCustomGameParticipantRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	customGameConfigurationDAO, exists, err := models.GetCustomGameDAO_byId(db.Root, req.CustomGameConfigId)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "custom game configuration not found")
		return
	}
	if customGameConfigurationDAO.CreatorUid != uid {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	// validate request
	if req.Team != 1 && req.Team != 2 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid team")
		return
	}

	if req.TargetPosition != types.PositionTop &&
		req.TargetPosition != types.PositionJungle &&
		req.TargetPosition != types.PositionMid &&
		req.TargetPosition != types.PositionAdc &&
		req.TargetPosition != types.PositionSupport {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid target position")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// check if candidate exists in config
	_, exists, err = models.GetCustomGameCandidateDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusBadRequest, "candidate not found")
		return
	}

	// check if candidate exists as participants
	srcParticipantDAO, moveFromParticipant, err := models.GetCustomGameParticipantDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// check if candidate exists in same place
	destParticipantDAO, exists, err := models.GetCustomGameParticipantDAO_byPosition(tx, req.CustomGameConfigId, req.Team, req.TargetPosition)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if exists {
		// delete participant
		if err := models.DeleteCustomGameParticipantDAO_byPuuid(tx, req.CustomGameConfigId, destParticipantDAO.Puuid); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		if moveFromParticipant {
			// src.participant -> dest.participant (swap)
			destParticipantDAO.Team = srcParticipantDAO.Team
			destParticipantDAO.Position = srcParticipantDAO.Position
			if err := destParticipantDAO.Upsert(tx); err != nil {
				log.Error(err)
				_ = tx.Rollback()
				util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
				return
			}
		}
	}

	if moveFromParticipant {
		// participant -> participant
		srcParticipantDAO.Team = req.Team
		srcParticipantDAO.Position = req.TargetPosition
		if err := srcParticipantDAO.Upsert(tx); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	} else {
		// candidate -> participant
		// add participant
		newParticipantDAO := models.CustomGameParticipantDAO{
			CustomGameConfigId: req.CustomGameConfigId,
			Puuid:              req.Puuid,
			Team:               req.Team,
			Position:           req.TargetPosition,
		}
		if err := newParticipantDAO.Upsert(tx); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if err := service.RecalculateCustomGameBalance(tx, req.CustomGameConfigId); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.CustomGameConfigId, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func UnarrangeCustomGameParticipant(c *gin.Context) {
	var req UnarrangeCustomGameParticipantRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	customGameConfigurationDAO, exists, err := models.GetCustomGameDAO_byId(db.Root, req.CustomGameConfigId)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "custom game configuration not found")
		return
	}
	if customGameConfigurationDAO.CreatorUid != uid {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := models.DeleteCustomGameParticipantDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := models.DeleteCustomGameParticipantColorLabelDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := service.RecalculateCustomGameBalance(tx, req.CustomGameConfigId); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.CustomGameConfigId, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func SetCustomGameParticipantFavorPosition(c *gin.Context) {
	var req SetCustomGameParticipantFavorPositionRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	permitted, err := service.CheckPermissionForCustomGameConfig(db.Root, req.CustomGameConfigId, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// check if candidate exists in config
	candidateDAO, exists, err := models.GetCustomGameCandidateDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusBadRequest, "candidate not found")
		return
	}

	if req.Strength == nil {
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid enabled")
		return
	}

	if *req.Strength < -1 || *req.Strength > 2 {
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid strength")
		return
	}

	// update candidate
	if req.FavorPosition == types.PositionTop {
		candidateDAO.FlavorTop = *req.Strength
	} else if req.FavorPosition == types.PositionJungle {
		candidateDAO.FlavorJungle = *req.Strength
	} else if req.FavorPosition == types.PositionMid {
		candidateDAO.FlavorMid = *req.Strength
	} else if req.FavorPosition == types.PositionAdc {
		candidateDAO.FlavorAdc = *req.Strength
	} else if req.FavorPosition == types.PositionSupport {
		candidateDAO.FlavorSupport = *req.Strength
	} else {
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid target position")
		return
	}

	if err := candidateDAO.Upsert(tx); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := service.RecalculateCustomGameBalance(tx, req.CustomGameConfigId); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.CustomGameConfigId, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func SetCustomGameCandidateCustomTierRank(c *gin.Context) {
	var req SetCustomGameCandidateCustomTierRankRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	permitted, err := service.CheckPermissionForCustomGameConfig(db.Root, req.CustomGameConfigId, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// check if candidate exists in config
	candidateDAO, exists, err := models.GetCustomGameCandidateDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusBadRequest, "candidate not found")
		return
	}

	if req.Tier == nil && req.Rank == nil {
		candidateDAO.CustomTier = nil
		candidateDAO.CustomRank = nil
	} else {
		if req.Tier == nil || req.Rank == nil {
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusBadRequest, "invalid tier rank: one of them is nil")
			return
		}

		if !service.IsValidTierRank(*req.Tier, *req.Rank) {
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusBadRequest, "invalid tier rank")
			return
		}

		// update candidate
		candidateDAO.CustomTier = req.Tier
		candidateDAO.CustomRank = req.Rank
	}

	if err := candidateDAO.Upsert(tx); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := service.RecalculateCustomGameBalance(tx, req.CustomGameConfigId); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.CustomGameConfigId, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func SetCustomGameCandidateCustomColorLabel(c *gin.Context) {
	var req SetCustomGameCandidateCustomColorLabelRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	permitted, err := service.CheckPermissionForCustomGameConfig(db.Root, req.CustomGameConfigId, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// check if candidate exists in config
	_, exists, err := models.GetCustomGameCandidateDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusBadRequest, "candidate not found")
		return
	}

	if req.ColorCode == nil {
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid color code")
		return

	}

	if *req.ColorCode != 0 {
		// get color labels
		var myColorDAO *models.CustomGameParticipantColorLabelDAO
		colorMap := make(map[string]int)
		colorLabelDAOs, err := models.GetCustomGameParticipantColorLabelDAOs_byCustomGameConfigId(tx, req.CustomGameConfigId)
		if err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
		for _, colorLabelDAO := range colorLabelDAOs {
			if colorLabelDAO.Puuid == req.Puuid {
				myColorDAO = &colorLabelDAO
				colorLabelDAO.ColorCode = *req.ColorCode
				break
			}
		}
		if myColorDAO == nil {
			myColorDAO = &models.CustomGameParticipantColorLabelDAO{
				CustomGameConfigId: req.CustomGameConfigId,
				Puuid:              req.Puuid,
				ColorCode:          *req.ColorCode,
			}
			colorLabelDAOs = append(colorLabelDAOs, *myColorDAO)
		}
		for _, colorLabelDAO := range colorLabelDAOs {
			if colorLabelDAO.ColorCode != 0 {
				if _, exists := colorMap[colorLabelDAO.Puuid]; !exists {
					colorMap[colorLabelDAO.Puuid] = 0
				}
				colorMap[colorLabelDAO.Puuid] = colorLabelDAO.ColorCode
			}
		}

		// get participants
		participantDAOs, err := models.GetCustomGameParticipantDAOs_byCustomGameConfigId(tx, req.CustomGameConfigId)
		if err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		participantColors := make(map[string]int)
		for _, participantDAO := range participantDAOs {
			colorCode, exists := colorMap[participantDAO.Puuid]
			if exists {
				participantColors[participantDAO.Puuid] = colorCode
			} else {
				participantColors[participantDAO.Puuid] = 0
			}
		}

		countMap := make(map[int]int)
		participantWithColor := 0
		for i := 1; i <= 5; i++ {
			countMap[i] = 0
		}
		for _, colorCode := range participantColors {
			if colorCode == 0 {
				continue
			}
			countMap[colorCode]++
			participantWithColor++
		}

		counts := make([]int, 0)
		for _, count := range countMap {
			counts = append(counts, count)
		}

		sort.SliceStable(counts, func(i, j int) bool {
			return counts[i] > counts[j]
		})

		// invalid groups
		// 2 2 2 2 2
		// 3 3 3 0 0
		// 3 3 3 1 0
		// 4 2 2 2 0
		// 4 3 3 0 0
		// 4 4 2 0 0

		// if most count 2 groups has over 5, error
		if counts[0] >= 6 {
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusConflict, "color code is already used 5 times")
			return
		} else if counts[0] == 4 {
			if counts[1] == 2 && counts[2] == 2 && counts[3] == 2 {
				_ = tx.Rollback()
				util.AbortWithStrJson(c, http.StatusNotAcceptable, "color code 4,2,2,2 group is invalid")
				return
			} else if counts[1] == 3 && counts[2] == 3 {
				_ = tx.Rollback()
				util.AbortWithStrJson(c, http.StatusNotAcceptable, "color code 4,3,3 group is invalid")
				return
			} else if counts[1] == 4 && counts[2] == 2 {
				_ = tx.Rollback()
				util.AbortWithStrJson(c, http.StatusNotAcceptable, "color code 4,4,2 group is invalid")
				return
			}
		} else if counts[0] == 3 {
			if counts[1] == 3 && counts[2] == 3 {
				_ = tx.Rollback()
				util.AbortWithStrJson(c, http.StatusNotAcceptable, "color code 3,3,3 group is invalid")
				return
			}
		} else if counts[0] == 2 {
			if counts[1] == 2 && counts[2] == 2 && counts[3] == 2 && counts[4] == 2 {
				_ = tx.Rollback()
				util.AbortWithStrJson(c, http.StatusNotAcceptable, "color code 2,2,2,2,2 group is invalid")
				return
			}
		}

		if err := myColorDAO.Upsert(tx); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	} else {
		// delete
		if err := models.DeleteCustomGameParticipantColorLabelDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.CustomGameConfigId, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func DeleteCustomGameCandidateCustomColorLabel(c *gin.Context) {
	var req DeleteCustomGameCandidateCustomColorLabelRequestDto
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	permitted, err := service.CheckPermissionForCustomGameConfig(db.Root, req.CustomGameConfigId, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// delete color labels
	if err := models.DeleteCustomGameParticipantColorLabels_byCustomGameConfigId(tx, req.CustomGameConfigId); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.CustomGameConfigId, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func OptimizeCustomGameConfiguration(c *gin.Context) {
	var req OptimizeCustomGameConfigurationRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// check if user is creator of custom game
	customGameConfigurationDAO, exists, err := models.GetCustomGameDAO_byId(tx, req.Id)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "custom game configuration not found")
		return
	}
	if customGameConfigurationDAO.CreatorUid != uid {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	if req.LineFairnessWeight == nil || *req.LineFairnessWeight < 0 || *req.LineFairnessWeight > 1 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid line fairness weight")
		return
	}
	if req.TopInfluenceWeight == nil || *req.TopInfluenceWeight < 0 || *req.TopInfluenceWeight > 1 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid top influence weight")
		return
	}
	if req.JungleInfluenceWeight == nil || *req.JungleInfluenceWeight < 0 || *req.JungleInfluenceWeight > 1 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid jungle influence weight")
		return
	}
	if req.MidInfluenceWeight == nil || *req.MidInfluenceWeight < 0 || *req.MidInfluenceWeight > 1 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid mid influence weight")
		return
	}
	if req.AdcInfluenceWeight == nil || *req.AdcInfluenceWeight < 0 || *req.AdcInfluenceWeight > 1 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid adc influence weight")
		return
	}

	customGameConfigurationDAO.LineFairnessWeight = *req.LineFairnessWeight
	customGameConfigurationDAO.TierFairnessWeight = *req.TierFairnessWeight
	customGameConfigurationDAO.LineSatisfactionWeight = 1 - *req.LineFairnessWeight - *req.TierFairnessWeight
	customGameConfigurationDAO.TopInfluenceWeight = *req.TopInfluenceWeight
	customGameConfigurationDAO.JungleInfluenceWeight = *req.JungleInfluenceWeight
	customGameConfigurationDAO.MidInfluenceWeight = *req.MidInfluenceWeight
	customGameConfigurationDAO.AdcInfluenceWeight = *req.AdcInfluenceWeight
	customGameConfigurationDAO.SupportInfluenceWeight = 1 - *req.TopInfluenceWeight - *req.JungleInfluenceWeight - *req.MidInfluenceWeight - *req.AdcInfluenceWeight

	if err := customGameConfigurationDAO.Upsert(tx); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	participantVOsMap, err := service.GetCurrentCustomGameTeamParticipantVOMap(tx, req.Id)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	colorLabelDAOs, err := models.GetCustomGameParticipantColorLabelDAOs_byCustomGameConfigId(tx, req.Id)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	colorMap := make(map[string]int)
	for _, colorLabelDAO := range colorLabelDAOs {
		if colorLabelDAO.ColorCode == 0 {
			continue
		}
		if _, exists := colorMap[colorLabelDAO.Puuid]; !exists {
			colorMap[colorLabelDAO.Puuid] = 0
		}
		colorMap[colorLabelDAO.Puuid] = colorLabelDAO.ColorCode
	}

	configWeightsVO := service.CustomGameConfigurationWeightsMixer(*customGameConfigurationDAO)
	optimizedParticipantVOsMap, err := service.FindBalancedCustomGameConfig(req.Id, participantVOsMap, configWeightsVO, colorMap)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// reorganize participants

	participantVOs, err := models.GetCustomGameParticipantDAOs_byCustomGameConfigId(tx, req.Id)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	for _, participantVO := range participantVOs {
		// delete
		if err := participantVO.Delete(tx); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	for _, participantVO := range participantVOs {
		teamParticipantVO, exists := (*optimizedParticipantVOsMap)[participantVO.Puuid]
		if !exists {
			log.Errorf("participant not found in optimized map")
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		participantVO.Team = teamParticipantVO.Team
		participantVO.Position = teamParticipantVO.Position
		if err := participantVO.Upsert(tx); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if err := service.RecalculateCustomGameBalance(tx, req.Id); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.Id, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func SelectMaxCandidates(c *gin.Context) {
	var req UtilityRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	permitted, err := service.CheckPermissionForCustomGameConfig(db.Root, req.Id, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// get all candidates
	candidateDAOs, err := models.GetCustomGameCandidateDAOs_byCustomGameConfigId(tx, req.Id)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// get all participants
	participantDAOs, err := models.GetCustomGameParticipantDAOs_byCustomGameConfigId(tx, req.Id)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	nonParticipantCandidateDAOs := make([]models.CustomGameCandidateDAO, 0)
	for _, candidateDAO := range candidateDAOs {
		exists := false
		for _, participantDAO := range participantDAOs {
			if candidateDAO.Puuid == participantDAO.Puuid {
				exists = true
				break
			}
		}
		if !exists {
			nonParticipantCandidateDAOs = append(nonParticipantCandidateDAOs, candidateDAO)
		}
	}

	possibleTeamPositions := service.GetPossibleTeamPositions()
	unOccupiedTeamPositions := make([]service.CustomGameTeamPositionVO, 0)
	for _, teamPosition := range possibleTeamPositions {
		exists := false
		for _, participantDAO := range participantDAOs {
			if teamPosition.Team == participantDAO.Team && teamPosition.Position == participantDAO.Position {
				exists = true
				break
			}
		}
		if !exists {
			unOccupiedTeamPositions = append(unOccupiedTeamPositions, teamPosition)
		}
	}

	i := 0
	j := 0
	for i < len(nonParticipantCandidateDAOs) && j < len(unOccupiedTeamPositions) {
		nonParticipantCandidateDAO := nonParticipantCandidateDAOs[i]
		unOccupiedTeamPosition := unOccupiedTeamPositions[j]

		// add participant
		newParticipantDAO := models.CustomGameParticipantDAO{
			CustomGameConfigId: req.Id,
			Puuid:              nonParticipantCandidateDAO.Puuid,
			Team:               unOccupiedTeamPosition.Team,
			Position:           unOccupiedTeamPosition.Position,
		}
		if err := newParticipantDAO.Upsert(tx); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		i++
		j++
	}

	if err := service.RecalculateCustomGameBalance(tx, req.Id); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.Id, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func UnarrangeAllParticipants(c *gin.Context) {
	var req UtilityRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	permitted, err := service.CheckPermissionForCustomGameConfig(db.Root, req.Id, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// delete all participants
	if err := models.DeleteCustomGameParticipantDAOs_byId(tx, req.Id); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// delete all color labels
	if err := models.DeleteCustomGameParticipantColorLabels_byCustomGameConfigId(tx, req.Id); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// recalculate balance
	if err := service.RecalculateCustomGameBalance(tx, req.Id); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.Id, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func SwapTeam(c *gin.Context) {
	var req UtilityRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	permitted, err := service.CheckPermissionForCustomGameConfig(db.Root, req.Id, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// get all participants
	participantDAOs, err := models.GetCustomGameParticipantDAOs_byCustomGameConfigId(tx, req.Id)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// delete all participants
	if err := models.DeleteCustomGameParticipantDAOs_byId(tx, req.Id); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	for _, participantDAO := range participantDAOs {
		if participantDAO.Team == 1 {
			participantDAO.Team = 2
		} else {
			participantDAO.Team = 1
		}

		if err := participantDAO.Upsert(tx); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if err := service.RecalculateCustomGameBalance(tx, req.Id); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.Id, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func ShuffleTeam(c *gin.Context) {
	var req UtilityRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	permitted, err := service.CheckPermissionForCustomGameConfig(db.Root, req.Id, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// get all participants
	participantDAOs, err := models.GetCustomGameParticipantDAOs_byCustomGameConfigId(tx, req.Id)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	possibleTeamPositions := service.GetPossibleTeamPositions()

	// shuffle
	rand.Shuffle(len(possibleTeamPositions), func(i, j int) {
		possibleTeamPositions[i], possibleTeamPositions[j] = possibleTeamPositions[j], possibleTeamPositions[i]
	})

	// delete all participants
	if err := models.DeleteCustomGameParticipantDAOs_byId(tx, req.Id); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	i := 0
	for _, participantDAO := range participantDAOs {
		participantDAO.Team = possibleTeamPositions[i].Team
		participantDAO.Position = possibleTeamPositions[i].Position

		if err := participantDAO.Upsert(tx); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		i++
	}

	if err := service.RecalculateCustomGameBalance(tx, req.Id); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.Id, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func RenewRanks(c *gin.Context) {
	var req UtilityRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	uid := c.GetString("uid")

	// check if user is creator of custom game
	permitted, err := service.CheckPermissionForCustomGameConfig(db.Root, req.Id, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	// get all participants
	participantDAOs, err := models.GetCustomGameParticipantDAOs_byCustomGameConfigId(tx, req.Id)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	for _, participantDAO := range participantDAOs {
		// get summoner info
		summonerDAO, exists, err := models.GetSummonerDAO_byPuuid(tx, participantDAO.Puuid)
		if err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
		if !exists {
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusNotFound, "summoner not found")
			return
		}

		// update profile
		if _, _, err := service.RenewSummonerInfoByPuuid(tx, summonerDAO.Puuid); err != nil {
			log.Warnf("failed to renew summoner info: %s, but whatever.", err)
			log.Warn(err)
		}

		// get rank info
		if err := service.RenewSummonerLeague(tx, summonerDAO.Puuid); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}

		// get mastery info
		if err := service.RenewSummonerMastery(tx, summonerDAO.Puuid); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, nil)
}
