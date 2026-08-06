package v1

import (
	"team.gg-server/models"
	"testing"
)

func TestReplayAnalysisStatusTransitionAllowed(t *testing.T) {
	tests := []struct {
		current string
		next    string
		allowed bool
	}{
		{models.ReplayAnalysisStatusQueued, models.ReplayAnalysisStatusUploading, true},
		{models.ReplayAnalysisStatusQueued, models.ReplayAnalysisStatusFailed, true},
		{models.ReplayAnalysisStatusUploading, models.ReplayAnalysisStatusAnalyzing, true},
		{models.ReplayAnalysisStatusAnalyzing, models.ReplayAnalysisStatusCompleted, true},
		{models.ReplayAnalysisStatusAnalyzing, models.ReplayAnalysisStatusFailed, true},
		{models.ReplayAnalysisStatusCompleted, models.ReplayAnalysisStatusCompleted, true},
		{models.ReplayAnalysisStatusCompleted, models.ReplayAnalysisStatusAnalyzing, false},
		{models.ReplayAnalysisStatusFailed, models.ReplayAnalysisStatusUploading, false},
		{models.ReplayAnalysisStatusQueued, models.ReplayAnalysisStatusCompleted, false},
	}
	for _, test := range tests {
		if actual := replayAnalysisStatusTransitionAllowed(test.current, test.next); actual != test.allowed {
			t.Fatalf("transition %s -> %s: got %v, want %v", test.current, test.next, actual, test.allowed)
		}
	}
}
