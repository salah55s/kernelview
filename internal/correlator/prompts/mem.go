package prompts

// oomKillTemplate returns the MEM-001/MEM-002 OOM Kill prompt from spec §4.2.
func oomKillTemplate() string {
	return `INCIDENT CLASS: OOM_KILL ({incident_code})
AFFECTED POD: {pod_name} in namespace {namespace} on node {node_name}
KILL MECHANISM: {kill_mechanism}
TIMESTAMP: {timestamp}

MEMORY TIMELINE (60 seconds before kill):
{memory_rss_timeseries_json}
  — Look for: linear growth (leak), sudden spike (batch job), plateau then drop (GC pressure)

CGROUP LIMIT: {memory_limit_bytes} bytes ({memory_limit_mb} MB)
RSS AT KILL:  {rss_at_kill_bytes} bytes ({rss_at_kill_mb} MB)

KUBERNETES EVENTS (last 10 minutes):
{k8s_events_json}

RECENT LOG LINES (last 20, secrets scrubbed):
{pod_logs}

SIBLING PODS AFFECTED: {sibling_pods_killed_json}
  — If siblings were killed: this is likely a cascade, not just one pod's fault

KNOWN PATTERNS TO CHECK:
  - Java app + rapid RSS growth = heap not released to OS (use -XX:+UseZGC)
  - Node.js + growth at traffic spikes = event loop leak or buffer accumulation
  - Go app + OOM = goroutine leak (look for goroutines: N in logs)
  - cgroup limit hit at 70% RSS = memory accounting includes page cache

Respond with JSON: {root_cause, trigger, confidence, severity, affected_language_runtime,
recommended_action, auto_remediation_safe, action_type, estimated_time_to_fix}`
}

func memoryLeakTemplate() string {
	return `INCIDENT CLASS: MEMORY_LEAK ({incident_code})
AFFECTED SERVICE: {service_name} in {namespace}

MEMORY GROWTH ANALYSIS:
  Current RSS growth rate: {rss_growth_mb_per_min} MB/min
  Duration of sustained growth: {growth_duration_minutes} minutes
  Current RSS: {current_rss_mb} MB
  Memory limit: {memory_limit_mb} MB
  Projected time to OOM: {projected_oom_minutes} minutes

RSS TIMELINE (last 30 minutes, 10s samples):
{memory_rss_timeseries_json}

KUBERNETES EVENTS:
{k8s_events_json}

RECENT LOGS (last 20 lines, scrubbed):
{pod_logs}

KNOWN PATTERNS:
  - Linear growth = classic memory leak (object not freed)
  - Staircase growth = cache without eviction
  - Growth only under load = connection/buffer leak
  - Go: goroutine count growing = goroutine leak

Respond with JSON: {root_cause, confidence, severity, leak_pattern,
recommended_action, auto_remediation_safe, estimated_time_to_oom_minutes}`
}

func nodeMemPressureTemplate() string {
	return `INCIDENT CLASS: NODE_MEMORY_PRESSURE (MEM-005)
NODE: {node_name}

NODE MEMORY: {node_memory_percent}% used for {duration_minutes} minutes
PODS ON NODE (sorted by RSS):
{pods_by_rss_json}

TOP MEMORY CONSUMERS:
{top_consumers_json}

EVICTION CANDIDATES (by QoS class):
  BestEffort pods: {besteffort_pods}
  Burstable pods:  {burstable_pods}

Respond with JSON: {root_cause, confidence, severity, eviction_candidates,
recommended_action, auto_remediation_safe}`
}

func jvmHeapTemplate() string {
	return `INCIDENT CLASS: JVM_HEAP_EXHAUSTION (MEM-006)
POD: {pod_name}  SERVICE: {service_name}

JVM METRICS:
  Heap used: {jvm_heap_percent}%
  GC pause time (last 5 min): {gc_pause_ms}ms total
  Full GC count (last hour): {full_gc_count}
  -Xmx setting: {xmx_value}

KNOWN PATTERNS:
  - Frequent Full GC with high heap = heap too small for workload
  - Metaspace OOM = class loader leak (common with Spring hot-reload)
  - G1GC humongous allocation = objects >50% of region size

Respond with JSON: {root_cause, confidence, jvm_fix_type,
recommended_xmx, gc_recommendation, auto_remediation_safe}`
}
