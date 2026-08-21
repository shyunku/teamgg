package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type Mode string

const (
	ModeUp       Mode = "up"
	ModeValidate Mode = "validate"
	ModeOff      Mode = "off"

	migrationLockName = "teamgg_schema_migrations"
)

//go:embed *.sql
var migrationFiles embed.FS

type Definition struct {
	Version  string
	FileName string
	Apply    func(context.Context, *sqlx.DB) error
	Validate func(context.Context, *sqlx.DB) (bool, error)
}

type migration struct {
	Definition
	Name     string
	Checksum string
}

type appliedMigration struct {
	Version  string `db:"version"`
	Checksum string `db:"checksum"`
	Dirty    bool   `db:"dirty"`
}

func ResolveMode(forceUp bool) (Mode, error) {
	if forceUp {
		return ModeUp, nil
	}
	configured := strings.ToLower(strings.TrimSpace(os.Getenv("DB_MIGRATION_MODE")))
	if configured == "" {
		return ModeValidate, nil
	}
	switch Mode(configured) {
	case ModeUp, ModeValidate, ModeOff:
		return Mode(configured), nil
	default:
		return "", fmt.Errorf("invalid DB_MIGRATION_MODE %q: use up, validate, or off", configured)
	}
}

func Run(ctx context.Context, database *sqlx.DB, mode Mode, appVersion string) error {
	if mode == ModeOff {
		return nil
	}
	if mode != ModeUp && mode != ModeValidate {
		return fmt.Errorf("unsupported database migration mode %q", mode)
	}

	connection, err := database.Connx(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()

	lockTimeout := migrationLockTimeout()
	var acquired int
	if err := connection.GetContext(ctx, &acquired, "SELECT GET_LOCK(?, ?)", migrationLockName, int(lockTimeout/time.Second)); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if acquired != 1 {
		return fmt.Errorf("database migration lock was not acquired within %s", lockTimeout)
	}
	defer func() {
		var released int
		_ = connection.GetContext(context.Background(), &released, "SELECT RELEASE_LOCK(?)", migrationLockName)
	}()

	if err := ensureMigrationMetadata(ctx, connection); err != nil {
		return err
	}
	if err := validateBaseSchema(ctx, database); err != nil {
		return err
	}
	definitions, err := loadMigrations()
	if err != nil {
		return err
	}
	applied, err := loadAppliedMigrations(ctx, connection)
	if err != nil {
		return err
	}
	known := make(map[string]struct{}, len(definitions))
	for _, current := range definitions {
		known[current.Version] = struct{}{}
	}
	for version := range applied {
		if _, exists := known[version]; !exists {
			return fmt.Errorf("database contains unknown migration %s; this binary may be older than the schema", version)
		}
	}

	for _, current := range definitions {
		record, exists := applied[current.Version]
		if exists {
			if record.Checksum != current.Checksum {
				return fmt.Errorf(
					"migration %s checksum mismatch: database=%s code=%s",
					current.Version,
					record.Checksum,
					current.Checksum,
				)
			}
			if record.Dirty && mode == ModeValidate {
				return fmt.Errorf("migration %s is marked dirty; run the migrate command to resume it", current.Version)
			}
			if !record.Dirty {
				valid, err := current.Validate(ctx, database)
				if err != nil {
					return fmt.Errorf("validate applied migration %s: %w", current.Version, err)
				}
				if !valid {
					return fmt.Errorf("schema drift detected after migration %s", current.Version)
				}
				continue
			}
		}

		valid, err := current.Validate(ctx, database)
		if err != nil {
			return fmt.Errorf("inspect pending migration %s: %w", current.Version, err)
		}
		if !exists && valid {
			if mode == ModeValidate {
				return fmt.Errorf("migration %s is present in the schema but is not recorded; run the migrate command to baseline it", current.Version)
			}
			if err := recordMigration(ctx, connection, current, appVersion, "baseline", 0, false, ""); err != nil {
				return err
			}
			continue
		}
		if mode == ModeValidate {
			return fmt.Errorf("pending database migration %s (%s)", current.Version, current.Name)
		}

		started := time.Now()
		if err := recordMigration(ctx, connection, current, appVersion, "migration", 0, true, ""); err != nil {
			return err
		}
		if err := current.Apply(ctx, database); err != nil {
			message := truncateMigrationError(err)
			_ = recordMigration(ctx, connection, current, appVersion, "migration", time.Since(started), true, message)
			return fmt.Errorf("apply migration %s: %w", current.Version, err)
		}
		valid, err = current.Validate(ctx, database)
		if err != nil {
			return fmt.Errorf("validate migration %s after apply: %w", current.Version, err)
		}
		if !valid {
			return fmt.Errorf("migration %s completed without producing the required schema", current.Version)
		}
		if err := recordMigration(ctx, connection, current, appVersion, "migration", time.Since(started), false, ""); err != nil {
			return err
		}
	}
	return nil
}

func loadMigrations() ([]migration, error) {
	definitions := migrationDefinitions()
	migrations := make([]migration, 0, len(definitions))
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if definition.Version == "" || definition.FileName == "" || definition.Apply == nil || definition.Validate == nil {
			return nil, errors.New("invalid migration definition")
		}
		if _, exists := seen[definition.Version]; exists {
			return nil, fmt.Errorf("duplicate migration version %s", definition.Version)
		}
		seen[definition.Version] = struct{}{}
		contents, err := migrationFiles.ReadFile(definition.FileName)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", definition.FileName, err)
		}
		digest := sha256.Sum256(contents)
		migrations = append(migrations, migration{
			Definition: definition,
			Name:       strings.TrimSuffix(definition.FileName, ".sql"),
			Checksum:   hex.EncodeToString(digest[:]),
		})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	return migrations, nil
}

