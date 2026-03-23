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

	"github.com/kernelview/kernelview/internal/collector/api"
	"github.com/kernelview/kernelview/internal/collector/ingest"
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

	// Initialize gRPC ingest server
	grpcServer := ingest.NewGRPCServer(logger)

	// Register event handlers for the 3 parallel pipelines
	grpcServer.OnEvent(func(nodeName string, events []byte) error {
		// Pipeline 1: Store to BadgerDB (72-hour raw traces)
		logger.Debug("storing events to BadgerDB", "node", nodeName, "bytes", len(events))
		return nil
	})

	grpcServer.OnEvent(func(nodeName string, events []byte) error {
		// Pipeline 2: Push to VictoriaMetrics (30-day aggregated metrics)
		logger.Debug("pushing metrics to VictoriaMetrics", "node", nodeName)
		return nil
	})

	grpcServer.OnEvent(func(nodeName string, events []byte) error {
		// Pipeline 3: Feed anomaly detection engine
		logger.Debug("processing events for anomaly detection", "node", nodeName)
		return nil
	})

	// Start gRPC server for agent connections
	go func() {
		if err := grpcServer.Serve(ctx, cfg.GRPCListenAddr); err != nil {
			logger.Error("gRPC server failed", "error", err)
		}
	}()

	// Start REST API server for dashboard
	restServer := api.NewServer(cfg.RESTListenAddr, logger)
	go func() {
		if err := restServer.ListenAndServe(); err != nil {
			logger.Error("REST API server failed", "error", err)
		}
	}()

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
