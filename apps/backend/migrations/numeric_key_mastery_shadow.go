package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	masteryNumericShadowProgressTable = "numeric_key_mastery_shadow_progress"
	masteryNumericShadowLock          = "teamgg_masteries_numeric_shadow"
	masteryNumericShadowInsertTrigger = "teamgg_masteries_numeric_shadow_insert"
	masteryNumericShadowUpdateTrigger = "teamgg_masteries_numeric_shadow_update"
	masteryNumericShadowDeleteTrigger = "teamgg_masteries_numeric_shadow_delete"
	masteryNumericShadowBatchQuery    = `
		SELECT puuid, champion_id, champion_points_until_next_level,
			chest_granted, last_play_time, champion_level, champion_points,
			champion_points_since_last_level, tokens_earned
		FROM masteries FORCE INDEX (PRIMARY)
		WHERE puuid > ? OR (puuid = ? AND champion_id > ?)
		ORDER BY puuid, champion_id
		LIMIT ?
	`
)

type MasteryNumericShadowOptions struct {
	BatchSize           int
	BatchTimeout        time.Duration
	WorkLimit           time.Duration
	MaxBatches          int
	OfflineAcknowledged bool
	DisableBinlog       bool
	Progress            func(MasteryNumericShadowResult)
}

type MasteryNumericShadowResult struct {
	ProcessedThisRun    int64
	ProcessedTotal      int64
	CopyCompleted       bool
	Validated           bool
	LastBatchDuration   time.Duration
	LastSelectDuration  time.Duration
	LastMappingDuration time.Duration
	LastInsertDuration  time.Duration
}

type masteryNumericShadowProgress struct {
	CursorPuuid      string `db:"cursor_puuid"`
	CursorChampionId int64  `db:"cursor_champion_id"`
	ProcessedRows    int64  `db:"processed_rows"`
	CopyCompleted    bool   `db:"copy_completed"`
	Validated        bool   `db:"validated"`
}

type masteryNumericShadowRow struct {
	Puuid                        string    `db:"puuid"`
	ChampionId                   int64     `db:"champion_id"`
	ChampionPointsUntilNextLevel int64     `db:"champion_points_until_next_level"`
	ChestGranted                 bool      `db:"chest_granted"`
	LastPlayTime                 time.Time `db:"last_play_time"`
	ChampionLevel                int       `db:"champion_level"`
	ChampionPoints               int       `db:"champion_points"`
	ChampionPointsSinceLastLevel int64     `db:"champion_points_since_last_level"`
	TokensEarned                 int       `db:"tokens_earned"`
}

type masteryNumericMapping struct {
	Puuid      string `db:"puuid"`
	SummonerId int64  `db:"summoner_id"`
}

type masteryNumericShadowDigest struct {
	Rows     int64         `db:"rows_count"`
	Checksum sql.NullInt64 `db:"rows_checksum"`
}

func MasteryNumericShadowOptionsFromEnvironment() MasteryNumericShadowOptions {
	offlineAcknowledged, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("MASTERY_NUMERIC_SHADOW_OFFLINE_ACK")))
	disableBinlog, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("MASTERY_NUMERIC_SHADOW_DISABLE_BINLOG")))
	return MasteryNumericShadowOptions{
		BatchSize:           boundedMasteryNumericShadowBatchSize(os.Getenv("MASTERY_NUMERIC_SHADOW_BATCH_SIZE")),
		BatchTimeout:        boundedMasteryNumericShadowBatchTimeout(os.Getenv("MASTERY_NUMERIC_SHADOW_BATCH_TIMEOUT")),
		WorkLimit:           boundedNumericKeyWorkLimit(os.Getenv("MASTERY_NUMERIC_SHADOW_WORK_LIMIT")),
		MaxBatches:          boundedMasteryNumericShadowMaxBatches(os.Getenv("MASTERY_NUMERIC_SHADOW_MAX_BATCHES")),
		OfflineAcknowledged: offlineAcknowledged,
		DisableBinlog:       disableBinlog,
	}
}

