package util

import (
	"path/filepath"
	"testing"
)

func TestGetProjectRootDirectoryUsesConfiguredRoot(t *testing.T) {
	configuredRoot := filepath.Join("configured", "project", "..", "root")
	t.Setenv("APP_PROJECT_ROOT", configuredRoot)

	if actual, expected := GetProjectRootDirectory(), filepath.Clean(configuredRoot); actual != expected {
		t.Fatalf("GetProjectRootDirectory() = %q, want %q", actual, expected)
	}
}
