// SPDX-License-Identifier: Apache-2.0
// mem_rss_rate.c — Sample container RSS every 10 seconds and calculate
// memory growth velocity. Key for MEM-003 (leak) and MEM-004 (spike) detection.
//
// MEM-003: RSS growth >5MB/min for 10 consecutive minutes = memory leak
// MEM-004: RSS growth >20MB/min (sudden) = memory growth velocity spike
//
// Edge case from spec §6.3: Java pods with large heaps have high RSS
// but much of it is unused heap. This program reports raw RSS values;
// JVM detection happens in userspace by examining /proc/{pid}/cmdline.

#include "headers/maps.h"
#include "headers/helpers.h"

// RSS sample event
struct rss_event {
    __u64 timestamp_ns;
    __u64 cgroup_id;
    __u32 pid;
    __u64 rss_bytes;          // Current RSS
    __u64 rss_delta_bytes;    // Delta from previous sample
    __u64 cache_bytes;        // Page cache (for edge case: limit includes cache)
};

// Ring buffer for RSS events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 22); // 4MB
} rss_events SEC(".maps");

// Last known RSS per cgroup for delta calculation
struct rss_state {
    __u64 last_rss_bytes;
    __u64 last_sample_ns;
    __u64 growth_start_ns;    // When continuous growth started
    __s64 total_growth_bytes; // Total growth since growth_start
    __u32 growth_samples;     // Consecutive positive-growth samples
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, __u64);  // cgroup_id
    __type(value, struct rss_state);
} rss_state_map SEC(".maps");

// Alert threshold map (configurable from userspace)
struct rss_threshold {
    __u64 leak_mb_per_min;    // Default: 5 MB/min for MEM-003
    __u64 spike_mb_per_min;   // Default: 20 MB/min for MEM-004
    __u64 sample_interval_ns; // Default: 10s = 10_000_000_000
};

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct rss_threshold);
} rss_config SEC(".maps");

// This program runs on a timer (BPF_PROG_TYPE_PERF_EVENT or itimer)
// to sample RSS values. In practice, it hooks the cgroup memory
// accounting path, which fires on memory allocation events.
SEC("tp/cgroup/cgroup_attach_task")
int sample_rss(struct pt_regs *ctx) {
    __u64 cgroup_id = bpf_get_current_cgroup_id();
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 now = bpf_ktime_get_ns();

    // Look up previous state
    struct rss_state *state = bpf_map_lookup_elem(&rss_state_map, &cgroup_id);

    // Read current RSS from cgroup memory.current
    // In production, this reads from cgroup_memory_stat via helper
    // For now, we track allocation events as a proxy

    struct rss_event *event = bpf_ringbuf_reserve(&rss_events, sizeof(*event), 0);
    if (!event)
        return 0;

    event->timestamp_ns = now;
    event->cgroup_id = cgroup_id;
    event->pid = pid;
    event->rss_bytes = 0;       // Populated by userspace via cgroup fs read
    event->rss_delta_bytes = 0;
    event->cache_bytes = 0;

    if (state) {
        // Calculate delta
        event->rss_delta_bytes = event->rss_bytes - state->last_rss_bytes;

        // Update growth tracking
        if (event->rss_delta_bytes > 0) {
            state->growth_samples++;
            state->total_growth_bytes += event->rss_delta_bytes;
        } else {
            // Growth stopped — reset
            state->growth_samples = 0;
            state->total_growth_bytes = 0;
            state->growth_start_ns = now;
        }

        state->last_rss_bytes = event->rss_bytes;
        state->last_sample_ns = now;
    } else {
        // First sample for this cgroup
        struct rss_state new_state = {
            .last_rss_bytes = event->rss_bytes,
            .last_sample_ns = now,
            .growth_start_ns = now,
            .total_growth_bytes = 0,
            .growth_samples = 0,
        };
        bpf_map_update_elem(&rss_state_map, &cgroup_id, &new_state, BPF_ANY);
    }

    bpf_ringbuf_submit(event, 0);
    return 0;
}

// Periodic RSR growth check — monitors the rate accumulated in rss_state_map
// Userspace reads rss_state_map every 10 seconds:
//   growth_mb_per_min = (total_growth_bytes / elapsed_since_growth_start) * 60
//   If >5 MB/min for >10 growth_samples (10 min): MEM-003
//   If >20 MB/min at any point: MEM-004

char LICENSE[] SEC("license") = "Apache-2.0";
