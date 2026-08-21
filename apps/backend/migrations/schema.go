package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

func validateBaseSchema(ctx context.Context, database *sqlx.DB) error {
	required := []string{
		"users",
		"summoners",
		"matches",
		"summoner_matches",
		"custom_game_configurations",
		"custom_game_candidates",
		"match_team_bans",
	}
	missing := make([]string, 0)
	for _, table := range required {
		exists, err := tableExists(ctx, database, table)
		if err != nil {
			return err
		}
		if !exists {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"base schema is incomplete; import the team.gg base schema before migrations (missing: %s)",
			strings.Join(missing, ", "),
		)
	}
	return nil
}

func tableExists(ctx context.Context, database *sqlx.DB, table string) (bool, error) {
	var count int
	err := database.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?
	`, table)
	return count > 0, err
}

func columnExists(ctx context.Context, database *sqlx.DB, table, column string) (bool, error) {
	var count int
	err := database.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?
	`, table, column)
	return count > 0, err
}

func columnsExist(ctx context.Context, database *sqlx.DB, required map[string][]string) (bool, error) {
	for table, columns := range required {
		for _, column := range columns {
			exists, err := columnExists(ctx, database, table, column)
			if err != nil || !exists {
				return false, err
			}
		}
	}
	return true, nil
}

func indexColumns(ctx context.Context, database *sqlx.DB, table, index string) ([]string, error) {
	columns := make([]string, 0)
	err := database.SelectContext(ctx, &columns, `
		SELECT column_name FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?
		ORDER BY seq_in_index
	`, table, index)
	return columns, err
}

func indexMatches(ctx context.Context, database *sqlx.DB, table, index string, expected ...string) (bool, error) {
	actual, err := indexColumns(ctx, database, table, index)
	if err != nil || len(actual) != len(expected) {
		return false, err
	}
	for index := range actual {
		if !strings.EqualFold(actual[index], expected[index]) {
			return false, nil
		}
	}
	return true, nil
}

func ensureIndex(
	ctx context.Context,
	database *sqlx.DB,
	table string,
	index string,
	definition string,
	expected ...string,
) error {
	valid, err := indexMatches(ctx, database, table, index, expected...)
	if err != nil {
		return err
	}
	if valid {
		return nil
	}
	existing, err := indexColumns(ctx, database, table, index)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("index %s.%s exists with columns %v, expected %v", table, index, existing, expected)
	}
	_, err = database.ExecContext(ctx, fmt.Sprintf("CREATE INDEX %s ON %s (%s)", index, table, definition))
	return err
}

func tablesExist(ctx context.Context, database *sqlx.DB, tables ...string) (bool, error) {
	for _, table := range tables {
		exists, err := tableExists(ctx, database, table)
		if err != nil || !exists {
			return false, err
		}
	}
	return true, nil
}
