// SPDX-License-Identifier: Apache-2.0
// KernelView — HTTP Request/Response Capture
//
// Hooks tcp_sendmsg and tcp_recvmsg to capture HTTP traffic at the
// kernel socket layer. Zero application changes required.

#include "headers/maps.h"
#include "headers/helpers.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// ============================================================
// tcp_recvmsg — Capture incoming data (HTTP requests to servers)
// ============================================================

SEC("kprobe/tcp_recvmsg")
int kprobe_tcp_recvmsg(struct pt_regs *ctx) {
    if (!should_sample())
        return 0;

    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);

    // Store the request timestamp for latency calculation.
    // When the corresponding tcp_sendmsg fires for the response,
    // we calculate duration = response_ts - request_ts.
    __u64 sk_key = (__u64)sk;
    __u8 *active = bpf_map_lookup_elem(&active_requests, &sk_key);
    if (active) {
        // Already tracking this socket — multi-call, skip
        return 0;
    }

    struct request_timestamp ts = {};
    ts.timestamp_ns = bpf_ktime_get_ns();
    ts.pid = bpf_get_current_pid_tgid() >> 32;
    ts.cgroup_id = get_cgroup_id();

    bpf_map_update_elem(&request_timestamps, &sk_key, &ts, BPF_ANY);

    // Mark this socket as having an active request
    __u8 one = 1;
    bpf_map_update_elem(&active_requests, &sk_key, &one, BPF_ANY);

    return 0;
}

// ============================================================
// tcp_sendmsg — Capture outgoing data (HTTP responses from servers)
// ============================================================

SEC("kprobe/tcp_sendmsg")
int kprobe_tcp_sendmsg(struct pt_regs *ctx) {
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    struct msghdr *msg = (struct msghdr *)PT_REGS_PARM2(ctx);
    size_t size = (size_t)PT_REGS_PARM3(ctx);

    // Read the first bytes of the message to detect HTTP
    char buf[MAX_HTTP_HEADER_SIZE];
    __builtin_memset(buf, 0, sizeof(buf));

    // Get iov_iter from msghdr
    struct iov_iter iter;
    BPF_CORE_READ_INTO(&iter, msg, msg_iter);

    // Read from userspace memory — MUST use bpf_probe_read_user
    const void __user *iov_base;

    // Try to read the first iovec base pointer
    // This handles the common case of a single iovec
    struct iovec *iov;
    BPF_CORE_READ_INTO(&iov, &iter, __iov);
    if (!iov)
        return 0;

    BPF_CORE_READ_INTO(&iov_base, iov, iov_base);
    if (!iov_base)
        return 0;

    int read_size = size < MAX_HTTP_HEADER_SIZE ? size : MAX_HTTP_HEADER_SIZE;
    if (bpf_probe_read_user(buf, read_size & 0xFF, iov_base) < 0)
        return 0;

    // Check if this is HTTP traffic
    int is_request = is_http_request(buf, read_size);
    int is_response = is_http_response(buf, read_size);

    if (!is_request && !is_response)
        return 0;

    if (!should_sample())
        return 0;

    // Build the HTTP event
    struct http_event *evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->pid = bpf_get_current_pid_tgid() >> 32;
    evt->cgroup_id = get_cgroup_id();
    evt->is_response = is_response ? 1 : 0;

    // Extract socket info
    extract_sock_info(sk, &evt->src_ip, &evt->dst_ip, &evt->src_port, &evt->dst_port);

    // Copy the first part of the buffer for method/path parsing in userspace
    __builtin_memcpy(evt->path, buf, MAX_HTTP_HEADER_SIZE);

    // Parse HTTP method
    if (is_request) {
        // Copy method (up to first space)
        for (int i = 0; i < 7 && buf[i] != ' '; i++) {
            evt->method[i] = buf[i];
        }
        evt->protocol = 1; // HTTP/1.x
    }

    evt->request_size = size;

    // Calculate latency for responses
    if (is_response) {
        __u64 sk_key = (__u64)sk;
        struct request_timestamp *req_ts = bpf_map_lookup_elem(&request_timestamps, &sk_key);
        if (req_ts) {
            evt->duration_ns = evt->timestamp_ns - req_ts->timestamp_ns;
            bpf_map_delete_elem(&request_timestamps, &sk_key);
        }
        // Clear active request flag
        bpf_map_delete_elem(&active_requests, &sk_key);

        // Parse status code from "HTTP/1.1 200" format
        if (read_size >= 12) {
            evt->status_code = (buf[9] - '0') * 100 + (buf[10] - '0') * 10 + (buf[11] - '0');
        }
    }

    bpf_ringbuf_submit(evt, 0);
    return 0;
}
