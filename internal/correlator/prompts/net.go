package prompts

// dnsNdotsTemplate returns the NET-001 DNS ndots:5 prompt from spec §4.4.
func dnsNdotsTemplate() string {
	return `INCIDENT CLASS: DNS_NDOTS_INEFFICIENCY (NET-001)
AFFECTED SERVICE: {service_name}

DNS QUERY ANALYSIS (last 5 minutes):
  Total DNS queries: {total_queries}
  NXDOMAIN responses: {nxdomain_count} ({nxdomain_rate}%)
  Query pattern sample (last 10 unique queries):
  {dns_query_sample_json}

CURRENT POD DNS CONFIG:
  nameserver: {nameserver}
  search: {search_domains}
  options: {options}

SERVICE LATENCY IMPACT:
  Measured DNS resolution time for external domains: {dns_latency_ms}ms
  Expected without NXDOMAIN waste: {expected_dns_latency_ms}ms
  Estimated latency tax per external call: {tax_ms}ms

COMMON EXTERNAL DOMAINS CALLED BY THIS SERVICE:
{external_domains_json}

The fix is adding dnsConfig to the pod spec. The specific fix depends on
whether the service makes mostly internal or mostly external DNS calls.
For external-heavy services: ndots:1
For mixed services: ndots:2 with explicit search for cluster.local

Respond with JSON: {root_cause, estimated_latency_saving_ms, fix_type,
pod_spec_patch_yaml, confidence, risk_if_applied}`
}

func mtuMismatchTemplate() string {
	return `INCIDENT CLASS: MTU_MISMATCH (NET-003)
AFFECTED SERVICES: {service_name}

NETWORK ANALYSIS:
  Overall retransmit rate: {retransmit_rate}%
  Large packet (>1350 bytes) retransmit rate: {large_retransmit_rate}%
  Small packet retransmit rate: {small_retransmit_rate}%

  Key diagnostic: failure rate correlated with PAYLOAD SIZE, not time or load

OVERLAY NETWORK:
  Type: {overlay_type}  (VXLAN adds 50 bytes, Geneve adds 38-60 bytes)
  Interface: {overlay_interface}
  NIC MTU: {nic_mtu}
  Effective pod MTU: {effective_mtu}

XDP DROP COUNTERS on overlay interface:
{xdp_drops_json}

SIGNATURE: Clean physical NIC stats + drops on overlay + retransmits only for large packets

Respond with JSON: {root_cause, confidence, calculated_safe_mtu,
recommended_mtu_setting, auto_remediation_safe}`
}

func conntrackTemplate() string {
	return `INCIDENT CLASS: CONNTRACK_EXHAUSTION (NET-007)
NODE: {node_name}

CONNTRACK TABLE:
  Current entries: {conntrack_count}
  Maximum: {conntrack_max}
  Usage: {conntrack_percent}%

TOP CONNECTION SOURCES (pods on this node):
{top_connection_pods_json}

IMPACT: New connections silently dropped at kernel level — no error message.
All pods on this node are affected, not just the high-connection pod.

The fix requires node-level sysctl change. Cannot be done from inside a pod.
Must use a DaemonSet job or node-level configuration to modify nf_conntrack_max.

Respond with JSON: {root_cause, confidence, recommended_conntrack_max,
noisy_pod_identified, sysctl_patch, auto_remediation_safe}`
}
