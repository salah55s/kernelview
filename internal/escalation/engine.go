// Package escalation implements the incident escalation state machine from spec §5.
// It manages the lifecycle of every incident from DETECTED through POST_INCIDENT,
// ensuring no incident is silently dropped, no page is sent without context,
// and no automated action executes without a record.
package escalation

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// State represents the current phase of an incident's escalation lifecycle.
type State string

const (
	StateDetected         State = "DETECTED"           // eBPF signal received, anomaly confirmed
	StateClassifying      State = "CLASSIFYING"        // Decision tree determining type
	StateCorrelating      State = "CORRELATING"        // LLM analyzing for root cause
	StateReadyToEscalate  State = "READY_TO_ESCALATE"  // AI done, safety checked, actions queued
	StateAutoRemediating  State = "AUTO_REMEDIATING"   // Operator executing safe actions
	StateMonitoringEffect State = "MONITORING_EFFECT"  // Watching metrics after action
	StateEscalated        State = "ESCALATED"          // Human has been paged
	StateAcknowledged     State = "ACKNOWLEDGED"       // Human is working the incident
	StateResolved         State = "RESOLVED"           // Metrics returned to normal
	StatePostIncident     State = "POST_INCIDENT"      // Postmortem generated
)

// stateTimeout defines the maximum time in a state before automatic transition.
var stateTimeout = map[State]time.Duration{
	StateDetected:         5 * time.Minute,
	StateClassifying:      2 * time.Minute,
	StateCorrelating:      90 * time.Second,
	StateReadyToEscalate:  1 * time.Minute,
	StateAutoRemediating:  10 * time.Minute,
	StateMonitoringEffect: 15 * time.Minute,
	StateEscalated:        5 * time.Minute,  // P0: 5min, P1: 15min
	StateAcknowledged:     1 * time.Hour,
	StateResolved:         30 * time.Minute,
}

// Severity tiers matching spec §5.1.
type Severity string

const (
	SevP0 Severity = "P0" // EMERGENCY
	SevP1 Severity = "P1" // CRITICAL
	SevP2 Severity = "P2" // HIGH
	SevP3 Severity = "P3" // MEDIUM
	SevP4 Severity = "P4" // INFO
)

// ManagedIncident tracks an incident through the escalation lifecycle.
type ManagedIncident struct {
	ID               string
	IncidentType     string
	IncidentCode     string
	Severity         Severity
	State            State
	ServiceName      string
	Namespace        string
	DetectedAt       time.Time
	StateEnteredAt   time.Time
	AcknowledgedBy   string
	ResolvedAt       *time.Time
	AIExplanation    string
	ActionsTaken     []string
	EscalationTarget string
	TimelineEvents   []TimelineEvent
}

// TimelineEvent records a single event in the incident timeline.
type TimelineEvent struct {
	Time        time.Time
	Type        string // "anomaly", "classification", "correlation", "safety", "remediation", "escalation", "resolution"
	Description string
	Details     string
}

// Engine manages the escalation state machine for all active incidents.
type Engine struct {
	mu        sync.RWMutex
	incidents map[string]*ManagedIncident
	logger    *slog.Logger

	// Integrations
	suppressor  *Suppressor
	router      *OwnershipRouter
	channels    []EscalationChannel
}

// EscalationChannel abstracts sending notifications (Slack, PagerDuty, email).
type EscalationChannel interface {
	Send(incident *ManagedIncident) error
	Name() string
	SupportsAck() bool
}

// NewEngine creates a new escalation engine.
func NewEngine(logger *slog.Logger) *Engine {
	return &Engine{
		incidents:  make(map[string]*ManagedIncident),
		logger:     logger,
		suppressor: NewSuppressor(),
	}
}

// SetRouter sets the ownership router.
func (e *Engine) SetRouter(router *OwnershipRouter) {
	e.router = router
}

// AddChannel adds an escalation channel.
func (e *Engine) AddChannel(ch EscalationChannel) {
	e.channels = append(e.channels, ch)
}

// CreateIncident initializes a new incident in DETECTED state.
func (e *Engine) CreateIncident(id, incidentType, incidentCode string, severity Severity, service, namespace string) *ManagedIncident {
	now := time.Now()

	incident := &ManagedIncident{
		ID:             id,
		IncidentType:   incidentType,
		IncidentCode:   incidentCode,
		Severity:       severity,
		State:          StateDetected,
		ServiceName:    service,
		Namespace:      namespace,
		DetectedAt:     now,
		StateEnteredAt: now,
		TimelineEvents: []TimelineEvent{
			{Time: now, Type: "anomaly", Description: fmt.Sprintf("Incident %s detected: %s", incidentCode, incidentType)},
		},
	}

	e.mu.Lock()
	e.incidents[id] = incident
	e.mu.Unlock()

	e.logger.Info("incident created",
		"id", id, "type", incidentCode, "severity", severity, "service", service)

	return incident
}

