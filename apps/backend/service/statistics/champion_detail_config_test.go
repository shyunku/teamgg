package statistics

import "testing"

func TestChampionDetailBatchConfigurationBounds(t *testing.T) {
	t.Setenv("STATISTICS_CHAMPION_DETAIL_SOURCE_BATCH_SIZE", "9")
	if _, err := integerEnvironment("STATISTICS_CHAMPION_DETAIL_SOURCE_BATCH_SIZE", 250, 10, 5000); err == nil {
		t.Fatal("unsafe source batch size was accepted")
	}
	t.Setenv("STATISTICS_CHAMPION_DETAIL_SOURCE_BATCH_SIZE", "500")
	value, err := integerEnvironment("STATISTICS_CHAMPION_DETAIL_SOURCE_BATCH_SIZE", 250, 10, 5000)
	if err != nil || value != 500 {
		t.Fatalf("valid source batch size was rejected: value=%d err=%v", value, err)
	}
	t.Setenv("STATISTICS_CHAMPION_DETAIL_SOURCE_BATCH_SIZE", "invalid")
	if _, err := integerEnvironment("STATISTICS_CHAMPION_DETAIL_SOURCE_BATCH_SIZE", 250, 10, 5000); err == nil {
		t.Fatal("non-integer source batch size was accepted")
	}
}
