package models

import (
	"database/sql"
	"strings"
	"testing"
)

type masteryStorageTestContext struct {
	selectQuery string
}

func (context *masteryStorageTestContext) Exec(string, ...interface{}) (sql.Result, error) {
	return nil, nil
}

func (context *masteryStorageTestContext) Get(interface{}, string, ...interface{}) error {
	return nil
}

func (context *masteryStorageTestContext) Select(_ interface{}, query string, _ ...interface{}) error {
	context.selectQuery = query
	return nil
}

func (context *masteryStorageTestContext) Rebind(query string) string { return query }

func TestConfigureMasteryReadSource(t *testing.T) {
	t.Cleanup(func() { _ = ConfigureMasteryReadSource(MasteryReadSourceLegacy) })

	if err := ConfigureMasteryReadSource(""); err != nil || MasteryNumericV2ReadsEnabled() {
		t.Fatalf("empty source must select legacy: enabled=%t err=%v", MasteryNumericV2ReadsEnabled(), err)
	}
	if err := ConfigureMasteryReadSource("numeric_v2"); err != nil || !MasteryNumericV2ReadsEnabled() {
		t.Fatalf("numeric_v2 source was not enabled: enabled=%t err=%v", MasteryNumericV2ReadsEnabled(), err)
	}
	if err := ConfigureMasteryReadSource("unknown"); err == nil {
		t.Fatal("invalid mastery read source was accepted")
	}
}

func TestConfigureMasteryWriteSource(t *testing.T) {
	t.Cleanup(func() { _ = ConfigureMasteryWriteSource(MasteryWriteSourceLegacy) })

	if err := ConfigureMasteryWriteSource(""); err != nil || MasteryNumericV2WritesEnabled() {
		t.Fatalf("empty source must select legacy: enabled=%t err=%v", MasteryNumericV2WritesEnabled(), err)
	}
	if err := ConfigureMasteryWriteSource("numeric_v2"); err != nil || !MasteryNumericV2WritesEnabled() {
		t.Fatalf("numeric_v2 source was not enabled: enabled=%t err=%v", MasteryNumericV2WritesEnabled(), err)
	}
	if err := ConfigureMasteryWriteSource("unknown"); err == nil {
		t.Fatal("invalid mastery write source was accepted")
	}
}

func TestGetMasteryDAOsSelectsConfiguredStorage(t *testing.T) {
	t.Cleanup(func() { _ = ConfigureMasteryReadSource(MasteryReadSourceLegacy) })
	database := &masteryStorageTestContext{}

	if err := ConfigureMasteryReadSource(MasteryReadSourceLegacy); err != nil {
		t.Fatal(err)
	}
	if _, err := GetMasteryDAOs(database, "puuid"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(database.selectQuery, "FROM masteries") || strings.Contains(database.selectQuery, "masteries_numeric_v2") {
		t.Fatalf("legacy source query is unexpected: %s", database.selectQuery)
	}

	if err := ConfigureMasteryReadSource(MasteryReadSourceNumericV2); err != nil {
		t.Fatal(err)
	}
	if _, err := GetMasteryDAOs(database, "puuid"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(database.selectQuery, "FROM summoner_numeric_keys") ||
		!strings.Contains(database.selectQuery, "INNER JOIN masteries_numeric_v2") {
		t.Fatalf("numeric source query is unexpected: %s", database.selectQuery)
	}
}
