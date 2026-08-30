package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("invalid administrator server configuration", "error", err)
		os.Exit(1)
	}
	application := newAdminServer(cfg)
	server := &http.Server{
		Addr:              cfg.host + ":" + cfg.port,
		Handler:           application.handler(),
		ReadHeaderTimeout: cfg.requestTimeout,
		ReadTimeout:       cfg.requestTimeout,
		WriteTimeout:      cfg.requestTimeout * 2,
		IdleTimeout:       cfg.requestTimeout * 12,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			slog.Error("administrator server shutdown failed", "error", err)
		}
	}()

	slog.Info("administrator server started", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("administrator server failed", "error", err)
		os.Exit(1)
	}
}
