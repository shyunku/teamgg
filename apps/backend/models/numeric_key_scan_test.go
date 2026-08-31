package models

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestNumericKeyColumnsHaveNullableScanDestinations(t *testing.T) {
	nullInt64Type := reflect.TypeOf(sql.NullInt64{})
	tests := []struct {
		name    string
		model   interface{}
		columns []string
	}{
		{name: "summoner", model: SummonerDAO{}, columns: []string{"summoner_pk"}},
		{name: "match", model: MatchDAO{}, columns: []string{"match_pk"}},
		{
			name:  "match participant",
			model: MatchParticipantDAO{},
			columns: []string{
				"match_participant_pk", "match_fk", "summoner_fk",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modelType := reflect.TypeOf(test.model)
			for _, column := range test.columns {
				field, ok := fieldByDBTag(modelType, column)
				if !ok {
					t.Fatalf("missing scan destination for %s", column)
				}
				if field.Type != nullInt64Type {
					t.Fatalf("%s must remain nullable during backfill; got %s", column, field.Type)
				}
			}
		})
	}
}

func fieldByDBTag(modelType reflect.Type, column string) (reflect.StructField, bool) {
	for index := 0; index < modelType.NumField(); index++ {
		field := modelType.Field(index)
		if field.Tag.Get("db") == column {
			return field, true
		}
	}
	return reflect.StructField{}, false
}
