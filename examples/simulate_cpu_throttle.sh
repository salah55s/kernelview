#!/bin/bash
# KernelView Example: Simulate CPU CFS Throttling (CPU-001)

set -e

echo "🟢 KernelView CPU-001 CFS Throttling Simulator"
echo ""
echo "This script simulates the 'Silent P99 Killer' pattern."
echo "It creates a pod with a very tight CPU limit but low average utilization."
echo "KernelView will detect this via the bpf/cfs_throttle.c program."
echo ""

# Create a namespace
kubectl create ns kv-simulation || true

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: cpu-throttled-pod
  namespace: kv-simulation
  labels:
    app: throttled-demo
spec:
  containers:
  - name: sysbench
    image: severalnines/sysbench
    command: ["sh", "-c"]
    # Run a CPU intensive task for 10ms, then sleep for 100ms.
    # This creates a bursty behavior that gets throttled by CFS quota,
    # despite the very low average CPU usage.
    args:
    - |
      while true; do 
        sysbench cpu --cpu-max-prime=2000 --time=1 run > /dev/null 2>&1
        sleep 0.1
      done
    resources:
      limits:
        cpu: "50m" # 0.05 CPU cores — severely limited
        memory: "128Mi"
      requests:
        cpu: "50m"
        memory: "128Mi"
EOF

echo ""
echo "✅ Pod 'cpu-throttled-pod' created in namespace 'kv-simulation'."
echo "Wait ~30 seconds for KernelView to detect the CFS throttle events."
echo "You will see a CPU-001 incident in the Dashboard."
echo ""
echo "To clean up: kubectl delete ns kv-simulation"
