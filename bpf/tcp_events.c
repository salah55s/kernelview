// SPDX-License-Identifier: Apache-2.0
// KernelView — TCP Connection Lifecycle Events
//
// Hooks tcp tracepoints to capture connection lifecycle events
// and retransmits for network health monitoring.

#include "headers/maps.h"
#include "headers/helpers.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// ============================================================
// tcp_retransmit_skb — Retransmit detection
// ============================================================

SEC("tracepoint/tcp/tcp_retransmit_skb")
int tracepoint_tcp_retransmit(struct trace_event_raw_tcp_retransmit_skb *ctx) {
    struct tcp_event *evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->pid = bpf_get_current_pid_tgid() >> 32;
    evt->cgroup_id = get_cgroup_id();
    evt->event_type = 4; // RETRANSMIT

    // Read addresses from tracepoint context
    evt->src_ip = BPF_CORE_READ(ctx, saddr);
    evt->dst_ip = BPF_CORE_READ(ctx, daddr);
    evt->src_port = BPF_CORE_READ(ctx, sport);
    evt->dst_port = __builtin_bswap16(BPF_CORE_READ(ctx, dport));

    bpf_ringbuf_submit(evt, 0);
    return 0;
}

// ============================================================
// inet_sock_set_state — TCP state change tracking
// ============================================================

SEC("tracepoint/sock/inet_sock_set_state")
int tracepoint_inet_sock_set_state(struct trace_event_raw_inet_sock_set_state *ctx) {
    // Only track TCP (protocol 6)
    int protocol = BPF_CORE_READ(ctx, protocol);
    if (protocol != 6) // IPPROTO_TCP
        return 0;

    struct tcp_event *evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->pid = bpf_get_current_pid_tgid() >> 32;
    evt->cgroup_id = get_cgroup_id();
    evt->event_type = 5; // STATE_CHANGE

    evt->old_state = BPF_CORE_READ(ctx, oldstate);
    evt->new_state = BPF_CORE_READ(ctx, newstate);

    evt->src_ip = BPF_CORE_READ(ctx, saddr);
    evt->dst_ip = BPF_CORE_READ(ctx, daddr);
    evt->src_port = BPF_CORE_READ(ctx, sport);
    evt->dst_port = __builtin_bswap16(BPF_CORE_READ(ctx, dport));

    bpf_ringbuf_submit(evt, 0);
    return 0;
}
