package statistics_models

import (
	"testing"
	"time"
)

func TestPrepareIncrementalChampionDetailStatisticsSourceBackfillsLateOlderMatch(t *testing.T) {
	database := &championDetailSourceTestContext{
		cursor:       "KR_9",
		processed:    9,
		matchBatches: [][]string{{}, {"KR_1"}, {}, {}},
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
		t.Fatalf("late older match was not backfilled: %+v", result)
	}
	if database.selectCalls < 4 {
		t.Fatalf("missing-match fallback was not checked: selectCalls=%d", database.selectCalls)
	}
}
