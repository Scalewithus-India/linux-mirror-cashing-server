package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/config"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/diskcache"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/metrics"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/mirror"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/store"
	"github.com/Scalewithus-India/linux-mirror-cashing-server/internal/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := &metrics.Metrics{}
	if m.LoadFromDisk(cfg.MetricsStatePath) {
		snap := m.Snapshot()
		slog.Info("Restored metrics",
			"path", cfg.MetricsStatePath,
			"hits_s3", snap["hits_s3"],
			"bytes_served", snap["bytes_served"],
		)
	} else {
		slog.Info("No metrics state; starting fresh", "path", cfg.MetricsStatePath)
	}

	st, err := store.New(ctx, cfg)
	if err != nil {
		slog.Error("s3 store", "err", err)
		os.Exit(1)
	}

	disk, err := diskcache.New(cfg.LocalCacheDir, cfg.LocalCacheReserveBytes, cfg.LocalCacheBytes)
	if err != nil {
		slog.Error("disk cache", "err", err)
		os.Exit(1)
	}

	mh := mirror.New(cfg, st, m, disk)
	site, err := web.New(cfg, st, m, mh)
	if err != nil {
		slog.Error("web", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	site.Register(mux)

	stopFlush := make(chan struct{})
	go m.FlushLoop(cfg.MetricsStatePath, cfg.MetricsFlushSeconds, stopFlush)
	go st.UsageLoop(ctx, cfg.S3UsageRefresh)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           web.AccessLog(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		slog.Info("listening", "addr", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("shutting down")
	cancel()
	close(stopFlush)
	shutdownCtx, c2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer c2()
	_ = srv.Shutdown(shutdownCtx)
	if err := m.SaveToDisk(cfg.MetricsStatePath); err != nil {
		slog.Warn("Final metrics save failed", "err", err)
	} else {
		slog.Info("Saved metrics", "path", cfg.MetricsStatePath)
	}
}
