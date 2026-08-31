package models

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"
	"time"
)

type dataExplorerTestResult int64

func (result dataExplorerTestResult) LastInsertId() (int64, error) { return 0, nil }
func (result dataExplorerTestResult) RowsAffected() (int64, error) { return int64(result), nil }

func TestDataExplorerClaimQueriesForceOrderedIndexes(t *testing.T) {
	tests := map[string]string{
		"summoner": dataExplorerSummonerClaimQuery,
		"match":    dataExplorerMatchClaimQuery,
	}
	for name, query := range tests {
		normalized := strings.ToLower(strings.Join(strings.Fields(query), " "))
		if !strings.Contains(normalized, "force index") || !strings.Contains(normalized, "priority desc") {
			t.Fatalf("%s claim query does not force the ordered index: %s", name, normalized)
		}
		if !strings.Contains(normalized, "limit 1 for update skip locked") {
			t.Fatalf("%s claim query lost bounded locking: %s", name, normalized)
		}
	}
}

type dataExplorerTestContext struct {
	cursorId      string
	cursorDone    bool
	backfillRows  []dataExplorerProcessingStateBackfillRow
	cleanupRows   []dataExplorerSourceCleanupRow
	selectedAfter string
	execQueries   []string
	execArgs      [][]interface{}
	results       []sql.Result
}

func (context *dataExplorerTestContext) Exec(query string, args ...interface{}) (sql.Result, error) {
	context.execQueries = append(context.execQueries, query)
	context.execArgs = append(context.execArgs, args)
	if len(context.results) == 0 {
		return dataExplorerTestResult(0), nil
	}
	result := context.results[0]
	context.results = context.results[1:]
	return result, nil
}

func (context *dataExplorerTestContext) Get(destination interface{}, _ string, _ ...interface{}) error {
	value := reflect.ValueOf(destination).Elem()
	if field := value.FieldByName("Id"); field.IsValid() {
		field.SetString(context.cursorId)
	}
	if field := value.FieldByName("Completed"); field.IsValid() {
		field.SetBool(context.cursorDone)
	}
	if field := value.FieldByName("MatchId"); field.IsValid() {
		field.SetString(context.cursorId)
	}
	return nil
}

func (context *dataExplorerTestContext) Select(destination interface{}, _ string, args ...interface{}) error {
	switch rows := destination.(type) {
	case *[]dataExplorerProcessingStateBackfillRow:
		context.selectedAfter = args[0].(string)
		*rows = append(*rows, context.backfillRows...)
	case *[]dataExplorerSourceCleanupRow:
		*rows = append(*rows, context.cleanupRows...)
	default:
		panic("unexpected select destination")
	}
	return nil
}

func (context *dataExplorerTestContext) Rebind(query string) string { return query }

func TestBackfillDataExplorerProcessingStateUsesPersistedCursor(t *testing.T) {
	updated := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	database := &dataExplorerTestContext{
		cursorId: "p1",
		backfillRows: []dataExplorerProcessingStateBackfillRow{
			{EntityId: "p2", Updated: updated},
			{EntityId: "p3", Updated: updated.Add(time.Second)},
		},
		results: []sql.Result{dataExplorerTestResult(2), dataExplorerTestResult(1)},
	}

	inserted, err := backfillDataExplorerProcessingStateBatch(
		database,
		"summoner_processing_state_backfill",
		"data_explorer_summoner_jobs",
		"puuid",
		"data_explorer_summoner_processing_state",
		30*24*time.Hour,
		2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if inserted != 2 || database.selectedAfter != "p1" {
		t.Fatalf("unexpected cursor batch: inserted=%d selectedAfter=%q", inserted, database.selectedAfter)
	}
	if len(database.execQueries) != 2 || !strings.Contains(database.execQueries[0], "INSERT IGNORE") {
		t.Fatalf("unexpected backfill statements: %d", len(database.execQueries))
	}
	if got := database.execArgs[1][0]; got != "p3" {
		t.Fatalf("cursor must advance to the last key, got %v", got)
	}
}

func TestBackfillDataExplorerProcessingStateMarksCursorComplete(t *testing.T) {
	database := &dataExplorerTestContext{cursorId: "last"}
	inserted, err := backfillDataExplorerProcessingStateBatch(
		database,
		"match_processing_state_backfill",
		"data_explorer_match_jobs",
		"match_id",
		"data_explorer_match_processing_state",
		365*24*time.Hour,
		500,
	)
	if err != nil {
		t.Fatal(err)
	}
	if inserted != 0 || len(database.execQueries) != 1 || !strings.Contains(database.execQueries[0], "completed = 1") {
		t.Fatal("empty restart batch must persist completion")
	}
}

func TestCleanupDataExplorerCompletedRowsUsesBoundedDoneOnlyDeletes(t *testing.T) {
	database := &dataExplorerTestContext{
		cursorId: "KR-1",
		cleanupRows: []dataExplorerSourceCleanupRow{
			{MatchId: "KR-2", Puuid: "p1", Eligible: true},
			{MatchId: "KR-2", Puuid: "p2", Eligible: true},
			{MatchId: "KR-3", Puuid: "p3", Eligible: true},
		},
		results: []sql.Result{
			dataExplorerTestResult(3),
			dataExplorerTestResult(1),
			dataExplorerTestResult(4),
			dataExplorerTestResult(5),
		},
	}
	result, err := CleanupDataExplorerCompletedRows(database, 24*time.Hour, 48*time.Hour, 50)
	if err != nil {
		t.Fatal(err)
	}
	if result.MatchSources != 3 || result.SummonerJobs != 4 || result.MatchJobs != 5 {
		t.Fatalf("unexpected cleanup result: %+v", result)
	}
	if len(database.execQueries) != 4 {
		t.Fatalf("expected source delete/cursor update and two job deletes, got %d", len(database.execQueries))
	}
	for _, query := range database.execQueries[2:] {
		if !strings.Contains(query, "LIMIT ?") {
			t.Fatalf("cleanup query is not batch bounded: %s", query)
		}
	}
	if !strings.Contains(database.execQueries[0], "match_id = ? AND puuid = ?") {
		t.Fatal("source cleanup must delete only rows selected by the persisted cursor")
	}
	if database.execArgs[1][0] != "KR-3" || database.execArgs[1][1] != "p3" {
		t.Fatalf("source cursor did not advance to the last scanned row: %v", database.execArgs[1])
	}
	if !strings.Contains(database.execQueries[2], "status = 'done'") ||
		!strings.Contains(database.execQueries[3], "status = 'done'") {
		t.Fatal("job cleanup must only delete done jobs")
	}
}

func TestCleanupDataExplorerMatchSourcesAdvancesPastIneligibleRows(t *testing.T) {
	database := &dataExplorerTestContext{
		cursorId: "KR-1",
		cleanupRows: []dataExplorerSourceCleanupRow{
			{MatchId: "KR-2", Puuid: "p1", Eligible: false},
			{MatchId: "KR-2", Puuid: "p2", Eligible: false},
		},
		results: []sql.Result{dataExplorerTestResult(1)},
	}
	deleted, err := cleanupDataExplorerMatchSourceBatch(database, 24*time.Hour, 2)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 || len(database.execQueries) != 1 {
		t.Fatalf("ineligible rows should only advance the cursor: deleted=%d queries=%d", deleted, len(database.execQueries))
	}
	if database.execArgs[0][0] != "KR-2" || database.execArgs[0][1] != "p2" {
		t.Fatalf("source cursor stalled on ineligible rows: %v", database.execArgs[0])
	}
}
