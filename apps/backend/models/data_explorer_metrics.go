package models

import (
	"database/sql"
	"strconv"
	"team.gg-server/libs/db"
	"time"
)

type DataExplorerOperationalMetrics struct {
	MetricDate             time.Time
	SummonerRows           int64
	MatchRows              int64
	MasteryRows            int64
	QueueRows              int64
	DailySummonerRowGrowth int64
	DailyMatchRowGrowth    int64
	DailyMasteryRowGrowth  int64
	DailyQueueRowGrowth    int64
	DatabaseBytes          int64
	DatabaseFreeBytes      int64
	CreatedTempTables      int64
	CreatedTempDiskTables  int64
	TempStatusAvailable    bool
	TempAllocatedBytes     int64
	TempFreeBytes          int64
	TempSpaceAvailable     bool
}

type dataExplorerTableMetrics struct {
	SummonerRows      int64 `db:"summoner_rows"`
	MatchRows         int64 `db:"match_rows"`
	MasteryRows       int64 `db:"mastery_rows"`
	DatabaseBytes     int64 `db:"database_bytes"`
	DatabaseFreeBytes int64 `db:"database_free_bytes"`
}

type dataExplorerMetricsDailyRow struct {
	MetricDate           time.Time `db:"metric_date"`
	BaselineSummonerRows int64     `db:"baseline_summoner_rows"`
	CurrentSummonerRows  int64     `db:"current_summoner_rows"`
	BaselineMatchRows    int64     `db:"baseline_match_rows"`
	CurrentMatchRows     int64     `db:"current_match_rows"`
	BaselineMasteryRows  int64     `db:"baseline_mastery_rows"`
	CurrentMasteryRows   int64     `db:"current_mastery_rows"`
	BaselineQueueRows    int64     `db:"baseline_queue_rows"`
	CurrentQueueRows     int64     `db:"current_queue_rows"`
}

type dataExplorerGlobalStatusRow struct {
	Name  string `db:"Variable_name"`
	Value string `db:"Value"`
}

type dataExplorerTempSpace struct {
	Allocated sql.NullInt64 `db:"allocated_bytes"`
	Free      sql.NullInt64 `db:"free_bytes"`
	FileCount int64         `db:"file_count"`
}

