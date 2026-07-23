package statistics

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestDurationEnvironment(t *testing.T) {
	t.Setenv("STATISTICS_TEST_PERIOD", "90m")
	value, err := durationEnvironment("STATISTICS_TEST_PERIOD", time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if value != 90*time.Minute {
		t.Fatalf("expected 90m, got %s", value)
	}
}

func TestDurationEnvironmentRejectsInvalidValue(t *testing.T) {
	t.Setenv("STATISTICS_TEST_PERIOD", "tomorrow")
	if _, err := durationEnvironment("STATISTICS_TEST_PERIOD", time.Hour, false); err == nil {
		t.Fatal("expected invalid duration error")
	}
}

func TestSharedPayloadCompressionRoundTrip(t *testing.T) {
	original := bytes.Repeat([]byte(`{"championId":1,"winRate":0.5}`), 100)
	encoded, err := encodeSharedPayload(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= len(original) {
		t.Fatalf("expected compressed payload to be smaller: encoded=%d original=%d", len(encoded), len(original))
	}
	decoded, err := decodeSharedPayload(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, original) {
		t.Fatal("decoded payload differs from original")
	}
}

func TestDecodeSharedPayloadSupportsLegacyJSON(t *testing.T) {
	legacy := []byte(`{"updatedAt":"2026-07-24T00:00:00Z"}`)
	decoded, err := decodeSharedPayload(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, legacy) {
		t.Fatal("legacy JSON payload was changed")
	}
}

func TestRunLoopStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	runLoop(ctx, "test", RunnerConfig{
		Enabled:      true,
		Period:       time.Hour,
		InitialDelay: time.Hour,
		RetryDelay:   time.Minute,
	}, func(context.Context) error {
		called = true
		return errors.New("must not run")
	})
	if called {
		t.Fatal("collector ran after cancellation")
	}
}
