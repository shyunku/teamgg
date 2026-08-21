package service

import (
	"errors"
	"fmt"
	log "github.com/shyunku-libraries/go-logger"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"team.gg-server/core"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/third_party/riot/api"
	"team.gg-server/types"
	"time"
)

const (
	explorerSummonerBudgetKind = "summoner"
	explorerMatchBudgetKind    = "match"

	participantDiscoveryDisabled = "disabled"
	participantDiscoveryBounded  = "bounded"
	minExplorerMetricsInterval   = time.Minute
	minExplorerTempStatusSamples = 10
	maxSupportedExplorerDepth    = 10
	maxExplorerCleanupBatch      = 5000
	minExplorerCleanupInterval   = 5 * time.Second
	minExplorerRetention         = time.Hour
	minExplorerRevisit           = 24 * time.Hour
)

type DataExplorer struct {
	leaseDuration         time.Duration
	pollInterval          time.Duration
	bootstrapInterval     time.Duration
	bootstrapBatchSize    int
	summonerWorkers       int
	matchWorkers          int
	maxAttempts           int
	dailySummonerBudget   int
	dailyMatchBudget      int
	recentMatchCount      int
	maxDepth              int
	participantDiscovery  string
	bootstrapEnabled      bool
	cleanupEnabled        bool
	cleanupInterval       time.Duration
	cleanupBatchSize      int
	completedRetention    time.Duration
	sourceRetention       time.Duration
	summonerRevisit       time.Duration
	matchRevisit          time.Duration
	metricsEnabled        bool
	metricsInterval       time.Duration
	alertBudgetPercent    int64
	alertSummonerQueue    int64
	alertMatchQueue       int64
	alertFailedJobs       int64
	alertDatabaseBytes    int64
	alertDailyRowGrowth   int64
	alertTempDiskPercent  int64
	previousTempTables    int64
	previousTempDisk      int64
	tempStatusInitialized bool
	metricAlertStates     map[string]bool
	debugEnabled          bool
	statusLogInterval     time.Duration

	explored atomic.Int64
	success  atomic.Int64
	failed   atomic.Int64
}

