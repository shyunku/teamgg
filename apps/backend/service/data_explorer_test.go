package service

import (
	"errors"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"team.gg-server/models"
)

func TestDataExplorerRetryDelayIsBounded(t *testing.T) {
	explorer := NewDataExplorer()
	if got := explorer.retryDelay(1); got != time.Second+137*time.Millisecond {
		t.Fatalf("unexpected first retry delay: %s", got)
	}
	if got, max := explorer.retryDelay(100), 128*time.Second+8*137*time.Millisecond; got != max {
		t.Fatalf("retry delay must be capped: got %s want %s", got, max)
	}
}

func TestDataExplorerEnabledEnvironment(t *testing.T) {
	for _, disabled := range []string{"", "0", "false", "OFF", "invalid"} {
		t.Run(disabled, func(t *testing.T) {
			t.Setenv("DATA_EXPLORER_ENABLED", disabled)
			if IsDataExplorerEnabled() {
				t.Fatalf("expected %q to disable DataExplorer", disabled)
			}
		})
	}
	t.Setenv("DATA_EXPLORER_ENABLED", "true")
	if !IsDataExplorerEnabled() {
		t.Fatal("expected true to enable DataExplorer")
	}
}

func TestDataExplorerUsesSafeExpansionDefaults(t *testing.T) {
	t.Setenv("DATA_EXPLORER_MAX_DEPTH", "")
	t.Setenv("DATA_EXPLORER_PARTICIPANT_DISCOVERY", "")
	t.Setenv("DATA_EXPLORER_BOOTSTRAP_ENABLED", "")

	explorer := NewDataExplorer()
	if explorer.maxDepth != 0 {
		t.Fatalf("expected max depth 0 by default, got %d", explorer.maxDepth)
	}
	if explorer.participantDiscovery != participantDiscoveryDisabled {
		t.Fatalf("expected participant discovery to be disabled, got %q", explorer.participantDiscovery)
	}
	if explorer.bootstrapEnabled {
		t.Fatal("expected participant bootstrap to be disabled by default")
	}
}

func TestDataExplorerDepthBoundary(t *testing.T) {
	t.Setenv("DATA_EXPLORER_PARTICIPANT_DISCOVERY", "bounded")
	t.Setenv("DATA_EXPLORER_MAX_DEPTH", "2")
	explorer := NewDataExplorer()

	for _, test := range []struct {
		depth int
		want  bool
	}{
		{depth: -1, want: false},
		{depth: 0, want: true},
		{depth: 1, want: true},
		{depth: 2, want: false},
		{depth: 3, want: false},
	} {
		if got := explorer.shouldDiscoverMatchParticipants(test.depth); got != test.want {
			t.Fatalf("depth %d: got %t want %t", test.depth, got, test.want)
		}
	}
}

func TestDataExplorerDisabledDiscoveryOverridesDepth(t *testing.T) {
	t.Setenv("DATA_EXPLORER_PARTICIPANT_DISCOVERY", "disabled")
	t.Setenv("DATA_EXPLORER_MAX_DEPTH", "10")
	if NewDataExplorer().shouldDiscoverMatchParticipants(0) {
		t.Fatal("disabled participant discovery must override max depth")
	}
}

func TestDataExplorerDepthIsBoundedFromEnvironment(t *testing.T) {
	t.Setenv("DATA_EXPLORER_MAX_DEPTH", "999")
	if got := NewDataExplorer().maxDepth; got != maxSupportedExplorerDepth {
		t.Fatalf("expected max depth clamp %d, got %d", maxSupportedExplorerDepth, got)
	}
	t.Setenv("DATA_EXPLORER_MAX_DEPTH", "-1")
	if got := NewDataExplorer().maxDepth; got != 0 {
		t.Fatalf("expected invalid negative depth to use safe default, got %d", got)
	}
}

func TestDataExplorerBudgetsRemainIndependent(t *testing.T) {
	t.Setenv("DATA_EXPLORER_DAILY_SUMMONER_BUDGET", "7")
	t.Setenv("DATA_EXPLORER_DAILY_MATCH_BUDGET", "11")
	explorer := NewDataExplorer()
	if explorer.dailySummonerBudget != 7 || explorer.dailyMatchBudget != 11 {
		t.Fatalf(
			"unexpected independent budgets: summoner=%d match=%d",
			explorer.dailySummonerBudget, explorer.dailyMatchBudget,
		)
	}
}

