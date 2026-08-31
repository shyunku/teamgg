package mixed

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestMatchParticipantExtraAcceptsNumericKeyColumns(t *testing.T) {
	modelType := reflect.TypeOf(MatchParticipantExtraMXDAO{})
	nullInt64Type := reflect.TypeOf(sql.NullInt64{})
	columns := []string{"match_pk", "match_participant_pk", "match_fk", "summoner_fk"}

	for _, column := range columns {
		found := false
		for index := 0; index < modelType.NumField(); index++ {
			field := modelType.Field(index)
			if field.Tag.Get("db") != column {
				continue
			}
			found = true
			if field.Type != nullInt64Type {
				t.Fatalf("%s must remain nullable during backfill; got %s", column, field.Type)
			}
			break
		}
		if !found {
			t.Fatalf("missing scan destination for %s", column)
		}
	}
}
