package migrations

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

const dataExplorerClaimIndexesMigrationVersion = "20260831_008"

type dataExplorerClaimIndex struct {
	table     string
	name      string
	idColumn  string
	columns   []string
	direction []string
}

func dataExplorerClaimIndexes() []dataExplorerClaimIndex {
	return []dataExplorerClaimIndex{
		{
			table: "data_explorer_summoner_jobs", name: "data_explorer_summoner_jobs_claim_v2_index", idColumn: "puuid",
			columns: []string{"status", "next_attempt_at", "priority", "created_at", "puuid"}, direction: []string{"A", "A", "D", "A", "A"},
		},
		{
			table: "data_explorer_match_jobs", name: "data_explorer_match_jobs_claim_v2_index", idColumn: "match_id",
			columns: []string{"status", "next_attempt_at", "priority", "created_at", "match_id"}, direction: []string{"A", "A", "D", "A", "A"},
		},
	}
}

func applyDataExplorerClaimIndexes(ctx context.Context, database *sqlx.DB) error {
	for _, index := range dataExplorerClaimIndexes() {
		valid, err := dataExplorerClaimIndexMatches(ctx, database, index)
		if err != nil {
			return err
		}
		if valid {
			continue
		}
		columns, err := indexColumns(ctx, database, index.table, index.name)
		if err != nil {
			return err
		}
		if len(columns) != 0 {
			if _, err := database.ExecContext(ctx, fmt.Sprintf(
				"ALTER TABLE `%s` DROP INDEX `%s`, ALGORITHM=INPLACE, LOCK=NONE",
				index.table, index.name,
			)); err != nil {
				return fmt.Errorf("drop invalid DataExplorer claim index %s: %w", index.name, err)
			}
		}
		statement := fmt.Sprintf(
			"ALTER TABLE `%s` ADD INDEX `%s` (`status`, `next_attempt_at`, `priority` DESC, `created_at`, `%s`), ALGORITHM=INPLACE, LOCK=NONE",
			index.table, index.name, index.idColumn,
		)
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("add DataExplorer claim index %s: %w", index.name, err)
		}
	}
	return nil
}

func validateDataExplorerClaimIndexes(ctx context.Context, database *sqlx.DB) (bool, error) {
	for _, index := range dataExplorerClaimIndexes() {
		valid, err := dataExplorerClaimIndexMatches(ctx, database, index)
		if err != nil || !valid {
			return false, err
		}
	}
	return true, nil
}

func dataExplorerClaimIndexMatches(ctx context.Context, database *sqlx.DB, expected dataExplorerClaimIndex) (bool, error) {
	type indexPart struct {
		Column    string `db:"COLUMN_NAME"`
		Direction string `db:"COLLATION"`
	}
	parts := make([]indexPart, 0, len(expected.columns))
	if err := database.SelectContext(ctx, &parts, `
		SELECT column_name, collation
		FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?
		ORDER BY seq_in_index
	`, expected.table, expected.name); err != nil {
		return false, err
	}
	if len(parts) != len(expected.columns) {
		return false, nil
	}
	for index, part := range parts {
		if part.Column != expected.columns[index] || part.Direction != expected.direction[index] {
			return false, nil
		}
	}
	return true, nil
}
