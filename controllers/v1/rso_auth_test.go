package v1

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"team.gg-server/core"
	"team.gg-server/libs/db"
	"testing"
	"time"
)

type rsoTestMemory struct {
	mutex  sync.Mutex
	values map[string]string
}

func (m *rsoTestMemory) Set(key, value string) error { return m.SetExp(key, value, 0) }
func (m *rsoTestMemory) SetExp(key, value string, _ time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.values[key] = value
	return nil
}
func (m *rsoTestMemory) Get(key string) (string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	value, exists := m.values[key]
	if !exists {
		return "", db.ErrValueNotFound
	}
	return value, nil
}
func (m *rsoTestMemory) Del(key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.values, key)
	return nil
}
func (m *rsoTestMemory) LPush(string, string) error { return errors.New("not implemented") }
func (m *rsoTestMemory) LPushExp(string, string, time.Duration) error {
	return errors.New("not implemented")
}
func (m *rsoTestMemory) LRange(string, int64, int64) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (m *rsoTestMemory) RPush(string, string) error { return errors.New("not implemented") }
func (m *rsoTestMemory) RPushExp(string, string, time.Duration) error {
	return errors.New("not implemented")
}
func (m *rsoTestMemory) LLen(string) (int64, error)         { return 0, errors.New("not implemented") }
func (m *rsoTestMemory) LRem(string, int64, string) error   { return errors.New("not implemented") }
func (m *rsoTestMemory) Expire(string, time.Duration) error { return errors.New("not implemented") }

func TestRsoStartCreatesPendingFlowAndAuthorizeURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousMemory := db.InMemoryDB
	previousClientId := core.RsoClientId
	previousClientSecret := core.RsoClientSecret
	previousCallback := core.RsoClientCallbackUri
	t.Cleanup(func() {
		db.InMemoryDB = previousMemory
		core.RsoClientId = previousClientId
		core.RsoClientSecret = previousClientSecret
		core.RsoClientCallbackUri = previousCallback
	})

	db.InMemoryDB = &rsoTestMemory{values: make(map[string]string)}
	core.RsoClientId = "test-client"
	core.RsoClientSecret = "test-secret"
	core.RsoClientCallbackUri = "https://example.com/v1/auth/rsoLogin"

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/auth/rso/start", nil)
	RsoStart(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		FlowId       string `json:"flowId"`
		AuthorizeUrl string `json:"authorizeUrl"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(response.AuthorizeUrl)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "auth.riotgames.com" || parsed.Query().Get("client_id") != "test-client" {
		t.Fatalf("unexpected authorize URL: %s", response.AuthorizeUrl)
	}
	if parsed.Query().Get("state") == "" || response.FlowId == "" {
		t.Fatal("state and flowId must be generated")
	}
	stateRaw, err := db.InMemoryDB.Get(rsoStateKeyPrefix + parsed.Query().Get("state"))
	if err != nil {
		t.Fatal(err)
	}
	var stateValue rsoState
	if err := json.Unmarshal([]byte(stateRaw), &stateValue); err != nil {
		t.Fatal(err)
	}
	if stateValue.FlowId != response.FlowId || stateValue.Mode != rsoModeLogin || stateValue.Uid != "" {
		t.Fatalf("unexpected login state: %+v", stateValue)
	}
	raw, err := db.InMemoryDB.Get(rsoFlowKeyPrefix + response.FlowId)
	if err != nil {
		t.Fatal(err)
	}
	var flow rsoFlowResult
	if err := json.Unmarshal([]byte(raw), &flow); err != nil {
		t.Fatal(err)
	}
	if flow.Status != rsoFlowPending {
		t.Fatalf("flow status: got %s, want %s", flow.Status, rsoFlowPending)
	}
}

func TestRsoStatusConsumesCompletedFlow(t *testing.T) {
	previousMemory := db.InMemoryDB
	t.Cleanup(func() { db.InMemoryDB = previousMemory })
	db.InMemoryDB = &rsoTestMemory{values: make(map[string]string)}

	const flowId = "completed-flow"
	if err := saveRsoFlow(flowId, rsoFlowResult{
		Status: rsoFlowComplete,
		Login:  &LoginResponseDto{Uid: "uid", UserId: "RiotUser#KR1"},
	}, time.Minute); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/v1/auth/rso/status?flowId="+flowId, nil)
	RsoStatus(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusOK)
	}
	if _, err := db.InMemoryDB.Get(rsoFlowKeyPrefix + flowId); !errors.Is(err, db.ErrValueNotFound) {
		t.Fatal("completed flow must be consumed")
	}
}