func boundedMasteryNumericShadowBatchTimeout(value string) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || parsed < 10*time.Second || parsed > 15*time.Minute {
		return 2 * time.Minute
	}
	return parsed
}

func boundedMasteryNumericShadowBatchSize(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 100 || parsed > 100000 {
		return 10000
	}
	return parsed
}

func boundedMasteryNumericShadowMaxBatches(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 || parsed > 100000 {
		return 0
	}
	return parsed
}

func PrepareMasteryNumericShadow(
	ctx context.Context,
	database *sqlx.DB,
	options MasteryNumericShadowOptions,
) (MasteryNumericShadowResult, error) {
	result := MasteryNumericShadowResult{}
	if !options.OfflineAcknowledged {
		return result, errors.New("mastery numeric shadow requires MASTERY_NUMERIC_SHADOW_OFFLINE_ACK=true after backend writes are stopped")
	}
	if options.BatchSize < 100 || options.BatchSize > 100000 {
		options.BatchSize = 10000
	}
	if options.BatchTimeout < 10*time.Second || options.BatchTimeout > 15*time.Minute {
		options.BatchTimeout = 2 * time.Minute
	}
	if options.WorkLimit < time.Second || options.WorkLimit > time.Hour {
		options.WorkLimit = 10 * time.Minute
	}

	connection, err := database.Connx(ctx)
	if err != nil {
		return result, err
	}
	defer connection.Close()

	var lockAcquired int
	if err := connection.GetContext(ctx, &lockAcquired, `SELECT GET_LOCK(?, 0)`, masteryNumericShadowLock); err != nil {
		return result, fmt.Errorf("acquire mastery numeric shadow lock: %w", err)
	}
	if lockAcquired != 1 {
		return result, errors.New("another mastery numeric shadow command is already running")
	}
	defer func() {
		_, _ = connection.ExecContext(context.Background(), `SELECT RELEASE_LOCK(?)`, masteryNumericShadowLock)
	}()
	if options.DisableBinlog {
		if _, err := connection.ExecContext(ctx, `SET SESSION sql_log_bin = 0`); err != nil {
			return result, fmt.Errorf("disable mastery shadow session binlog: %w", err)
		}
	}

	if err := ensureMasteryNumericShadowSchema(ctx, connection); err != nil {
		return result, err
	}
	if err := ensureMasteryNumericMappings(ctx, connection); err != nil {
		return result, err
	}

	progress, err := loadMasteryNumericShadowProgress(ctx, connection)
	if err != nil {
		return result, err
	}
	result.ProcessedTotal = progress.ProcessedRows
	result.CopyCompleted = progress.CopyCompleted
	result.Validated = progress.Validated

	deadline := time.Now().Add(options.WorkLimit)
	batches := 0
	for !progress.CopyCompleted && time.Now().Before(deadline) && (options.MaxBatches == 0 || batches < options.MaxBatches) {
		batchStarted := time.Now()
		batchContext, cancelBatch := context.WithTimeout(ctx, options.BatchTimeout)
		selectStarted := time.Now()
		rows, err := selectMasteryNumericShadowBatch(
			batchContext, connection, progress.CursorPuuid, progress.CursorChampionId, options.BatchSize,
		)
		if err != nil {
			cancelBatch()
			return result, err
		}
		result.LastSelectDuration = time.Since(selectStarted)
		if len(rows) == 0 {
			cancelBatch()
			progress.CopyCompleted = true
			if err := saveMasteryNumericShadowProgress(ctx, connection, progress); err != nil {
				return result, err
			}
			break
		}

		mappingStarted := time.Now()
		mappings, err := loadMasteryNumericMappings(batchContext, connection, rows)
		if err != nil {
			cancelBatch()
			return result, err
		}
		result.LastMappingDuration = time.Since(mappingStarted)
		last := rows[len(rows)-1]
		tx, err := beginNumericKeyBackfillTransactionOnConnection(batchContext, connection)
		if err != nil {
			cancelBatch()
			return result, err
		}
		insertStarted := time.Now()
		if err := copyMasteryNumericShadowBatch(batchContext, tx, rows, mappings); err != nil {
			_ = tx.Rollback()
			cancelBatch()
			return result, err
		}

		progress.CursorPuuid = last.Puuid
		progress.CursorChampionId = last.ChampionId
		progress.ProcessedRows += int64(len(rows))
		progress.Validated = false
		if err := saveMasteryNumericShadowProgress(batchContext, tx, progress); err != nil {
			_ = tx.Rollback()
			cancelBatch()
			return result, err
		}
		if err := tx.Commit(); err != nil {
			cancelBatch()
			return result, err
		}
		result.LastInsertDuration = time.Since(insertStarted)
		cancelBatch()
		result.ProcessedThisRun += int64(len(rows))
		result.ProcessedTotal = progress.ProcessedRows
		result.LastBatchDuration = time.Since(batchStarted)
		batches++
		if options.Progress != nil {
			options.Progress(result)
		}
	}

	result.CopyCompleted = progress.CopyCompleted
	if progress.CopyCompleted {
		validated, err := validateMasteryNumericShadow(ctx, connection)
		if err != nil {
			return result, err
		}
		progress.Validated = validated
		if err := saveMasteryNumericShadowProgress(ctx, connection, progress); err != nil {
			return result, err
		}
		result.Validated = validated
		if !validated {
			return result, errors.New("mastery numeric shadow row count or checksum does not match the legacy table")
		}
		if err := ensureMasteryNumericShadowSyncTriggers(ctx, connection); err != nil {
			return result, err
		}
	}
	return result, nil
}

