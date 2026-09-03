package migrations

import (
	"context"
	"strings"
	"testing"
)

func TestMasteryNumericDirectWriteCutoverRequiresOfflineAcknowledgement(t *testing.T) {
	t.Setenv("MASTERY_WRITE_CUTOVER_OFFLINE_ACK", "false")
	err := applyMasteryNumericDirectWrites(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "backend to be stopped") {
		t.Fatalf("expected offline acknowledgement error, got %v", err)
	}
}

func TestMasteryNumericDirectWriteMigrationIsRegistered(t *testing.T) {
	for _, definition := range migrationDefinitions() {
		if definition.Version == masteryNumericDirectWritesMigrationVersion {
			return
		}
	}
	t.Fatalf("migration %s is not registered", masteryNumericDirectWritesMigrationVersion)
}
