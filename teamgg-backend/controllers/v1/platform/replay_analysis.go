package platform

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"team.gg-server/controllers/socket"
	"team.gg-server/libs/db"
	"team.gg-server/libs/replayauth"
	"team.gg-server/models"
	"team.gg-server/util"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/shyunku-libraries/go-logger"
)

const replayUploadTicketLifetime = 15 * time.Minute
const defaultReplayAnalysisStaleAfter = 45 * time.Minute

func replayAnalysisStaleAfter() time.Duration {
	raw := strings.TrimSpace(os.Getenv("REPLAY_ANALYSIS_STALE_AFTER"))
	if raw == "" {
		return defaultReplayAnalysisStaleAfter
	}
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		log.Warnf("invalid REPLAY_ANALYSIS_STALE_AFTER %q; using %s", raw, defaultReplayAnalysisStaleAfter)
		return defaultReplayAnalysisStaleAfter
	}
	return duration
}

func useReplayAnalysisRouter(g *gin.RouterGroup) {
	g.POST("/replay-analysis", CreateCustomGameReplayAnalysis)
	g.GET("/replay-analyses", ListCustomGameReplayAnalyses)
	g.GET("/replay-analysis/:id", GetCustomGameReplayAnalysis)
	g.DELETE("/replay-analysis/:id", DeleteCustomGameReplayAnalysis)
}

func customGameReplayAccess(configId, uid string) (canView, canManage bool, err error) {
	configuration, exists, err := models.GetCustomGameDAO_byId(db.Root, configId)
	if err != nil {
		return false, false, err
	}
	if !exists {
		return false, false, nil
	}
	if configuration.CreatorUid == uid {
		return true, true, nil
	}
	ownedPuuids, err := getUserRiotPuuids(db.Root, uid)
	if err != nil {
		return false, false, err
	}
	for _, puuid := range ownedPuuids {
		if _, exists, lookupErr := models.GetCustomGameCandidateDAO_byPuuid(db.Root, configId, puuid); lookupErr != nil {
			return false, false, lookupErr
		} else if exists {
			return true, false, nil
		}
		if _, exists, lookupErr := models.GetCustomGameParticipantDAO_byPuuid(db.Root, configId, puuid); lookupErr != nil {
			return false, false, lookupErr
		} else if exists {
			return true, false, nil
		}
	}
	return false, false, nil
}

