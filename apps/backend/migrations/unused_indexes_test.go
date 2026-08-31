package migrations

import (
	"reflect"
	"strings"
	"testing"
)

func TestTask63RemovableIndexesAreExplicit(t *testing.T) {
	expected := []removableIndex{
		{"match_participant_perk_styles", "match_participant_perk_styles_description_index", []string{"description"}},
		{"masteries", "masteries_champion_id_champion_points_index", []string{"champion_id", "champion_points"}},
		{"match_participants", "match_participants_participant_id_index", []string{"participant_id"}},
		{"match_participants", "match_participants_team_position_index", []string{"team_position"}},
	}
	if !reflect.DeepEqual(task63RemovableIndexes, expected) {
		t.Fatalf("unexpected removable indexes: %#v", task63RemovableIndexes)
	}
}

func TestIndexColumnNamesMatch(t *testing.T) {
	if !indexColumnNamesMatch([]string{"Champion_ID", "champion_points"}, []string{"champion_id", "champion_points"}) {
		t.Fatal("column comparison should be case-insensitive")
	}
	if indexColumnNamesMatch([]string{"champion_id"}, []string{"champion_id", "champion_points"}) {
		t.Fatal("different column counts must not match")
	}
	if indexColumnNamesMatch([]string{"champion_points", "champion_id"}, []string{"champion_id", "champion_points"}) {
		t.Fatal("different column order must not match")
	}
}

func TestUnusedIndexDropStatementUsesOnlineDDL(t *testing.T) {
	statement := unusedIndexDropStatement(task63RemovableIndexes[0])
	for _, expected := range []string{
		"ALTER TABLE `match_participant_perk_styles`",
		"DROP INDEX `match_participant_perk_styles_description_index`",
		"ALGORITHM=INPLACE",
		"LOCK=NONE",
	} {
		if !strings.Contains(statement, expected) {
			t.Fatalf("statement %q does not contain %q", statement, expected)
		}
	}
}
