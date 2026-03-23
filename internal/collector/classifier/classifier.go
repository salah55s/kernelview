package classifier

import (
	"log/slog"
	"time"
)

// Signals contains all observability signals available for classification.
type Signals struct {
	// Memory signals
	OOMKillDetected     bool
	OOMKillSource       string  // "kernel" or "cgroup"
	RSSGrowthMBPerMin   float64
	RSSGrowthDuration   time.Duration
	NodeMemoryPercent   float64
	NodeMemDuration     time.Duration
	IsJVMProcess        bool
	JVMHeapPercent      float64

	// CPU signals
	ThrottleRatio       float64 // throttled_periods / total_periods
	LatencyP50Ms        float64
	LatencyP99Ms        float64
	AvgCPUPercent       float64
	SyscallRate         float64
	SyscallRateMean     float64
	SyscallRateStdDev   float64
	SyscallDuration     time.Duration
	NodeCPUPercent      float64
	NodeCPUDuration     time.Duration
	RunqueueWaitMs      float64

	// Network signals
	NXDOMAINRate        float64
	CoreDNSCPUPercent   float64
	DNSLatencyMs        float64
	RetransmitRate      float64
	LargePacketRetransmitRate float64
	NewTCPConnsPerSec   float64
	TCPRetransmitRate   float64
	EndpointChangesPerMin float64
	ConntrackPercent    float64

	// Application signals
	ExitCode            int
	StartupDurationMs   int
	HasNetworkCallInStartup bool
	NetworkCallFailed   bool
	NetworkCallResult   int    // HTTP status or 0
	HasConfigReadFail   bool
	HasBindFail         bool
	HasTCPRefused       bool
	RestartCount        int
	ImagePullBackOff    bool
	InitContainerFailed bool
	ReadinessProbeFailure bool

	// Control plane signals
	EtcdWALFsyncP99Ms  float64
	APIServerP99Ms     float64
	WebhookTimeoutSec  float64
	ControllerQueueDepth int
	PendingPodCount    int
	PendingPodDuration time.Duration

	// Context
	ServiceName        string
	Namespace          string
	PodName            string
	NodeName           string
}

// Classifier dispatches signals to family-specific classifiers.
type Classifier struct {
	logger *slog.Logger
}

// NewClassifier creates a new incident classifier.
func NewClassifier(logger *slog.Logger) *Classifier {
	return &Classifier{logger: logger}
}

// Classify analyzes signals and returns a classified incident.
// Classification happens BEFORE the LLM is invoked.
func (c *Classifier) Classify(signals Signals) *ClassifiedIncident {
	now := time.Now()

	// Run classifiers in priority order (most impactful first)
	// Control plane issues are checked first — they affect everything
	if incident := classifyControlPlane(signals, now); incident != nil {
		c.logger.Info("classified incident", "type", incident.Type, "family", incident.Family)
		return incident
	}

	// Memory: OOM kills and memory pressure
	if incident := classifyMemory(signals, now); incident != nil {
		c.logger.Info("classified incident", "type", incident.Type, "family", incident.Family)
		return incident
	}

	// Application: CrashLoopBackOff patterns
	if incident := classifyApplication(signals, now); incident != nil {
		c.logger.Info("classified incident", "type", incident.Type, "family", incident.Family)
		return incident
	}

	// Network: DNS, MTU, conntrack, connection floods
	if incident := classifyNetwork(signals, now); incident != nil {
		c.logger.Info("classified incident", "type", incident.Type, "family", incident.Family)
		return incident
	}

	// CPU: throttling, noisy neighbor, saturation
	if incident := classifyCPU(signals, now); incident != nil {
		c.logger.Info("classified incident", "type", incident.Type, "family", incident.Family)
		return incident
	}

	return nil // No incident classified
}

// buildIncident is a helper to construct a ClassifiedIncident from a type.
func buildIncident(incType IncidentType, signals Signals, now time.Time, primarySignal string, signalValue, threshold, confidence float64) *ClassifiedIncident {
	info, ok := Registry[incType]
	if !ok {
		return nil
	}

	return &ClassifiedIncident{
		Family:          info.Family,
		Type:            info.Type,
		Severity:        info.DefaultSev,
		AutoRemediable:  info.AutoRemediable,
		Description:     info.Description,
		DetectedAt:      now,
		ServiceName:     signals.ServiceName,
		Namespace:       signals.Namespace,
		PodName:         signals.PodName,
		NodeName:        signals.NodeName,
		PrimarySignal:   primarySignal,
		SignalValue:     signalValue,
		Threshold:       threshold,
		Confidence:      confidence,
		ExitCode:        signals.ExitCode,
		StartupDuration: time.Duration(signals.StartupDurationMs) * time.Millisecond,
		RestartCount:    signals.RestartCount,
	}
}
