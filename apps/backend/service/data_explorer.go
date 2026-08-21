package service

import (
	"errors"
	"fmt"
	log "github.com/shyunku-libraries/go-logger"
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
	maxSupportedExplorerDepth    = 10
	maxExplorerCleanupBatch      = 5000
	minExplorerCleanupInterval   = 5 * time.Second
	minExplorerRetention         = time.Hour
	minExplorerRevisit           = 24 * time.Hour
)

type DataExplorer struct {
	leaseDuration        time.Duration
	pollInterval         time.Duration
	bootstrapInterval    time.Duration
	bootstrapBatchSize   int
	summonerWorkers      int
	matchWorkers         int
	maxAttempts          int
	dailySummonerBudget  int
	dailyMatchBudget     int
	recentMatchCount     int
	maxDepth             int
	participantDiscovery string
	bootstrapEnabled     bool
	cleanupEnabled       bool
	cleanupInterval      time.Duration
	cleanupBatchSize     int
	completedRetention   time.Duration
	sourceRetention      time.Duration
	summonerRevisit      time.Duration
	matchRevisit         time.Duration
	debugEnabled         bool
	statusLogInterval    time.Duration

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
