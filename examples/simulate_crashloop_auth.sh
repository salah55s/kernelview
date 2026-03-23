#!/bin/bash
# KernelView Example: Simulate CrashLoopBackOff — Auth Failure (APP-001)

set -e

echo "🟢 KernelView APP-001 CrashLoopBackOff Auth Failure Simulator"
echo ""
echo "This script creates a pod that crashes within 2 seconds because"
echo "it tries to connect to a fake API with bad credentials."
echo "KernelView classifies this as APP-001 (Auth Failure) using:"
echo "  - Exit code 1"
echo "  - Startup duration < 2 seconds"
echo "  - Failed network call during startup"
echo ""

kubectl create ns kv-simulation || true

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: crashloop-auth-fail
  namespace: kv-simulation
  labels:
    app: crashloop-auth-demo
spec:
  restartPolicy: Always
  containers:
  - name: app
    image: curlimages/curl
    command: ["sh", "-c"]
    # Attempt to call an API with bad credentials, exit 1 on failure.
    # This happens within 1 second of container start.
    args:
    - |
      echo "Starting up..."
      echo "Connecting to auth service..."
      curl -sf -H "Authorization: Bearer INVALID_TOKEN" \
        https://httpstat.us/401 || exit 1
    resources:
      limits:
        memory: "64Mi"
        cpu: "50m"
EOF

echo ""
echo "✅ Pod 'crashloop-auth-fail' created. It will CrashLoopBackOff."
echo ""
echo "KernelView's classifier will detect:"
echo "  - Exit code 1"  
echo "  - Startup duration < 2 seconds"
echo "  - Failed network call with HTTP 401"
echo "  → APP-001 (Auth Failure) with confidence 0.90"
echo ""
echo "To clean up: kubectl delete ns kv-simulation"
