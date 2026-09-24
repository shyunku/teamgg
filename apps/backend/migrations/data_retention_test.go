package migrations

import (
	"strings"
	"testing"
	"time"
)

func TestDataRetentionOptionsDefaultToDryRun(t *testing.T) {
	for _, key := range []string{
		"DATA_RETENTION_DRY_RUN", "DATA_RETENTION_DELETE_ACK", "DATA_RETENTION_OFFLINE_ACK",
		"DATA_RETENTION_MATCH_PATCHES", "DATA_RETENTION_BATCH_SIZE",
		"DATA_RETENTION_BATCH_TIMEOUT", "DATA_RETENTION_WORK_LIMIT",
		"DATA_RETENTION_ENFORCE_LOAD_GUARD", "DATA_RETENTION_MAX_THREADS_RUNNING",
		"DATA_RETENTION_MAX_LOCK_WAITS",
	} {
		t.Setenv(key, "")
	}
	options := DataRetentionOptionsFromEnvironment()
	if !options.DryRun || options.DeleteAcknowledged || options.OfflineAcknowledged {
		t.Fatalf("retention defaults are unsafe: %+v", options)
	}
	if options.RetainedPatches != 8 || options.BatchSize != 100 ||
		options.BatchTimeout != 2*time.Minute || options.WorkLimit != 10*time.Minute {
		t.Fatalf("unexpected retention defaults: %+v", options)
	}
	if options.EnforceLoadGuard || options.MaxThreadsRunning != 4 || options.MaxLockWaits != 0 {
		t.Fatalf("unexpected retention load guard defaults: %+v", options)
	}
}

func TestDataRetentionLoadGuardEnvironment(t *testing.T) {
	t.Setenv("DATA_RETENTION_ENFORCE_LOAD_GUARD", "true")
	t.Setenv("DATA_RETENTION_MAX_THREADS_RUNNING", "7")
	t.Setenv("DATA_RETENTION_MAX_LOCK_WAITS", "2")
	options := DataRetentionOptionsFromEnvironment()
	if !options.EnforceLoadGuard || options.MaxThreadsRunning != 7 || options.MaxLockWaits != 2 {
		t.Fatalf("unexpected retention load guard options: %+v", options)
	}
}

func TestClassifyRetentionMatchVersions(t *testing.T) {
	rows := []retentionVersionRow{
		{GameVersion: "16.17.1", Count: 2},
		{GameVersion: "16.17.2", Count: 3},
		{GameVersion: "16.16.1", Count: 5},
		{GameVersion: "16.15.1", Count: 7},
		{GameVersion: "invalid", Count: 100},
	}
	retained, expired, eligible := classifyRetentionMatchVersions(rows, 2)
	if strings.Join(retained, ",") != "16.17,16.16" {
		t.Fatalf("unexpected retained patches: %v", retained)
	}
	if strings.Join(expired, ",") != "16.15.1" || eligible != 7 {
		t.Fatalf("unexpected expired versions: versions=%v eligible=%d", expired, eligible)
	}
}

func TestRetentionDeletionRequiresBothAcknowledgements(t *testing.T) {
	_, err := CleanupRetainedData(t.Context(), nil, DataRetentionOptions{DryRun: false})
	if err == nil || !strings.Contains(err.Error(), "DELETE_ACK") || !strings.Contains(err.Error(), "OFFLINE_ACK") {
		t.Fatalf("destructive retention was not rejected: %v", err)
	}
}

func TestRetentionDeletesChildrenBeforeMatches(t *testing.T) {
	if len(retentionMatchDeleteStatements) == 0 {
		t.Fatal("retention delete plan is empty")
	}
	if retentionMatchDeleteStatements[0].Name != "match_participant_perk_style_selections" {
		t.Fatalf("deepest child is not deleted first: %s", retentionMatchDeleteStatements[0].Name)
	}
	if retentionMatchDeleteStatements[len(retentionMatchDeleteStatements)-1].Name != "matches" {
		t.Fatalf("matches must be deleted last: %s", retentionMatchDeleteStatements[len(retentionMatchDeleteStatements)-1].Name)
	}
}