func ResetMasteryNumericShadow(ctx context.Context, database *sqlx.DB, offlineAcknowledged, resetAcknowledged bool) error {
	if !offlineAcknowledged || !resetAcknowledged {
		return errors.New("reset mastery numeric shadow requires MASTERY_NUMERIC_SHADOW_OFFLINE_ACK=true and MASTERY_NUMERIC_SHADOW_RESET_ACK=true")
	}
	connection, err := database.Connx(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()

	var lockAcquired int
	if err := connection.GetContext(ctx, &lockAcquired, `SELECT GET_LOCK(?, 0)`, masteryNumericShadowLock); err != nil {
		return fmt.Errorf("acquire mastery numeric shadow reset lock: %w", err)
	}
	if lockAcquired != 1 {
		return errors.New("another mastery numeric shadow command is already running")
	}
	defer func() {
		_, _ = connection.ExecContext(context.Background(), `SELECT RELEASE_LOCK(?)`, masteryNumericShadowLock)
	}()

	for _, trigger := range masteryNumericShadowSyncTriggers() {
		if _, err := connection.ExecContext(ctx, "DROP TRIGGER IF EXISTS `"+trigger.name+"`"); err != nil {
			return fmt.Errorf("drop mastery numeric shadow trigger %s during reset: %w", trigger.name, err)
		}
	}
	if _, err := connection.ExecContext(ctx, `DROP TABLE IF EXISTS masteries_numeric_v2`); err != nil {
		return fmt.Errorf("drop partial mastery numeric shadow: %w", err)
	}
	if exists, err := tableExists(ctx, database, masteryNumericShadowProgressTable); err != nil {
		return err
	} else if exists {
		if _, err := connection.ExecContext(ctx, `DELETE FROM numeric_key_mastery_shadow_progress WHERE state_key = 'masteries'`); err != nil {
			return fmt.Errorf("reset mastery numeric shadow progress: %w", err)
		}
	}
	return nil
}

func ensureMasteryNumericShadowSchema(ctx context.Context, connection *sqlx.Conn) error {
	for _, statement := range masteryNumericShadowSchemaStatements() {
		if _, err := connection.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("prepare mastery numeric shadow schema: %w", err)
		}
	}
	return validateMasteryNumericShadowSchema(ctx, connection)
}

