package classifier

import "time"

// classifyCPU handles CPU-001 through CPU-006.
func classifyCPU(s Signals, now time.Time) *ClassifiedIncident {
	// CPU-001: CFS throttling with p99 latency impact
	// Edge case §6.2: Look at throttle ratio + p99/p50, NOT average CPU
	if s.ThrottleRatio > 0.25 {
		// Check if this is actually impacting latency (p99 > 3x p50)
		if s.LatencyP99Ms > 0 && s.LatencyP50Ms > 0 && s.LatencyP99Ms > 3*s.LatencyP50Ms {
			return buildIncident(CPU001_CFSThrottle, s, now,
				"cfs_throttle_ratio_with_latency", s.ThrottleRatio, 0.25, 0.90)
		}
		// Throttling at low average CPU = limit too tight for burst (CPU-006)
		if s.AvgCPUPercent < 60 {
			return buildIncident(CPU006_CPULimitTight, s, now,
				"cfs_throttle_at_low_avg_cpu", s.ThrottleRatio, 0.25, 0.80)
		}
	}

	// CPU-003: Node CPU saturation (>95% for 3+ minutes)
	if s.NodeCPUPercent > 95 && s.NodeCPUDuration >= 3*time.Minute {
		return buildIncident(CPU003_NodeCPUSaturation, s, now,
			"node_cpu_percent", s.NodeCPUPercent, 95, 0.90)
	}

	// CPU-002: Noisy neighbor (syscall rate > mean + 3σ for 30+ seconds)
	if s.SyscallRateStdDev > 0 {
		threshold := s.SyscallRateMean + 3*s.SyscallRateStdDev
		if s.SyscallRate > threshold && s.SyscallDuration >= 30*time.Second {
			return buildIncident(CPU002_NoisyNeighbor, s, now,
				"syscall_rate_sigma", s.SyscallRate, threshold, 0.85)
		}
	}

	// CPU-004: Scheduling starvation (runqueue wait >50ms)
	if s.RunqueueWaitMs > 50 {
		return buildIncident(CPU004_SchedStarvation, s, now,
			"cfs_runqueue_wait_ms", s.RunqueueWaitMs, 50, 0.70)
	}

	return nil
}
