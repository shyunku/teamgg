package statistics_models

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

type championDetailSourceTestResult int64

func (result championDetailSourceTestResult) LastInsertId() (int64, error) { return 0, nil }
func (result championDetailSourceTestResult) RowsAffected() (int64, error) { return int64(result), nil }

type championDetailSourceTestContext struct {
	rowCount    int64
	execQueries []string
	execArgs    [][]interface{}
}

func (context *championDetailSourceTestContext) Exec(query string, args ...interface{}) (sql.Result, error) {
	context.execQueries = append(context.execQueries, query)
	context.execArgs = append(context.execArgs, args)
	return championDetailSourceTestResult(1), nil
}

func (context *championDetailSourceTestContext) Get(destination interface{}, _ string, _ ...interface{}) error {
	rowCount, ok := destination.(*int64)
	if !ok {
		panic("unexpected get destination")
	}
	*rowCount = context.rowCount
	return nil
}

func (context *championDetailSourceTestContext) Select(interface{}, string, ...interface{}) error {
	panic("unexpected select")
}

func (context *championDetailSourceTestContext) Rebind(query string) string { return query }

func TestPrepareChampionDetailStatisticsSourceFiltersBeforeJoiningLargeTables(t *testing.T) {
	database := &championDetailSourceTestContext{rowCount: 10}
	versions := []string{"16.16.1", "16.15.1", "16.14.1"}
	if err := PrepareChampionDetailStatisticsSource(database, versions); err != nil {
		t.Fatal(err)
	}
	if len(database.execQueries) != 2 {
		t.Fatalf("expected truncate and one population statement, got %d", len(database.execQueries))
	}
	if strings.TrimSpace(database.execQueries[0]) != "TRUNCATE TABLE champion_detail_statistics_source" {
		t.Fatalf("unexpected staging reset: %s", database.execQueries[0])
	}
	query := database.execQueries[1]
	if strings.Contains(strings.ToUpper(query), "CREATE TEMPORARY TABLE") {
		t.Fatalf("population query recreated the legacy temporary-table pipeline: %s", query)
	}
	filterAt := strings.Index(query, "FROM matches FORCE INDEX (matches_game_version_index)")
	participantAt := strings.Index(query, "INNER JOIN match_participants")
	if filterAt < 0 || participantAt < 0 || filterAt > participantAt {
		t.Fatalf("recent match filtering does not precede participant expansion")
	}
	if !strings.Contains(query, "WHERE game_version IN (?, ?, ?)") {
		t.Fatalf("patch filter was not safely expanded: %s", query)
	}
	if !reflect.DeepEqual(database.execArgs[1], []interface{}{"16.16.1", "16.15.1", "16.14.1"}) {
		t.Fatalf("unexpected patch arguments: %#v", database.execArgs[1])
	}
}

func TestPrepareChampionDetailStatisticsSourceRejectsMissingVersions(t *testing.T) {
	database := &championDetailSourceTestContext{}
	if err := PrepareChampionDetailStatisticsSource(database, nil); err == nil {
		t.Fatal("missing patch versions must be rejected")
	}
	if len(database.execQueries) != 0 {
		t.Fatal("staging table was mutated without patch versions")
	}
}

func TestChampionDetailQueriesUseOneReusableStagingTable(t *testing.T) {
	for name, query := range map[string]string{
		"meta":    championDetailMetaQuery,
		"counter": championCounterQuery,
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(query, "FROM champion_detail_statistics_source") {
				t.Fatal("query does not use the filtered staging source")
			}
			for _, legacy := range []string{"RecentParticipants", "MainGroup", "FinalRankedMetas", "CounterGroup"} {
				if strings.Contains(query, legacy) {
					t.Fatalf("query still depends on legacy temporary table %s", legacy)
				}
			}
		})
	}
}