func validateMasteryNumericShadowSchema(ctx context.Context, connection *sqlx.Conn) error {
	var puuidColumns int
	if err := connection.GetContext(ctx, &puuidColumns, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = 'masteries_numeric_v2'
		  AND column_name = 'puuid'
	`); err != nil {
		return err
	}
	if puuidColumns != 0 {
		return errors.New("masteries_numeric_v2 must not contain PUUID")
	}

	indexes := []struct {
		name    string
		columns string
	}{
		{"PRIMARY", "summoner_fk,champion_id"},
		{"masteries_numeric_champion_points_level_covering_index", "champion_id,champion_points,champion_level"},
	}
	for _, expected := range indexes {
		var columns sql.NullString
		if err := connection.GetContext(ctx, &columns, `
			SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index)
			FROM information_schema.statistics
			WHERE table_schema = DATABASE() AND table_name = 'masteries_numeric_v2'
			  AND index_name = ?
		`, expected.name); err != nil {
			return err
		}
		if !columns.Valid || columns.String != expected.columns {
			return fmt.Errorf("masteries_numeric_v2 index %s is %q, want %q", expected.name, columns.String, expected.columns)
		}
	}
	return nil
}

func masteryNumericShadowSchemaStatements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS masteries_numeric_v2 (
			summoner_fk BIGINT UNSIGNED NOT NULL,
			champion_id BIGINT NOT NULL,
			champion_points_until_next_level BIGINT NOT NULL,
			chest_granted TINYINT(1) NOT NULL,
			last_play_time DATETIME NOT NULL,
			champion_level INT NOT NULL,
			champion_points INT NOT NULL,
			champion_points_since_last_level BIGINT NOT NULL,
			tokens_earned INT NOT NULL,
			PRIMARY KEY (summoner_fk, champion_id),
			KEY masteries_numeric_champion_points_level_covering_index
				(champion_id, champion_points DESC, champion_level)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS numeric_key_mastery_shadow_progress (
			state_key VARCHAR(32) NOT NULL,
			cursor_puuid VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
			cursor_champion_id BIGINT NOT NULL DEFAULT 0,
			processed_rows BIGINT UNSIGNED NOT NULL DEFAULT 0,
			copy_completed TINYINT(1) NOT NULL DEFAULT 0,
			validated TINYINT(1) NOT NULL DEFAULT 0,
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			validated_at DATETIME(6) NULL,
			PRIMARY KEY (state_key)
		) ENGINE=InnoDB`,
		`INSERT IGNORE INTO numeric_key_mastery_shadow_progress (state_key) VALUES ('masteries')`,
	}
}

func ensureMasteryNumericMappings(ctx context.Context, connection *sqlx.Conn) error {
	var missing bool
	if err := connection.GetContext(ctx, &missing, `
		SELECT EXISTS(
			SELECT 1
			FROM summoners source
			LEFT JOIN summoner_numeric_keys numeric_key ON numeric_key.puuid = source.puuid
			WHERE source.summoner_pk IS NULL
			   OR numeric_key.summoner_id IS NULL
			   OR source.summoner_pk <> numeric_key.summoner_id
			LIMIT 1
		)
	`); err != nil {
		return fmt.Errorf("validate summoner numeric mappings for mastery shadow: %w", err)
	}
	if missing {
		return errors.New("summoner numeric identities must be complete before mastery shadow copy")
	}
	return nil
}

func loadMasteryNumericShadowProgress(ctx context.Context, connection *sqlx.Conn) (masteryNumericShadowProgress, error) {
	progress := masteryNumericShadowProgress{}
	err := connection.GetContext(ctx, &progress, `
		SELECT cursor_puuid, cursor_champion_id, processed_rows, copy_completed, validated
		FROM numeric_key_mastery_shadow_progress
		WHERE state_key = 'masteries'
	`)
	return progress, err
}

func saveMasteryNumericShadowProgress(
	ctx context.Context,
	store numericKeyProgressStore,
	progress masteryNumericShadowProgress,
) error {
	_, err := store.ExecContext(ctx, `
		UPDATE numeric_key_mastery_shadow_progress
		SET cursor_puuid = ?, cursor_champion_id = ?, processed_rows = ?,
			copy_completed = ?, validated = ?,
			validated_at = CASE WHEN ? THEN CURRENT_TIMESTAMP(6) ELSE NULL END,
			updated_at = CURRENT_TIMESTAMP(6)
		WHERE state_key = 'masteries'
	`, progress.CursorPuuid, progress.CursorChampionId, progress.ProcessedRows,
		progress.CopyCompleted, progress.Validated, progress.Validated)
	return err
}

