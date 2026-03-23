package escalation_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/kernelview/kernelview/internal/escalation"
)

// ──────────────────────────────────────────────────────────────────
// State Machine Tests
// ──────────────────────────────────────────────────────────────────

func TestEngine_CreateIncident(t *testing.T) {
	e := escalation.NewEngine(slog.Default())
	inc := e.CreateIncident("inc-001", "OOM Kill", "MEM-001", escalation.SevP1, "payment-svc", "production")

	if inc.State != escalation.StateDetected {
		t.Errorf("New incident should be in DETECTED state, got %s", inc.State)
	}
	if inc.ID != "inc-001" {
		t.Errorf("Expected ID inc-001, got %s", inc.ID)
	}
	if len(inc.TimelineEvents) != 1 {
		t.Errorf("Expected 1 timeline event, got %d", len(inc.TimelineEvents))
	}
}

func TestEngine_ValidTransitions(t *testing.T) {
	e := escalation.NewEngine(slog.Default())
	e.CreateIncident("inc-002", "CFS Throttle", "CPU-001", escalation.SevP2, "api-gateway", "prod")

	transitions := []struct {
		state  escalation.State
		detail string
	}{
		{escalation.StateClassifying, "Decision tree running"},
		{escalation.StateCorrelating, "LLM analyzing with Gemini Flash"},
		{escalation.StateReadyToEscalate, "AI analysis complete, confidence 0.85"},
		{escalation.StateAutoRemediating, "Removing CPU limit via operator"},
		{escalation.StateMonitoringEffect, "Watching p99 latency for 15 min"},
		{escalation.StateResolved, "p99 returned to normal after limit removal"},
		{escalation.StatePostIncident, "Postmortem generated"},
	}

	for _, tr := range transitions {
		if err := e.Transition("inc-002", tr.state, tr.detail); err != nil {
			t.Fatalf("Transition to %s failed: %v", tr.state, err)
		}
	}

	inc, ok := e.GetIncident("inc-002")
	if !ok {
		t.Fatal("Incident not found")
	}
	if inc.State != escalation.StatePostIncident {
		t.Errorf("Expected POST_INCIDENT, got %s", inc.State)
	}
	// Initial event + 7 transitions = 8 events
	if len(inc.TimelineEvents) != 8 {
		t.Errorf("Expected 8 timeline events, got %d", len(inc.TimelineEvents))
	}
}

func TestEngine_InvalidTransition(t *testing.T) {
	e := escalation.NewEngine(slog.Default())
	e.CreateIncident("inc-003", "DNS ndots", "NET-001", escalation.SevP2, "checkout", "prod")

	// Cannot jump from DETECTED directly to RESOLVED without going through CLASSIFYING
	err := e.Transition("inc-003", escalation.StateCorrelating, "skip classifying")
	if err == nil {
		t.Error("Expected error for invalid transition DETECTED → CORRELATING")
	}
}

func TestEngine_ActiveIncidents(t *testing.T) {
	e := escalation.NewEngine(slog.Default())
	e.CreateIncident("a1", "t1", "MEM-001", escalation.SevP1, "s1", "ns1")
	e.CreateIncident("a2", "t2", "CPU-001", escalation.SevP2, "s2", "ns2")
	e.CreateIncident("a3", "t3", "NET-001", escalation.SevP3, "s3", "ns3")

	// Resolve one
	e.Transition("a2", escalation.StateClassifying, "")
	e.Transition("a2", escalation.StateCorrelating, "")
	e.Transition("a2", escalation.StateReadyToEscalate, "")
	e.Transition("a2", escalation.StateAutoRemediating, "")
	e.Transition("a2", escalation.StateMonitoringEffect, "")
	e.Transition("a2", escalation.StateResolved, "fixed")

	active := e.ActiveIncidents()
	if len(active) != 2 {
		t.Errorf("Expected 2 active incidents, got %d", len(active))
	}
}

// ──────────────────────────────────────────────────────────────────
// Suppressor Tests (6 Rules from §5.3.3)
// ──────────────────────────────────────────────────────────────────

func TestSuppressor_DeploymentWindow(t *testing.T) {
	s := escalation.NewSuppressor()
	s.RecordRollout("payment-svc")

	inc := &escalation.ManagedIncident{
		IncidentCode: "APP-001",
		ServiceName:  "payment-svc",
		Severity:     escalation.SevP2,
	}

	suppressed, reason := s.ShouldSuppress(inc)
	if !suppressed {
		t.Error("APP incident should be suppressed within deployment window")
	}
	if reason == "" {
		t.Error("Expected a suppression reason")
	}
}

