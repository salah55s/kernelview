#!/bin/bash
# KernelView Example: Simulate Memory Leak (MEM-003)

set -e

echo "🟢 KernelView MEM-003 Memory Leak Simulator"
echo ""
echo "This script creates a pod that steadily leaks memory at ~8MB/min."
echo "KernelView's mem_rss_rate.c eBPF program will track the RSS growth"
echo "rate per cgroup. After 10 consecutive minutes of growth >5MB/min,"
echo "the classifier triggers MEM-003 (Memory Leak)."
echo ""

kubectl create ns kv-simulation || true

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: memory-leaker
  namespace: kv-simulation
  labels:
    app: memory-leak-demo
spec:
  containers:
  - name: leaker
    image: python:3.12-slim
    command: ["python3", "-c"]
    args:
    - |
      import time
      leak = []
      while True:
          # Allocate ~1MB every 7 seconds ≈ 8.5 MB/min
          leak.append(bytearray(1024 * 1024))
          time.sleep(7)
    resources:
      limits:
        memory: "512Mi"
        cpu: "100m"
      requests:
        memory: "64Mi"
        cpu: "50m"
EOF

echo ""
echo "✅ Pod 'memory-leaker' created. It leaks ~8MB/min."
echo "Timeline:"
echo "  t+0:   RSS ~64MB"
echo "  t+10m: RSS ~150MB → MEM-003 fires (>5MB/min for 10 min)"
echo "  t+50m: RSS ~490MB → Approaches limit, MEM-004 may fire"
echo "  t+55m: OOM kill → MEM-002 fires"
echo ""
echo "To clean up: kubectl delete ns kv-simulation"
