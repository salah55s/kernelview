package prompts_test

import (
	"strings"
	"testing"

	"github.com/kernelview/kernelview/internal/correlator/prompts"
)

func TestBuilder_RoutesToCorrectTemplate(t *testing.T) {
	b := prompts.NewBuilder()

	tests := []struct {
		code          string
		expectContain string
	}{
		{"MEM-001", "OOM_KILL"},
		{"MEM-003", "MEMORY_LEAK"},
		{"MEM-005", "NODE_MEMORY_PRESSURE"},
		{"MEM-006", "JVM_HEAP_EXHAUSTION"},
		{"CPU-001", "CPU_THROTTLING"},
		{"CPU-002", "NOISY_NEIGHBOR"},
		{"CPU-003", "NODE_CPU_SATURATION"},
		{"CPU-006", "CPU_LIMIT_TOO_TIGHT"},
		{"NET-001", "DNS_NDOTS_INEFFICIENCY"},
		{"NET-003", "MTU_MISMATCH"},
		{"NET-007", "CONNTRACK_EXHAUSTION"},
		{"APP-001", "CRASH_LOOP_BACKOFF"},
		{"APP-005", "CRASH_LOOP_BACKOFF"},
		{"CTL-001", "ETCD_IO_SATURATION"},
		{"CTL-003", "WEBHOOK_TIMEOUT_CASCADE"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := b.Build(tt.code, map[string]string{})
			if !strings.Contains(result, tt.expectContain) {
				t.Errorf("Prompt for %s should contain '%s', got:\n%s", tt.code, tt.expectContain, result[:200])
			}
		})
	}
}

func TestBuilder_PopulatesPlaceholders(t *testing.T) {
	b := prompts.NewBuilder()
	data := map[string]string{
		"pod_name":  "payment-svc-7f8c-abc12",
		"namespace": "production",
		"node_name": "node-01",
	}

	result := b.Build("MEM-001", data)

	if !strings.Contains(result, "payment-svc-7f8c-abc12") {
		t.Error("Expected pod_name to be populated in prompt")
	}
	if !strings.Contains(result, "production") {
		t.Error("Expected namespace to be populated in prompt")
	}
	if !strings.Contains(result, "node-01") {
		t.Error("Expected node_name to be populated in prompt")
	}
}

func TestBuilder_UnknownCodeFallsToGeneric(t *testing.T) {
	b := prompts.NewBuilder()
	result := b.Build("NEW-999", map[string]string{"service_name": "test-svc"})

	if !strings.Contains(result, "NEW-999") {
		t.Error("Generic template should include the incident code")
	}
	if !strings.Contains(result, "test-svc") {
		t.Error("Generic template should populate service_name")
	}
}

func TestTruncator_FitsWithinLimit(t *testing.T) {
	tr := prompts.NewTruncator("ollama") // Smallest window: 32k tokens = 128k chars

	// Create data significantly larger than 128k chars to force truncation
	data := map[string]string{
		"pod_logs":    strings.Repeat("ERROR 2026-03-23 payment-svc OOM kill detected in production namespace with pid 12345\n", 2000), // ~180k chars
		"k8s_events_json": `{"event": "OOMKilled"}`,       // Small — must be preserved
	}

	result := tr.TruncateBundle(data)

	// K8s events should be preserved (truncation priority: logs first)
	if result["k8s_events_json"] != `{"event": "OOMKilled"}` {
		t.Error("K8s events should be preserved during truncation")
	}

	// Logs should be truncated
	if len(result["pod_logs"]) >= len(data["pod_logs"]) {
		t.Error("pod_logs should have been truncated")
	}
}

func TestTruncator_NoTruncationNeeded(t *testing.T) {
	tr := prompts.NewTruncator("gemini") // 1M token window

	data := map[string]string{
		"pod_logs": "short log",
		"metrics_json": `{"cpu": 0.5}`,
	}

	result := tr.TruncateBundle(data)

	if result["pod_logs"] != "short log" {
		t.Error("Short data should not be modified")
	}
}

func TestMaxTokensForProvider(t *testing.T) {
	tests := map[string]int{
		"gemini":    1_000_000,
		"anthropic": 200_000,
		"openai":    128_000,
		"ollama":    32_000,
		"unknown":   32_000,
	}

	for provider, expected := range tests {
		got := prompts.MaxTokensForProvider(provider)
		if got != expected {
			t.Errorf("Provider %s: expected %d, got %d", provider, expected, got)
		}
	}
}
