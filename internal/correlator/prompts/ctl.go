package prompts

// etcdIOTemplate returns the CTL-001 etcd I/O saturation prompt from spec §4.6.
func etcdIOTemplate() string {
	return `INCIDENT CLASS: ETCD_IO_SATURATION (CTL-001)
NODE: {etcd_node_name}

etcd METRICS (last 15 minutes):
  wal_fsync p50: {wal_p50_ms}ms  [warning: >5ms]
  wal_fsync p99: {wal_p99_ms}ms  [critical: >100ms]
  backend_commit p99: {commit_p99_ms}ms
  leader_changes last hour: {leader_changes}
  proposal_failed_total delta: {proposal_failures}

DISK METRICS ON etcd NODE:
  IOPS (read): {disk_read_iops}
  IOPS (write): {disk_write_iops}
  IOPS limit (cloud): {disk_iops_limit}
  Queue depth: {io_queue_depth}

API SERVER IMPACT:
  apiserver_request_duration p99: {apiserver_p99_ms}ms
  apiserver_request_total (last 1m): {apiserver_rps}
  apiserver_current_inflight_requests: {inflight}

CLUSTER ACTIVITY (triggers for etcd write amplification):
  Active deployments (last 10 min): {active_deployments}
  HPA scale events (last 10 min): {hpa_events}
  CRD objects modified (last 10 min): {crd_modifications}

SEVERITY CLASSIFICATION:
  wal_fsync p99 >10ms  = WARNING: performance degradation beginning
  wal_fsync p99 >100ms = CRITICAL: control plane will become unresponsive
  wal_fsync p99 >500ms = P0 EMERGENCY: cluster split-brain risk

Respond with JSON: {severity, time_to_impact_minutes, disk_bottleneck_confirmed,
recommended_immediate_action, recommended_permanent_fix, cluster_risk_level,
should_reduce_api_server_load, confidence}`
}

func webhookTimeoutTemplate() string {
	return `INCIDENT CLASS: WEBHOOK_TIMEOUT_CASCADE (CTL-003)
SEVERITY: P0 — BLOCKS ALL POD CREATION

API SERVER SYMPTOMS:
  CREATE/UPDATE operations: FAILING (503 or timeout)
  GET operations: SUCCEEDING
  This asymmetry is the signature of a webhook deadlock.

WEBHOOK CONFIGURATION:
{webhook_configs_json}

WEBHOOK ENDPOINT STATUS:
{webhook_endpoint_status_json}

DEADLOCK CHECK: Is the webhook endpoint itself a Kubernetes pod that
needs to be created? If yes, this is a circular deadlock:
  pod can't start → can't validate → can't start

Respond with JSON: {root_cause, is_deadlock, affected_webhook_name,
immediate_action, bypass_label_suggested, confidence,
auto_remediation_safe}`
}
