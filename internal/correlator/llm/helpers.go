package llm

import (
	"encoding/json"
	"strings"
)

// buildSystemPrompt returns the shared system prompt from spec §4.1.
func buildSystemPrompt() string {
	return `You are KernelView, an expert Site Reliability Engineer AI with deep knowledge of:
- Linux kernel internals, eBPF, cgroups v1/v2, TCP/IP networking
- Kubernetes internals: kubelet, kube-proxy, CoreDNS, etcd, HPA, VPA
- Real production incident patterns from companies including Zalando, Algolia,
  Spotify, Datadog, Target, and documented k8s.af failure stories
- CFS CPU scheduling, kernel OOM killer, conntrack, overlay networking

Rules:
1. Always distinguish root_cause (systemic) from trigger (proximate event)
2. Always state confidence: HIGH (>80% certain), MEDIUM (50-80%), LOW (<50%)
3. Never recommend restarting a service as a root cause fix — restarts mask problems
4. Always check for cascading failure patterns before assigning single root cause
5. If logs contain [REDACTED], acknowledge data was scrubbed and state what's missing
6. Output ONLY valid JSON. No markdown. No explanation outside JSON fields.`
}

// parseCorrelationJSON attempts to parse a CorrelationResult from LLM output text.
// Handles both clean JSON and JSON embedded in markdown code blocks.
func parseCorrelationJSON(text string) *CorrelationResult {
	result := &CorrelationResult{}

	// Try direct JSON parse first
	cleaned := strings.TrimSpace(text)

	// Strip markdown code fences if present
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		// Remove first line (```json) and last line (```)
		if len(lines) > 2 {
			cleaned = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	if err := json.Unmarshal([]byte(cleaned), result); err != nil {
		// JSON parse failed — try to extract from text
		result.ParseError = "failed to parse JSON: " + err.Error()
		result.RawText = text

		// Try to extract key fields from text
		result.RootCause = extractField(text, "root_cause")
		result.Confidence = extractConfidence(text)
		result.RecommendedAction = extractField(text, "recommended_action")

		if result.RootCause == "" {
			result.RootCause = text // Last resort: use the raw text
		}
	}

	// Normalize confidence
	result.Confidence = normalizeConfidence(result.Confidence)

	return result
}

// extractField tries to extract a JSON field value from text.
func extractField(text, field string) string {
	// Look for "field": "value" or "field":"value"
	patterns := []string{
		`"` + field + `": "`,
		`"` + field + `":"`,
	}

	for _, pattern := range patterns {
		idx := strings.Index(text, pattern)
		if idx < 0 {
			continue
		}
		start := idx + len(pattern)
		end := strings.Index(text[start:], `"`)
		if end < 0 {
			continue
		}
		return text[start : start+end]
	}
	return ""
}

// extractConfidence looks for HIGH/MEDIUM/LOW in the text.
func extractConfidence(text string) string {
	upper := strings.ToUpper(text)
	if strings.Contains(upper, "HIGH") {
		return "HIGH"
	}
	if strings.Contains(upper, "MEDIUM") {
		return "MEDIUM"
	}
	if strings.Contains(upper, "LOW") {
		return "LOW"
	}
	return "UNKNOWN"
}

// normalizeConfidence ensures confidence is one of the valid values.
func normalizeConfidence(c string) string {
	switch strings.ToUpper(strings.TrimSpace(c)) {
	case "HIGH":
		return "HIGH"
	case "MEDIUM":
		return "MEDIUM"
	case "LOW":
		return "LOW"
	default:
		return "UNKNOWN"
	}
}
