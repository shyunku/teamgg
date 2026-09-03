package models

import (
	"fmt"
	"strings"
	"sync/atomic"
)

const (
	MasteryReadSourceLegacy     = "legacy"
	MasteryReadSourceNumericV2  = "numeric_v2"
	MasteryWriteSourceLegacy    = "legacy"
	MasteryWriteSourceNumericV2 = "numeric_v2"
)

var masteryNumericV2Reads atomic.Bool
var masteryNumericV2Writes atomic.Bool

func ConfigureMasteryReadSource(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", MasteryReadSourceLegacy:
		masteryNumericV2Reads.Store(false)
		return nil
	case MasteryReadSourceNumericV2:
		masteryNumericV2Reads.Store(true)
		return nil
	default:
		return fmt.Errorf("invalid MASTERY_READ_SOURCE %q: expected legacy or numeric_v2", value)
	}
}

func MasteryNumericV2ReadsEnabled() bool {
	return masteryNumericV2Reads.Load()
}

func ConfigureMasteryWriteSource(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", MasteryWriteSourceLegacy:
		masteryNumericV2Writes.Store(false)
		return nil
	case MasteryWriteSourceNumericV2:
		masteryNumericV2Writes.Store(true)
		return nil
	default:
		return fmt.Errorf("invalid MASTERY_WRITE_SOURCE %q: expected legacy or numeric_v2", value)
	}
}

func MasteryNumericV2WritesEnabled() bool {
	return masteryNumericV2Writes.Load()
}
