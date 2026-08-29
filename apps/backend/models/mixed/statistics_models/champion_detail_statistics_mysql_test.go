package statistics_models

import (
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func TestChampionDetailStatisticsQueriesAgainstMySQL8(t *testing.T) {
	dsn := os.Getenv("TEAMGG_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("TEAMGG_MYSQL_TEST_DSN is not set")
	}
	database, err := sqlx.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	statements := []string{
		`CREATE TABLE matches (
			match_id VARCHAR(255) PRIMARY KEY,
			game_version VARCHAR(255) NOT NULL,
			KEY matches_game_version_index (game_version)
		)`,
		`CREATE TABLE match_participants (
			match_id VARCHAR(255) NOT NULL,
			match_participant_id VARCHAR(255) NOT NULL,
			champion_id INT NOT NULL,
			champion_name VARCHAR(255) NOT NULL,
			team_position VARCHAR(32) NOT NULL,
			kills INT NOT NULL, deaths INT NOT NULL, assists INT NOT NULL,
			team_id INT NOT NULL,
			summoner1_id INT NOT NULL, summoner2_id INT NOT NULL,
			item0 INT NOT NULL, item1 INT NOT NULL, item2 INT NOT NULL,
			item3 INT NOT NULL, item4 INT NOT NULL, item5 INT NOT NULL, item6 INT NOT NULL,
			win TINYINT(1) NOT NULL,
			PRIMARY KEY (match_id, match_participant_id),
			KEY match_participants_participant_index (match_participant_id)
		)`,
		`CREATE TABLE match_participant_perk_styles (
			match_participant_id VARCHAR(255) NOT NULL,
			style_id VARCHAR(255) PRIMARY KEY,
			description VARCHAR(255) NOT NULL,
			style INT NOT NULL,
			KEY match_participant_perk_styles_participant_index (match_participant_id)
		)`,
		`CREATE TABLE match_participant_perk_style_selections (
			style_id VARCHAR(255) NOT NULL,
			perk INT NOT NULL,
			KEY match_participant_perk_selections_style_index (style_id)
		)`,
		`CREATE TABLE match_participant_perks (
			match_participant_id VARCHAR(255) PRIMARY KEY,
			stat_perk_defense INT NOT NULL,
			stat_perk_flex INT NOT NULL,
			stat_perk_offense INT NOT NULL
		)`,
		`CREATE TABLE static_items (
			id INT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			gold_total INT NOT NULL,
			depth INT NOT NULL,
			required_ally VARCHAR(255) NULL,
			gold_purchasable TINYINT(1) NOT NULL
		)`,
		`CREATE TABLE champion_detail_statistics_source (
			source_id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			match_id VARCHAR(255) NOT NULL,
			match_participant_id VARCHAR(255) NOT NULL,
			champion_id INT NOT NULL,
			champion_name VARCHAR(255) NOT NULL,
			team_position VARCHAR(32) NOT NULL,
			kills INT NOT NULL, deaths INT NOT NULL, assists INT NOT NULL,
			team_id INT NOT NULL,
			summoner1_id INT NOT NULL, summoner2_id INT NOT NULL,
			win TINYINT(1) NOT NULL,
			enemy_champion_id INT NULL,
			enemy_champion_name VARCHAR(255) NULL,
			enemy_kills INT NULL, enemy_deaths INT NULL, enemy_assists INT NULL,
			enemy_win TINYINT(1) NULL,
			primary_style INT NOT NULL,
			primary_perk0 INT NOT NULL, primary_perk1 INT NOT NULL,
			primary_perk2 INT NOT NULL, primary_perk3 INT NOT NULL,
			sub_style INT NOT NULL, sub_perk0 INT NOT NULL, sub_perk1 INT NOT NULL,
			stat_perk_defense INT NOT NULL, stat_perk_flex INT NOT NULL, stat_perk_offense INT NOT NULL,
			item0_id INT NULL, item0_name VARCHAR(255) NULL,
			item1_id INT NULL, item1_name VARCHAR(255) NULL,
			item2_id INT NULL, item2_name VARCHAR(255) NULL,
			item3_id INT NULL, item3_name VARCHAR(255) NULL,
			item4_id INT NULL, item4_name VARCHAR(255) NULL,
			item5_id INT NULL, item5_name VARCHAR(255) NULL,
			KEY champion_detail_source_meta_index (champion_id, team_position, primary_style, sub_style),
			KEY champion_detail_source_counter_index (champion_id, enemy_champion_id, team_position)
		)`,
		`INSERT INTO matches VALUES ('KR-test', '16.16.1'), ('KR-old', '16.13.1')`,
		`INSERT INTO match_participants VALUES
			('KR-test', 'p1', 1, 'Annie', 'MIDDLE', 5, 2, 4, 100, 4, 14, 1001, 1002, 1003, 0, 0, 0, 0, 1),
			('KR-test', 'p2', 2, 'Olaf', 'MIDDLE', 2, 5, 1, 200, 4, 12, 1001, 1002, 1003, 0, 0, 0, 0, 0),
			('KR-old', 'old', 3, 'Galio', 'MIDDLE', 1, 1, 1, 100, 4, 14, 1001, 1002, 1003, 0, 0, 0, 0, 1)`,
		`INSERT INTO match_participant_perk_styles VALUES
			('p1', 'p1-primary', 'primaryStyle', 8000), ('p1', 'p1-sub', 'subStyle', 8100),
			('p2', 'p2-primary', 'primaryStyle', 8000), ('p2', 'p2-sub', 'subStyle', 8100),
			('old', 'old-primary', 'primaryStyle', 8000), ('old', 'old-sub', 'subStyle', 8100)`,
		`INSERT INTO match_participant_perk_style_selections VALUES
			('p1-primary', 8001), ('p1-primary', 8002), ('p1-primary', 8003), ('p1-primary', 8004),
			('p1-sub', 8101), ('p1-sub', 8102),
			('p2-primary', 8001), ('p2-primary', 8002), ('p2-primary', 8003), ('p2-primary', 8004),
			('p2-sub', 8101), ('p2-sub', 8102),
			('old-primary', 8001), ('old-primary', 8002), ('old-primary', 8003), ('old-primary', 8004),
			('old-sub', 8101), ('old-sub', 8102)`,
		`INSERT INTO match_participant_perks VALUES
			('p1', 5001, 5002, 5003), ('p2', 5001, 5002, 5003), ('old', 5001, 5002, 5003)`,
		`INSERT INTO static_items VALUES
			(1001, 'Item 1', 3000, 3, NULL, 1),
			(1002, 'Item 2', 3100, 3, NULL, 1),
			(1003, 'Item 3', 3200, 3, NULL, 1)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("fixture statement failed: %v\n%s", err, statement)
		}
	}

	if err := PrepareChampionDetailStatisticsSource(database, []string{"16.16.1"}); err != nil {
		t.Fatal(err)
	}
	var sourceCount int
	if err := database.Get(&sourceCount, "SELECT COUNT(*) FROM champion_detail_statistics_source"); err != nil {
		t.Fatal(err)
	}
	if sourceCount != 2 {
		t.Fatalf("recent patch filter produced %d rows, want 2", sourceCount)
	}
	metas, err := GetChampionDetailStatisticsMetaMXDAOs(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("meta query produced %d rows, want 2", len(metas))
	}
	for _, meta := range metas {
		if meta.Total != 1 || meta.Item0Id != 1003 || meta.Item1Id != 1002 || meta.Item2Id != 1001 {
			t.Fatalf("unexpected meta row: %+v", meta)
		}
	}
	counters, err := GetChampionCounterStatisticsMXDAOs(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(counters) != 2 {
		t.Fatalf("counter query produced %d rows, want 2", len(counters))
	}
	for _, counter := range counters {
		if counter.Total != 1 || counter.EnemyChampionId == 0 {
			t.Fatalf("unexpected counter row: %+v", counter)
		}
	}
}