func NewDataExplorer() *DataExplorer {
	return &DataExplorer{
		leaseDuration:        explorerEnvDuration("DATA_EXPLORER_LEASE", 5*time.Minute),
		pollInterval:         explorerEnvDuration("DATA_EXPLORER_POLL_INTERVAL", time.Second),
		bootstrapInterval:    explorerEnvDuration("DATA_EXPLORER_BOOTSTRAP_INTERVAL", 5*time.Second),
		bootstrapBatchSize:   explorerEnvInt("DATA_EXPLORER_BOOTSTRAP_BATCH_SIZE", 500),
		summonerWorkers:      explorerEnvInt("DATA_EXPLORER_SUMMONER_WORKERS", 1),
		matchWorkers:         explorerEnvInt("DATA_EXPLORER_MATCH_WORKERS", 2),
		maxAttempts:          explorerEnvInt("DATA_EXPLORER_MAX_ATTEMPTS", 8),
		dailySummonerBudget:  explorerEnvInt("DATA_EXPLORER_DAILY_SUMMONER_BUDGET", 500),
		dailyMatchBudget:     explorerEnvInt("DATA_EXPLORER_DAILY_MATCH_BUDGET", 1500),
		recentMatchCount:     explorerEnvInt("DATA_EXPLORER_MATCH_COUNT", types.DataExplorerLoadMatchesCount),
		maxDepth:             explorerEnvIntMax("DATA_EXPLORER_MAX_DEPTH", 0, maxSupportedExplorerDepth),
		participantDiscovery: explorerParticipantDiscoveryPolicy(),
		bootstrapEnabled:     explorerEnvBool("DATA_EXPLORER_BOOTSTRAP_ENABLED", false),
		cleanupEnabled:       explorerEnvBool("DATA_EXPLORER_CLEANUP_ENABLED", false),
		cleanupInterval:      explorerEnvDurationMin("DATA_EXPLORER_CLEANUP_INTERVAL", 30*time.Second, minExplorerCleanupInterval),
		cleanupBatchSize:     explorerEnvIntMax("DATA_EXPLORER_CLEANUP_BATCH_SIZE", 500, maxExplorerCleanupBatch),
		completedRetention:   explorerEnvDurationMin("DATA_EXPLORER_COMPLETED_JOB_RETENTION", 24*time.Hour, minExplorerRetention),
		sourceRetention:      explorerEnvDurationMin("DATA_EXPLORER_SOURCE_RETENTION", 24*time.Hour, minExplorerRetention),
		summonerRevisit:      explorerEnvDurationMin("DATA_EXPLORER_SUMMONER_REVISIT_INTERVAL", 30*24*time.Hour, minExplorerRevisit),
		matchRevisit:         explorerEnvDurationMin("DATA_EXPLORER_MATCH_REVISIT_INTERVAL", 365*24*time.Hour, minExplorerRevisit),
		metricsEnabled:       explorerEnvBool("DATA_EXPLORER_METRICS_ENABLED", true),
		metricsInterval:      explorerEnvDurationMin("DATA_EXPLORER_METRICS_INTERVAL", 5*time.Minute, minExplorerMetricsInterval),
		alertBudgetPercent:   explorerEnvPercent("DATA_EXPLORER_ALERT_BUDGET_PERCENT", 80),
		alertSummonerQueue:   explorerEnvInt64("DATA_EXPLORER_ALERT_SUMMONER_QUEUE", 10000),
		alertMatchQueue:      explorerEnvInt64("DATA_EXPLORER_ALERT_MATCH_QUEUE", 10000),
		alertFailedJobs:      explorerEnvInt64("DATA_EXPLORER_ALERT_FAILED_JOBS", 100),
		alertDatabaseBytes:   explorerEnvBytes("DATA_EXPLORER_ALERT_DATABASE_BYTES", 0),
		alertDailyRowGrowth:  explorerEnvInt64("DATA_EXPLORER_ALERT_DAILY_ROW_GROWTH", 1000000),
		alertTempDiskPercent: explorerEnvPercent("DATA_EXPLORER_ALERT_TEMP_DISK_PERCENT", 25),
		metricAlertStates:    make(map[string]bool),
		debugEnabled:         explorerEnvBool("DATA_EXPLORER_DEBUG", core.DebugMode),
		statusLogInterval:    explorerEnvDuration("DATA_EXPLORER_STATUS_LOG_INTERVAL", 30*time.Second),
	}
}

func explorerEnvIntMax(key string, fallback, maximum int) int {
	value := explorerEnvInt(key, fallback)
	if value > maximum {
		return maximum
	}
	return value
}

func explorerParticipantDiscoveryPolicy() string {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("DATA_EXPLORER_PARTICIPANT_DISCOVERY")))
	if value == participantDiscoveryBounded {
		return participantDiscoveryBounded
	}
	return participantDiscoveryDisabled
}

func explorerEnvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	default:
		return fallback
	}
}

func explorerEnvInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func explorerEnvInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func explorerEnvBytes(key string, fallback int64) int64 {
	value := strings.ToUpper(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}

	unit := ""
	for _, candidate := range []string{"TB", "GB", "MB", "KB", "T", "G", "M", "K", "B"} {
		if strings.HasSuffix(value, candidate) {
			unit = candidate
			value = strings.TrimSpace(strings.TrimSuffix(value, candidate))
			break
		}
	}
	if unit == "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 0 {
			return fallback
		}
		return parsed
	}

	multiplier := float64(1)
	switch unit {
	case "K", "KB":
		multiplier = 1 << 10
	case "M", "MB":
		multiplier = 1 << 20
	case "G", "GB":
		multiplier = 1 << 30
	case "T", "TB":
		multiplier = 1 << 40
	}

	parsed, err := strconv.ParseFloat(value, 64)
	bytes := parsed * multiplier
	if err != nil || parsed < 0 || math.IsNaN(bytes) || math.IsInf(bytes, 0) || bytes >= float64(math.MaxInt64) {
		return fallback
	}
	return int64(bytes)
}

func explorerEnvPercent(key string, fallback int64) int64 {
	value := explorerEnvInt64(key, fallback)
	if value > 100 {
		return 100
	}
	return value
}
func explorerEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}

