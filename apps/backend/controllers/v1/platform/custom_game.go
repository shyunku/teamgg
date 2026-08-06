package platform

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/shyunku-libraries/go-logger"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"team.gg-server/controllers/socket"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/service"
	"team.gg-server/third_party/riot/api"
	"team.gg-server/types"
	"team.gg-server/util"
	"time"
	"unicode/utf8"
)

type summonerMatchesRefreshJob struct {
	configId string
	puuid    string
}

var (
	summonerMatchesRefreshQueue   = make(chan summonerMatchesRefreshJob, 100)
	summonerMatchesRefreshOnce    sync.Once
	pendingSummonerMatchesRefresh sync.Map
)

func beginCustomGameConfigurationMutation(c *gin.Context, configId string) bool {
	if service.TryBeginCustomGameConfigurationMutation(configId) {
		return true
	}
	util.AbortWithStrJson(c, http.StatusLocked, "최적의 조합을 계산 중이라 내전 구성을 변경할 수 없습니다.")
	return false
}

func canEditCustomGameCandidatePreference(database db.Context, configId, puuid, uid string) (bool, error) {
	permitted, err := service.CheckPermissionForCustomGameConfig(database, configId, uid)
	if err != nil || permitted {
		return permitted, err
	}
	ownedPuuids, err := getUserRiotPuuids(database, uid)
	if err != nil {
		return false, err
	}
	for _, ownedPuuid := range ownedPuuids {
		if ownedPuuid == puuid {
			return true, nil
		}
	}
	return false, nil
}

// RSO's account subject is not guaranteed to be the same identifier stored by
// the LoL summoner APIs. Resolve the current Riot ID to the summoner PUUID too.
func getUserRiotPuuids(database db.Context, uid string) ([]string, error) {
	identities, err := models.GetUserIdentityDAOs_byUid(database, models.UserIdentityProviderRiot, uid)
	if err != nil {
		return nil, err
	}
	puuidSet := make(map[string]struct{}, len(identities)*2)
	for _, identity := range identities {
		if identity.ProviderSubject != "" {
			puuidSet[identity.ProviderSubject] = struct{}{}
		}
		separator := strings.LastIndex(identity.DisplayName, "#")
		if separator <= 0 || separator >= len(identity.DisplayName)-1 {
			continue
		}
		summoner, exists, err := models.GetSummonerDAO_byNameTag(
			database,
			identity.DisplayName[:separator],
			identity.DisplayName[separator+1:],
		)
		if err != nil {
			return nil, err
		}
		if exists {
			puuidSet[summoner.Puuid] = struct{}{}
		}
	}
	puuids := make([]string, 0, len(puuidSet))
	for puuid := range puuidSet {
		puuids = append(puuids, puuid)
	}
	return puuids, nil
}

func queueSummonerMatchesRefresh(configId, puuid string) {
	jobKey := configId + ":" + puuid
	if _, loaded := pendingSummonerMatchesRefresh.LoadOrStore(jobKey, struct{}{}); loaded {
		return
	}

	summonerMatchesRefreshOnce.Do(func() {
		go func() {
			for job := range summonerMatchesRefreshQueue {
				key := job.configId + ":" + job.puuid
				if err := service.RenewSummonerMatches(db.Root, job.puuid, nil); err != nil {
					log.Warnf("background match refresh failed: %v", err)
				} else if service.TryBeginCustomGameConfigurationMutation(job.configId) {
					if err := service.RecalculateCustomGameBalance(db.Root, job.configId); err != nil {
						log.Warnf("background custom game balance refresh failed: %v", err)
					} else {
						socket.SocketIO.BroadcastToCustomConfigRoom(job.configId, socket.EventCustomConfigUpdated, nil)
					}
					service.EndCustomGameConfigurationMutation(job.configId)
				}
				pendingSummonerMatchesRefresh.Delete(key)
			}
		}()
	})

	select {
	case summonerMatchesRefreshQueue <- summonerMatchesRefreshJob{configId: configId, puuid: puuid}:
	default:
		pendingSummonerMatchesRefresh.Delete(jobKey)
		log.Warn("summoner matches refresh queue is full")
	}
}

