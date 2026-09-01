package migrations

import (
	"strings"
	"testing"
)

func TestNumericKeyChildMigrationCoversCoreRelationshipTables(t *testing.T) {
	expectedColumns := map[string][]string{
		"masteries":                     {"summoner_fk"},
		"leagues":                       {"summoner_fk"},
		"summoner_matches":              {"summoner_fk", "match_fk"},
		"match_participant_details":     {"match_participant_fk", "match_fk"},
		"match_participant_perks":       {"match_participant_fk"},
		"match_participant_perk_styles": {"match_participant_fk"},
		"match_teams":                   {"match_fk"},
		"match_team_bans":               {"match_fk"},
	}
	actual := make(map[string][]string)
	for _, column := range numericKeyChildColumns {
		actual[column.table] = append(actual[column.table], column.column)
	}
	if len(actual) != len(expectedColumns) {
		t.Fatalf("got %d child tables, want %d", len(actual), len(expectedColumns))
	}
	for table, expected := range expectedColumns {
		got := actual[table]
		if strings.Join(got, ",") != strings.Join(expected, ",") {
			t.Fatalf("%s columns: got %v, want %v", table, got, expected)
		}
	}
}

func TestNumericKeyChildTriggersDualWriteInsertAndUpdate(t *testing.T) {
	triggers := numericKeyChildTriggers()
	if len(triggers) != 16 {
		t.Fatalf("got %d triggers, want 16", len(triggers))
	}
	seen := make(map[string]struct{}, len(triggers))
	all := make([]string, 0, len(triggers))
	for _, trigger := range triggers {
		if _, exists := seen[trigger.name]; exists {
			t.Fatalf("duplicate trigger %s", trigger.name)
		}
		seen[trigger.name] = struct{}{}
		all = append(all, strings.ToLower(strings.Join(strings.Fields(trigger.statement), " ")))
	}
	joined := strings.Join(all, "\n")
	for _, expected := range []string{
		"before insert on masteries", "before update on masteries",
		"before insert on summoner_matches", "set new.summoner_fk = last_insert_id()",
		"set new.match_fk = last_insert_id()",
		"before insert on match_participant_details", "set new.match_participant_fk = last_insert_id()",
		"before insert on match_participant_perk_styles",
		"before insert on match_team_bans",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("child triggers do not contain %q", expected)
		}
	}
	if strings.Contains(joined, "signal sqlstate") {
		t.Fatal("child dual-write triggers must not reject the existing write order")
	}
}

func TestNumericKeyChildBackfillSpecsAreBoundedAndOrdered(t *testing.T) {
	expected := []string{
		"leagues", "summoner_matches", "match_teams", "match_team_bans",
		"match_participant_details", "match_participant_perks",
		"match_participant_perk_styles",
	}
	specs := numericKeyChildBackfillSpecs()
	if len(specs) != len(expected) {
		t.Fatalf("got %d specs, want %d", len(specs), len(expected))
	}
	for index, entity := range expected {
		if specs[index].entity != entity {
			t.Fatalf("spec %d: got %s, want %s", index, specs[index].entity, entity)
		}
	}
	for _, spec := range specs {
		if spec.table == "masteries" {
			t.Fatal("masteries must use the compact shadow copy, not in-place UPDATE backfill")
		}
	}
}
