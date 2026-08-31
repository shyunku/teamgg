package migrations

import (
	"strings"
	"testing"
	"time"
)

func TestNumericKeyBackfillBatchSizeBounds(t *testing.T) {
	tests := []struct {
		value    string
		expected int
	}{
		{"", 1000},
		{"9", 1000},
		{"10", 10},
		{"2500", 2500},
		{"10000", 10000},
		{"10001", 1000},
		{"invalid", 1000},
	}
	for _, test := range tests {
		if actual := boundedNumericKeyBatchSize(test.value); actual != test.expected {
			t.Fatalf("batch size %q: got %d, want %d", test.value, actual, test.expected)
		}
	}
}

func TestNumericKeyBackfillWorkLimitBounds(t *testing.T) {
	tests := []struct {
		value    string
		expected time.Duration
	}{
		{"", 10 * time.Minute},
		{"500ms", 10 * time.Minute},
		{"1s", time.Second},
		{"15m", 15 * time.Minute},
		{"1h", time.Hour},
		{"61m", 10 * time.Minute},
		{"invalid", 10 * time.Minute},
	}
	for _, test := range tests {
		if actual := boundedNumericKeyWorkLimit(test.value); actual != test.expected {
			t.Fatalf("work limit %q: got %s, want %s", test.value, actual, test.expected)
		}
	}
}

func TestNumericKeyTriggersCoverEveryParentWrite(t *testing.T) {
	triggers := numericKeyTriggers()
	if len(triggers) != len(numericKeyTriggerNames) {
		t.Fatalf("got %d triggers, want %d", len(triggers), len(numericKeyTriggerNames))
	}
	joined := make([]string, 0, len(triggers))
	for _, trigger := range triggers {
		joined = append(joined, trigger.name+"\n"+trigger.statement)
	}
	all := strings.Join(joined, "\n")
	for _, expected := range []string{
		"BEFORE INSERT ON summoners",
		"BEFORE UPDATE ON summoners",
		"BEFORE INSERT ON matches",
		"BEFORE INSERT ON match_participants",
		"SET NEW.summoner_pk",
		"SET NEW.match_pk",
		"SET NEW.match_participant_pk",
		"SET NEW.match_fk",
		"SET NEW.summoner_fk",
	} {
		if !strings.Contains(all, expected) {
			t.Fatalf("numeric key triggers do not contain %q", expected)
		}
	}
}

func TestParticipantTriggerPreallocatesPendingParents(t *testing.T) {
	statement := strings.ToLower(strings.Join(strings.Fields(numericKeyParticipantInsertTrigger().statement), " "))
	required := []string{
		"insert into match_numeric_keys (riot_match_id) values (new.match_id)",
		"insert into summoner_numeric_keys (puuid) values (new.puuid)",
		"set new.match_fk = last_insert_id()",
		"set new.summoner_fk = last_insert_id()",
	}
	for _, fragment := range required {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("participant trigger is missing %q", fragment)
		}
	}
	if strings.Contains(statement, "signal sqlstate") {
		t.Fatal("participant trigger must not reject the existing participant-before-summoner write order")
	}
}
