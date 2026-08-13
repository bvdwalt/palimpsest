package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bvdwalt/palimpsest/internal/config"
	"github.com/bvdwalt/palimpsest/internal/db"
	"github.com/bvdwalt/palimpsest/internal/server"
	"github.com/bvdwalt/palimpsest/internal/store"
	"github.com/grafana/pyroscope-go"
)

func run(ctx context.Context, cfg *config.Config) error {
	if err := db.MigrateLegacyPath(cfg.DBPath); err != nil {
		return err
	}

	sqlDB, err := db.Connect(cfg.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	st := store.New(sqlDB)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.New(st, cfg.AutosaveIntervalSeconds),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	cfg := config.Load()

	if cfg.PyroscopeServerAddress != "" {
		if _, err := pyroscope.Start(pyroscope.Config{
			ApplicationName: "palimpsest",
			ServerAddress:   cfg.PyroscopeServerAddress,
			ProfileTypes: []pyroscope.ProfileType{
				pyroscope.ProfileCPU,
				pyroscope.ProfileAllocObjects,
				pyroscope.ProfileAllocSpace,
				pyroscope.ProfileInuseObjects,
				pyroscope.ProfileInuseSpace,
				pyroscope.ProfileGoroutines,
			},
		}); err != nil {
			slog.Error("failed to start pyroscope", "err", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}
