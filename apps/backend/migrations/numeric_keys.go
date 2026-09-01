package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
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
	MasteriesReady        bool
}

type numericKeyProgress struct {
	CursorText   string `db:"cursor_text"`
	CursorNumber int    `db:"cursor_number"`
	Processed    int64  `db:"processed_rows"`
	Completed    bool   `db:"completed"`
}

type participantNumericKeyRow struct {
	MatchId           string        `db:"match_id"`
	ParticipantId     int           `db:"participant_id"`
	LegacyParticipant string        `db:"match_participant_id"`
	Puuid             string        `db:"puuid"`
	MatchNumericId    sql.NullInt64 `db:"match_numeric_id"`
	SummonerNumericId sql.NullInt64 `db:"summoner_numeric_id"`
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
	childrenReady, err := validateNumericKeyChildrenBackfill(ctx, database)
	if err != nil {
		return result, err
	}
	result.MasteriesReady, err = masteryNumericShadowReady(ctx, database)
	if err != nil {
		return result, err
	}
	result.Ready = childrenReady && result.MasteriesReady
	return result, nil
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

		tx, err := beginNumericKeyBackfillTransaction(ctx, database)
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

		tx, err := beginNumericKeyBackfillTransaction(ctx, database)
		if err != nil {
			return false, processedThisRun, err
		}
		err = upsertParticipantNumericKeyBatch(ctx, tx, rows)
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

func upsertParticipantNumericKeyBatch(
	ctx context.Context,
	tx *sqlx.Tx,
	rows []participantNumericKeyRow,
) error {
	matchIds, summonerIds, err := reserveParticipantParentNumericKeys(ctx, tx, rows)
	if err != nil {
		return err
	}
	placeholders := make([]string, 0, len(rows))
	insertArgs := make([]interface{}, 0, len(rows)*4)
	legacyKeys := make([]string, 0, len(rows))
	for index := range rows {
		row := &rows[index]
		if numericId, ok := matchIds[row.MatchId]; ok {
			row.MatchNumericId = sql.NullInt64{Int64: numericId, Valid: true}
		}
		if numericId, ok := summonerIds[row.Puuid]; ok {
			row.SummonerNumericId = sql.NullInt64{Int64: numericId, Valid: true}
		}
		if !row.MatchNumericId.Valid || !row.SummonerNumericId.Valid {
			return fmt.Errorf(
				"participant %s/%d has no matching numeric parent key",
				row.MatchId, row.ParticipantId,
			)
		}
		placeholders = append(placeholders, "(?, ?, ?, ?)")
		insertArgs = append(insertArgs,
			row.LegacyParticipant, row.MatchNumericId.Int64, row.SummonerNumericId.Int64, row.ParticipantId,
		)
		legacyKeys = append(legacyKeys, row.LegacyParticipant)
	}
	insertQuery := `
		INSERT INTO match_participant_numeric_keys
			(legacy_match_participant_id, match_id, summoner_id, participant_id)
		VALUES ` + strings.Join(placeholders, ",") + `
		ON DUPLICATE KEY UPDATE
			match_id = VALUES(match_id), summoner_id = VALUES(summoner_id),
			participant_id = VALUES(participant_id)`
	if _, err := tx.ExecContext(ctx, tx.Rebind(insertQuery), insertArgs...); err != nil {
		return err
	}

	type participantKey struct {
		Id     int64  `db:"match_participant_id"`
		Legacy string `db:"legacy_match_participant_id"`
	}
	keyQuery, keyArgs, err := sqlx.In(`
		SELECT match_participant_id, legacy_match_participant_id
		FROM match_participant_numeric_keys
		WHERE legacy_match_participant_id IN (?)
	`, legacyKeys)
	if err != nil {
		return err
	}
	keys := make([]participantKey, 0, len(rows))
	if err := tx.SelectContext(ctx, &keys, tx.Rebind(keyQuery), keyArgs...); err != nil {
		return err
	}
	ids := make(map[string]int64, len(keys))
	for _, key := range keys {
		ids[key.Legacy] = key.Id
	}
	if len(ids) != len(rows) {
		return fmt.Errorf("resolved %d of %d participant numeric keys", len(ids), len(rows))
	}

	statement, err := tx.PrepareContext(ctx, `
		UPDATE match_participants
		SET match_participant_pk = ?, match_fk = ?, summoner_fk = ?
		WHERE match_id = ? AND participant_id = ?
	`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, row := range rows {
		result, err := statement.ExecContext(ctx,
			ids[row.LegacyParticipant], row.MatchNumericId.Int64, row.SummonerNumericId.Int64,
			row.MatchId, row.ParticipantId,
		)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf(
				"updated %d rows for participant %s/%d; expected 1",
				affected, row.MatchId, row.ParticipantId,
			)
		}
	}
	return nil
}

func reserveParticipantParentNumericKeys(
	ctx context.Context,
	tx *sqlx.Tx,
	rows []participantNumericKeyRow,
) (map[string]int64, map[string]int64, error) {
	matchKeys := make([]string, 0, len(rows))
	summonerKeys := make([]string, 0, len(rows))
	for _, row := range rows {
		matchKeys = append(matchKeys, row.MatchId)
		summonerKeys = append(summonerKeys, row.Puuid)
	}
	matchKeys = uniqueSortedStrings(matchKeys)
	summonerKeys = uniqueSortedStrings(summonerKeys)
	if err := reserveNumericKeyIdentities(ctx, tx, "match_numeric_keys", "riot_match_id", matchKeys); err != nil {
		return nil, nil, err
	}
	if err := reserveNumericKeyIdentities(ctx, tx, "summoner_numeric_keys", "puuid", summonerKeys); err != nil {
		return nil, nil, err
	}
	matchIds, err := loadNumericKeyIdentities(
		ctx, tx, "match_numeric_keys", "riot_match_id", "match_id", matchKeys,
	)
	if err != nil {
		return nil, nil, err
	}
	summonerIds, err := loadNumericKeyIdentities(
		ctx, tx, "summoner_numeric_keys", "puuid", "summoner_id", summonerKeys,
	)
	return matchIds, summonerIds, err
}

func uniqueSortedStrings(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		unique[value] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func reserveNumericKeyIdentities(
	ctx context.Context,
	tx *sqlx.Tx,
	table, legacyColumn string,
	keys []string,
) error {
	placeholders := make([]string, len(keys))
	args := make([]interface{}, len(keys))
	for index, key := range keys {
		placeholders[index] = "(?)"
		args[index] = key
	}
	query := fmt.Sprintf(`
		INSERT INTO %s (%s) VALUES %s
		ON DUPLICATE KEY UPDATE %s = VALUES(%s)
	`, table, legacyColumn, strings.Join(placeholders, ","), legacyColumn, legacyColumn)
	_, err := tx.ExecContext(ctx, tx.Rebind(query), args...)
	return err
}

func loadNumericKeyIdentities(
	ctx context.Context,
	tx *sqlx.Tx,
	table, legacyColumn, numericColumn string,
	keys []string,
) (map[string]int64, error) {
	type numericIdentity struct {
		Legacy string `db:"legacy_key"`
		Id     int64  `db:"numeric_id"`
	}
	query, args, err := sqlx.In(fmt.Sprintf(`
		SELECT %s AS legacy_key, %s AS numeric_id
		FROM %s WHERE %s IN (?)
	`, legacyColumn, numericColumn, table, legacyColumn), keys)
	if err != nil {
		return nil, err
	}
	rows := make([]numericIdentity, 0, len(keys))
	if err := tx.SelectContext(ctx, &rows, tx.Rebind(query), args...); err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		result[row.Legacy] = row.Id
	}
	if len(result) != len(keys) {
		return nil, fmt.Errorf("resolved %d of %d numeric identities from %s", len(result), len(keys), table)
	}
	return result, nil
}

type numericKeyProgressStore interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}

func beginNumericKeyBackfillTransaction(ctx context.Context, database *sqlx.DB) (*sqlx.Tx, error) {
	tx, err := database.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SET SESSION binlog_row_image = 'MINIMAL'`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("set numeric key backfill binlog row image: %w", err)
	}
	return tx, nil
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
