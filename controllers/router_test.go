package controllers

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"team.gg-server/core"
	"testing"
)

func TestServerVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDebugMode := core.DebugMode
	core.DebugMode = true
	t.Cleanup(func() { core.DebugMode = previousDebugMode })
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	serverVersion(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Version      string `json:"version"`
		IsProduction bool   `json:"isProduction"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Version != core.Version || response.IsProduction {
		t.Fatalf("body: got %+v, want version=%q isProduction=false", response, core.Version)
	}
}
