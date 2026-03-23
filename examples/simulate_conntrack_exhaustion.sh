#!/bin/bash
# KernelView Example: Simulate Conntrack Exhaustion (NET-007)

set -e

echo "🟢 KernelView NET-007 Conntrack Exhaustion Simulator"
echo ""
echo "This script creates a pod that opens thousands of TCP connections"
echo "per second to exhaust the node's conntrack table."
echo "When nf_conntrack exceeds 90% of max, new connections are silently"
echo "dropped at the kernel level — invisible to application-level monitoring."
echo ""
echo "KernelView detects this via eBPF conntrack monitoring and classifies"
echo "it as NET-007 (Conntrack Exhaustion), a node-level P1."
echo ""

kubectl create ns kv-simulation || true

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: conntrack-flood
  namespace: kv-simulation
  labels:
    app: conntrack-demo
spec:
  containers:
  - name: flood
    image: busybox
    command: ["sh", "-c"]
    # Open many short-lived connections to exhaust conntrack
    args:
    - |
      while true; do 
        for i in \$(seq 1 50); do
          (echo -n "" | nc -w 1 1.1.1.1 80 &) 2>/dev/null
        done
        sleep 0.1
      done
    resources:
      limits:
        memory: "128Mi"
        cpu: "200m"
EOF

echo ""
echo "✅ Pod 'conntrack-flood' created. It opens ~500 connections/second."
echo ""
echo "KernelView will detect:"
echo "  - NET-007: conntrack table approaching limit"
echo "  - Affects ALL pods on the node, not just this one"
echo "  - Recommends sysctl nf_conntrack_max increase via DaemonSet"
echo ""
echo "To clean up: kubectl delete ns kv-simulation"
