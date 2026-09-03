package models

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

type dataExplorerMetricsTestContext struct {
	tables     dataExplorerTableMetrics
	daily      dataExplorerMetricsDailyRow
	temp       dataExplorerTempSpace
	statuses   []dataExplorerGlobalStatusRow
	getQueries []string
	execCount  int
}

func (context *dataExplorerMetricsTestContext) Exec(_ string, _ ...interface{}) (sql.Result, error) {
	context.execCount++
	return dataExplorerTestResult(1), nil
}

func (context *dataExplorerMetricsTestContext) Get(destination interface{}, query string, _ ...interface{}) error {
	context.getQueries = append(context.getQueries, query)
	switch value := destination.(type) {
	case *dataExplorerTableMetrics:
		*value = context.tables
	case *dataExplorerMetricsDailyRow:
		*value = context.daily
	case *dataExplorerTempSpace:
		*value = context.temp
	default:
		panic("unexpected metrics get destination")
	}
	return nil
}

func (context *dataExplorerMetricsTestContext) Select(destination interface{}, _ string, _ ...interface{}) error {
	rows, ok := destination.(*[]dataExplorerGlobalStatusRow)
	if !ok {
		panic("unexpected metrics select destination")
	}
	*rows = append(*rows, context.statuses...)
	return nil
}

func (context *dataExplorerMetricsTestContext) Rebind(query string) string { return query }

func TestCollectDataExplorerOperationalMetricsUsesEstimatedRowsAndDailyBaseline(t *testing.T) {
	metricDate := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	database := &dataExplorerMetricsTestContext{
		tables: dataExplorerTableMetrics{
			SummonerRows: 120, MatchRows: 230, MasteryRows: 340,
			DatabaseBytes: 450, DatabaseFreeBytes: 50,
		},
		daily: dataExplorerMetricsDailyRow{
			MetricDate:           metricDate,
			BaselineSummonerRows: 100, CurrentSummonerRows: 120,
			BaselineMatchRows: 200, CurrentMatchRows: 230,
			BaselineMasteryRows: 300, CurrentMasteryRows: 340,
			BaselineQueueRows: 10, CurrentQueueRows: 15,
		},
		temp: dataExplorerTempSpace{
			Allocated: sql.NullInt64{Int64: 1024, Valid: true},
			Free:      sql.NullInt64{Int64: 256, Valid: true},
			FileCount: 1,
		},
		statuses: []dataExplorerGlobalStatusRow{
			{Name: "Created_tmp_tables", Value: "20"},
			{Name: "Created_tmp_disk_tables", Value: "4"},
		},
	}

	metrics, err := CollectDataExplorerOperationalMetrics(database, 15)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.DailySummonerRowGrowth != 20 || metrics.DailyMatchRowGrowth != 30 ||
		metrics.DailyMasteryRowGrowth != 40 || metrics.DailyQueueRowGrowth != 5 {
		t.Fatalf("unexpected daily growth: %+v", metrics)
	}
	if !metrics.TempStatusAvailable || !metrics.TempSpaceAvailable ||
		metrics.CreatedTempDiskTables != 4 || metrics.TempAllocatedBytes != 1024 {
		t.Fatalf("unexpected temp metrics: %+v", metrics)
	}
	if database.execCount != 1 {
		t.Fatalf("expected one daily snapshot upsert, got %d", database.execCount)
	}
	if len(database.getQueries) == 0 || strings.Contains(strings.ToUpper(database.getQueries[0]), "COUNT(") {
		t.Fatal("large table metrics must not use COUNT(*)")
	}
	if !strings.Contains(database.getQueries[0], "table_name = 'masteries_numeric_v2'") ||
		strings.Contains(database.getQueries[0], "table_name = 'masteries'") {
		t.Fatalf("mastery metrics must use numeric storage: %s", database.getQueries[0])
	}
}
