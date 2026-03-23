# KernelView Examples

This directory contains scripts to artificially generate the complex Kubernetes failure modes that KernelView is designed to detect and remediate. 

You can use these scripts on a test cluster to see KernelView's eBPF agents, Incident Classifier, and LLM Correlator in action.

## 1. CPU CFS Throttling (The "Silent P99 Killer")
**Script:** [`simulate_cpu_throttle.sh`](./simulate_cpu_throttle.sh)

**What it does:** Deploys a pod with an extremely tight CPU limit (`50m`) that runs a bursty `sysbench` workload. The average CPU utilization remains low (< 5%), but the container is constantly throttled by the Linux Completely Fair Scheduler (CFS), destroying tail latency. 

**KernelView's Response:** Detects the `CPU-001` signature (throttle ratio > 25% with severe latency impact) via `cfs_throttle.c` and flags the incorrect limit configuration.

## 2. DNS ndots:5 Amplification
**Script:** [`simulate_dns_ndots.sh`](./simulate_dns_ndots.sh)

**What it does:** Deploys a pod making continuous external API calls (e.g., `api.stripe.com`). Because of standard Kubernetes `ndots:5` configuration, each call triggers 3 failing `NXDOMAIN` queries against the internal CoreDNS before resolving the external domain, adding a significant latency tax.

**KernelView's Response:** Detects the `NET-001` signature by hooking `udp_sendmsg`/`udp_recvmsg` on port 53 via `dns_trace.c`, tracks the high `NXDOMAIN` ratio per cgroup, and automatically recommends the `ndots:2` `dnsConfig` patch.

## Running Examples
```bash
# Ensure KernelView is deployed first
kubectl get pods -n kernelview

# Run a simulation
./simulate_cpu_throttle.sh

# Open the dashboard and watch the incident appear
kubectl port-forward svc/kernelview-dashboard 8080:8080 -n kernelview
```
