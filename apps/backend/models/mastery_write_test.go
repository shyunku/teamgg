package models

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

type masteryWriteTestContext struct {
	execQueries []string
	execArgs    [][]interface{}
	summonerFk  int64
}

func (context *masteryWriteTestContext) Exec(query string, args ...interface{}) (sql.Result, error) {
	context.execQueries = append(context.execQueries, query)
	context.execArgs = append(context.execArgs, args)
	return nil, nil
}

func (context *masteryWriteTestContext) Get(dest interface{}, _ string, _ ...interface{}) error {
	*(dest.(*int64)) = context.summonerFk
	return nil
}

func (context *masteryWriteTestContext) Select(interface{}, string, ...interface{}) error { return nil }
func (context *masteryWriteTestContext) Rebind(query string) string                       { return query }

func TestReplaceMasteriesWritesNumericAndDeletesStaleRows(t *testing.T) {
	database := &masteryWriteTestContext{summonerFk: 42}
	masteries := []*MasteryDAO{
		{Puuid: "puuid", ChampionId: 1},
		{Puuid: "puuid", ChampionId: 2},
	}

	if err := ReplaceMasteries(database, "puuid", masteries); err != nil {
		t.Fatal(err)
	}
	if len(database.execQueries) != 3 {
		t.Fatalf("unexpected query count: %d", len(database.execQueries))
	}
	for _, index := range []int{0, 1} {
		if !strings.Contains(database.execQueries[index], "INSERT INTO masteries_numeric_v2") {
			t.Fatalf("mastery write must target numeric storage: %#v", database.execQueries)
		}
	}
	if !strings.Contains(database.execQueries[2], "DELETE FROM masteries_numeric_v2") {
		t.Fatalf("stale mastery delete must target numeric storage: %#v", database.execQueries)
	}
	if !reflect.DeepEqual(database.execArgs[2], []interface{}{int64(42), int64(1), int64(2)}) {
		t.Fatalf("unexpected numeric delete args: %#v", database.execArgs[2])
	}
	for _, mastery := range masteries {
		if !mastery.SummonerFk.Valid || mastery.SummonerFk.Int64 != 42 {
			t.Fatalf("numeric key was not assigned: %#v", mastery.SummonerFk)
		}
	}
}

func TestReplaceMasteriesRejectsDuplicateChampion(t *testing.T) {
	database := &masteryWriteTestContext{summonerFk: 7}
	err := ReplaceMasteries(database, "puuid", []*MasteryDAO{
		{Puuid: "puuid", ChampionId: 1},
		{Puuid: "puuid", ChampionId: 1},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate champion") {
		t.Fatalf("expected duplicate champion error, got %v", err)
	}
}
