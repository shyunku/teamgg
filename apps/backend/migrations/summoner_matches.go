package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

const (
	summonerMatchesMigrationVersion = "20260720_001"
	summonerMatchesTargetTable      = "summoner_matches_v2"
	summonerMatchesLegacyTable      = "summoner_matches_legacy_20260820"
	summonerMatchesMirrorTrigger    = "teamgg_summoner_matches_migration_insert"
)

type summonerMatchPair struct {
	Puuid   string `db:"puuid"`
	MatchId string `db:"match_id"`
}

func ensureSummonerMatchesPrimaryKey(ctx context.Context, database *sqlx.DB) error {
	valid, err := indexMatches(ctx, database, "summoner_matches", "PRIMARY", "puuid", "match_id")
	if err != nil {
		return err
	}
	if valid {
		_, _ = database.ExecContext(ctx, "DROP TRIGGER IF EXISTS "+summonerMatchesMirrorTrigger)
		return nil
	}

	legacyExists, err := tableExists(ctx, database, summonerMatchesLegacyTable)
	if err != nil {
		return err
	}
	if legacyExists {
		return fmt.Errorf(
			"%s already exists while summoner_matches still lacks its primary key; resolve the previous table rebuild before retrying",
			summonerMatchesLegacyTable,
		)
	}

	if err := prepareSummonerMatchesTarget(ctx, database); err != nil {
		return err
	}
	if err := ensureSummonerMatchesMirrorTrigger(ctx, database); err != nil {
		return err
	}
	if err := backfillSummonerMatchesByKeyset(ctx, database); err != nil {
		return err
	}
	if err := copyMissingSummonerMatches(ctx, database); err != nil {
		return err
	}

	if _, err := database.ExecContext(ctx, fmt.Sprintf(`
		RENAME TABLE summoner_matches TO %s, %s TO summoner_matches
	`, summonerMatchesLegacyTable, summonerMatchesTargetTable)); err != nil {
		return fmt.Errorf("atomically replace summoner_matches: %w", err)
	}
	if _, err := database.ExecContext(ctx, "DROP TRIGGER IF EXISTS "+summonerMatchesMirrorTrigger); err != nil {
		return fmt.Errorf("drop summoner_matches mirror trigger: %w", err)
	}
	valid, err = indexMatches(ctx, database, "summoner_matches", "PRIMARY", "puuid", "match_id")
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("summoner_matches replacement does not have PRIMARY KEY (puuid, match_id)")
	}
	return nil
}

func prepareSummonerMatchesTarget(ctx context.Context, database *sqlx.DB) error {
	exists, err := tableExists(ctx, database, summonerMatchesTargetTable)
	if err != nil {
		return err
	}
	if !exists {
		if _, err := database.ExecContext(ctx, "CREATE TABLE "+summonerMatchesTargetTable+" LIKE summoner_matches"); err != nil {
			return fmt.Errorf("create summoner_matches replacement: %w", err)
		}
	}

	primary, err := indexColumns(ctx, database, summonerMatchesTargetTable, "PRIMARY")
	if err != nil {
		return err
	}
	if len(primary) == 0 {
		if _, err := database.ExecContext(ctx, "ALTER TABLE "+summonerMatchesTargetTable+" ADD PRIMARY KEY (puuid, match_id)"); err != nil {
			return fmt.Errorf("add replacement primary key: %w", err)
		}
	} else {
		valid, err := indexMatches(ctx, database, summonerMatchesTargetTable, "PRIMARY", "puuid", "match_id")
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("existing %s has an unexpected primary key %v", summonerMatchesTargetTable, primary)
		}
	}
	if err := ensureIndex(
		ctx,
		database,
		summonerMatchesTargetTable,
		"summoner_matches_v2_match_id_index",
		"match_id",
		"match_id",
	); err != nil {
		return err
	}
	if err := ensureForeignKey(
		ctx,
		database,
		summonerMatchesTargetTable,
		"teamgg_sm_v2_matches_fk",
		"match_id",
		"matches",
		"match_id",
	); err != nil {
		return err
	}
	return ensureForeignKey(
		ctx,
		database,
		summonerMatchesTargetTable,
		"teamgg_sm_v2_summoners_fk",
		"puuid",
		"summoners",
		"puuid",
	)
}

