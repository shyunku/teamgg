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
	t.Setenv("MASTERY_NUMERIC_SHADOW_WORK_LIMIT", "5m")
	t.Setenv("MASTERY_NUMERIC_SHADOW_MAX_BATCHES", "3")
	t.Setenv("MASTERY_NUMERIC_SHADOW_OFFLINE_ACK", "true")
	t.Setenv("MASTERY_NUMERIC_SHADOW_DISABLE_BINLOG", "true")

	options := MasteryNumericShadowOptionsFromEnvironment()
	if options.BatchSize != 500 || options.WorkLimit != 5*time.Minute || options.MaxBatches != 3 ||
		!options.OfflineAcknowledged || !options.DisableBinlog {
		t.Fatalf("unexpected options: %+v", options)
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
		"(champion_id, champion_points desc, champion_level)",
	} {
		if !strings.Contains(tableDDL, expected) {
			t.Fatalf("shadow DDL does not contain %q: %s", expected, tableDDL)
		}
	}
	if strings.Contains(tableDDL, "puuid") {
		t.Fatalf("compact shadow table must not store PUUID: %s", tableDDL)
	}
}

func TestGenericChildBackfillNeverUpdatesMasteriesInPlace(t *testing.T) {
	for _, spec := range numericKeyChildBackfillSpecs() {
		if spec.table == "masteries" {
			t.Fatal("masteries must be copied into the compact shadow table")
		}
	}
}
