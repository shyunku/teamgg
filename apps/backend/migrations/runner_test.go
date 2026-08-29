package migrations

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestResolveModeDefaultsToValidation(t *testing.T) {
	t.Setenv("DB_MIGRATION_MODE", "")
	mode, err := ResolveMode(false)
	if err != nil {
		t.Fatal(err)
	}
	if mode != ModeValidate {
		t.Fatalf("default mode must not mutate schema, got %s", mode)
	}
}

func TestResolveModeSupportsExplicitModesAndMigrationCommand(t *testing.T) {
	for _, expected := range []Mode{ModeUp, ModeValidate, ModeOff} {
		t.Run(string(expected), func(t *testing.T) {
			t.Setenv("DB_MIGRATION_MODE", strings.ToUpper(string(expected)))
			actual, err := ResolveMode(false)
			if err != nil {
				t.Fatal(err)
			}
			if actual != expected {
				t.Fatalf("got %s, want %s", actual, expected)
			}
		})
	}
	t.Setenv("DB_MIGRATION_MODE", "invalid")
	if _, err := ResolveMode(false); err == nil {
		t.Fatal("invalid migration mode must fail")
	}
	mode, err := ResolveMode(true)
	if err != nil || mode != ModeUp {
		t.Fatalf("migration command must force up mode: mode=%s err=%v", mode, err)
	}
}

func TestMigrationManifestIsOrderedAndChecksummed(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 12 {
		t.Fatalf("unexpected migration count: %d", len(migrations))
	}
	seen := make(map[string]struct{}, len(migrations))
	for index, migration := range migrations {
		if len(migration.Checksum) != 64 {
			t.Fatalf("migration %s has invalid checksum %q", migration.Version, migration.Checksum)
		}
		if _, exists := seen[migration.Version]; exists {
			t.Fatalf("duplicate migration version %s", migration.Version)
		}
		seen[migration.Version] = struct{}{}
		if index > 0 && migrations[index-1].Version >= migration.Version {
			t.Fatalf("manifest is not ordered: %s then %s", migrations[index-1].Version, migration.Version)
		}
	}
}

func TestMigrationSafetyConfigurationBounds(t *testing.T) {
	t.Setenv("DB_MIGRATION_LOCK_TIMEOUT", "10m")
	if got := migrationLockTimeout(); got != 30*time.Second {
		t.Fatalf("invalid lock timeout should use fallback, got %s", got)
	}
	t.Setenv("DB_MIGRATION_LOCK_TIMEOUT", "45s")
	if got := migrationLockTimeout(); got != 45*time.Second {
		t.Fatalf("valid lock timeout not applied: %s", got)
	}

	t.Setenv("DB_MIGRATION_SUMMONER_MATCH_BATCH", "1")
	if got := summonerMatchesCatchupBatch(); got != 1000 {
		t.Fatalf("unsafe small batch should use fallback, got %d", got)
	}
	t.Setenv("DB_MIGRATION_SUMMONER_MATCH_BATCH", "5000")
	if got := summonerMatchesCatchupBatch(); got != 5000 {
		t.Fatalf("valid batch not applied: %d", got)
	}
}

func TestTruncateMigrationError(t *testing.T) {
	message := strings.Repeat("x", 5000)
	if got := truncateMigrationError(errors.New(message)); len(got) != 4000 {
		t.Fatalf("unexpected truncated length: %d", len(got))
	}
}
