package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

const (
	masteryStatisticsCoveringIndex = "masteries_champion_points_level_covering_index"
	masteryStatisticsInsertTrigger = "teamgg_masteries_statistics_insert"
	masteryStatisticsUpdateTrigger = "teamgg_masteries_statistics_update"
	masteryStatisticsDeleteTrigger = "teamgg_masteries_statistics_delete"
)

type masteryStatisticsTrigger struct {
	name      string
	timing    string
	event     string
	statement string
}

func applyMasteryStatisticsAggregates(ctx context.Context, database *sqlx.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS mastery_statistics_aggregates (
			champion_id BIGINT NOT NULL,
			max_mastery BIGINT NOT NULL,
			total_mastery BIGINT NOT NULL,
			mastered_count BIGINT NOT NULL,
			summoner_count BIGINT NOT NULL,
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (champion_id),
			KEY mastery_statistics_aggregates_updated_index (updated_at)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS mastery_statistics_dirty_champions (
			champion_id BIGINT NOT NULL,
			dirty_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			PRIMARY KEY (champion_id),
			KEY mastery_statistics_dirty_at_index (dirty_at, champion_id)
		) ENGINE=InnoDB`,
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := ensureMasteryStatisticsCoveringIndex(ctx, database); err != nil {
		return fmt.Errorf("ensure mastery statistics covering index: %w", err)
	}

	for _, trigger := range masteryStatisticsTriggers("masteries") {
		if err := ensureMasteryStatisticsTrigger(ctx, database, "masteries", trigger.name, trigger.timing, trigger.event, trigger.statement); err != nil {
			return err
		}
	}
	_, err := database.ExecContext(ctx, `
		INSERT INTO mastery_statistics_dirty_champions (champion_id, dirty_at)
		SELECT DISTINCT champion_id, NOW(6)
		FROM masteries
		ON DUPLICATE KEY UPDATE dirty_at = LEAST(dirty_at, VALUES(dirty_at))
	`)
	return err
}

func masteryStatisticsTriggers(table string) []masteryStatisticsTrigger {
	return []masteryStatisticsTrigger{
		{
			masteryStatisticsInsertTrigger, "AFTER", "INSERT",
			fmt.Sprintf(`CREATE TRIGGER %s AFTER INSERT ON %s FOR EACH ROW
				INSERT INTO mastery_statistics_dirty_champions (champion_id, dirty_at)
				VALUES (NEW.champion_id, NOW(6))
				ON DUPLICATE KEY UPDATE dirty_at = VALUES(dirty_at)`, masteryStatisticsInsertTrigger, table),
		},
		{
			masteryStatisticsUpdateTrigger, "AFTER", "UPDATE",
			fmt.Sprintf(`CREATE TRIGGER %s AFTER UPDATE ON %s FOR EACH ROW
				INSERT INTO mastery_statistics_dirty_champions (champion_id, dirty_at)
				SELECT NEW.champion_id, NOW(6) FROM DUAL
				WHERE NOT (OLD.champion_points <=> NEW.champion_points)
				   OR NOT (OLD.champion_level <=> NEW.champion_level)
				ON DUPLICATE KEY UPDATE dirty_at = VALUES(dirty_at)`, masteryStatisticsUpdateTrigger, table),
		},
		{
			masteryStatisticsDeleteTrigger, "AFTER", "DELETE",
			fmt.Sprintf(`CREATE TRIGGER %s AFTER DELETE ON %s FOR EACH ROW
				INSERT INTO mastery_statistics_dirty_champions (champion_id, dirty_at)
				VALUES (OLD.champion_id, NOW(6))
				ON DUPLICATE KEY UPDATE dirty_at = VALUES(dirty_at)`, masteryStatisticsDeleteTrigger, table),
		},
	}
}

func ensureMasteryStatisticsCoveringIndex(ctx context.Context, database *sqlx.DB) error {
	expected := []string{"champion_id", "champion_points", "champion_level"}
	valid, err := indexMatches(ctx, database, "masteries", masteryStatisticsCoveringIndex, expected...)
	if err != nil {
		return err
	}
	if valid {
		return nil
	}
	existing, err := indexColumns(ctx, database, "masteries", masteryStatisticsCoveringIndex)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf(
			"index masteries.%s exists with columns %v, expected %v",
			masteryStatisticsCoveringIndex,
			existing,
			expected,
		)
	}
	_, err = database.ExecContext(ctx, `
		ALTER TABLE masteries
		ADD INDEX masteries_champion_points_level_covering_index
			(champion_id ASC, champion_points DESC, champion_level),
		ALGORITHM=INPLACE,
		LOCK=NONE
	`)
	return err
}

func ensureMasteryStatisticsTrigger(ctx context.Context, database *sqlx.DB, table, name, timing, event, statement string) error {
	var existing struct {
		Table  string
		Timing string
		Event  string
	}
	// information_schema column labels can be returned in upper case depending
	// on the MySQL server. Scan by position so trigger validation does not rely
	// on sqlx's case-sensitive struct-field mapping.
	err := database.QueryRowxContext(ctx, `
		SELECT event_object_table, action_timing, event_manipulation
		FROM information_schema.triggers
		WHERE trigger_schema = DATABASE() AND trigger_name = ?
	`, name).Scan(&existing.Table, &existing.Timing, &existing.Event)
	if err == nil {
		if existing.Table == table && existing.Timing == timing && existing.Event == event {
			return nil
		}
		return fmt.Errorf("trigger %s exists with an unexpected definition", name)
	}
	if err != sql.ErrNoRows {
		return err
	}
	if _, err := database.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("create trigger %s: %w", name, err)
	}
	return nil
}

func validateMasteryStatisticsAggregates(ctx context.Context, database *sqlx.DB) (bool, error) {
	tables, err := tablesExist(ctx, database, "mastery_statistics_aggregates", "mastery_statistics_dirty_champions")
	if err != nil || !tables {
		return false, err
	}
	legacy, err := validateMasteryStatisticsStorage(ctx, database, "masteries", masteryStatisticsCoveringIndex)
	if err != nil || legacy {
		return legacy, err
	}
	return validateMasteryStatisticsStorage(ctx, database, "masteries_numeric_v2", "masteries_numeric_champion_points_level_covering_index")
}

func validateMasteryStatisticsStorage(ctx context.Context, database *sqlx.DB, table, indexName string) (bool, error) {
	index, err := indexMatches(ctx, database, table, indexName, "champion_id", "champion_points", "champion_level")
	if err != nil || !index {
		return false, err
	}
	for _, trigger := range masteryStatisticsTriggers(table) {
		var count int
		if err := database.GetContext(ctx, &count, `
			SELECT COUNT(*) FROM information_schema.triggers
			WHERE trigger_schema = DATABASE() AND trigger_name = ?
			  AND event_object_table = ? AND action_timing = ? AND event_manipulation = ?
		`, trigger.name, table, trigger.timing, trigger.event); err != nil {
			return false, err
		}
		if count != 1 {
			return false, nil
		}
	}
	return true, nil
}

func switchMasteryStatisticsToNumeric(ctx context.Context, database *sqlx.DB) error {
	valid, err := indexMatches(ctx, database, "masteries_numeric_v2", "masteries_numeric_champion_points_level_covering_index", "champion_id", "champion_points", "champion_level")
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("masteries_numeric_v2 covering index is not ready")
	}
	for _, trigger := range masteryStatisticsTriggers("masteries_numeric_v2") {
		if _, err := database.ExecContext(ctx, "DROP TRIGGER IF EXISTS `"+trigger.name+"`"); err != nil {
			return fmt.Errorf("drop legacy mastery statistics trigger %s: %w", trigger.name, err)
		}
		if _, err := database.ExecContext(ctx, trigger.statement); err != nil {
			return fmt.Errorf("create numeric mastery statistics trigger %s: %w", trigger.name, err)
		}
	}
	return nil
}
