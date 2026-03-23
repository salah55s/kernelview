# Helm Values Reference

## Installation

```bash
helm install kernelview deploy/helm/kernelview/ \
  --namespace kernelview \
  --create-namespace \
  -f my-values.yaml
```

## Full Values Reference

### Agent

```yaml
agent:
  # Controls the eBPF agent DaemonSet
  enabled: true

  image:
    repository: ghcr.io/kernelview/agent
    tag: latest
    pullPolicy: IfNotPresent

  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "500m"
      memory: "200Mi"

  # Node selector for agent DaemonSet
  nodeSelector: {}

  # Tolerations (e.g., to run on control plane nodes)
  tolerations:
    - key: node-role.kubernetes.io/control-plane
      operator: Exists
      effect: NoSchedule

  # Which eBPF programs to load (disable for incompatible kernels)
  programs:
    httpTrace: true
    grpcTrace: true
    tcpEvents: true
    syscallRate: true
    oomWatch: true
    execWatch: true
    netPolicy: true
    dnsTrace: true
    cfsThrottle: true
    memRssRate: true

  # Sampling rate (1.0 = capture everything, 0.1 = 10% sample)
  samplingRate: 1.0
```

### Collector

```yaml
collector:
  enabled: true
  replicas: 3

  image:
    repository: ghcr.io/kernelview/collector
    tag: latest

  resources:
    requests:
      cpu: "500m"
      memory: "512Mi"
    limits:
      cpu: "2000m"
      memory: "2Gi"

  # gRPC ingest port (agents connect here)
  grpcPort: 4317

  # REST API port (dashboard connects here)
  apiPort: 8080

  # BadgerDB configuration
  badger:
    # Path for trace storage
    dataDir: /data/badger
    # Retention period for raw traces
    retentionHours: 72
    # PVC size
    storageSize: 50Gi
    storageClass: ""  # Uses default StorageClass

  # VictoriaMetrics remote_write target
  victoriaMetrics:
    enabled: true
    url: "http://vminsert:8480/insert/0/prometheus/api/v1/write"
    # Retention period for aggregated metrics
    retentionDays: 30
```

### Dashboard

```yaml
dashboard:
  enabled: true
  replicas: 1

  image:
    repository: ghcr.io/kernelview/dashboard
    tag: latest

  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "500m"
      memory: "256Mi"

  # Service type for the dashboard
  service:
    type: ClusterIP
    port: 8080

  # Ingress configuration
  ingress:
    enabled: false
    className: nginx
    annotations: {}
    hosts:
      - host: kernelview.example.com
        paths:
          - path: /
            pathType: Prefix
    tls: []
```

### Enterprise Components

```yaml
# AI Correlator (Enterprise license required)
correlator:
  enabled: false

  image:
    repository: ghcr.io/kernelview/correlator
    tag: latest

  # LLM Provider Configuration
  llm:
    # BYOC mode: route all data to local Ollama (never leaves cluster)
    byocMode: false

    # Provider API keys (use Kubernetes secrets in production)
    anthropicApiKey: ""
    googleApiKey: ""
    openaiApiKey: ""

    # Ollama endpoint (for BYOC mode)
    ollamaEndpoint: "http://ollama.kernelview.svc:11434"

# Remediation Operator (Enterprise license required)
operator:
  enabled: false

  image:
    repository: ghcr.io/kernelview/operator
    tag: latest

  # Safety controls
  safety:
    # Maximum concurrent remediations
    maxConcurrent: 3
    # Require approval for P0 actions
    requireApprovalP0: false
    # Business hours (UTC) - block risky actions outside these hours
    businessHoursStart: "08:00"
    businessHoursEnd: "18:00"
    # Change freeze dates (ISO 8601)
    changeFreezes: []
```

### Global Settings

```yaml
# Global namespace for all components
namespace: kernelview

# Image pull secrets
imagePullSecrets: []

# Service account
serviceAccount:
  create: true
  name: kernelview
  annotations: {}

# Pod security context
podSecurityContext:
  runAsNonRoot: true
  seccompProfile:
    type: RuntimeDefault

# RBAC
rbac:
  create: true
```
