package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/shyunku-libraries/go-logger"
	"team.gg-server/core"
	"team.gg-server/libs/auth"
	"team.gg-server/libs/crypto"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/service"
	riotapi "team.gg-server/third_party/riot/api"
	"team.gg-server/util"
)

const (
	rsoStateKeyPrefix = "auth:rso:state:"
	rsoFlowKeyPrefix  = "auth:rso:flow:"
	rsoSetupKeyPrefix = "auth:rso:setup:"
	rsoFlowPending    = "pending"
	rsoFlowComplete   = "complete"
	rsoFlowAction     = "action_required"
	rsoFlowError      = "error"
	rsoModeLogin      = "login"
	rsoModeLink       = "link"
)

var rsoHTTPClient = &http.Client{Timeout: 15 * time.Second}

type rsoFlowResult struct {
	Status          string            `json:"status"`
	Login           *LoginResponseDto `json:"login,omitempty"`
	SetupToken      string            `json:"setupToken,omitempty"`
	RiotDisplayName string            `json:"riotDisplayName,omitempty"`
	Error           string            `json:"error,omitempty"`
}

type rsoState struct {
	FlowId string `json:"flowId"`
	Mode   string `json:"mode"`
	Uid    string `json:"uid,omitempty"`
}

type rsoSetup struct {
	Puuid       string `json:"puuid"`
	DisplayName string `json:"displayName"`
	GameName    string `json:"gameName,omitempty"`
	TagLine     string `json:"tagLine,omitempty"`
}

