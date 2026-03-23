# Architecture Deep Dive

## System Overview

KernelView is a **zero-instrumentation** Kubernetes observability and remediation platform. It installs as a single Helm chart and deploys four components across the cluster.

```
┌──────────────────────────────────────────────────────────────────────┐
│  Kubernetes Node                                                      │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │  Linux Kernel                                                    │  │
│  │                                                                  │  │
│  │  kprobes ──┐   tracepoints ──┐   uprobes ──┐   XDP ──┐         │  │
│  │            v                  v              v         v         │  │
│  │  ┌──────────────────────────────────────────────────────┐       │  │
│  │  │              eBPF Ring Buffers (per-program)          │       │  │
│  │  └──────────────────────┬───────────────────────────────┘       │  │
│  └─────────────────────────┼───────────────────────────────────────┘  │
│                             │                                          │
│  ┌──────────────────────────v──────────────────────────────────────┐  │
│  │  KernelView Agent (DaemonSet, 1 per node)                       │  │
│  │                                                                  │  │
│  │  BPF Loader ─→ Ring Buffer Consumer ─→ Metadata Enricher        │  │
│  │                    (cgroup → pod)         (pod → service)        │  │
│  │                                                                  │  │
│  │  Output: gRPC stream of enriched events ─────────────────────┐  │  │
│  └──────────────────────────────────────────────────────────────┼──┘  │
└─────────────────────────────────────────────────────────────────┼─────┘
                                                                  │
              ┌───────────────────────────────────────────────────v──┐
              │  KernelView Collector (Deployment, 3 replicas)       │
              │                                                      │
              │  gRPC Ingest ─→ Anomaly Engine ─→ VictoriaMetrics   │
              │       │              │                               │
              │       v              v                               │
              │  BadgerDB         Classifier                         │
              │  (72hr traces)   (32 incident types)                 │
              │                      │                               │
              │              ┌───────v────────┐                      │
              │              │  REST API       │                     │
              │              └───────┬────────┘                      │
              └──────────────────────┼──────────────────────────────┘
                                     │
              ┌──────────────────────v──────────────────────────────┐
              │  Dashboard (Deployment)                              │
              │  Vite + React + TypeScript + shadcn/ui               │
              │  Service Map │ Traces │ Incidents │ Right-Sizing     │
              └─────────────────────────────────────────────────────┘

              ┌─────────────────────────────────────────────────────┐
              │  AI Correlator (Enterprise)                          │
              │  LLM Router → Claude/Gemini/GPT-4o/Ollama           │
              │  Prompt Library → Per-incident AI analysis           │
              └──────────────────────┬──────────────────────────────┘
                                     │
              ┌──────────────────────v──────────────────────────────┐
              │  Remediation Operator (Enterprise)                   │
              │  Safety Engine → Blast Radius Check → K8s API       │
              │  Escalation Engine (10-state machine)                │
              └─────────────────────────────────────────────────────┘
```

## Data Flow

### 1. Capture (Kernel → Agent)

Every eBPF program writes events to a **BPF ring buffer**. Ring buffers are lock-free, per-CPU data structures that are the fastest way to get data from kernel to userspace.

| Program | Hook Type | What It Captures |
|---------|-----------|-----------------|
| `http_trace.c` | kprobe on `tcp_sendmsg` | HTTP/1.x request/response pairs |
| `grpc_trace.c` | uprobe on `SSL_read/SSL_write` | gRPC calls through TLS |
| `tcp_events.c` | tracepoint `sock:inet_sock_set_state` | TCP connection lifecycle |
| `syscall_rate.c` | tracepoint `raw_syscalls:sys_enter` | Per-cgroup syscall rates |
| `oom_watch.c` | kprobe on `oom_kill_process` | OOM kill events with victim context |
| `exec_watch.c` | tracepoint `sched:sched_process_exec` | Process execution for audit |
| `net_policy.c` | XDP | Network policy enforcement drops |
| `dns_trace.c` | kprobe on `udp_sendmsg/recvmsg` | DNS query/response on port 53 |
| `cfs_throttle.c` | tracepoint `sched:sched_stat_*` | CFS throttled periods per cgroup |
| `mem_rss_rate.c` | tracepoint `cgroup:cgroup_attach_task` | RSS growth velocity per cgroup |

### 2. Enrich (Agent)

Raw eBPF events contain **cgroup IDs**, not pod names. The agent enriches events by:

1. **Cgroup detection** (`internal/agent/cgroup/detect.go`): Maps cgroup ID → container ID by reading `/proc/<pid>/cgroup`.
2. **Metadata resolution** (`internal/agent/metadata/resolver.go`): Uses the Kubernetes API informer to map container ID → pod name → service name → namespace → node.

### 3. Ingest (Agent → Collector)

Enriched events are streamed over **gRPC** to the Collector using the `AgentService.StreamEvents` RPC defined in `proto/agent.proto`. Events are batched every 100ms or 1000 events to minimize gRPC overhead.

### 4. Detect (Collector)

The Collector runs three parallel pipelines:

- **Storage**: Events are written to BadgerDB (72-hour retention) and pushed to VictoriaMetrics via `remote_write`.
- **Anomaly Detection** (`internal/collector/anomaly/engine.go`): Statistical anomaly detection using sliding windows. Detects latency spikes, error rate increases, and traffic pattern changes.
- **Classification** (`internal/collector/classifier/`): Rule-based incident classifier with 7 families and 32 types. Runs **before** the LLM to select the right provider, prompt, and escalation tier.

### 5. Correlate (Enterprise — AI Correlator)

The AI Correlator receives classified incidents and:

1. Selects the LLM provider via the **3-factor Router** (data sovereignty → severity → type specialization).
2. Builds a per-incident prompt from the **Prompt Library** with truncation for the provider's context window.
3. Sends the prompt and parses the structured JSON response.
4. Returns the root cause, confidence score, and recommended actions.

### 6. Remediate (Enterprise — Operator)

The Remediation Operator executes actions through the Kubernetes API:

- CPU limit removal (CPU-001)
- Pod restart with backoff (APP-002)
- DNS config patching (NET-001)
- Conntrack sysctl increase (NET-007)

Every action passes through the **Safety Engine** which checks blast radius, business hours, and change freeze windows.

## Security Model

| Layer | Mechanism |
|-------|-----------|
| Agent privileges | `CAP_BPF`, `CAP_PERFMON`, `CAP_NET_ADMIN` — NOT `--privileged` |
| Secret scrubbing | All log content passes through `internal/correlator/scrubber/` before LLM |
| Data sovereignty | BYOC mode routes all data to local Ollama, never leaving the cluster |
| Network policy | Agent → Collector only (no external egress by default) |
| RBAC | Operator uses a scoped ServiceAccount with namespace-limited permissions |

## Performance Budget

| Metric | Target | Mechanism |
|--------|--------|-----------|
| Agent CPU | < 2% of one core per node | Per-CPU ring buffers, batch processing |
| Agent Memory | < 200MB RSS per node | BPF map size limits, event sampling |
| Event Latency | < 5 seconds kernel → dashboard | gRPC streaming, no disk queue |
| Collector Storage | 72 hours raw + 30 days aggregated | BadgerDB TTL + VictoriaMetrics downsampling |
