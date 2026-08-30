package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRedactValueRemovesNestedSensitiveFields(t *testing.T) {
	redacted := redactValue(map[string]interface{}{
		"token":  "secret-token",
		"nested": map[string]interface{}{"password": "password", "safe": "value"},
	}).(map[string]interface{})
	if redacted["token"] != "[REDACTED]" {
		t.Fatal("token was not redacted")
	}
	nested := redacted["nested"].(map[string]interface{})
	if nested["password"] != "[REDACTED]" || nested["safe"] != "value" {
		t.Fatalf("unexpected nested redaction: %+v", nested)
	}
}

func TestClampEventLimit(t *testing.T) {
	if clampEventLimit("") != 100 || clampEventLimit("999") != 200 || clampEventLimit("12") != 12 {
		t.Fatal("event limit bounds are invalid")
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	server := newAdminServer(config{allowedOrigins: map[string]struct{}{"https://teamgg.kr": {}}, requestTimeout: time.Second})
	handler := server.cors(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusOK) }))
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestProbeServiceDoesNotExposeResponseBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte("secret body"))
	}))
	defer upstream.Close()
	status := probeService(context.Background(), upstream.Client(), "service", upstream.URL)
	if status.Healthy || status.StatusCode != http.StatusServiceUnavailable || status.Error != "unhealthy response" {
		t.Fatalf("unexpected probe status: %+v", status)
	}
}
