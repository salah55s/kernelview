# Collector & Anomaly Detection

## Overview

The Collector is the central data plane of KernelView. It receives enriched events from all Agents via gRPC, stores them, runs anomaly detection, and serves the Dashboard REST API.

Deployed as a **Deployment with 3 replicas** for high availability.

## Ingest Pipeline

```
Agent gRPC streams ──→ OTLP Receiver (port 4317)
                              │
                ┌─────────────┼──────────────┐
                v             v              v
          BadgerDB     VictoriaMetrics   Anomaly Engine
        (72hr traces)  (30-day metrics)  (real-time)
                                              │
                                              v
                                         Classifier
                                        (32 types)
```

### OTLP Receiver (`internal/collector/ingest/otlp.go`)

Accepts OpenTelemetry-compatible gRPC streams on port 4317. Each `StreamEvents` call is a long-lived bidirectional stream from one Agent. Events are unmarshalled from protobuf and dispatched to three parallel consumers.

### Storage: BadgerDB (`internal/collector/store/badger.go`)

Raw trace data is stored in BadgerDB with a 72-hour TTL. This allows the Dashboard to display individual traces and the AI Correlator to fetch context for root cause analysis.

Key schema: `{service}/{timestamp_ns}/{trace_id}`

### Storage: VictoriaMetrics (`internal/collector/store/victoriametrics.go`)

Aggregated metrics are pushed to VictoriaMetrics via the Prometheus `remote_write` protocol. Downsampled to 10-second intervals. Retained for 30 days.

Metrics pushed:
- `kernelview_http_request_duration_seconds` (histogram)
- `kernelview_tcp_connections_total` (counter)
- `kernelview_syscall_rate_per_second` (gauge)
- `kernelview_memory_rss_bytes` (gauge)
- `kernelview_dns_nxdomain_rate` (gauge)
- `kernelview_cfs_throttle_ratio` (gauge)

## Anomaly Detection Engine (`internal/collector/anomaly/engine.go`)

The engine runs **four detector types** in parallel:

### 1. Latency Spike Detector
- Sliding window: 5 minutes, 10-second buckets
- Alert when: p99 > 3× p50 for >= 30 seconds
- Maps to: CPU-001 (CFS throttling)

### 2. Error Rate Detector
- Sliding window: 2 minutes
- Alert when: error_rate > 5% AND error_rate > 2× baseline
- Maps to: APP family incidents

### 3. Traffic Pattern Detector
- Sliding window: 10 minutes
- Alert when: RPS drops > 50% in < 1 minute (upstream failure)
- Alert when: RPS spikes > 5× baseline (DDoS or retry storm)

### 4. Noisy Neighbor Detector
- Sliding window: 1 minute per cgroup
- Alert when: syscall_rate > mean + 3σ for >= 30 seconds
- Maps to: CPU-002

### Statistical Method

All detectors use the **Modified Z-Score** with MAD (Median Absolute Deviation) instead of standard deviation to be robust to outliers:

```
modified_z = 0.6745 × (x - median) / MAD
anomaly if |modified_z| > 3.5
```

## Incident Classifier (`internal/collector/classifier/`)

When the anomaly engine fires, signals are passed to the **Incident Classifier**. The classifier is a rule-based decision tree that runs **before** any LLM invocation.

### Why classify before the LLM?

1. **Select the right LLM provider** — APP incidents → GPT-4o (code analysis), network incidents → Gemini (structured JSON)
2. **Select the right prompt** — each of the 32 incident types has a dedicated prompt template with known patterns
3. **Select the right escalation tier** — P0 gets PagerDuty, P3 goes to weekly email digest
4. **Speed** — classification takes < 1ms, LLM takes 2-8 seconds

### Classification Priority Order

```
Control Plane (CTL) → checked first: affects everything
    ↓
Memory (MEM) → checked second: OOM kills are urgent
    ↓
Application (APP) → checked third: CrashLoopBackOff
    ↓
Network (NET) → checked fourth: DNS, MTU, conntrack
    ↓
CPU → checked last: throttling is less urgent
```

### The 7 Families and 32 Types

| Family | Types | Examples |
|--------|-------|---------|
| MEM | 6 | OOM kill (kernel), OOM kill (cgroup), memory leak, growth spike, node pressure, JVM heap |
| CPU | 6 | CFS throttle, noisy neighbor, node saturation, sched starvation, spin-lock, limit too tight |
| NET | 7 | DNS ndots, CoreDNS overload, MTU mismatch, connection flood, TCP retransmits, endpoint churn, conntrack exhaustion |
| APP | 8 | Auth failure, OOM crash, config missing, readiness probe, port conflict, dependency unavailable, image pull, init container |
| CTL | 5 | etcd I/O, API server overload, webhook timeout, controller backlog, scheduler pending |
| STO | — | Reserved for future storage incidents |
| SEC | — | Reserved for future security incidents |

## REST API

The Collector exposes a REST API on port 8080 for the Dashboard:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/services` | GET | List all discovered services with health status |
| `/api/v1/services/{name}` | GET | Service detail: latency, error rate, throughput |
| `/api/v1/services/{name}/traces` | GET | Recent traces for a service |
| `/api/v1/traces/{id}` | GET | Individual trace detail |
| `/api/v1/incidents` | GET | Active and recent incidents |
| `/api/v1/incidents/{id}` | GET | Incident detail with timeline |
| `/api/v1/rightsizing` | GET | CPU/memory right-sizing recommendations |
