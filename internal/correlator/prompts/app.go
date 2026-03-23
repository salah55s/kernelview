package prompts

// crashLoopTemplate returns the APP-001 through APP-008 CrashLoopBackOff
// decision tree prompt from spec §4.5.
func crashLoopTemplate() string {
	return `INCIDENT CLASS: CRASH_LOOP_BACKOFF ({incident_code})
POD: {pod_name}  NAMESPACE: {namespace}
RESTART COUNT: {restart_count}  LAST RESTART: {last_restart_ago}

EXIT CODE ANALYSIS:
  Last exit code: {exit_code}
  Exit code meaning:
    0   = process exited cleanly → readiness probe is failing (APP-004)
    1   = general error → check network calls and config reads
    137 = SIGKILL / OOM → memory limit too low (APP-002)
    143 = SIGTERM → probe timeout or deployment rolling update
    126 = permission denied → init script or binary permissions
    128+N = signal N killed the process → check dmesg/kernel

TIME FROM CONTAINER START TO EXIT: {startup_duration_ms}ms
  <2000ms with network call failure = AUTH_FAILURE (APP-001)
  <2000ms with no network calls     = CONFIG_MISSING (APP-003)
  >5000ms with gradual memory growth = MEMORY_LEAK (MEM-003)

NETWORK CALLS IN FIRST 5 SECONDS OF LIFE:
{network_calls_json}
  Look for: failed TLS handshakes, TCP refused, 401/403 HTTP responses

SYSCALL PATTERN AT CRASH:
{syscall_sequence_json}
  openat() failures = missing files (config, secrets, certs)
  connect() failures = dependency unavailable
  mmap() large = pre-OOM memory pressure

KUBERNETES EVENTS:
{k8s_events_json}

POD LOGS (last 30 lines, scrubbed):
{pod_logs}

CLASSIFY this as exactly one of: APP-001 (auth), APP-002 (OOM),
APP-003 (config), APP-004 (readiness), APP-005 (port conflict),
APP-006 (dependency), APP-007 (image pull), APP-008 (init container)

Respond with JSON: {incident_subtype, root_cause, evidence_from_logs,
evidence_from_syscalls, recommended_action, human_required,
estimated_fix_time_minutes, confidence}`
}
