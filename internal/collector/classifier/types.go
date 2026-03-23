// Package classifier provides incident classification for KernelView.
// Every incident is classified into one of 7 families and one of 32 types
// before the LLM is invoked. Classification determines: which LLM provider,
// which prompt template, which escalation tier, and which remediation actions.
package classifier

import "time"

// IncidentFamily is one of the 7 top-level incident families.
type IncidentFamily string

const (
	FamilyMemory       IncidentFamily = "MEM"
	FamilyCPU          IncidentFamily = "CPU"
	FamilyNetwork      IncidentFamily = "NET"
	FamilyApplication  IncidentFamily = "APP"
	FamilyControlPlane IncidentFamily = "CTL"
	FamilyStorage      IncidentFamily = "STO"
	FamilySecurity     IncidentFamily = "SEC"
)

// IncidentType is one of the 32 specific incident types.
type IncidentType string

// Family MEM — Memory Pressure
const (
	MEM001_OOMKillKernel     IncidentType = "MEM-001" // Kernel OOM killer fired
	MEM002_OOMKillCgroup     IncidentType = "MEM-002" // Cgroup memory limit hit
	MEM003_MemoryLeak        IncidentType = "MEM-003" // RSS growth >5MB/min for 10 min
	MEM004_MemoryGrowthSpike IncidentType = "MEM-004" // RSS growth >20MB/min sudden
	MEM005_NodeMemPressure   IncidentType = "MEM-005" // Node RSS >85% for 5 min
	MEM006_JVMHeapExhaust    IncidentType = "MEM-006" // JVM GC >95% heap + OOM
)

// Family CPU — CPU Starvation
const (
	CPU001_CFSThrottle       IncidentType = "CPU-001" // throttled_periods > 25%
	CPU002_NoisyNeighbor     IncidentType = "CPU-002" // syscall rate > mean + 3σ
	CPU003_NodeCPUSaturation IncidentType = "CPU-003" // Node CPU >95% for 3 min
	CPU004_SchedStarvation   IncidentType = "CPU-004" // CFS runqueue wait >50ms
	CPU005_SpinLock          IncidentType = "CPU-005" // High voluntary vs involuntary ctxsw
	CPU006_CPULimitTight     IncidentType = "CPU-006" // Consistent throttle at low avg CPU
)

// Family NET — Network Anomaly
const (
	NET001_DNSNdots          IncidentType = "NET-001" // NXDOMAIN rate >40%
	NET002_CoreDNSOverload   IncidentType = "NET-002" // CoreDNS CPU >80%, latency >50ms
	NET003_MTUMismatch       IncidentType = "NET-003" // Retransmits >5% for large packets
	NET004_ConnectionFlood   IncidentType = "NET-004" // >500 new TCP conns/sec from one pod
	NET005_TCPRetransmits    IncidentType = "NET-005" // Retransmit rate >10%
	NET006_EndpointChurn     IncidentType = "NET-006" // Endpoints updating >10/min
	NET007_ConntrackExhaust  IncidentType = "NET-007" // conntrack >90% of max
)

// Family APP — Application Crash
const (
	APP001_CrashAuth        IncidentType = "APP-001" // Exit 1 + network fail <2s
	APP002_CrashOOM         IncidentType = "APP-002" // Exit 137
	APP003_CrashConfig      IncidentType = "APP-003" // Exit 1 + no network, config read fail
	APP004_CrashReadiness   IncidentType = "APP-004" // Exit 0 + readiness failing
	APP005_CrashPortConflict IncidentType = "APP-005" // Exit 1 + bind() failed
	APP006_CrashDependency  IncidentType = "APP-006" // Exit 1 + TCP connect refused to dep
	APP007_ImagePullFailure IncidentType = "APP-007" // ImagePullBackOff
	APP008_InitContainer    IncidentType = "APP-008" // Init container exits non-zero
)

