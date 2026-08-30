package statistics_models

import (
	"os"
	"strings"
	"testing"
	"time"

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
			game_duration BIGINT NOT NULL,
			KEY matches_game_version_index (game_version)
		)`,
		`CREATE TABLE match_participants (
			match_id VARCHAR(255) NOT NULL,
			match_participant_id VARCHAR(255) NOT NULL,
			champion_id INT NOT NULL, champion_name VARCHAR(255) NOT NULL,
			team_position VARCHAR(32) NOT NULL,
			kills INT NOT NULL, deaths INT NOT NULL, assists INT NOT NULL,
			team_id INT NOT NULL, summoner1_id INT NOT NULL, summoner2_id INT NOT NULL,
			item0 INT NOT NULL, item1 INT NOT NULL, item2 INT NOT NULL,
			item3 INT NOT NULL, item4 INT NOT NULL, item5 INT NOT NULL, item6 INT NOT NULL,
			win TINYINT(1) NOT NULL,
			total_minions_killed INT NOT NULL, gold_earned INT NOT NULL,
			total_damage_dealt_to_champions INT NOT NULL, total_damage_taken INT NOT NULL,
			total_heal INT NOT NULL, vision_score INT NOT NULL, total_time_cc_dealt INT NOT NULL,
			PRIMARY KEY (match_id, match_participant_id),
			KEY match_participants_participant_index (match_participant_id)
		)`,
		`CREATE TABLE match_participant_details (
			match_id VARCHAR(255) NOT NULL,
			match_participant_id VARCHAR(255) NOT NULL,
			damage_self_mitigated INT NOT NULL,
			damage_dealt_to_buildings INT NOT NULL,
			damage_dealt_to_objectives INT NOT NULL,
			damage_dealt_to_turrets INT NOT NULL,
			PRIMARY KEY (match_id, match_participant_id)
		)`,
		`CREATE TABLE match_team_bans (
			match_id VARCHAR(255) NOT NULL, team_id INT NOT NULL,
			champion_id INT NOT NULL, pick_turn INT NOT NULL,
			PRIMARY KEY (match_id, team_id, pick_turn)
		)`,
		`CREATE TABLE match_participant_perk_styles (
			match_participant_id VARCHAR(255) NOT NULL,
			style_id VARCHAR(255) PRIMARY KEY,
			description VARCHAR(255) NOT NULL, style INT NOT NULL,
			KEY match_participant_perk_styles_participant_index (match_participant_id)
		)`,
		`CREATE TABLE match_participant_perk_style_selections (
			style_id VARCHAR(255) NOT NULL, perk INT NOT NULL,
			KEY match_participant_perk_selections_style_index (style_id)
		)`,
		`CREATE TABLE match_participant_perks (
			match_participant_id VARCHAR(255) PRIMARY KEY,
			stat_perk_defense INT NOT NULL, stat_perk_flex INT NOT NULL, stat_perk_offense INT NOT NULL
		)`,
		`CREATE TABLE static_items (
			id INT PRIMARY KEY, name VARCHAR(255) NOT NULL,
			gold_total INT NOT NULL, depth INT NOT NULL,
			required_ally VARCHAR(255) NULL, gold_purchasable TINYINT(1) NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("fixture schema failed: %v\n%s", err, statement)
		}
	}
	migrationSQL, err := os.ReadFile("../../../migrations/20260830_create_incremental_champion_detail_statistics.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range strings.Split(string(migrationSQL), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("incremental schema failed: %v\n%s", err, statement)
		}
	}

	fixtures := []string{
		`INSERT INTO matches (match_id, game_version, game_duration)
		 VALUES ('KR-test', '16.16.1', 1800), ('KR-old', '16.13.1', 1800)`,
		`INSERT INTO match_participants (
			match_id, match_participant_id, champion_id, champion_name, team_position,
			kills, deaths, assists, team_id, summoner1_id, summoner2_id,
			item0, item1, item2, item3, item4, item5, item6, win,
			total_minions_killed, gold_earned, total_damage_dealt_to_champions,
			total_damage_taken, total_heal, vision_score, total_time_cc_dealt
		) VALUES
			('KR-test', 'p1', 1, 'Annie', 'MIDDLE', 5, 2, 4, 100, 4, 14,
			 1001, 1002, 1003, 0, 0, 0, 0, 1, 200, 12000, 20000, 15000, 1000, 25, 10),
			('KR-test', 'p2', 2, 'Olaf', 'MIDDLE', 2, 5, 1, 200, 4, 12,
			 1001, 1002, 1003, 0, 0, 0, 0, 0, 180, 10000, 15000, 20000, 500, 15, 5),
			('KR-old', 'old', 3, 'Galio', 'MIDDLE', 1, 1, 1, 100, 4, 14,
			 1001, 1002, 1003, 0, 0, 0, 0, 1, 150, 9000, 10000, 10000, 300, 10, 3)`,
		`INSERT INTO match_participant_details VALUES
			('KR-test', 'p1', 10000, 1000, 2000, 800),
			('KR-test', 'p2', 9000, 500, 1000, 400),
			('KR-old', 'old', 8000, 300, 700, 200)`,
		`INSERT INTO match_team_bans VALUES ('KR-test', 100, 1, 1), ('KR-test', 200, 2, 1)`,
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
	for _, fixture := range fixtures {
		if _, err := database.Exec(fixture); err != nil {
			t.Fatalf("fixture data failed: %v\n%s", err, fixture)
		}
	}

	options := ChampionDetailSourceOptions{BatchSize: 10, CleanupSize: 100, WorkLimit: time.Minute}
	prepared, err := PrepareIncrementalChampionDetailStatisticsSource(database, []string{"16.16.1"}, options)
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Ready || prepared.ProcessedMatches != 1 || prepared.ParticipantRows != 2 {
		t.Fatalf("unexpected initial source result: %+v", prepared)
	}
	repeated, err := PrepareIncrementalChampionDetailStatisticsSource(database, []string{"16.16.1"}, options)
	if err != nil {
		t.Fatal(err)
	}
	if !repeated.Ready || repeated.ProcessedMatches != 0 || repeated.ParticipantRows != 2 {
		t.Fatalf("incremental rerun rebuilt existing matches: %+v", repeated)
	}

	base, err := GetChampionDetailStatisticMXDAOs(database, []string{"16.16.1"})
	if err != nil || len(base) != 2 {
		t.Fatalf("base statistics: rows=%d err=%v", len(base), err)
	}
	positions, err := GetChampionPositionStatisticsMXDAOs(database, []string{"16.16.1"})
	if err != nil || len(positions) != 2 {
		t.Fatalf("position statistics: rows=%d err=%v", len(positions), err)
	}
	metas, err := GetChampionDetailStatisticsMetaMXDAOs(database)
	if err != nil || len(metas) != 2 {
		t.Fatalf("meta statistics: rows=%d err=%v", len(metas), err)
	}
	for _, meta := range metas {
		if meta.Total != 1 || meta.Item0Id != 1003 || meta.Item1Id != 1002 || meta.Item2Id != 1001 {
			t.Fatalf("unexpected meta row: %+v", meta)
		}
	}
	counters, err := GetChampionCounterStatisticsMXDAOs(database)
	if err != nil || len(counters) != 2 {
		t.Fatalf("counter statistics: rows=%d err=%v", len(counters), err)
	}
}
