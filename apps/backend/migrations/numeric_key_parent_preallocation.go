package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

const numericKeyParentPreallocationMigrationVersion = "20260831_006"

func applyNumericKeyParentPreallocation(ctx context.Context, database *sqlx.DB) error {
	trigger := numericKeyParticipantInsertTrigger()
	if _, err := database.ExecContext(ctx, "DROP TRIGGER IF EXISTS `"+trigger.name+"`"); err != nil {
		return fmt.Errorf("drop numeric key trigger %s: %w", trigger.name, err)
	}
	if _, err := database.ExecContext(ctx, trigger.statement); err != nil {
		return fmt.Errorf("create numeric key trigger %s: %w", trigger.name, err)
	}
	return nil
}

func validateNumericKeyParentPreallocation(ctx context.Context, database *sqlx.DB) (bool, error) {
	var statement string
	if err := database.GetContext(ctx, &statement, `
		SELECT action_statement
		FROM information_schema.triggers
		WHERE trigger_schema = DATABASE()
		  AND trigger_name = 'match_participants_numeric_key_bi'
	`); err != nil {
		return false, err
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(statement), " "))
	required := []string{
		"insert into match_numeric_keys (riot_match_id) values (new.match_id)",
		"insert into summoner_numeric_keys (puuid) values (new.puuid)",
		"set new.match_fk = last_insert_id()",
		"set new.summoner_fk = last_insert_id()",
	}
	for _, fragment := range required {
		if !strings.Contains(normalized, fragment) {
			return false, nil
		}
	}
	return !strings.Contains(normalized, "signal sqlstate"), nil
}
