package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
	"time"
)

type StatisticsSnapshotDAO struct {
	Key       string    `db:"snapshot_key"`
	Payload   []byte    `db:"payload"`
	UpdatedAt time.Time `db:"updated_at"`
	Fresh     bool      `db:"fresh"`
}

func EnsureStatisticsSnapshotSchema(database db.Context) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS statistics_snapshots (
			snapshot_key VARCHAR(64) NOT NULL,
			payload LONGBLOB NOT NULL,
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			PRIMARY KEY (snapshot_key),
			KEY statistics_snapshots_updated_at_index (updated_at)
		) ENGINE=InnoDB
	`)
	return err
}

func GetStatisticsSnapshot(database db.Context, key string, maxAge time.Duration) (*StatisticsSnapshotDAO, error) {
	var snapshot StatisticsSnapshotDAO
	maxAgeSeconds := int64(maxAge / time.Second)
	if maxAgeSeconds < 1 {
		maxAgeSeconds = 1
	}
	err := database.Get(&snapshot, `
		SELECT snapshot_key,
		       payload,
		       updated_at,
		       updated_at >= DATE_SUB(NOW(6), INTERVAL ? SECOND) AS fresh
		FROM statistics_snapshots
		WHERE snapshot_key = ?
	`, maxAgeSeconds, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &snapshot, nil
}

func UpsertStatisticsSnapshot(database db.Context, key string, payload []byte) error {
	_, err := database.Exec(`
		INSERT INTO statistics_snapshots (snapshot_key, payload, updated_at)
		VALUES (?, ?, NOW(6))
		ON DUPLICATE KEY UPDATE
			payload = VALUES(payload),
			updated_at = VALUES(updated_at)
	`, key, payload)
	return err
}