func explorerEnvDurationMin(key string, fallback, minimum time.Duration) time.Duration {
	duration := explorerEnvDuration(key, fallback)
	if duration < minimum {
		return minimum
	}
	return duration
}

func IsDataExplorerEnabled() bool {
	return explorerEnvBool("DATA_EXPLORER_ENABLED", false)
}

func (de *DataExplorer) Loop() {
	if !IsDataExplorerEnabled() {
		log.Info("DataExplorer is disabled")
		return
	}

	log.Infof("DataExplorer enabled=%t debug=%t", true, de.debugEnabled)
	log.Infof(
		"DataExplorer config: summonerWorkers=%d matchWorkers=%d matchCount=%d dailyBudget=%d/%d lease=%s poll=%s bootstrap=%t/%d/%s participantDiscovery=%s maxDepth=%d revisit=%s/%s cleanup=%t/%s/%d retention=%s/%s",
		de.summonerWorkers, de.matchWorkers, de.recentMatchCount,
		de.dailySummonerBudget, de.dailyMatchBudget, de.leaseDuration,
		de.pollInterval, de.bootstrapEnabled, de.bootstrapBatchSize, de.bootstrapInterval,
		de.participantDiscovery, de.maxDepth,
		de.summonerRevisit, de.matchRevisit,
		de.cleanupEnabled, de.cleanupInterval, de.cleanupBatchSize,
		de.completedRetention, de.sourceRetention,
	)
	log.Infof(
		"event=data_explorer_metrics_config enabled=%t interval=%s alert_budget_percent=%d alert_summoner_queue=%d alert_match_queue=%d alert_failed_jobs=%d alert_database_bytes=%d alert_daily_row_growth=%d alert_temp_disk_percent=%d",
		de.metricsEnabled, de.metricsInterval, de.alertBudgetPercent,
		de.alertSummonerQueue, de.alertMatchQueue, de.alertFailedJobs,
		de.alertDatabaseBytes, de.alertDailyRowGrowth, de.alertTempDiskPercent,
	)

	var workers sync.WaitGroup
	if err := models.RecoverExpiredDataExplorerJobs(db.Root); err != nil {
		log.Errorf("DataExplorer initial lease recovery failed: %v", err)
	}
	workers.Add(1)
	go func() {
		de.recoveryWorker()
		workers.Done()
	}()
	if de.debugEnabled {
		workers.Add(1)
		go func() {
			de.statusWorker()
			workers.Done()
		}()
	}
	if de.metricsEnabled {
		workers.Add(1)
		go func() {
			de.metricsWorker()
			workers.Done()
		}()
	} else {
		log.Info("event=data_explorer_metrics_config enabled=false")
	}

	if de.bootstrapEnabled {
		workers.Add(1)
		go func() {
			de.bootstrapExistingParticipants()
			workers.Done()
		}()
	} else {
		log.Info("DataExplorer participant bootstrap is disabled")
	}
	workers.Add(1)
	go func() {
		de.maintenanceWorker()
		workers.Done()
	}()
	if !de.cleanupEnabled {
		log.Info("DataExplorer completed job cleanup is disabled")
	}
	for i := 0; i < de.summonerWorkers; i++ {
		workers.Add(1)
		go func() {
			de.summonerWorker()
			workers.Done()
		}()
	}
	for i := 0; i < de.matchWorkers; i++ {
		workers.Add(1)
		go func() {
			de.matchWorker()
			workers.Done()
		}()
	}
	workers.Wait()
}

func (de *DataExplorer) maintenanceWorker() {
	ticker := time.NewTicker(de.cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		backfill, err := models.BackfillDataExplorerProcessingState(
			db.Root,
			de.summonerRevisit,
			de.matchRevisit,
			de.cleanupBatchSize,
		)
		if err != nil {
			log.Errorf("DataExplorer processing state backfill failed: %v", err)
			continue
		}
		if de.debugEnabled && (backfill.Summoners > 0 || backfill.Matches > 0) {
			log.Debugf(
				"DataExplorer processing state backfill: summoners=%d matches=%d",
				backfill.Summoners,
				backfill.Matches,
			)
		}
		if !de.cleanupEnabled {
			continue
		}
		result, err := models.CleanupDataExplorerCompletedRows(
			db.Root,
			de.completedRetention,
			de.sourceRetention,
			de.cleanupBatchSize,
		)
		if err != nil {
			log.Errorf("DataExplorer cleanup failed: %v", err)
			continue
		}
		if de.debugEnabled && (result.SummonerJobs > 0 || result.MatchJobs > 0 || result.MatchSources > 0) {
			log.Debugf(
				"DataExplorer cleanup: summonerJobs=%d matchJobs=%d matchSources=%d",
				result.SummonerJobs,
				result.MatchJobs,
				result.MatchSources,
			)
		}
	}
}

