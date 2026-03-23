// Package prompt builds structured prompts for the AI Correlator.
package prompt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AnomalyContext contains all data needed to build the LLM prompt.
type AnomalyContext struct {
	AnomalyType     string
	ServiceName     string
	Namespace       string
	Timestamp       string
	MetricsJSON     string
	K8sEvents       string
	PodLogs         []string
	UpstreamMetrics string
	SyscallRates    string
	OOMEvents       string
}

// MaxTokens is the maximum prompt size before truncation.
const MaxTokens = 4096

// Build constructs the LLM prompt from the anomaly context.
// Implements truncation priority order from PRD §9.3:
// 1. Remove log lines first (least structured)
// 2. Reduce metric time window from 60s to 30s
// 3. Reduce upstream dependencies from 5 to 2
// 4. Always preserve K8s events (most informative)
func Build(ctx AnomalyContext) string {
	prompt := buildFullPrompt(ctx)

	// Estimate token count (rough: 4 chars ≈ 1 token)
	estimatedTokens := len(prompt) / 4

	if estimatedTokens <= MaxTokens {
		return prompt
	}

	// Truncation phase 1: Remove log lines
	ctx.PodLogs = truncateLogs(ctx.PodLogs, 10)
	prompt = buildFullPrompt(ctx)
	if len(prompt)/4 <= MaxTokens {
		return prompt
	}

	// Truncation phase 2: Further reduce logs
	ctx.PodLogs = truncateLogs(ctx.PodLogs, 5)
	prompt = buildFullPrompt(ctx)
	if len(prompt)/4 <= MaxTokens {
		return prompt
	}

	// Truncation phase 3: Remove logs entirely
	ctx.PodLogs = nil
	prompt = buildFullPrompt(ctx)
	if len(prompt)/4 <= MaxTokens {
		return prompt
	}

	// Truncation phase 4: Reduce upstream metrics
	ctx.UpstreamMetrics = truncateJSON(ctx.UpstreamMetrics, 2)
	prompt = buildFullPrompt(ctx)

	return prompt
}

func buildFullPrompt(ctx AnomalyContext) string {
	var b strings.Builder

	b.WriteString("You are an expert SRE analyzing a Kubernetes incident.\n\n")
	b.WriteString(fmt.Sprintf("ANOMALY: %s detected on service %s\n", ctx.AnomalyType, ctx.ServiceName))
	b.WriteString(fmt.Sprintf("NAMESPACE: %s\n", ctx.Namespace))
	b.WriteString(fmt.Sprintf("TIME: %s\n\n", ctx.Timestamp))

	if ctx.MetricsJSON != "" {
		b.WriteString("METRICS (last 60 seconds):\n")
		b.WriteString(ctx.MetricsJSON)
		b.WriteString("\n\n")
	}

	if ctx.K8sEvents != "" {
		b.WriteString("KUBERNETES EVENTS:\n")
		b.WriteString(ctx.K8sEvents)
		b.WriteString("\n\n")
	}

	if len(ctx.PodLogs) > 0 {
		b.WriteString(fmt.Sprintf("RECENT LOGS (last %d lines):\n", len(ctx.PodLogs)))
		for _, line := range ctx.PodLogs {
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if ctx.UpstreamMetrics != "" {
		b.WriteString("UPSTREAM SERVICES STATUS:\n")
		b.WriteString(ctx.UpstreamMetrics)
		b.WriteString("\n\n")
	}

	if ctx.SyscallRates != "" {
		b.WriteString("SYSCALL RATES:\n")
		b.WriteString(ctx.SyscallRates)
		b.WriteString("\n\n")
	}

	if ctx.OOMEvents != "" {
		b.WriteString("OOM EVENTS (last 10 minutes):\n")
		b.WriteString(ctx.OOMEvents)
		b.WriteString("\n\n")
	}

	b.WriteString("Provide:\n")
	b.WriteString("(1) Most likely root cause in 2 sentences.\n")
	b.WriteString("(2) Confidence: HIGH / MEDIUM / LOW.\n")
	b.WriteString("(3) Recommended immediate action.\n")
	b.WriteString("(4) Whether automated remediation is safe: YES / NO / REQUIRES_HUMAN.\n")
	b.WriteString("(5) Remediation type if applicable: THROTTLE_POD / RESTART_POD / ISOLATE_POD / ADJUST_LIMITS / NONE.\n")

	return b.String()
}

func truncateLogs(logs []string, maxLines int) []string {
	if len(logs) <= maxLines {
		return logs
	}
	return logs[len(logs)-maxLines:]
}

func truncateJSON(jsonStr string, maxItems int) string {
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		return jsonStr
	}
	if len(items) <= maxItems {
		return jsonStr
	}
	items = items[:maxItems]
	result, _ := json.Marshal(items)
	return string(result)
}

// ParseResponse extracts structured fields from the LLM response text.
type ParsedResponse struct {
	RootCause       string
	Confidence      string
	Action          string
	RemediationSafe string
	RemediationType string
	ParseSuccess    bool
	ParseError      string
	RawText         string
}

// Parse attempts to extract structured data from the LLM response.
func Parse(response string) *ParsedResponse {
	result := &ParsedResponse{
		RawText:      response,
		ParseSuccess: true,
	}

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "(1)") || strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "1)") {
			result.RootCause = strings.TrimSpace(strings.TrimLeft(line, "(1).) "))
		}
		if strings.HasPrefix(line, "(2)") || strings.HasPrefix(line, "2.") || strings.HasPrefix(line, "2)") {
			upper := strings.ToUpper(line)
			if strings.Contains(upper, "HIGH") {
				result.Confidence = "HIGH"
			} else if strings.Contains(upper, "MEDIUM") {
				result.Confidence = "MEDIUM"
			} else if strings.Contains(upper, "LOW") {
				result.Confidence = "LOW"
			}
		}
		if strings.HasPrefix(line, "(3)") || strings.HasPrefix(line, "3.") || strings.HasPrefix(line, "3)") {
			result.Action = strings.TrimSpace(strings.TrimLeft(line, "(3).) "))
		}
		if strings.HasPrefix(line, "(4)") || strings.HasPrefix(line, "4.") || strings.HasPrefix(line, "4)") {
			upper := strings.ToUpper(line)
			if strings.Contains(upper, "YES") || strings.Contains(upper, "SAFE") {
				result.RemediationSafe = "SAFE"
			} else if strings.Contains(upper, "REQUIRES_HUMAN") || strings.Contains(upper, "HUMAN") {
				result.RemediationSafe = "HUMAN_REQUIRED"
			} else if strings.Contains(upper, "NO") || strings.Contains(upper, "UNSAFE") {
				result.RemediationSafe = "UNSAFE"
			}
		}
		if strings.HasPrefix(line, "(5)") || strings.HasPrefix(line, "5.") || strings.HasPrefix(line, "5)") {
			upper := strings.ToUpper(line)
			for _, rt := range []string{"THROTTLE_POD", "RESTART_POD", "ISOLATE_POD", "ADJUST_LIMITS", "NONE"} {
				if strings.Contains(upper, rt) {
					result.RemediationType = rt
					break
				}
			}
		}
	}

	// Mark as failed if we couldn't extract key fields
	if result.RootCause == "" && result.Confidence == "" {
		result.ParseSuccess = false
		result.ParseError = "could not extract structured response from LLM output"
		result.Confidence = "PARSE_ERROR"
	}

	return result
}
