package escalation

import (
	"sync"
	"time"
)

// Suppressor implements the 6 alert suppression rules from spec §5.3.3.
// False alerts destroy trust faster than missed alerts.
type Suppressor struct {
	mu sync.RWMutex

	// Deployment window tracking
	activeRollouts map[string]time.Time // service → rollout complete time

	// Maintenance windows
	maintenanceUntil map[string]time.Time // service → maintenance end

	// Cascade detection
	recentAlerts []alertRecord

	// Flapping detection
	flapHistory map[string][]time.Time // service+type → fire times

	// Learning period
	serviceFirstSeen map[string]time.Time // service → first deployment time
}

type alertRecord struct {
	service   string
	timestamp time.Time
}

// NewSuppressor creates a new alert suppressor.
func NewSuppressor() *Suppressor {
	return &Suppressor{
		activeRollouts:   make(map[string]time.Time),
		maintenanceUntil: make(map[string]time.Time),
		flapHistory:      make(map[string][]time.Time),
		serviceFirstSeen: make(map[string]time.Time),
	}
}

// ShouldSuppress returns true if the incident should be suppressed, with reason.
func (s *Suppressor) ShouldSuppress(incident *ManagedIncident) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Rule 1: DEPLOYMENT WINDOW
	// Suppress APP family alerts for 5 minutes after deployment rollout completes
	if incident.IncidentCode[:3] == "APP" {
		if completeTime, ok := s.activeRollouts[incident.ServiceName]; ok {
			if time.Since(completeTime) < 5*time.Minute {
				return true, "DEPLOYMENT_WINDOW: rollout completed " + time.Since(completeTime).String() + " ago"
			}
		}
	}

	// Rule 2: KNOWN MAINTENANCE
	// Suppress P2/P3 alerts if service has maintenance window. P0/P1 still fire.
	if incident.Severity == SevP2 || incident.Severity == SevP3 {
		if until, ok := s.maintenanceUntil[incident.ServiceName]; ok {
			if time.Now().Before(until) {
				return true, "MAINTENANCE_WINDOW: suppressed until " + until.Format(time.RFC3339)
			}
		}
	}

	// Rule 3: CASCADE DETECTION
	// If >3 services alert within 60 seconds, suppress individuals → fire CASCADE
	recentCount := 0
	uniqueServices := make(map[string]bool)
	cutoff := time.Now().Add(-60 * time.Second)
	for _, alert := range s.recentAlerts {
		if alert.timestamp.After(cutoff) {
			recentCount++
			uniqueServices[alert.service] = true
		}
	}
	if len(uniqueServices) > 3 {
		return true, "CASCADE_DETECTED: " + string(rune(len(uniqueServices))) + " services alerting simultaneously"
	}

	// Rule 4: FLAPPING DETECTION
	// Same incident fires and resolves >3 times in 1 hour → suppress
	key := incident.ServiceName + ":" + incident.IncidentCode
	if fireTimes, ok := s.flapHistory[key]; ok {
		hourAgo := time.Now().Add(-1 * time.Hour)
		recentFires := 0
		for _, t := range fireTimes {
			if t.After(hourAgo) {
				recentFires++
			}
		}
		if recentFires > 3 {
			return true, "FLAPPING: incident fired " + string(rune(recentFires)) + " times in last hour"
		}
	}

	// Rule 5: BURST DETECTION
	// Error rate spike that recovers within 2 minutes → classify as P4 TRANSIENT only
	// (Handled by the anomaly detector, not here — burst is pre-filtered)

	// Rule 6: LEARNING PERIOD
	// New services (<1 hour) suppress P2/P3 alerts for 30 minutes
	if incident.Severity == SevP2 || incident.Severity == SevP3 {
		if firstSeen, ok := s.serviceFirstSeen[incident.ServiceName]; ok {
			if time.Since(firstSeen) < 30*time.Minute {
				return true, "LEARNING_PERIOD: service deployed " + time.Since(firstSeen).String() + " ago"
			}
		}
	}

	return false, ""
}

// RecordRollout records that a deployment rollout completed for a service.
func (s *Suppressor) RecordRollout(service string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeRollouts[service] = time.Now()
}

// SetMaintenance sets a maintenance window for a service.
func (s *Suppressor) SetMaintenance(service string, until time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maintenanceUntil[service] = until
}

// RecordAlert records an alert for cascade and flapping detection.
func (s *Suppressor) RecordAlert(incident *ManagedIncident) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// For cascade detection
	s.recentAlerts = append(s.recentAlerts, alertRecord{
		service:   incident.ServiceName,
		timestamp: now,
	})

	// Trim old records
	cutoff := now.Add(-2 * time.Minute)
	var trimmed []alertRecord
	for _, a := range s.recentAlerts {
		if a.timestamp.After(cutoff) {
			trimmed = append(trimmed, a)
		}
	}
	s.recentAlerts = trimmed

	// For flapping detection
	key := incident.ServiceName + ":" + incident.IncidentCode
	s.flapHistory[key] = append(s.flapHistory[key], now)

	// Trim old flap records
	hourAgo := now.Add(-1 * time.Hour)
	var trimmedFlaps []time.Time
	for _, t := range s.flapHistory[key] {
		if t.After(hourAgo) {
			trimmedFlaps = append(trimmedFlaps, t)
		}
	}
	s.flapHistory[key] = trimmedFlaps
}

// RegisterService registers a new service for learning period tracking.
func (s *Suppressor) RegisterService(service string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.serviceFirstSeen[service]; !ok {
		s.serviceFirstSeen[service] = time.Now()
	}
}
