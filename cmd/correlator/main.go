// KernelView AI Correlator — Main Entry Point
//
// Enterprise component: Receives anomaly bundles from the Collector,
// sends them to an LLM for root cause analysis, and forwards
// remediation recommendations to the Operator.
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

	logger.Info("starting KernelView AI correlator", "version", version)

	cfg := config.DefaultCorrelatorConfig()

	if cfg.LLMAPIKey == "" && cfg.LLMProvider != "ollama" {
		logger.Warn("LLM_API_KEY not set — correlator will run in degraded mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// TODO: Initialize LLM client
	// TODO: Initialize secrets scrubber
	// TODO: Start gRPC server for Collector → Correlator calls
	// TODO: Start gRPC client for Correlator → Operator calls

	logger.Info("correlator started", "llm_provider", cfg.LLMProvider, "llm_model", cfg.LLMModel)

	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	case <-ctx.Done():
	}

	logger.Info("correlator shutdown complete")
}
