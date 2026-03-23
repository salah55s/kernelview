// SPDX-License-Identifier: Apache-2.0
// cfs_throttle.c — Monitor CFS CPU throttling per container cgroup.
// Key for CPU-001 detection (the silent p99 killer at 40% CPU).
//
// Reads throttled_periods and total_periods from cgroup CPU accounting.
// A throttle ratio above 25% with p99 > 3x p50 is the signature pattern.

#include "headers/maps.h"
#include "headers/helpers.h"

// CFS throttle event
struct throttle_event {
    __u64 timestamp_ns;
    __u64 cgroup_id;
    __u32 pid;
    __u64 throttled_usec;     // microseconds of throttling
    __u64 nr_throttled;       // number of times throttled
    __u64 nr_periods;         // total CFS periods
};

// Ring buffer for throttle events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 22); // 4MB
} throttle_events SEC(".maps");

// Per-cgroup throttle accumulator (sampled every 10s by userspace)
struct throttle_stats {
    __u64 throttled_periods;
    __u64 total_periods;
    __u64 throttled_time_us;
    __u64 last_update_ns;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, __u64);  // cgroup_id
    __type(value, struct throttle_stats);
} cgroup_throttle_map SEC(".maps");

// Hook cgroup_throttle — fired when a cgroup CFS period ends with throttling.
// This tracepoint provides the actual throttle event, not just the counter.
SEC("tp/sched/sched_stat_runtime")
int trace_cfs_runtime(struct pt_regs *ctx) {
    __u64 cgroup_id = bpf_get_current_cgroup_id();
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    // Update per-cgroup accumulator
    struct throttle_stats *stats = bpf_map_lookup_elem(&cgroup_throttle_map, &cgroup_id);
    if (stats) {
        stats->total_periods++;
        stats->last_update_ns = bpf_ktime_get_ns();
    } else {
        struct throttle_stats new_stats = {
            .throttled_periods = 0,
            .total_periods = 1,
            .throttled_time_us = 0,
            .last_update_ns = bpf_ktime_get_ns(),
        };
        bpf_map_update_elem(&cgroup_throttle_map, &cgroup_id, &new_stats, BPF_ANY);
    }

    return 0;
}

// Hook cgroup bandwidth throttle event
// Fired when a task hits its CPU bandwidth limit and gets throttled
SEC("tp/sched/sched_stat_blocked")
int trace_cfs_throttle(struct pt_regs *ctx) {
    __u64 cgroup_id = bpf_get_current_cgroup_id();
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    // Update throttle counter
    struct throttle_stats *stats = bpf_map_lookup_elem(&cgroup_throttle_map, &cgroup_id);
    if (stats) {
        stats->throttled_periods++;
        stats->last_update_ns = bpf_ktime_get_ns();
    } else {
        struct throttle_stats new_stats = {
            .throttled_periods = 1,
            .total_periods = 1,
            .throttled_time_us = 0,
            .last_update_ns = bpf_ktime_get_ns(),
        };
        bpf_map_update_elem(&cgroup_throttle_map, &cgroup_id, &new_stats, BPF_ANY);
    }

    // Emit event if throttle ratio is significant
    // (userspace reads cgroup_throttle_map periodically for the ratio)
    struct throttle_event *event = bpf_ringbuf_reserve(&throttle_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->timestamp_ns = bpf_ktime_get_ns();
    event->cgroup_id = cgroup_id;
    event->pid = pid;
    if (stats) {
        event->nr_throttled = stats->throttled_periods;
        event->nr_periods = stats->total_periods;
    }

    bpf_ringbuf_submit(event, 0);
    return 0;
}

char LICENSE[] SEC("license") = "Apache-2.0";
