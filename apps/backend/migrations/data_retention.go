package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	version "github.com/hashicorp/go-version"
	"github.com/jmoiron/sqlx"
)

const (
	defaultRetentionMatchPatches = 8
	defaultRetentionBatchSize    = 100
	defaultRetentionBatchTimeout = 2 * time.Minute
	defaultRetentionWorkLimit    = 10 * time.Minute
)

type DataRetentionOptions struct {
	DryRun              bool
	DeleteAcknowledged  bool
	OfflineAcknowledged bool
	RetainedPatches     int
	BatchSize           int
	BatchTimeout        time.Duration
	WorkLimit           time.Duration
	Progress            func(DataRetentionResult)
}

type DataRetentionResult struct {
	DryRun           bool             `json:"dryRun"`
	RetainedPatches  []string         `json:"retainedPatches"`
	ExpiredVersions  []string         `json:"expiredVersions"`
	EligibleMatches  int64            `json:"eligibleMatches"`
	DeletedMatches   int64            `json:"deletedMatches"`
	DeletedRows      map[string]int64 `json:"deletedRows,omitempty"`
	DeleteDurationMs map[string]int64 `json:"deleteDurationMs,omitempty"`
	Completed        bool             `json:"completed"`
	Duration         time.Duration    `json:"-"`
	DurationMillis   int64            `json:"durationMs"`
}

func (result DataRetentionResult) String() string {
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("marshal retention result: %v", err)
	}
	return string(encoded)
}

type retentionVersionRow struct {
	GameVersion string `db:"game_version"`
	Count       int64  `db:"match_count"`
}

type retentionDeleteStatement struct {
	Name  string
	Query string
}

var retentionMatchDeleteStatements = []retentionDeleteStatement{
	{
		Name: "match_participant_perk_style_selections",
		Query: `DELETE selection_row
			FROM match_participant_perk_style_selections selection_row
			INNER JOIN match_participant_perk_styles style_row ON style_row.style_id = selection_row.style_id
			INNER JOIN match_participants participant ON participant.match_participant_id = style_row.match_participant_id
			WHERE participant.match_id IN (?)`,
	},
	{
		Name: "match_participant_perk_styles",
		Query: `DELETE style_row
			FROM match_participant_perk_styles style_row
			INNER JOIN match_participants participant ON participant.match_participant_id = style_row.match_participant_id
			WHERE participant.match_id IN (?)`,
	},
	{
		Name: "match_participant_perks",
		Query: `DELETE perk_row
			FROM match_participant_perks perk_row
			INNER JOIN match_participants participant ON participant.match_participant_id = perk_row.match_participant_id
			WHERE participant.match_id IN (?)`,
	},
	{
		Name: "match_participant_details",
		Query: `DELETE detail_row
			FROM match_participants participant
			STRAIGHT_JOIN match_participant_details detail_row
				ON detail_row.match_participant_id = participant.match_participant_id
			WHERE participant.match_id IN (?)`,
	},
	{
		Name: "summoner_matches",
		Query: `DELETE summoner_match
			FROM match_participants participant
			STRAIGHT_JOIN summoner_matches summoner_match
				ON summoner_match.puuid = participant.puuid
			   AND summoner_match.match_id = participant.match_id
			WHERE participant.match_id IN (?)`,
	},
	{
		Name: "match_participant_numeric_keys",
		Query: `DELETE numeric_participant
			FROM match_participant_numeric_keys numeric_participant
			INNER JOIN match_participants participant
				ON participant.match_participant_id = numeric_participant.legacy_match_participant_id
			WHERE participant.match_id IN (?)`,
	},
	{
		Name:  "match_participants",
		Query: `DELETE FROM match_participants WHERE match_id IN (?)`,
	},
	{
		Name:  "match_team_bans",
		Query: `DELETE FROM match_team_bans WHERE match_id IN (?)`,
	},
	{
		Name:  "match_teams",
		Query: `DELETE FROM match_teams WHERE match_id IN (?)`,
	},
	{
		Name:  "data_explorer_match_sources",
		Query: `DELETE FROM data_explorer_match_sources WHERE match_id IN (?)`,
	},
	{
		Name:  "data_explorer_match_jobs",
		Query: `DELETE FROM data_explorer_match_jobs WHERE match_id IN (?)`,
	},
	{
		Name:  "data_explorer_match_processing_state",
		Query: `DELETE FROM data_explorer_match_processing_state WHERE match_id IN (?)`,
	},
	{
		Name: "champion_detail_statistics_processed_matches",
		Query: `DELETE processed_match
			FROM matches match_row
			STRAIGHT_JOIN champion_detail_statistics_processed_matches processed_match
				ON processed_match.game_version = match_row.game_version
			   AND processed_match.match_id = match_row.match_id
			WHERE match_row.match_id IN (?)`,
	},
	{
		Name:  "match_numeric_keys",
		Query: `DELETE FROM match_numeric_keys WHERE riot_match_id IN (?)`,
	},
	{
		Name:  "matches",
		Query: `DELETE FROM matches WHERE match_id IN (?)`,
	},
}

