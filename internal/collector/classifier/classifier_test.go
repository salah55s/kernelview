package classifier_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/kernelview/kernelview/internal/collector/classifier"
)

// ──────────────────────────────────────────────────────────────────
// Memory Family Tests (MEM-001 → MEM-006)
// ──────────────────────────────────────────────────────────────────

func TestMemory_OOMKillKernel(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		OOMKillDetected: true,
		OOMKillSource:   "kernel",
	})
	assertClassification(t, result, "MEM-001", classifier.FamilyMemory, classifier.SevP1)
}

func TestMemory_OOMKillCgroup(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		OOMKillDetected: true,
		OOMKillSource:   "cgroup",
	})
	assertClassification(t, result, "MEM-002", classifier.FamilyMemory, classifier.SevP1)
}

func TestMemory_LeakSustained(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		RSSGrowthMBPerMin: 7,
		RSSGrowthDuration: 12 * time.Minute,
	})
	assertClassification(t, result, "MEM-003", classifier.FamilyMemory, classifier.SevP2)
}

func TestMemory_LeakNotSustainedEnough(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		RSSGrowthMBPerMin: 7,
		RSSGrowthDuration: 5 * time.Minute, // < 10 min threshold
	})
	// Should NOT classify as MEM-003 — insufficient duration
	if result != nil {
		t.Errorf("Expected nil (no classification), got %s", result.Type)
	}
}

func TestMemory_GrowthSpike(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		RSSGrowthMBPerMin: 25,
	})
	assertClassification(t, result, "MEM-004", classifier.FamilyMemory, classifier.SevP1)
}

func TestMemory_NodePressure(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		NodeMemoryPercent: 92,
		NodeMemDuration:   7 * time.Minute,
	})
	assertClassification(t, result, "MEM-005", classifier.FamilyMemory, classifier.SevP1)
}

func TestMemory_JVMHeap(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		IsJVMProcess:   true,
		JVMHeapPercent: 97,
	})
	assertClassification(t, result, "MEM-006", classifier.FamilyMemory, classifier.SevP1)
}

func TestMemory_JVMHeapPriorityOverLeak(t *testing.T) {
	// Edge case §6.3: JVM heap should be checked BEFORE generic memory leak
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		IsJVMProcess:      true,
		JVMHeapPercent:    98,
		RSSGrowthMBPerMin: 25, // Would match MEM-004
	})
	assertClassification(t, result, "MEM-006", classifier.FamilyMemory, classifier.SevP1)
}

// ──────────────────────────────────────────────────────────────────
// CPU Family Tests (CPU-001 → CPU-006)
// ──────────────────────────────────────────────────────────────────

func TestCPU_CFSThrottleWithLatencyImpact(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ThrottleRatio: 0.40,
		LatencyP50Ms:  5,
		LatencyP99Ms:  50, // 10x p50 → > 3x threshold
	})
	assertClassification(t, result, "CPU-001", classifier.FamilyCPU, classifier.SevP2)
}

func TestCPU_ThrottleAtLowAvgCPU(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ThrottleRatio: 0.30,
		AvgCPUPercent: 20, // Low average CPU — limit is too tight for burst
	})
	assertClassification(t, result, "CPU-006", classifier.FamilyCPU, classifier.SevP3)
}

func TestCPU_NoisyNeighbor(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		SyscallRate:       15000,
		SyscallRateMean:   3000,
		SyscallRateStdDev: 2000,       // threshold = 3000 + 3*2000 = 9000
		SyscallDuration:   45 * time.Second,
	})
	assertClassification(t, result, "CPU-002", classifier.FamilyCPU, classifier.SevP2)
}

func TestCPU_NoisyNeighborBelowThreshold(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		SyscallRate:       8000,         // Below threshold 9000
		SyscallRateMean:   3000,
		SyscallRateStdDev: 2000,
		SyscallDuration:   45 * time.Second,
	})
	if result != nil {
		t.Errorf("Expected nil (below threshold), got %s", result.Type)
	}
}

func TestCPU_NodeSaturation(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		NodeCPUPercent:  97,
		NodeCPUDuration: 5 * time.Minute,
	})
	assertClassification(t, result, "CPU-003", classifier.FamilyCPU, classifier.SevP1)
}

func TestCPU_SchedStarvation(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		RunqueueWaitMs: 75, // > 50ms threshold
	})
	assertClassification(t, result, "CPU-004", classifier.FamilyCPU, classifier.SevP2)
}

// ──────────────────────────────────────────────────────────────────
// Network Family Tests (NET-001 → NET-007)
// ──────────────────────────────────────────────────────────────────

func TestNetwork_DNSNdots(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		NXDOMAINRate: 55,
	})
	assertClassification(t, result, "NET-001", classifier.FamilyNetwork, classifier.SevP2)
}