func ensureForeignKey(
	ctx context.Context,
	database *sqlx.DB,
	table string,
	constraint string,
	column string,
	referencedTable string,
	referencedColumn string,
) error {
	var count int
	if err := database.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM information_schema.key_column_usage
		WHERE constraint_schema = DATABASE() AND table_name = ?
		  AND column_name = ? AND referenced_table_name = ? AND referenced_column_name = ?
	`, table, column, referencedTable, referencedColumn); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := database.ExecContext(ctx, fmt.Sprintf(`
		ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s)
		REFERENCES %s (%s) ON UPDATE CASCADE ON DELETE CASCADE
	`, table, constraint, column, referencedTable, referencedColumn))
	return err
}

func ensureSummonerMatchesMirrorTrigger(ctx context.Context, database *sqlx.DB) error {
	var eventTable sql.NullString
	err := database.GetContext(ctx, &eventTable, `
		SELECT event_object_table FROM information_schema.triggers
		WHERE trigger_schema = DATABASE() AND trigger_name = ?
	`, summonerMatchesMirrorTrigger)
	if err == nil {
		if eventTable.Valid && eventTable.String == "summoner_matches" {
			return nil
		}
		return fmt.Errorf("trigger %s exists on unexpected table %s", summonerMatchesMirrorTrigger, eventTable.String)
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = database.ExecContext(ctx, fmt.Sprintf(`
		CREATE TRIGGER %s AFTER INSERT ON summoner_matches
		FOR EACH ROW
		INSERT IGNORE INTO %s (puuid, match_id) VALUES (NEW.puuid, NEW.match_id)
	`, summonerMatchesMirrorTrigger, summonerMatchesTargetTable))
	return err
}

func backfillSummonerMatchesByKeyset(ctx context.Context, database *sqlx.DB) error {
	matchCursor, _, err := loadMigrationProgress(ctx, database, summonerMatchesMigrationVersion, "match_id")
	if err != nil {
		return err
	}
	puuidCursor, _, err := loadMigrationProgress(ctx, database, summonerMatchesMigrationVersion, "puuid")
	if err != nil {
		return err
	}
	batchSize := summonerMatchesCatchupBatch()
	for {
		rows := make([]summonerMatchPair, 0, batchSize)
		if err := database.SelectContext(ctx, &rows, `
			SELECT puuid, match_id
			FROM summoner_matches
			WHERE match_id > ? OR (match_id = ? AND puuid > ?)
			ORDER BY match_id, puuid
			LIMIT ?
		`, matchCursor, matchCursor, puuidCursor, batchSize); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := insertSummonerMatchPairs(ctx, database, rows); err != nil {
			return err
		}
		last := rows[len(rows)-1]
		if err := saveSummonerMatchCursor(ctx, database, last); err != nil {
			return err
		}
		matchCursor = last.MatchId
		puuidCursor = last.Puuid
	}
}

func saveSummonerMatchCursor(ctx context.Context, database *sqlx.DB, cursor summonerMatchPair) error {
	transaction, err := database.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback() }()
	statement := `
		INSERT INTO schema_migration_progress (migration_version, state_key, state_value)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE state_value = VALUES(state_value), updated_at = NOW(6)
	`
	if _, err := transaction.ExecContext(
		ctx,
		statement,
		summonerMatchesMigrationVersion,
		"match_id",
		cursor.MatchId,
	); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(
		ctx,
		statement,
		summonerMatchesMigrationVersion,
		"puuid",
		cursor.Puuid,
	); err != nil {
		return err
	}
	return transaction.Commit()
}

func copyMissingSummonerMatches(ctx context.Context, database *sqlx.DB) error {
	batchSize := summonerMatchesCatchupBatch()
	for {
		missing := make([]summonerMatchPair, 0, batchSize)
		if err := database.SelectContext(ctx, &missing, fmt.Sprintf(`
			SELECT source.puuid, source.match_id
			FROM summoner_matches source
			LEFT JOIN %s target
			       ON target.puuid = source.puuid AND target.match_id = source.match_id
			WHERE target.puuid IS NULL
			LIMIT ?
		`, summonerMatchesTargetTable), batchSize); err != nil {
			return err
		}
		if len(missing) == 0 {
			return nil
		}
		if err := insertSummonerMatchPairs(ctx, database, missing); err != nil {
			return err
		}
	}
}

func insertSummonerMatchPairs(ctx context.Context, database *sqlx.DB, pairs []summonerMatchPair) error {
	values := make([]string, 0, len(pairs))
	args := make([]interface{}, 0, len(pairs)*2)
	for _, pair := range pairs {
		values = append(values, "(?, ?)")
		args = append(args, pair.Puuid, pair.MatchId)
	}
	_, err := database.ExecContext(ctx, fmt.Sprintf(`
		INSERT IGNORE INTO %s (puuid, match_id) VALUES %s
	`, summonerMatchesTargetTable, strings.Join(values, ",")), args...)
	return err
}

func loadMigrationProgress(
	ctx context.Context,
	database *sqlx.DB,
	version string,
	key string,
) (string, bool, error) {
	var value string
	err := database.GetContext(ctx, &value, `
		SELECT state_value FROM schema_migration_progress
		WHERE migration_version = ? AND state_key = ?
	`, version, key)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return value, err == nil, err
}

func summonerMatchesCatchupBatch() int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv("DB_MIGRATION_SUMMONER_MATCH_BATCH")))
	if err != nil || value < 100 || value > 5000 {
		return 1000
	}
	return value
}
