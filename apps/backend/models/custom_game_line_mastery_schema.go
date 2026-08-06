package models

import "team.gg-server/libs/db"

// EnsureCustomGameLineMasterySchema keeps existing installations compatible
// with the line-mastery fields introduced after the original custom-game schema.
func EnsureCustomGameLineMasterySchema(database db.Context) error {
	columns := []struct {
		tableName  string
		columnName string
		definition string
	}{
		{"custom_game_configurations", "mastery_influence_weight", "DOUBLE NOT NULL DEFAULT 0.5"},
		{"custom_game_candidates", "mastery_top", "INT NOT NULL DEFAULT 0"},
		{"custom_game_candidates", "mastery_jungle", "INT NOT NULL DEFAULT 0"},
		{"custom_game_candidates", "mastery_mid", "INT NOT NULL DEFAULT 0"},
		{"custom_game_candidates", "mastery_adc", "INT NOT NULL DEFAULT 0"},
		{"custom_game_candidates", "mastery_support", "INT NOT NULL DEFAULT 0"},
		{"riot_custom_game_preferences", "mastery_top", "INT NOT NULL DEFAULT 0"},
		{"riot_custom_game_preferences", "mastery_jungle", "INT NOT NULL DEFAULT 0"},
		{"riot_custom_game_preferences", "mastery_mid", "INT NOT NULL DEFAULT 0"},
		{"riot_custom_game_preferences", "mastery_adc", "INT NOT NULL DEFAULT 0"},
		{"riot_custom_game_preferences", "mastery_support", "INT NOT NULL DEFAULT 0"},
	}

	for _, column := range columns {
		var count int
		if err := database.Get(&count, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?
		`, column.tableName, column.columnName); err != nil {
			return err
		}
		if count != 0 {
			continue
		}
		if _, err := database.Exec("ALTER TABLE " + column.tableName + " ADD COLUMN " + column.columnName + " " + column.definition); err != nil {
			return err
		}
	}
	return nil
}
