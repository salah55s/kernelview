# eBPF Agent Design

## Overview

The KernelView Agent is a Go binary deployed as a **DaemonSet** (one per node). It loads eBPF programs into the kernel, consumes events from ring buffers, enriches them with Kubernetes metadata, and streams them to the Collector over gRPC.

## Agent Lifecycle

```
1. Start
2. Load eBPF programs (bpf/loader.go)
3. Start Kubernetes informer (metadata/resolver.go)
4. Open ring buffer readers for each program
5. Consume events in parallel goroutines
6. Enrich: cgroup ID → container ID → pod → service
7. Batch events (100ms or 1000 events)
8. Stream to Collector via gRPC
9. On shutdown: detach eBPF programs, drain buffers
```

## eBPF Programs

### 10 Active Programs

| Program | Type | Hook Point | Ring Buffer Size |
|---------|------|-----------|-----------------|
| `http_trace.c` | kprobe | `tcp_sendmsg` | 8MB |
| `grpc_trace.c` | uprobe | `SSL_read`, `SSL_write` | 8MB |
| `tcp_events.c` | tracepoint | `inet_sock_set_state` | 4MB |
| `syscall_rate.c` | tracepoint | `raw_syscalls:sys_enter` | 4MB |
| `oom_watch.c` | kprobe | `oom_kill_process` | 2MB |
| `exec_watch.c` | tracepoint | `sched_process_exec` | 4MB |
| `net_policy.c` | XDP | NIC ingress | 4MB |
| `dns_trace.c` | kprobe | `udp_sendmsg`, `udp_recvmsg` | 16MB |
| `cfs_throttle.c` | tracepoint | `sched_stat_runtime`, `sched_stat_blocked` | 4MB |
| `mem_rss_rate.c` | tracepoint | `cgroup_attach_task` | 4MB |

### Program Loading

Programs are compiled with `clang -target bpf` and loaded via `cilium/ebpf` Go library:

```go
// internal/agent/bpf/loader.go
spec, _ := ebpf.LoadCollectionSpec("bpf/http_trace.o")
coll, _ := ebpf.NewCollection(spec)
reader, _ := ringbuf.NewReader(coll.Maps["events"])
```

The loader verifies kernel version compatibility before loading each program and falls back gracefully if a specific feature (e.g., XDP) is unavailable.

## Cgroup-to-Pod Resolution

eBPF events contain `cgroup_id` (from `bpf_get_current_cgroup_id()`), not pod names. The resolution chain:

```
cgroup_id → /proc/<pid>/cgroup → container_id
container_id → K8s API informer → Pod{name, namespace, labels}
Pod labels → service name (from app.kubernetes.io/name)
```

This lookup is cached in a `sync.Map` with a 60-second TTL to avoid hammering the API server.

## Security Requirements

The agent runs with **three specific Linux capabilities**, NOT `--privileged`:

```yaml
securityContext:
  capabilities:
    add:
      - BPF           # Load and manage eBPF programs
      - PERFMON        # Access perf events and ring buffers
      - NET_ADMIN      # Attach XDP programs to interfaces
    drop:
      - ALL
```

This follows the principle of least privilege — the agent can observe but cannot modify the kernel, file system, or network stack.

## Resource Limits

```yaml
resources:
  requests:
    cpu: "100m"
    memory: "128Mi"
  limits:
    cpu: "500m"     # Burst for initial BPF load
    memory: "200Mi" # Includes all BPF map memory
```

The agent monitors its own RSS and will reduce sampling frequency if it approaches the memory limit.
