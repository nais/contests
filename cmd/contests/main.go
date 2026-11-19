package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nais/contests/internal/bucket"
	"github.com/nais/contests/internal/database"
	"github.com/nais/contests/internal/kafka"
	"github.com/nais/contests/internal/probe"
	"github.com/nais/contests/internal/valkey"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	addr := flag.String("bind-address", ":8080", "HTTP listen address")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *addr, logger); err != nil {
		logger.With("err", err).Error("Contests stopped")
		os.Exit(1)
	}
}

func run(ctx context.Context, addr string, logger *slog.Logger) error {
	checks, err := configuredChecks(os.Getenv("CHECKS"))
	if err != nil {
		return err
	}
	registry := newRegistry()
	runner := probe.New(checks, registry, logger)
	server := &http.Server{
		Addr:              addr,
		Handler:           newHandler(registry),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	wg.Go(func() { runner.Run(ctx) })
	var shutdownErr error
	wg.Go(func() {
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		shutdownErr = server.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			shutdownErr = errors.Join(shutdownErr, server.Close())
		}
	})
	logger.With("address", addr).Info("Starting contests")
	err = server.ListenAndServe()
	cancel()
	wg.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return errors.Join(err, shutdownErr)
}

func configuredChecks(names string) ([]probe.Check, error) {
	if names == "none" {
		return nil, nil
	}
	if names == "" {
		names = "postgres,kafka,bucket,valkey"
	}
	available := map[string]func(context.Context) error{
		"postgres": database.Check,
		"kafka":    kafka.Check,
		"bucket":   func(ctx context.Context) error { return bucket.Check(ctx, os.Getenv("BUCKET_NAME")) },
		"valkey":   valkey.Check,
	}
	var checks []probe.Check
	seen := make(map[string]bool)
	for name := range strings.SplitSeq(names, ",") {
		check, ok := available[name]
		if !ok || seen[name] {
			return nil, fmt.Errorf("unknown or duplicate check %q", name)
		}
		seen[name] = true
		checks = append(checks, probe.Check{Name: name, Run: check})
	}
	return checks, nil
}

func newRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return registry
}

func newHandler(registry prometheus.Gatherer) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}
