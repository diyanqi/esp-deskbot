package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"data-collect-server/internal/ai"
	"data-collect-server/internal/config"
	"data-collect-server/internal/handler"
	"data-collect-server/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}
	logger := newLogger(cfg)
	slog.SetDefault(logger)

	if cfg.PushToken == "" {
		logger.Warn("IOTDA_PUSH_TOKEN is empty; unsigned push requests are enabled for development only")
	}
	if cfg.APIToken == "" {
		logger.Warn("API_TOKEN is empty; the query API is open to anyone who can reach it")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := store.New(ctx, cfg.DatabaseURL, store.Options{
		MaxConns:       cfg.DBMaxConns,
		MinConns:       cfg.DBMinConns,
		MaxConnLife:    cfg.DBMaxConnLife,
		ConnectTimeout: cfg.DBConnectTimeout,
		RunMigrations:  cfg.RunMigrations,
		Logger:         logger,
	})
	if err != nil {
		logger.Error("initialize PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	var aiClient *ai.Client
	if cfg.AIEnabled {
		aiClient = ai.New(cfg.AIAPIURL, cfg.AIModel, cfg.AIAPIKey, cfg.AITimeout)
		logger.Info("AI health report enabled", "model", cfg.AIModel, "endpoint", cfg.AIAPIURL)
	} else {
		logger.Info("AI health report disabled; using deterministic reports")
	}
	if cfg.DemoEnabled {
		logger.Warn("demo scenario endpoint is enabled")
	}

	httpHandler := handler.New(database, handler.Options{
		PushToken:        cfg.PushToken,
		SignatureMaxAge:  cfg.SignatureMaxAge,
		DevAllowUnsigned: cfg.DevAllowUnsigned,
		APIToken:         cfg.APIToken,
		CORSAllowOrigins: cfg.CORSAllowOrigins,
		MaxBodyBytes:     cfg.MaxBodyBytes,
		AIClient:         aiClient,
		DemoEnabled:      cfg.DemoEnabled,
	}, logger)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpHandler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("starting data collection service", "config", cfg.Describe())
		if cfg.TLSEnabled() {
			logger.Info("HTTPS listener active", "addr", cfg.Addr)
			serverErrors <- server.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
			return
		}
		logger.Warn("HTTP listener active; IoTDA HTTPS pushes require TLS termination", "addr", cfg.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		_ = server.Close()
	}
	logger.Info("service stopped")
}

func newLogger(cfg *config.Config) *slog.Logger {
	options := &slog.HandlerOptions{Level: cfg.SlogLevel()}
	if cfg.LogJSON {
		return slog.New(slog.NewJSONHandler(os.Stdout, options))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, options))
}
