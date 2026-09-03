package migrations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

const masteryLegacyRetirementMigrationVersion = "20260903_002"

func applyMasteryLegacyRetirement(ctx context.Context, database *sqlx.DB) error {
	offlineAcknowledged, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("MASTERY_LEGACY_DROP_OFFLINE_ACK")))
	dropAcknowledged, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("MASTERY_LEGACY_DROP_ACK")))
	if !offlineAcknowledged || !dropAcknowledged {
		return errors.New("legacy mastery retirement requires the backend to be stopped and both MASTERY_LEGACY_DROP_OFFLINE_ACK=true and MASTERY_LEGACY_DROP_ACK=true")
	}
	if err := ValidateMasteryNumericStorage(ctx, database); err != nil {
		return fmt.Errorf("validate numeric mastery storage before legacy retirement: %w", err)
	}
	if err := switchMasteryStatisticsToNumeric(ctx, database); err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, "DROP TABLE IF EXISTS `masteries`"); err != nil {
		return fmt.Errorf("drop legacy masteries table: %w", err)
	}
	return nil
}

func validateMasteryLegacyRetirement(ctx context.Context, database *sqlx.DB) (bool, error) {
	if err := ValidateMasteryNumericStorage(ctx, database); err != nil {
		return false, err
	}
	exists, err := tableExists(ctx, database, "masteries")
	if err != nil || exists {
		return false, err
	}
	return validateMasteryStatisticsStorage(ctx, database, "masteries_numeric_v2", "masteries_numeric_champion_points_level_covering_index")
}
