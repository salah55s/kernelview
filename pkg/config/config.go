// Package config provides shared configuration for all KernelView components.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// AgentConfig holds configuration for the eBPF agent.
type AgentConfig struct {
	// NodeName is the Kubernetes node this agent runs on.
	NodeName string

	// CollectorEndpoint is the gRPC address of the Collector.
	CollectorEndpoint string

	// BPF program settings
	BPFPinPath       string        // Default: /sys/fs/bpf/kernelview/
	RingBufferSize   int           // Default: 64MB
	SamplingRate     float64       // Default: 1.0 (capture all)
	SamplingCPULimit float64       // Default: 1.5 (activate sampling above this %)

	// Batching
	HTTPBatchSize    int           // Default: 100
	HTTPBatchTimeout time.Duration // Default: 1s
	TCPBatchSize     int           // Default: 50
	TCPBatchTimeout  time.Duration // Default: 5s

	// Syscall monitoring
	SyscallReportInterval time.Duration // Default: 5s

	// Buffer for network partition
	EventBufferSize   int           // Default: 50MB
	EventBufferMaxAge time.Duration // Default: 5m

	// TLS
	TLSCertFile string
	TLSKeyFile  string
	TLSCAFile   string
}

// CollectorConfig holds configuration for the Collector.
type CollectorConfig struct {
	// gRPC listen address for agent connections
	GRPCListenAddr string // Default: :4317

	// REST API listen address for dashboard
	RESTListenAddr string // Default: :8080

	// VictoriaMetrics
	VMRemoteWriteURL string // Default: http://victoriametrics:8428/api/v1/write
	VMWriteBufferSize int   // Default: 100MB

	// BadgerDB
	BadgerDataDir    string        // Default: /data/badger
	BadgerRetention  time.Duration // Default: 72h
	BadgerGCInterval time.Duration // Default: 1h

	// Anomaly detection
	LatencyBaselineWindow  time.Duration // Default: 1h
	LatencyAnomalyFactor   float64       // Default: 3.0
	ErrorRateBaselineWindow time.Duration // Default: 24h
	ErrorRateThresholdPP   float64       // Default: 15.0 (percentage points)
	ErrorRateAbsoluteMax   float64       // Default: 20.0 (percent)
	NoisyNeighborSigma     float64       // Default: 3.0
	NoisyNeighborDuration  time.Duration // Default: 30s

	// TLS
	TLSCertFile string
	TLSKeyFile  string
	TLSCAFile   string
}

// CorrelatorConfig holds configuration for the AI Correlator.
type CorrelatorConfig struct {
	// gRPC listen address
	GRPCListenAddr string // Default: :4318

	// LLM configuration
	LLMProvider   string // "claude", "openai", "ollama"
	LLMModel      string // e.g., "claude-3-sonnet-20240229", "gpt-4", "llama3:8b"
	LLMAPIKey     string
	LLMEndpoint   string        // For Ollama: http://ollama:11434
	LLMTimeout    time.Duration // Default: 30s
	LLMMaxRetries int           // Default: 3

	// Secrets scrubbing
	ScrubSecrets bool // Default: true
	ScrubLogs    bool // Default: true

	// Context window management
	MaxPromptTokens int // Default: 4096
}

// OperatorConfig holds configuration for the Remediation Operator.
type OperatorConfig struct {
	// Safety rules
	MaxActionsPerPodPerHour int     // Default: 3
	MinCPUThrottlePercent   float64 // Default: 10.0 (never below 10% of declared)
	ProtectedNamespaces     []string // Default: ["kube-system"]
	RequireApprovalForIsolate bool   // Default: true (non-negotiable)
	MaxCordonnedNodePercent  float64 // Default: 20.0

	// Dry-run mode
	DryRun bool // Default: false

	// Revert
	DefaultRevertDuration time.Duration // Default: 5m
}

