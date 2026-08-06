package models

import (
	"testing"
	"time"
)

func TestStatisticsSnapshotRemainingFreshness(t *testing.T) {
	tests := []struct {
		name      string
		fresh     bool
		remaining float64
		expected  time.Duration
	}{
		{name: "stale", fresh: false, remaining: 10, expected: 0},
		{name: "fresh at expiry boundary", fresh: true, remaining: 0, expected: time.Second},
		{name: "fresh fractional seconds", fresh: true, remaining: 1.25, expected: 1250 * time.Millisecond},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := StatisticsSnapshotDAO{
				Fresh:                 test.fresh,
				RemainingFreshSeconds: test.remaining,
			}
			if actual := snapshot.RemainingFreshness(); actual != test.expected {
				t.Fatalf("remaining freshness: got %s, want %s", actual, test.expected)
			}
		})
	}
}
