package classifier

import "time"

// classifyControlPlane handles CTL-001 through CTL-005.
// Control plane issues are checked first because they affect everything.
func classifyControlPlane(s Signals, now time.Time) *ClassifiedIncident {
	// CTL-003: Webhook timeout cascade (P0 — blocks ALL pod creation)
	// Edge case §6.8: failurePolicy:Fail + webhook pod down = deadlock
	if s.WebhookTimeoutSec > 10 {
		return buildIncident(CTL003_WebhookTimeout, s, now,
			"webhook_timeout_sec", s.WebhookTimeoutSec, 10, 0.90)
	}

	// CTL-001: etcd disk I/O saturation
	// Edge case §6.6: doesn't show up in node CPU/memory — only etcd_disk_wal_fsync
	if s.EtcdWALFsyncP99Ms > 100 {
		// CRITICAL: >100ms = control plane will become unresponsive
		inc := buildIncident(CTL001_EtcdDiskIO, s, now,
			"etcd_wal_fsync_p99_ms", s.EtcdWALFsyncP99Ms, 100, 0.90)
		inc.Severity = SevP0 // Upgrade to P0 when >100ms
		return inc
	}
	if s.EtcdWALFsyncP99Ms > 10 {
		// WARNING: performance degradation beginning
		return buildIncident(CTL001_EtcdDiskIO, s, now,
			"etcd_wal_fsync_p99_ms", s.EtcdWALFsyncP99Ms, 10, 0.85)
	}

	// CTL-002: API server overload
	if s.APIServerP99Ms > 1000 {
		return buildIncident(CTL002_APIServerOverload, s, now,
			"apiserver_request_p99_ms", s.APIServerP99Ms, 1000, 0.85)
	}

	// CTL-005: Scheduler pending pods spike
	if s.PendingPodCount > 50 && s.PendingPodDuration >= 2*time.Minute {
		return buildIncident(CTL005_SchedulerPending, s, now,
			"pending_pod_count", float64(s.PendingPodCount), 50, 0.80)
	}

	// CTL-004: Controller manager backlog
	if s.ControllerQueueDepth > 1000 {
		return buildIncident(CTL004_ControllerBacklog, s, now,
			"controller_queue_depth", float64(s.ControllerQueueDepth), 1000, 0.75)
	}

	return nil
}