func DataRetentionOptionsFromEnvironment() DataRetentionOptions {
	return DataRetentionOptions{
		DryRun:              retentionEnvBool("DATA_RETENTION_DRY_RUN", true),
		DeleteAcknowledged:  retentionEnvBool("DATA_RETENTION_DELETE_ACK", false),
		OfflineAcknowledged: retentionEnvBool("DATA_RETENTION_OFFLINE_ACK", false),
		RetainedPatches:     retentionEnvInt("DATA_RETENTION_MATCH_PATCHES", defaultRetentionMatchPatches, 3, 30),
		BatchSize:           retentionEnvInt("DATA_RETENTION_BATCH_SIZE", defaultRetentionBatchSize, 10, 1000),
		BatchTimeout:        retentionEnvDuration("DATA_RETENTION_BATCH_TIMEOUT", defaultRetentionBatchTimeout, 10*time.Second, 15*time.Minute),
		WorkLimit:           retentionEnvDuration("DATA_RETENTION_WORK_LIMIT", defaultRetentionWorkLimit, time.Second, time.Hour),
	}
}

func retentionEnvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func retentionEnvInt(key string, fallback, minimum, maximum int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}

func retentionEnvDuration(key string, fallback, minimum, maximum time.Duration) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key)))
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}

func CleanupRetainedData(ctx context.Context, database *sqlx.DB, options DataRetentionOptions) (result DataRetentionResult, returnedErr error) {
	result = DataRetentionResult{
		DryRun: options.DryRun, DeletedRows: make(map[string]int64), DeleteDurationMs: make(map[string]int64),
	}
	started := time.Now()
	defer func() {
		result.Duration = time.Since(started)
		result.DurationMillis = result.Duration.Milliseconds()
	}()

	if !options.DryRun && (!options.DeleteAcknowledged || !options.OfflineAcknowledged) {
		return result, errors.New("retention deletion requires DATA_RETENTION_DELETE_ACK=true and DATA_RETENTION_OFFLINE_ACK=true after backend writes are stopped")
	}
	if database == nil {
		return result, errors.New("retention cleanup database is required")
	}
	if options.RetainedPatches < 3 || options.RetainedPatches > 30 {
		options.RetainedPatches = defaultRetentionMatchPatches
	}
	if options.BatchSize < 10 || options.BatchSize > 1000 {
		options.BatchSize = defaultRetentionBatchSize
	}
	if options.BatchTimeout < 10*time.Second || options.BatchTimeout > 15*time.Minute {
		options.BatchTimeout = defaultRetentionBatchTimeout
	}
	if options.WorkLimit < time.Second || options.WorkLimit > time.Hour {
		options.WorkLimit = defaultRetentionWorkLimit
	}

	versions, err := loadRetentionMatchVersions(ctx, database)
	if err != nil {
		return result, err
	}
	result.RetainedPatches, result.ExpiredVersions, result.EligibleMatches = classifyRetentionMatchVersions(versions, options.RetainedPatches)
	if len(result.ExpiredVersions) == 0 {
		result.Completed = true
		return result, nil
	}
	if options.DryRun {
		return result, nil
	}

	connection, err := database.Connx(ctx)
	if err != nil {
		return result, err
	}
	defer connection.Close()
	deadline := time.Now().Add(options.WorkLimit)
	for time.Now().Before(deadline) {
		batchContext, cancel := context.WithTimeout(ctx, options.BatchTimeout)
		matchIDs, selectErr := selectRetentionMatchBatch(batchContext, connection, result.ExpiredVersions, options.BatchSize)
		if selectErr != nil {
			cancel()
			return result, selectErr
		}
		if len(matchIDs) == 0 {
			cancel()
			result.Completed = true
			break
		}

		tx, txErr := beginNumericKeyBackfillTransactionOnConnection(batchContext, connection)
		if txErr != nil {
			cancel()
			return result, txErr
		}
		batchRows, batchDurations, deleteErr := deleteRetentionMatchBatch(batchContext, tx, matchIDs)
		if deleteErr != nil {
			_ = tx.Rollback()
			cancel()
			return result, deleteErr
		}
		if commitErr := tx.Commit(); commitErr != nil {
			cancel()
			return result, commitErr
		}
		cancel()
		for table, rows := range batchRows {
			result.DeletedRows[table] += rows
		}
		for table, duration := range batchDurations {
			result.DeleteDurationMs[table] += duration.Milliseconds()
		}
		result.DeletedMatches += int64(len(matchIDs))
		if options.Progress != nil {
			options.Progress(result)
		}
	}
	return result, nil
}