func ensureMigrationMetadata(ctx context.Context, connection *sqlx.Conn) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(96) NOT NULL,
			name VARCHAR(255) NOT NULL,
			checksum CHAR(64) NOT NULL,
			apply_kind VARCHAR(16) NOT NULL,
			app_version VARCHAR(32) NOT NULL,
			dirty TINYINT(1) NOT NULL DEFAULT 1,
			duration_ms BIGINT NOT NULL DEFAULT 0,
			error_message TEXT NULL,
			applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (version)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS schema_migration_progress (
			migration_version VARCHAR(96) NOT NULL,
			state_key VARCHAR(64) NOT NULL,
			state_value VARCHAR(255) NOT NULL DEFAULT '',
			updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (migration_version, state_key)
		) ENGINE=InnoDB`,
	}
	for _, statement := range statements {
		if _, err := connection.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize migration metadata: %w", err)
		}
	}
	return nil
}

func loadAppliedMigrations(ctx context.Context, connection *sqlx.Conn) (map[string]appliedMigration, error) {
	rows := make([]appliedMigration, 0)
	if err := connection.SelectContext(ctx, &rows, `
		SELECT version, checksum, dirty FROM schema_migrations ORDER BY version
	`); err != nil {
		return nil, err
	}
	result := make(map[string]appliedMigration, len(rows))
	for _, row := range rows {
		result[row.Version] = row
	}
	return result, nil
}

func recordMigration(
	ctx context.Context,
	connection *sqlx.Conn,
	migration migration,
	appVersion string,
	applyKind string,
	duration time.Duration,
	dirty bool,
	errorMessage string,
) error {
	var nullableError interface{}
	if errorMessage != "" {
		nullableError = errorMessage
	}
	_, err := connection.ExecContext(ctx, `
		INSERT INTO schema_migrations
			(version, name, checksum, apply_kind, app_version, dirty, duration_ms, error_message, applied_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(6))
		ON DUPLICATE KEY UPDATE
			name = VALUES(name), checksum = VALUES(checksum), apply_kind = VALUES(apply_kind),
			app_version = VALUES(app_version), dirty = VALUES(dirty),
			duration_ms = VALUES(duration_ms), error_message = VALUES(error_message),
			updated_at = NOW(6)
	`, migration.Version, migration.Name, migration.Checksum, applyKind, appVersion, dirty, duration.Milliseconds(), nullableError)
	if err != nil {
		return fmt.Errorf("record migration %s: %w", migration.Version, err)
	}
	return nil
}

func migrationLockTimeout() time.Duration {
	value := strings.TrimSpace(os.Getenv("DB_MIGRATION_LOCK_TIMEOUT"))
	if value == "" {
		return 30 * time.Second
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < 0 || duration > 5*time.Minute {
		return 30 * time.Second
	}
	return duration
}

func truncateMigrationError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 4000 {
		return message[:4000]
	}
	return message
}
