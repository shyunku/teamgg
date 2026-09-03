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

const masteryNumericDirectWritesMigrationVersion = "20260903_001"

func applyMasteryNumericDirectWrites(ctx context.Context, database *sqlx.DB) error {
	offlineAcknowledged, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("MASTERY_WRITE_CUTOVER_OFFLINE_ACK")))
	if !offlineAcknowledged {
		return errors.New("mastery direct-write cutover requires the backend to be stopped and MASTERY_WRITE_CUTOVER_OFFLINE_ACK=true")
	}
	if err := ValidateMasteryNumericStorage(ctx, database); err != nil {
		return fmt.Errorf("validate mastery numeric storage before direct-write cutover: %w", err)
	}
	for _, name := range []string{
		masteryNumericShadowInsertTrigger,
		masteryNumericShadowUpdateTrigger,
		masteryNumericShadowDeleteTrigger,
	} {
		if _, err := database.ExecContext(ctx, "DROP TRIGGER IF EXISTS `"+name+"`"); err != nil {
			return fmt.Errorf("drop legacy mastery shadow trigger %s: %w", name, err)
		}
	}
	return nil
}

func validateMasteryNumericDirectWrites(ctx context.Context, database *sqlx.DB) (bool, error) {
	if err := ValidateMasteryNumericStorage(ctx, database); err != nil {
		return false, err
	}
	for _, name := range []string{
		masteryNumericShadowInsertTrigger,
		masteryNumericShadowUpdateTrigger,
		masteryNumericShadowDeleteTrigger,
	} {
		var count int
		if err := database.GetContext(ctx, &count, `
			SELECT COUNT(*) FROM information_schema.triggers
			WHERE trigger_schema = DATABASE() AND trigger_name = ?
		`, name); err != nil {
			return false, err
		}
		if count != 0 {
			return false, nil
		}
	}
	return true, nil
}