func loadRetentionMatchVersions(ctx context.Context, database *sqlx.DB) ([]retentionVersionRow, error) {
	rows := make([]retentionVersionRow, 0)
	if err := database.SelectContext(ctx, &rows, `
		SELECT game_version, COUNT(*) AS match_count
		FROM matches FORCE INDEX (matches_game_version_index)
		WHERE game_version != ''
		GROUP BY game_version
	`); err != nil {
		return nil, fmt.Errorf("load match versions for retention: %w", err)
	}
	return rows, nil
}

func classifyRetentionMatchVersions(rows []retentionVersionRow, retainedPatchCount int) ([]string, []string, int64) {
	type parsedVersionRow struct {
		row     retentionVersionRow
		version *version.Version
		short   string
	}
	parsed := make([]parsedVersionRow, 0, len(rows))
	for _, row := range rows {
		current, err := version.NewVersion(row.GameVersion)
		if err != nil || len(current.Segments()) < 2 {
			continue
		}
		segments := current.Segments()
		parsed = append(parsed, parsedVersionRow{
			row: row, version: current, short: fmt.Sprintf("%d.%d", segments[0], segments[1]),
		})
	}
	sort.SliceStable(parsed, func(i, j int) bool { return parsed[i].version.GreaterThan(parsed[j].version) })

	retainedShort := make([]string, 0, retainedPatchCount)
	retainedSet := make(map[string]struct{}, retainedPatchCount)
	for _, current := range parsed {
		if _, exists := retainedSet[current.short]; exists {
			continue
		}
		if len(retainedShort) >= retainedPatchCount {
			break
		}
		retainedSet[current.short] = struct{}{}
		retainedShort = append(retainedShort, current.short)
	}

	expiredVersions := make([]string, 0)
	var eligible int64
	for _, current := range parsed {
		if _, retained := retainedSet[current.short]; retained {
			continue
		}
		expiredVersions = append(expiredVersions, current.row.GameVersion)
		eligible += current.row.Count
	}
	sort.Strings(expiredVersions)
	return retainedShort, expiredVersions, eligible
}

func selectRetentionMatchBatch(ctx context.Context, connection *sqlx.Conn, versions []string, batchSize int) ([]string, error) {
	query, args, err := sqlx.In(`
		SELECT match_id
		FROM matches FORCE INDEX (matches_game_version_index)
		WHERE game_version IN (?)
		ORDER BY game_version, match_id
		LIMIT ?
	`, versions, batchSize)
	if err != nil {
		return nil, fmt.Errorf("bind retention match batch: %w", err)
	}
	matchIDs := make([]string, 0, batchSize)
	if err := connection.SelectContext(ctx, &matchIDs, connection.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("select retention match batch: %w", err)
	}
	return matchIDs, nil
}

func deleteRetentionMatchBatch(ctx context.Context, tx *sqlx.Tx, matchIDs []string) (map[string]int64, map[string]time.Duration, error) {
	deleted := make(map[string]int64, len(retentionMatchDeleteStatements))
	durations := make(map[string]time.Duration, len(retentionMatchDeleteStatements))
	for _, statement := range retentionMatchDeleteStatements {
		query, args, err := sqlx.In(statement.Query, matchIDs)
		if err != nil {
			return nil, nil, fmt.Errorf("bind retention delete for %s: %w", statement.Name, err)
		}
		started := time.Now()
		result, err := tx.ExecContext(ctx, tx.Rebind(query), args...)
		durations[statement.Name] = time.Since(started)
		if err != nil {
			return nil, nil, fmt.Errorf("delete retained rows from %s: %w", statement.Name, err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return nil, nil, fmt.Errorf("read retention delete result for %s: %w", statement.Name, err)
		}
		deleted[statement.Name] = rows
	}
	return deleted, durations, nil
}
