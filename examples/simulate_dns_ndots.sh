#!/bin/bash
# KernelView Example: Simulate DNS ndots:5 Latency Tax (NET-001)

set -e

echo "🟢 KernelView NET-001 DNS ndots:5 Latency Tax Simulator"
echo ""
echo "This script simulates the Kubernetes ndots:5 DNS amplification issue."
echo "It makes continuous external API queries."
echo "KernelView's bpf/dns_trace.c will capture all the failing NXDOMAIN queries."
echo ""

kubectl create ns kv-simulation || true

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: dns-ndots-pod
  namespace: kv-simulation
  labels:
    app: dns-demo
spec:
  containers:
  - name: curl
    image: curlimages/curl
    command: ["sh", "-c"]
    # Continuously curl an external domain.
    # Because of standard k8s ndots:5, resolving api.stripe.com will first query:
    # 1. api.stripe.com.kv-simulation.svc.cluster.local (NXDOMAIN)
    # 2. api.stripe.com.svc.cluster.local (NXDOMAIN)
    # 3. api.stripe.com.cluster.local (NXDOMAIN)
    # 4. api.stripe.com (SUCCESS)
    args:
    - |
      while true; do 
        curl -s https://api.stripe.com/healthcheck > /dev/null || true
        # Also query some non-existent subdomains to generate extra NXDOMAINs
        nslookup random-fake-service.local > /dev/null 2>&1 || true
        sleep 0.5
      done
EOF

echo ""
echo "✅ Pod 'dns-ndots-pod' created in namespace 'kv-simulation'."
echo "KernelView eBPF DNS tracer will now record the NXDOMAIN / total query ratio."
echo "Once the ratio crosses the 30% threshold, it will trigger a NET-001 incident."
echo ""
echo "To clean up: kubectl delete ns kv-simulation"