func CreateCustomGameReplayAnalysis(c *gin.Context) {
	uid := c.GetString("uid")
	if uid == "" {
		util.AbortWithStrJson(c, http.StatusUnauthorized, "로그인이 필요합니다.")
		return
	}
	var req CreateReplayAnalysisRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "리플레이 파일 정보가 올바르지 않습니다.")
		return
	}
	_, canManage, err := customGameReplayAccess(req.CustomGameConfigId, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "내전 구성 권한을 확인하지 못했습니다.")
		return
	}
	if !canManage {
		util.AbortWithStrJson(c, http.StatusForbidden, "내전 리플레이는 방장만 업로드할 수 있습니다.")
		return
	}
	fileName := filepath.Base(strings.ReplaceAll(strings.TrimSpace(req.FileName), `\`, "/"))
	if !strings.HasSuffix(strings.ToLower(fileName), ".rofl") {
		util.AbortWithStrJson(c, http.StatusBadRequest, ".rofl 리플레이 파일만 업로드할 수 있습니다.")
		return
	}
	now := time.Now().UTC()
	analysis := models.CustomGameReplayAnalysisDAO{
		Id: uuid.NewString(), CustomGameConfigId: req.CustomGameConfigId, CreatorUid: uid,
		FileName: fileName, FileSize: req.FileSize, Status: models.ReplayAnalysisStatusQueued,
		Stage: "업로드 대기 중", Progress: 0, CreatedAt: now, UpdatedAt: now,
	}
	baseUrl := strings.TrimRight(strings.TrimSpace(os.Getenv("REPLAY_ANALYZER_BASE_URL")), "/")
	if baseUrl == "" {
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "리플레이 분석 서버 주소가 설정되지 않았습니다.")
		return
	}
	ticket, err := replayauth.CreateUploadTicket(os.Getenv("REPLAY_ANALYZER_SHARED_SECRET"), analysis.Id, now.Add(replayUploadTicketLifetime))
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusServiceUnavailable, "리플레이 분석 서버 연동이 설정되지 않았습니다.")
		return
	}

	// Lock the custom-game row so concurrent requests cannot both create a running job.
	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "분석 작업을 생성하지 못했습니다.")
		return
	}
	defer func() { _ = tx.Rollback() }()
	var lockedConfigId string
	if err := tx.Get(&lockedConfigId, `SELECT id FROM custom_game_configurations WHERE id = ? FOR UPDATE`, req.CustomGameConfigId); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "내전 구성 상태를 잠그지 못했습니다.")
		return
	}
	staleCount, err := models.FailStaleCustomGameReplayAnalyses(
		tx,
		req.CustomGameConfigId,
		now.Add(-replayAnalysisStaleAfter()),
		now,
	)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "오래된 분석 작업을 정리하지 못했습니다.")
		return
	}
	running, err := models.HasRunningCustomGameReplayAnalysis(tx, req.CustomGameConfigId)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "분석 작업 상태를 확인하지 못했습니다.")
		return
	}
	if running {
		util.AbortWithStrJson(c, http.StatusConflict, "이미 업로드 또는 분석 중인 리플레이가 있습니다.")
		return
	}
	if err := analysis.Insert(tx); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "분석 작업을 생성하지 못했습니다.")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "분석 작업을 저장하지 못했습니다.")
		return
	}
	if socket.SocketIO.Io != nil {
		if staleCount > 0 {
			socket.SocketIO.BroadcastToCustomConfigRoom(
				analysis.CustomGameConfigId,
				socket.EventCustomConfigReplayAnalysisUpdated,
				gin.H{"customGameId": analysis.CustomGameConfigId, "status": "stale_failed"},
			)
		}
		socket.SocketIO.BroadcastToCustomConfigRoom(
			analysis.CustomGameConfigId,
			socket.EventCustomConfigReplayAnalysisUpdated,
			gin.H{"id": analysis.Id, "customGameId": analysis.CustomGameConfigId, "status": analysis.Status, "stage": analysis.Stage, "progress": analysis.Progress},
		)
	}
	c.JSON(http.StatusCreated, CreateReplayAnalysisResponseDto{
		Analysis:     analysis,
		UploadUrl:    baseUrl + "/v1/replays/jobs/" + analysis.Id + "/upload/stream",
		UploadTicket: ticket,
	})
}

func ListCustomGameReplayAnalyses(c *gin.Context) {
	uid := c.GetString("uid")
	var req ListReplayAnalysesRequestDto
	if uid == "" {
		util.AbortWithStrJson(c, http.StatusUnauthorized, "로그인이 필요합니다.")
		return
	}
	if c.ShouldBindQuery(&req) != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "내전 구성 정보가 올바르지 않습니다.")
		return
	}
	canView, canManage, err := customGameReplayAccess(req.CustomGameConfigId, uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "분석 내역 권한을 확인하지 못했습니다.")
		return
	}
	if !canView {
		util.AbortWithStrJson(c, http.StatusForbidden, "이 내전의 리플레이 분석 내역을 볼 권한이 없습니다.")
		return
	}
	analyses, err := models.GetCustomGameReplayAnalysesByConfigId(db.Root, req.CustomGameConfigId)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "분석 내역을 불러오지 못했습니다.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analyses": analyses, "canManage": canManage})
}

func GetCustomGameReplayAnalysis(c *gin.Context) {
	uid := c.GetString("uid")
	if uid == "" {
		util.AbortWithStrJson(c, http.StatusUnauthorized, "로그인이 필요합니다.")
		return
	}
	analysis, exists, err := models.GetCustomGameReplayAnalysisById(db.Root, c.Param("id"))
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "분석 결과를 불러오지 못했습니다.")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "분석 결과를 찾을 수 없습니다.")
		return
	}
	canView, canManage, err := customGameReplayAccess(analysis.CustomGameConfigId, uid)
	if err != nil || !canView {
		util.AbortWithStrJson(c, http.StatusForbidden, "이 분석 결과를 볼 권한이 없습니다.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis": analysis, "canManage": canManage})
}

func DeleteCustomGameReplayAnalysis(c *gin.Context) {
	uid := c.GetString("uid")
	analysis, exists, err := models.GetCustomGameReplayAnalysisById(db.Root, c.Param("id"))
	if err != nil || !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "분석 내역을 찾을 수 없습니다.")
		return
	}
	_, canManage, err := customGameReplayAccess(analysis.CustomGameConfigId, uid)
	if err != nil || !canManage {
		util.AbortWithStrJson(c, http.StatusForbidden, "분석 내역은 방장만 삭제할 수 있습니다.")
		return
	}
	if analysis.Status == models.ReplayAnalysisStatusQueued || analysis.Status == models.ReplayAnalysisStatusUploading || analysis.Status == models.ReplayAnalysisStatusAnalyzing {
		util.AbortWithStrJson(c, http.StatusConflict, "진행 중인 분석은 삭제할 수 없습니다.")
		return
	}
	if err := models.DeleteCustomGameReplayAnalysis(db.Root, analysis.Id); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "분석 내역을 삭제하지 못했습니다.")
		return
	}
	if socket.SocketIO.Io != nil {
		socket.SocketIO.BroadcastToCustomConfigRoom(
			analysis.CustomGameConfigId,
			socket.EventCustomConfigReplayAnalysisUpdated,
			gin.H{"id": analysis.Id, "customGameId": analysis.CustomGameConfigId, "status": "deleted"},
		)
	}
	c.Status(http.StatusNoContent)
}
