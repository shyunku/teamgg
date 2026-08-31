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

const numericKeyMigrationVersion = "20260830_005"

var numericKeyTriggerNames = []string{
	"summoners_numeric_key_bi",
	"summoners_numeric_key_bu",
	"matches_numeric_key_bi",
	"match_participants_numeric_key_bi",
}

type NumericKeyBackfillOptions struct {
	BatchSize int
	WorkLimit time.Duration
}

type NumericKeyBackfillResult struct {
	Ready                 bool
	SummonersProcessed    int64
	MatchesProcessed      int64
	ParticipantsProcessed int64
	SummonersCompleted    bool
	MatchesCompleted      bool
	ParticipantsCompleted bool
	ChildrenProcessed     int64
	ChildrenCompleted     bool
}

type numericKeyProgress struct {
	CursorText   string `db:"cursor_text"`
	CursorNumber int    `db:"cursor_number"`
	Processed    int64  `db:"processed_rows"`
	Completed    bool   `db:"completed"`
}

type participantNumericKeyRow struct {
	MatchId            string `db:"match_id"`
	ParticipantId      int    `db:"participant_id"`
	MatchParticipantId string `db:"match_participant_id"`
	Puuid              string `db:"puuid"`
}

func applyNumericKeyFoundation(ctx context.Context, database *sqlx.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS summoner_numeric_keys (
			summoner_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			puuid VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			PRIMARY KEY (summoner_id),
			UNIQUE KEY summoner_numeric_keys_puuid_uindex (puuid)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS match_numeric_keys (
			match_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			riot_match_id VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			PRIMARY KEY (match_id),
			UNIQUE KEY match_numeric_keys_riot_match_id_uindex (riot_match_id)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS match_participant_numeric_keys (
			match_participant_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			legacy_match_participant_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
			match_id BIGINT UNSIGNED NULL,
			summoner_id BIGINT UNSIGNED NULL,
			participant_id TINYINT UNSIGNED NULL,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			PRIMARY KEY (match_participant_id),
			UNIQUE KEY match_participant_numeric_keys_legacy_uindex (legacy_match_participant_id),
			UNIQUE KEY match_participant_numeric_keys_match_slot_uindex (match_id, participant_id)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS numeric_key_backfill_progress (
			entity_name VARCHAR(32) NOT NULL,
			cursor_text VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
			cursor_number INT NOT NULL DEFAULT 0,
			processed_rows BIGINT UNSIGNED NOT NULL DEFAULT 0,
			completed TINYINT(1) NOT NULL DEFAULT 0,
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (entity_name)
		) ENGINE=InnoDB`,
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create numeric key foundation: %w", err)
		}
	}

	columns := []struct{ table, column, definition string }{
		{"summoners", "summoner_pk", "BIGINT UNSIGNED NULL"},
		{"matches", "match_pk", "BIGINT UNSIGNED NULL"},
		{"match_participants", "match_participant_pk", "BIGINT UNSIGNED NULL"},
		{"match_participants", "match_fk", "BIGINT UNSIGNED NULL"},
		{"match_participants", "summoner_fk", "BIGINT UNSIGNED NULL"},
	}
	for _, column := range columns {
		if err := ensureNumericKeyColumn(ctx, database, column.table, column.column, column.definition); err != nil {
			return err
		}
	}

	for _, trigger := range numericKeyTriggers() {
		if _, err := database.ExecContext(ctx, "DROP TRIGGER IF EXISTS `"+trigger.name+"`"); err != nil {
			return fmt.Errorf("drop numeric key trigger %s: %w", trigger.name, err)
		}
		if _, err := database.ExecContext(ctx, trigger.statement); err != nil {
			return fmt.Errorf("create numeric key trigger %s: %w", trigger.name, err)
		}
	}
	return nil
}

func ensureNumericKeyColumn(ctx context.Context, database *sqlx.DB, table, column, definition string) error {
	exists, err := columnExists(ctx, database, table, column)
	if err != nil || exists {
		return err
	}
	statement := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s, ALGORITHM=INSTANT", table, column, definition)
	if _, err := database.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("add numeric key column %s.%s: %w", table, column, err)
	}
	return nil
}

type numericKeyTrigger struct {
	name      string
	statement string
}

func numericKeyTriggers() []numericKeyTrigger {
	return []numericKeyTrigger{
		{
			name: "summoners_numeric_key_bi",
			statement: `CREATE TRIGGER summoners_numeric_key_bi BEFORE INSERT ON summoners
			FOR EACH ROW
			BEGIN
				INSERT INTO summoner_numeric_keys (puuid) VALUES (NEW.puuid)
				ON DUPLICATE KEY UPDATE summoner_id = LAST_INSERT_ID(summoner_id);
				SET NEW.summoner_pk = LAST_INSERT_ID();
			END`,
		},
		{
			name: "summoners_numeric_key_bu",
			statement: `CREATE TRIGGER summoners_numeric_key_bu BEFORE UPDATE ON summoners
			FOR EACH ROW
			BEGIN
				IF NEW.summoner_pk IS NULL OR NOT (NEW.puuid <=> OLD.puuid) THEN
					INSERT INTO summoner_numeric_keys (puuid) VALUES (NEW.puuid)
					ON DUPLICATE KEY UPDATE summoner_id = LAST_INSERT_ID(summoner_id);
					SET NEW.summoner_pk = LAST_INSERT_ID();
				END IF;
			END`,
		},
		{
			name: "matches_numeric_key_bi",
			statement: `CREATE TRIGGER matches_numeric_key_bi BEFORE INSERT ON matches
			FOR EACH ROW
			BEGIN
				INSERT INTO match_numeric_keys (riot_match_id) VALUES (NEW.match_id)
				ON DUPLICATE KEY UPDATE match_id = LAST_INSERT_ID(match_id);
				SET NEW.match_pk = LAST_INSERT_ID();
			END`,
		},
		numericKeyParticipantInsertTrigger(),
	}
}

func numericKeyParticipantInsertTrigger() numericKeyTrigger {
	return numericKeyTrigger{
		name: "match_participants_numeric_key_bi",
		statement: `CREATE TRIGGER match_participants_numeric_key_bi BEFORE INSERT ON match_participants
			FOR EACH ROW
			BEGIN
				INSERT INTO match_numeric_keys (riot_match_id) VALUES (NEW.match_id)
				ON DUPLICATE KEY UPDATE match_id = LAST_INSERT_ID(match_id);
				SET NEW.match_fk = LAST_INSERT_ID();
				INSERT INTO summoner_numeric_keys (puuid) VALUES (NEW.puuid)
				ON DUPLICATE KEY UPDATE summoner_id = LAST_INSERT_ID(summoner_id);
				SET NEW.summoner_fk = LAST_INSERT_ID();
				INSERT INTO match_participant_numeric_keys
					(legacy_match_participant_id, match_id, summoner_id, participant_id)
				VALUES
					(NEW.match_participant_id, NEW.match_fk, NEW.summoner_fk, NEW.participant_id)
				ON DUPLICATE KEY UPDATE
					match_participant_id = LAST_INSERT_ID(match_participant_id),
					match_id = VALUES(match_id), summoner_id = VALUES(summoner_id),
					participant_id = VALUES(participant_id);
				SET NEW.match_participant_pk = LAST_INSERT_ID();
			END`,
	}
}

func validateNumericKeyFoundation(ctx context.Context, database *sqlx.DB) (bool, error) {
	tables, err := tablesExist(ctx, database,
		"summoner_numeric_keys", "match_numeric_keys",
		"match_participant_numeric_keys", "numeric_key_backfill_progress",
	)
	if err != nil || !tables {
		return false, err
	}
	columns, err := columnsExist(ctx, database, map[string][]string{
		"summoners":                      {"summoner_pk"},
		"matches":                        {"match_pk"},
		"match_participants":             {"match_participant_pk", "match_fk", "summoner_fk"},
		"summoner_numeric_keys":          {"summoner_id", "puuid"},
		"match_numeric_keys":             {"match_id", "riot_match_id"},
		"match_participant_numeric_keys": {"match_participant_id", "legacy_match_participant_id", "match_id", "summoner_id", "participant_id"},
		"numeric_key_backfill_progress":  {"entity_name", "cursor_text", "cursor_number", "processed_rows", "completed"},
	})
	if err != nil || !columns {
		return false, err
	}
	indexes := []struct {
		table   string
		name    string
		columns []string
	}{
		{"summoner_numeric_keys", "PRIMARY", []string{"summoner_id"}},
		{"summoner_numeric_keys", "summoner_numeric_keys_puuid_uindex", []string{"puuid"}},
		{"match_numeric_keys", "PRIMARY", []string{"match_id"}},
		{"match_numeric_keys", "match_numeric_keys_riot_match_id_uindex", []string{"riot_match_id"}},
		{"match_participant_numeric_keys", "PRIMARY", []string{"match_participant_id"}},
		{"match_participant_numeric_keys", "match_participant_numeric_keys_legacy_uindex", []string{"legacy_match_participant_id"}},
		{"match_participant_numeric_keys", "match_participant_numeric_keys_match_slot_uindex", []string{"match_id", "participant_id"}},
		{"numeric_key_backfill_progress", "PRIMARY", []string{"entity_name"}},
	}
	for _, index := range indexes {
		valid, err := indexMatches(ctx, database, index.table, index.name, index.columns...)
		if err != nil || !valid {
			return false, err
		}
	}
	for _, name := range numericKeyTriggerNames {
		var count int
		if err := database.GetContext(ctx, &count, `
			SELECT COUNT(*) FROM information_schema.triggers
			WHERE trigger_schema = DATABASE() AND trigger_name = ?
		`, name); err != nil || count != 1 {
			return false, err
		}
	}
	return true, nil
}

func NumericKeyBackfillOptionsFromEnvironment() NumericKeyBackfillOptions {
	return NumericKeyBackfillOptions{
		BatchSize: boundedNumericKeyBatchSize(os.Getenv("NUMERIC_KEY_BACKFILL_BATCH_SIZE")),
		WorkLimit: boundedNumericKeyWorkLimit(os.Getenv("NUMERIC_KEY_BACKFILL_WORK_LIMIT")),
	}
}

func boundedNumericKeyBatchSize(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 10 || parsed > 10000 {
		return 1000
	}
	return parsed
}

func boundedNumericKeyWorkLimit(value string) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || parsed < time.Second || parsed > time.Hour {
		return 10 * time.Minute
	}
	return parsed
}

func BackfillNumericKeys(ctx context.Context, database *sqlx.DB, options NumericKeyBackfillOptions) (NumericKeyBackfillResult, error) {
	result := NumericKeyBackfillResult{}
	if options.BatchSize < 10 || options.BatchSize > 10000 {
		options.BatchSize = 1000
	}
	if options.WorkLimit < time.Second || options.WorkLimit > time.Hour {
		options.WorkLimit = 10 * time.Minute
	}
	ready, err := validateNumericKeyFoundation(ctx, database)
	if err != nil || !ready {
		if err == nil {
			err = errors.New("numeric key foundation is incomplete; run migrations first")
		}
		return result, err
	}

	deadline := time.Now().Add(options.WorkLimit)
	result.SummonersCompleted, result.SummonersProcessed, err = backfillSimpleNumericKeys(
		ctx, database, deadline, options.BatchSize,
		"summoners", "summoners", "puuid", "summoner_pk",
		"summoner_numeric_keys", "puuid", "summoner_id",
	)
	if err != nil || !result.SummonersCompleted {
		return result, err
	}
	result.MatchesCompleted, result.MatchesProcessed, err = backfillSimpleNumericKeys(
		ctx, database, deadline, options.BatchSize,
		"matches", "matches", "match_id", "match_pk",
		"match_numeric_keys", "riot_match_id", "match_id",
	)
	if err != nil || !result.MatchesCompleted {
		return result, err
	}
	result.ParticipantsCompleted, result.ParticipantsProcessed, err = backfillParticipantNumericKeys(
		ctx, database, deadline, options.BatchSize,
	)
	if err != nil || !result.ParticipantsCompleted {
		return result, err
	}

	parentsReady, err := validateNumericKeyBackfill(ctx, database)
	if err != nil || !parentsReady {
		return result, err
	}
	result.ChildrenCompleted, result.ChildrenProcessed, err = backfillNumericKeyChildren(
		ctx, database, deadline, options.BatchSize,
	)
	if err != nil || !result.ChildrenCompleted {
		return result, err
	}
	result.Ready, err = validateNumericKeyChildrenBackfill(ctx, database)
	return result, err
}

func backfillSimpleNumericKeys(
	ctx context.Context,
	database *sqlx.DB,
	deadline time.Time,
	batchSize int,
	entity, sourceTable, legacyColumn, sourceNumericColumn, keyTable, keyLegacyColumn, keyNumericColumn string,
) (bool, int64, error) {
	progress, err := loadNumericKeyProgress(ctx, database, entity)
	if err != nil {
		return false, 0, err
	}
	processedThisRun := int64(0)
	for time.Now().Before(deadline) {
		keys := make([]string, 0, batchSize)
		query := fmt.Sprintf(
			`SELECT %s FROM %s WHERE %s IS NULL AND %s > ? ORDER BY %s LIMIT ?`,
			legacyColumn, sourceTable, sourceNumericColumn, legacyColumn, legacyColumn,
		)
		if err := database.SelectContext(ctx, &keys, query, progress.CursorText, batchSize); err != nil {
			return false, processedThisRun, err
		}
		if len(keys) == 0 && progress.CursorText != "" {
			progress.CursorText = ""
			if err := database.SelectContext(ctx, &keys, query, "", batchSize); err != nil {
				return false, processedThisRun, err
			}
		}
		if len(keys) == 0 {
			if err := saveNumericKeyProgress(ctx, database, entity, progress.CursorText, 0, progress.Processed, true); err != nil {
				return false, processedThisRun, err
			}
			return true, processedThisRun, nil
		}

		tx, err := database.BeginTxx(ctx, nil)
		if err != nil {
			return false, processedThisRun, err
		}
		insertQuery, insertArgs, err := sqlx.In(
			fmt.Sprintf(
				`INSERT INTO %s (%s) SELECT %s FROM %s WHERE %s IN (?) ON DUPLICATE KEY UPDATE %s = VALUES(%s)`,
				keyTable, keyLegacyColumn, legacyColumn, sourceTable, legacyColumn, keyNumericColumn, keyNumericColumn,
			),
			keys,
		)
		if err == nil {
			_, err = tx.ExecContext(ctx, tx.Rebind(insertQuery), insertArgs...)
		}
		if err == nil {
			updateQuery, updateArgs, bindErr := sqlx.In(
				fmt.Sprintf(
					`UPDATE %s source INNER JOIN %s numeric_key ON numeric_key.%s = source.%s SET source.%s = numeric_key.%s WHERE source.%s IN (?) AND source.%s IS NULL`,
					sourceTable, keyTable, keyLegacyColumn, legacyColumn, sourceNumericColumn, keyNumericColumn, legacyColumn, sourceNumericColumn,
				),
				keys,
			)
			if bindErr != nil {
				err = bindErr
			} else {
				_, err = tx.ExecContext(ctx, tx.Rebind(updateQuery), updateArgs...)
			}
		}
		if err != nil {
			_ = tx.Rollback()
			return false, processedThisRun, fmt.Errorf("backfill numeric keys for %s: %w", entity, err)
		}

		progress.CursorText = keys[len(keys)-1]
		progress.Processed += int64(len(keys))
		if err := saveNumericKeyProgress(ctx, tx, entity, progress.CursorText, 0, progress.Processed, false); err != nil {
			_ = tx.Rollback()
			return false, processedThisRun, err
		}
		if err := tx.Commit(); err != nil {
			return false, processedThisRun, err
		}
		processedThisRun += int64(len(keys))
	}
	return false, processedThisRun, nil
}

func backfillParticipantNumericKeys(ctx context.Context, database *sqlx.DB, deadline time.Time, batchSize int) (bool, int64, error) {
	progress, err := loadNumericKeyProgress(ctx, database, "match_participants")
	if err != nil {
		return false, 0, err
	}
	processedThisRun := int64(0)
	for time.Now().Before(deadline) {
		rows := make([]participantNumericKeyRow, 0, batchSize)
		query := `
			SELECT match_id, participant_id, match_participant_id, puuid
			FROM match_participants FORCE INDEX (PRIMARY)
			WHERE (match_participant_pk IS NULL OR match_fk IS NULL OR summoner_fk IS NULL)
			  AND (match_id > ? OR (match_id = ? AND participant_id > ?))
			ORDER BY match_id, participant_id
			LIMIT ?`
		if err := database.SelectContext(ctx, &rows, query, progress.CursorText, progress.CursorText, progress.CursorNumber, batchSize); err != nil {
			return false, processedThisRun, err
		}
		if len(rows) == 0 && (progress.CursorText != "" || progress.CursorNumber != 0) {
			progress.CursorText, progress.CursorNumber = "", 0
			if err := database.SelectContext(ctx, &rows, query, "", "", 0, batchSize); err != nil {
				return false, processedThisRun, err
			}
		}
		if len(rows) == 0 {
			if err := saveNumericKeyProgress(ctx, database, "match_participants", progress.CursorText, progress.CursorNumber, progress.Processed, true); err != nil {
				return false, processedThisRun, err
			}
			return true, processedThisRun, nil
		}

		participantKeys := make([]string, 0, len(rows))
		for _, row := range rows {
			participantKeys = append(participantKeys, row.MatchParticipantId)
		}
		tx, err := database.BeginTxx(ctx, nil)
		if err != nil {
			return false, processedThisRun, err
		}
		insertQuery, insertArgs, err := sqlx.In(`
			INSERT INTO match_participant_numeric_keys (legacy_match_participant_id)
			SELECT match_participant_id FROM match_participants WHERE match_participant_id IN (?)
			ON DUPLICATE KEY UPDATE
				legacy_match_participant_id = VALUES(legacy_match_participant_id)
		`, participantKeys)
		if err == nil {
			_, err = tx.ExecContext(ctx, tx.Rebind(insertQuery), insertArgs...)
		}
		if err == nil {
			updateMapQuery, updateMapArgs, bindErr := sqlx.In(`
				UPDATE match_participant_numeric_keys participant_key
				INNER JOIN match_participants participant
					ON participant.match_participant_id = participant_key.legacy_match_participant_id
				INNER JOIN match_numeric_keys match_key ON match_key.riot_match_id = participant.match_id
				INNER JOIN summoner_numeric_keys summoner_key ON summoner_key.puuid = participant.puuid
				SET participant_key.match_id = match_key.match_id,
					participant_key.summoner_id = summoner_key.summoner_id,
					participant_key.participant_id = participant.participant_id
				WHERE participant_key.legacy_match_participant_id IN (?)
			`, participantKeys)
			if bindErr != nil {
				err = bindErr
			} else {
				_, err = tx.ExecContext(ctx, tx.Rebind(updateMapQuery), updateMapArgs...)
			}
		}
		if err == nil {
			updateSourceQuery, updateSourceArgs, bindErr := sqlx.In(`
				UPDATE match_participants participant
				INNER JOIN match_participant_numeric_keys participant_key
					ON participant_key.legacy_match_participant_id = participant.match_participant_id
				SET participant.match_participant_pk = participant_key.match_participant_id,
					participant.match_fk = participant_key.match_id,
					participant.summoner_fk = participant_key.summoner_id
				WHERE participant.match_participant_id IN (?)
				  AND participant_key.match_id IS NOT NULL
				  AND participant_key.summoner_id IS NOT NULL
			`, participantKeys)
			if bindErr != nil {
				err = bindErr
			} else {
				_, err = tx.ExecContext(ctx, tx.Rebind(updateSourceQuery), updateSourceArgs...)
			}
		}
		if err == nil {
			validationQuery, validationArgs, bindErr := sqlx.In(`
				SELECT COUNT(*)
				FROM match_participants participant
				LEFT JOIN match_participant_numeric_keys participant_key
					ON participant_key.legacy_match_participant_id = participant.match_participant_id
				WHERE participant.match_participant_id IN (?)
				  AND (participant.match_participant_pk IS NULL
				    OR participant.match_fk IS NULL OR participant.summoner_fk IS NULL
				    OR participant_key.match_id IS NULL OR participant_key.summoner_id IS NULL)
			`, participantKeys)
			if bindErr != nil {
				err = bindErr
			} else {
				var unresolved int
				err = tx.GetContext(ctx, &unresolved, tx.Rebind(validationQuery), validationArgs...)
				if err == nil && unresolved != 0 {
					err = fmt.Errorf(
						"%d participant rows have no matching summoner or match numeric key",
						unresolved,
					)
				}
			}
		}
		if err != nil {
			_ = tx.Rollback()
			return false, processedThisRun, fmt.Errorf("backfill participant numeric keys: %w", err)
		}

		last := rows[len(rows)-1]
		progress.CursorText, progress.CursorNumber = last.MatchId, last.ParticipantId
		progress.Processed += int64(len(rows))
		if err := saveNumericKeyProgress(ctx, tx, "match_participants", progress.CursorText, progress.CursorNumber, progress.Processed, false); err != nil {
			_ = tx.Rollback()
			return false, processedThisRun, err
		}
		if err := tx.Commit(); err != nil {
			return false, processedThisRun, err
		}
		processedThisRun += int64(len(rows))
	}
	return false, processedThisRun, nil
}

type numericKeyProgressStore interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}

func loadNumericKeyProgress(ctx context.Context, database *sqlx.DB, entity string) (numericKeyProgress, error) {
	progress := numericKeyProgress{}
	err := database.GetContext(ctx, &progress, `
		SELECT cursor_text, cursor_number, processed_rows, completed
		FROM numeric_key_backfill_progress WHERE entity_name = ?
	`, entity)
	if errors.Is(err, sql.ErrNoRows) {
		return progress, nil
	}
	return progress, err
}

func saveNumericKeyProgress(
	ctx context.Context,
	store numericKeyProgressStore,
	entity, cursorText string,
	cursorNumber int,
	processed int64,
	completed bool,
) error {
	_, err := store.ExecContext(ctx, `
		INSERT INTO numeric_key_backfill_progress
			(entity_name, cursor_text, cursor_number, processed_rows, completed)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			cursor_text = VALUES(cursor_text), cursor_number = VALUES(cursor_number),
			processed_rows = VALUES(processed_rows), completed = VALUES(completed),
			updated_at = CURRENT_TIMESTAMP(6)
	`, entity, cursorText, cursorNumber, processed, completed)
	return err
}

func validateNumericKeyBackfill(ctx context.Context, database *sqlx.DB) (bool, error) {
	checks := []string{
		`SELECT COUNT(*) FROM summoners source
		 LEFT JOIN summoner_numeric_keys numeric_key ON numeric_key.puuid = source.puuid
		 WHERE source.summoner_pk IS NULL OR numeric_key.summoner_id IS NULL
		    OR source.summoner_pk <> numeric_key.summoner_id`,
		`SELECT COUNT(*) FROM matches source
		 LEFT JOIN match_numeric_keys numeric_key ON numeric_key.riot_match_id = source.match_id
		 WHERE source.match_pk IS NULL OR numeric_key.match_id IS NULL
		    OR source.match_pk <> numeric_key.match_id`,
		`SELECT COUNT(*) FROM match_participants source
		 LEFT JOIN match_participant_numeric_keys numeric_key
		   ON numeric_key.legacy_match_participant_id = source.match_participant_id
		 WHERE source.match_participant_pk IS NULL OR source.match_fk IS NULL OR source.summoner_fk IS NULL
		    OR numeric_key.match_participant_id IS NULL
		    OR numeric_key.match_id IS NULL OR numeric_key.summoner_id IS NULL
		    OR source.match_participant_pk <> numeric_key.match_participant_id
		    OR source.match_fk <> numeric_key.match_id
		    OR source.summoner_fk <> numeric_key.summoner_id`,
	}
	for _, query := range checks {
		var count int64
		if err := database.GetContext(ctx, &count, query); err != nil {
			return false, err
		}
		if count != 0 {
			return false, nil
		}
	}
	return true, nil
}
