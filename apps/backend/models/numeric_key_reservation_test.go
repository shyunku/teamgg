package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestReserveMatchNumericKeysUsesDeterministicOrder(t *testing.T) {
	database := &dataExplorerTestContext{}
	err := ReserveMatchNumericKeys(database, "KR_1", []string{"z", "a", "z", "", "m"})
	if err != nil {
		t.Fatal(err)
	}
	if len(database.execQueries) != 2 {
		t.Fatalf("got %d reservation statements, want 2", len(database.execQueries))
	}
	if !strings.Contains(database.execQueries[0], "match_numeric_keys") || !reflect.DeepEqual(database.execArgs[0], []interface{}{"KR_1"}) {
		t.Fatalf("match key was not reserved first: query=%q args=%v", database.execQueries[0], database.execArgs[0])
	}
	if !strings.Contains(database.execQueries[1], "summoner_numeric_keys") {
		t.Fatalf("summoner keys were not reserved second: %q", database.execQueries[1])
	}
	if !reflect.DeepEqual(database.execArgs[1], []interface{}{"a", "m", "z"}) {
		t.Fatalf("summoner keys are not sorted and unique: %v", database.execArgs[1])
	}
}
