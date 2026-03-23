package channels

import (
	"fmt"
	"strings"
	"time"

	"github.com/kernelview/kernelview/internal/escalation"
)

// EmailChannel sends weekly digest emails for P3/P4 incidents per spec §5.3.2.
type EmailChannel struct {
	smtpHost    string
	smtpPort    int
	from        string
	recipients  []string
	buffer      []*escalation.ManagedIncident // Buffered for weekly digest
}

// NewEmailChannel creates an email channel.
func NewEmailChannel(smtpHost string, smtpPort int, from string, recipients []string) *EmailChannel {
	return &EmailChannel{
		smtpHost:   smtpHost,
		smtpPort:   smtpPort,
		from:       from,
		recipients: recipients,
	}
}

func (e *EmailChannel) Name() string        { return "email" }
func (e *EmailChannel) SupportsAck() bool    { return false }

// Send buffers incidents for weekly digest (P3/P4) or sends immediately (P0-P2).
func (e *EmailChannel) Send(incident *escalation.ManagedIncident) error {
	if incident.Severity == escalation.SevP3 || incident.Severity == escalation.SevP4 {
		e.buffer = append(e.buffer, incident)
		return nil // Will be sent in weekly digest
	}

	// P0-P2: send immediately
	return e.sendImmediate(incident)
}

func (e *EmailChannel) sendImmediate(incident *escalation.ManagedIncident) error {
	subject := fmt.Sprintf("[KernelView %s] %s: %s in %s",
		incident.Severity, incident.IncidentCode, incident.IncidentType, incident.ServiceName)

	body := fmt.Sprintf(`KernelView Incident Alert

Severity: %s
Service: %s/%s
Type: %s (%s)
Detected: %s

AI Analysis:
%s

Actions Taken: %s

View in Dashboard: http://localhost:8080/incidents/%s
`,
		incident.Severity,
		incident.Namespace, incident.ServiceName,
		incident.IncidentType, incident.IncidentCode,
		incident.DetectedAt.Format(time.RFC3339),
		incident.AIExplanation,
		strings.Join(incident.ActionsTaken, ", "),
		incident.ID,
	)

	_ = subject
	_ = body

	// TODO: Implement actual SMTP sending
	// smtp.SendMail(fmt.Sprintf("%s:%d", e.smtpHost, e.smtpPort), auth, e.from, e.recipients, message)
	return nil
}

// FlushDigest sends the weekly digest email with all buffered P3/P4 incidents.
func (e *EmailChannel) FlushDigest() error {
	if len(e.buffer) == 0 {
		return nil
	}

	var body strings.Builder
	body.WriteString("KernelView Weekly Digest\n")
	body.WriteString(fmt.Sprintf("Period: %s\n\n", time.Now().Format("2006-01-02")))
	body.WriteString(fmt.Sprintf("Total incidents this week: %d\n\n", len(e.buffer)))

	for _, inc := range e.buffer {
		body.WriteString(fmt.Sprintf("• [%s] %s: %s in %s/%s (%s)\n",
			inc.Severity, inc.IncidentCode, inc.IncidentType,
			inc.Namespace, inc.ServiceName,
			inc.DetectedAt.Format("Jan 02 15:04"),
		))
	}

	e.buffer = nil // Clear buffer after sending

	// TODO: Send via SMTP
	_ = body.String()
	return nil
}