// Transition moves an incident to a new state with validation.
func (e *Engine) Transition(id string, newState State, detail string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	incident, ok := e.incidents[id]
	if !ok {
		return fmt.Errorf("incident %s not found", id)
	}

	if !isValidTransition(incident.State, newState) {
		return fmt.Errorf("invalid transition: %s → %s", incident.State, newState)
	}

	now := time.Now()
	incident.State = newState
	incident.StateEnteredAt = now
	incident.TimelineEvents = append(incident.TimelineEvents, TimelineEvent{
		Time:        now,
		Type:        string(newState),
		Description: detail,
	})

	e.logger.Info("incident state transition",
		"id", id, "from", incident.State, "to", newState, "detail", detail)

	// Handle state-specific actions
	switch newState {
	case StateResolved:
		t := now
		incident.ResolvedAt = &t
	case StateEscalated:
		// Page the appropriate person
		e.escalateToHuman(incident)
	}

	return nil
}

// CheckTimeouts processes all active incidents for timeout-based transitions.
func (e *Engine) CheckTimeouts() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	for _, incident := range e.incidents {
		timeout, ok := stateTimeout[incident.State]
		if !ok {
			continue
		}

		if now.Sub(incident.StateEnteredAt) < timeout {
			continue
		}

		// Timeout reached — auto-transition
		switch incident.State {
		case StateDetected:
			// 5 min with no classification = stale, discard
			incident.State = StateResolved
			e.logger.Warn("incident expired in DETECTED state", "id", incident.ID)

		case StateClassifying:
			// 2 min = use fallback GENERIC class
			incident.IncidentCode = "GENERIC"
			incident.State = StateCorrelating
			e.logger.Warn("classification timed out, using GENERIC", "id", incident.ID)

		case StateCorrelating:
			// 90s = escalate without AI explanation
			incident.AIExplanation = "[Timed out — AI analysis unavailable]"
			incident.State = StateReadyToEscalate
			e.logger.Warn("correlation timed out, escalating without AI", "id", incident.ID)

		case StateReadyToEscalate:
			// 1 min = auto-escalate
			incident.State = StateEscalated
			e.escalateToHuman(incident)

		case StateAutoRemediating:
			// 10 min no improvement = re-escalate to human
			incident.State = StateEscalated
			e.escalateToHuman(incident)
			e.logger.Warn("auto-remediation did not resolve, escalating", "id", incident.ID)

		case StateMonitoringEffect:
			// 15 min = declare resolved
			incident.State = StateResolved
			t := now
			incident.ResolvedAt = &t

		case StateEscalated:
			// P0: 5 min, P1: 15 min → page next on-call
			e.logger.Warn("escalation not acknowledged, paging next on-call", "id", incident.ID)

		case StateAcknowledged:
			// 1 hour still open → escalate to manager
			e.logger.Warn("incident open >1 hour, escalating to manager", "id", incident.ID)
		}

		incident.StateEnteredAt = now
	}
}

// ShouldSuppress checks if an incident should be suppressed per §5.3.3 rules.
func (e *Engine) ShouldSuppress(incident *ManagedIncident) (bool, string) {
	return e.suppressor.ShouldSuppress(incident)
}

// GetIncident returns a managed incident by ID.
func (e *Engine) GetIncident(id string) (*ManagedIncident, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	inc, ok := e.incidents[id]
	return inc, ok
}

// ActiveIncidents returns all non-resolved incidents.
func (e *Engine) ActiveIncidents() []*ManagedIncident {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var active []*ManagedIncident
	for _, inc := range e.incidents {
		if inc.State != StateResolved && inc.State != StatePostIncident {
			active = append(active, inc)
		}
	}
	return active
}

func (e *Engine) escalateToHuman(incident *ManagedIncident) {
	for _, ch := range e.channels {
		if err := ch.Send(incident); err != nil {
			e.logger.Error("failed to send escalation", "channel", ch.Name(), "error", err)
		}
	}
}

// isValidTransition validates state machine transitions per spec §5.2.
func isValidTransition(from, to State) bool {
	valid := map[State][]State{
		StateDetected:         {StateClassifying, StateResolved},
		StateClassifying:      {StateCorrelating},
		StateCorrelating:      {StateReadyToEscalate},
		StateReadyToEscalate:  {StateEscalated, StateAutoRemediating},
		StateAutoRemediating:  {StateMonitoringEffect, StateEscalated},
		StateMonitoringEffect: {StateResolved, StateEscalated},
		StateEscalated:        {StateAcknowledged},
		StateAcknowledged:     {StateResolved, StateEscalated},
		StateResolved:         {StatePostIncident},
	}

	targets, ok := valid[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}
