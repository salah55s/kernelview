package classifier

import "time"

// classifyMemory handles MEM-001 through MEM-006.
func classifyMemory(s Signals, now time.Time) *ClassifiedIncident {
	// MEM-001: Kernel OOM kill
	if s.OOMKillDetected && s.OOMKillSource == "kernel" {
		return buildIncident(MEM001_OOMKillKernel, s, now,
			"kprobe:oom_kill_process", 1, 0, 0.95)
	}

	// MEM-002: Cgroup OOM kill
	if s.OOMKillDetected && s.OOMKillSource == "cgroup" {
		return buildIncident(MEM002_OOMKillCgroup, s, now,
			"cgroup_oom_event", 1, 0, 0.95)
	}

	// MEM-006: JVM heap exhaustion (check before MEM-003 as it's more specific)
	// Edge case §6.3: Java pods with large heaps need different limit recommendations
	if s.IsJVMProcess && s.JVMHeapPercent > 95 {
		return buildIncident(MEM006_JVMHeapExhaust, s, now,
			"jvm_heap_percent", s.JVMHeapPercent, 95, 0.85)
	}

	// MEM-004: Sudden memory growth spike (>20MB/min)
	if s.RSSGrowthMBPerMin > 20 {
		return buildIncident(MEM004_MemoryGrowthSpike, s, now,
			"rss_growth_mb_per_min", s.RSSGrowthMBPerMin, 20, 0.80)
	}

	// MEM-003: Memory leak (>5MB/min for 10+ consecutive minutes)
	if s.RSSGrowthMBPerMin > 5 && s.RSSGrowthDuration >= 10*time.Minute {
		return buildIncident(MEM003_MemoryLeak, s, now,
			"rss_growth_mb_per_min_sustained", s.RSSGrowthMBPerMin, 5, 0.75)
	}

	// MEM-005: Node memory pressure (>85% for 5+ minutes)
	if s.NodeMemoryPercent > 85 && s.NodeMemDuration >= 5*time.Minute {
		return buildIncident(MEM005_NodeMemPressure, s, now,
			"node_memory_percent", s.NodeMemoryPercent, 85, 0.90)
	}

	return nil
}