func selectMasteryNumericShadowBatch(
	ctx context.Context,
	connection *sqlx.Conn,
	cursorPuuid string,
	cursorChampionId int64,
	batchSize int,
) ([]masteryNumericShadowRow, error) {
	rows := make([]masteryNumericShadowRow, 0, batchSize)
	err := connection.SelectContext(
		ctx, &rows, masteryNumericShadowBatchQuery,
		cursorPuuid, cursorPuuid, cursorChampionId, batchSize,
	)
	if err != nil {
		return nil, fmt.Errorf("select mastery numeric shadow batch: %w", err)
	}
	return rows, nil
}

func loadMasteryNumericMappings(
	ctx context.Context,
	connection *sqlx.Conn,
	rows []masteryNumericShadowRow,
) (map[string]int64, error) {
	puuids := make([]string, 0)
	seen := make(map[string]struct{})
	for _, row := range rows {
		if _, exists := seen[row.Puuid]; exists {
			continue
		}
		seen[row.Puuid] = struct{}{}
		puuids = append(puuids, row.Puuid)
	}
	query, arguments, err := sqlx.In(`
		SELECT puuid, summoner_id
		FROM summoner_numeric_keys FORCE INDEX (summoner_numeric_keys_puuid_uindex)
		WHERE puuid IN (?)
	`, puuids)
	if err != nil {
		return nil, fmt.Errorf("build mastery numeric mapping query: %w", err)
	}
	var mappings []masteryNumericMapping
	if err := connection.SelectContext(ctx, &mappings, connection.Rebind(query), arguments...); err != nil {
		return nil, fmt.Errorf("load mastery numeric mappings: %w", err)
	}
	byPuuid := make(map[string]int64, len(mappings))
	for _, mapping := range mappings {
		byPuuid[mapping.Puuid] = mapping.SummonerId
	}
	if len(byPuuid) != len(puuids) {
		return nil, fmt.Errorf("mastery numeric mappings are incomplete: expected=%d actual=%d", len(puuids), len(byPuuid))
	}
	return byPuuid, nil
}

