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

func TestNumericKeyFoundationAndBackfillMySQL(t *testing.T) {
	dsn := os.Getenv("TEAMGG_NUMERIC_KEY_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("TEAMGG_NUMERIC_KEY_MYSQL_TEST_DSN is not set")
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	config.DBName = ""
	admin, err := sqlx.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	if err := pingNumericKeyTestDatabase(ctx, admin, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	testDatabase := fmt.Sprintf("teamgg_numeric_key_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+testDatabase+"`"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + testDatabase + "`"); err != nil {
			t.Error(err)
		}
	})
	config.DBName = testDatabase
	database, err := sqlx.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	statements := []string{
		`CREATE TABLE summoners (puuid VARCHAR(255) NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE matches (match_id VARCHAR(255) NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE match_participants (
			match_id VARCHAR(255) NOT NULL,
			participant_id INT NOT NULL,
			match_participant_id VARCHAR(255) NOT NULL,
			puuid VARCHAR(255) NOT NULL,
			PRIMARY KEY (match_id, participant_id),
			UNIQUE KEY match_participants_legacy_uindex (match_participant_id)
		) ENGINE=InnoDB`,
		`INSERT INTO summoners (puuid) VALUES ('legacy-puuid')`,
		`INSERT INTO matches (match_id) VALUES ('KR_legacy')`,
		`INSERT INTO match_participants
			(match_id, participant_id, match_participant_id, puuid)
		 VALUES ('KR_legacy', 1, 'legacy-participant', 'legacy-puuid')`,
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	if err := applyNumericKeyFoundation(ctx, database); err != nil {
		t.Fatal(err)
	}
	result, err := BackfillNumericKeys(ctx, database, NumericKeyBackfillOptions{
		BatchSize: 10,
		WorkLimit: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready || !result.SummonersCompleted || !result.MatchesCompleted || !result.ParticipantsCompleted {
		t.Fatalf("unexpected first backfill result: %+v", result)
	}

	if _, err := database.ExecContext(ctx, `INSERT INTO summoners (puuid) VALUES ('new-puuid')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO matches (match_id) VALUES ('KR_new')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO match_participants
			(match_id, participant_id, match_participant_id, puuid)
		VALUES ('KR_new', 1, 'new-participant', 'new-puuid')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO match_participants
			(match_id, participant_id, match_participant_id, puuid)
		VALUES ('KR_new', 2, 'orphan-participant', 'missing-puuid')
	`); err != nil {
		t.Fatalf("participant trigger blocked a summoner pending discovery: %v", err)
	}
	var reservedSummonerId, participantSummonerId int64
	if err := database.QueryRowxContext(ctx, `
		SELECT numeric_key.summoner_id, participant.summoner_fk
		FROM match_participants participant
		INNER JOIN summoner_numeric_keys numeric_key ON numeric_key.puuid = participant.puuid
		WHERE participant.match_participant_id = 'orphan-participant'
	`).Scan(&reservedSummonerId, &participantSummonerId); err != nil {
		t.Fatal(err)
	}
	if reservedSummonerId == 0 || reservedSummonerId != participantSummonerId {
		t.Fatalf("pending summoner identity was not reserved consistently: key=%d participant=%d", reservedSummonerId, participantSummonerId)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO summoners (puuid) VALUES ('missing-puuid')`); err != nil {
		t.Fatal(err)
	}
	var discoveredSummonerId int64
	if err := database.GetContext(ctx, &discoveredSummonerId, `
		SELECT summoner_pk FROM summoners WHERE puuid = 'missing-puuid'
	`); err != nil {
		t.Fatal(err)
	}
	if discoveredSummonerId != reservedSummonerId {
		t.Fatalf("discovered summoner did not reuse reserved identity: reserved=%d discovered=%d", reservedSummonerId, discoveredSummonerId)
	}

	ready, err := validateNumericKeyBackfill(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("new writes were not assigned consistent numeric keys")
	}
	second, err := BackfillNumericKeys(ctx, database, NumericKeyBackfillOptions{
		BatchSize: 10,
		WorkLimit: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Ready || second.SummonersProcessed != 0 || second.MatchesProcessed != 0 || second.ParticipantsProcessed != 0 {
		t.Fatalf("backfill was not idempotent: %+v", second)
	}
}

func pingNumericKeyTestDatabase(ctx context.Context, database *sqlx.DB, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastError error
	for time.Now().Before(deadline) {
		if err := database.PingContext(ctx); err == nil {
			return nil
		} else {
			lastError = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("isolated MySQL did not become reachable: %w", lastError)
}
