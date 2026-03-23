// KernelView Collector — Main Entry Point
//
// The Collector receives event streams from all agents, normalizes data,
// stores metrics/traces, runs anomaly detection, and serves the REST API.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/kernelview/kernelview/pkg/config"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("starting KernelView collector", "version", version)

	cfg := config.DefaultCollectorConfig()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// TODO: Initialize BadgerDB store
	// TODO: Initialize VictoriaMetrics writer
	// TODO: Initialize anomaly detection engine
	// TODO: Start gRPC ingestion server on cfg.GRPCListenAddr
	// TODO: Start REST API server on cfg.RESTListenAddr
	// TODO: Start Kubernetes informer for deployment events

	logger.Info("collector started",
		"grpc_addr", cfg.GRPCListenAddr,
		"rest_addr", cfg.RESTListenAddr,
	)

	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	case <-ctx.Done():
	}

	logger.Info("collector shutdown complete")
}