func TestNetwork_CoreDNSOverload(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		CoreDNSCPUPercent: 90,
		DNSLatencyMs:      60,
	})
	assertClassification(t, result, "NET-002", classifier.FamilyNetwork, classifier.SevP1)
}

func TestNetwork_MTUMismatch(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		LargePacketRetransmitRate: 8,  // > 5%
		RetransmitRate:            2,  // < 5% — only large packets affected
	})
	assertClassification(t, result, "NET-003", classifier.FamilyNetwork, classifier.SevP2)
}

func TestNetwork_MTUMismatch_NotIfGeneralRetransmitHigh(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		LargePacketRetransmitRate: 8,
		RetransmitRate:            8, // BOTH are high → not MTU, it's general network issue
	})
	// Should classify as TCP retransmit storm (NET-005), not MTU
	if result != nil && string(result.Type) == "NET-003" {
		t.Errorf("Expected NOT NET-003 when both rates are high, got %s", result.Type)
	}
}

func TestNetwork_ConnectionFlood(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		NewTCPConnsPerSec: 800,
	})
	assertClassification(t, result, "NET-004", classifier.FamilyNetwork, classifier.SevP1)
}

func TestNetwork_ConntrackExhaustion(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ConntrackPercent: 95,
	})
	assertClassification(t, result, "NET-007", classifier.FamilyNetwork, classifier.SevP1)
}

func TestNetwork_EndpointChurn(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		EndpointChangesPerMin: 15,
	})
	assertClassification(t, result, "NET-006", classifier.FamilyNetwork, classifier.SevP2)
}

// ──────────────────────────────────────────────────────────────────
// Application Family Tests (APP-001 → APP-008)
// ──────────────────────────────────────────────────────────────────

func TestApp_AuthFailure_401(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ExitCode:                1,
		StartupDurationMs:       500,
		HasNetworkCallInStartup: true,
		NetworkCallFailed:       true,
		NetworkCallResult:       401,
	})
	assertClassification(t, result, "APP-001", classifier.FamilyApplication, classifier.SevP1)
	if result.Confidence != 0.90 {
		t.Errorf("401 auth failure should have 0.90 confidence, got %.2f", result.Confidence)
	}
}

func TestApp_AuthFailure_GenericNetwork(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ExitCode:                1,
		StartupDurationMs:       800,
		HasNetworkCallInStartup: true,
		NetworkCallFailed:       true,
		NetworkCallResult:       0, // Connection refused, no HTTP status
	})
	assertClassification(t, result, "APP-001", classifier.FamilyApplication, classifier.SevP1)
	if result.Confidence != 0.75 {
		t.Errorf("Generic network fail should have 0.75 confidence, got %.2f", result.Confidence)
	}
}

func TestApp_OOMKill(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{ExitCode: 137})
	assertClassification(t, result, "APP-002", classifier.FamilyApplication, classifier.SevP1)
}

func TestApp_ConfigMissing_NoNetwork(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ExitCode:                1,
		StartupDurationMs:       300,
		HasNetworkCallInStartup: false,
	})
	assertClassification(t, result, "APP-003", classifier.FamilyApplication, classifier.SevP1)
}

func TestApp_ReadinessProbe(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ExitCode:              0,
		ReadinessProbeFailure: true,
	})
	assertClassification(t, result, "APP-004", classifier.FamilyApplication, classifier.SevP2)
}

func TestApp_ExitZero_NoReadiness_NilResult(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ExitCode:              0,
		ReadinessProbeFailure: false,
	})
	// Edge case §6.4: Exit 0 without readiness failure should NOT classify
	if result != nil {
		t.Errorf("Exit 0 without readiness failure should be nil, got %s", result.Type)
	}
}

func TestApp_PortConflict(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ExitCode:    1,
		HasBindFail: true,
	})
	assertClassification(t, result, "APP-005", classifier.FamilyApplication, classifier.SevP2)
}

func TestApp_DependencyUnavailable(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ExitCode:      1,
		HasTCPRefused: true,
	})
	assertClassification(t, result, "APP-006", classifier.FamilyApplication, classifier.SevP1)
}

func TestApp_ImagePull(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{ImagePullBackOff: true})
	assertClassification(t, result, "APP-007", classifier.FamilyApplication, classifier.SevP2)
}

func TestApp_InitContainer(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{InitContainerFailed: true})
	assertClassification(t, result, "APP-008", classifier.FamilyApplication, classifier.SevP2)
}

// ──────────────────────────────────────────────────────────────────
// Control Plane Tests (CTL-001 → CTL-005)
// ──────────────────────────────────────────────────────────────────

func TestControlPlane_EtcdWarning(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{EtcdWALFsyncP99Ms: 50})
	assertClassification(t, result, "CTL-001", classifier.FamilyControlPlane, classifier.SevP1)
}