func UseCustomGameRouter(r *gin.RouterGroup) {
	g := r.Group("/custom-game")
	useReplayAnalysisRouter(g)

	g.GET("/list", GetCustomGameConfigurationList)
	g.GET("/joined-list", GetJoinedCustomGameConfigurationList)
	g.GET("/info", GetCustomGameConfiguration)
	g.POST("/create", CreateCustomGameConfiguration)
	g.PATCH("/name", UpdateCustomGameConfigurationName)

	g.GET("/tier-rank", GetTierRank)
	g.GET("/balance", GetCustomConfigurationBalance)

	g.PUT("/candidate", AddCandidateToCustomGameConfiguration)
	g.DELETE("/candidate", DeleteCandidateFromCustomGameConfiguration)

	g.POST("/arrange", ArrangeCustomGameParticipant)
	g.POST("/unarrange", UnarrangeCustomGameParticipant)
	g.POST("/favor-position", SetCustomGameParticipantFavorPosition)
	g.POST("/line-mastery", SetCustomGameParticipantLineMastery)
	g.POST("/default-favor-position", SaveCustomGameDefaultFavorPosition)
	g.POST("/reset-favor-position", ResetCustomGameFavorPositionToDefault)
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

func GetJoinedCustomGameConfigurationList(c *gin.Context) {
	uid := c.GetString("uid")
	if uid == "" {
		util.AbortWithStrJson(c, http.StatusUnauthorized, "user not found")
		return
	}

	ownedPuuids, err := getUserRiotPuuids(db.Root, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 계정 연결 정보를 확인하지 못했습니다.")
		return
	}
	resp, err := service.GetJoinedCustomGameConfigurationVOs(uid, ownedPuuids)
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
	uid := c.GetString("uid")
	resp.CanManage = resp.CreatorUid == uid
	ownedPuuids, err := getUserRiotPuuids(db.Root, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 계정 연결 정보를 확인하지 못했습니다.")
		return
	}
	resp.OwnedPuuids = ownedPuuids

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

	ownerName := userDAO.UserId
	if identity, connected, identityErr := models.GetUserIdentityDAO_byUid(db.Root, models.UserIdentityProviderRiot, uid); identityErr != nil {
		log.Error(identityErr)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	} else if connected {
		separator := strings.LastIndex(identity.DisplayName, "#")
		if separator <= 0 || separator >= len(identity.DisplayName)-1 {
			// Keep the team.gg id when the stored Riot display name is invalid.
		} else if summoner, isLolAccount, summonerErr := models.GetSummonerDAO_byNameTag(
			db.Root,
			identity.DisplayName[:separator],
			identity.DisplayName[separator+1:],
		); summonerErr != nil {
			log.Error(summonerErr)
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		} else if isLolAccount {
			ownerName = fmt.Sprintf("%s#%s", summoner.GameName, summoner.TagLine)
		}
	}

	var name string
	namePrefix := fmt.Sprintf("%s의 내전 팀 구성", ownerName)
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
		MasteryInfluenceWeight: types.WeightMasteryInfluence,
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

func UpdateCustomGameConfigurationName(c *gin.Context) {
	var req UpdateCustomGameConfigurationNameRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}
	if !beginCustomGameConfigurationMutation(c, req.Id) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.Id)

	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid name")
		return
	}

	uid := c.GetString("uid")
	configuration, exists, err := models.GetCustomGameDAO_byId(db.Root, req.Id)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "custom game configuration not found")
		return
	}
	if configuration.CreatorUid != uid {
		util.AbortWithStrJson(c, http.StatusForbidden, "user is not creator of custom game")
		return
	}

	updatedAt := time.Now()
	if err := models.UpdateCustomGameConfigurationName(db.Root, req.Id, name, updatedAt); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.Id, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, gin.H{"name": name, "lastUpdatedAt": updatedAt})
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
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

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
			_ = tx.Rollback()
			if status == http.StatusNotFound {
				util.AbortWithStrJson(c, http.StatusNotFound, "invalid game name")
				return
			}
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusBadRequest, "internal server error")
			return
		}

		summonerDAO, _, err = service.RenewSummonerInfoByPuuid(tx, account.Puuid)
		if err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
		if err := service.RenewSummonerLeague(tx, summonerDAO.Puuid); err != nil {
			log.Error(err)
			_ = tx.Rollback()
			util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
			return
		}
		if err := service.RenewSummonerMastery(tx, summonerDAO.Puuid); err != nil {
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
	defaultPreference, hasDefaultPreference, err := models.GetRiotCustomGamePreferenceDAO_byPuuid(db.Root, summonerDAO.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "기본 포지션 선호도와 숙련도를 불러오지 못했습니다.")
		return
	}
	if hasDefaultPreference {
		newCandidateDAO.FlavorTop = defaultPreference.FlavorTop
		newCandidateDAO.FlavorJungle = defaultPreference.FlavorJungle
		newCandidateDAO.FlavorMid = defaultPreference.FlavorMid
		newCandidateDAO.FlavorAdc = defaultPreference.FlavorAdc
		newCandidateDAO.FlavorSupport = defaultPreference.FlavorSupport
		newCandidateDAO.MasteryTop = defaultPreference.MasteryTop
		newCandidateDAO.MasteryJungle = defaultPreference.MasteryJungle
		newCandidateDAO.MasteryMid = defaultPreference.MasteryMid
		newCandidateDAO.MasteryAdc = defaultPreference.MasteryAdc
		newCandidateDAO.MasterySupport = defaultPreference.MasterySupport
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
	queueSummonerMatchesRefresh(req.CustomGameConfigId, newCandidateDAO.Puuid)

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
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

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
	rollback := func(err error) {
		log.Error(err)
		_ = tx.Rollback()
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
	}

	if err := models.DeleteCustomGameParticipantColorLabelDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid); err != nil {
		rollback(err)
		return
	}
	if err := models.DeleteCustomGameParticipantDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid); err != nil {
		rollback(err)
		return
	}
	if err := models.DeleteCustomGameCandidateDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid); err != nil {
		rollback(err)
		return
	}
	if err := service.RecalculateCustomGameBalance(tx, req.CustomGameConfigId); err != nil {
		rollback(err)
		return
	}
	if err := tx.Commit(); err != nil {
		rollback(err)
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
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

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
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

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
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

	uid := c.GetString("uid")

	// The configuration owner can edit everyone; linked Riot account owners can edit themselves.
	permitted, err := canEditCustomGameCandidatePreference(db.Root, req.CustomGameConfigId, req.Puuid, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "본인 Riot 계정의 포지션 선호도만 변경할 수 있습니다.")
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

func SetCustomGameParticipantLineMastery(c *gin.Context) {
	var req SetCustomGameParticipantLineMasteryRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "요청 정보를 확인해주세요.")
		return
	}
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

	uid := c.GetString("uid")
	permitted, err := canEditCustomGameCandidatePreference(db.Root, req.CustomGameConfigId, req.Puuid, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 계정 연결 정보를 확인하지 못했습니다.")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "본인 Riot 계정의 라인 숙련도만 변경할 수 있습니다.")
		return
	}
	if req.Level == nil || *req.Level < 0 || *req.Level > 5 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "라인 숙련도는 0~5단계로 설정해야 합니다.")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "라인 숙련도를 변경하지 못했습니다.")
		return
	}
	defer func() { _ = tx.Rollback() }()

	candidate, exists, err := models.GetCustomGameCandidateDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "라인 숙련도를 확인하지 못했습니다.")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "이 구성에서 Riot 계정을 찾지 못했습니다.")
		return
	}

	switch strings.ToUpper(req.Position) {
	case types.PositionTop:
		candidate.MasteryTop = *req.Level
	case types.PositionJungle:
		candidate.MasteryJungle = *req.Level
	case types.PositionMid:
		candidate.MasteryMid = *req.Level
	case types.PositionAdc:
		candidate.MasteryAdc = *req.Level
	case types.PositionSupport:
		candidate.MasterySupport = *req.Level
	default:
		util.AbortWithStrJson(c, http.StatusBadRequest, "지원하지 않는 라인입니다.")
		return
	}

	if err := candidate.Upsert(tx); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "라인 숙련도를 저장하지 못했습니다.")
		return
	}
	if err := service.RecalculateCustomGameBalance(tx, req.CustomGameConfigId); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "내전 밸런스를 다시 계산하지 못했습니다.")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "라인 숙련도를 저장하지 못했습니다.")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.CustomGameConfigId, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, nil)
}

