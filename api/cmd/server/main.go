// Command server runs the idpad REST API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ipedrazas/idpad/api/internal/config"
	"github.com/ipedrazas/idpad/api/internal/httpapi"
	"github.com/ipedrazas/idpad/api/internal/store"
)

func main() {
	// The image is distroless, so it ships no curl for the compose health
	// check; the binary probes itself instead.
	health := flag.Bool("health", false, "probe the local /healthz endpoint and exit")
	flag.Parse()

	if *health {
		if err := probeHealth(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		slog.Error("server exited with an error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := newLogger(cfg)
	slog.SetDefault(log)

	// The signal context cancels start-up work as well as the running server,
	// so a Ctrl-C during the database retry loop exits promptly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.MigrateOnStart {
		if err := store.Migrate(cfg.DatabaseURL, log); err != nil {
			return err
		}
	}

	pool, err := store.Connect(ctx, cfg.DatabaseURL, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	// A response must be allowed to outlast the slowest handler. Automatic
	// tagging waits on an external service, so the write timeout is derived
	// from its budget rather than fixed, or a cold start upstream would be cut
	// off by this server rather than by the timeout configured for it.
	writeTimeout := 60 * time.Second
	if budget := cfg.TaggerTimeout + 15*time.Second; budget > writeTimeout {
		writeTimeout = budget
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewServer(store.New(pool), log, cfg).Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       90 * time.Second,
	}

	if cfg.AutoTaggingEnabled() {
		log.Info("automatic tagging enabled", "url", cfg.TaggerURL, "timeout", cfg.TaggerTimeout)
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("shutdown complete")
	return <-serveErr
}

// newLogger builds the structured logger every component is handed.
func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

// probeHealth performs the container health check against the local server.
func probeHealth() error {
	addr := os.Getenv("IDPAD_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse IDPAD_ADDR %q: %w", addr, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/healthz", nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", res.StatusCode)
	}
	return nil
}
