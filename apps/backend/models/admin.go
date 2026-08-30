package models

import (
	"database/sql"
	"errors"
	"team.gg-server/libs/db"
	"time"
)

const (
	UserRoleAdmin  = "admin"
	UserRoleViewer = "viewer"
)

type AdminAuditLogDAO struct {
	Id           uint64    `db:"id" json:"id"`
	ActorUid     string    `db:"actor_uid" json:"actorUid"`
	Action       string    `db:"action" json:"action"`
	Resource     string    `db:"resource" json:"resource"`
	Result       string    `db:"result" json:"result"`
	ClientIp     string    `db:"client_ip" json:"clientIp"`
	MetadataJson *string   `db:"metadata_json" json:"metadataJson,omitempty"`
	CreatedAt    time.Time `db:"created_at" json:"createdAt"`
}

type AdminOperationalEventDAO struct {
	Id          uint64    `db:"id" json:"id"`
	Source      string    `db:"source" json:"source"`
	Level       string    `db:"level" json:"level"`
	EventType   string    `db:"event_type" json:"eventType"`
	Message     string    `db:"message" json:"message"`
	DetailsJson *string   `db:"details_json" json:"detailsJson,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

type AdminStatusCount struct {
	Status string `db:"status" json:"status"`
	Count  int64  `db:"count" json:"count"`
}

type AdminStatisticsSnapshotSummary struct {
	Key       string    `db:"snapshot_key" json:"key"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

type AdminMigrationSummary struct {
	Version   string    `db:"version" json:"version"`
	Dirty     bool      `db:"dirty" json:"dirty"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

func EnsureAdminOperationsSchema(database db.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS user_roles (
			uid VARCHAR(255) NOT NULL,
			role VARCHAR(24) NOT NULL,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (uid),
			KEY user_roles_role_index (role),
			CONSTRAINT user_roles_user_fk FOREIGN KEY (uid) REFERENCES users (uid)
				ON UPDATE CASCADE ON DELETE CASCADE
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS admin_audit_logs (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			actor_uid VARCHAR(255) NOT NULL,
			action VARCHAR(64) NOT NULL,
			resource VARCHAR(128) NOT NULL,
			result VARCHAR(24) NOT NULL,
			client_ip VARCHAR(64) NOT NULL DEFAULT '',
			metadata_json TEXT NULL,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			PRIMARY KEY (id),
			KEY admin_audit_logs_actor_created_index (actor_uid, created_at),
			KEY admin_audit_logs_created_index (created_at)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS admin_operational_events (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
			source VARCHAR(48) NOT NULL,
			level VARCHAR(16) NOT NULL,
			event_type VARCHAR(64) NOT NULL,
			message VARCHAR(500) NOT NULL,
			details_json TEXT NULL,
			created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			PRIMARY KEY (id),
			KEY admin_operational_events_created_index (created_at),
			KEY admin_operational_events_type_created_index (event_type, created_at)
		) ENGINE=InnoDB`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func GetUserRole(database db.Context, uid string) (string, bool, error) {
	var role string
	err := database.Get(&role, `SELECT role FROM user_roles WHERE uid = ?`, uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return role, true, nil
}

func UpsertUserRole(database db.Context, uid, role string) error {
	_, err := database.Exec(`
		INSERT INTO user_roles (uid, role)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE role = VALUES(role), updated_at = NOW(6)
	`, uid, role)
	return err
}

func InsertAdminAuditLog(database db.Context, entry *AdminAuditLogDAO) error {
	_, err := database.Exec(`
		INSERT INTO admin_audit_logs
			(actor_uid, action, resource, result, client_ip, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(6))
	`, entry.ActorUid, entry.Action, entry.Resource, entry.Result, entry.ClientIp, entry.MetadataJson)
	return err
}

func ListAdminAuditLogs(database db.Context, limit int) ([]AdminAuditLogDAO, error) {
	logs := make([]AdminAuditLogDAO, 0)
	err := database.Select(&logs, `
		SELECT id, actor_uid, action, resource, result, client_ip, metadata_json, created_at
		FROM admin_audit_logs
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	return logs, err
}

func InsertAdminOperationalEvent(database db.Context, event *AdminOperationalEventDAO) error {
	_, err := database.Exec(`
		INSERT INTO admin_operational_events
			(source, level, event_type, message, details_json, created_at)
		VALUES (?, ?, ?, ?, ?, NOW(6))
	`, event.Source, event.Level, event.EventType, event.Message, event.DetailsJson)
	return err
}

func ListAdminOperationalEvents(database db.Context, limit int) ([]AdminOperationalEventDAO, error) {
	events := make([]AdminOperationalEventDAO, 0)
	err := database.Select(&events, `
		SELECT id, source, level, event_type, message, details_json, created_at
		FROM admin_operational_events
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	return events, err
}

func GetReplayAnalysisStatusCounts(database db.Context) ([]AdminStatusCount, error) {
	counts := make([]AdminStatusCount, 0)
	err := database.Select(&counts, `
		SELECT status, COUNT(*) AS count
		FROM custom_game_replay_analyses
		GROUP BY status
	`)
	return counts, err
}

func GetStatisticsSnapshotSummaries(database db.Context) ([]AdminStatisticsSnapshotSummary, error) {
	summaries := make([]AdminStatisticsSnapshotSummary, 0)
	err := database.Select(&summaries, `
		SELECT snapshot_key, updated_at
		FROM statistics_snapshots
		ORDER BY snapshot_key
	`)
	return summaries, err
}

func GetLatestMigrationSummary(database db.Context) (*AdminMigrationSummary, error) {
	var summary AdminMigrationSummary
	err := database.Get(&summary, `
		SELECT version, dirty, updated_at
		FROM schema_migrations
		ORDER BY version DESC
		LIMIT 1
	`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &summary, nil
}