func SaveCustomGameDefaultFavorPosition(c *gin.Context) {
	var req SaveCustomGameDefaultFavorPositionRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "요청 정보를 확인해주세요.")
		return
	}
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

	uid := c.GetString("uid")
	ownedPuuids, err := getUserRiotPuuids(db.Root, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 계정 연결 정보를 확인하지 못했습니다.")
		return
	}
	isOwned := false
	for _, ownedPuuid := range ownedPuuids {
		if ownedPuuid == req.Puuid {
			isOwned = true
			break
		}
	}
	if !isOwned {
		util.AbortWithStrJson(c, http.StatusForbidden, "본인 Riot 계정의 선호도와 숙련도만 기본값으로 저장할 수 있습니다.")
		return
	}
	candidate, exists, err := models.GetCustomGameCandidateDAO_byPuuid(db.Root, req.CustomGameConfigId, req.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "포지션 선호도와 숙련도를 확인하지 못했습니다.")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "이 구성에서 Riot 계정을 찾지 못했습니다.")
		return
	}
	preference := models.RiotCustomGamePreferenceDAO{
		Puuid: candidate.Puuid, FlavorTop: candidate.FlavorTop, FlavorJungle: candidate.FlavorJungle,
		FlavorMid: candidate.FlavorMid, FlavorAdc: candidate.FlavorAdc,
		FlavorSupport: candidate.FlavorSupport,
		MasteryTop:    candidate.MasteryTop, MasteryJungle: candidate.MasteryJungle,
		MasteryMid: candidate.MasteryMid, MasteryAdc: candidate.MasteryAdc,
		MasterySupport: candidate.MasterySupport, UpdatedAt: time.Now(),
	}
	if err := preference.Upsert(db.Root); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "기본 포지션 선호도와 숙련도를 저장하지 못했습니다.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func ResetCustomGameFavorPositionToDefault(c *gin.Context) {
	var req SaveCustomGameDefaultFavorPositionRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "요청 정보를 확인해주세요.")
		return
	}
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

	uid := c.GetString("uid")
	permitted, err := canEditCustomGameCandidatePreference(db.Root, req.CustomGameConfigId, req.Puuid, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 계정 연결 정보를 확인하지 못했습니다.")
		return
	}
	if !permitted {
		util.AbortWithStrJson(c, http.StatusForbidden, "본인 Riot 계정의 포지션 선호도와 숙련도만 초기화할 수 있습니다.")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "기본 포지션 선호도와 숙련도를 적용하지 못했습니다.")
		return
	}
	defer func() { _ = tx.Rollback() }()

	candidate, exists, err := models.GetCustomGameCandidateDAO_byPuuid(tx, req.CustomGameConfigId, req.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "포지션 선호도와 숙련도를 확인하지 못했습니다.")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "이 구성에서 Riot 계정을 찾지 못했습니다.")
		return
	}

	// A Riot account without a saved preference starts from the neutral default.
	candidate.FlavorTop = 0
	candidate.FlavorJungle = 0
	candidate.FlavorMid = 0
	candidate.FlavorAdc = 0
	candidate.FlavorSupport = 0
	candidate.MasteryTop = 0
	candidate.MasteryJungle = 0
	candidate.MasteryMid = 0
	candidate.MasteryAdc = 0
	candidate.MasterySupport = 0
	preference, hasPreference, err := models.GetRiotCustomGamePreferenceDAO_byPuuid(tx, req.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "기본 포지션 선호도와 숙련도를 불러오지 못했습니다.")
		return
	}
	if hasPreference {
		candidate.FlavorTop = preference.FlavorTop
		candidate.FlavorJungle = preference.FlavorJungle
		candidate.FlavorMid = preference.FlavorMid
		candidate.FlavorAdc = preference.FlavorAdc
		candidate.FlavorSupport = preference.FlavorSupport
		candidate.MasteryTop = preference.MasteryTop
		candidate.MasteryJungle = preference.MasteryJungle
		candidate.MasteryMid = preference.MasteryMid
		candidate.MasteryAdc = preference.MasteryAdc
		candidate.MasterySupport = preference.MasterySupport
	}
	if err := candidate.Upsert(tx); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "기본 포지션 선호도와 숙련도를 적용하지 못했습니다.")
		return
	}
	if err := service.RecalculateCustomGameBalance(tx, req.CustomGameConfigId); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "내전 밸런스를 다시 계산하지 못했습니다.")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "기본 포지션 선호도와 숙련도를 적용하지 못했습니다.")
		return
	}

	socket.SocketIO.MulticastToCustomConfigRoom(req.CustomGameConfigId, uid, socket.EventCustomConfigUpdated, nil)
	c.JSON(http.StatusOK, gin.H{"ok": true, "usedSavedDefault": hasPreference})
}

