package migrations

import (
	"context"
	"strings"
	"testing"
)

func TestMasteryLegacyRetirementRequiresBothAcknowledgements(t *testing.T) {
	for _, acknowledgements := range [][2]string{{"false", "false"}, {"true", "false"}, {"false", "true"}} {
		t.Run(acknowledgements[0]+"_"+acknowledgements[1], func(t *testing.T) {
			t.Setenv("MASTERY_LEGACY_DROP_OFFLINE_ACK", acknowledgements[0])
			t.Setenv("MASTERY_LEGACY_DROP_ACK", acknowledgements[1])
			err := applyMasteryLegacyRetirement(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), "requires the backend to be stopped") {
				t.Fatalf("expected acknowledgement error, got %v", err)
			}
		})
	}
}

func TestMasteryLegacyRetirementMigrationIsRegistered(t *testing.T) {
	for _, definition := range migrationDefinitions() {
		if definition.Version == masteryLegacyRetirementMigrationVersion {
			return
		}
	}
	t.Fatalf("migration %s is not registered", masteryLegacyRetirementMigrationVersion)
}

func TestMasteryStatisticsTriggersTargetNumericStorage(t *testing.T) {
	for _, trigger := range masteryStatisticsTriggers("masteries_numeric_v2") {
		definition := strings.ToLower(trigger.statement)
		if !strings.Contains(definition, " on masteries_numeric_v2 ") {
			t.Fatalf("trigger %s does not target numeric storage: %s", trigger.name, trigger.statement)
		}
	}
}
