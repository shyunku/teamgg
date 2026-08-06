package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"team.gg-server/libs/db"
	"time"
)

const (
	DataExplorerJobPending    = "pending"
	DataExplorerJobProcessing = "processing"
	DataExplorerJobDone       = "done"
	DataExplorerJobFailed     = "failed"
)

type DataExplorerSummonerJobDAO struct {
	Puuid                 string     `db:"puuid"`
	Status                string     `db:"status"`
	Priority              int        `db:"priority"`
	Depth                 int        `db:"depth"`
	Attempts              int        `db:"attempts"`
	NextAttemptAt         time.Time  `db:"next_attempt_at"`
	LeaseUntil            *time.Time `db:"lease_until"`
	DiscoveredFromMatchId *string    `db:"discovered_from_match_id"`
	LastError             *string    `db:"last_error"`
}

type DataExplorerMatchJobDAO struct {
	MatchId       string     `db:"match_id"`
	Status        string     `db:"status"`
	Priority      int        `db:"priority"`
	Depth         int        `db:"depth"`
	Attempts      int        `db:"attempts"`
	NextAttemptAt time.Time  `db:"next_attempt_at"`
	LeaseUntil    *time.Time `db:"lease_until"`
	LastError     *string    `db:"last_error"`
}

type DataExplorerParticipantCursor struct {
	MatchId       string `db:"match_id"`
	ParticipantId int    `db:"participant_id"`
	Puuid         string `db:"puuid"`
}

type DataExplorerDiagnostics struct {
	SummonerJobs         map[string]int64
	MatchJobs            map[string]int64
	DailyUsage           map[string]int64
	BootstrapMatchId     string
	BootstrapParticipant int
	BootstrapCompleted   bool
}

type dataExplorerStatusCount struct {
	Status string `db:"status"`
	Count  int64  `db:"count"`
}

type dataExplorerUsageCount struct {
	Kind  string `db:"usage_kind"`
	Count int64  `db:"usage_count"`
}