func SetCustomGameCandidateCustomTierRank(c *gin.Context) {
	var req SetCustomGameCandidateCustomTierRankRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

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
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

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
	if !beginCustomGameConfigurationMutation(c, req.CustomGameConfigId) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.CustomGameConfigId)

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

	// check if user is creator of custom game
	customGameConfigurationDAO, exists, err := models.GetCustomGameDAO_byId(db.Root, req.Id)
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
	if !service.TryLockCustomGameConfigurationForOptimization(req.Id) {
		util.AbortWithStrJson(c, http.StatusLocked, "다른 사용자가 내전 구성을 변경 중이거나 이미 조합을 계산하고 있습니다.")
		return
	}
	defer func() {
		service.UnlockCustomGameConfigurationForOptimization(req.Id)
		socket.SocketIO.BroadcastToCustomConfigRoom(req.Id, socket.EventCustomConfigUpdated, nil)
	}()
	socket.SocketIO.BroadcastToCustomConfigRoom(req.Id, socket.EventCustomConfigUpdated, nil)

	weights := []*float64{
		req.LineFairnessWeight,
		req.TierFairnessWeight,
		req.LineSatisfactionWeight,
	}
	for _, weight := range weights {
		if weight == nil || *weight < 0 || *weight > 1 {
			util.AbortWithStrJson(c, http.StatusBadRequest, "invalid balance weight")
			return
		}
	}
	weightSum := *req.LineFairnessWeight + *req.TierFairnessWeight + *req.LineSatisfactionWeight
	if weightSum < 0.999999 || weightSum > 1.000001 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "balance weights must sum to 1")
		return
	}
	if req.MasteryInfluenceWeight == nil || *req.MasteryInfluenceWeight < 0 || *req.MasteryInfluenceWeight > 1 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid mastery influence weight")
		return
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	defer func() { _ = tx.Rollback() }()

	customGameConfigurationDAO.LineFairnessWeight = *req.LineFairnessWeight
	customGameConfigurationDAO.TierFairnessWeight = *req.TierFairnessWeight
	customGameConfigurationDAO.LineSatisfactionWeight = *req.LineSatisfactionWeight
	customGameConfigurationDAO.MasteryInfluenceWeight = *req.MasteryInfluenceWeight
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
	if !beginCustomGameConfigurationMutation(c, req.Id) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.Id)

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
	if !beginCustomGameConfigurationMutation(c, req.Id) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.Id)

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
	if !beginCustomGameConfigurationMutation(c, req.Id) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.Id)

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
	if !beginCustomGameConfigurationMutation(c, req.Id) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.Id)

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
	if !beginCustomGameConfigurationMutation(c, req.Id) {
		return
	}
	defer service.EndCustomGameConfigurationMutation(req.Id)

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
