# Troubleshooting

## Agent Issues

### Agent pods are CrashLoopBackOff

**Symptom:** Agent DaemonSet pods restart repeatedly.

**Most likely cause:** Kernel version too old for eBPF features.

```bash
# Check kernel version (must be 5.8+)
kubectl exec -n kernelview daemonset/kernelview-agent -- uname -r

# Check agent logs for specific errors
kubectl logs -n kernelview daemonset/kernelview-agent --tail=50
```

**Common errors:**
| Error | Cause | Fix |
|-------|-------|-----|
| `BPF program type not supported` | Kernel < 5.8 | Upgrade kernel or disable specific programs in Helm values |
| `permission denied` | Missing capabilities | Ensure `CAP_BPF`, `CAP_PERFMON`, `CAP_NET_ADMIN` are granted |
| `failed to load BTF` | BTF not enabled | Rebuild kernel with `CONFIG_DEBUG_INFO_BTF=y` |

### Agent shows high CPU usage

**Expected:** < 2% of one core. If higher:

1. Check sampling rate: `kubectl get cm kernelview-agent -n kernelview -o yaml`
2. Reduce to `samplingRate: 0.5` in Helm values
3. Disable high-frequency programs: `syscallRate: false` in values

### No events reaching the Collector

```bash
# Verify agent → collector connectivity
kubectl exec -n kernelview daemonset/kernelview-agent -- \
  nc -zv kernelview-collector.kernelview.svc 4317

# Check gRPC stream status
kubectl logs -n kernelview deploy/kernelview-collector --tail=20 | grep "stream"
```

## Collector Issues

### BadgerDB out of disk space

```bash
# Check PVC usage
kubectl exec -n kernelview deploy/kernelview-collector -- df -h /data/badger

# Force garbage collection
kubectl exec -n kernelview deploy/kernelview-collector -- \
  kill -USR1 1  # Triggers manual GC
```

**Fix:** Increase `collector.badger.storageSize` in Helm values and redeploy.

### VictoriaMetrics remote_write failures

```bash
# Check collector logs for write errors
kubectl logs -n kernelview deploy/kernelview-collector | grep "victoria"

# Verify endpoint
kubectl exec -n kernelview deploy/kernelview-collector -- \
  curl -s http://vminsert:8480/health
```

### High memory usage on Collector

The Collector buffers events in memory before flushing. If memory is consistently > 1.5GB:

1. Increase replicas: `collector.replicas: 5`
2. Reduce trace retention: `collector.badger.retentionHours: 24`

## Dashboard Issues

### Dashboard shows "Disconnected"

The dashboard connects to the Collector REST API. Check:

```bash
# Verify the API is responding
kubectl exec -n kernelview deploy/kernelview-dashboard -- \
  curl -s http://kernelview-collector:8080/api/v1/services

# Check for CORS issues in browser console
# If accessing via Ingress, ensure the API URL matches
```

### Service Map is empty

Services appear only after the Agent captures traffic. Verify:

1. Agent pods are running: `kubectl get ds -n kernelview`
2. At least one HTTP/gRPC call has been made between services
3. Wait 30 seconds for the map to populate

## eBPF Issues

### Specific eBPF program fails to load

```bash
# Check which programs loaded successfully
kubectl logs -n kernelview daemonset/kernelview-agent | grep "loaded\|failed"
```

**Disable failing programs** in Helm values without affecting others:

```yaml
agent:
  programs:
    grpcTrace: false  # Disable if uprobe fails on this kernel
```

### XDP program conflicts

If another tool (Cilium, Calico with eBPF) is using XDP on the same interface:

```bash
# Check existing XDP programs
kubectl exec -n kernelview daemonset/kernelview-agent -- \
  ip link show dev eth0 | grep xdp
```

**Fix:** Disable KernelView's XDP program: `agent.programs.netPolicy: false`

## Enterprise Issues

### LLM API errors

```bash
# Check correlator logs
kubectl logs -n kernelview deploy/kernelview-correlator | grep "error\|timeout"
```

| Error | Cause | Fix |
|-------|-------|-----|
| `401 Unauthorized` | Invalid API key | Update the secret with a valid key |
| `429 Too Many Requests` | Rate limited | Circuit breaker will auto-recover in 5 min |
| `timeout` | Provider slow or unreachable | Circuit breaker falls back to next provider |

### Remediation actions not executing

Check the Safety Engine logs:

```bash
kubectl logs -n kernelview deploy/kernelview-operator | grep "safety\|blocked"
```

Common reasons for blocked actions:
- Outside business hours (configurable in Helm values)
- Change freeze active
- Maximum concurrent remediations reached
- Blast radius check failed (> 3 pods affected)

## Getting Help

- GitHub Issues: [github.com/salah55s/kernelview/issues](https://github.com/salah55s/kernelview/issues)
- Email: salah.nb.03@gmail.com