func (de *DataExplorer) statusWorker() {
	de.logStatus()
	ticker := time.NewTicker(de.statusLogInterval)
	defer ticker.Stop()
	for range ticker.C {
		de.logStatus()
	}
}

func (de *DataExplorer) logStatus() {
	diagnostics, err := models.GetDataExplorerDiagnostics(db.Root)
	if err != nil {
		log.Errorf("DataExplorer diagnostics failed: %v", err)
		return
	}
	log.Debugf(
		"DataExplorer status: bootstrap={completed:%t cursor:%s/%d} summoner={pending:%d processing:%d done:%d failed:%d usage:%d/%d} match={pending:%d processing:%d done:%d failed:%d usage:%d/%d} runtime={processed:%d success:%d failed:%d}",
		diagnostics.BootstrapCompleted, diagnostics.BootstrapMatchId, diagnostics.BootstrapParticipant,
		diagnostics.SummonerJobs[models.DataExplorerJobPending], diagnostics.SummonerJobs[models.DataExplorerJobProcessing],
		diagnostics.SummonerJobs[models.DataExplorerJobDone], diagnostics.SummonerJobs[models.DataExplorerJobFailed],
		diagnostics.DailyUsage[explorerSummonerBudgetKind], de.dailySummonerBudget,
		diagnostics.MatchJobs[models.DataExplorerJobPending], diagnostics.MatchJobs[models.DataExplorerJobProcessing],
		diagnostics.MatchJobs[models.DataExplorerJobDone], diagnostics.MatchJobs[models.DataExplorerJobFailed],
		diagnostics.DailyUsage[explorerMatchBudgetKind], de.dailyMatchBudget,
		de.explored.Load(), de.success.Load(), de.failed.Load(),
	)
}

type dataExplorerMetricCheck struct {
	name      string
	value     int64
	threshold int64
	available bool
}

func (de *DataExplorer) metricsWorker() {
	de.logOperationalMetrics()
	ticker := time.NewTicker(de.metricsInterval)
	defer ticker.Stop()
	for range ticker.C {
		de.logOperationalMetrics()
	}
}