type rsoTokenResponse struct {
	AccessToken      string `json:"access_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type riotAccountResponse struct {
	Puuid    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

func RsoStart(c *gin.Context) {
	startRso(c, rsoModeLogin, "")
}

func RsoLinkStart(c *gin.Context) {
	uid, ok := requireAccessToken(c)
	if !ok {
		return
	}
	startRso(c, rsoModeLink, uid)
}

func startRso(c *gin.Context, mode, uid string) {
	if core.RsoClientId == "" || core.RsoClientSecret == "" || core.RsoClientCallbackUri == "" {
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "RSO is not configured")
		return
	}

	state := uuid.NewString()
	flowId := uuid.NewString()
	stateValue, err := json.Marshal(rsoState{FlowId: flowId, Mode: mode, Uid: uid})
	if err != nil {
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to initialize RSO login")
		return
	}
	if err := db.InMemoryDB.SetExp(rsoStateKeyPrefix+state, string(stateValue), 10*time.Minute); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to initialize RSO login")
		return
	}
	if err := saveRsoFlow(flowId, rsoFlowResult{Status: rsoFlowPending}, 10*time.Minute); err != nil {
		_ = db.InMemoryDB.Del(rsoStateKeyPrefix + state)
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to initialize RSO login")
		return
	}

	query := url.Values{}
	query.Set("client_id", core.RsoClientId)
	query.Set("redirect_uri", core.RsoClientCallbackUri)
	query.Set("response_type", "code")
	query.Set("scope", "openid offline_access")
	query.Set("state", state)
	if mode == rsoModeLink {
		// Linking must ask for credentials again so another Riot account can be selected.
		query.Set("prompt", "login")
	}

	c.JSON(http.StatusOK, gin.H{
		"flowId":       flowId,
		"authorizeUrl": "https://auth.riotgames.com/authorize?" + query.Encode(),
	})
}

func RsoStatus(c *gin.Context) {
	flowId := c.Query("flowId")
	if flowId == "" {
		util.AbortWithStrJson(c, http.StatusBadRequest, "flowId is required")
		return
	}

	raw, err := db.InMemoryDB.Get(rsoFlowKeyPrefix + flowId)
	if err != nil {
		if errors.Is(err, db.ErrValueNotFound) {
			util.AbortWithStrJson(c, http.StatusNotFound, "RSO login expired")
			return
		}
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to read RSO login")
		return
	}

	var result rsoFlowResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "invalid RSO login state")
		return
	}
	if result.Status != rsoFlowPending {
		_ = db.InMemoryDB.Del(rsoFlowKeyPrefix + flowId)
	}
	c.JSON(http.StatusOK, result)
}

func RsoLogin(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		renderRsoResult(c, false, "인증 요청이 올바르지 않습니다.")
		return
	}
	if issuer := c.Query("iss"); issuer != "" && issuer != "https://auth.riotgames.com" {
		renderRsoResult(c, false, "인증 제공자를 확인할 수 없습니다.")
		return
	}

	stateRaw, err := db.InMemoryDB.Get(rsoStateKeyPrefix + state)
	if err != nil {
		renderRsoResult(c, false, "만료되었거나 이미 사용된 인증 요청입니다.")
		return
	}
	_ = db.InMemoryDB.Del(rsoStateKeyPrefix + state)
	var loginState rsoState
	if err := json.Unmarshal([]byte(stateRaw), &loginState); err != nil || loginState.FlowId == "" {
		renderRsoResult(c, false, "인증 상태를 확인하지 못했습니다.")
		return
	}

	account, err := exchangeRsoAccount(code)
	if err != nil {
		log.Error(err)
		failRsoFlow(loginState.FlowId, "Riot 계정 정보를 확인하지 못했습니다.")
		renderRsoResult(c, false, "Riot 계정 정보를 확인하지 못했습니다.")
		return
	}
	renewRsoLolSummoner(account)

	displayName := riotDisplayName(account)
	if loginState.Mode == rsoModeLink {
		if err := linkRiotIdentity(c, loginState.Uid, account, true); err != nil {
			log.Error(err)
			failRsoFlow(loginState.FlowId, err.Error())
			renderRsoResult(c, false, err.Error())
			return
		}
		if err := saveRsoFlow(loginState.FlowId, rsoFlowResult{Status: rsoFlowComplete, RiotDisplayName: displayName}, 2*time.Minute); err != nil {
			log.Error(err)
			renderRsoResult(c, false, "연결 결과를 저장하지 못했습니다.")
			return
		}
		renderRsoResult(c, true, "Riot 계정이 team.gg와 연결되었습니다. 잠시 후 창이 자동으로 닫힙니다.")
		return
	}

	identity, exists, err := models.GetUserIdentityDAO(db.Root, models.UserIdentityProviderRiot, account.Puuid)
	if err != nil {
		log.Error(err)
		failRsoFlow(loginState.FlowId, "team.gg 계정을 확인하지 못했습니다.")
		renderRsoResult(c, false, "team.gg 계정을 확인하지 못했습니다.")
		return
	}
	if !exists {
		setupToken := uuid.NewString()
		setupRaw, _ := json.Marshal(rsoSetup{
			Puuid:       account.Puuid,
			DisplayName: displayName,
			GameName:    account.GameName,
			TagLine:     account.TagLine,
		})
		if err := db.InMemoryDB.SetExp(rsoSetupKeyPrefix+setupToken, string(setupRaw), 5*time.Minute); err != nil {
			failRsoFlow(loginState.FlowId, "로그인 설정을 저장하지 못했습니다.")
			renderRsoResult(c, false, "로그인 설정을 저장하지 못했습니다.")
			return
		}
		if err := saveRsoFlow(loginState.FlowId, rsoFlowResult{Status: rsoFlowAction, SetupToken: setupToken, RiotDisplayName: displayName}, 2*time.Minute); err != nil {
			log.Error(err)
			renderRsoResult(c, false, "로그인 결과를 저장하지 못했습니다.")
			return
		}
		renderRsoResult(c, true, "Riot 인증이 완료되었습니다. team.gg에서 계정 설정을 마무리하세요.")
		return
	}
	userDAO, userExists, err := models.GetUserDAO_byUid(db.Root, identity.Uid)
	if err != nil || !userExists {
		if err == nil {
			err = errors.New("연결된 team.gg 계정이 없습니다")
		}
		log.Error(err)
		failRsoFlow(loginState.FlowId, "연결된 team.gg 계정을 확인하지 못했습니다.")
		renderRsoResult(c, false, "연결된 team.gg 계정을 확인하지 못했습니다.")
		return
	}
	identity.DisplayName = displayName
	identity.UpdatedAt = time.Now()
	if err := identity.Upsert(db.Root); err != nil {
		log.Error(err)
	}
	loginResponse, err := createLoginResponse(userDAO, userDAO.UserId)
	if err != nil {
		log.Error(err)
		failRsoFlow(loginState.FlowId, "로그인 토큰을 생성하지 못했습니다.")
		renderRsoResult(c, false, "로그인 토큰을 생성하지 못했습니다.")
		return
	}
	if err := saveRsoFlow(loginState.FlowId, rsoFlowResult{Status: rsoFlowComplete, Login: loginResponse}, 2*time.Minute); err != nil {
		log.Error(err)
		renderRsoResult(c, false, "로그인 결과를 저장하지 못했습니다.")
		return
	}

	renderRsoResult(c, true, "인증이 완료되었습니다. 잠시 후 창이 자동으로 닫힙니다.")
}

func exchangeRsoAccount(code string) (*riotAccountResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", core.RsoClientCallbackUri)

	tokenRequest, err := http.NewRequest(http.MethodPost, "https://auth.riotgames.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	tokenRequest.SetBasicAuth(core.RsoClientId, core.RsoClientSecret)
	tokenRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenResponse, err := rsoHTTPClient.Do(tokenRequest)
	if err != nil {
		return nil, err
	}
	defer tokenResponse.Body.Close()

	var token rsoTokenResponse
	if err := json.NewDecoder(tokenResponse.Body).Decode(&token); err != nil {
		return nil, err
	}
	if tokenResponse.StatusCode < 200 || tokenResponse.StatusCode >= 300 || token.AccessToken == "" {
		return nil, fmt.Errorf("RSO token exchange failed: status=%d error=%s description=%s", tokenResponse.StatusCode, token.Error, token.ErrorDescription)
	}

	accountRequest, err := http.NewRequest(http.MethodGet, "https://asia.api.riotgames.com/riot/account/v1/accounts/me", nil)
	if err != nil {
		return nil, err
	}
	accountRequest.Header.Set("Authorization", "Bearer "+token.AccessToken)
	accountResponse, err := rsoHTTPClient.Do(accountRequest)
	if err != nil {
		return nil, err
	}
	defer accountResponse.Body.Close()

	var account riotAccountResponse
	if err := json.NewDecoder(accountResponse.Body).Decode(&account); err != nil {
		return nil, err
	}
	if accountResponse.StatusCode < 200 || accountResponse.StatusCode >= 300 || account.Puuid == "" {
		return nil, fmt.Errorf("RSO account lookup failed: status=%d", accountResponse.StatusCode)
	}
	return &account, nil
}

func riotDisplayName(account *riotAccountResponse) string {
	displayName := account.GameName
	if account.TagLine != "" {
		displayName += "#" + account.TagLine
	}
	return displayName
}

func renewRsoLolSummoner(account *riotAccountResponse) {
	apiAccount, _, err := riotapi.GetAccountByRiotId(account.GameName, account.TagLine)
	if err != nil {
		log.Warnf("RSO Riot ID could not be resolved with the Riot API key: %v", err)
		return
	}
	if _, _, err := service.RenewSummonerInfoByPuuid(db.Root, apiAccount.Puuid); err != nil {
		log.Warnf("RSO account is not confirmed as a KR LoL account: %v", err)
	}
}

func splitRiotDisplayName(displayName string) (string, string, bool) {
	separator := strings.LastIndex(displayName, "#")
	if separator <= 0 || separator >= len(displayName)-1 {
		return "", "", false
	}
	return displayName[:separator], displayName[separator+1:], true
}

func createRiotOnlyUser(c *gin.Context, setup rsoSetup) (*models.UserDAO, error) {
	uid := uuid.NewSHA1(uuid.NameSpaceURL, []byte("team.gg:riot:"+setup.Puuid)).String()
	compactUid := strings.ReplaceAll(uid, "-", "")
	userDAO := &models.UserDAO{
		Uid:               uid,
		UserId:            "riot_" + compactUid[:20],
		EncryptedPassword: util.Sha256(uuid.NewString()),
	}
	now := time.Now()
	identity := &models.UserIdentityDAO{
		Provider:        models.UserIdentityProviderRiot,
		ProviderSubject: setup.Puuid,
		Uid:             uid,
		DisplayName:     setup.DisplayName,
		IsPrimary:       true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		return nil, err
	}
	if err := userDAO.Upsert(tx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := identity.Upsert(tx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return userDAO, nil
}

func requireAccessToken(c *gin.Context) (string, bool) {
	token, err := auth.ExtractAuthToken(c.Request)
	if err != nil {
		util.AbortWithStrJson(c, http.StatusUnauthorized, "로그인이 필요합니다.")
		return "", false
	}
	uid, err := auth.ValidateToken(token, crypto.JwtAccessSecretKey)
	if err != nil || uid == "" {
		util.AbortWithStrJson(c, http.StatusUnauthorized, "인증 정보가 만료되었습니다.")
		return "", false
	}
	return uid, true
}

func loadRsoSetup(token string) (*rsoSetup, error) {
	raw, err := db.InMemoryDB.Get(rsoSetupKeyPrefix + token)
	if err != nil {
		return nil, errors.New("Riot 계정 설정이 만료되었습니다. 다시 인증해주세요")
	}
	var setup rsoSetup
	if err := json.Unmarshal([]byte(raw), &setup); err != nil || setup.Puuid == "" {
		return nil, errors.New("Riot 계정 설정을 확인하지 못했습니다")
	}
	return &setup, nil
}

func RsoCompleteExisting(c *gin.Context) {
	var req struct {
		SetupToken        string `json:"setupToken" binding:"required"`
		UserId            string `json:"userId" binding:"required"`
		EncryptedPassword string `json:"encryptedPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "입력값을 확인해주세요.")
		return
	}
	setup, err := loadRsoSetup(req.SetupToken)
	if err != nil {
		util.AbortWithStrJson(c, http.StatusGone, err.Error())
		return
	}
	password := util.Sha256(req.UserId + req.EncryptedPassword)
	userDAO, exists, err := models.GetUserDAO_byUserId_withPassword(db.Root, req.UserId, password)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "계정을 확인하지 못했습니다.")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusUnauthorized, "아이디 또는 비밀번호가 일치하지 않습니다.")
		return
	}
	gameName, tagLine := setup.GameName, setup.TagLine
	if gameName == "" || tagLine == "" {
		gameName, tagLine, _ = splitRiotDisplayName(setup.DisplayName)
	}
	account := &riotAccountResponse{Puuid: setup.Puuid, GameName: gameName, TagLine: tagLine}
	if err := linkRiotIdentity(c, userDAO.Uid, account, false); err != nil {
		util.AbortWithStrJson(c, http.StatusConflict, err.Error())
		return
	}
	_ = db.InMemoryDB.Del(rsoSetupKeyPrefix + req.SetupToken)
	response, err := createLoginResponse(userDAO, userDAO.UserId)
	if err != nil {
		util.AbortWithStrJson(c, http.StatusInternalServerError, "로그인 토큰을 생성하지 못했습니다.")
		return
	}
	c.JSON(http.StatusOK, response)
}

