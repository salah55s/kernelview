#!/bin/bash
# KernelView Example: Simulate OOM Kill Cascade (MEM-001 / MEM-002)

set -e

echo "🟢 KernelView MEM-001/002 OOM Kill Cascade Simulator"
echo ""
echo "This script creates 3 pods that compete for memory on the same node,"
echo "triggering a cgroup OOM kill cascade. KernelView's oom_watch.c will"
echo "capture the kill context and the classifier will distinguish between"
echo "kernel OOM (MEM-001) and cgroup OOM (MEM-002)."
echo ""

kubectl create ns kv-simulation || true

for i in 1 2 3; do
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: oom-victim-$i
  namespace: kv-simulation
  labels:
    app: oom-cascade-demo
    victim-order: "$i"
spec:
  containers:
  - name: stress
    image: polinux/stress
    command: ["stress"]
    # Allocate memory rapidly until cgroup limit kills the container
    args: ["--vm", "1", "--vm-bytes", "200M", "--vm-hang", "0"]
    resources:
      limits:
        memory: "128Mi"   # Limit is BELOW what stress will allocate
        cpu: "100m"
      requests:
        memory: "128Mi"
        cpu: "100m"
EOF
done

echo ""
echo "✅ 3 pods created in namespace 'kv-simulation', each requesting 200MB"
echo "   but limited to 128MB. They WILL be OOM killed."
echo ""
echo "KernelView will detect:"
echo "  1. MEM-002 (cgroup OOM) for each pod individually"
echo "  2. Escalation engine cascade detection if >3 services alert in 60s"
echo ""
echo "To clean up: kubectl delete ns kv-simulation"