func (de *DataExplorer) logOperationalMetrics() {
	diagnostics, err := models.GetDataExplorerDiagnostics(db.Root)
	if err != nil {
		log.Errorf("event=data_explorer_metrics result=failed stage=diagnostics error=%v", err)
		return
	}
	queueRows := explorerJobTotal(diagnostics.SummonerJobs) + explorerJobTotal(diagnostics.MatchJobs)
	metrics, err := models.CollectDataExplorerOperationalMetrics(db.Root, queueRows)
	if err != nil {
		log.Errorf("event=data_explorer_metrics result=failed stage=collection error=%v", err)
		return
	}

	tempTablesDelta, tempDiskDelta, tempIntervalAvailable := de.tempStatusInterval(metrics)
	tempDiskPercent := int64(-1)
	if tempIntervalAvailable && tempTablesDelta > 0 {
		tempDiskPercent = tempDiskDelta * 100 / tempTablesDelta
	}
	log.Infof(
		"event=data_explorer_metrics result=ok metric_date=%s interval=%s summoner_usage=%d summoner_budget=%d match_usage=%d match_budget=%d summoner_pending=%d summoner_processing=%d summoner_failed=%d match_pending=%d match_processing=%d match_failed=%d summoner_rows_estimated=%d match_rows_estimated=%d mastery_rows_estimated=%d queue_rows=%d daily_summoner_growth_estimated=%d daily_match_growth_estimated=%d daily_mastery_growth_estimated=%d daily_queue_growth=%d database_bytes=%d database_reclaimable_bytes=%d temp_tables_delta=%d temp_disk_tables_delta=%d temp_disk_percent=%d temp_status_available=%t temp_allocated_bytes=%d temp_free_bytes=%d temp_space_available=%t",
		metrics.MetricDate.Format("2006-01-02"), de.metricsInterval,
		diagnostics.DailyUsage[explorerSummonerBudgetKind], de.dailySummonerBudget,
		diagnostics.DailyUsage[explorerMatchBudgetKind], de.dailyMatchBudget,
		diagnostics.SummonerJobs[models.DataExplorerJobPending],
		diagnostics.SummonerJobs[models.DataExplorerJobProcessing],
		diagnostics.SummonerJobs[models.DataExplorerJobFailed],
		diagnostics.MatchJobs[models.DataExplorerJobPending],
		diagnostics.MatchJobs[models.DataExplorerJobProcessing],
		diagnostics.MatchJobs[models.DataExplorerJobFailed],
		metrics.SummonerRows, metrics.MatchRows, metrics.MasteryRows, metrics.QueueRows,
		metrics.DailySummonerRowGrowth, metrics.DailyMatchRowGrowth,
		metrics.DailyMasteryRowGrowth, metrics.DailyQueueRowGrowth,
		metrics.DatabaseBytes, metrics.DatabaseFreeBytes,
		tempTablesDelta, tempDiskDelta, tempDiskPercent, tempIntervalAvailable,
		metrics.TempAllocatedBytes, metrics.TempFreeBytes, metrics.TempSpaceAvailable,
	)

	de.updateMetricAlerts(de.metricChecks(
		diagnostics,
		metrics,
		tempTablesDelta,
		tempDiskDelta,
		tempIntervalAvailable,
	))
}

func explorerJobTotal(counts map[string]int64) int64 {
	return counts[models.DataExplorerJobPending] +
		counts[models.DataExplorerJobProcessing] +
		counts[models.DataExplorerJobDone] +
		counts[models.DataExplorerJobFailed]
}

func (de *DataExplorer) tempStatusInterval(metrics *models.DataExplorerOperationalMetrics) (int64, int64, bool) {
	if !metrics.TempStatusAvailable {
		return -1, -1, false
	}
	if !de.tempStatusInitialized {
		de.previousTempTables = metrics.CreatedTempTables
		de.previousTempDisk = metrics.CreatedTempDiskTables
		de.tempStatusInitialized = true
		return -1, -1, false
	}
	tables := metrics.CreatedTempTables - de.previousTempTables
	disk := metrics.CreatedTempDiskTables - de.previousTempDisk
	de.previousTempTables = metrics.CreatedTempTables
	de.previousTempDisk = metrics.CreatedTempDiskTables
	if tables < 0 || disk < 0 {
		return -1, -1, false
	}
	return tables, disk, true
}

func (de *DataExplorer) metricChecks(
	diagnostics *models.DataExplorerDiagnostics,
	metrics *models.DataExplorerOperationalMetrics,
	tempTablesDelta int64,
	tempDiskDelta int64,
	tempIntervalAvailable bool,
) []dataExplorerMetricCheck {
	checks := []dataExplorerMetricCheck{
		{name: "summoner_queue_pending", value: diagnostics.SummonerJobs[models.DataExplorerJobPending], threshold: de.alertSummonerQueue, available: true},
		{name: "match_queue_pending", value: diagnostics.MatchJobs[models.DataExplorerJobPending], threshold: de.alertMatchQueue, available: true},
		{name: "failed_jobs", value: diagnostics.SummonerJobs[models.DataExplorerJobFailed] + diagnostics.MatchJobs[models.DataExplorerJobFailed], threshold: de.alertFailedJobs, available: true},
		{name: "database_bytes", value: metrics.DatabaseBytes, threshold: de.alertDatabaseBytes, available: true},
		{name: "daily_summoner_growth_estimated", value: metrics.DailySummonerRowGrowth, threshold: de.alertDailyRowGrowth, available: true},
		{name: "daily_match_growth_estimated", value: metrics.DailyMatchRowGrowth, threshold: de.alertDailyRowGrowth, available: true},
		{name: "daily_mastery_growth_estimated", value: metrics.DailyMasteryRowGrowth, threshold: de.alertDailyRowGrowth, available: true},
		{name: "daily_queue_growth", value: metrics.DailyQueueRowGrowth, threshold: de.alertDailyRowGrowth, available: true},
	}
	if de.dailySummonerBudget > 0 {
		checks = append(checks, dataExplorerMetricCheck{
			name: "summoner_budget_percent", value: diagnostics.DailyUsage[explorerSummonerBudgetKind] * 100 / int64(de.dailySummonerBudget),
			threshold: de.alertBudgetPercent, available: true,
		})
	}
	if de.dailyMatchBudget > 0 {
		checks = append(checks, dataExplorerMetricCheck{
			name: "match_budget_percent", value: diagnostics.DailyUsage[explorerMatchBudgetKind] * 100 / int64(de.dailyMatchBudget),
			threshold: de.alertBudgetPercent, available: true,
		})
	}
	if tempIntervalAvailable && tempTablesDelta >= minExplorerTempStatusSamples {
		checks = append(checks, dataExplorerMetricCheck{
			name: "temp_disk_percent", value: tempDiskDelta * 100 / tempTablesDelta,
			threshold: de.alertTempDiskPercent, available: true,
		})
	}
	return checks
}

