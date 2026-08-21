package service

import (
	"testing"
	"time"

	"team.gg-server/models"
)

func TestDataExplorerMetricsDefaults(t *testing.T) {
	for _, key := range []string{
		"DATA_EXPLORER_METRICS_ENABLED",
		"DATA_EXPLORER_METRICS_INTERVAL",
		"DATA_EXPLORER_ALERT_BUDGET_PERCENT",
		"DATA_EXPLORER_ALERT_SUMMONER_QUEUE",
		"DATA_EXPLORER_ALERT_MATCH_QUEUE",
		"DATA_EXPLORER_ALERT_FAILED_JOBS",
		"DATA_EXPLORER_ALERT_DATABASE_BYTES",
		"DATA_EXPLORER_ALERT_DAILY_ROW_GROWTH",
		"DATA_EXPLORER_ALERT_TEMP_DISK_PERCENT",
	} {
		t.Setenv(key, "")
	}
	explorer := NewDataExplorer()
	if !explorer.metricsEnabled || explorer.metricsInterval != 5*time.Minute {
		t.Fatalf("unexpected metrics defaults: enabled=%t interval=%s", explorer.metricsEnabled, explorer.metricsInterval)
	}
	if explorer.alertBudgetPercent != 80 || explorer.alertTempDiskPercent != 25 {
		t.Fatalf("unexpected percentage thresholds: budget=%d temp=%d", explorer.alertBudgetPercent, explorer.alertTempDiskPercent)
	}
	if explorer.alertSummonerQueue != 10000 || explorer.alertMatchQueue != 10000 ||
		explorer.alertFailedJobs != 100 || explorer.alertDailyRowGrowth != 1000000 {
		t.Fatal("unexpected operational alert defaults")
	}
	if explorer.alertDatabaseBytes != 0 {
		t.Fatal("database size alert must remain disabled until capacity is configured")
	}
}

func TestDataExplorerMetricsEnvironmentBounds(t *testing.T) {
	t.Setenv("DATA_EXPLORER_METRICS_INTERVAL", "1s")
	t.Setenv("DATA_EXPLORER_ALERT_BUDGET_PERCENT", "999")
	t.Setenv("DATA_EXPLORER_ALERT_TEMP_DISK_PERCENT", "0")
	t.Setenv("DATA_EXPLORER_ALERT_DATABASE_BYTES", "107374182400")

	explorer := NewDataExplorer()
	if explorer.metricsInterval != minExplorerMetricsInterval {
		t.Fatalf("metrics interval was not clamped: %s", explorer.metricsInterval)
	}
	if explorer.alertBudgetPercent != 100 {
		t.Fatalf("budget percentage was not clamped: %d", explorer.alertBudgetPercent)
	}
	if explorer.alertTempDiskPercent != 0 {
		t.Fatalf("zero must disable the temp disk alert: %d", explorer.alertTempDiskPercent)
	}
	if explorer.alertDatabaseBytes != 107374182400 {
		t.Fatalf("64-bit database threshold was not parsed: %d", explorer.alertDatabaseBytes)
	}
}

func TestExplorerEnvBytesAcceptsHumanReadableAndLegacyValues(t *testing.T) {
	tests := []struct {
		value string
		want  int64
	}{
		{value: "0", want: 0},
		{value: "200M", want: 200 * 1024 * 1024},
		{value: "5G", want: 5 * 1024 * 1024 * 1024},
		{value: "1.5GB", want: 1536 * 1024 * 1024},
		{value: "107374182400", want: 107374182400},
		{value: " 2 tb ", want: 2 * 1024 * 1024 * 1024 * 1024},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Setenv("TEST_EXPLORER_BYTES", test.value)
			if got := explorerEnvBytes("TEST_EXPLORER_BYTES", 123); got != test.want {
				t.Fatalf("explorerEnvBytes(%q) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestExplorerEnvBytesFallsBackForInvalidValues(t *testing.T) {
	for _, value := range []string{"-1G", "invalid", "1PB", "999999999999999999999T"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("TEST_EXPLORER_BYTES", value)
			if got := explorerEnvBytes("TEST_EXPLORER_BYTES", 123); got != 123 {
				t.Fatalf("invalid value %q returned %d", value, got)
			}
		})
	}
}

func TestDataExplorerTempStatusUsesIntervalDeltas(t *testing.T) {
	explorer := NewDataExplorer()
	first := &models.DataExplorerOperationalMetrics{
		TempStatusAvailable:   true,
		CreatedTempTables:     100,
		CreatedTempDiskTables: 20,
	}
	if _, _, available := explorer.tempStatusInterval(first); available {
		t.Fatal("the first cumulative status sample cannot produce an interval")
	}
	second := &models.DataExplorerOperationalMetrics{
		TempStatusAvailable:   true,
		CreatedTempTables:     140,
		CreatedTempDiskTables: 30,
	}
	tables, disk, available := explorer.tempStatusInterval(second)
	if !available || tables != 40 || disk != 10 {
		t.Fatalf("unexpected temp interval: available=%t tables=%d disk=%d", available, tables, disk)
	}
}

func TestDataExplorerMetricChecksCoverBudgetsQueuesGrowthAndStorage(t *testing.T) {
	explorer := NewDataExplorer()
	explorer.dailySummonerBudget = 100
	explorer.dailyMatchBudget = 200
	explorer.alertDatabaseBytes = 1000
	diagnostics := &models.DataExplorerDiagnostics{
		SummonerJobs: map[string]int64{models.DataExplorerJobPending: 10000, models.DataExplorerJobFailed: 60},
		MatchJobs:    map[string]int64{models.DataExplorerJobPending: 9999, models.DataExplorerJobFailed: 40},
		DailyUsage:   map[string]int64{explorerSummonerBudgetKind: 80, explorerMatchBudgetKind: 100},
	}
	metrics := &models.DataExplorerOperationalMetrics{
		DatabaseBytes:          1000,
		DailySummonerRowGrowth: 1000000,
		DailyMatchRowGrowth:    10,
		DailyMasteryRowGrowth:  20,
		DailyQueueRowGrowth:    30,
	}
	checks := explorer.metricChecks(diagnostics, metrics, 20, 5, true)
	values := make(map[string]dataExplorerMetricCheck, len(checks))
	for _, check := range checks {
		values[check.name] = check
	}
	for _, name := range []string{
		"summoner_queue_pending",
		"match_queue_pending",
		"failed_jobs",
		"database_bytes",
		"daily_summoner_growth_estimated",
		"summoner_budget_percent",
		"match_budget_percent",
		"temp_disk_percent",
	} {
		if _, exists := values[name]; !exists {
			t.Fatalf("missing metric check %s", name)
		}
	}
	if values["summoner_budget_percent"].value != 80 || values["match_budget_percent"].value != 50 {
		t.Fatal("daily usage was not converted to budget percentage")
	}
	if values["temp_disk_percent"].value != 25 {
		t.Fatal("temp disk ratio must use interval deltas")
	}
}
