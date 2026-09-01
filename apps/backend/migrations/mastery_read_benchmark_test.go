package migrations

import (
	"database/sql"
	"testing"
	"time"
)

func TestMasteryReadBenchmarkOptionsAreBounded(t *testing.T) {
	if got := boundedMasteryBenchmarkOption("0", 5, 1, 20); got != 5 {
		t.Fatalf("invalid lower bound returned %d", got)
	}
	if got := boundedMasteryBenchmarkOption("21", 5, 1, 20); got != 5 {
		t.Fatalf("invalid upper bound returned %d", got)
	}
	if got := boundedMasteryBenchmarkOption("7", 5, 1, 20); got != 7 {
		t.Fatalf("valid option returned %d", got)
	}
}

func TestSummarizeBenchmarkTimings(t *testing.T) {
	timing := summarizeBenchmarkTimings([]time.Duration{
		5 * time.Millisecond,
		1 * time.Millisecond,
		3 * time.Millisecond,
		2 * time.Millisecond,
		4 * time.Millisecond,
	})
	if timing.Samples != 5 || timing.P50Ms != 3 || timing.P95Ms != 4 || timing.MeanMs != 3 {
		t.Fatalf("unexpected timing summary: %+v", timing)
	}
}

func TestMasteryBenchmarkQueriesUseDistinctStorage(t *testing.T) {
	if legacyMasteryLookupQuery == numericMasteryLookupQuery ||
		legacyMasteryAggregateQuery == numericMasteryAggregateQuery ||
		legacyMasteryTopRankersQuery == numericMasteryTopRankersQuery {
		t.Fatal("legacy and numeric benchmark queries must remain distinct")
	}
}

func TestMasteryBenchmarkRowsIgnoreTransitionalNumericKey(t *testing.T) {
	legacy := []masteryBenchmarkRow{{ChampionID: 1, ChampionPoints: 100}}
	numeric := []masteryBenchmarkRow{{
		ChampionID:     1,
		ChampionPoints: 100,
		SummonerFK:     sql.NullInt64{Int64: 42, Valid: true},
	}}
	if !equalMasteryBenchmarkRows(legacy, numeric) {
		t.Fatal("transitional legacy summoner_fk must not affect payload equality")
	}
}