func (de *DataExplorer) updateMetricAlerts(checks []dataExplorerMetricCheck) {
	for _, check := range checks {
		if !check.available || check.threshold <= 0 {
			continue
		}
		firing := check.value >= check.threshold
		wasFiring := de.metricAlertStates[check.name]
		switch {
		case firing && !wasFiring:
			log.Warnf(
				"event=data_explorer_alert state=firing metric=%s value=%d threshold=%d",
				check.name, check.value, check.threshold,
			)
		case !firing && wasFiring:
			log.Infof(
				"event=data_explorer_alert state=recovered metric=%s value=%d threshold=%d",
				check.name, check.value, check.threshold,
			)
		}
		de.metricAlertStates[check.name] = firing
	}
}
func (de *DataExplorer) logJob(kind, id, result string, count int64) {
	if !de.debugEnabled || (count > 10 && count%100 != 0) {
		return
	}
	log.Infof("DataExplorer job: kind=%s id=%s result=%s processed=%d", kind, id, result, count)
}

func (de *DataExplorer) shouldLogProgress() bool {
	count := de.explored.Load()
	return de.debugEnabled && (count <= 10 || count%100 == 0)
}

func (de *DataExplorer) recoveryWorker() {
	interval := de.leaseDuration / 2
	if interval > time.Minute {
		interval = time.Minute
	}
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	for {
		time.Sleep(interval)
		if err := models.RecoverExpiredDataExplorerJobs(db.Root); err != nil {
			log.Errorf("DataExplorer lease recovery failed: %v", err)
		}
	}
}

func (de *DataExplorer) bootstrapExistingParticipants() {
	for {
		participants, completed, err := models.LoadDataExplorerBootstrapBatch(db.Root, de.bootstrapBatchSize)
		if err != nil {
			log.Errorf("DataExplorer bootstrap failed: %v", err)
			time.Sleep(de.retryDelay(1))
			continue
		}
		enqueueFailed := false
		for _, participant := range participants {
			fromMatchId := participant.MatchId
			if err := models.EnqueueDataExplorerSummonerJob(db.Root, participant.Puuid, -20, 0, &fromMatchId); err != nil {
				log.Errorf("DataExplorer bootstrap enqueue failed: %v", err)
				enqueueFailed = true
				break
			}
		}
		if enqueueFailed {
			time.Sleep(de.retryDelay(1))
			continue
		}
		if completed {
			if err := models.AdvanceDataExplorerBootstrapCursor(db.Root, nil, true); err != nil {
				log.Errorf("DataExplorer bootstrap completion failed: %v", err)
				time.Sleep(de.retryDelay(1))
				continue
			}
			log.Info("DataExplorer participant bootstrap completed")
			return
		}
		last := participants[len(participants)-1]
		if err := models.AdvanceDataExplorerBootstrapCursor(db.Root, &last, false); err != nil {
			log.Errorf("DataExplorer bootstrap cursor update failed: %v", err)
			time.Sleep(de.retryDelay(1))
			continue
		}
		if de.debugEnabled {
			log.Infof("DataExplorer bootstrap: enqueued=%d cursor=%s/%d", len(participants), last.MatchId, last.ParticipantId)
		}
		time.Sleep(de.bootstrapInterval)
	}
}

