package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

type removableIndex struct {
	table   string
	name    string
	columns []string
}

var task63RemovableIndexes = []removableIndex{
	{
		table:   "match_participant_perk_styles",
		name:    "match_participant_perk_styles_description_index",
		columns: []string{"description"},
	},
	{
		table:   "masteries",
		name:    "masteries_champion_id_champion_points_index",
		columns: []string{"champion_id", "champion_points"},
	},
	{
		table:   "match_participants",
		name:    "match_participants_participant_id_index",
		columns: []string{"participant_id"},
	},
	{
		table:   "match_participants",
		name:    "match_participants_team_position_index",
		columns: []string{"team_position"},
	},
}

func applyUnusedIndexCleanup(ctx context.Context, database *sqlx.DB) error {
	for _, candidate := range task63RemovableIndexes {
		actual, err := indexColumns(ctx, database, candidate.table, candidate.name)
		if err != nil {
			return err
		}
		if len(actual) == 0 {
			continue
		}
		if !indexColumnNamesMatch(actual, candidate.columns) {
			return fmt.Errorf(
				"refusing to remove index %s.%s with columns %v; expected %v",
				candidate.table,
				candidate.name,
				actual,
				candidate.columns,
			)
		}
		if _, err := database.ExecContext(ctx, unusedIndexDropStatement(candidate)); err != nil {
			return fmt.Errorf("remove index %s.%s: %w", candidate.table, candidate.name, err)
		}
	}
	return nil
}

func validateUnusedIndexCleanup(ctx context.Context, database *sqlx.DB) (bool, error) {
	for _, candidate := range task63RemovableIndexes {
		actual, err := indexColumns(ctx, database, candidate.table, candidate.name)
		if err != nil {
			return false, err
		}
		if len(actual) > 0 {
			return false, nil
		}
	}
	return true, nil
}

func indexColumnNamesMatch(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if !strings.EqualFold(actual[index], expected[index]) {
			return false
		}
	}
	return true
}

func unusedIndexDropStatement(candidate removableIndex) string {
	return fmt.Sprintf(
		"ALTER TABLE `%s` DROP INDEX `%s`, ALGORITHM=INPLACE, LOCK=NONE",
		candidate.table,
		candidate.name,
	)
}
