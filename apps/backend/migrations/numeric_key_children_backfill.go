package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type numericKeyChildProgress struct {
	CursorText  string `db:"cursor_text"`
	CursorText2 string `db:"cursor_text_2"`
	Processed   int64  `db:"processed_rows"`
	Completed   bool   `db:"completed"`
}

type numericKeyChildBackfillSpec struct {
	entity        string
	legacyColumn  string
	numericColumn string
	table         string
	updateJoin    string
	batchLimit    int
}

func numericKeyChildBackfillSpecs() []numericKeyChildBackfillSpec {
	return []numericKeyChildBackfillSpec{
		{
			entity: "leagues", legacyColumn: "puuid", numericColumn: "summoner_fk", table: "leagues",
			updateJoin: `INNER JOIN summoner_numeric_keys numeric_key ON numeric_key.puuid = source.puuid
				SET source.summoner_fk = numeric_key.summoner_id`,
		},
		{
			entity: "summoner_matches", legacyColumn: "puuid", numericColumn: "summoner_fk", table: "summoner_matches",
			updateJoin: `INNER JOIN summoner_numeric_keys summoner_key ON summoner_key.puuid = source.puuid
				INNER JOIN match_numeric_keys match_key ON match_key.riot_match_id = source.match_id
				SET source.summoner_fk = summoner_key.summoner_id, source.match_fk = match_key.match_id`,
		},
		{
			entity: "match_teams", legacyColumn: "match_id", numericColumn: "match_fk", table: "match_teams",
			updateJoin: `INNER JOIN match_numeric_keys numeric_key ON numeric_key.riot_match_id = source.match_id
				SET source.match_fk = numeric_key.match_id`,
		},
		{
			entity: "match_team_bans", legacyColumn: "match_id", numericColumn: "match_fk", table: "match_team_bans",
			updateJoin: `INNER JOIN match_numeric_keys numeric_key ON numeric_key.riot_match_id = source.match_id
				SET source.match_fk = numeric_key.match_id`,
		},
		{
			entity: "match_participant_details", legacyColumn: "match_participant_id", numericColumn: "match_participant_fk", table: "match_participant_details",
			updateJoin: `INNER JOIN match_participant_numeric_keys participant_key ON participant_key.legacy_match_participant_id = source.match_participant_id
				INNER JOIN match_numeric_keys match_key ON match_key.riot_match_id = source.match_id
				SET source.match_participant_fk = participant_key.match_participant_id, source.match_fk = match_key.match_id`,
		},
		{
			entity: "match_participant_perks", legacyColumn: "match_participant_id", numericColumn: "match_participant_fk", table: "match_participant_perks",
			updateJoin: `INNER JOIN match_participant_numeric_keys numeric_key ON numeric_key.legacy_match_participant_id = source.match_participant_id
				SET source.match_participant_fk = numeric_key.match_participant_id`,
		},
		{
			entity: "match_participant_perk_styles", legacyColumn: "match_participant_id", numericColumn: "match_participant_fk", table: "match_participant_perk_styles",
			updateJoin: `INNER JOIN match_participant_numeric_keys numeric_key ON numeric_key.legacy_match_participant_id = source.match_participant_id
				SET source.match_participant_fk = numeric_key.match_participant_id`,
		},
		{
			entity: "masteries", legacyColumn: "puuid", numericColumn: "summoner_fk", table: "masteries", batchLimit: 100,
			updateJoin: `INNER JOIN summoner_numeric_keys numeric_key ON numeric_key.puuid = source.puuid
				SET source.summoner_fk = numeric_key.summoner_id`,
		},
	}
}

func backfillNumericKeyChildren(
	ctx context.Context,
	database *sqlx.DB,
	deadline time.Time,
	batchSize int,
) (bool, int64, error) {
	ready, err := validateNumericKeyChildren(ctx, database)
	if err != nil || !ready {
		if err == nil {
			err = errors.New("numeric key child foundation is incomplete; run migrations first")
		}
		return false, 0, err
	}

	processed := int64(0)
	for _, spec := range numericKeyChildBackfillSpecs() {
		completed, count, err := backfillNumericKeyChild(ctx, database, deadline, batchSize, spec)
		processed += count
		if err != nil || !completed {
			return false, processed, err
		}
	}
	return true, processed, nil
}

