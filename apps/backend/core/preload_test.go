package core

import "testing"

func TestEnvironmentBool(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "on", "yes"} {
		t.Run("true_"+value, func(t *testing.T) {
			t.Setenv("TEST_BOOLEAN", value)
			if !environmentBool("TEST_BOOLEAN") {
				t.Fatalf("expected %q to be true", value)
			}
		})
	}
	for _, value := range []string{"", "0", "false", "off", "no", "invalid"} {
		t.Run("false_"+value, func(t *testing.T) {
			t.Setenv("TEST_BOOLEAN", value)
			if environmentBool("TEST_BOOLEAN") {
				t.Fatalf("expected %q to be false", value)
			}
		})
	}
}
