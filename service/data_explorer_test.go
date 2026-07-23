package service

import (
	"errors"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"testing"
	"time"
)

func TestDataExplorerRetryDelayIsBounded(t *testing.T) {
	explorer := NewDataExplorer()
	if got := explorer.retryDelay(1); got != time.Second+137*time.Millisecond {
		t.Fatalf("unexpected first retry delay: %s", got)
	}
	if got, max := explorer.retryDelay(100), 128*time.Second+8*137*time.Millisecond; got != max {
		t.Fatalf("retry delay must be capped: got %s want %s", got, max)
	}
}

func TestDataExplorerEnabledEnvironment(t *testing.T) {
	for _, disabled := range []string{"0", "false", "OFF"} {
		t.Run(disabled, func(t *testing.T) {
			t.Setenv("DATA_EXPLORER_ENABLED", disabled)
			if IsDataExplorerEnabled() {
				t.Fatalf("expected %q to disable DataExplorer", disabled)
			}
		})
	}
	t.Setenv("DATA_EXPLORER_ENABLED", "true")
	if !IsDataExplorerEnabled() {
		t.Fatal("expected true to enable DataExplorer")
	}
}

func TestRetryableDatabaseErrors(t *testing.T) {
	for _, number := range []uint16{1062, 1205, 1213} {
		if !isRetryableDatabaseError(&mysqlDriver.MySQLError{Number: number}) {
			t.Fatalf("mysql error %d must be retryable", number)
		}
	}
	if isRetryableDatabaseError(&mysqlDriver.MySQLError{Number: 1045}) {
		t.Fatal("authentication errors must not be retried")
	}
	if isRetryableDatabaseError(errors.New("plain error")) {
		t.Fatal("non-MySQL errors must not be treated as database races")
	}
}