func backfillNumericKeyChild(
	ctx context.Context,
	database *sqlx.DB,
	deadline time.Time,
	batchSize int,
	spec numericKeyChildBackfillSpec,
) (bool, int64, error) {
	progress, err := loadNumericKeyChildProgress(ctx, database, spec.entity)
	if err != nil || progress.Completed {
		return progress.Completed, 0, err
	}
	if spec.batchLimit > 0 && batchSize > spec.batchLimit {
		batchSize = spec.batchLimit
	}

	processedThisRun := int64(0)
	for time.Now().Before(deadline) {
		keys, err := selectNumericKeyChildBatch(ctx, database, spec, progress.CursorText, batchSize)
		if err != nil {
			return false, processedThisRun, err
		}
		if len(keys) == 0 && progress.CursorText != "" {
			progress.CursorText = ""
			keys, err = selectNumericKeyChildBatch(ctx, database, spec, "", batchSize)
			if err != nil {
				return false, processedThisRun, err
			}
		}
		if len(keys) == 0 {
			if err := saveNumericKeyChildProgress(ctx, database, spec.entity, "", progress.Processed, true); err != nil {
				return false, processedThisRun, err
			}
			return true, processedThisRun, nil
		}

		query, args, err := sqlx.In(fmt.Sprintf(`
			UPDATE %s source
			%s
			WHERE source.%s IN (?) AND source.%s IS NULL
		`, spec.table, spec.updateJoin, spec.legacyColumn, spec.numericColumn), keys)
		if err != nil {
			return false, processedThisRun, err
		}
		tx, err := beginNumericKeyBackfillTransaction(ctx, database)
		if err != nil {
			return false, processedThisRun, err
		}
		result, err := tx.ExecContext(ctx, tx.Rebind(query), args...)
		if err != nil {
			_ = tx.Rollback()
			return false, processedThisRun, fmt.Errorf("backfill numeric child %s: %w", spec.entity, err)
		}
		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			_ = tx.Rollback()
			if err == nil {
				err = fmt.Errorf("selected %d keys but updated no rows", len(keys))
			}
			return false, processedThisRun, fmt.Errorf("backfill numeric child %s: %w", spec.entity, err)
		}

		progress.CursorText = keys[len(keys)-1]
		progress.Processed += affected
		if err := saveNumericKeyChildProgress(ctx, tx, spec.entity, progress.CursorText, progress.Processed, false); err != nil {
			_ = tx.Rollback()
			return false, processedThisRun, err
		}
		if err := tx.Commit(); err != nil {
			return false, processedThisRun, err
		}
		processedThisRun += affected
	}
	return false, processedThisRun, nil
}

func selectNumericKeyChildBatch(
	ctx context.Context,
	database *sqlx.DB,
	spec numericKeyChildBackfillSpec,
	cursor string,
	batchSize int,
) ([]string, error) {
	keys := make([]string, 0, batchSize)
	query := fmt.Sprintf(`
		SELECT DISTINCT %s FROM %s
		WHERE %s IS NULL AND %s > ?
		ORDER BY %s LIMIT ?
	`, spec.legacyColumn, spec.table, spec.numericColumn, spec.legacyColumn, spec.legacyColumn)
	if err := database.SelectContext(ctx, &keys, query, cursor, batchSize); err != nil {
		return nil, fmt.Errorf("select numeric child %s batch: %w", spec.entity, err)
	}
	return keys, nil
}

func loadNumericKeyChildProgress(ctx context.Context, database *sqlx.DB, entity string) (numericKeyChildProgress, error) {
	progress := numericKeyChildProgress{}
	err := database.GetContext(ctx, &progress, `
		SELECT cursor_text, cursor_text_2, processed_rows, completed
		FROM numeric_key_child_backfill_progress WHERE entity_name = ?
	`, entity)
	if errors.Is(err, sql.ErrNoRows) {
		return progress, nil
	}
	return progress, err
}

