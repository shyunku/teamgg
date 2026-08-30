package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type adminServer struct {
	config  config
	backend *backendClient
	probes  *http.Client
}

func newAdminServer(cfg config) *adminServer {
	return &adminServer{
		config:  cfg,
		backend: newBackendClient(cfg),
		probes:  &http.Client{Timeout: cfg.requestTimeout},
	}
}

func (server *adminServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /v1/admin/session", server.requireAdmin(server.session))
	mux.HandleFunc("GET /v1/admin/overview", server.requireAdmin(server.overview))
	mux.HandleFunc("GET /v1/admin/events", server.requireAdmin(server.events))
	return server.cors(server.recover(mux))
}

type adminHandler func(http.ResponseWriter, *http.Request, adminActor)

func (server *adminServer) requireAdmin(next adminHandler) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		authorization := strings.TrimSpace(request.Header.Get("Authorization"))
		if !strings.HasPrefix(authorization, "Bearer ") {
			writeError(writer, http.StatusUnauthorized, "로그인이 필요합니다.")
			return
		}
		actor, status, err := server.backend.authorize(request.Context(), authorization)
		if err != nil {
			slog.Error("administrator authorization failed", "error", err)
			writeError(writer, http.StatusBadGateway, "관리자 인증 서버에 연결하지 못했습니다.")
			return
		}
		if actor == nil {
			if status == http.StatusForbidden {
				writeError(writer, status, "관리자 권한이 없습니다.")
			} else {
				writeError(writer, status, "관리자 인증에 실패했습니다.")
			}
			return
		}
		next(writer, request, *actor)
	}
}

func (server *adminServer) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]interface{}{"status": "ok", "time": time.Now().UTC()})
}

func (server *adminServer) session(writer http.ResponseWriter, request *http.Request, actor adminActor) {
	writeJSON(writer, http.StatusOK, actor)
	server.recordAudit(request, actor, "admin.session", "session", "success", nil)
}

func (server *adminServer) overview(writer http.ResponseWriter, request *http.Request, actor adminActor) {
	backendOverview, err := server.backend.overview(request.Context())
	if err != nil {
		slog.Error("admin overview failed", "error", err, "actor_uid", actor.Uid)
		server.recordAudit(request, actor, "admin.overview.read", "overview", "failed", nil)
		writeError(writer, http.StatusBadGateway, "운영 지표를 불러오지 못했습니다.")
		return
	}
	statuses := server.serviceStatuses(request.Context())
	var backendData interface{}
	if err := json.Unmarshal(backendOverview, &backendData); err != nil {
		writeError(writer, http.StatusBadGateway, "운영 지표 형식이 올바르지 않습니다.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]interface{}{
		"generatedAt": time.Now().UTC(), "services": statuses, "operations": redactValue(backendData),
	})
	server.recordAudit(request, actor, "admin.overview.read", "overview", "success", nil)
}

func (server *adminServer) events(writer http.ResponseWriter, request *http.Request, actor adminActor) {
	limit := clampEventLimit(request.URL.Query().Get("limit"))
	events, err := server.backend.events(request.Context(), limit)
	if err != nil {
		slog.Error("admin events failed", "error", err, "actor_uid", actor.Uid)
		server.recordAudit(request, actor, "admin.events.read", "events", "failed", map[string]interface{}{"limit": limit})
		writeError(writer, http.StatusBadGateway, "운영 이벤트를 불러오지 못했습니다.")
		return
	}
	var data interface{}
	if err := json.Unmarshal(events, &data); err != nil {
		writeError(writer, http.StatusBadGateway, "운영 이벤트 형식이 올바르지 않습니다.")
		return
	}
	writeJSON(writer, http.StatusOK, redactValue(data))
	server.recordAudit(request, actor, "admin.events.read", "events", "success", map[string]interface{}{"limit": limit})
}

func (server *adminServer) serviceStatuses(ctx context.Context) []serviceStatus {
	definitions := []struct{ name, endpoint string }{
		{name: "backend", endpoint: server.config.teamggAPIBaseURL + "/ping"},
		{name: "replay-analyzer", endpoint: server.config.replayAPIBaseURL + "/health"},
	}
	statuses := make([]serviceStatus, len(definitions))
	var wait sync.WaitGroup
	for index, definition := range definitions {
		wait.Add(1)
		go func(index int, definition struct{ name, endpoint string }) {
			defer wait.Done()
			statuses[index] = probeService(ctx, server.probes, definition.name, definition.endpoint)
		}(index, definition)
	}
	wait.Wait()
	return statuses
}

func (server *adminServer) recordAudit(request *http.Request, actor adminActor, action, resource, result string, metadata map[string]interface{}) {
	ctx, cancel := context.WithTimeout(context.Background(), server.config.requestTimeout)
	defer cancel()
	if err := server.backend.audit(ctx, actor, action, resource, result, clientIP(request), metadata); err != nil {
		slog.Error("failed to persist administrator audit", "error", err, "actor_uid", actor.Uid, "action", action)
	}
}

func (server *adminServer) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := strings.TrimRight(strings.TrimSpace(request.Header.Get("Origin")), "/")
		if origin != "" {
			if _, allowed := server.config.allowedOrigins[origin]; !allowed {
				writeError(writer, http.StatusForbidden, "허용되지 않은 Origin입니다.")
				return
			}
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Vary", "Origin")
		}
		writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
		writer.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (server *adminServer) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("admin request panic", "panic", recovered, "path", request.URL.Path)
				writeError(writer, http.StatusInternalServerError, "관리자 서버 오류가 발생했습니다.")
			}
		}()
		next.ServeHTTP(writer, request)
	})
}

func clientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}

func writeJSON(writer http.ResponseWriter, status int, value interface{}) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]interface{}{"code": status, "message": message})
}
