// SPDX-License-Identifier: Apache-2.0
// KernelView — Per-PID Syscall Rate Monitoring
//
// Hooks raw_syscalls:sys_enter to count syscalls per PID and cgroup.
// Uses per-CPU maps to avoid lock contention.

#include "headers/maps.h"
#include "headers/helpers.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

SEC("tracepoint/raw_syscalls/sys_enter")
int tracepoint_sys_enter(struct trace_event_raw_sys_enter *ctx) {
    __u32 pid = bpf_get_current_pid_tgid() >> 32;

    // Increment per-PID counter (per-CPU, no lock contention)
    __u64 *count = bpf_map_lookup_elem(&syscall_counts, &pid);
    if (count) {
        __sync_fetch_and_add(count, 1);
    } else {
        __u64 init_val = 1;
        bpf_map_update_elem(&syscall_counts, &pid, &init_val, BPF_ANY);
    }

    // Also increment per-cgroup counter (PID-recycling safe)
    __u64 cgroup_id = get_cgroup_id();
    __u64 *cg_count = bpf_map_lookup_elem(&cgroup_syscall_counts, &cgroup_id);
    if (cg_count) {
        __sync_fetch_and_add(cg_count, 1);
    } else {
        __u64 init_val = 1;
        bpf_map_update_elem(&cgroup_syscall_counts, &cgroup_id, &init_val, BPF_ANY);
    }

    return 0;
}