func (de *DataExplorer) summonerWorker() {
	for {
		job, found, err := models.ClaimDataExplorerSummonerJob(de.leaseDuration)
		if err != nil {
			log.Errorf("DataExplorer summoner claim failed: %v", err)
			time.Sleep(de.retryDelay(1))
			continue
		}
		if !found {
			time.Sleep(de.pollInterval)
			continue
		}
		de.explored.Add(1)
		processed := de.explored.Load()
		if err := de.processSummonerJob(job); err != nil {
			de.failed.Add(1)
			de.logJob("summoner", job.Puuid, "failed", processed)
			if errors.Is(err, ErrRiotIdentityNotFound) {
				log.Warnf("DataExplorer summoner permanently skipped: puuid=%s reason=%v", job.Puuid, err)
				if failErr := models.FailDataExplorerSummonerJob(db.Root, job.Puuid, err); failErr != nil {
					log.Errorf("DataExplorer summoner failure scheduling failed: %v", failErr)
				}
				continue
			}
			log.Errorf("DataExplorer summoner %s failed: %v", job.Puuid, err)
			if retryErr := models.RetryDataExplorerSummonerJob(
				db.Root, job.Puuid, job.Attempts, de.maxAttempts,
				de.retryDelay(job.Attempts), err,
			); retryErr != nil {
				log.Errorf("DataExplorer summoner retry scheduling failed: %v", retryErr)
			}
			continue
		}
		de.success.Add(1)
		de.logJob("summoner", job.Puuid, "completed", processed)
	}
}

func (de *DataExplorer) processSummonerJob(job *models.DataExplorerSummonerJobDAO) error {
	_, exists, err := models.GetSummonerDAO_byPuuid(db.Root, job.Puuid)
	if err != nil {
		return err
	}
	// A first-attempt job for a summoner already stored is bootstrap noise. A
	// retry may have stored only the summoner before a later API call failed.
	if exists && job.Attempts == 1 {
		if de.shouldLogProgress() {
			log.Infof("DataExplorer summoner skipped: puuid=%s reason=already_cached", job.Puuid)
		}
		return models.CompleteDataExplorerSummonerJob(db.Root, job.Puuid, de.summonerRevisit)
	}

	allowed, err := models.ConsumeDataExplorerDailyBudget(db.Root, explorerSummonerBudgetKind, de.dailySummonerBudget)
	if err != nil {
		return err
	}
	if !allowed {
		if de.shouldLogProgress() {
			log.Infof("DataExplorer summoner deferred: puuid=%s reason=daily_budget", job.Puuid)
		}
		return models.DeferDataExplorerSummonerJob(db.Root, job.Puuid)
	}

	summoner, _, err := RenewSummonerInfoByPuuid(db.Root, job.Puuid)
	if err != nil {
		return err
	}
	if err := RenewSummonerLeague(db.Root, summoner.Puuid); err != nil {
		return err
	}
	if err := RenewSummonerMastery(db.Root, summoner.Puuid); err != nil {
		return err
	}

	matchIds, err := api.GetMatchIdsInterval(summoner.Puuid, &api.MatchIdsReqOption{
		QueueId: types.QueueTypeAll,
		Count:   de.recentMatchCount,
	})
	if err != nil {
		return err
	}
	for _, matchId := range *matchIds {
		if err := models.EnqueueDataExplorerMatchJob(db.Root, matchId, summoner.Puuid, job.Priority, job.Depth); err != nil {
			return err
		}
	}
	return models.CompleteDataExplorerSummonerJob(db.Root, job.Puuid, de.summonerRevisit)
}