func TestSuppressor_MaintenanceWindow(t *testing.T) {
	s := escalation.NewSuppressor()
	s.SetMaintenance("batch-svc", time.Now().Add(1*time.Hour))

	// P3 should be suppressed during maintenance
	p3 := &escalation.ManagedIncident{
		IncidentCode: "CPU-006",
		ServiceName:  "batch-svc",
		Severity:     escalation.SevP3,
	}
	suppressed, _ := s.ShouldSuppress(p3)
	if !suppressed {
		t.Error("P3 should be suppressed during maintenance")
	}

	// P1 should NOT be suppressed during maintenance
	p1 := &escalation.ManagedIncident{
		IncidentCode: "MEM-001",
		ServiceName:  "batch-svc",
		Severity:     escalation.SevP1,
	}
	suppressed, _ = s.ShouldSuppress(p1)
	if suppressed {
		t.Error("P1 should NOT be suppressed during maintenance")
	}
}

func TestSuppressor_LearningPeriod(t *testing.T) {
	s := escalation.NewSuppressor()
	s.RegisterService("new-service")

	p3 := &escalation.ManagedIncident{
		IncidentCode: "CPU-006",
		ServiceName:  "new-service",
		Severity:     escalation.SevP3,
	}
	suppressed, _ := s.ShouldSuppress(p3)
	if !suppressed {
		t.Error("P3 for new service should be suppressed during learning period")
	}
}

func TestSuppressor_FlappingDetection(t *testing.T) {
	s := escalation.NewSuppressor()

	inc := &escalation.ManagedIncident{
		IncidentCode: "NET-001",
		ServiceName:  "api-gateway",
		Severity:     escalation.SevP2,
	}

	// Fire 4 times — should trigger flapping on the 5th check
	for i := 0; i < 4; i++ {
		s.RecordAlert(inc)
	}

	suppressed, _ := s.ShouldSuppress(inc)
	if !suppressed {
		t.Error("Incident should be suppressed after 4 fires in 1 hour (flapping)")
	}
}

// ──────────────────────────────────────────────────────────────────
// Ownership Router Tests
// ──────────────────────────────────────────────────────────────────

func TestRouter_ServiceAnnotation(t *testing.T) {
	r := escalation.NewOwnershipRouter("platform-team@company.com")
	r.SetK8sLookups(
		func(service, namespace, key string) string {
			if service == "payment-svc" && key == "kernelview.io/owner" {
				return "payments-oncall@company.com"
			}
			return ""
		},
		func(namespace, key string) string { return "" },
	)

	inc := &escalation.ManagedIncident{
		ServiceName: "payment-svc",
		Namespace:   "production",
		Severity:    escalation.SevP1,
	}

	owner, source := r.ResolveOwner(inc)
	if owner != "payments-oncall@company.com" {
		t.Errorf("Expected service annotation owner, got %s", owner)
	}
	if source != "service_annotation" {
		t.Errorf("Expected source 'service_annotation', got %s", source)
	}
}

func TestRouter_FallbackToPlatform(t *testing.T) {
	r := escalation.NewOwnershipRouter("platform-team@company.com")

	inc := &escalation.ManagedIncident{
		ServiceName: "unknown-svc",
		Namespace:   "default",
		Severity:    escalation.SevP2,
	}

	owner, source := r.ResolveOwner(inc)
	if owner != "platform-team@company.com" {
		t.Errorf("Expected platform fallback, got %s", owner)
	}
	if source != "platform_oncall_fallback" {
		t.Errorf("Expected source 'platform_oncall_fallback', got %s", source)
	}
}

func TestRouter_P0AlwaysPlatform(t *testing.T) {
	r := escalation.NewOwnershipRouter("platform-team@company.com")

	inc := &escalation.ManagedIncident{
		ServiceName: "any-service",
		Namespace:   "prod",
		Severity:    escalation.SevP0,
	}

	owner, source := r.ResolveOwner(inc)
	if owner != "platform-team@company.com" {
		t.Errorf("P0 should always route to platform team, got %s", owner)
	}
	if source != "platform_oncall_p0" {
		t.Errorf("Expected source 'platform_oncall_p0', got %s", source)
	}
}