func TestDataExplorerRetentionDefaultsAreSafe(t *testing.T) {
	t.Setenv("DATA_EXPLORER_CLEANUP_ENABLED", "")
	t.Setenv("DATA_EXPLORER_CLEANUP_INTERVAL", "")
	t.Setenv("DATA_EXPLORER_CLEANUP_BATCH_SIZE", "")
	t.Setenv("DATA_EXPLORER_COMPLETED_JOB_RETENTION", "")
	t.Setenv("DATA_EXPLORER_SOURCE_RETENTION", "")
	t.Setenv("DATA_EXPLORER_SUMMONER_REVISIT_INTERVAL", "")
	t.Setenv("DATA_EXPLORER_MATCH_REVISIT_INTERVAL", "")

	explorer := NewDataExplorer()
	if explorer.cleanupEnabled {
		t.Fatal("destructive cleanup must be opt-in")
	}
	if explorer.cleanupInterval != 30*time.Second || explorer.cleanupBatchSize != 500 {
		t.Fatalf("unexpected cleanup pacing: interval=%s batch=%d", explorer.cleanupInterval, explorer.cleanupBatchSize)
	}
	if explorer.completedRetention != 24*time.Hour || explorer.sourceRetention != 24*time.Hour {
		t.Fatalf(
			"unexpected retention: completed=%s sources=%s",
			explorer.completedRetention,
			explorer.sourceRetention,
		)
	}
	if explorer.summonerRevisit != 30*24*time.Hour || explorer.matchRevisit != 365*24*time.Hour {
		t.Fatalf(
			"unexpected revisit windows: summoner=%s match=%s",
			explorer.summonerRevisit,
			explorer.matchRevisit,
		)
	}
}

func TestDataExplorerRetentionEnvironmentOverrides(t *testing.T) {
	t.Setenv("DATA_EXPLORER_CLEANUP_ENABLED", "true")
	t.Setenv("DATA_EXPLORER_CLEANUP_INTERVAL", "2m")
	t.Setenv("DATA_EXPLORER_CLEANUP_BATCH_SIZE", "25")
	t.Setenv("DATA_EXPLORER_COMPLETED_JOB_RETENTION", "48h")
	t.Setenv("DATA_EXPLORER_SOURCE_RETENTION", "72h")
	t.Setenv("DATA_EXPLORER_SUMMONER_REVISIT_INTERVAL", "168h")
	t.Setenv("DATA_EXPLORER_MATCH_REVISIT_INTERVAL", "720h")

	explorer := NewDataExplorer()
	if !explorer.cleanupEnabled || explorer.cleanupInterval != 2*time.Minute || explorer.cleanupBatchSize != 25 {
		t.Fatalf(
			"cleanup overrides not applied: enabled=%t interval=%s batch=%d",
			explorer.cleanupEnabled,
			explorer.cleanupInterval,
			explorer.cleanupBatchSize,
		)
	}
	if explorer.completedRetention != 48*time.Hour || explorer.sourceRetention != 72*time.Hour {
		t.Fatal("retention overrides not applied")
	}
	if explorer.summonerRevisit != 168*time.Hour || explorer.matchRevisit != 720*time.Hour {
		t.Fatal("revisit overrides not applied")
	}
}

func TestDataExplorerRetentionEnvironmentHasSafetyBounds(t *testing.T) {
	t.Setenv("DATA_EXPLORER_CLEANUP_INTERVAL", "1ms")
	t.Setenv("DATA_EXPLORER_CLEANUP_BATCH_SIZE", "999999")
	t.Setenv("DATA_EXPLORER_COMPLETED_JOB_RETENTION", "1s")
	t.Setenv("DATA_EXPLORER_SOURCE_RETENTION", "1s")
	t.Setenv("DATA_EXPLORER_SUMMONER_REVISIT_INTERVAL", "1s")
	t.Setenv("DATA_EXPLORER_MATCH_REVISIT_INTERVAL", "1s")

	explorer := NewDataExplorer()
	if explorer.cleanupInterval != minExplorerCleanupInterval {
		t.Fatalf("cleanup interval was not clamped: %s", explorer.cleanupInterval)
	}
	if explorer.cleanupBatchSize != maxExplorerCleanupBatch {
		t.Fatalf("cleanup batch was not clamped: %d", explorer.cleanupBatchSize)
	}
	if explorer.completedRetention != minExplorerRetention || explorer.sourceRetention != minExplorerRetention {
		t.Fatal("retention window was not clamped")
	}
	if explorer.summonerRevisit != minExplorerRevisit || explorer.matchRevisit != minExplorerRevisit {
		t.Fatal("revisit window was not clamped")
	}
}

func TestDataExplorerRestartKeepsPersistedDepthBoundary(t *testing.T) {
	t.Setenv("DATA_EXPLORER_PARTICIPANT_DISCOVERY", "bounded")
	t.Setenv("DATA_EXPLORER_MAX_DEPTH", "1")
	recoveredJob := &models.DataExplorerMatchJobDAO{Depth: 1}

	beforeRestart := NewDataExplorer()
	afterRestart := NewDataExplorer()
	if beforeRestart.shouldDiscoverMatchParticipants(recoveredJob.Depth) ||
		afterRestart.shouldDiscoverMatchParticipants(recoveredJob.Depth) {
		t.Fatal("a recovered job at max depth must not expand after restart")
	}
}

func TestRetryableDatabaseErrors(t *testing.T) {
	for _, number := range []uint16{1062, 1205, 1213} {
		if !isRetryableDatabaseError(&mysqlDriver.MySQLError{Number: number}) {
			t.Fatalf("mysql error %d must be retryable", number)
		}
	}
	if isRetryableDatabaseError(&mysqlDriver.MySQLError{Number: 1045}) {
		t.Fatal("authentication errors must not be retried")
	}
	if isRetryableDatabaseError(errors.New("plain error")) {
		t.Fatal("non-MySQL errors must not be treated as database races")
	}
}