func (de *DataExplorer) matchWorker() {
	for {
		job, found, err := models.ClaimDataExplorerMatchJob(de.leaseDuration)
		if err != nil {
			log.Errorf("DataExplorer match claim failed: %v", err)
			time.Sleep(de.retryDelay(1))
			continue
		}
		if !found {
			time.Sleep(de.pollInterval)
			continue
		}
		de.explored.Add(1)
		processed := de.explored.Load()
		if err := de.processMatchJob(job); err != nil {
			de.failed.Add(1)
			de.logJob("match", job.MatchId, "failed", processed)
			log.Errorf("DataExplorer match %s failed: %v", job.MatchId, err)
			if retryErr := models.RetryDataExplorerMatchJob(
				db.Root, job.MatchId, job.Attempts, de.maxAttempts,
				de.retryDelay(job.Attempts), err,
			); retryErr != nil {
				log.Errorf("DataExplorer match retry scheduling failed: %v", retryErr)
			}
			continue
		}
		de.success.Add(1)
		de.logJob("match", job.MatchId, "completed", processed)
	}
}

func (de *DataExplorer) processMatchJob(job *models.DataExplorerMatchJobDAO) error {
	sources, err := models.GetDataExplorerMatchSources(db.Root, job.MatchId)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return fmt.Errorf("match job has no source summoner")
	}

	_, exists, err := models.GetMatchDAO(db.Root, job.MatchId)
	if err != nil {
		return err
	}
	if exists {
		if de.shouldLogProgress() {
			log.Infof("DataExplorer match skipped: matchId=%s reason=already_cached sources=%d", job.MatchId, len(sources))
		}
		for _, puuid := range sources {
			link := models.SummonerMatchDAO{Puuid: puuid, MatchId: job.MatchId}
			if err := link.Upsert(db.Root); err != nil {
				return err
			}
		}
		if err := de.discoverMatchParticipants(job); err != nil {
			return err
		}
		return models.CompleteDataExplorerMatchJob(db.Root, job.MatchId, de.matchRevisit)
	}

	allowed, err := models.ConsumeDataExplorerDailyBudget(db.Root, explorerMatchBudgetKind, de.dailyMatchBudget)
	if err != nil {
		return err
	}
	if !allowed {
		if de.shouldLogProgress() {
			log.Infof("DataExplorer match deferred: matchId=%s reason=daily_budget", job.MatchId)
		}
		return models.DeferDataExplorerMatchJob(db.Root, job.MatchId)
	}

	match, err := api.GetMatchByMatchId(job.MatchId)
	if err != nil {
		return err
	}
	if err := SaveDataExplorerMatch(*match, sources); err != nil {
		return err
	}
	if err := de.discoverMatchParticipants(job); err != nil {
		return err
	}
	return models.CompleteDataExplorerMatchJob(db.Root, job.MatchId, de.matchRevisit)
}

func (de *DataExplorer) shouldDiscoverMatchParticipants(matchDepth int) bool {
	return de.participantDiscovery == participantDiscoveryBounded &&
		matchDepth >= 0 && matchDepth < de.maxDepth
}

func (de *DataExplorer) discoverMatchParticipants(job *models.DataExplorerMatchJobDAO) error {
	if !de.shouldDiscoverMatchParticipants(job.Depth) {
		return nil
	}
	participants, err := models.GetMatchParticipantDAOs(db.Root, job.MatchId)
	if err != nil {
		return err
	}
	nextDepth := job.Depth + 1
	fromMatchId := job.MatchId
	for _, participant := range participants {
		if err := models.EnqueueDataExplorerSummonerJob(
			db.Root, participant.Puuid, job.Priority-10, nextDepth, &fromMatchId,
		); err != nil {
			return err
		}
	}
	return nil
}

func (de *DataExplorer) retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<uint(attempt-1))*time.Second + time.Duration(attempt*137)*time.Millisecond
}

// Explore remains as a compatibility hook for diagnostics. Production uses
// the continuously running workers in Loop.
func (de *DataExplorer) Explore() bool {
	job, found, err := models.ClaimDataExplorerSummonerJob(de.leaseDuration)
	if err != nil || !found {
		return false
	}
	de.explored.Add(1)
	if err := de.processSummonerJob(job); err != nil {
		_ = models.RetryDataExplorerSummonerJob(
			db.Root, job.Puuid, job.Attempts, de.maxAttempts,
			de.retryDelay(job.Attempts), err,
		)
		return false
	}
	de.success.Add(1)
	return true
}

func (de *DataExplorer) GetExploreCaches() int {
	return int(de.explored.Load())
}
