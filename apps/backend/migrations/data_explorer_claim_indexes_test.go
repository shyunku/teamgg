package migrations

import (
	"strings"
	"testing"
)

func TestDataExplorerClaimIndexesMatchQueueOrder(t *testing.T) {
	indexes := dataExplorerClaimIndexes()
	if len(indexes) != 2 {
		t.Fatalf("got %d indexes, want 2", len(indexes))
	}
	for _, index := range indexes {
		if strings.Join(index.columns, ",") != "status,next_attempt_at,priority,created_at,"+index.idColumn {
			t.Fatalf("unexpected %s columns: %v", index.name, index.columns)
		}
		if strings.Join(index.direction, ",") != "A,A,D,A,A" {
			t.Fatalf("unexpected %s directions: %v", index.name, index.direction)
		}
	}
}
