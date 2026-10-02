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
	ctx := context.Background()
	database := openDataRetentionTestDatabase(t)
	createDataRetentionFixture(t, database)

	dryRun, err := CleanupRetainedData(ctx, database, DataRetentionOptions{
		DryRun: true, EnforceLoadGuard: true, MaxThreadsRunning: 100, MaxLockWaits: 100,
		RetainedPatches: 3, BatchSize: 10,
		BatchTimeout: time.Minute, WorkLimit: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.EligibleMatches != 1 || dryRun.DeletedMatches != 0 || dryRun.Completed {
		t.Fatalf("unexpected dry-run result: %+v", dryRun)
	}
	assertRetentionFixtureCounts(t, database, 4, 4, 4)

	deleted, err := CleanupRetainedData(ctx, database, DataRetentionOptions{
		DryRun: false, DeleteAcknowledged: true, OfflineAcknowledged: true,
		EnforceLoadGuard: true, MaxThreadsRunning: 100, MaxLockWaits: 100,
		RetainedPatches: 3, BatchSize: 10,
		BatchTimeout: time.Minute, WorkLimit: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if deleted.EligibleMatches != 1 || deleted.DeletedMatches != 1 || !deleted.Completed {
		t.Fatalf("unexpected delete result: %+v", deleted)
	}
	assertRetentionFixtureCounts(t, database, 3, 3, 4)
}

func TestDataRetentionMySQLResumesAfterBatchFailure(t *testing.T) {
	ctx := context.Background()
	database := openDataRetentionTestDatabase(t)
	createDataRetentionFixture(t, database)
	// KR-4 plus 15 extra matches on the expired patch span two batches of 10.
	for index := 0; index < 15; index++ {
		insertRetentionFixtureMatch(t, database, 5+index, fmt.Sprintf("KR-E%02d", index), "16.14.1")
	}
	// Fail the second batch after its child rows were already deleted inside the transaction.
	if _, err := database.Exec(`
		CREATE TRIGGER retention_test_fail_delete BEFORE DELETE ON matches FOR EACH ROW
		BEGIN
			IF OLD.match_id = 'KR-E12' THEN
				SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'retention test failure';
			END IF;
		END`); err != nil {
		t.Fatal(err)
	}
	options := DataRetentionOptions{
		DryRun: false, DeleteAcknowledged: true, OfflineAcknowledged: true,
		EnforceLoadGuard: true, MaxThreadsRunning: 100, MaxLockWaits: 100,
		RetainedPatches: 3, BatchSize: 10,
		BatchTimeout: time.Minute, WorkLimit: time.Minute,
	}

	failed, err := CleanupRetainedData(ctx, database, options)
	if err == nil {
		t.Fatalf("expected injected batch failure: %+v", failed)
	}
	if failed.EligibleMatches != 16 || failed.DeletedMatches != 10 || failed.Completed {
		t.Fatalf("unexpected failed result: %+v", failed)
	}
	// The committed first batch stays deleted and the failed batch is rolled back as a whole.
	assertRetentionFixtureCounts(t, database, 9, 9, 19)

	if _, err := database.Exec(`DROP TRIGGER retention_test_fail_delete`); err != nil {
		t.Fatal(err)
	}
	resumed, err := CleanupRetainedData(ctx, database, options)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.EligibleMatches != 6 || resumed.DeletedMatches != 6 || !resumed.Completed {
		t.Fatalf("unexpected resumed result: %+v", resumed)
	}
	assertRetentionFixtureCounts(t, database, 3, 3, 19)

	repeated, err := CleanupRetainedData(ctx, database, options)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.EligibleMatches != 0 || repeated.DeletedMatches != 0 || !repeated.Completed {
		t.Fatalf("unexpected repeated result: %+v", repeated)
	}
}

func TestDataRetentionMySQLOnlineRetriesLockConflicts(t *testing.T) {
	ctx := context.Background()
	database := openDataRetentionTestDatabase(t)
	createDataRetentionFixture(t, database)
	for index := 0; index < 15; index++ {
		insertRetentionFixtureMatch(t, database, 5+index, fmt.Sprintf("KR-E%02d", index), "16.14.1")
	}

	// A concurrent API-style writer holds an expired match row longer than the online lock wait.
	writer, err := database.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec(`SELECT match_id FROM matches WHERE match_id = 'KR-E12' FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() {
		defer close(released)
		time.Sleep(7 * time.Second)
		_ = writer.Rollback()
	}()
	t.Cleanup(func() { <-released })

	result, err := CleanupRetainedData(ctx, database, DataRetentionOptions{
		DryRun: false, DeleteAcknowledged: true, Online: true,
		EnforceLoadGuard: true, MaxThreadsRunning: 100, MaxLockWaits: 100,
		RetainedPatches: 3, BatchSize: 10, BatchPause: 10 * time.Millisecond,
		BatchTimeout: time.Minute, WorkLimit: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Online || result.EligibleMatches != 16 || result.DeletedMatches != 16 || !result.Completed || result.RetriedBatches < 1 {
		t.Fatalf("unexpected online result: %+v", result)
	}
	assertRetentionFixtureCounts(t, database, 3, 3, 19)
}

func openDataRetentionTestDatabase(t *testing.T) *sqlx.DB {
	t.Helper()
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
	t.Cleanup(func() { _ = admin.Close() })
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
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func createDataRetentionFixture(t *testing.T, database *sqlx.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE matches (match_id VARCHAR(32) PRIMARY KEY, game_version VARCHAR(32) NOT NULL, KEY matches_game_version_index (game_version)) ENGINE=InnoDB`,
		`CREATE TABLE match_participants (match_id VARCHAR(32), participant_id INT, match_participant_id VARCHAR(32) PRIMARY KEY, puuid VARCHAR(32), KEY match_participants_match_id_index (match_id)) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_details (match_participant_id VARCHAR(32) PRIMARY KEY, match_id VARCHAR(32)) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_perks (match_participant_id VARCHAR(32) PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_perk_styles (match_participant_id VARCHAR(32), style_id VARCHAR(32) PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_perk_style_selections (style_id VARCHAR(32), perk INT) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_numeric_keys (match_participant_id BIGINT PRIMARY KEY, legacy_match_participant_id VARCHAR(32), UNIQUE KEY match_participant_numeric_keys_legacy_uindex (legacy_match_participant_id)) ENGINE=InnoDB`,
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
		insertRetentionFixtureMatch(t, database, index+1, fmt.Sprintf("KR-%d", index+1), patch)
	}
}

func insertRetentionFixtureMatch(t *testing.T, database *sqlx.DB, identity int, matchID, patch string) {
	t.Helper()
	participantID := "participant-" + matchID
	styleID := "style-" + matchID
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
		{`INSERT INTO match_participant_numeric_keys VALUES (?, ?)`, []interface{}{identity, participantID}},
		{`INSERT INTO match_team_bans VALUES (?, 100)`, []interface{}{matchID}},
		{`INSERT INTO match_teams VALUES (?, 100)`, []interface{}{matchID}},
		{`INSERT INTO summoner_matches VALUES ('puuid', ?)`, []interface{}{matchID}},
		{`INSERT INTO data_explorer_match_sources VALUES (?, 'puuid')`, []interface{}{matchID}},
		{`INSERT INTO data_explorer_match_jobs VALUES (?)`, []interface{}{matchID}},
		{`INSERT INTO data_explorer_match_processing_state VALUES (?)`, []interface{}{matchID}},
		{`INSERT INTO champion_detail_statistics_processed_matches VALUES (?, ?)`, []interface{}{patch, matchID}},
		{`INSERT INTO match_numeric_keys VALUES (?, ?)`, []interface{}{identity, matchID}},
	}
	for _, query := range queries {
		if _, err := database.Exec(query.query, query.args...); err != nil {
			t.Fatal(err)
		}
	}
}

func assertRetentionFixtureCounts(t *testing.T, database *sqlx.DB, matches, children, identities int) {
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
		if table == "match_participant_numeric_keys" || table == "match_numeric_keys" {
			expected = identities
		}
		if count != expected {
			t.Fatalf("unexpected %s count: got=%d want=%d", table, count, expected)
		}
	}
}
