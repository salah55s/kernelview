package channels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kernelview/kernelview/internal/escalation"
)

// PagerDutyChannel sends incidents via PagerDuty Events API v2.
// Used for P0/P1 after Slack non-acknowledgement per spec §5.3.2.
type PagerDutyChannel struct {
	routingKey string
	client     *http.Client
}

// NewPagerDutyChannel creates a PagerDuty integration.
func NewPagerDutyChannel(routingKey string) *PagerDutyChannel {
	return &PagerDutyChannel{
		routingKey: routingKey,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *PagerDutyChannel) Name() string     { return "pagerduty" }
func (p *PagerDutyChannel) SupportsAck() bool { return true }

func (p *PagerDutyChannel) Send(incident *escalation.ManagedIncident) error {
	pdSeverity := "warning"
	switch incident.Severity {
	case escalation.SevP0:
		pdSeverity = "critical"
	case escalation.SevP1:
		pdSeverity = "error"
	case escalation.SevP2:
		pdSeverity = "warning"
	default:
		pdSeverity = "info"
	}

	payload := map[string]interface{}{
		"routing_key":  p.routingKey,
		"event_action": "trigger",
		"dedup_key":    incident.ID,
		"payload": map[string]interface{}{
			"summary":  fmt.Sprintf("[%s] %s: %s in %s/%s", incident.Severity, incident.IncidentCode, incident.IncidentType, incident.Namespace, incident.ServiceName),
			"severity": pdSeverity,
			"source":   "kernelview",
			"group":    incident.Namespace,
			"class":    incident.IncidentCode,
			"custom_details": map[string]interface{}{
				"incident_id":    incident.ID,
				"service":        incident.ServiceName,
				"namespace":      incident.Namespace,
				"ai_explanation": incident.AIExplanation,
				"detected_at":    incident.DetectedAt.Format(time.RFC3339),
				"actions_taken":  incident.ActionsTaken,
			},
		},
		"links": []map[string]string{
			{"href": fmt.Sprintf("http://localhost:8080/incidents/%s", incident.ID), "text": "KernelView Dashboard"},
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling pagerduty payload: %w", err)
	}

	resp, err := p.client.Post(
		"https://events.pagerduty.com/v2/enqueue",
		"application/json",
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return fmt.Errorf("sending pagerduty event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		return fmt.Errorf("pagerduty API error: status %d", resp.StatusCode)
	}
	return nil
}
