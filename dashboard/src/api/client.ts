const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export interface ServiceMapNode {
  service: {
    name: string;
    namespace: string;
    podCount: number;
  };
  requestRate: number;
  errorRate: number;
  latencyP50: number;
  latencyP99: number;
  health: 'healthy' | 'degraded' | 'critical' | 'learning' | 'unknown';
  hasAnomaly: boolean;
  hasRemediation: boolean;
}

export interface ServiceMapEdge {
  source: string;
  target: string;
  requestRate: number;
  errorRate: number;
}

export interface TraceEntry {
  traceId: string;
  timestamp: string;
  service: string;
  method: string;
  path: string;
  statusCode: number;
  latencyMs: number;
  upstream?: string;
}

export interface Anomaly {
  id: string;
  type: string;
  service: string;
  namespace: string;
  pod?: string;
  node?: string;
  detectedAt: string;
  resolvedAt?: string;
  severity: 'warning' | 'critical';
  description: string;
  correlationId?: string;
}

export interface Incident {
  id: string;
  anomalies: Anomaly[];
  rootCause?: string;
  confidence?: string;
  recommendedAction?: string;
  remediationActions?: RemediationSummary[];
  createdAt: string;
  resolvedAt?: string;
  status: 'active' | 'resolved' | 'silenced';
}

export interface RemediationSummary {
  id: string;
  action: string;
  targetPod: string;
  phase: string;
  executedAt?: string;
}

export interface RightSizingRecommendation {
  deployment: string;
  namespace: string;
  resource: 'cpu' | 'memory';
  currentLimit: string;
  p99ActualUsage: string;
  recommendedLimit: string;
  monthlySaving: number;
  dataDays: number;
}

async function fetchAPI<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`);
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export const api = {
  getServiceMap: () =>
    fetchAPI<{ nodes: ServiceMapNode[]; edges: ServiceMapEdge[] }>('/api/v1/service-map'),

  getServiceMetrics: (service: string, namespace?: string, duration = '1h') =>
    fetchAPI(`/api/v1/services/${service}/metrics?namespace=${namespace || ''}&duration=${duration}`),

  getTraces: (service?: string, duration = '1h', limit = 200) =>
    fetchAPI<TraceEntry[]>(`/api/v1/traces?service=${service || ''}&duration=${duration}&limit=${limit}`),

  getAnomalies: () =>
    fetchAPI<Anomaly[]>('/api/v1/anomalies'),

  getIncidents: (service?: string, limit = 50) =>
    fetchAPI<Incident[]>(`/api/v1/incidents?service=${service || ''}&limit=${limit}`),

  getIncident: (id: string) =>
    fetchAPI<Incident>(`/api/v1/incidents/${id}`),

  getRightSizing: () =>
    fetchAPI<RightSizingRecommendation[]>('/api/v1/right-sizing'),
};
