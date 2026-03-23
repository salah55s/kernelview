package classifier

import "time"

// classifyApplication handles APP-001 through APP-008.
// Implements the exit code decision tree from spec §4.5.
//
// Decision tree:
//   Exit 0 + readiness failing       → APP-004 (readiness probe)
//   Exit 1 + network fail <2s        → APP-001 (auth failure)
//   Exit 1 + no network, config fail → APP-003 (config missing)
//   Exit 1 + bind() failed           → APP-005 (port conflict)
//   Exit 1 + TCP refused to dep      → APP-006 (dependency unavailable)
//   Exit 137                         → APP-002 (OOM killed) — BUT check liveness probe first!
//   ImagePullBackOff                 → APP-007 (image pull failure)
//   Init container non-zero          → APP-008 (init container failure)
func classifyApplication(s Signals, now time.Time) *ClassifiedIncident {
	// APP-007: Image pull failure (no exit code involved)
	if s.ImagePullBackOff {
		return buildIncident(APP007_ImagePullFailure, s, now,
			"image_pull_backoff", 1, 0, 0.95)
	}

	// APP-008: Init container failure
	if s.InitContainerFailed {
		return buildIncident(APP008_InitContainer, s, now,
			"init_container_failed", 1, 0, 0.90)
	}

	// Exit code 137: SIGKILL / OOM killed
	// Edge case §6.4: Exit 137 can be OOM OR liveness probe timeout → kubelet SIGKILL
	if s.ExitCode == 137 {
		return buildIncident(APP002_CrashOOM, s, now,
			"exit_code_137", 137, 0, 0.85)
	}

	// Exit code 0: Process exited cleanly but pod is restarting
	// Edge case §6.4: CLI tools with restartPolicy:Always exit 0 intentionally
	if s.ExitCode == 0 {
		if s.ReadinessProbeFailure {
			return buildIncident(APP004_CrashReadiness, s, now,
				"exit_0_readiness_fail", 0, 0, 0.80)
		}
		// Exit 0 without readiness failure = possible restartPolicy misconfiguration
		// Don't classify — let it fall through to avoid false positives
		return nil
	}

	// Exit code 1: The ambiguous case — use startup network pattern analysis
	// Edge case §6.4: Distinguish auth failures from config failures by network activity
	if s.ExitCode == 1 {
		// Check startup duration + network calls
		if s.StartupDurationMs < 2000 && s.HasNetworkCallInStartup && s.NetworkCallFailed {
			// Fast crash + network call that failed = auth failure pattern
			// Check if it got 401/403 (strong signal) or connection refused (weaker)
			if s.NetworkCallResult == 401 || s.NetworkCallResult == 403 {
				return buildIncident(APP001_CrashAuth, s, now,
					"exit_1_auth_failure_401_403", 1, 0, 0.90)
			}
			return buildIncident(APP001_CrashAuth, s, now,
				"exit_1_network_fail_startup", 1, 0, 0.75)
		}

		// Exit 1 + bind() failed = port conflict
		if s.HasBindFail {
			return buildIncident(APP005_CrashPortConflict, s, now,
				"exit_1_bind_failed", 1, 0, 0.85)
		}

		// Exit 1 + TCP connect refused to dependency
		if s.HasTCPRefused {
			return buildIncident(APP006_CrashDependency, s, now,
				"exit_1_tcp_refused", 1, 0, 0.80)
		}

		// Exit 1 + no network activity + config read failure
		if !s.HasNetworkCallInStartup && s.HasConfigReadFail {
			return buildIncident(APP003_CrashConfig, s, now,
				"exit_1_config_read_fail", 1, 0, 0.80)
		}

		// Exit 1 + no network activity at all = likely config error
		if !s.HasNetworkCallInStartup {
			return buildIncident(APP003_CrashConfig, s, now,
				"exit_1_no_network_activity", 1, 0, 0.60)
		}
	}

	return nil
}
