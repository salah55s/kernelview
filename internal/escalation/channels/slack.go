package channels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kernelview/kernelview/internal/escalation"
)

// SlackChannel sends rich block messages to Slack per spec §5.3.2.
type SlackChannel struct {
	webhookURL string
	channelID  string // #incidents channel for P0/P1
	client     *http.Client
}

// NewSlackChannel creates a Slack channel integration.
func NewSlackChannel(webhookURL, channelID string) *SlackChannel {
	return &SlackChannel{
		webhookURL: webhookURL,
		channelID:  channelID,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackChannel) Name() string        { return "slack" }
func (s *SlackChannel) SupportsAck() bool    { return true }

func (s *SlackChannel) Send(incident *escalation.ManagedIncident) error {
	payload := s.buildBlocks(incident)
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling slack payload: %w", err)
	}

	resp, err := s.client.Post(s.webhookURL, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("sending slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("slack API error: status %d", resp.StatusCode)
	}
	return nil
}

func (s *SlackChannel) buildBlocks(incident *escalation.ManagedIncident) map[string]interface{} {
	severityEmoji := map[escalation.Severity]string{
		escalation.SevP0: "🔴", escalation.SevP1: "🟠",
		escalation.SevP2: "🟡", escalation.SevP3: "🔵", escalation.SevP4: "⚪",
	}[incident.Severity]

	return map[string]interface{}{
		"channel": s.channelID,
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]string{
					"type": "plain_text",
					"text": fmt.Sprintf("%s %s Incident: %s", severityEmoji, incident.Severity, incident.IncidentCode),
				},
			},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": fmt.Sprintf("*Service:*\n%s", incident.ServiceName)},
					{"type": "mrkdwn", "text": fmt.Sprintf("*Namespace:*\n%s", incident.Namespace)},
					{"type": "mrkdwn", "text": fmt.Sprintf("*Type:*\n%s", incident.IncidentType)},
					{"type": "mrkdwn", "text": fmt.Sprintf("*Detected:*\n%s", incident.DetectedAt.Format("15:04:05"))},
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*AI Analysis:*\n%s", truncate(incident.AIExplanation, 500)),
				},
			},
			{
				"type": "actions",
				"elements": []map[string]interface{}{
					{
						"type":      "button",
						"text":      map[string]string{"type": "plain_text", "text": "✅ Acknowledge"},
						"style":     "primary",
						"action_id": "ack_incident",
						"value":     incident.ID,
					},
					{
						"type":      "button",
						"text":      map[string]string{"type": "plain_text", "text": "🔍 View Details"},
						"action_id": "view_incident",
						"value":     incident.ID,
					},
				},
			},
		},
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
