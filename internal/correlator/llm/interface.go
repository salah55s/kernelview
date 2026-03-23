// Package llm provides the multi-provider LLM integration layer for KernelView.
// It defines the common interface that all LLM clients implement and the types
// shared across providers.
package llm

import (
	"context"
	"time"
)

// Provider identifies the LLM backend.
type Provider string

const (
	ProviderClaude Provider = "anthropic"
	ProviderGemini Provider = "gemini"
	ProviderOpenAI Provider = "openai"
	ProviderOllama Provider = "ollama"
)

// Severity tiers for incident routing.
type Severity string

const (
	SeverityP0Emergency Severity = "P0"
	SeverityP1Critical  Severity = "P1"
	SeverityP2High      Severity = "P2"
	SeverityP3Medium    Severity = "P3"
	SeverityP4Info      Severity = "P4"
)

// IncidentType is used by the router for type-specific provider selection.
type IncidentType string

const (
	IncidentCodeCrash IncidentType = "APP_CRASH"
	IncidentGeneric   IncidentType = "GENERIC"
)

// ProviderConfig holds the configuration for a specific provider + model combo.
type ProviderConfig struct {
	Provider Provider
	Model    string
	APIKey   string
	Endpoint string
	Timeout  time.Duration
}

// AnomalyBundle is the input to the LLM for correlation.
type AnomalyBundle struct {
	IncidentType    string
	IncidentCode    string // e.g., "MEM-001", "CPU-001"
	Severity        Severity
	ServiceName     string
	Namespace       string
	NodeName        string
	PodName         string
	Timestamp       time.Time

	// Metrics and signals
	MetricsJSON       string
	K8sEventsJSON     string
	PodLogs           []string
	SyscallRates      string
	NetworkSignals    string
	MemoryTimeline    string
	CPUMetrics        string
	DNSQueryData      string

	// Context for classification
	ExitCode          int
	StartupDurationMs int
	RestartCount      int
	CgroupMemoryLimit int64
	RSSAtEvent        int64
}

// CorrelationResult is the structured output from the LLM.
type CorrelationResult struct {
	RootCause           string   `json:"root_cause"`
	Trigger             string   `json:"trigger,omitempty"`
	Confidence          string   `json:"confidence"` // HIGH, MEDIUM, LOW
	Severity            string   `json:"severity"`   // P0, P1, P2, P3
	IncidentClass       string   `json:"incident_class,omitempty"`
	RecommendedAction   string   `json:"recommended_action"`
	AutoRemediationSafe bool     `json:"auto_remediation_safe"`
	ActionType          string   `json:"action_type,omitempty"`
	AffectedRuntime     string   `json:"affected_language_runtime,omitempty"`
	EstimatedFixTime    string   `json:"estimated_time_to_fix,omitempty"`
	ContributingFactors []string `json:"contributing_factors,omitempty"`

	// Metadata
	Provider   Provider      `json:"provider"`
	Model      string        `json:"model"`
	Latency    time.Duration `json:"latency"`
	TokensIn   int           `json:"tokens_in"`
	TokensOut  int           `json:"tokens_out"`
	RawText    string        `json:"-"`
	ParseError string        `json:"parse_error,omitempty"`
}

// LLMClient is the interface all providers must implement.
type LLMClient interface {
	// Correlate sends an anomaly bundle to the LLM and returns a structured result.
	Correlate(ctx context.Context, bundle AnomalyBundle, prompt string) (*CorrelationResult, error)

	// Provider returns the provider identifier.
	Provider() Provider

	// Model returns the current model name.
	Model() string

	// Healthy returns true if the provider is reachable.
	Healthy(ctx context.Context) bool
}