// DefaultAgentConfig returns a AgentConfig with sensible defaults.
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		NodeName:              os.Getenv("NODE_NAME"),
		CollectorEndpoint:     envOrDefault("COLLECTOR_ENDPOINT", "kernelview-collector:4317"),
		BPFPinPath:            envOrDefault("BPF_PIN_PATH", "/sys/fs/bpf/kernelview/"),
		RingBufferSize:        envOrDefaultInt("RING_BUFFER_SIZE", 64*1024*1024),
		SamplingRate:          1.0,
		SamplingCPULimit:      1.5,
		HTTPBatchSize:         100,
		HTTPBatchTimeout:      1 * time.Second,
		TCPBatchSize:          50,
		TCPBatchTimeout:       5 * time.Second,
		SyscallReportInterval: 5 * time.Second,
		EventBufferSize:       50 * 1024 * 1024,
		EventBufferMaxAge:     5 * time.Minute,
		TLSCertFile:           envOrDefault("TLS_CERT_FILE", ""),
		TLSKeyFile:            envOrDefault("TLS_KEY_FILE", ""),
		TLSCAFile:             envOrDefault("TLS_CA_FILE", ""),
	}
}

// DefaultCollectorConfig returns a CollectorConfig with sensible defaults.
func DefaultCollectorConfig() *CollectorConfig {
	return &CollectorConfig{
		GRPCListenAddr:          envOrDefault("GRPC_LISTEN_ADDR", ":4317"),
		RESTListenAddr:          envOrDefault("REST_LISTEN_ADDR", ":8080"),
		VMRemoteWriteURL:        envOrDefault("VM_REMOTE_WRITE_URL", "http://victoriametrics:8428/api/v1/write"),
		VMWriteBufferSize:       envOrDefaultInt("VM_WRITE_BUFFER_SIZE", 100*1024*1024),
		BadgerDataDir:           envOrDefault("BADGER_DATA_DIR", "/data/badger"),
		BadgerRetention:         72 * time.Hour,
		BadgerGCInterval:        1 * time.Hour,
		LatencyBaselineWindow:   1 * time.Hour,
		LatencyAnomalyFactor:    3.0,
		ErrorRateBaselineWindow: 24 * time.Hour,
		ErrorRateThresholdPP:    15.0,
		ErrorRateAbsoluteMax:    20.0,
		NoisyNeighborSigma:     3.0,
		NoisyNeighborDuration:  30 * time.Second,
	}
}

// DefaultCorrelatorConfig returns a CorrelatorConfig with sensible defaults.
func DefaultCorrelatorConfig() *CorrelatorConfig {
	return &CorrelatorConfig{
		GRPCListenAddr: envOrDefault("GRPC_LISTEN_ADDR", ":4318"),
		LLMProvider:    envOrDefault("LLM_PROVIDER", "claude"),
		LLMModel:       envOrDefault("LLM_MODEL", "claude-3-sonnet-20240229"),
		LLMAPIKey:      os.Getenv("LLM_API_KEY"),
		LLMEndpoint:    envOrDefault("LLM_ENDPOINT", ""),
		LLMTimeout:     30 * time.Second,
		LLMMaxRetries:  3,
		ScrubSecrets:   true,
		ScrubLogs:      true,
		MaxPromptTokens: 4096,
	}
}

// DefaultOperatorConfig returns an OperatorConfig with sensible defaults.
func DefaultOperatorConfig() *OperatorConfig {
	return &OperatorConfig{
		MaxActionsPerPodPerHour:  3,
		MinCPUThrottlePercent:    10.0,
		ProtectedNamespaces:      []string{"kube-system"},
		RequireApprovalForIsolate: true,
		MaxCordonnedNodePercent:  20.0,
		DryRun:                   envOrDefault("DRY_RUN", "false") == "true",
		DefaultRevertDuration:    5 * time.Minute,
	}
}

// Validate checks the AgentConfig for required fields.
func (c *AgentConfig) Validate() error {
	if c.NodeName == "" {
		return fmt.Errorf("NODE_NAME is required")
	}
	if c.CollectorEndpoint == "" {
		return fmt.Errorf("COLLECTOR_ENDPOINT is required")
	}
	return nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envOrDefaultInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
