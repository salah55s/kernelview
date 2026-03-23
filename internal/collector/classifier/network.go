package classifier

import "time"

// classifyNetwork handles NET-001 through NET-007.
func classifyNetwork(s Signals, now time.Time) *ClassifiedIncident {
	// NET-007: Conntrack exhaustion (most critical — silent drops, no error msg)
	// Edge case §6.7: conntrack is per-node, affects ALL pods
	if s.ConntrackPercent > 90 {
		return buildIncident(NET007_ConntrackExhaust, s, now,
			"conntrack_percent", s.ConntrackPercent, 90, 0.95)
	}

	// NET-004: Connection flood (>500 new TCP conns/sec from one pod)
	if s.NewTCPConnsPerSec > 500 {
		return buildIncident(NET004_ConnectionFlood, s, now,
			"new_tcp_conns_per_sec", s.NewTCPConnsPerSec, 500, 0.90)
	}

	// NET-002: CoreDNS overload
	if s.CoreDNSCPUPercent > 80 && s.DNSLatencyMs > 50 {
		return buildIncident(NET002_CoreDNSOverload, s, now,
			"coredns_cpu_and_latency", s.CoreDNSCPUPercent, 80, 0.85)
	}

	// NET-001: DNS NXDOMAIN storm (ndots:5 problem)
	// Edge case §6.1: check if service uses short names internally before recommending fix
	if s.NXDOMAINRate > 40 {
		return buildIncident(NET001_DNSNdots, s, now,
			"nxdomain_rate_percent", s.NXDOMAINRate, 40, 0.85)
	}

	// NET-003: MTU mismatch
	// Edge case §6.5: failure correlates with payload size, not time/load
	// Only classify if retransmits are specifically on large packets
	if s.LargePacketRetransmitRate > 5 && s.RetransmitRate < 5 {
		return buildIncident(NET003_MTUMismatch, s, now,
			"large_packet_retransmit_rate", s.LargePacketRetransmitRate, 5, 0.80)
	}

	// NET-005: TCP retransmit storm (general, not size-specific)
	if s.TCPRetransmitRate > 10 {
		return buildIncident(NET005_TCPRetransmits, s, now,
			"tcp_retransmit_rate", s.TCPRetransmitRate, 10, 0.75)
	}

	// NET-006: Endpoint churn (>10 updates/min)
	if s.EndpointChangesPerMin > 10 {
		return buildIncident(NET006_EndpointChurn, s, now,
			"endpoint_changes_per_min", s.EndpointChangesPerMin, 10, 0.70)
	}

	return nil
}