func RsoCompleteNew(c *gin.Context) {
	var req struct {
		SetupToken string `json:"setupToken" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "입력값을 확인해주세요.")
		return
	}
	setup, err := loadRsoSetup(req.SetupToken)
	if err != nil {
		util.AbortWithStrJson(c, http.StatusGone, err.Error())
		return
	}
	userDAO, err := createRiotOnlyUser(c, *setup)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusConflict, "계정을 생성하지 못했습니다.")
		return
	}
	_ = db.InMemoryDB.Del(rsoSetupKeyPrefix + req.SetupToken)
	response, err := createLoginResponse(userDAO, userDAO.UserId)
	if err != nil {
		util.AbortWithStrJson(c, http.StatusInternalServerError, "로그인 토큰을 생성하지 못했습니다.")
		return
	}
	c.JSON(http.StatusOK, response)
}

func linkRiotIdentity(c *gin.Context, uid string, account *riotAccountResponse, allowAutoAccountMerge bool) error {
	if uid == "" {
		return errors.New("로그인된 team.gg 계정을 확인하지 못했습니다")
	}
	displayName := riotDisplayName(account)
	currentIdentities, err := models.GetUserIdentityDAOs_byUid(db.Root, models.UserIdentityProviderRiot, uid)
	if err != nil {
		return err
	}
	hasCurrentPrimary := false
	for _, current := range currentIdentities {
		if current.IsPrimary {
			hasCurrentPrimary = true
			break
		}
	}

	identity, exists, err := models.GetUserIdentityDAO(db.Root, models.UserIdentityProviderRiot, account.Puuid)
	if err != nil {
		return err
	}
	if exists && identity.Uid == uid {
		identity.DisplayName = displayName
		identity.UpdatedAt = time.Now()
		if !hasCurrentPrimary {
			identity.IsPrimary = true
		}
		return identity.Upsert(db.Root)
	}
	if exists {
		if !allowAutoAccountMerge {
			return errors.New("이미 다른 team.gg 계정에 연결된 Riot 계정입니다")
		}
		oldUser, oldUserExists, err := models.GetUserDAO_byUid(db.Root, identity.Uid)
		if err != nil {
			return err
		}
		if !oldUserExists || !strings.HasPrefix(oldUser.UserId, "riot_") {
			return errors.New("이미 다른 team.gg 계정에 연결된 Riot 계정입니다")
		}
		tx, err := db.Root.BeginTxx(c, nil)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE custom_game_configurations SET creator_uid = ? WHERE creator_uid = ?`, uid, identity.Uid); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`UPDATE user_identities SET is_primary = 0 WHERE provider = ? AND uid = ?`, models.UserIdentityProviderRiot, identity.Uid); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`UPDATE user_identities SET uid = ? WHERE uid = ?`, uid, identity.Uid); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`UPDATE user_identities SET display_name = ?, updated_at = ? WHERE provider = ? AND provider_subject = ?`, displayName, time.Now(), models.UserIdentityProviderRiot, account.Puuid); err != nil {
			_ = tx.Rollback()
			return err
		}
		if !hasCurrentPrimary {
			if err := models.SetPrimaryUserIdentityDAO(tx, models.UserIdentityProviderRiot, uid, account.Puuid, time.Now()); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		if _, err := tx.Exec(`DELETE FROM users WHERE uid = ?`, identity.Uid); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		_ = auth.DeleteRefreshToken(identity.Uid)
		return nil
	}

	now := time.Now()
	return (&models.UserIdentityDAO{
		Provider:        models.UserIdentityProviderRiot,
		ProviderSubject: account.Puuid,
		Uid:             uid,
		DisplayName:     displayName,
		IsPrimary:       !hasCurrentPrimary,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Upsert(db.Root)
}

func GetMyAccount(c *gin.Context) {
	uid, ok := requireAccessToken(c)
	if !ok {
		return
	}
	userDAO, exists, err := models.GetUserDAO_byUid(db.Root, uid)
	if err != nil || !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "team.gg 계정을 찾지 못했습니다.")
		return
	}
	identities, err := models.GetUserIdentityDAOs_byUid(db.Root, models.UserIdentityProviderRiot, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "연결 정보를 확인하지 못했습니다.")
		return
	}
	connected := len(identities) > 0
	rsoInfo := gin.H{"connected": connected, "accounts": make([]gin.H, 0)}
	displayName := userDAO.UserId
	if connected {
		primaryIndex := 0
		for index := range identities {
			if identities[index].IsPrimary {
				primaryIndex = index
				break
			}
		}
		if !identities[primaryIndex].IsPrimary {
			if err := models.SetPrimaryUserIdentityDAO(db.Root, models.UserIdentityProviderRiot, uid, identities[primaryIndex].ProviderSubject, time.Now()); err != nil {
				log.Error(err)
				util.AbortWithStrJson(c, http.StatusInternalServerError, "대표 Riot 계정을 설정하지 못했습니다.")
				return
			}
			identities[primaryIndex].IsPrimary = true
		}

		rsoAccounts := make([]gin.H, 0, len(identities))
		canUnlink := !strings.HasPrefix(userDAO.UserId, "riot_") || len(identities) > 1
		for index, identity := range identities {
			accountInfo := gin.H{
				"puuid": identity.ProviderSubject, "displayName": identity.DisplayName,
				"isPrimary": index == primaryIndex, "canUnlink": canUnlink, "isLolAccount": false,
			}
			gameName, tagLine, validRiotId := splitRiotDisplayName(identity.DisplayName)
			if validRiotId {
				summoner, isLolAccount, summonerErr := models.GetSummonerDAO_byNameTag(db.Root, gameName, tagLine)
				if summonerErr != nil {
					log.Error(summonerErr)
					util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 계정 정보를 확인하지 못했습니다.")
					return
				}
				if isLolAccount {
					accountInfo["gameName"] = summoner.GameName
					accountInfo["tagLine"] = summoner.TagLine
					accountInfo["profileIconId"] = summoner.ProfileIconId
					accountInfo["isLolAccount"] = true
					accountInfo["displayName"] = fmt.Sprintf("%s#%s", summoner.GameName, summoner.TagLine)
				}
			}
			rsoAccounts = append(rsoAccounts, accountInfo)
			if index == primaryIndex {
				for key, value := range accountInfo {
					rsoInfo[key] = value
				}
				displayName = accountInfo["displayName"].(string)
			}
		}
		rsoInfo["accounts"] = rsoAccounts
		rsoInfo["canUnlink"] = canUnlink
	}
	c.JSON(http.StatusOK, gin.H{"uid": userDAO.Uid, "userId": userDAO.UserId, "displayName": displayName, "riot": rsoInfo})
}

