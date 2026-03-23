package classifier_test

import (
	"log/slog"
	"testing"

	"github.com/kernelview/kernelview/internal/collector/classifier"
)

func TestClassifier_ClassifyApplication(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())

	tests := []struct {
		name         string
		signal       classifier.Signals
		expectedCode string
	}{
		{
			name: "Authentication Failure (Fast Crash with Network Call)",
			signal: classifier.Signals{
				ExitCode:                1,
				StartupDurationMs:       1500, // < 2 seconds
				HasNetworkCallInStartup: true,
				NetworkCallFailed:       true,
			},
			expectedCode: "APP-001",
		},
		{
			name: "Configuration Error (Fast Crash with No Network Call)",
			signal: classifier.Signals{
				ExitCode:                1,
				StartupDurationMs:       800, // < 2 seconds
				HasNetworkCallInStartup: false,
			},
			expectedCode: "APP-003",
		},
		{
			name: "OOM Kill Exit Code 137",
			signal: classifier.Signals{
				ExitCode: 137,
			},
			expectedCode: "APP-002",
		},
		{
			name: "Clean Exit Code 0 (Failing Readiness Probe)",
			signal: classifier.Signals{
				ExitCode:              0,
				ReadinessProbeFailure: true,
			},
			expectedCode: "APP-004",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Classify(tt.signal)
			if result == nil {
				t.Fatalf("Expected classification, got nil")
			}
			if string(result.Type) != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, result.Type)
			}
		})
	}
}

func TestClassifier_PriorityRouting(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())

	signal := classifier.Signals{
		EtcdWALFsyncP99Ms: 150,  // Triggers CTL-001
		ExitCode:          137,  // Triggers APP-002
		OOMKillDetected:   true, // Triggers MEM-001
	}

	result := c.Classify(signal)
	if result == nil {
		t.Fatalf("Expected classification, got nil")
	}

	// CTL-001 etcd I/O saturation has highest priority (Priority 1)
	if string(result.Type) != "CTL-001" {
		t.Errorf("Priority routing failed. Expected CTL-001, got %s", result.Type)
	}
}

func TestClassifier_NetworkMTUMismatch(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())

	signal := classifier.Signals{
		RetransmitRate:            0.5, // Not high enough for general retransmit
		LargePacketRetransmitRate: 6.0, // High large packet retransmit
	}

	result := c.Classify(signal)
	if result == nil {
		t.Fatalf("Expected classification, got nil")
	}

	// MTU Mismatch signature
	if string(result.Type) != "NET-003" {
		t.Errorf("Expected NET-003 MTU mismatch, got %s", result.Type)
	}
}
