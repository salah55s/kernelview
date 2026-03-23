// KernelView Remediation Operator — Main Entry Point
//
// Enterprise component: Watches RemediationAction CRDs and executes
// remediation actions against the Kubernetes API. All actions are
// validated against safety rules before execution.
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

	logger.Info("starting KernelView remediation operator", "version", version)

	cfg := config.DefaultOperatorConfig()

	if cfg.DryRun {
		logger.Warn("operator running in DRY-RUN mode — no actions will be executed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// TODO: Initialize Kubernetes client
	// TODO: Initialize safety rule engine
	// TODO: Start CRD watcher (informer)
	// TODO: Start reconciliation loop

	logger.Info("operator started",
		"dry_run", cfg.DryRun,
		"protected_namespaces", cfg.ProtectedNamespaces,
		"max_actions_per_hour", cfg.MaxActionsPerPodPerHour,
	)

	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	case <-ctx.Done():
	}

	logger.Info("operator shutdown complete")
}
