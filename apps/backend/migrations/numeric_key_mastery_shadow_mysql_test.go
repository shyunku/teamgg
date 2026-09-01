package migrations

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func TestMasteryNumericShadowMySQL(t *testing.T) {
	dsn := os.Getenv("TEAMGG_NUMERIC_KEY_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("TEAMGG_NUMERIC_KEY_MYSQL_TEST_DSN is not set")
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ParseTime = true
	config.MultiStatements = true
	config.DBName = ""

	ctx := context.Background()
	admin, err := sqlx.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err := pingNumericKeyTestDatabase(ctx, admin, 30*time.Second); err != nil {
		t.Fatal(err)
	}

	databaseName := fmt.Sprintf("teamgg_mastery_shadow_test_%d", time.Now().UnixNano())
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
	summonerCount := 1000
	if configured := os.Getenv("TEAMGG_MASTERY_SHADOW_TEST_SUMMONERS"); configured != "" {
		parsed, parseErr := strconv.Atoi(configured)
		if parseErr != nil || parsed < 1000 || parsed > 10000 {
			t.Fatalf("invalid TEAMGG_MASTERY_SHADOW_TEST_SUMMONERS %q", configured)
		}
		summonerCount = parsed
	}
	expectedRows := int64(summonerCount * 100)
	fixtureStarted := time.Now()
	createMasteryNumericShadowFixture(t, ctx, database, summonerCount)
	t.Logf("created %d mastery fixture rows in %s", expectedRows, time.Since(fixtureStarted))

	lockConnection, err := database.Connx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var acquired int
	if err := lockConnection.GetContext(ctx, &acquired, `SELECT GET_LOCK(?, 0)`, masteryNumericShadowLock); err != nil || acquired != 1 {
		_ = lockConnection.Close()
		t.Fatalf("failed to reserve integration-test advisory lock: acquired=%d err=%v", acquired, err)
	}
	if _, err := PrepareMasteryNumericShadow(ctx, database, MasteryNumericShadowOptions{
		BatchSize:           1000,
		WorkLimit:           time.Minute,
		OfflineAcknowledged: true,
	}); err == nil || !strings.Contains(err.Error(), "already running") {
		_, _ = lockConnection.ExecContext(ctx, `SELECT RELEASE_LOCK(?)`, masteryNumericShadowLock)
		_ = lockConnection.Close()
		t.Fatalf("concurrent shadow command was not rejected: %v", err)
	}
	_, _ = lockConnection.ExecContext(ctx, `SELECT RELEASE_LOCK(?)`, masteryNumericShadowLock)
	_ = lockConnection.Close()

	firstStarted := time.Now()
	first, err := PrepareMasteryNumericShadow(ctx, database, MasteryNumericShadowOptions{
		BatchSize:           1000,
		WorkLimit:           time.Minute,
		MaxBatches:          1,
		OfflineAcknowledged: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ProcessedThisRun != 1000 || first.CopyCompleted || first.Validated {
		t.Fatalf("first bounded copy did not stop after one batch: %+v", first)
	}
	t.Logf("first bounded batch copied %d rows in %s", first.ProcessedThisRun, time.Since(firstStarted))

	resumeStarted := time.Now()
	second, err := PrepareMasteryNumericShadow(ctx, database, MasteryNumericShadowOptions{
		BatchSize:           50000,
		WorkLimit:           5 * time.Minute,
		OfflineAcknowledged: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.ProcessedThisRun != expectedRows-1000 || second.ProcessedTotal != expectedRows || !second.CopyCompleted || !second.Validated {
		t.Fatalf("resumed mastery shadow copy did not validate: %+v", second)
	}
	t.Logf("resumed copy and validation processed %d rows in %s", second.ProcessedThisRun, time.Since(resumeStarted))

	third, err := PrepareMasteryNumericShadow(ctx, database, MasteryNumericShadowOptions{
		BatchSize:           50000,
		WorkLimit:           time.Minute,
		OfflineAcknowledged: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if third.ProcessedThisRun != 0 || third.ProcessedTotal != expectedRows || !third.Validated {
		t.Fatalf("idempotent mastery shadow rerun changed rows: %+v", third)
	}

	var legacyNumericRows int
	if err := database.GetContext(ctx, &legacyNumericRows, `SELECT COUNT(*) FROM masteries WHERE summoner_fk IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	if legacyNumericRows != 0 {
		t.Fatalf("legacy masteries were updated in place: %d rows", legacyNumericRows)
	}

	var sourcePoints, shadowPoints int
	if err := database.QueryRowxContext(ctx, `
		SELECT source.champion_points, shadow.champion_points
		FROM masteries source
		INNER JOIN summoner_numeric_keys numeric_key ON numeric_key.puuid = source.puuid
		INNER JOIN masteries_numeric_v2 shadow
			ON shadow.summoner_fk = numeric_key.summoner_id
		   AND shadow.champion_id = source.champion_id
		WHERE source.puuid = ? AND source.champion_id = 42
	`, fixtureMasteryPuuid(123)).Scan(&sourcePoints, &shadowPoints); err != nil {
		t.Fatal(err)
	}
	if sourcePoints != shadowPoints {
		t.Fatalf("numeric mastery read changed value: source=%d shadow=%d", sourcePoints, shadowPoints)
	}

	if _, err := database.ExecContext(ctx, `
		UPDATE masteries_numeric_v2
		SET champion_points = champion_points + 1
		WHERE summoner_fk = 124 AND champion_id = 42
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareMasteryNumericShadow(ctx, database, MasteryNumericShadowOptions{
		BatchSize:           50000,
		WorkLimit:           time.Minute,
		OfflineAcknowledged: true,
	}); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("checksum corruption was not rejected: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE masteries_numeric_v2 shadow
		INNER JOIN summoner_numeric_keys numeric_key ON numeric_key.summoner_id = shadow.summoner_fk
		INNER JOIN masteries source
			ON source.puuid = numeric_key.puuid AND source.champion_id = shadow.champion_id
		SET shadow.champion_points = source.champion_points
		WHERE shadow.summoner_fk = 124 AND shadow.champion_id = 42
	`); err != nil {
		t.Fatal(err)
	}
	repaired, err := PrepareMasteryNumericShadow(ctx, database, MasteryNumericShadowOptions{
		BatchSize:           50000,
		WorkLimit:           time.Minute,
		OfflineAcknowledged: true,
	})
	if err != nil || !repaired.Validated {
		t.Fatalf("repaired shadow did not validate: result=%+v err=%v", repaired, err)
	}
	if err := ValidateMasteryNumericShadowCutover(ctx, database); err != nil {
		t.Fatalf("validated shadow was not ready for read cutover: %v", err)
	}

	if _, err := database.ExecContext(ctx, `
		UPDATE masteries
		SET champion_points = champion_points + 17
		WHERE puuid = ? AND champion_id = 42
	`, fixtureMasteryPuuid(123)); err != nil {
		t.Fatalf("update legacy mastery through shadow sync trigger: %v", err)
	}
	if err := database.QueryRowxContext(ctx, `
		SELECT source.champion_points, shadow.champion_points
		FROM masteries source
		INNER JOIN summoner_numeric_keys numeric_key ON numeric_key.puuid = source.puuid
		INNER JOIN masteries_numeric_v2 shadow
			ON shadow.summoner_fk = numeric_key.summoner_id
		   AND shadow.champion_id = source.champion_id
		WHERE source.puuid = ? AND source.champion_id = 42
	`, fixtureMasteryPuuid(123)).Scan(&sourcePoints, &shadowPoints); err != nil {
		t.Fatal(err)
	}
	if sourcePoints != shadowPoints {
		t.Fatalf("legacy update was not mirrored: source=%d shadow=%d", sourcePoints, shadowPoints)
	}

	if _, err := database.ExecContext(ctx, `
		DELETE FROM masteries WHERE puuid = ? AND champion_id = 43
	`, fixtureMasteryPuuid(123)); err != nil {
		t.Fatalf("delete legacy mastery through shadow sync trigger: %v", err)
	}
	var deletedShadowRows int
	if err := database.GetContext(ctx, &deletedShadowRows, `
		SELECT COUNT(*)
		FROM masteries_numeric_v2
		WHERE summoner_fk = 124 AND champion_id = 43
	`); err != nil {
		t.Fatal(err)
	}
	if deletedShadowRows != 0 {
		t.Fatalf("legacy delete was not mirrored: %d shadow rows remain", deletedShadowRows)
	}

	if _, err := database.ExecContext(ctx, `ANALYZE TABLE masteries, masteries_numeric_v2`); err != nil {
		t.Fatal(err)
	}
	var legacyDataBytes, legacyIndexBytes, shadowDataBytes, shadowIndexBytes int64
	if err := database.QueryRowxContext(ctx, `
		SELECT
			MAX(CASE WHEN table_name = 'masteries' THEN data_length END),
			MAX(CASE WHEN table_name = 'masteries' THEN index_length END),
			MAX(CASE WHEN table_name = 'masteries_numeric_v2' THEN data_length END),
			MAX(CASE WHEN table_name = 'masteries_numeric_v2' THEN index_length END)
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_name IN ('masteries', 'masteries_numeric_v2')
	`).Scan(&legacyDataBytes, &legacyIndexBytes, &shadowDataBytes, &shadowIndexBytes); err != nil {
		t.Fatal(err)
	}
	legacyBytes := legacyDataBytes + legacyIndexBytes
	shadowBytes := shadowDataBytes + shadowIndexBytes
	if shadowBytes >= legacyBytes {
		t.Fatalf("compact shadow did not reduce allocated bytes: legacy=%d shadow=%d", legacyBytes, shadowBytes)
	}
	t.Logf("mastery shadow allocated bytes: legacy=%d (data=%d index=%d) shadow=%d (data=%d index=%d) reduction=%.2f%%",
		legacyBytes, legacyDataBytes, legacyIndexBytes,
		shadowBytes, shadowDataBytes, shadowIndexBytes,
		100*(1-float64(shadowBytes)/float64(legacyBytes)))
}

func createMasteryNumericShadowFixture(t *testing.T, ctx context.Context, database *sqlx.DB, summonerCount int) {
	t.Helper()
	statements := []string{
		`CREATE TABLE summoners (
			puuid VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
			summoner_pk BIGINT UNSIGNED NULL,
			PRIMARY KEY (puuid)
		) ENGINE=InnoDB`,
		`CREATE TABLE summoner_numeric_keys (
			summoner_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			puuid VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
			PRIMARY KEY (summoner_id),
			UNIQUE KEY summoner_numeric_keys_puuid_uindex (puuid)
		) ENGINE=InnoDB`,
		`CREATE TABLE masteries (
			puuid VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
			champion_points_until_next_level BIGINT NOT NULL,
			chest_granted TINYINT(1) NOT NULL,
			champion_id BIGINT NOT NULL,
			last_play_time DATETIME NOT NULL,
			champion_level INT NOT NULL,
			champion_points INT NOT NULL,
			champion_points_since_last_level BIGINT NOT NULL,
			tokens_earned INT NOT NULL,
			summoner_fk BIGINT UNSIGNED NULL,
			PRIMARY KEY (puuid, champion_id),
			KEY masteries_champion_points_level_covering_index
				(champion_id, champion_points DESC, champion_level)
		) ENGINE=InnoDB`,
		`CREATE TABLE fixture_champions (champion_id BIGINT NOT NULL PRIMARY KEY) ENGINE=InnoDB`,
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	tx, err := database.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for summoner := 0; summoner < summonerCount; summoner++ {
		puuid := fixtureMasteryPuuid(summoner)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO summoner_numeric_keys (summoner_id, puuid) VALUES (?, ?)
		`, summoner+1, puuid); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO summoners (puuid, summoner_pk) VALUES (?, ?)
		`, puuid, summoner+1); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	for champion := 1; champion <= 100; champion++ {
		if _, err := tx.ExecContext(ctx, `INSERT INTO fixture_champions (champion_id) VALUES (?)`, champion); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO masteries (
			puuid, champion_points_until_next_level, chest_granted, champion_id,
			last_play_time, champion_level, champion_points,
			champion_points_since_last_level, tokens_earned, summoner_fk
		)
		SELECT summoner.puuid,
			champion.champion_id * 100,
			MOD(champion.champion_id, 2),
			champion.champion_id,
			DATE_ADD('2026-01-01 00:00:00', INTERVAL champion.champion_id DAY),
			MOD(champion.champion_id, 7) + 1,
			summoner.summoner_pk * 1000 + champion.champion_id,
			champion.champion_id * 10,
			MOD(champion.champion_id, 3),
			NULL
		FROM summoners summoner
		CROSS JOIN fixture_champions champion
	`); err != nil {
		t.Fatal(err)
	}
}

func fixtureMasteryPuuid(index int) string {
	return fmt.Sprintf("%072d-puuid", index)
}
