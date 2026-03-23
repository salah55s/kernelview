// SPDX-License-Identifier: Apache-2.0
// KernelView — Shared BPF Map Definitions
#ifndef __KERNELVIEW_MAPS_H
#define __KERNELVIEW_MAPS_H

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

// ============================================================
// Common Constants
// ============================================================

#define MAX_HTTP_HEADER_SIZE  256
#define MAX_COMM_SIZE         16
#define MAX_FILENAME_SIZE     256
#define MAX_ENTRIES           65536
#define RINGBUF_SIZE          (64 * 1024 * 1024) // 64MB

// ============================================================
// HTTP Trace Types
// ============================================================

struct http_event {
    __u64 timestamp_ns;
    __u32 pid;
    __u32 tid;
    __u64 cgroup_id;
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u32 status_code;
    __u64 duration_ns;
    __u64 request_size;
    __u64 response_size;
    char method[8];
    char path[MAX_HTTP_HEADER_SIZE];
    char host[64];
    __u8 is_response;
    __u8 protocol; // 0=unknown, 1=HTTP/1.x, 2=HTTP/2
};

// Per-socket request tracking for latency calculation
struct request_timestamp {
    __u64 timestamp_ns;
    __u32 pid;
    __u64 cgroup_id;
};

// ============================================================
// TCP Event Types
// ============================================================

struct tcp_event {
    __u64 timestamp_ns;
    __u32 pid;
    __u64 cgroup_id;
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u8  event_type; // 1=connect, 2=accept, 3=close, 4=retransmit, 5=state_change
    __u8  old_state;
    __u8  new_state;
    __u32 retransmit_count;
};

// ============================================================
// OOM Event Types
// ============================================================

struct oom_event {
    __u64 timestamp_ns;
    __u32 victim_pid;
    char  victim_comm[MAX_COMM_SIZE];
    __u64 pages_requested;
    __s32 oom_score_adj;
    __u64 cgroup_id;
    __u8  kill_source; // 0=kernel_oom, 1=cgroup_oom
};

// ============================================================
// Exec Event Types
// ============================================================

struct exec_event {
    __u64 timestamp_ns;
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u64 cgroup_id;
    char  comm[MAX_COMM_SIZE];
    char  filename[MAX_FILENAME_SIZE];
};

// ============================================================
// Shared Ring Buffer (all events go through this)
// ============================================================

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, RINGBUF_SIZE);
} events SEC(".maps");

// ============================================================
// HTTP Request Timestamp Map (for latency calculation)
// ============================================================

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u64);                  // struct sock pointer
    __type(value, struct request_timestamp);
} request_timestamps SEC(".maps");

// ============================================================
// Syscall Count Maps (per-CPU to avoid lock contention)
// ============================================================

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u32);   // PID
    __type(value, __u64); // syscall count
} syscall_counts SEC(".maps");

// Per-cgroup syscall count (PID-recycling safe)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u64);   // cgroup ID
    __type(value, __u64); // syscall count
} cgroup_syscall_counts SEC(".maps");

// ============================================================
// Sampling Configuration
// ============================================================

struct sampling_config {
    __u8  enabled;       // 0=off, 1=on
    __u32 sample_rate;   // 1 in N (e.g., 10 = capture 1 in 10)
    __u32 counter;       // Internal counter
};

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct sampling_config);
} sampling SEC(".maps");

// ============================================================
// Per-socket tracking (for multi-call HTTP request handling)
// ============================================================

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u64);   // struct sock pointer
    __type(value, __u8);  // 1 = already captured this request
} active_requests SEC(".maps");

#endif // __KERNELVIEW_MAPS_H
