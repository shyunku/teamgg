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
		`CREATE TABLE masteries (puuid VARCHAR(255) NOT NULL, champion_id INT NOT NULL, PRIMARY KEY (puuid, champion_id)) ENGINE=InnoDB`,
		`CREATE TABLE leagues (puuid VARCHAR(255) NOT NULL, league_id VARCHAR(64) NOT NULL, queue_type VARCHAR(64) NOT NULL, PRIMARY KEY (puuid, league_id, queue_type)) ENGINE=InnoDB`,
		`CREATE TABLE summoner_matches (puuid VARCHAR(255) NOT NULL, match_id VARCHAR(255) NOT NULL, PRIMARY KEY (puuid, match_id)) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_details (match_participant_id VARCHAR(255) NOT NULL PRIMARY KEY, match_id VARCHAR(255) NOT NULL) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_perks (match_participant_id VARCHAR(255) NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE match_participant_perk_styles (match_participant_id VARCHAR(255) NOT NULL, style_id VARCHAR(64) NOT NULL) ENGINE=InnoDB`,
		`CREATE TABLE match_teams (match_id VARCHAR(255) NOT NULL, team_id INT NOT NULL, PRIMARY KEY (match_id, team_id)) ENGINE=InnoDB`,
		`CREATE TABLE match_team_bans (match_id VARCHAR(255) NOT NULL, team_id INT NOT NULL, champion_id INT NOT NULL, pick_turn INT NOT NULL) ENGINE=InnoDB`,
		`INSERT INTO summoners (puuid) VALUES ('legacy-puuid')`,
		`INSERT INTO matches (match_id) VALUES ('KR_legacy')`,
		`INSERT INTO match_participants
			(match_id, participant_id, match_participant_id, puuid)
		 VALUES ('KR_legacy', 1, 'legacy-participant', 'legacy-puuid')`,
		`INSERT INTO masteries VALUES ('legacy-puuid', 1)`,
		`INSERT INTO leagues VALUES ('legacy-puuid', 'league', 'RANKED_SOLO_5x5')`,
		`INSERT INTO summoner_matches VALUES ('legacy-puuid', 'KR_legacy')`,
		`INSERT INTO match_participant_details VALUES ('legacy-participant', 'KR_legacy')`,
		`INSERT INTO match_participant_perks VALUES ('legacy-participant')`,
		`INSERT INTO match_participant_perk_styles VALUES ('legacy-participant', 'primary'), ('legacy-participant', 'secondary')`,
		`INSERT INTO match_teams VALUES ('KR_legacy', 100)`,
		`INSERT INTO match_team_bans VALUES ('KR_legacy', 100, 1, 1)`,
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	if err := applyNumericKeyFoundation(ctx, database); err != nil {
		t.Fatal(err)
	}
	if err := applyNumericKeyChildren(ctx, database); err != nil {
		t.Fatal(err)
	}
	result, err := BackfillNumericKeys(ctx, database, NumericKeyBackfillOptions{
		BatchSize: 10,
		WorkLimit: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready || !result.SummonersCompleted || !result.MatchesCompleted || !result.ParticipantsCompleted || !result.ChildrenCompleted {
		t.Fatalf("unexpected first backfill result: %+v", result)
	}
	childrenReady, err := validateNumericKeyChildrenBackfill(ctx, database)
	if err != nil || !childrenReady {
		t.Fatalf("legacy child rows were not backfilled: ready=%t err=%v", childrenReady, err)
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
	if _, err := database.ExecContext(ctx, `INSERT INTO masteries (puuid, champion_id) VALUES ('new-puuid', 2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO match_participant_perks (match_participant_id) VALUES ('new-participant')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO match_teams (match_id, team_id) VALUES ('KR_new', 100)`); err != nil {
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
	ready, err := validateNumericKeyBackfill(ctx, database)
	if err != nil || !ready {
		t.Fatalf("reserved identity must remain valid before profile discovery: ready=%t err=%v", ready, err)
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

	ready, err = validateNumericKeyBackfill(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	if !ready {
		t.Fatal("new writes were not assigned consistent numeric keys")
	}
	childrenReady, err = validateNumericKeyChildrenBackfill(ctx, database)
	if err != nil || !childrenReady {
		t.Fatalf("new child writes were not assigned consistent numeric keys: ready=%t err=%v", childrenReady, err)
	}
	second, err := BackfillNumericKeys(ctx, database, NumericKeyBackfillOptions{
		BatchSize: 10,
		WorkLimit: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Ready || second.SummonersProcessed != 0 || second.MatchesProcessed != 0 || second.ParticipantsProcessed != 0 || second.ChildrenProcessed != 0 {
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
