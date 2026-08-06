package v1

import (
	"crypto/hmac"
	"net/http"
	"os"
	"strings"
	"team.gg-server/controllers/socket"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/util"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/shyunku-libraries/go-logger"
)

type replayAnalysisUpdateRequest struct {
	RequestId *string `json:"requestId"`
	Status    *string `json:"status"`
	Stage     *string `json:"stage"`
	Progress  *int    `json:"progress" binding:"omitempty,gte=0,lte=100"`
	Analysis  *string `json:"analysis"`
	Model     *string `json:"model"`
	Error     *string `json:"error"`
}

func replayAnalysisStatusTransitionAllowed(current, next string) bool {
	if current == next {
		return true
	}
	switch current {
	case models.ReplayAnalysisStatusQueued:
		return next == models.ReplayAnalysisStatusUploading || next == models.ReplayAnalysisStatusFailed
	case models.ReplayAnalysisStatusUploading:
		return next == models.ReplayAnalysisStatusAnalyzing || next == models.ReplayAnalysisStatusFailed
	case models.ReplayAnalysisStatusAnalyzing:
		return next == models.ReplayAnalysisStatusCompleted || next == models.ReplayAnalysisStatusFailed
	default:
		return false
	}
}

func useReplayAnalysisInternalRouter(g *gin.RouterGroup) {
	g.PUT("/internal/replay-analysis/:id", UpdateReplayAnalysisInternal)
}

func validReplayAnalyzerSecret(c *gin.Context) bool {
	expected := strings.TrimSpace(os.Getenv("REPLAY_ANALYZER_SHARED_SECRET"))
	provided := c.GetHeader("X-Replay-Analyzer-Secret")
	return expected != "" && hmac.Equal([]byte(expected), []byte(provided))
}

func UpdateReplayAnalysisInternal(c *gin.Context) {
	if !validReplayAnalyzerSecret(c) {
		util.AbortWithStrJson(c, http.StatusUnauthorized, "invalid replay analyzer credentials")
		return
	}
	var req replayAnalysisUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid replay analysis update")
		return
	}
	if req.Status == nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "replay analysis status is required")
		return
	}
	tx, err := db.Root.BeginTxx(c, nil)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to begin replay analysis update")
		return
	}
	defer func() { _ = tx.Rollback() }()
	analysis, exists, err := models.GetCustomGameReplayAnalysisByIdForUpdate(tx, c.Param("id"))
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to load replay analysis")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusNotFound, "replay analysis not found")
		return
	}
	if !replayAnalysisStatusTransitionAllowed(analysis.Status, *req.Status) {
		util.AbortWithStrJson(c, http.StatusConflict, "invalid replay analysis status transition")
		return
	}
	if req.Progress != nil && *req.Progress < analysis.Progress {
		util.AbortWithStrJson(c, http.StatusConflict, "replay analysis progress cannot move backwards")
		return
	}
	if req.RequestId != nil && analysis.RequestId != nil && *analysis.RequestId != *req.RequestId {
		util.AbortWithStrJson(c, http.StatusConflict, "replay analysis request is already claimed")
		return
	}
	if req.RequestId != nil {
		analysis.RequestId = req.RequestId
	}
	if req.Status != nil {
		switch *req.Status {
		case models.ReplayAnalysisStatusQueued, models.ReplayAnalysisStatusUploading,
			models.ReplayAnalysisStatusAnalyzing, models.ReplayAnalysisStatusCompleted,
			models.ReplayAnalysisStatusFailed:
			analysis.Status = *req.Status
		default:
			util.AbortWithStrJson(c, http.StatusBadRequest, "invalid replay analysis status")
			return
		}
	}
	if req.Stage != nil {
		analysis.Stage = *req.Stage
	}
	if req.Progress != nil {
		analysis.Progress = *req.Progress
	}
	if req.Analysis != nil {
		analysis.Analysis = req.Analysis
	}
	if req.Model != nil {
		analysis.Model = req.Model
	}
	if req.Error != nil {
		analysis.ErrorMessage = req.Error
	}
	if analysis.Status == models.ReplayAnalysisStatusCompleted {
		if analysis.Analysis == nil || strings.TrimSpace(*analysis.Analysis) == "" {
			util.AbortWithStrJson(c, http.StatusBadRequest, "completed replay analysis requires a result")
			return
		}
		analysis.Progress = 100
		completedAt := time.Now().UTC()
		analysis.CompletedAt = &completedAt
		analysis.ErrorMessage = nil
	}
	if analysis.Status == models.ReplayAnalysisStatusFailed {
		completedAt := time.Now().UTC()
		analysis.CompletedAt = &completedAt
	}
	analysis.UpdatedAt = time.Now().UTC()
	if err := analysis.Update(tx); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to update replay analysis")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to commit replay analysis update")
		return
	}
	if socket.SocketIO.Io != nil {
		socket.SocketIO.BroadcastToCustomConfigRoom(
			analysis.CustomGameConfigId,
			socket.EventCustomConfigReplayAnalysisUpdated,
			gin.H{"id": analysis.Id, "customGameId": analysis.CustomGameConfigId, "status": analysis.Status, "stage": analysis.Stage, "progress": analysis.Progress},
		)
	}
	c.Status(http.StatusNoContent)
}
