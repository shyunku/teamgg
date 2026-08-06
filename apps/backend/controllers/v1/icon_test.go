package v1

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"team.gg-server/service"
	"testing"
)

func TestSetIconCacheHeaders(t *testing.T) {
	previousVersion := service.DataDragonVersion
	service.DataDragonVersion = "16.15.1"
	t.Cleanup(func() { service.DataDragonVersion = previousVersion })

	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{name: "versioned", version: "16.15.1", expected: iconVersionedCacheControl},
		{name: "missing version", version: "", expected: iconShortCacheControl},
		{name: "stale version", version: "16.14.1", expected: iconShortCacheControl},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			setIconCacheHeaders(context, test.version)
			if got := recorder.Header().Get("Cache-Control"); got != test.expected {
				t.Fatalf("Cache-Control: got %q, want %q", got, test.expected)
			}
			if got := recorder.Header().Get("CDN-Cache-Control"); got != test.expected {
				t.Fatalf("CDN-Cache-Control: got %q, want %q", got, test.expected)
			}
		})
	}
}
