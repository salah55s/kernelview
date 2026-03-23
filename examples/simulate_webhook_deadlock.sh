#!/bin/bash
# KernelView Example: Simulate Webhook Timeout Deadlock (CTL-003)

set -e

echo "🟢 KernelView CTL-003 Webhook Timeout Deadlock Simulator"
echo ""
echo "This script simulates the most dangerous control plane failure:"
echo "A ValidatingWebhookConfiguration pointing to a pod that doesn't"
echo "exist yet. New pods can't be created because the webhook can't"
echo "validate them, and the webhook can't start because it needs to"
echo "be created as a pod. This is a P0 circular deadlock."
echo ""
echo "⚠️  WARNING: This will BLOCK ALL pod creation in the namespace."
echo "   Only run on a disposable test cluster!"
echo ""

kubectl create ns kv-webhook-test || true

# Deploy a webhook that points to a non-existent service
cat <<EOF | kubectl apply -f -
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: kernelview-webhook-deadlock-demo
webhooks:
- name: deadlock.demo.kernelview.io
  admissionReviewVersions: ["v1"]
  sideEffects: None
  failurePolicy: Fail          # <-- This is what causes the deadlock
  timeoutSeconds: 5
  clientConfig:
    service:
      name: nonexistent-webhook-svc
      namespace: kv-webhook-test
      path: "/validate"
  rules:
  - apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["pods"]
    operations: ["CREATE"]
  namespaceSelector:
    matchLabels:
      kernelview-webhook-test: "true"
EOF

# Label the namespace so the webhook only applies there
kubectl label ns kv-webhook-test kernelview-webhook-test=true --overwrite

echo ""
echo "✅ Webhook deadlock configured in 'kv-webhook-test'."
echo "Try creating a pod to confirm the deadlock:"
echo "  kubectl run test --image=nginx -n kv-webhook-test"
echo ""
echo "KernelView will detect:"
echo "  - CTL-003 (P0): Webhook timeout cascade"
echo "  - CREATE/UPDATE failing while GET succeeds (asymmetry signature)"
echo "  - AI analysis will identify the circular deadlock"
echo ""
echo "To clean up:"
echo "  kubectl delete validatingwebhookconfiguration kernelview-webhook-deadlock-demo"
echo "  kubectl delete ns kv-webhook-test"
