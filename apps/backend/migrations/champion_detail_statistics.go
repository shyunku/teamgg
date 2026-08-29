package migrations

import (
	"context"

	"github.com/jmoiron/sqlx"
)

func applyChampionDetailStatisticsSource(ctx context.Context, database *sqlx.DB) error {
	_, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS champion_detail_statistics_source (
			source_id BIGINT NOT NULL AUTO_INCREMENT,
			match_id VARCHAR(255) NOT NULL,
			match_participant_id VARCHAR(255) NOT NULL,
			champion_id INT NOT NULL,
			champion_name VARCHAR(255) NOT NULL,
			team_position VARCHAR(32) NOT NULL,
			kills INT NOT NULL,
			deaths INT NOT NULL,
			assists INT NOT NULL,
			team_id INT NOT NULL,
			summoner1_id INT NOT NULL,
			summoner2_id INT NOT NULL,
			win TINYINT(1) NOT NULL,
			enemy_champion_id INT NULL,
			enemy_champion_name VARCHAR(255) NULL,
			enemy_kills INT NULL,
			enemy_deaths INT NULL,
			enemy_assists INT NULL,
			enemy_win TINYINT(1) NULL,
			primary_style INT NOT NULL,
			primary_perk0 INT NOT NULL,
			primary_perk1 INT NOT NULL,
			primary_perk2 INT NOT NULL,
			primary_perk3 INT NOT NULL,
			sub_style INT NOT NULL,
			sub_perk0 INT NOT NULL,
			sub_perk1 INT NOT NULL,
			stat_perk_defense INT NOT NULL,
			stat_perk_flex INT NOT NULL,
			stat_perk_offense INT NOT NULL,
			item0_id INT NULL,
			item0_name VARCHAR(255) NULL,
			item1_id INT NULL,
			item1_name VARCHAR(255) NULL,
			item2_id INT NULL,
			item2_name VARCHAR(255) NULL,
			item3_id INT NULL,
			item3_name VARCHAR(255) NULL,
			item4_id INT NULL,
			item4_name VARCHAR(255) NULL,
			item5_id INT NULL,
			item5_name VARCHAR(255) NULL,
			PRIMARY KEY (source_id),
			KEY champion_detail_source_participant_index (match_participant_id),
			KEY champion_detail_source_meta_index
				(champion_id, team_position, primary_style, sub_style),
			KEY champion_detail_source_counter_index
				(champion_id, enemy_champion_id, team_position)
		) ENGINE=InnoDB
	`)
	return err
}

func validateChampionDetailStatisticsSource(ctx context.Context, database *sqlx.DB) (bool, error) {
	columns, err := columnsExist(ctx, database, map[string][]string{
		"champion_detail_statistics_source": {
			"source_id",
			"match_id",
			"match_participant_id",
			"champion_id",
			"team_position",
			"enemy_champion_id",
			"primary_style",
			"item0_id",
		},
	})
	if err != nil || !columns {
		return false, err
	}
	indexes := []struct {
		name    string
		columns []string
	}{
		{"PRIMARY", []string{"source_id"}},
		{"champion_detail_source_participant_index", []string{"match_participant_id"}},
		{"champion_detail_source_meta_index", []string{"champion_id", "team_position", "primary_style", "sub_style"}},
		{"champion_detail_source_counter_index", []string{"champion_id", "enemy_champion_id", "team_position"}},
	}
	for _, index := range indexes {
		valid, err := indexMatches(ctx, database, "champion_detail_statistics_source", index.name, index.columns...)
		if err != nil || !valid {
			return false, err
		}
	}
	return true, nil
}