func EnsureDataExplorerMetricsSchema(database db.Context) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS data_explorer_metrics_daily (
			metric_date DATE NOT NULL,
			baseline_summoner_rows BIGINT NOT NULL DEFAULT 0,
			current_summoner_rows BIGINT NOT NULL DEFAULT 0,
			baseline_match_rows BIGINT NOT NULL DEFAULT 0,
			current_match_rows BIGINT NOT NULL DEFAULT 0,
			baseline_mastery_rows BIGINT NOT NULL DEFAULT 0,
			current_mastery_rows BIGINT NOT NULL DEFAULT 0,
			baseline_queue_rows BIGINT NOT NULL DEFAULT 0,
			current_queue_rows BIGINT NOT NULL DEFAULT 0,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (metric_date)
		) ENGINE=InnoDB
	`)
	return err
}

// CollectDataExplorerOperationalMetrics avoids COUNT(*) on large InnoDB tables.
// information_schema.tables.table_rows is approximate, but cheap enough for
// periodic operational monitoring on production-sized datasets.
func CollectDataExplorerOperationalMetrics(database db.Context, queueRows int64) (*DataExplorerOperationalMetrics, error) {
	var tables dataExplorerTableMetrics
	if err := database.Get(&tables, `
		SELECT
			COALESCE(SUM(CASE WHEN table_name = 'summoners' THEN table_rows ELSE 0 END), 0) AS summoner_rows,
			COALESCE(SUM(CASE WHEN table_name = 'matches' THEN table_rows ELSE 0 END), 0) AS match_rows,
			COALESCE(SUM(CASE WHEN table_name = 'masteries' THEN table_rows ELSE 0 END), 0) AS mastery_rows,
			COALESCE(SUM(data_length + index_length), 0) AS database_bytes,
			COALESCE(SUM(data_free), 0) AS database_free_bytes
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
	`); err != nil {
		return nil, err
	}

	if _, err := database.Exec(`
		INSERT INTO data_explorer_metrics_daily
			(metric_date,
			 baseline_summoner_rows, current_summoner_rows,
			 baseline_match_rows, current_match_rows,
			 baseline_mastery_rows, current_mastery_rows,
			 baseline_queue_rows, current_queue_rows)
		VALUES (CURRENT_DATE(), ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			current_summoner_rows = VALUES(current_summoner_rows),
			current_match_rows = VALUES(current_match_rows),
			current_mastery_rows = VALUES(current_mastery_rows),
			current_queue_rows = VALUES(current_queue_rows),
			updated_at = NOW(6)
	`,
		tables.SummonerRows, tables.SummonerRows,
		tables.MatchRows, tables.MatchRows,
		tables.MasteryRows, tables.MasteryRows,
		queueRows, queueRows,
	); err != nil {
		return nil, err
	}

	var daily dataExplorerMetricsDailyRow
	if err := database.Get(&daily, `
		SELECT metric_date,
		       baseline_summoner_rows, current_summoner_rows,
		       baseline_match_rows, current_match_rows,
		       baseline_mastery_rows, current_mastery_rows,
		       baseline_queue_rows, current_queue_rows
		FROM data_explorer_metrics_daily
		WHERE metric_date = CURRENT_DATE()
	`); err != nil {
		return nil, err
	}

	metrics := &DataExplorerOperationalMetrics{
		MetricDate:             daily.MetricDate,
		SummonerRows:           tables.SummonerRows,
		MatchRows:              tables.MatchRows,
		MasteryRows:            tables.MasteryRows,
		QueueRows:              queueRows,
		DailySummonerRowGrowth: daily.CurrentSummonerRows - daily.BaselineSummonerRows,
		DailyMatchRowGrowth:    daily.CurrentMatchRows - daily.BaselineMatchRows,
		DailyMasteryRowGrowth:  daily.CurrentMasteryRows - daily.BaselineMasteryRows,
		DailyQueueRowGrowth:    daily.CurrentQueueRows - daily.BaselineQueueRows,
		DatabaseBytes:          tables.DatabaseBytes,
		DatabaseFreeBytes:      tables.DatabaseFreeBytes,
	}
	collectDataExplorerTempStatus(database, metrics)
	collectDataExplorerTempSpace(database, metrics)
	return metrics, nil
}

func collectDataExplorerTempStatus(database db.Context, metrics *DataExplorerOperationalMetrics) {
	var rows []dataExplorerGlobalStatusRow
	if err := database.Select(&rows, `
		SHOW GLOBAL STATUS
		WHERE Variable_name IN ('Created_tmp_tables', 'Created_tmp_disk_tables')
	`); err != nil {
		return
	}
	for _, row := range rows {
		value, err := strconv.ParseInt(row.Value, 10, 64)
		if err != nil {
			return
		}
		switch row.Name {
		case "Created_tmp_tables":
			metrics.CreatedTempTables = value
		case "Created_tmp_disk_tables":
			metrics.CreatedTempDiskTables = value
		}
	}
	metrics.TempStatusAvailable = len(rows) == 2
}

func collectDataExplorerTempSpace(database db.Context, metrics *DataExplorerOperationalMetrics) {
	var space dataExplorerTempSpace
	if err := database.Get(&space, `
		SELECT
			COUNT(*) AS file_count,
			COALESCE(SUM(total_extents * extent_size), 0) AS allocated_bytes,
			COALESCE(SUM(free_extents * extent_size), 0) AS free_bytes
		FROM information_schema.files
		WHERE tablespace_name = 'innodb_temporary'
		   OR tablespace_name LIKE 'innodb_temporary_%'
		   OR file_name LIKE '%#innodb_temp%'
	`); err != nil {
		return
	}
	if space.FileCount == 0 || !space.Allocated.Valid || !space.Free.Valid {
		return
	}
	metrics.TempAllocatedBytes = space.Allocated.Int64
	metrics.TempFreeBytes = space.Free.Int64
	metrics.TempSpaceAvailable = true
}