func beginNumericKeyBackfillTransactionOnConnection(ctx context.Context, connection *sqlx.Conn) (*sqlx.Tx, error) {
	tx, err := connection.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SET SESSION binlog_row_image = 'MINIMAL'`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("set mastery shadow binlog row image: %w", err)
	}
	return tx, nil
}

func copyMasteryNumericShadowBatch(
	ctx context.Context,
	tx *sqlx.Tx,
	rows []masteryNumericShadowRow,
	mappings map[string]int64,
) error {
	const chunkSize = 500
	for start := 0; start < len(rows); start += chunkSize {
		end := start + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		var query strings.Builder
		query.WriteString(`INSERT INTO masteries_numeric_v2 (
			summoner_fk, champion_id, champion_points_until_next_level,
			chest_granted, last_play_time, champion_level, champion_points,
			champion_points_since_last_level, tokens_earned
		) VALUES `)
		arguments := make([]interface{}, 0, (end-start)*9)
		for index, row := range rows[start:end] {
			summonerId, exists := mappings[row.Puuid]
			if !exists {
				return fmt.Errorf("mastery numeric mapping is missing for puuid %s", row.Puuid)
			}
			if index > 0 {
				query.WriteString(",")
			}
			query.WriteString("(?,?,?,?,?,?,?,?,?)")
			arguments = append(arguments,
				summonerId, row.ChampionId, row.ChampionPointsUntilNextLevel,
				row.ChestGranted, row.LastPlayTime, row.ChampionLevel,
				row.ChampionPoints, row.ChampionPointsSinceLastLevel, row.TokensEarned,
			)
		}
		query.WriteString(` ON DUPLICATE KEY UPDATE
			champion_points_until_next_level = VALUES(champion_points_until_next_level),
			chest_granted = VALUES(chest_granted),
			last_play_time = VALUES(last_play_time),
			champion_level = VALUES(champion_level),
			champion_points = VALUES(champion_points),
			champion_points_since_last_level = VALUES(champion_points_since_last_level),
			tokens_earned = VALUES(tokens_earned)`)
		if _, err := tx.ExecContext(ctx, query.String(), arguments...); err != nil {
			return fmt.Errorf("copy mastery numeric shadow batch rows %d-%d: %w", start, end, err)
		}
	}
	return nil
}

func validateMasteryNumericShadow(ctx context.Context, connection *sqlx.Conn) (bool, error) {
	tx, err := connection.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var legacyRows int64
	if err := tx.GetContext(ctx, &legacyRows, `SELECT COUNT(*) FROM masteries`); err != nil {
		return false, fmt.Errorf("count legacy masteries: %w", err)
	}

	var source, shadow masteryNumericShadowDigest
	if err := tx.GetContext(ctx, &source, `
		SELECT COUNT(*) AS rows_count,
			BIT_XOR(CRC32(CONCAT_WS('#',
				numeric_key.summoner_id, source.champion_id,
				source.champion_points_until_next_level, source.chest_granted,
				DATE_FORMAT(source.last_play_time, '%Y-%m-%d %H:%i:%s'),
				source.champion_level, source.champion_points,
				source.champion_points_since_last_level, source.tokens_earned
			))) AS rows_checksum
		FROM masteries source
		INNER JOIN summoner_numeric_keys numeric_key ON numeric_key.puuid = source.puuid
	`); err != nil {
		return false, fmt.Errorf("digest legacy masteries: %w", err)
	}
	if err := tx.GetContext(ctx, &shadow, `
		SELECT COUNT(*) AS rows_count,
			BIT_XOR(CRC32(CONCAT_WS('#',
				summoner_fk, champion_id, champion_points_until_next_level,
				chest_granted, DATE_FORMAT(last_play_time, '%Y-%m-%d %H:%i:%s'),
				champion_level, champion_points,
				champion_points_since_last_level, tokens_earned
			))) AS rows_checksum
		FROM masteries_numeric_v2
	`); err != nil {
		return false, fmt.Errorf("digest numeric mastery shadow: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return legacyRows == source.Rows &&
		source.Rows == shadow.Rows &&
		source.Checksum.Int64 == shadow.Checksum.Int64, nil
}

func ensureMasteryNumericShadowSyncTriggers(ctx context.Context, connection *sqlx.Conn) error {
	for _, trigger := range masteryNumericShadowSyncTriggers() {
		if _, err := connection.ExecContext(ctx, "DROP TRIGGER IF EXISTS `"+trigger.name+"`"); err != nil {
			return fmt.Errorf("drop mastery numeric shadow trigger %s: %w", trigger.name, err)
		}
		if _, err := connection.ExecContext(ctx, trigger.statement); err != nil {
			return fmt.Errorf("create mastery numeric shadow trigger %s: %w", trigger.name, err)
		}
	}
	return nil
}

func masteryNumericShadowSyncTriggers() []struct {
	name      string
	event     string
	statement string
} {
	values := `COALESCE(NEW.summoner_fk,
		(SELECT summoner_id FROM summoner_numeric_keys WHERE puuid = NEW.puuid)),
		NEW.champion_id,
		NEW.champion_points_until_next_level, NEW.chest_granted,
		NEW.last_play_time, NEW.champion_level, NEW.champion_points,
		NEW.champion_points_since_last_level, NEW.tokens_earned`
	upsert := `ON DUPLICATE KEY UPDATE
		champion_points_until_next_level = VALUES(champion_points_until_next_level),
		chest_granted = VALUES(chest_granted),
		last_play_time = VALUES(last_play_time),
		champion_level = VALUES(champion_level),
		champion_points = VALUES(champion_points),
		champion_points_since_last_level = VALUES(champion_points_since_last_level),
		tokens_earned = VALUES(tokens_earned)`
	columns := `summoner_fk, champion_id, champion_points_until_next_level,
		chest_granted, last_play_time, champion_level, champion_points,
		champion_points_since_last_level, tokens_earned`
	return []struct {
		name      string
		event     string
		statement string
	}{
		{
			name:  masteryNumericShadowInsertTrigger,
			event: "INSERT",
			statement: fmt.Sprintf(`CREATE TRIGGER %s AFTER INSERT ON masteries FOR EACH ROW
				INSERT INTO masteries_numeric_v2 (%s) VALUES (%s) %s`,
				masteryNumericShadowInsertTrigger, columns, values, upsert),
		},
		{
			name:  masteryNumericShadowUpdateTrigger,
			event: "UPDATE",
			statement: fmt.Sprintf(`CREATE TRIGGER %s AFTER UPDATE ON masteries FOR EACH ROW
				BEGIN
					DELETE FROM masteries_numeric_v2
					WHERE summoner_fk = COALESCE(OLD.summoner_fk,
						(SELECT summoner_id FROM summoner_numeric_keys WHERE puuid = OLD.puuid))
					  AND champion_id = OLD.champion_id;
					INSERT INTO masteries_numeric_v2 (%s) VALUES (%s) %s;
				END`, masteryNumericShadowUpdateTrigger, columns, values, upsert),
		},
		{
			name:  masteryNumericShadowDeleteTrigger,
			event: "DELETE",
			statement: fmt.Sprintf(`CREATE TRIGGER %s AFTER DELETE ON masteries FOR EACH ROW
				DELETE FROM masteries_numeric_v2
				WHERE summoner_fk = COALESCE(OLD.summoner_fk,
					(SELECT summoner_id FROM summoner_numeric_keys WHERE puuid = OLD.puuid))
				  AND champion_id = OLD.champion_id`,
				masteryNumericShadowDeleteTrigger),
		},
	}
}

func validateMasteryNumericShadowSyncTriggers(ctx context.Context, connection *sqlx.Conn) (bool, error) {
	for _, trigger := range masteryNumericShadowSyncTriggers() {
		var count int
		if err := connection.GetContext(ctx, &count, `
			SELECT COUNT(*)
			FROM information_schema.triggers
			WHERE trigger_schema = DATABASE()
			  AND trigger_name = ?
			  AND event_object_table = 'masteries'
			  AND action_timing = 'AFTER'
			  AND event_manipulation = ?
		`, trigger.name, trigger.event); err != nil {
			return false, err
		}
		if count != 1 {
			return false, nil
		}
	}
	return true, nil
}

func ValidateMasteryNumericShadowCutover(ctx context.Context, database *sqlx.DB) error {
	connection, err := database.Connx(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()

	if err := validateMasteryNumericShadowSchema(ctx, connection); err != nil {
		return err
	}
	progress, err := loadMasteryNumericShadowProgress(ctx, connection)
	if err != nil {
		return fmt.Errorf("load mastery numeric shadow cutover progress: %w", err)
	}
	if !progress.CopyCompleted || !progress.Validated {
		return errors.New("mastery numeric shadow copy is not completed and validated")
	}
	triggersReady, err := validateMasteryNumericShadowSyncTriggers(ctx, connection)
	if err != nil {
		return err
	}
	if !triggersReady {
		return errors.New("mastery numeric shadow synchronization triggers are not ready")
	}
	return nil
}

func masteryNumericShadowReady(ctx context.Context, database *sqlx.DB) (bool, error) {
	exists, err := tableExists(ctx, database, masteryNumericShadowProgressTable)
	if err != nil || !exists {
		return false, err
	}
	var progress struct {
		CopyCompleted bool `db:"copy_completed"`
		Validated     bool `db:"validated"`
	}
	if err := database.GetContext(ctx, &progress, `
		SELECT copy_completed, validated
		FROM numeric_key_mastery_shadow_progress
		WHERE state_key = 'masteries'
	`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if !progress.CopyCompleted || !progress.Validated {
		return false, nil
	}
	if err := ValidateMasteryNumericShadowCutover(ctx, database); err != nil {
		return false, nil
	}
	return true, nil
}
