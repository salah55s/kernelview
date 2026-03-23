// SPDX-License-Identifier: Apache-2.0
// KernelView — XDP Network Policy Enforcement
//
// XDP program for dynamic network policy enforcement
// without iptables overhead.

#include "headers/maps.h"
#include "headers/helpers.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

// Network policy rule
struct net_rule {
    __u32 src_ip;
    __u32 dst_ip;
    __u16 dst_port;
    __u8  action;  // 0=DROP, 1=PASS
    __u8  proto;   // 6=TCP, 17=UDP
};

// Policy map: hash of (src_ip, dst_ip, port) → rule
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, struct net_rule);
    __type(value, __u8); // action: 0=drop, 1=pass
} net_policy_rules SEC(".maps");

// Stats
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 2); // 0=passed, 1=dropped
    __type(key, __u32);
    __type(value, __u64);
} net_policy_stats SEC(".maps");

SEC("xdp")
int xdp_net_policy(struct xdp_md *ctx) {
    void *data = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    // Parse Ethernet header
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_PASS;

    // Only handle IPv4
    if (eth->h_proto != __constant_htons(0x0800))
        return XDP_PASS;

    // Parse IP header
    struct iphdr *ip = (void *)(eth + 1);
    if ((void *)(ip + 1) > data_end)
        return XDP_PASS;

    __u16 dst_port = 0;
    __u8 proto = ip->protocol;

    // Parse TCP/UDP for port info
    if (proto == 6) { // TCP
        struct tcphdr *tcp = (void *)ip + (ip->ihl * 4);
        if ((void *)(tcp + 1) > data_end)
            return XDP_PASS;
        dst_port = __builtin_bswap16(tcp->dest);
    } else if (proto == 17) { // UDP
        struct udphdr *udp = (void *)ip + (ip->ihl * 4);
        if ((void *)(udp + 1) > data_end)
            return XDP_PASS;
        dst_port = __builtin_bswap16(udp->dest);
    } else {
        return XDP_PASS; // Let non-TCP/UDP through
    }

    // Look up policy rule
    struct net_rule rule = {
        .src_ip = ip->saddr,
        .dst_ip = ip->daddr,
        .dst_port = dst_port,
        .proto = proto,
    };

    __u8 *action = bpf_map_lookup_elem(&net_policy_rules, &rule);
    if (action && *action == 0) {
        // DROP — increment stats
        __u32 key = 1;
        __u64 *drop_count = bpf_map_lookup_elem(&net_policy_stats, &key);
        if (drop_count)
            __sync_fetch_and_add(drop_count, 1);
        return XDP_DROP;
    }

    // PASS — increment stats
    __u32 key = 0;
    __u64 *pass_count = bpf_map_lookup_elem(&net_policy_stats, &key);
    if (pass_count)
        __sync_fetch_and_add(pass_count, 1);

    return XDP_PASS;
}
