// Package prompts provides per-incident-type prompt templates for KernelView.
// Every incident type has a dedicated prompt that includes known patterns,
// edge cases, and the specific data fields relevant to that incident class.
package prompts

import (
	"fmt"
	"strings"
)

// Builder selects and populates the correct prompt template for an incident type.
type Builder struct{}

// NewBuilder creates a new prompt builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// Build returns the complete prompt for an incident type with populated fields.
func (b *Builder) Build(incidentCode string, data map[string]string) string {
	var template string

	switch {
	// MEM family
	case strings.HasPrefix(incidentCode, "MEM-001"), strings.HasPrefix(incidentCode, "MEM-002"):
		template = oomKillTemplate()
	case strings.HasPrefix(incidentCode, "MEM-003"), strings.HasPrefix(incidentCode, "MEM-004"):
		template = memoryLeakTemplate()
	case strings.HasPrefix(incidentCode, "MEM-005"):
		template = nodeMemPressureTemplate()
	case strings.HasPrefix(incidentCode, "MEM-006"):
		template = jvmHeapTemplate()

	// CPU family
	case strings.HasPrefix(incidentCode, "CPU-001"):
		template = cfsThrottleTemplate()
	case strings.HasPrefix(incidentCode, "CPU-002"):
		template = noisyNeighborTemplate()
	case strings.HasPrefix(incidentCode, "CPU-003"):
		template = nodeCPUTemplate()
	case strings.HasPrefix(incidentCode, "CPU-006"):
		template = cpuLimitTightTemplate()

	// NET family
	case strings.HasPrefix(incidentCode, "NET-001"):
		template = dnsNdotsTemplate()
	case strings.HasPrefix(incidentCode, "NET-003"):
		template = mtuMismatchTemplate()
	case strings.HasPrefix(incidentCode, "NET-007"):
		template = conntrackTemplate()

	// APP family (all use the CrashLoop decision tree prompt)
	case strings.HasPrefix(incidentCode, "APP-"):
		template = crashLoopTemplate()

	// CTL family
	case strings.HasPrefix(incidentCode, "CTL-001"):
		template = etcdIOTemplate()
	case strings.HasPrefix(incidentCode, "CTL-003"):
		template = webhookTimeoutTemplate()

	default:
		template = genericTemplate(incidentCode)
	}

	// Populate template with data fields
	return populateTemplate(template, data)
}

// MaxTokensForProvider returns the context window limit for truncation.
func MaxTokensForProvider(provider string) int {
	switch provider {
	case "gemini":
		return 1_000_000 // 1M tokens — use for multi-service cascading failures
	case "anthropic":
		return 200_000
	case "openai":
		return 128_000
	case "ollama":
		return 32_000
	default:
		return 32_000
	}
}

// populateTemplate replaces {key} placeholders with values from data.
func populateTemplate(template string, data map[string]string) string {
	result := template
	for k, v := range data {
		result = strings.ReplaceAll(result, "{"+k+"}", v)
	}
	return result
}

func genericTemplate(code string) string {
	return fmt.Sprintf(`INCIDENT CLASS: %s
AFFECTED SERVICE: {service_name} in {namespace}
POD: {pod_name}  NODE: {node_name}
TIMESTAMP: {timestamp}

AVAILABLE METRICS:
{metrics_json}

KUBERNETES EVENTS (last 10 minutes):
{k8s_events_json}

POD LOGS (last 30 lines, scrubbed):
{pod_logs}

Respond with JSON: {root_cause, trigger, confidence, severity,
recommended_action, auto_remediation_safe, action_type}`, code)
}
