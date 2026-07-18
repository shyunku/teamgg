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

func TestCorsUsesCurrentTeamGgDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := SetupRouter()

	allowed := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	allowedRequest.Header.Set("Origin", "https://teamgg.kr")
	router.ServeHTTP(allowed, allowedRequest)
	if got := allowed.Header().Get("Access-Control-Allow-Origin"); got != "https://teamgg.kr" {
		t.Fatalf("allowed origin: got %q", got)
	}

	removed := httptest.NewRecorder()
	removedRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	removedRequest.Header.Set("Origin", "https://team-gg.net")
	router.ServeHTTP(removed, removedRequest)
	if removed.Code != http.StatusForbidden {
		t.Fatalf("removed origin status: got %d, want %d", removed.Code, http.StatusForbidden)
	}
}
