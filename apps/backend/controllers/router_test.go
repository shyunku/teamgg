package controllers

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"team.gg-server/core"
	"team.gg-server/service"
	"testing"
)

func TestConfigureGinModeFromProductionEnvironment(t *testing.T) {
	previousMode := gin.Mode()
	previousProductionMode := core.IsProduction
	t.Cleanup(func() {
		gin.SetMode(previousMode)
		core.IsProduction = previousProductionMode
	})

	gin.SetMode(gin.DebugMode)
	core.IsProduction = true
	configureGinMode()
	if gin.Mode() != gin.ReleaseMode {
		t.Fatalf("production Gin mode: got %q, want %q", gin.Mode(), gin.ReleaseMode)
	}

	gin.SetMode(gin.ReleaseMode)
	core.IsProduction = false
	configureGinMode()
	if gin.Mode() != gin.DebugMode {
		t.Fatalf("development Gin mode: got %q, want %q", gin.Mode(), gin.DebugMode)
	}

	gin.SetMode(gin.TestMode)
	core.IsProduction = true
	configureGinMode()
	if gin.Mode() != gin.TestMode {
		t.Fatalf("explicit test Gin mode was overwritten: got %q", gin.Mode())
	}
}

func TestServerVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousProductionMode := core.IsProduction
	previousDebugMode := core.DebugMode
	core.IsProduction = true
	core.DebugMode = true
	t.Cleanup(func() {
		core.IsProduction = previousProductionMode
		core.DebugMode = previousDebugMode
	})
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	serverVersion(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Version           string `json:"version"`
		IsProduction      bool   `json:"isProduction"`
		DataDragonVersion string `json:"dataDragonVersion"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Version != core.Version || !response.IsProduction || response.DataDragonVersion != service.DataDragonVersion {
		t.Fatalf("body: got %+v, want version=%q isProduction=true", response, core.Version)
	}
}

func TestProductionModeControlsTestRoutesIndependentlyFromDebug(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousProductionMode := core.IsProduction
	previousDebugMode := core.DebugMode
	t.Cleanup(func() {
		core.IsProduction = previousProductionMode
		core.DebugMode = previousDebugMode
	})

	core.IsProduction = true
	core.DebugMode = true
	productionRecorder := httptest.NewRecorder()
	productionRequest := httptest.NewRequest(http.MethodGet, "/test/riotApiCalls", nil)
	SetupRouter().ServeHTTP(productionRecorder, productionRequest)
	if productionRecorder.Code != http.StatusNotFound {
		t.Fatalf("production test route status: got %d, want %d", productionRecorder.Code, http.StatusNotFound)
	}

	core.IsProduction = false
	core.DebugMode = false
	developmentRecorder := httptest.NewRecorder()
	developmentRequest := httptest.NewRequest(http.MethodGet, "/test/riotApiCalls", nil)
	SetupRouter().ServeHTTP(developmentRecorder, developmentRequest)
	if developmentRecorder.Code != http.StatusOK {
		t.Fatalf("development test route status: got %d, want %d", developmentRecorder.Code, http.StatusOK)
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
