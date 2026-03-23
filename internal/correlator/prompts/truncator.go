package prompts

import (
	"strings"
)

// Truncator handles context-window-aware prompt truncation.
// Different providers have different limits:
//   Gemini:    1,000,000 tokens
//   Claude:      200,000 tokens
//   OpenAI:      128,000 tokens
//   Ollama:    8,000–32,000 tokens
//
// Truncation priority (last to be cut → first to be cut):
//   1. K8s events — always preserved (smallest, most diagnostic)
//   2. Metrics — preserved (compact JSON)
//   3. Dependencies — preserved if cascading
//   4. Logs — truncated first (largest, noisy)
type Truncator struct {
	maxTokens int
}

// NewTruncator creates a truncator for a specific provider.
func NewTruncator(provider string) *Truncator {
	return &Truncator{
		maxTokens: MaxTokensForProvider(provider),
	}
}

// TruncateBundle ensures the total prompt stays within the context window.
// Returns the truncated data map.
func (t *Truncator) TruncateBundle(data map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range data {
		result[k] = v
	}

	totalChars := t.estimateTotalChars(result)
	// Rough heuristic: 1 token ≈ 4 characters
	maxChars := t.maxTokens * 4

	if totalChars <= maxChars {
		return result // Fits within context
	}

	// Truncation order: logs → metrics window → dependencies
	// Always preserve: K8s events, service name, incident code

	// Step 1: Truncate logs
	if logs, ok := result["pod_logs"]; ok {
		lines := strings.Split(logs, "\n")
		for len(lines) > 10 && t.estimateTotalChars(result) > maxChars {
			lines = lines[1:] // Remove oldest lines first
			result["pod_logs"] = strings.Join(lines, "\n")
		}
	}

	// Step 2: Truncate timeseries data
	for _, key := range []string{"memory_rss_timeseries_json", "requests_per_second_timeseries"} {
		if ts, ok := result[key]; ok && t.estimateTotalChars(result) > maxChars {
			// Keep first and last 20% of timeseries
			lines := strings.Split(ts, "\n")
			if len(lines) > 20 {
				keep := len(lines) / 5
				truncated := append(lines[:keep], "... [TRUNCATED] ...")
				truncated = append(truncated, lines[len(lines)-keep:]...)
				result[key] = strings.Join(truncated, "\n")
			}
		}
	}

	// Step 3: Truncate dependency/network data
	for _, key := range []string{"network_calls_json", "dns_query_sample_json", "external_domains_json"} {
		if v, ok := result[key]; ok && t.estimateTotalChars(result) > maxChars {
			if len(v) > 500 {
				result[key] = v[:500] + "... [TRUNCATED]"
			}
		}
	}

	// Step 4: If still over limit, truncate syscall and metric data
	for _, key := range []string{"syscall_sequence_json", "metrics_json"} {
		if v, ok := result[key]; ok && t.estimateTotalChars(result) > maxChars {
			if len(v) > 300 {
				result[key] = v[:300] + "... [TRUNCATED]"
			}
		}
	}

	return result
}

func (t *Truncator) estimateTotalChars(data map[string]string) int {
	total := 0
	for _, v := range data {
		total += len(v)
	}
	return total
}
