package migrations

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

const numericKeyChildrenMigrationVersion = "20260831_007"

type numericKeyChildColumn struct {
	table      string
	column     string
	definition string
}

var numericKeyChildColumns = []numericKeyChildColumn{
	{"masteries", "summoner_fk", "BIGINT UNSIGNED NULL"},
	{"leagues", "summoner_fk", "BIGINT UNSIGNED NULL"},
	{"summoner_matches", "summoner_fk", "BIGINT UNSIGNED NULL"},
	{"summoner_matches", "match_fk", "BIGINT UNSIGNED NULL"},
	{"match_participant_details", "match_participant_fk", "BIGINT UNSIGNED NULL"},
	{"match_participant_details", "match_fk", "BIGINT UNSIGNED NULL"},
	{"match_participant_perks", "match_participant_fk", "BIGINT UNSIGNED NULL"},
	{"match_participant_perk_styles", "match_participant_fk", "BIGINT UNSIGNED NULL"},
	{"match_teams", "match_fk", "BIGINT UNSIGNED NULL"},
	{"match_team_bans", "match_fk", "BIGINT UNSIGNED NULL"},
}

func applyNumericKeyChildren(ctx context.Context, database *sqlx.DB) error {
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS numeric_key_child_backfill_progress (
			entity_name VARCHAR(64) NOT NULL,
			cursor_text VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
			cursor_text_2 VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
			processed_rows BIGINT UNSIGNED NOT NULL DEFAULT 0,
			completed TINYINT(1) NOT NULL DEFAULT 0,
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (entity_name)
		) ENGINE=InnoDB
	`); err != nil {
		return fmt.Errorf("create numeric key child progress: %w", err)
	}

	for _, column := range numericKeyChildColumns {
		if err := ensureNumericKeyColumn(ctx, database, column.table, column.column, column.definition); err != nil {
			return err
		}
	}
	for _, trigger := range numericKeyChildTriggers() {
		if _, err := database.ExecContext(ctx, "DROP TRIGGER IF EXISTS `"+trigger.name+"`"); err != nil {
			return fmt.Errorf("drop numeric key child trigger %s: %w", trigger.name, err)
		}
		if _, err := database.ExecContext(ctx, trigger.statement); err != nil {
			return fmt.Errorf("create numeric key child trigger %s: %w", trigger.name, err)
		}
	}
	return nil
}

func numericKeyChildTriggers() []numericKeyTrigger {
	return []numericKeyTrigger{
		summonerChildTrigger("masteries", "masteries_numeric_key_bi", "INSERT", "TRUE"),
		summonerChildTrigger("masteries", "masteries_numeric_key_bu", "UPDATE", "NEW.summoner_fk IS NULL OR NOT (NEW.puuid <=> OLD.puuid)"),
		summonerChildTrigger("leagues", "leagues_numeric_key_bi", "INSERT", "TRUE"),
		summonerChildTrigger("leagues", "leagues_numeric_key_bu", "UPDATE", "NEW.summoner_fk IS NULL OR NOT (NEW.puuid <=> OLD.puuid)"),
		summonerMatchChildTrigger("summoner_matches_numeric_key_bi", "INSERT", "TRUE"),
		summonerMatchChildTrigger("summoner_matches_numeric_key_bu", "UPDATE", "NEW.summoner_fk IS NULL OR NEW.match_fk IS NULL OR NOT (NEW.puuid <=> OLD.puuid) OR NOT (NEW.match_id <=> OLD.match_id)"),
		participantDetailChildTrigger("match_participant_details_numeric_key_bi", "INSERT", "TRUE"),
		participantDetailChildTrigger("match_participant_details_numeric_key_bu", "UPDATE", "NEW.match_participant_fk IS NULL OR NEW.match_fk IS NULL OR NOT (NEW.match_participant_id <=> OLD.match_participant_id) OR NOT (NEW.match_id <=> OLD.match_id)"),
		participantChildTrigger("match_participant_perks", "match_participant_perks_numeric_key_bi", "INSERT", "TRUE"),
		participantChildTrigger("match_participant_perks", "match_participant_perks_numeric_key_bu", "UPDATE", "NEW.match_participant_fk IS NULL OR NOT (NEW.match_participant_id <=> OLD.match_participant_id)"),
		participantChildTrigger("match_participant_perk_styles", "match_participant_perk_styles_numeric_key_bi", "INSERT", "TRUE"),
		participantChildTrigger("match_participant_perk_styles", "match_participant_perk_styles_numeric_key_bu", "UPDATE", "NEW.match_participant_fk IS NULL OR NOT (NEW.match_participant_id <=> OLD.match_participant_id)"),
		matchChildTrigger("match_teams", "match_teams_numeric_key_bi", "INSERT", "TRUE"),
		matchChildTrigger("match_teams", "match_teams_numeric_key_bu", "UPDATE", "NEW.match_fk IS NULL OR NOT (NEW.match_id <=> OLD.match_id)"),
		matchChildTrigger("match_team_bans", "match_team_bans_numeric_key_bi", "INSERT", "TRUE"),
		matchChildTrigger("match_team_bans", "match_team_bans_numeric_key_bu", "UPDATE", "NEW.match_fk IS NULL OR NOT (NEW.match_id <=> OLD.match_id)"),
	}
}

func summonerChildTrigger(table, name, event, condition string) numericKeyTrigger {
	return numericKeyTrigger{name: name, statement: fmt.Sprintf(`CREATE TRIGGER %s BEFORE %s ON %s
		FOR EACH ROW
		BEGIN
			IF %s THEN
				INSERT INTO summoner_numeric_keys (puuid) VALUES (NEW.puuid)
				ON DUPLICATE KEY UPDATE summoner_id = LAST_INSERT_ID(summoner_id);
				SET NEW.summoner_fk = LAST_INSERT_ID();
			END IF;
		END`, name, event, table, condition)}
}

func summonerMatchChildTrigger(name, event, condition string) numericKeyTrigger {
	return numericKeyTrigger{name: name, statement: fmt.Sprintf(`CREATE TRIGGER %s BEFORE %s ON summoner_matches
		FOR EACH ROW
		BEGIN
			IF %s THEN
				INSERT INTO summoner_numeric_keys (puuid) VALUES (NEW.puuid)
				ON DUPLICATE KEY UPDATE summoner_id = LAST_INSERT_ID(summoner_id);
				SET NEW.summoner_fk = LAST_INSERT_ID();
				INSERT INTO match_numeric_keys (riot_match_id) VALUES (NEW.match_id)
				ON DUPLICATE KEY UPDATE match_id = LAST_INSERT_ID(match_id);
				SET NEW.match_fk = LAST_INSERT_ID();
			END IF;
		END`, name, event, condition)}
}

func participantChildTrigger(table, name, event, condition string) numericKeyTrigger {
	return numericKeyTrigger{name: name, statement: fmt.Sprintf(`CREATE TRIGGER %s BEFORE %s ON %s
		FOR EACH ROW
		BEGIN
			IF %s THEN
				INSERT INTO match_participant_numeric_keys (legacy_match_participant_id)
				VALUES (NEW.match_participant_id)
				ON DUPLICATE KEY UPDATE match_participant_id = LAST_INSERT_ID(match_participant_id);
				SET NEW.match_participant_fk = LAST_INSERT_ID();
			END IF;
		END`, name, event, table, condition)}
}

func participantDetailChildTrigger(name, event, condition string) numericKeyTrigger {
	return numericKeyTrigger{name: name, statement: fmt.Sprintf(`CREATE TRIGGER %s BEFORE %s ON match_participant_details
		FOR EACH ROW
		BEGIN
			IF %s THEN
				INSERT INTO match_participant_numeric_keys (legacy_match_participant_id)
				VALUES (NEW.match_participant_id)
				ON DUPLICATE KEY UPDATE match_participant_id = LAST_INSERT_ID(match_participant_id);
				SET NEW.match_participant_fk = LAST_INSERT_ID();
				INSERT INTO match_numeric_keys (riot_match_id) VALUES (NEW.match_id)
				ON DUPLICATE KEY UPDATE match_id = LAST_INSERT_ID(match_id);
				SET NEW.match_fk = LAST_INSERT_ID();
			END IF;
		END`, name, event, condition)}
}

func matchChildTrigger(table, name, event, condition string) numericKeyTrigger {
	return numericKeyTrigger{name: name, statement: fmt.Sprintf(`CREATE TRIGGER %s BEFORE %s ON %s
		FOR EACH ROW
		BEGIN
			IF %s THEN
				INSERT INTO match_numeric_keys (riot_match_id) VALUES (NEW.match_id)
				ON DUPLICATE KEY UPDATE match_id = LAST_INSERT_ID(match_id);
				SET NEW.match_fk = LAST_INSERT_ID();
			END IF;
		END`, name, event, table, condition)}
}

func validateNumericKeyChildren(ctx context.Context, database *sqlx.DB) (bool, error) {
	legacyMasteriesExist, err := tableExists(ctx, database, "masteries")
	if err != nil {
		return false, err
	}
	columns := map[string][]string{
		"numeric_key_child_backfill_progress": {"entity_name", "cursor_text", "cursor_text_2", "processed_rows", "completed"},
	}
	for _, column := range numericKeyChildColumns {
		if column.table == "masteries" && !legacyMasteriesExist {
			continue
		}
		columns[column.table] = append(columns[column.table], column.column)
	}
	valid, err := columnsExist(ctx, database, columns)
	if err != nil || !valid {
		return false, err
	}
	valid, err = indexMatches(ctx, database, "numeric_key_child_backfill_progress", "PRIMARY", "entity_name")
	if err != nil || !valid {
		return false, err
	}
	for _, trigger := range numericKeyChildTriggers() {
		if !legacyMasteriesExist && (trigger.name == "masteries_numeric_key_bi" || trigger.name == "masteries_numeric_key_bu") {
			continue
		}
		var count int
		if err := database.GetContext(ctx, &count, `
			SELECT COUNT(*) FROM information_schema.triggers
			WHERE trigger_schema = DATABASE() AND trigger_name = ?
		`, trigger.name); err != nil || count != 1 {
			return false, err
		}
	}
	return true, nil
}