func EnsureDataExplorerSchema(database db.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS data_explorer_summoner_jobs (
			puuid VARCHAR(255) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'pending',
			priority INT NOT NULL DEFAULT 0,
			depth INT NOT NULL DEFAULT 0,
			attempts INT NOT NULL DEFAULT 0,
			next_attempt_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			lease_until DATETIME(6) NULL,
			discovered_from_match_id VARCHAR(255) NULL,
			last_error TEXT NULL,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (puuid),
			KEY data_explorer_summoner_jobs_claim_index (status, next_attempt_at, priority, created_at),
			KEY data_explorer_summoner_jobs_lease_index (status, lease_until)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS data_explorer_match_jobs (
			match_id VARCHAR(255) NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'pending',
			priority INT NOT NULL DEFAULT 0,
			depth INT NOT NULL DEFAULT 0,
			attempts INT NOT NULL DEFAULT 0,
			next_attempt_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			lease_until DATETIME(6) NULL,
			last_error TEXT NULL,
			rescan_requested TINYINT(1) NOT NULL DEFAULT 1,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (match_id),
			KEY data_explorer_match_jobs_claim_index (status, next_attempt_at, priority, created_at),
			KEY data_explorer_match_jobs_lease_index (status, lease_until)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS data_explorer_match_sources (
			match_id VARCHAR(255) NOT NULL,
			puuid VARCHAR(255) NOT NULL,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			PRIMARY KEY (match_id, puuid),
			KEY data_explorer_match_sources_puuid_index (puuid)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS data_explorer_state (
			state_key VARCHAR(64) NOT NULL,
			cursor_match_id VARCHAR(255) NOT NULL DEFAULT '',
			cursor_participant_id INT NOT NULL DEFAULT 0,
			completed TINYINT(1) NOT NULL DEFAULT 0,
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (state_key)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS data_explorer_daily_usage (
			usage_date DATE NOT NULL,
			usage_kind VARCHAR(16) NOT NULL,
			usage_count INT NOT NULL DEFAULT 0,
			PRIMARY KEY (usage_date, usage_kind)
		) ENGINE=InnoDB`,
		`INSERT IGNORE INTO data_explorer_state
			(state_key, cursor_match_id, cursor_participant_id, completed)
			VALUES ('match_participant_bootstrap', '', 0, 0)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			return err
		}
	}
	var rescanColumnCount int
	if err := database.Get(&rescanColumnCount, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = 'data_explorer_match_jobs'
		  AND column_name = 'rescan_requested'
	`); err != nil {
		return err
	}
	if rescanColumnCount == 0 {
		if _, err := database.Exec(`
			ALTER TABLE data_explorer_match_jobs
			ADD COLUMN rescan_requested TINYINT(1) NOT NULL DEFAULT 1 AFTER last_error
		`); err != nil {
			return err
		}
	}
	return nil
}

func EnqueueDataExplorerSummonerJob(database db.Context, puuid string, priority, depth int, fromMatchId *string) error {
	if puuid == "" {
		return nil
	}
	_, err := database.Exec(`
		INSERT INTO data_explorer_summoner_jobs
			(puuid, status, priority, depth, attempts, next_attempt_at, lease_until, discovered_from_match_id, last_error)
		VALUES (?, 'pending', ?, ?, 0, NOW(6), NULL, ?, NULL)
		ON DUPLICATE KEY UPDATE
			priority = GREATEST(priority, VALUES(priority)),
			depth = LEAST(depth, VALUES(depth))
	`, puuid, priority, depth, fromMatchId)
	return err
}

func EnqueueDataExplorerMatchJob(database db.Context, matchId, puuid string, priority, depth int) error {
	if matchId == "" || puuid == "" {
		return nil
	}
	sourceResult, err := database.Exec(`
		INSERT IGNORE INTO data_explorer_match_sources (match_id, puuid) VALUES (?, ?)
	`, matchId, puuid)
	if err != nil {
		return err
	}
	newSourceCount, err := sourceResult.RowsAffected()
	if err != nil {
		return err
	}
	_, err = database.Exec(`
		INSERT INTO data_explorer_match_jobs
			(match_id, status, priority, depth, attempts, next_attempt_at, lease_until, last_error, rescan_requested)
		VALUES (?, 'pending', ?, ?, 0, NOW(6), NULL, NULL, 1)
		ON DUPLICATE KEY UPDATE
			priority = GREATEST(priority, VALUES(priority)),
			depth = LEAST(depth, VALUES(depth)),
			next_attempt_at = IF(? > 0 AND status = 'done', NOW(6), next_attempt_at),
			status = IF(? > 0 AND status = 'done', 'pending', status),
			rescan_requested = IF(? > 0, 1, rescan_requested)
	`, matchId, priority, depth, newSourceCount, newSourceCount, newSourceCount)
	return err
}

func claimDataExplorerJob(query string, destination interface{}, lease time.Duration) (bool, error) {
	tx, err := db.Root.BeginTxx(context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := tx.Get(destination, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	leaseMicroseconds := lease.Microseconds()
	switch job := destination.(type) {
	case *DataExplorerSummonerJobDAO:
		if _, err := tx.Exec(`
			UPDATE data_explorer_summoner_jobs
			SET status = 'processing', attempts = attempts + 1,
			    lease_until = TIMESTAMPADD(MICROSECOND, ?, NOW(6)), updated_at = NOW(6)
			WHERE puuid = ?
		`, leaseMicroseconds, job.Puuid); err != nil {
			return false, err
		}
		job.Attempts++
	case *DataExplorerMatchJobDAO:
		if _, err := tx.Exec(`
			UPDATE data_explorer_match_jobs
			SET status = 'processing', attempts = attempts + 1,
			    lease_until = TIMESTAMPADD(MICROSECOND, ?, NOW(6)),
			    rescan_requested = 0, updated_at = NOW(6)
			WHERE match_id = ?
		`, leaseMicroseconds, job.MatchId); err != nil {
			return false, err
		}
		job.Attempts++
	default:
		return false, fmt.Errorf("unsupported data explorer job type %T", destination)
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func ClaimDataExplorerSummonerJob(lease time.Duration) (*DataExplorerSummonerJobDAO, bool, error) {
	job := &DataExplorerSummonerJobDAO{}
	found, err := claimDataExplorerJob(`
		SELECT puuid, status, priority, depth, attempts, next_attempt_at, lease_until,
		       discovered_from_match_id, last_error
		FROM data_explorer_summoner_jobs
		WHERE status = 'pending' AND next_attempt_at <= NOW(6)
		ORDER BY next_attempt_at ASC, priority DESC, created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`, job, lease)
	return job, found, err
}

func ClaimDataExplorerMatchJob(lease time.Duration) (*DataExplorerMatchJobDAO, bool, error) {
	job := &DataExplorerMatchJobDAO{}
	found, err := claimDataExplorerJob(`
		SELECT match_id, status, priority, depth, attempts, next_attempt_at, lease_until, last_error
		FROM data_explorer_match_jobs
		WHERE status = 'pending' AND next_attempt_at <= NOW(6)
		ORDER BY next_attempt_at ASC, priority DESC, created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`, job, lease)
	return job, found, err
}

func RecoverExpiredDataExplorerJobs(database db.Context) error {
	statements := []string{
		`UPDATE data_explorer_summoner_jobs
		 SET status = 'pending', lease_until = NULL, next_attempt_at = NOW(6)
		 WHERE status = 'processing' AND lease_until < NOW(6)`,
		`UPDATE data_explorer_match_jobs
		 SET status = 'pending', lease_until = NULL, next_attempt_at = NOW(6), rescan_requested = 1
		 WHERE status = 'processing' AND lease_until < NOW(6)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func GetDataExplorerDiagnostics(database db.Context) (*DataExplorerDiagnostics, error) {
	diagnostics := &DataExplorerDiagnostics{
		SummonerJobs: make(map[string]int64),
		MatchJobs:    make(map[string]int64),
		DailyUsage:   make(map[string]int64),
	}

	var summonerCounts []dataExplorerStatusCount
	if err := database.Select(&summonerCounts, `
		SELECT status, COUNT(*) AS count
		FROM data_explorer_summoner_jobs
		GROUP BY status
	`); err != nil {
		return nil, err
	}
	for _, count := range summonerCounts {
		diagnostics.SummonerJobs[count.Status] = count.Count
	}

	var matchCounts []dataExplorerStatusCount
	if err := database.Select(&matchCounts, `
		SELECT status, COUNT(*) AS count
		FROM data_explorer_match_jobs
		GROUP BY status
	`); err != nil {
		return nil, err
	}
	for _, count := range matchCounts {
		diagnostics.MatchJobs[count.Status] = count.Count
	}

	var usages []dataExplorerUsageCount
	if err := database.Select(&usages, `
		SELECT usage_kind, usage_count
		FROM data_explorer_daily_usage
		WHERE usage_date = CURRENT_DATE()
	`); err != nil {
		return nil, err
	}
	for _, usage := range usages {
		diagnostics.DailyUsage[usage.Kind] = usage.Count
	}

	var bootstrap struct {
		MatchId       string `db:"cursor_match_id"`
		ParticipantId int    `db:"cursor_participant_id"`
		Completed     bool   `db:"completed"`
	}
	if err := database.Get(&bootstrap, `
		SELECT cursor_match_id, cursor_participant_id, completed
		FROM data_explorer_state
		WHERE state_key = 'match_participant_bootstrap'
	`); err != nil {
		return nil, err
	}
	diagnostics.BootstrapMatchId = bootstrap.MatchId
	diagnostics.BootstrapParticipant = bootstrap.ParticipantId
	diagnostics.BootstrapCompleted = bootstrap.Completed
	return diagnostics, nil
}

func CompleteDataExplorerSummonerJob(database db.Context, puuid string) error {
	_, err := database.Exec(`
		UPDATE data_explorer_summoner_jobs
		SET status = 'done', lease_until = NULL, last_error = NULL, updated_at = NOW(6)
		WHERE puuid = ?
	`, puuid)
	return err
}

func CompleteDataExplorerMatchJob(database db.Context, matchId string) error {
	_, err := database.Exec(`
		UPDATE data_explorer_match_jobs
		SET status = IF(rescan_requested = 1, 'pending', 'done'),
		    next_attempt_at = IF(rescan_requested = 1, NOW(6), next_attempt_at),
		    lease_until = NULL, last_error = NULL, updated_at = NOW(6)
		WHERE match_id = ?
	`, matchId)
	return err
}

func RetryDataExplorerSummonerJob(database db.Context, puuid string, attempts, maxAttempts int, retryDelay time.Duration, failure error) error {
	status := DataExplorerJobPending
	if attempts >= maxAttempts {
		status = DataExplorerJobFailed
	}
	_, err := database.Exec(`
		UPDATE data_explorer_summoner_jobs
		SET status = ?, next_attempt_at = TIMESTAMPADD(MICROSECOND, ?, NOW(6)),
		    lease_until = NULL, last_error = ?, updated_at = NOW(6)
		WHERE puuid = ?
	`, status, retryDelay.Microseconds(), truncateExplorerError(failure), puuid)
	return err
}

func RetryDataExplorerMatchJob(database db.Context, matchId string, attempts, maxAttempts int, retryDelay time.Duration, failure error) error {
	status := DataExplorerJobPending
	if attempts >= maxAttempts {
		status = DataExplorerJobFailed
	}
	_, err := database.Exec(`
		UPDATE data_explorer_match_jobs
		SET status = ?, next_attempt_at = TIMESTAMPADD(MICROSECOND, ?, NOW(6)),
		    lease_until = NULL, last_error = ?, updated_at = NOW(6)
		WHERE match_id = ?
	`, status, retryDelay.Microseconds(), truncateExplorerError(failure), matchId)
	return err
}

func FailDataExplorerSummonerJob(database db.Context, puuid string, failure error) error {
	_, err := database.Exec(`
		UPDATE data_explorer_summoner_jobs
		SET status = 'failed', lease_until = NULL, last_error = ?, updated_at = NOW(6)
		WHERE puuid = ?
	`, truncateExplorerError(failure), puuid)
	return err
}

func DeferDataExplorerSummonerJob(database db.Context, puuid string) error {
	_, err := database.Exec(`
		UPDATE data_explorer_summoner_jobs
		SET status = 'pending', attempts = GREATEST(attempts - 1, 0),
		    next_attempt_at = TIMESTAMPADD(MINUTE, 5, TIMESTAMPADD(DAY, 1, CURRENT_DATE())),
		    lease_until = NULL, updated_at = NOW(6)
		WHERE puuid = ?
	`, puuid)
	return err
}

func DeferDataExplorerMatchJob(database db.Context, matchId string) error {
	_, err := database.Exec(`
		UPDATE data_explorer_match_jobs
		SET status = 'pending', attempts = GREATEST(attempts - 1, 0),
		    next_attempt_at = TIMESTAMPADD(MINUTE, 5, TIMESTAMPADD(DAY, 1, CURRENT_DATE())),
		    lease_until = NULL, updated_at = NOW(6)
		WHERE match_id = ?
	`, matchId)
	return err
}

func truncateExplorerError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 2000 {
		return message[:2000]
	}
	return message
}

func GetDataExplorerMatchSources(database db.Context, matchId string) ([]string, error) {
	sources := make([]string, 0)
	err := database.Select(&sources, `
		SELECT puuid FROM data_explorer_match_sources WHERE match_id = ? ORDER BY puuid
	`, matchId)
	return sources, err
}

func ConsumeDataExplorerDailyBudget(database db.Context, kind string, limit int) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	if _, err := database.Exec(`
		INSERT IGNORE INTO data_explorer_daily_usage (usage_date, usage_kind, usage_count)
		VALUES (CURRENT_DATE(), ?, 0)
	`, kind); err != nil {
		return false, err
	}
	result, err := database.Exec(`
		UPDATE data_explorer_daily_usage
		SET usage_count = usage_count + 1
		WHERE usage_date = CURRENT_DATE() AND usage_kind = ? AND usage_count < ?
	`, kind, limit)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func LoadDataExplorerBootstrapBatch(database db.Context, limit int) ([]DataExplorerParticipantCursor, bool, error) {
	var state struct {
		MatchId       string `db:"cursor_match_id"`
		ParticipantId int    `db:"cursor_participant_id"`
		Completed     bool   `db:"completed"`
	}
	if err := database.Get(&state, `
		SELECT cursor_match_id, cursor_participant_id, completed
		FROM data_explorer_state WHERE state_key = 'match_participant_bootstrap'
	`); err != nil {
		return nil, false, err
	}
	if state.Completed {
		return nil, true, nil
	}

	participants := make([]DataExplorerParticipantCursor, 0, limit)
	if err := database.Select(&participants, `
		SELECT match_id, participant_id, puuid
		FROM match_participants
		WHERE match_id > ? OR (match_id = ? AND participant_id > ?)
		ORDER BY match_id, participant_id
		LIMIT ?
	`, state.MatchId, state.MatchId, state.ParticipantId, limit); err != nil {
		return nil, false, err
	}

	if len(participants) == 0 {
		return participants, true, nil
	}
	return participants, false, nil
}

func AdvanceDataExplorerBootstrapCursor(database db.Context, last *DataExplorerParticipantCursor, completed bool) error {
	if completed {
		_, err := database.Exec(`
			UPDATE data_explorer_state SET completed = 1, updated_at = NOW(6)
			WHERE state_key = 'match_participant_bootstrap'
		`)
		return err
	}
	if last == nil {
		return nil
	}
	_, err := database.Exec(`
		UPDATE data_explorer_state
		SET cursor_match_id = ?, cursor_participant_id = ?, updated_at = NOW(6)
		WHERE state_key = 'match_participant_bootstrap'
	`, last.MatchId, last.ParticipantId)
	return err
}