func TestControlPlane_EtcdCritical_SeverityUpgrade(t *testing.T) {
	// Edge case §6.6: >100ms wal_fsync should auto-upgrade to P0
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{EtcdWALFsyncP99Ms: 150})
	assertClassification(t, result, "CTL-001", classifier.FamilyControlPlane, classifier.SevP0)
}

func TestControlPlane_WebhookTimeout(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{WebhookTimeoutSec: 15})
	assertClassification(t, result, "CTL-003", classifier.FamilyControlPlane, classifier.SevP0)
}

func TestControlPlane_APIServerOverload(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{APIServerP99Ms: 2000})
	assertClassification(t, result, "CTL-002", classifier.FamilyControlPlane, classifier.SevP1)
}

func TestControlPlane_SchedulerPending(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		PendingPodCount:    80,
		PendingPodDuration: 3 * time.Minute,
	})
	assertClassification(t, result, "CTL-005", classifier.FamilyControlPlane, classifier.SevP1)
}

func TestControlPlane_ControllerBacklog(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{ControllerQueueDepth: 1500})
	assertClassification(t, result, "CTL-004", classifier.FamilyControlPlane, classifier.SevP2)
}

// ──────────────────────────────────────────────────────────────────
// Priority Routing Tests
// ──────────────────────────────────────────────────────────────────

func TestPriority_CTLBeatsMemory(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		EtcdWALFsyncP99Ms: 150,       // CTL-001 P0
		OOMKillDetected:   true,       // MEM-001
		OOMKillSource:     "kernel",
	})
	assertClassification(t, result, "CTL-001", classifier.FamilyControlPlane, classifier.SevP0)
}

func TestPriority_MemoryBeatsApp(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		OOMKillDetected: true,
		OOMKillSource:   "cgroup",
		ExitCode:        137, // Also APP-002
	})
	assertClassification(t, result, "MEM-002", classifier.FamilyMemory, classifier.SevP1)
}

func TestPriority_AppBeatsNetwork(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ExitCode:         137,         // APP-002
		ConntrackPercent: 95,          // NET-007
	})
	assertClassification(t, result, "APP-002", classifier.FamilyApplication, classifier.SevP1)
}

func TestPriority_NetworkBeatsCPU(t *testing.T) {
	c := classifier.NewClassifier(slog.Default())
	result := c.Classify(classifier.Signals{
		ConntrackPercent: 95,          // NET-007
		RunqueueWaitMs:   75,          // CPU-004
	})
	assertClassification(t, result, "NET-007", classifier.FamilyNetwork, classifier.SevP1)
}

// ──────────────────────────────────────────────────────────────────
// Registry Tests
// ──────────────────────────────────────────────────────────────────

func TestRegistry_All32TypesRegistered(t *testing.T) {
	expectedCount := 32
	if len(classifier.Registry) != expectedCount {
		t.Errorf("Expected %d registered incident types, got %d", expectedCount, len(classifier.Registry))
	}
}

func TestRegistry_AutoRemediable(t *testing.T) {
	autoRemediable := map[classifier.IncidentType]bool{
		classifier.MEM002_OOMKillCgroup:    true,
		classifier.MEM004_MemoryGrowthSpike: true,
		classifier.MEM005_NodeMemPressure:   true,
		classifier.CPU001_CFSThrottle:       true,
		classifier.CPU002_NoisyNeighbor:     true,
		classifier.CPU003_NodeCPUSaturation: true,
		classifier.CPU006_CPULimitTight:     true,
		classifier.NET002_CoreDNSOverload:   true,
		classifier.NET004_ConnectionFlood:   true,
		classifier.NET007_ConntrackExhaust:  true,
		classifier.APP002_CrashOOM:          true,
		classifier.CTL003_WebhookTimeout:    true,
	}

	for incType, info := range classifier.Registry {
		expectAuto, isAuto := autoRemediable[incType]
		if isAuto && !info.AutoRemediable {
			t.Errorf("%s should be auto-remediable but isn't", incType)
		}
		if !isAuto && info.AutoRemediable {
			t.Errorf("%s should NOT be auto-remediable but is", incType)
		}
		_ = expectAuto
	}
}

// ──────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────

func assertClassification(t *testing.T, result *classifier.ClassifiedIncident, expectedType string, expectedFamily classifier.IncidentFamily, expectedSev classifier.Severity) {
	t.Helper()
	if result == nil {
		t.Fatalf("Expected classification %s, got nil", expectedType)
	}
	if string(result.Type) != expectedType {
		t.Errorf("Type: expected %s, got %s", expectedType, result.Type)
	}
	if result.Family != expectedFamily {
		t.Errorf("Family: expected %s, got %s", expectedFamily, result.Family)
	}
	if result.Severity != expectedSev {
		t.Errorf("Severity: expected %s, got %s", expectedSev, result.Severity)
	}
}
