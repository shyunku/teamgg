package migrations

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func TestDataRetentionMySQL(t *testing.T) {
	dsn := os.Getenv("TEAMGG_NUMERIC_KEY_MYSQL_TEST_DSN")
	if dsn == "" && os.Getenv("TEAMGG_NUMERIC_KEY_MYSQL_TEST_FROM_DB_ENV") == "true" {
		config := mysql.NewConfig()
		config.User = os.Getenv("DB_USER")
		config.Passwd = os.Getenv("DB_PASSWORD")
		config.Net = "tcp"
		config.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
		config.ParseTime = true
		config.MultiStatements = true
		dsn = config.FormatDSN()
	}
	if dsn == "" {
		t.Skip("TEAMGG_NUMERIC_KEY_MYSQL_TEST_DSN is not set")
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.DBName = ""
	config.ParseTime = true
	config.MultiStatements = true

	ctx := context.Background()
	admin, err := sqlx.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err := pingNumericKeyTestDatabase(ctx, admin, 30*time.Second); err != nil {
		t.Fatal(err)
	}

	databaseName := fmt.Sprintf("teamgg_retention_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+databaseName+"`"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS `"+databaseName+"`") })
	config.DBName = databaseName
	database, err := sqlx.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	createDataRetentionFixture(t, database)

	dryRun, err := CleanupRetainedData(ctx, database, DataRetentionOptions{
		DryRun: true, RetainedPatches: 3, BatchSize: 10,
		BatchTimeout: time.Minute, WorkLimit: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.EligibleMatches != 1 || dryRun.DeletedMatches != 0 || dryRun.Completed {
		t.Fatalf("unexpected dry-run result: %+v", dryRun)
	}
	assertRetentionFixtureCounts(t, database, 4, 4)

	deleted, err := CleanupRetainedData(ctx, database, DataRetentionOptions{
		DryRun: false, DeleteAcknowledged: true, OfflineAcknowledged: true,
		RetainedPatches: 3, BatchSize: 10,
		BatchTimeout: time.Minute, WorkLimit: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if deleted.EligibleMatches != 1 || deleted.DeletedMatches != 1 || !deleted.Completed {
		t.Fatalf("unexpected delete result: %+v", deleted)
	}
	assertRetentionFixtureCounts(t, database, 3, 3)
}

func createDataRetentionFixture(t *testing.T, database *sqlx.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE matches (match_id VARCHAR(32) PRIMARY KEY, game_version VARCHAR(32) NOT NULL, KEY matches_game_version_index (game_version)) ENGINE=InnoDB`,
		`CREATE TABLE match_participants (match_id VARCHAR(32), participant_id INT, match_participant_id VARCHAR(32) PRIMARY KEY, puuid VARCHAR(32)) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_details (match_participant_id VARCHAR(32) PRIMARY KEY, match_id VARCHAR(32)) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_perks (match_participant_id VARCHAR(32) PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_perk_styles (match_participant_id VARCHAR(32), style_id VARCHAR(32) PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_perk_style_selections (style_id VARCHAR(32), perk INT) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_numeric_keys (match_participant_id BIGINT PRIMARY KEY, legacy_match_participant_id VARCHAR(32)) ENGINE=InnoDB`,
		`CREATE TABLE match_team_bans (match_id VARCHAR(32), team_id INT) ENGINE=InnoDB`,
		`CREATE TABLE match_teams (match_id VARCHAR(32), team_id INT) ENGINE=InnoDB`,
		`CREATE TABLE summoner_matches (puuid VARCHAR(32), match_id VARCHAR(32)) ENGINE=InnoDB`,
		`CREATE TABLE data_explorer_match_sources (match_id VARCHAR(32), puuid VARCHAR(32)) ENGINE=InnoDB`,
		`CREATE TABLE data_explorer_match_jobs (match_id VARCHAR(32) PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE data_explorer_match_processing_state (match_id VARCHAR(32) PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE champion_detail_statistics_processed_matches (game_version VARCHAR(32), match_id VARCHAR(32)) ENGINE=InnoDB`,
		`CREATE TABLE match_numeric_keys (match_id BIGINT PRIMARY KEY, riot_match_id VARCHAR(32)) ENGINE=InnoDB`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for index, patch := range []string{"16.17.1", "16.16.1", "16.15.1", "16.14.1"} {
		matchID := fmt.Sprintf("KR-%d", index+1)
		participantID := fmt.Sprintf("participant-%d", index+1)
		styleID := fmt.Sprintf("style-%d", index+1)
		queries := []struct {
			query string
			args  []interface{}
		}{
			{`INSERT INTO matches VALUES (?, ?)`, []interface{}{matchID, patch}},
			{`INSERT INTO match_participants VALUES (?, 1, ?, 'puuid')`, []interface{}{matchID, participantID}},
			{`INSERT INTO match_participant_details VALUES (?, ?)`, []interface{}{participantID, matchID}},
			{`INSERT INTO match_participant_perks VALUES (?)`, []interface{}{participantID}},
			{`INSERT INTO match_participant_perk_styles VALUES (?, ?)`, []interface{}{participantID, styleID}},
			{`INSERT INTO match_participant_perk_style_selections VALUES (?, 1)`, []interface{}{styleID}},
			{`INSERT INTO match_participant_numeric_keys VALUES (?, ?)`, []interface{}{index + 1, participantID}},
			{`INSERT INTO match_team_bans VALUES (?, 100)`, []interface{}{matchID}},
			{`INSERT INTO match_teams VALUES (?, 100)`, []interface{}{matchID}},
			{`INSERT INTO summoner_matches VALUES ('puuid', ?)`, []interface{}{matchID}},
			{`INSERT INTO data_explorer_match_sources VALUES (?, 'puuid')`, []interface{}{matchID}},
			{`INSERT INTO data_explorer_match_jobs VALUES (?)`, []interface{}{matchID}},
			{`INSERT INTO data_explorer_match_processing_state VALUES (?)`, []interface{}{matchID}},
			{`INSERT INTO champion_detail_statistics_processed_matches VALUES (?, ?)`, []interface{}{patch, matchID}},
			{`INSERT INTO match_numeric_keys VALUES (?, ?)`, []interface{}{index + 1, matchID}},
		}
		for _, query := range queries {
			if _, err := database.Exec(query.query, query.args...); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func assertRetentionFixtureCounts(t *testing.T, database *sqlx.DB, matches, children int) {
	t.Helper()
	for _, table := range []string{
		"matches", "match_participants", "match_participant_details", "match_participant_perks",
		"match_participant_perk_styles", "match_participant_perk_style_selections",
		"match_participant_numeric_keys", "match_team_bans", "match_teams", "summoner_matches",
		"data_explorer_match_sources", "data_explorer_match_jobs", "data_explorer_match_processing_state",
		"champion_detail_statistics_processed_matches", "match_numeric_keys",
	} {
		var count int
		if err := database.Get(&count, "SELECT COUNT(*) FROM `"+table+"`"); err != nil {
			t.Fatal(err)
		}
		expected := children
		if table == "matches" {
			expected = matches
		}
		if count != expected {
			t.Fatalf("unexpected %s count: got=%d want=%d", table, count, expected)
		}
	}
}
