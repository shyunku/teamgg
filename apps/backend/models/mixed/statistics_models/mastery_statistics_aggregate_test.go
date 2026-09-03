package statistics_models

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"
	"time"
)

type masteryStatisticsTestResult int64

func (result masteryStatisticsTestResult) LastInsertId() (int64, error) { return 0, nil }
func (result masteryStatisticsTestResult) RowsAffected() (int64, error) { return int64(result), nil }

type masteryStatisticsTestContext struct {
	cutoff        time.Time
	dirty         []int
	aggregates    map[int]MasteryStatisticsMXDAO
	statistics    []*MasteryStatisticsMXDAO
	rankers       []*MasteryStatisticsTopRankersMXDAO
	selectCalls   int
	selectQueries []string
	selectArgs    [][]interface{}
	getQueries    []string
	execQueries   []string
	execArgs      [][]interface{}
}

func (context *masteryStatisticsTestContext) Exec(query string, args ...interface{}) (sql.Result, error) {
	context.execQueries = append(context.execQueries, query)
	context.execArgs = append(context.execArgs, args)
	return masteryStatisticsTestResult(1), nil
}

func (context *masteryStatisticsTestContext) Get(destination interface{}, query string, args ...interface{}) error {
	context.getQueries = append(context.getQueries, query)
	switch value := destination.(type) {
	case *time.Time:
		*value = context.cutoff
	case *MasteryStatisticsMXDAO:
		championID := args[len(args)-1].(int)
		*value = context.aggregates[championID]
	default:
		panic("unexpected get destination")
	}
	return nil
}

func (context *masteryStatisticsTestContext) Select(destination interface{}, query string, args ...interface{}) error {
	context.selectCalls++
	context.selectQueries = append(context.selectQueries, query)
	context.selectArgs = append(context.selectArgs, args)
	switch rows := destination.(type) {
	case *[]int:
		*rows = append(*rows, context.dirty...)
	case *[]*MasteryStatisticsMXDAO:
		*rows = append(*rows, context.statistics...)
	case *[]*MasteryStatisticsTopRankersMXDAO:
		*rows = append(*rows, context.rankers...)
	default:
		panic("unexpected select destination")
	}
	return nil
}

func (context *masteryStatisticsTestContext) Rebind(query string) string { return query }

func TestRefreshDirtyMasteryStatisticsAggregatesIsBoundedAndRestartSafe(t *testing.T) {
	cutoff := time.Date(2026, 8, 30, 4, 30, 0, 0, time.UTC)
	database := &masteryStatisticsTestContext{
		cutoff: cutoff,
		dirty:  []int{1, 2},
		aggregates: map[int]MasteryStatisticsMXDAO{
			1: {ChampionId: 1, MaxMastery: 900000, TotalMastery: 1200000, MasteredCount: 2, Count: 3},
			2: {ChampionId: 2},
		},
	}

	result, err := RefreshDirtyMasteryStatisticsAggregates(database, maxMasteryStatisticsBatch+1)
	if err != nil {
		t.Fatal(err)
	}
	if result.DirtyChampions != 2 || result.Refreshed != 1 || result.Removed != 1 {
		t.Fatalf("unexpected refresh result: %+v", result)
	}
	if got := database.selectArgs[0][1]; got != maxMasteryStatisticsBatch {
		t.Fatalf("unsafe batch was not clamped: %v", got)
	}
	if len(database.execQueries) != 4 {
		t.Fatalf("expected save/delete and acknowledgement per champion, got %d executions", len(database.execQueries))
	}
	for index := 1; index < len(database.execQueries); index += 2 {
		if !strings.Contains(database.execQueries[index], "dirty_at < ?") {
			t.Fatalf("acknowledgement does not preserve concurrent writes: %s", database.execQueries[index])
		}
		if !reflect.DeepEqual(database.execArgs[index], []interface{}{database.dirty[index/2], cutoff}) {
			t.Fatalf("unexpected acknowledgement args: %#v", database.execArgs[index])
		}
	}
	for _, query := range database.getQueries[1:] {
		if !strings.Contains(query, "FORCE INDEX (masteries_numeric_champion_points_level_covering_index)") ||
			!strings.Contains(query, "WHERE champion_id = ?") {
			t.Fatalf("aggregate query is not a bounded covering-index scan: %s", query)
		}
	}
}

func TestGetMasteryStatisticsReadsMaterializedAggregates(t *testing.T) {
	database := &masteryStatisticsTestContext{}
	if _, err := GetMasteryStatisticsMXDAOs(database); err != nil {
		t.Fatal(err)
	}
	query := database.selectQueries[0]
	if !strings.Contains(query, "FROM mastery_statistics_aggregates") || strings.Contains(query, "GROUP BY") {
		t.Fatalf("statistics query is not materialized: %s", query)
	}
}

func TestGetMasteryStatisticsTopRankersUsesBoundedCoveringIndexLookup(t *testing.T) {
	database := &masteryStatisticsTestContext{
		dirty: []int{1},
		rankers: []*MasteryStatisticsTopRankersMXDAO{
			{Puuid: "p1", ChampionId: 1, ChampionPoints: 100},
		},
	}
	rankers, err := GetMasteryStatisticsTopRankersMXDAOs(database, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(rankers) != 1 || rankers[0].Ranks != 1 {
		t.Fatalf("unexpected rankers: %#v", rankers)
	}
	if !strings.Contains(database.selectQueries[0], "FROM mastery_statistics_aggregates") {
		t.Fatalf("champion key set does not use aggregates: %s", database.selectQueries[0])
	}
	if !strings.Contains(database.selectQueries[1], "FORCE INDEX (masteries_numeric_champion_points_level_covering_index)") ||
		!strings.Contains(database.selectQueries[1], "WHERE m.champion_id = ?") {
		t.Fatalf("ranker query is not bounded by champion: %s", database.selectQueries[1])
	}
}

func TestMasteryStatisticsUsesNumericStorage(t *testing.T) {
	database := &masteryStatisticsTestContext{
		cutoff: time.Now(),
		dirty:  []int{1},
		aggregates: map[int]MasteryStatisticsMXDAO{
			1: {ChampionId: 1, Count: 1},
		},
		rankers: []*MasteryStatisticsTopRankersMXDAO{{ChampionId: 1}},
	}
	if _, err := RefreshDirtyMasteryStatisticsAggregates(database, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(database.getQueries[1], "FROM masteries_numeric_v2") ||
		!strings.Contains(database.getQueries[1], "masteries_numeric_champion_points_level_covering_index") {
		t.Fatalf("numeric aggregate query is unexpected: %s", database.getQueries[1])
	}

	database.selectQueries = nil
	if _, err := GetMasteryStatisticsTopRankersMXDAOs(database, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(database.selectQueries[1], "FROM masteries_numeric_v2") ||
		!strings.Contains(database.selectQueries[1], "m.summoner_fk = numeric_key.summoner_id") ||
		!strings.Contains(database.selectQueries[1], "numeric_key.puuid = s.puuid") {
		t.Fatalf("numeric ranker query is unexpected: %s", database.selectQueries[1])
	}
}
