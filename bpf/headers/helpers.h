// SPDX-License-Identifier: Apache-2.0
// KernelView — Shared BPF Helper Functions
#ifndef __KERNELVIEW_HELPERS_H
#define __KERNELVIEW_HELPERS_H

#include "maps.h"

// ============================================================
// HTTP Method Detection
// ============================================================

// Check if the buffer starts with an HTTP method.
// Returns 1 if HTTP detected, 0 otherwise.
static __always_inline int is_http_request(const char *buf, int len) {
    if (len < 4)
        return 0;

    // GET
    if (buf[0] == 'G' && buf[1] == 'E' && buf[2] == 'T' && buf[3] == ' ')
        return 1;
    // POST
    if (len >= 5 && buf[0] == 'P' && buf[1] == 'O' && buf[2] == 'S' && buf[3] == 'T' && buf[4] == ' ')
        return 1;
    // PUT
    if (buf[0] == 'P' && buf[1] == 'U' && buf[2] == 'T' && buf[3] == ' ')
        return 1;
    // DELETE
    if (len >= 7 && buf[0] == 'D' && buf[1] == 'E' && buf[2] == 'L' && buf[3] == 'E' &&
        buf[4] == 'T' && buf[5] == 'E' && buf[6] == ' ')
        return 1;
    // PATCH
    if (len >= 6 && buf[0] == 'P' && buf[1] == 'A' && buf[2] == 'T' && buf[3] == 'C' &&
        buf[4] == 'H' && buf[5] == ' ')
        return 1;
    // HEAD
    if (len >= 5 && buf[0] == 'H' && buf[1] == 'E' && buf[2] == 'A' && buf[3] == 'D' && buf[4] == ' ')
        return 1;
    // OPTIONS
    if (len >= 8 && buf[0] == 'O' && buf[1] == 'P' && buf[2] == 'T' && buf[3] == 'I' &&
        buf[4] == 'O' && buf[5] == 'N' && buf[6] == 'S' && buf[7] == ' ')
        return 1;

    return 0;
}

// Check if the buffer starts with an HTTP response.
static __always_inline int is_http_response(const char *buf, int len) {
    if (len < 5)
        return 0;
    // HTTP/
    return (buf[0] == 'H' && buf[1] == 'T' && buf[2] == 'T' && buf[3] == 'P' && buf[4] == '/');
}

// ============================================================
// Sampling
// ============================================================

// Returns 1 if this event should be captured, 0 if it should be skipped.
static __always_inline int should_sample(void) {
    __u32 key = 0;
    struct sampling_config *cfg = bpf_map_lookup_elem(&sampling, &key);
    if (!cfg || !cfg->enabled)
        return 1; // Sampling disabled → capture everything

    __u32 counter = __sync_fetch_and_add(&cfg->counter, 1);
    return (counter % cfg->sample_rate) == 0;
}

// ============================================================
// Socket Helpers
// ============================================================

// Extract IPv4 addresses and ports from a sock struct.
static __always_inline void extract_sock_info(struct sock *sk,
                                               __u32 *src_ip, __u32 *dst_ip,
                                               __u16 *src_port, __u16 *dst_port) {
    BPF_CORE_READ_INTO(src_ip, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(dst_ip, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(src_port, sk, __sk_common.skc_num);
    __u16 dport;
    BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
    *dst_port = __builtin_bswap16(dport);
}

// ============================================================
// Cgroup ID Helper
// ============================================================

// Get the cgroup ID for the current task.
static __always_inline __u64 get_cgroup_id(void) {
    return bpf_get_current_cgroup_id();
}

#endif // __KERNELVIEW_HELPERS_H
