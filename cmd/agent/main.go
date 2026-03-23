// KernelView Agent — Main Entry Point
//
// The agent runs as a DaemonSet on every node. It loads eBPF programs,
// captures kernel events, and streams them to the Collector via gRPC.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/kernelview/kernelview/internal/agent/bpf"
	"github.com/kernelview/kernelview/internal/agent/cgroup"
	"github.com/kernelview/kernelview/internal/agent/metadata"
	"github.com/kernelview/kernelview/internal/agent/stream"
	"github.com/kernelview/kernelview/pkg/config"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("starting KernelView agent", "version", version)

	// Load configuration
	cfg := config.DefaultAgentConfig()
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// Initialize cgroup detection
	cgroupDetector := cgroup.NewDetector(logger)
	logger.Info("cgroup version detected", "version", cgroupDetector.Version())

	// Initialize metadata resolver
	resolver := metadata.NewResolver(logger)
	_ = resolver // Will be wired to BPF event handler

	// Initialize BPF loader
	loader, err := bpf.NewLoader(cfg.BPFPinPath, logger)
	if err != nil {
		logger.Error("failed to create BPF loader", "error", err)
		os.Exit(1)
	}
	defer loader.Close()

	// Log detected capabilities
	logger.Info("kernel capabilities",
		"kernel", loader.KernelVersion,
		"cgroup", loader.CgroupVersion,
		"btf", loader.HasBTF,
		"ringbuf", loader.HasRingBuf,
	)

	// Load BPF programs
	programs := []struct {
		name string
		path string
	}{
		{"http_trace", "bpf/http_trace.o"},
		{"tcp_events", "bpf/tcp_events.o"},
		{"syscall_rate", "bpf/syscall_rate.o"},
		{"oom_watch", "bpf/oom_watch.o"},
		{"exec_watch", "bpf/exec_watch.o"},
		// grpc_trace requires uprobe setup — loaded separately
		// net_policy requires XDP attachment — loaded separately
	}

	for _, prog := range programs {
		if err := loader.LoadProgram(prog.name, prog.path); err != nil {
			// Log error but continue — don't crash if one program fails
			// (PRD: "If a kprobe attachment fails, log the error and continue")
			logger.Error("failed to load BPF program (continuing)",
				"name", prog.name, "error", err)
		}
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start gRPC stream to Collector
	streamClient := stream.NewClient(
		cfg.CollectorEndpoint,
		cfg.NodeName,
		cfg.NodeName, // agentID = nodeName for now
		loader,
		resolver,
		logger,
	)

	// Start heartbeat sender
	go streamClient.SendHeartbeat(ctx)

	// Start streaming in the background
	go func() {
		if err := streamClient.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("stream client exited with error", "error", err)
		}
	}()

	logger.Info("agent started successfully",
		"node", cfg.NodeName,
		"collector", cfg.CollectorEndpoint,
	)

	select {
	case sig := <-sigCh:
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	case <-ctx.Done():
	}

	logger.Info("agent shutdown complete")
}