// Family CTL — Control Plane
const (
	CTL001_EtcdDiskIO       IncidentType = "CTL-001" // etcd wal_fsync p99 >10ms
	CTL002_APIServerOverload IncidentType = "CTL-002" // apiserver p99 >1s
	CTL003_WebhookTimeout   IncidentType = "CTL-003" // Admission webhook >10s
	CTL004_ControllerBacklog IncidentType = "CTL-004" // Work queue depth >1000
	CTL005_SchedulerPending IncidentType = "CTL-005" // Pending pods >50 for >2 min
)

// Severity tiers per spec §5.1
type Severity string

const (
	SevP0 Severity = "P0" // EMERGENCY: control plane down, data loss
	SevP1 Severity = "P1" // CRITICAL: SLO breach, cascading failure
	SevP2 Severity = "P2" // HIGH: single service degraded, elevated errors
	SevP3 Severity = "P3" // MEDIUM: anomaly, right-sizing, trend warning
	SevP4 Severity = "P4" // INFO: successful remediation, routine report
)

// ClassifiedIncident is the output of the classifier.
type ClassifiedIncident struct {
	Family          IncidentFamily
	Type            IncidentType
	Severity        Severity
	AutoRemediable  bool
	Description     string
	DetectedAt      time.Time

	// Service context
	ServiceName     string
	Namespace       string
	PodName         string
	NodeName        string

	// Signal evidence
	PrimarySignal   string  // The main metric/event that triggered classification
	SignalValue     float64 // The value that triggered thresholding
	Threshold       float64 // The threshold that was exceeded
	Confidence      float64 // 0.0-1.0 classifier confidence

	// For APP family: additional context
	ExitCode        int
	StartupDuration time.Duration
	RestartCount    int
}

// IncidentTypeInfo describes a registered incident type.
type IncidentTypeInfo struct {
	Type           IncidentType
	Family         IncidentFamily
	DefaultSev     Severity
	AutoRemediable bool
	Description    string
	DetectionSignal string
}

