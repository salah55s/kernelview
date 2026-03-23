package escalation

import (
	"fmt"
	"strings"
	"time"
)

// PostmortemBuilder generates automated postmortem drafts following
// Google SRE postmortem format, per spec §5.4.
type PostmortemBuilder struct{}

// NewPostmortemBuilder creates a new builder.
func NewPostmortemBuilder() *PostmortemBuilder {
	return &PostmortemBuilder{}
}

// PostmortemDraft is the generated postmortem document.
type PostmortemDraft struct {
	IncidentID   string
	Title        string
	Severity     string
	Duration     time.Duration
	StartTime    time.Time
	EndTime      time.Time
	Services     []string
	Timeline     []string
	RootCause    string
	AIExplanation string
	ActionsTaken []string
	ActionItems  []ActionItem
	PromptForLLM string
}

// ActionItem is a specific, assignable post-incident task.
type ActionItem struct {
	Title       string
	Description string
	Priority    string // P0, P1, P2
	OwnerTeam   string
}

// BuildPrompt generates the LLM prompt for postmortem generation.
func (b *PostmortemBuilder) BuildPrompt(incident *ManagedIncident) string {
	var timeline strings.Builder
	for _, event := range incident.TimelineEvents {
		timeline.WriteString(fmt.Sprintf("[%s] %s (source: %s)\n",
			event.Time.Format("15:04"), event.Description, event.Type))
	}

	var endTime string
	duration := "ongoing"
	if incident.ResolvedAt != nil {
		endTime = incident.ResolvedAt.Format(time.RFC3339)
		duration = incident.ResolvedAt.Sub(incident.DetectedAt).String()
	}

	return fmt.Sprintf(`POSTMORTEM GENERATION PROMPT

You are generating a blameless postmortem for a Kubernetes incident.
Follow Google SRE postmortem format. Be specific. Avoid vague statements.

INCIDENT SUMMARY:
  ID: %s
  Duration: %s to %s (%s)
  Severity: %s
  Services affected: %s
  Incident type: %s (%s)

TIMELINE OF EVENTS:
%s
ACTIONS TAKEN:
  Automated: %s

AI ROOT CAUSE ANALYSIS:
%s

Generate a postmortem with these sections:
1. Summary (3 sentences max)
2. Timeline (bullet points, chronological)
3. Root Cause (distinguish root cause from trigger)
4. Contributing Factors (systemic issues that enabled this incident)
5. Impact (estimated users affected, SLO breach duration)
6. Detection (how was it found, could it be found sooner?)
7. Action Items (3-5 specific, assignable tasks with priorities)

Action items must be specific: not 'improve monitoring' but
'Add eBPF memory growth velocity alert for the payment-service pod'.

Output as JSON: {summary, timeline, root_cause, contributing_factors,
impact, detection_gap, action_items: [{title, description, priority, owner_team}]}`,
		incident.ID,
		incident.DetectedAt.Format(time.RFC3339), endTime, duration,
		string(incident.Severity),
		incident.ServiceName+" ("+incident.Namespace+")",
		incident.IncidentType, incident.IncidentCode,
		timeline.String(),
		strings.Join(incident.ActionsTaken, ", "),
		incident.AIExplanation,
	)
}
