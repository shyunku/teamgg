package statistics_models

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type championDetailSourceTestResult int64

func (result championDetailSourceTestResult) LastInsertId() (int64, error) { return 0, nil }
func (result championDetailSourceTestResult) RowsAffected() (int64, error) { return int64(result), nil }

type championDetailSourceTestContext struct {
	execQueries  []string
	execArgs     [][]interface{}
	selectCalls  int
	matchBatches [][]string
	cursor       string
	processed    int64
}

func (context *championDetailSourceTestContext) Exec(query string, args ...interface{}) (sql.Result, error) {
	context.execQueries = append(context.execQueries, query)
	context.execArgs = append(context.execArgs, args)
	return championDetailSourceTestResult(0), nil
}

func (context *championDetailSourceTestContext) Get(destination interface{}, query string, _ ...interface{}) error {
	switch value := destination.(type) {
	case *championDetailProgress:
		if context.cursor == "" {
			return sql.ErrNoRows
		}
		value.LastMatchId = context.cursor
		value.ProcessedMatches = context.processed
		return nil
	case *int64:
		if strings.Contains(query, championDetailBanSourceTable) {
			*value = 2
		} else {
			*value = 20
		}
		return nil
	default:
		return errors.New("unexpected get destination")
	}
}

func (context *championDetailSourceTestContext) Select(destination interface{}, _ string, _ ...interface{}) error {
	matchIds, ok := destination.(*[]string)
	if !ok {
		return errors.New("unexpected select destination")
	}
	if context.selectCalls < len(context.matchBatches) {
		*matchIds = append(*matchIds, context.matchBatches[context.selectCalls]...)
	}
	context.selectCalls++
	return nil
}

func (context *championDetailSourceTestContext) Rebind(query string) string { return query }

func TestPrepareIncrementalChampionDetailStatisticsSourceUsesBoundedCursorBatches(t *testing.T) {
	database := &championDetailSourceTestContext{matchBatches: [][]string{{"KR_1", "KR_2"}, {}}}
	result, err := PrepareIncrementalChampionDetailStatisticsSource(
		database,
		[]string{"16.16.1"},
		ChampionDetailSourceOptions{BatchSize: 2, CleanupSize: 100, WorkLimit: time.Minute},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready || result.ProcessedBatches != 1 || result.ProcessedMatches != 2 {
		t.Fatalf("unexpected preparation result: %+v", result)
	}
	for _, query := range database.execQueries {
		if strings.Contains(strings.ToUpper(query), "TRUNCATE") {
			t.Fatalf("incremental source must not truncate reusable data: %s", query)
		}
	}
	participantBatch := -1
	for index, query := range database.execQueries {
		if strings.Contains(query, "INSERT INTO champion_detail_statistics_participants") {
			participantBatch = index
			break
		}
	}
	if participantBatch < 0 {
		t.Fatal("participant batch was not populated")
	}
	if !reflect.DeepEqual(database.execArgs[participantBatch], []interface{}{"16.16.1", "KR_1", "KR_2"}) {
		t.Fatalf("unexpected bounded batch arguments: %#v", database.execArgs[participantBatch])
	}
	query := database.execQueries[participantBatch]
	for _, required := range []string{
		"game_version = ? AND match_id IN (?, ?)",
		"ON DUPLICATE KEY UPDATE",
		"LEFT JOIN match_participant_details",
	} {
		if !strings.Contains(query, required) {
			t.Fatalf("participant batch query is missing %q", required)
		}
	}
}

func TestPrepareIncrementalChampionDetailStatisticsSourceResumesAfterCursor(t *testing.T) {
	database := &championDetailSourceTestContext{
		cursor:       "KR_2",
		processed:    2,
		matchBatches: [][]string{{"KR_3"}, {}},
	}
	result, err := PrepareIncrementalChampionDetailStatisticsSource(
		database,
		[]string{"16.16.1"},
		ChampionDetailSourceOptions{BatchSize: 2, CleanupSize: 100, WorkLimit: time.Minute},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready || result.ProcessedMatches != 1 {
		t.Fatalf("unexpected resume result: %+v", result)
	}
	found := false
	for index, query := range database.execQueries {
		if strings.Contains(query, "INSERT INTO champion_detail_statistics_participants") {
			found = true
			if !reflect.DeepEqual(database.execArgs[index], []interface{}{"16.16.1", "KR_3"}) {
				t.Fatalf("resume did not honor cursor: %#v", database.execArgs[index])
			}
		}
	}
	if !found {
		t.Fatal("resumed participant batch was not populated")
	}
}

func TestPrepareIncrementalChampionDetailStatisticsSourceRejectsUnsafeOptions(t *testing.T) {
	database := &championDetailSourceTestContext{}
	for _, test := range []struct {
		name     string
		versions []string
		options  ChampionDetailSourceOptions
	}{
		{name: "versions", options: ChampionDetailSourceOptions{BatchSize: 1, CleanupSize: 1, WorkLimit: time.Second}},
		{name: "batch", versions: []string{"16.16.1"}, options: ChampionDetailSourceOptions{CleanupSize: 1, WorkLimit: time.Second}},
		{name: "cleanup", versions: []string{"16.16.1"}, options: ChampionDetailSourceOptions{BatchSize: 1, WorkLimit: time.Second}},
		{name: "duration", versions: []string{"16.16.1"}, options: ChampionDetailSourceOptions{BatchSize: 1, CleanupSize: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := PrepareIncrementalChampionDetailStatisticsSource(database, test.versions, test.options); err == nil {
				t.Fatal("invalid source configuration was accepted")
			}
		})
	}
}

func TestChampionDetailQueriesUseIncrementalSources(t *testing.T) {
	for name, query := range map[string]string{
		"meta":    championDetailMetaQuery,
		"counter": championCounterQuery,
	} {
		t.Run(name, func(t *testing.T) {
			if strings.Contains(query, "FROM matches ") || strings.Contains(query, "FROM match_participants") {
				t.Fatalf("%s query still scans raw match tables", name)
			}
			if name == "meta" || name == "counter" {
				if !strings.Contains(query, "FROM champion_detail_statistics_valid_builds") {
					t.Fatal("build query does not use the validated incremental view")
				}
			} else if !strings.Contains(query, "FROM champion_detail_statistics_participants") {
				t.Fatal("aggregate query does not use the incremental participant source")
			}
		})
	}
}

func TestChampionDetailExactMatchBatchesUsePrimaryIndex(t *testing.T) {
	for name, query := range map[string]string{
		"participants": populateChampionDetailParticipantBatchQuery,
		"bans":         populateChampionDetailBanBatchQuery,
		"processed":    markChampionDetailProcessedMatchesQuery,
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(query, "FORCE INDEX (PRIMARY)") {
				t.Fatalf("%s exact-match query does not force primary-key lookup", name)
			}
			if strings.Contains(query, "FORCE INDEX (matches_game_version_index)") {
				t.Fatalf("%s exact-match query still forces a full version range scan", name)
			}
		})
	}
}
