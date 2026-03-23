// Package models provides shared data types used across KernelView components.
package models

import "time"

// ServiceInfo represents a discovered Kubernetes service.
type ServiceInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	ClusterIP string `json:"clusterIP,omitempty"`
	PodCount  int    `json:"podCount"`
}

// ServiceMapNode is a node in the service dependency graph.
type ServiceMapNode struct {
	Service      ServiceInfo `json:"service"`
	RequestRate  float64     `json:"requestRate"`  // requests/sec
	ErrorRate    float64     `json:"errorRate"`     // percentage
	LatencyP50   float64     `json:"latencyP50"`   // ms
	LatencyP99   float64     `json:"latencyP99"`   // ms
	Health       HealthState `json:"health"`
	HasAnomaly   bool        `json:"hasAnomaly"`
	HasRemediation bool      `json:"hasRemediation"`
}

// ServiceMapEdge is an edge in the service dependency graph.
type ServiceMapEdge struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	RequestRate float64 `json:"requestRate"`
	ErrorRate   float64 `json:"errorRate"`
}

// HealthState represents the health of a service.
type HealthState string

const (
	HealthHealthy  HealthState = "healthy"
	HealthDegraded HealthState = "degraded"
	HealthCritical HealthState = "critical"
	HealthLearning HealthState = "learning"
	HealthUnknown  HealthState = "unknown"
)

// Anomaly represents a detected anomaly.
type Anomaly struct {
	ID            string      `json:"id"`
	Type          string      `json:"type"` // "latency_spike", "error_rate_spike", "noisy_neighbor", "oom_kill"
	Service       string      `json:"service"`
	Namespace     string      `json:"namespace"`
	Pod           string      `json:"pod,omitempty"`
	Node          string      `json:"node,omitempty"`
	DetectedAt    time.Time   `json:"detectedAt"`
	ResolvedAt    *time.Time  `json:"resolvedAt,omitempty"`
	Severity      string      `json:"severity"` // "warning", "critical"
	Description   string      `json:"description"`
	CorrelationID string      `json:"correlationId,omitempty"`
}

// Incident is a correlated incident with AI analysis.
type Incident struct {
	ID                string             `json:"id"`
	Anomalies         []Anomaly          `json:"anomalies"`
	RootCause         string             `json:"rootCause,omitempty"`
	Confidence        string             `json:"confidence,omitempty"`
	RecommendedAction string             `json:"recommendedAction,omitempty"`
	RemediationActions []RemediationSummary `json:"remediationActions,omitempty"`
	CreatedAt         time.Time          `json:"createdAt"`
	ResolvedAt        *time.Time         `json:"resolvedAt,omitempty"`
	Status            string             `json:"status"` // "active", "resolved", "silenced"
}

// RemediationSummary is a brief view of a remediation action.
type RemediationSummary struct {
	ID         string    `json:"id"`
	Action     string    `json:"action"`
	TargetPod  string    `json:"targetPod"`
	Phase      string    `json:"phase"`
	ExecutedAt time.Time `json:"executedAt,omitempty"`
}

// RightSizingRecommendation is a cost optimization recommendation.
type RightSizingRecommendation struct {
	Deployment       string  `json:"deployment"`
	Namespace        string  `json:"namespace"`
	Resource         string  `json:"resource"` // "cpu" or "memory"
	CurrentLimit     string  `json:"currentLimit"`
	P99ActualUsage   string  `json:"p99ActualUsage"`
	RecommendedLimit string  `json:"recommendedLimit"`
	MonthlySaving    float64 `json:"monthlySaving"` // USD
	DataDays         int     `json:"dataDays"`       // How many days of data
}

// TraceEntry represents a single HTTP trace for the dashboard.
type TraceEntry struct {
	TraceID    string    `json:"traceId"`
	Timestamp  time.Time `json:"timestamp"`
	Service    string    `json:"service"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"statusCode"`
	LatencyMs  float64   `json:"latencyMs"`
	Upstream   string    `json:"upstream,omitempty"`
}
