# KernelView Examples

This directory contains scripts that reproduce real Kubernetes failure modes on a test cluster. Each script maps to a specific KernelView incident type.

## Quick Start
```bash
# Ensure KernelView is deployed
kubectl get pods -n kernelview

# Run any example
./simulate_cpu_throttle.sh

# Open the dashboard
kubectl port-forward svc/kernelview-dashboard 8080:8080 -n kernelview
```

## Examples by Incident Family

### Memory (MEM)
| Script | Incident | What It Does |
|--------|----------|-------------|
| `simulate_oom_cascade.sh` | MEM-001/002 | 3 pods fighting for memory, all OOM killed |
| `simulate_memory_leak.sh` | MEM-003 | Python app leaking ~8MB/min until OOM |

### CPU
| Script | Incident | What It Does |
|--------|----------|-------------|
| `simulate_cpu_throttle.sh` | CPU-001 | Bursty sysbench on 50m limit — the "Silent P99 Killer" |

### Network (NET)
| Script | Incident | What It Does |
|--------|----------|-------------|
| `simulate_dns_ndots.sh` | NET-001 | Continuous external DNS calls hitting ndots:5 amplification |
| `simulate_conntrack_exhaustion.sh` | NET-007 | 500 conns/sec flooding the node conntrack table |

### Application (APP)
| Script | Incident | What It Does |
|--------|----------|-------------|
| `simulate_crashloop_auth.sh` | APP-001 | Pod crashes <2s with 401 auth failure |

### Control Plane (CTL)
| Script | Incident | What It Does |
|--------|----------|-------------|
| `simulate_webhook_deadlock.sh` | CTL-003 | ValidatingWebhook with failurePolicy:Fail → circular deadlock |

## Cleanup
```bash
kubectl delete ns kv-simulation kv-webhook-test 2>/dev/null || true
kubectl delete validatingwebhookconfiguration kernelview-webhook-deadlock-demo 2>/dev/null || true
```
