package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const adminSecretHeader = "X-Teamgg-Admin-Secret"

type adminActor struct {
	Uid    string `json:"uid"`
	UserId string `json:"userId"`
	Role   string `json:"role"`
}

type backendClient struct {
	baseURL string
	secret  string
	client  *http.Client
}

func newBackendClient(cfg config) *backendClient {
	return &backendClient{
		baseURL: cfg.teamggAPIBaseURL,
		secret:  cfg.internalSecret,
		client:  &http.Client{Timeout: cfg.requestTimeout},
	}
}

func (client *backendClient) authorize(ctx context.Context, authorization string) (*adminActor, int, error) {
	request, err := client.request(ctx, http.MethodPost, "/v1/internal/admin/authorize", nil)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	request.Header.Set("Authorization", authorization)
	response, err := client.client.Do(request)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, nil
	}
	var actor adminActor
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&actor); err != nil {
		return nil, http.StatusBadGateway, err
	}
	if actor.Uid == "" || actor.Role == "" {
		return nil, http.StatusBadGateway, fmt.Errorf("backend returned an incomplete administrator identity")
	}
	return &actor, http.StatusOK, nil
}

func (client *backendClient) overview(ctx context.Context) (json.RawMessage, error) {
	return client.getJSON(ctx, "/v1/internal/admin/overview")
}

func (client *backendClient) events(ctx context.Context, limit int) (json.RawMessage, error) {
	return client.getJSON(ctx, "/v1/internal/admin/events?limit="+url.QueryEscape(strconv.Itoa(limit)))
}

func (client *backendClient) audit(ctx context.Context, actor adminActor, action, resource, result, clientIP string, metadata map[string]interface{}) error {
	payload := map[string]interface{}{
		"actorUid": actor.Uid, "action": action, "resource": resource,
		"result": result, "clientIp": clientIP, "metadata": redactValue(metadata),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := client.request(ctx, http.MethodPost, "/v1/internal/admin/audit", bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("audit endpoint returned %d", response.StatusCode)
	}
	return nil
}

func (client *backendClient) getJSON(ctx context.Context, path string) (json.RawMessage, error) {
	request, err := client.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend endpoint returned %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("backend returned invalid JSON")
	}
	return json.RawMessage(body), nil
}

func (client *backendClient) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set(adminSecretHeader, client.secret)
	request.Header.Set("Accept", "application/json")
	return request, nil
}

func clampEventLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit < 1 {
		return 100
	}
	if limit > 200 {
		return 200
	}
	return limit
}

type serviceStatus struct {
	Name       string `json:"name"`
	Healthy    bool   `json:"healthy"`
	StatusCode int    `json:"statusCode"`
	LatencyMs  int64  `json:"latencyMs"`
	Error      string `json:"error,omitempty"`
}

func probeService(ctx context.Context, client *http.Client, name, endpoint string) serviceStatus {
	started := time.Now()
	status := serviceStatus{Name: name}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		status.Error = "invalid endpoint"
		return status
	}
	response, err := client.Do(request)
	status.LatencyMs = time.Since(started).Milliseconds()
	if err != nil {
		status.Error = "connection failed"
		return status
	}
	defer response.Body.Close()
	status.StatusCode = response.StatusCode
	status.Healthy = response.StatusCode >= 200 && response.StatusCode < 300
	if !status.Healthy {
		status.Error = "unhealthy response"
	}
	return status
}
