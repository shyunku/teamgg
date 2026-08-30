package v1

import (
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"team.gg-server/core"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/util"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/shyunku-libraries/go-logger"
)

const (
	adminInternalSecretHeader = "X-Teamgg-Admin-Secret"
	adminEventLimitDefault    = 100
	adminEventLimitMaximum    = 200
)

type adminAuthorizeResponse struct {
	Uid    string `json:"uid"`
	UserId string `json:"userId"`
	Role   string `json:"role"`
}

type adminAuditRequest struct {
	ActorUid string                 `json:"actorUid" binding:"required"`
	Action   string                 `json:"action" binding:"required"`
	Resource string                 `json:"resource" binding:"required"`
	Result   string                 `json:"result" binding:"required"`
	ClientIp string                 `json:"clientIp"`
	Metadata map[string]interface{} `json:"metadata"`
}

func useAdminInternalRouter(g *gin.RouterGroup) {
	g.POST("/internal/admin/authorize", AuthorizeAdminInternal)
	g.GET("/internal/admin/overview", GetAdminOverviewInternal)
	g.GET("/internal/admin/events", GetAdminEventsInternal)
	g.POST("/internal/admin/audit", CreateAdminAuditInternal)
}

func validAdminInternalSecret(c *gin.Context) bool {
	expected := strings.TrimSpace(os.Getenv("ADMIN_INTERNAL_SECRET"))
	provided := c.GetHeader(adminInternalSecretHeader)
	return expected != "" && hmac.Equal([]byte(expected), []byte(provided))
}

func requireAdminInternalSecret(c *gin.Context) bool {
	if validAdminInternalSecret(c) {
		return true
	}
	util.AbortWithStrJson(c, http.StatusUnauthorized, "invalid admin service credentials")
	return false
}

func resolveAdminRole(uid string) (string, *models.UserDAO, error) {
	user, exists, err := models.GetUserDAO_byUid(db.Root, uid)
	if err != nil || !exists {
		if err == nil {
			err = fmt.Errorf("user %s not found", uid)
		}
		return "", nil, err
	}
	role, roleExists, err := models.GetUserRole(db.Root, uid)
	if err != nil {
		return "", nil, err
	}
	if roleExists && (role == models.UserRoleAdmin || role == models.UserRoleViewer) {
		return role, user, nil
	}
	for _, identifier := range strings.Split(os.Getenv("ADMIN_BOOTSTRAP_USER_IDS"), ",") {
		identifier = strings.TrimSpace(identifier)
		if identifier != "" && (identifier == uid || strings.EqualFold(identifier, user.UserId)) {
			return models.UserRoleAdmin, user, nil
		}
	}
	return "", user, nil
}

func AuthorizeAdminInternal(c *gin.Context) {
	if !requireAdminInternalSecret(c) {
		return
	}
	uid, ok := requireAccessToken(c)
	if !ok {
		return
	}
	role, user, err := resolveAdminRole(uid)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to resolve admin role")
		return
	}
	if role == "" {
		util.AbortWithStrJson(c, http.StatusForbidden, "administrator permission is required")
		return
	}
	c.JSON(http.StatusOK, adminAuthorizeResponse{Uid: uid, UserId: user.UserId, Role: role})
}

func GetAdminOverviewInternal(c *gin.Context) {
	if !requireAdminInternalSecret(c) {
		return
	}
	diagnostics, err := models.GetDataExplorerDiagnostics(db.Root)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to load data explorer diagnostics")
		return
	}
	queueRows := adminJobTotal(diagnostics.SummonerJobs) + adminJobTotal(diagnostics.MatchJobs)
	metrics, err := getAdminOperationalMetrics(queueRows)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to load operational metrics")
		return
	}
	replayCounts, err := models.GetReplayAnalysisStatusCounts(db.Root)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to load replay status")
		return
	}
	statistics, err := models.GetStatisticsSnapshotSummaries(db.Root)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to load statistics status")
		return
	}
	migration, err := models.GetLatestMigrationSummary(db.Root)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to load migration status")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"generatedAt":    time.Now().UTC(),
		"backend":        gin.H{"version": core.Version, "isProduction": core.IsProduction},
		"dataExplorer":   gin.H{"diagnostics": diagnostics, "metrics": metrics},
		"replayAnalyses": replayCounts,
		"statistics":     statistics,
		"migration":      migration,
	})
}

func GetAdminEventsInternal(c *gin.Context) {
	if !requireAdminInternalSecret(c) {
		return
	}
	limit := adminEventLimit(c.Query("limit"))
	events, err := models.ListAdminOperationalEvents(db.Root, limit)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to load operational events")
		return
	}
	audits, err := models.ListAdminAuditLogs(db.Root, limit)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to load audit logs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events, "audits": audits})
}

func CreateAdminAuditInternal(c *gin.Context) {
	if !requireAdminInternalSecret(c) {
		return
	}
	var req adminAuditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid audit event")
		return
	}
	metadata, err := sanitizedAdminMetadata(req.Metadata)
	if err != nil {
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid audit metadata")
		return
	}
	if err := models.InsertAdminAuditLog(db.Root, &models.AdminAuditLogDAO{
		ActorUid: req.ActorUid,
		Action:   truncateAdminValue(req.Action, 64), Resource: truncateAdminValue(req.Resource, 128),
		Result: truncateAdminValue(req.Result, 24), ClientIp: truncateAdminValue(req.ClientIp, 64),
		MetadataJson: metadata,
	}); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to save audit event")
		return
	}
	c.Status(http.StatusNoContent)
}

var adminOperationalMetricsCache struct {
	sync.Mutex
	value     *models.DataExplorerOperationalMetrics
	expiresAt time.Time
}

func getAdminOperationalMetrics(queueRows int64) (*models.DataExplorerOperationalMetrics, error) {
	now := time.Now()
	adminOperationalMetricsCache.Lock()
	defer adminOperationalMetricsCache.Unlock()
	if adminOperationalMetricsCache.value != nil && now.Before(adminOperationalMetricsCache.expiresAt) {
		value := *adminOperationalMetricsCache.value
		return &value, nil
	}
	metrics, err := models.CollectDataExplorerOperationalMetrics(db.Root, queueRows)
	if err != nil {
		return nil, err
	}
	value := *metrics
	adminOperationalMetricsCache.value = &value
	adminOperationalMetricsCache.expiresAt = now.Add(time.Minute)
	return metrics, nil
}
func adminEventLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit < 1 {
		return adminEventLimitDefault
	}
	if limit > adminEventLimitMaximum {
		return adminEventLimitMaximum
	}
	return limit
}

func adminJobTotal(counts map[string]int64) int64 {
	return counts[models.DataExplorerJobPending] + counts[models.DataExplorerJobProcessing] +
		counts[models.DataExplorerJobDone] + counts[models.DataExplorerJobFailed]
}

func sanitizedAdminMetadata(metadata map[string]interface{}) (*string, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	sanitized := sanitizeAdminValue(metadata)
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return nil, err
	}
	value := truncateAdminValue(string(encoded), 4000)
	return &value, nil
}

func sanitizeAdminValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
				strings.Contains(lower, "password") || strings.Contains(lower, "cookie") ||
				strings.Contains(lower, "authorization") {
				result[key] = "[REDACTED]"
				continue
			}
			result[key] = sanitizeAdminValue(nested)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(typed))
		for index, nested := range typed {
			result[index] = sanitizeAdminValue(nested)
		}
		return result
	case string:
		return truncateAdminValue(typed, 500)
	default:
		return value
	}
}

func truncateAdminValue(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maximum {
		return value
	}
	return value[:maximum]
}
