package migrations

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMasteryNumericShadowRequiresExplicitOfflineAcknowledgement(t *testing.T) {
	_, err := PrepareMasteryNumericShadow(context.Background(), nil, MasteryNumericShadowOptions{})
	if err == nil || !strings.Contains(err.Error(), "OFFLINE_ACK=true") {
		t.Fatalf("expected offline acknowledgement error, got %v", err)
	}
}

func TestMasteryNumericShadowOptionsAreBounded(t *testing.T) {
	t.Setenv("MASTERY_NUMERIC_SHADOW_BATCH_SIZE", "500")
	t.Setenv("MASTERY_NUMERIC_SHADOW_BATCH_TIMEOUT", "45s")
	t.Setenv("MASTERY_NUMERIC_SHADOW_WORK_LIMIT", "5m")
	t.Setenv("MASTERY_NUMERIC_SHADOW_MAX_BATCHES", "3")
	t.Setenv("MASTERY_NUMERIC_SHADOW_OFFLINE_ACK", "true")
	t.Setenv("MASTERY_NUMERIC_SHADOW_DISABLE_BINLOG", "true")

	options := MasteryNumericShadowOptionsFromEnvironment()
	if options.BatchSize != 500 || options.BatchTimeout != 45*time.Second || options.WorkLimit != 5*time.Minute || options.MaxBatches != 3 ||
		!options.OfflineAcknowledged || !options.DisableBinlog {
		t.Fatalf("unexpected options: %+v", options)
	}
}

func TestMasteryNumericShadowBatchSelectCannotJoinBeforeLimit(t *testing.T) {
	query := strings.ToLower(strings.Join(strings.Fields(masteryNumericShadowBatchQuery), " "))
	if strings.Contains(query, " join ") {
		t.Fatalf("bounded source query must not contain a join: %s", query)
	}
	for _, expected := range []string{
		"from masteries force index (primary)",
		"where puuid > ? or (puuid = ? and champion_id > ?)",
		"order by puuid, champion_id limit ?",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("bounded source query does not contain %q: %s", expected, query)
		}
	}
	if strings.Contains(query, "(puuid, champion_id) >") {
		t.Fatalf("row-constructor cursor can degrade into a prefix scan on production MySQL: %s", query)
	}
}

func TestResetMasteryNumericShadowRequiresBothAcknowledgements(t *testing.T) {
	for _, acknowledgements := range [][2]bool{{false, false}, {true, false}, {false, true}} {
		if err := ResetMasteryNumericShadow(context.Background(), nil, acknowledgements[0], acknowledgements[1]); err == nil {
			t.Fatalf("reset accepted acknowledgements offline=%t reset=%t", acknowledgements[0], acknowledgements[1])
		}
	}
}

func TestMasteryNumericShadowTriggersMirrorLegacyWrites(t *testing.T) {
	triggers := masteryNumericShadowSyncTriggers()
	if len(triggers) != 3 {
		t.Fatalf("expected insert/update/delete shadow triggers, got %d", len(triggers))
	}
	definitions := strings.ToLower(strings.Join([]string{
		triggers[0].statement,
		triggers[1].statement,
		triggers[2].statement,
	}, " "))
	for _, expected := range []string{
		"after insert on masteries",
		"after update on masteries",
		"after delete on masteries",
		"masteries_numeric_v2",
		"summoner_numeric_keys",
	} {
		if !strings.Contains(definitions, expected) {
			t.Fatalf("shadow triggers do not contain %q: %s", expected, definitions)
		}
	}
}

func TestMasteryNumericShadowSchemaUsesCompactNumericKeys(t *testing.T) {
	statements := masteryNumericShadowSchemaStatements()
	if len(statements) == 0 {
		t.Fatal("missing mastery numeric shadow schema")
	}
	tableDDL := strings.ToLower(strings.Join(strings.Fields(statements[0]), " "))
	for _, expected := range []string{
		"summoner_fk bigint unsigned not null",
		"primary key (summoner_fk, champion_id)",
	} {
		if !strings.Contains(tableDDL, expected) {
			t.Fatalf("shadow DDL does not contain %q: %s", expected, tableDDL)
		}
	}
	if strings.Contains(tableDDL, "puuid") {
		t.Fatalf("compact shadow table must not store PUUID: %s", tableDDL)
	}
	if strings.Contains(tableDDL, "masteries_numeric_champion_points_level_covering_index") {
		t.Fatalf("covering index must be bulk-built only after the copy validates: %s", tableDDL)
	}
}

func TestGenericChildBackfillNeverUpdatesMasteriesInPlace(t *testing.T) {
	for _, spec := range numericKeyChildBackfillSpecs() {
		if spec.table == "masteries" {
			t.Fatal("masteries must be copied into the compact shadow table")
		}
	}
}