func saveNumericKeyChildProgress(
	ctx context.Context,
	store numericKeyProgressStore,
	entity, cursor string,
	processed int64,
	completed bool,
) error {
	_, err := store.ExecContext(ctx, `
		INSERT INTO numeric_key_child_backfill_progress
			(entity_name, cursor_text, cursor_text_2, processed_rows, completed)
		VALUES (?, ?, '', ?, ?)
		ON DUPLICATE KEY UPDATE
			cursor_text = VALUES(cursor_text), cursor_text_2 = VALUES(cursor_text_2),
			processed_rows = VALUES(processed_rows), completed = VALUES(completed),
			updated_at = CURRENT_TIMESTAMP(6)
	`, entity, cursor, processed, completed)
	return err
}

func validateNumericKeyChildrenBackfill(ctx context.Context, database *sqlx.DB) (bool, error) {
	checks := []string{
		`SELECT EXISTS(SELECT 1 FROM masteries source LEFT JOIN summoner_numeric_keys numeric_key ON numeric_key.puuid = source.puuid WHERE source.summoner_fk IS NULL OR numeric_key.summoner_id IS NULL OR source.summoner_fk <> numeric_key.summoner_id LIMIT 1)`,
		`SELECT EXISTS(SELECT 1 FROM leagues source LEFT JOIN summoner_numeric_keys numeric_key ON numeric_key.puuid = source.puuid WHERE source.summoner_fk IS NULL OR numeric_key.summoner_id IS NULL OR source.summoner_fk <> numeric_key.summoner_id LIMIT 1)`,
		`SELECT EXISTS(SELECT 1 FROM summoner_matches source LEFT JOIN summoner_numeric_keys summoner_key ON summoner_key.puuid = source.puuid LEFT JOIN match_numeric_keys match_key ON match_key.riot_match_id = source.match_id WHERE source.summoner_fk IS NULL OR source.match_fk IS NULL OR summoner_key.summoner_id IS NULL OR match_key.match_id IS NULL OR source.summoner_fk <> summoner_key.summoner_id OR source.match_fk <> match_key.match_id LIMIT 1)`,
		`SELECT EXISTS(SELECT 1 FROM match_participant_details source LEFT JOIN match_participant_numeric_keys participant_key ON participant_key.legacy_match_participant_id = source.match_participant_id LEFT JOIN match_numeric_keys match_key ON match_key.riot_match_id = source.match_id WHERE source.match_participant_fk IS NULL OR source.match_fk IS NULL OR participant_key.match_participant_id IS NULL OR match_key.match_id IS NULL OR source.match_participant_fk <> participant_key.match_participant_id OR source.match_fk <> match_key.match_id LIMIT 1)`,
		`SELECT EXISTS(SELECT 1 FROM match_participant_perks source LEFT JOIN match_participant_numeric_keys numeric_key ON numeric_key.legacy_match_participant_id = source.match_participant_id WHERE source.match_participant_fk IS NULL OR numeric_key.match_participant_id IS NULL OR source.match_participant_fk <> numeric_key.match_participant_id LIMIT 1)`,
		`SELECT EXISTS(SELECT 1 FROM match_participant_perk_styles source LEFT JOIN match_participant_numeric_keys numeric_key ON numeric_key.legacy_match_participant_id = source.match_participant_id WHERE source.match_participant_fk IS NULL OR numeric_key.match_participant_id IS NULL OR source.match_participant_fk <> numeric_key.match_participant_id LIMIT 1)`,
		`SELECT EXISTS(SELECT 1 FROM match_teams source LEFT JOIN match_numeric_keys numeric_key ON numeric_key.riot_match_id = source.match_id WHERE source.match_fk IS NULL OR numeric_key.match_id IS NULL OR source.match_fk <> numeric_key.match_id LIMIT 1)`,
		`SELECT EXISTS(SELECT 1 FROM match_team_bans source LEFT JOIN match_numeric_keys numeric_key ON numeric_key.riot_match_id = source.match_id WHERE source.match_fk IS NULL OR numeric_key.match_id IS NULL OR source.match_fk <> numeric_key.match_id LIMIT 1)`,
	}
	for _, query := range checks {
		var mismatch bool
		if err := database.GetContext(ctx, &mismatch, query); err != nil {
			return false, err
		}
		if mismatch {
			return false, nil
		}
	}
	return true, nil
}