func UnlinkRiotAccount(c *gin.Context) {
	uid, ok := requireAccessToken(c)
	if !ok {
		return
	}
	userDAO, exists, err := models.GetUserDAO_byUid(db.Root, uid)
	if err != nil || !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "team.gg 계정을 찾지 못했습니다.")
		return
	}
	identities, err := models.GetUserIdentityDAOs_byUid(db.Root, models.UserIdentityProviderRiot, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 연결 정보를 확인하지 못했습니다.")
		return
	}
	if len(identities) == 0 {
		util.AbortWithStrJson(c, http.StatusNotFound, "연결된 Riot 계정이 없습니다.")
		return
	}
	if strings.HasPrefix(userDAO.UserId, "riot_") && len(identities) == 1 {
		util.AbortWithStrJson(c, http.StatusConflict, "Riot 로그인 전용 계정은 연결을 해제할 수 없습니다.")
		return
	}
	puuid := c.Query("puuid")
	if puuid == "" {
		for _, identity := range identities {
			if identity.IsPrimary {
				puuid = identity.ProviderSubject
				break
			}
		}
		if puuid == "" {
			puuid = identities[0].ProviderSubject
		}
	}
	var target *models.UserIdentityDAO
	for index := range identities {
		if identities[index].ProviderSubject == puuid {
			target = &identities[index]
			break
		}
	}
	if target == nil {
		util.AbortWithStrJson(c, http.StatusNotFound, "연결된 Riot 계정을 찾지 못했습니다.")
		return
	}
	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 연결을 해제하지 못했습니다.")
		return
	}
	if err := models.DeleteUserIdentityDAO_bySubject(tx, models.UserIdentityProviderRiot, uid, puuid); err != nil {
		_ = tx.Rollback()
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 연결을 해제하지 못했습니다.")
		return
	}
	if target.IsPrimary && len(identities) > 1 {
		for _, identity := range identities {
			if identity.ProviderSubject != puuid {
				if err := models.SetPrimaryUserIdentityDAO(tx, models.UserIdentityProviderRiot, uid, identity.ProviderSubject, time.Now()); err != nil {
					_ = tx.Rollback()
					log.Error(err)
					util.AbortWithStrJson(c, http.StatusInternalServerError, "대표 Riot 계정을 변경하지 못했습니다.")
					return
				}
				break
			}
		}
	}
	if err := tx.Commit(); err != nil {
		util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 연결을 해제하지 못했습니다.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func SetPrimaryRiotAccount(c *gin.Context) {
	uid, ok := requireAccessToken(c)
	if !ok {
		return
	}
	var req struct {
		Puuid string `json:"puuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "Riot 계정을 선택해주세요.")
		return
	}
	identity, exists, err := models.GetUserIdentityDAO(db.Root, models.UserIdentityProviderRiot, req.Puuid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "Riot 연결 정보를 확인하지 못했습니다.")
		return
	}
	if !exists || identity.Uid != uid {
		util.AbortWithStrJson(c, http.StatusNotFound, "연결된 Riot 계정을 찾지 못했습니다.")
		return
	}
	if err := models.SetPrimaryUserIdentityDAO(db.Root, models.UserIdentityProviderRiot, uid, req.Puuid, time.Now()); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "대표 Riot 계정을 변경하지 못했습니다.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func saveRsoFlow(flowId string, result rsoFlowResult, ttl time.Duration) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return db.InMemoryDB.SetExp(rsoFlowKeyPrefix+flowId, string(raw), ttl)
}

func failRsoFlow(flowId, message string) {
	if err := saveRsoFlow(flowId, rsoFlowResult{Status: rsoFlowError, Error: message}, 2*time.Minute); err != nil {
		log.Error(err)
	}
}

func renderRsoResult(c *gin.Context, success bool, message string) {
	title := "Riot 인증 실패"
	color := "#d13639"
	closeScript := ""
	if success {
		title = "Riot 인증 완료"
		color = "#c89b3c"
		closeScript = `<script>setTimeout(function(){window.close()},800)</script>`
	}
	html := fmt.Sprintf(`<!doctype html><html lang="ko"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title></head><body style="margin:0;background:#01030e;color:#f0e6d2;font-family:sans-serif;display:flex;min-height:100vh;align-items:center;justify-content:center"><main style="text-align:center;padding:32px"><h1 style="color:%s;font-size:24px">%s</h1><p style="color:#a09b8c">%s</p></main>%s</body></html>`, title, color, title, message, closeScript)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