// Registry contains all 32 registered incident types.
var Registry = map[IncidentType]IncidentTypeInfo{
	// MEM family
	MEM001_OOMKillKernel:     {MEM001_OOMKillKernel, FamilyMemory, SevP1, false, "OOM Kill — container killed by kernel", "kprobe:oom_kill_process"},
	MEM002_OOMKillCgroup:     {MEM002_OOMKillCgroup, FamilyMemory, SevP1, true, "OOM Kill — cgroup limit hit", "cgroup OOM event"},
	MEM003_MemoryLeak:        {MEM003_MemoryLeak, FamilyMemory, SevP2, false, "Memory leak in progress", "RSS growth >5MB/min for 10 min"},
	MEM004_MemoryGrowthSpike: {MEM004_MemoryGrowthSpike, FamilyMemory, SevP1, true, "Memory growth velocity spike", "RSS growth >20MB/min"},
	MEM005_NodeMemPressure:   {MEM005_NodeMemPressure, FamilyMemory, SevP1, true, "Node memory pressure", "Node RSS >85% for 5 min"},
	MEM006_JVMHeapExhaust:    {MEM006_JVMHeapExhaust, FamilyMemory, SevP1, false, "JVM heap exhaustion", "GC >95% heap + OOM"},

	// CPU family
	CPU001_CFSThrottle:       {CPU001_CFSThrottle, FamilyCPU, SevP2, true, "CFS CPU throttling", "throttled_periods/total > 25%"},
	CPU002_NoisyNeighbor:     {CPU002_NoisyNeighbor, FamilyCPU, SevP2, true, "Noisy neighbor syscall rate", "syscall rate > mean + 3σ for 30s"},
	CPU003_NodeCPUSaturation: {CPU003_NodeCPUSaturation, FamilyCPU, SevP1, true, "Node CPU saturation", "Node CPU >95% for 3 min"},
	CPU004_SchedStarvation:   {CPU004_SchedStarvation, FamilyCPU, SevP2, false, "Scheduling starvation", "CFS runqueue wait >50ms"},
	CPU005_SpinLock:          {CPU005_SpinLock, FamilyCPU, SevP2, false, "Spin-lock contention", "High vol vs invol ctxsw ratio"},
	CPU006_CPULimitTight:     {CPU006_CPULimitTight, FamilyCPU, SevP3, true, "CPU limit too tight for burst", "Consistent throttle at low avg CPU"},

	// NET family
	NET001_DNSNdots:          {NET001_DNSNdots, FamilyNetwork, SevP2, false, "DNS NXDOMAIN storm (ndots:5)", "NXDOMAIN rate >40%"},
	NET002_CoreDNSOverload:   {NET002_CoreDNSOverload, FamilyNetwork, SevP1, true, "CoreDNS overload", "CoreDNS CPU >80% + latency >50ms"},
	NET003_MTUMismatch:       {NET003_MTUMismatch, FamilyNetwork, SevP2, false, "MTU mismatch", "Retransmits >5% for large packets only"},
	NET004_ConnectionFlood:   {NET004_ConnectionFlood, FamilyNetwork, SevP1, true, "Connection flood from pod", "New TCP conns >500/sec from one pod"},
	NET005_TCPRetransmits:    {NET005_TCPRetransmits, FamilyNetwork, SevP2, false, "TCP retransmit storm", "TCP retransmit rate >10%"},
	NET006_EndpointChurn:     {NET006_EndpointChurn, FamilyNetwork, SevP2, false, "Service endpoint churn", "Endpoints updating >10/min"},
	NET007_ConntrackExhaust:  {NET007_ConntrackExhaust, FamilyNetwork, SevP1, true, "Conntrack table exhaustion", "nf_conntrack >90% of max"},

	// APP family
	APP001_CrashAuth:         {APP001_CrashAuth, FamilyApplication, SevP1, false, "CrashLoop — auth failure", "Exit 1 + network fail <2s"},
	APP002_CrashOOM:          {APP002_CrashOOM, FamilyApplication, SevP1, true, "CrashLoop — OOM killed", "Exit code 137"},
	APP003_CrashConfig:       {APP003_CrashConfig, FamilyApplication, SevP1, false, "CrashLoop — config missing", "Exit 1 + no network, config read fail"},
	APP004_CrashReadiness:    {APP004_CrashReadiness, FamilyApplication, SevP2, false, "CrashLoop — readiness probe", "Exit 0 + readiness failing"},
	APP005_CrashPortConflict: {APP005_CrashPortConflict, FamilyApplication, SevP2, false, "CrashLoop — port conflict", "Exit 1 + bind() failed"},
	APP006_CrashDependency:   {APP006_CrashDependency, FamilyApplication, SevP1, false, "CrashLoop — dependency unavailable", "Exit 1 + TCP refused to dep"},
	APP007_ImagePullFailure:  {APP007_ImagePullFailure, FamilyApplication, SevP2, false, "Container image pull failure", "ImagePullBackOff"},
	APP008_InitContainer:     {APP008_InitContainer, FamilyApplication, SevP2, false, "Init container failure", "Init container exits non-zero"},

	// CTL family
	CTL001_EtcdDiskIO:        {CTL001_EtcdDiskIO, FamilyControlPlane, SevP1, false, "etcd disk I/O saturation", "etcd wal_fsync p99 >10ms"},
	CTL002_APIServerOverload: {CTL002_APIServerOverload, FamilyControlPlane, SevP1, false, "API server overload", "apiserver p99 >1s"},
	CTL003_WebhookTimeout:    {CTL003_WebhookTimeout, FamilyControlPlane, SevP0, true, "Webhook timeout cascade", "Admission webhook >10s"},
	CTL004_ControllerBacklog: {CTL004_ControllerBacklog, FamilyControlPlane, SevP2, false, "Controller manager backlog", "Work queue depth >1000"},
	CTL005_SchedulerPending:  {CTL005_SchedulerPending, FamilyControlPlane, SevP1, false, "Scheduler pending pods spike", "Pending pods >50 for >2 min"},
}
