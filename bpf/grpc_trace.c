// SPDX-License-Identifier: Apache-2.0
// KernelView — TLS/gRPC Capture via uprobes
//
// Hooks SSL_read/SSL_write in libssl.so to capture TLS-encrypted
// traffic (including gRPC) after decryption. Also supports Go
// crypto/tls via tls.(*Conn).Read/Write hooks.

#include "headers/maps.h"
#include "headers/helpers.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// ============================================================
// SSL_write — Capture outgoing encrypted data after encryption
// ============================================================

SEC("uprobe/SSL_write")
int uprobe_ssl_write(struct pt_regs *ctx) {
    if (!should_sample())
        return 0;

    // SSL_write(SSL *ssl, const void *buf, int num)
    const void *buf = (const void *)PT_REGS_PARM2(ctx);
    int num = (int)PT_REGS_PARM3(ctx);

    if (num <= 0 || !buf)
        return 0;

    // Read the first bytes to check for HTTP/gRPC
    char header[MAX_HTTP_HEADER_SIZE];
    __builtin_memset(header, 0, sizeof(header));

    int read_size = num < MAX_HTTP_HEADER_SIZE ? num : MAX_HTTP_HEADER_SIZE;
    if (bpf_probe_read_user(header, read_size & 0xFF, buf) < 0)
        return 0;

    // Check for HTTP or gRPC (HTTP/2)
    if (!is_http_request(header, read_size) && !is_http_response(header, read_size))
        return 0;

    struct http_event *evt = bpf_ringbuf_reserve(&events, sizeof(*evt), 0);
    if (!evt)
        return 0;

    __builtin_memset(evt, 0, sizeof(*evt));
    evt->timestamp_ns = bpf_ktime_get_ns();
    evt->pid = bpf_get_current_pid_tgid() >> 32;
    evt->cgroup_id = get_cgroup_id();
    evt->protocol = 2; // Mark as coming from TLS layer
    evt->request_size = num;

    __builtin_memcpy(evt->path, header, MAX_HTTP_HEADER_SIZE);

    if (is_http_request(header, read_size)) {
        evt->is_response = 0;
        for (int i = 0; i < 7 && header[i] != ' '; i++) {
            evt->method[i] = header[i];
        }
    } else {
        evt->is_response = 1;
        if (read_size >= 12) {
            evt->status_code = (header[9] - '0') * 100 + (header[10] - '0') * 10 + (header[11] - '0');
        }
    }

    bpf_ringbuf_submit(evt, 0);
    return 0;
}

// ============================================================
// SSL_read — Capture incoming encrypted data after decryption
// ============================================================

SEC("uprobe/SSL_read")
int uprobe_ssl_read(struct pt_regs *ctx) {
    if (!should_sample())
        return 0;

    // SSL_read(SSL *ssl, void *buf, int num)
    // We need the return probe to get actual read data
    // For now, record the timestamp for latency matching
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    struct request_timestamp ts = {};
    ts.timestamp_ns = bpf_ktime_get_ns();
    ts.pid = pid;
    ts.cgroup_id = get_cgroup_id();

    // Use PID as key (simplified — in production, use SSL* pointer)
    __u64 key = pid_tgid;
    bpf_map_update_elem(&request_timestamps, &key, &ts, BPF_ANY);

    return 0;
}
