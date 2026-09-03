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

func TestReplaceMasteriesWritesNumericFirstAndDeletesStaleRows(t *testing.T) {
	t.Cleanup(func() { _ = ConfigureMasteryWriteSource(MasteryWriteSourceLegacy) })
	if err := ConfigureMasteryWriteSource(MasteryWriteSourceNumericV2); err != nil {
		t.Fatal(err)
	}
	database := &masteryWriteTestContext{summonerFk: 42}
	masteries := []*MasteryDAO{
		{Puuid: "puuid", ChampionId: 1},
		{Puuid: "puuid", ChampionId: 2},
	}

	if err := ReplaceMasteries(database, "puuid", masteries); err != nil {
		t.Fatal(err)
	}
	if len(database.execQueries) != 6 {
		t.Fatalf("unexpected query count: %d", len(database.execQueries))
	}
	for _, pair := range [][2]int{{0, 1}, {2, 3}} {
		if !strings.Contains(database.execQueries[pair[0]], "INSERT INTO masteries_numeric_v2") ||
			!strings.Contains(database.execQueries[pair[1]], "INSERT INTO masteries") {
			t.Fatalf("numeric write must precede legacy mirror: %#v", database.execQueries)
		}
	}
	if !strings.Contains(database.execQueries[4], "DELETE FROM masteries_numeric_v2") ||
		!strings.Contains(database.execQueries[5], "DELETE FROM masteries") {
		t.Fatalf("numeric delete must precede legacy mirror: %#v", database.execQueries)
	}
	if !reflect.DeepEqual(database.execArgs[4], []interface{}{int64(42), int64(1), int64(2)}) {
		t.Fatalf("unexpected numeric delete args: %#v", database.execArgs[4])
	}
	for _, mastery := range masteries {
		if !mastery.SummonerFk.Valid || mastery.SummonerFk.Int64 != 42 {
			t.Fatalf("numeric key was not assigned: %#v", mastery.SummonerFk)
		}
	}
}

func TestReplaceMasteriesLegacyRollbackWritesLegacyFirst(t *testing.T) {
	t.Cleanup(func() { _ = ConfigureMasteryWriteSource(MasteryWriteSourceLegacy) })
	if err := ConfigureMasteryWriteSource(MasteryWriteSourceLegacy); err != nil {
		t.Fatal(err)
	}
	database := &masteryWriteTestContext{summonerFk: 7}

	if err := ReplaceMasteries(database, "puuid", []*MasteryDAO{{Puuid: "puuid", ChampionId: 1}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(database.execQueries[0], "INSERT INTO masteries") ||
		strings.Contains(database.execQueries[0], "masteries_numeric_v2") ||
		!strings.Contains(database.execQueries[1], "INSERT INTO masteries_numeric_v2") {
		t.Fatalf("legacy write must precede numeric mirror: %#v", database.execQueries)
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
