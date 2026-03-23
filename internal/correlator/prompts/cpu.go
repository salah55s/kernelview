package prompts

// cfsThrottleTemplate returns the CPU-001 CFS Throttling prompt from spec §4.3.
func cfsThrottleTemplate() string {
	return `INCIDENT CLASS: CPU_THROTTLING (CPU-001)
AFFECTED SERVICE: {service_name} in {namespace}

CPU METRICS (last 30 minutes):
  Average CPU utilization: {avg_cpu_percent}%
  CFS throttled periods: {throttled_periods} / {total_periods} = {throttle_rate}%
  p50 latency: {p50_ms}ms
  p99 latency: {p99_ms}ms
  p99/p50 ratio: {p99_p50_ratio}x  [ALERT if >3x]

DECLARED CPU LIMIT: {cpu_limit_millicores}m
DECLARED CPU REQUEST: {cpu_request_millicores}m

TRAFFIC PATTERN:
{requests_per_second_timeseries}

KNOWN PATTERN: CFS throttling causes p99 explosions even at low average CPU.
The fix is NOT to increase CPU limits — it is often to REMOVE them for
latency-sensitive services and rely on requests-only scheduling.
Only recommend removing limits if: (1) this is not a shared tenant cluster,
(2) other pods on the same node are not being starved.

CHECK: Is this service part of a critical path? If yes, removing limits is safer.
CHECK: Is the node CPU >80%? If yes, removing limits would hurt neighbors.

Respond with JSON: {root_cause, is_shared_cluster_risk, recommended_action,
limit_recommendation, confidence, auto_remediation_safe}`
}

func noisyNeighborTemplate() string {
	return `INCIDENT CLASS: NOISY_NEIGHBOR (CPU-002)
AFFECTED NODE: {node_name}

NOISY POD: {noisy_pod_name} in {noisy_namespace}
  Syscall rate: {syscall_rate}/sec (mean: {mean_rate}, threshold: {threshold_rate})
  Duration above threshold: {duration_seconds}s

VICTIM PODS ON SAME NODE:
{victim_pods_json}

Respond with JSON: {root_cause, noisy_pod, confidence, recommended_action,
throttle_target_millicores, auto_remediation_safe}`
}

func nodeCPUTemplate() string {
	return `INCIDENT CLASS: NODE_CPU_SATURATION (CPU-003)
NODE: {node_name}
CPU: {node_cpu_percent}% for {duration_minutes} minutes

TOP CPU CONSUMERS:
{top_cpu_pods_json}

Respond with JSON: {root_cause, confidence, eviction_candidates,
scale_recommendation, auto_remediation_safe}`
}

func cpuLimitTightTemplate() string {
	return `INCIDENT CLASS: CPU_LIMIT_TOO_TIGHT (CPU-006)
SERVICE: {service_name} in {namespace}

  Average CPU: {avg_cpu_percent}% (well below limit)
  Throttle rate: {throttle_rate}%
  CPU limit: {cpu_limit_millicores}m

This pattern indicates burst CPU usage exceeding the limit even though
average utilization is low. The service needs more headroom for burst.

Respond with JSON: {root_cause, confidence, recommended_limit_millicores,
auto_remediation_safe}`
}
