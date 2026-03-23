import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { AlertTriangle, CheckCircle2, Clock, Wrench, Brain, ShieldCheck, ChevronRight, ExternalLink } from 'lucide-react';
import { useNavigate, useParams } from 'react-router-dom';
import { useState } from 'react';

interface TimelineEvent {
  time: string;
  type: 'anomaly' | 'correlation' | 'safety' | 'remediation' | 'resolved';
  title: string;
  description: string;
  details?: string;
}

const demoIncidents = [
  {
    id: 'INC-2024-001',
    status: 'active' as const,
    severity: 'critical' as const,
    title: 'notification-service: Error rate spike + OOM kills',
    services: ['notification-service', 'email-worker'],
    detectedAt: '14:18:02',
    summary: 'Error rate jumped to 12.5% with p99 latency at 4.2s. Two OOM kills detected in last 5 minutes.',
    timeline: [
      { time: '14:18:02', type: 'anomaly', title: 'Error rate anomaly detected', description: 'Error rate 12.5% exceeded baseline (1.2%) + 15pp threshold' },
      { time: '14:18:05', type: 'anomaly', title: 'Latency spike detected', description: 'p99 latency 4200ms exceeds 3x baseline (120ms)' },
      { time: '14:18:08', type: 'anomaly', title: 'OOM kill detected', description: 'notification-service-7f4d8-xk2j9 killed by cgroup OOM (memory.max=256Mi)' },
      { time: '14:18:12', type: 'correlation', title: 'AI Correlator analysis complete', description: 'Root cause: Memory leak in notification template rendering. Email queue backlog causing memory pressure.', details: 'Confidence: HIGH\n\nThe notification-service is experiencing OOM kills because the email template rendering engine is leaking memory when processing bulk notification batches. The queue backlog (currently 12,000 items) is causing each pod to attempt to render too many templates simultaneously.\n\nThe upstream email-worker service is also showing degraded performance, suggesting the issue may have cascaded.' },
      { time: '14:18:15', type: 'safety', title: 'Safety rules evaluated', description: '7/8 rules passed. Blocked: single-replica pod restart without PDB.' },
      { time: '14:18:16', type: 'remediation', title: 'CPU throttle applied', description: 'CPU limit reduced to 200m (from 500m) to slow queue processing and reduce memory pressure. Auto-revert in 5 minutes.' },
    ] as TimelineEvent[],
    rootCause: 'Memory leak in notification template rendering engine. Email queue backlog (12,000 items) causing each pod to render too many templates simultaneously.',
    confidence: 'HIGH',
    safetyResults: [
      { rule: 'protected_namespace', passed: true, reason: 'workers is not protected' },
      { rule: 'single_replica_restart', passed: false, reason: 'Cannot restart — only 1 replica, no PDB' },
      { rule: 'min_cpu_throttle', passed: true, reason: 'New limit 200m is above 10% of 500m' },
      { rule: 'action_rate_limit', passed: true, reason: '0/3 actions in last hour' },
      { rule: 'isolate_requires_approval', passed: true, reason: 'Not an isolate action' },
      { rule: 'pdb_compliance', passed: true, reason: 'No PDB present' },
      { rule: 'statefulset_restart', passed: true, reason: 'Not a StatefulSet' },
      { rule: 'cluster_upgrade_state', passed: true, reason: '0% nodes cordoned' },
    ],
  },
  {
    id: 'INC-2024-002',
    status: 'resolved' as const,
    severity: 'warning' as const,
    title: 'order-service: Latency degradation (p99 > 3x baseline)',
    services: ['order-service', 'postgres'],
    detectedAt: '13:45:10',
    resolvedAt: '13:52:30',
    summary: 'p99 latency exceeded 180ms (baseline: 45ms). Caused by slow PostgreSQL queries during vacuum.',
    timeline: [
      { time: '13:45:10', type: 'anomaly', title: 'Latency anomaly detected', description: 'p99 180ms exceeds 3x baseline (45ms) for 2 consecutive windows' },
      { time: '13:45:15', type: 'correlation', title: 'AI Correlator identified root cause', description: 'PostgreSQL auto-vacuum on orders table causing I/O contention' },
      { time: '13:52:30', type: 'resolved', title: 'Auto-resolved', description: 'Vacuum completed. Latency returned to baseline.' },
    ] as TimelineEvent[],
    rootCause: 'PostgreSQL auto-vacuum on orders table causing I/O contention.',
    confidence: 'HIGH',
    safetyResults: [],
  },
];

const typeIcon = (type: string) => {
  switch (type) {
    case 'anomaly': return <AlertTriangle className="h-4 w-4 text-red-500" />;
    case 'correlation': return <Brain className="h-4 w-4 text-purple-500" />;
    case 'safety': return <ShieldCheck className="h-4 w-4 text-blue-500" />;
    case 'remediation': return <Wrench className="h-4 w-4 text-amber-500" />;
    case 'resolved': return <CheckCircle2 className="h-4 w-4 text-emerald-500" />;
    default: return <Clock className="h-4 w-4" />;
  }
};

export default function IncidentDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [expandedEvent, setExpandedEvent] = useState<number | null>(null);

  // If no ID, show list
  if (!id) {
    return (
      <div className="space-y-4 max-w-4xl mx-auto">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">Incidents</h1>
          <div className="flex gap-2">
            <Button variant="outline" size="sm">Active</Button>
            <Button variant="ghost" size="sm">Resolved</Button>
            <Button variant="ghost" size="sm">Silenced</Button>
          </div>
        </div>

        {demoIncidents.map(incident => (
          <Card
            key={incident.id}
            className={`cursor-pointer hover:border-foreground/20 transition-colors ${
              incident.status === 'active' ? 'border-l-4 border-l-red-500' : 'border-l-4 border-l-emerald-500'
            }`}
            onClick={() => navigate(`/incidents/${incident.id}`)}
          >
            <CardContent className="p-4">
              <div className="flex items-start justify-between">
                <div className="space-y-1.5">
                  <div className="flex items-center gap-2">
                    <Badge variant={incident.severity === 'critical' ? 'error' : 'warning'}>{incident.severity}</Badge>
                    <span className="text-sm font-mono text-muted-foreground">{incident.id}</span>
                  </div>
                  <h3 className="font-medium">{incident.title}</h3>
                  <p className="text-sm text-muted-foreground">{incident.summary}</p>
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    <span className="flex items-center gap-1"><Clock className="h-3 w-3" />{incident.detectedAt}</span>
                    <span>{incident.services.join(', ')}</span>
                    {incident.confidence && <Badge variant="outline" className="text-xs">AI: {incident.confidence}</Badge>}
                  </div>
                </div>
                <ChevronRight className="h-5 w-5 text-muted-foreground" />
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  // Detail view
  const incident = demoIncidents.find(i => i.id === id) || demoIncidents[0];

  return (
    <div className="space-y-6 max-w-5xl mx-auto">
      {/* Header */}
      <div>
        <Button variant="ghost" size="sm" onClick={() => navigate('/incidents')} className="mb-2">
          ← Back to Incidents
        </Button>
        <div className="flex items-center gap-3">
          <Badge variant={incident.severity === 'critical' ? 'error' : 'warning'}>{incident.severity}</Badge>
          <Badge variant={incident.status === 'active' ? 'destructive' : 'success'}>{incident.status}</Badge>
          <span className="text-sm font-mono text-muted-foreground">{incident.id}</span>
        </div>
        <h1 className="text-2xl font-bold mt-2">{incident.title}</h1>
      </div>

      <div className="grid grid-cols-3 gap-6">
        {/* Timeline */}
        <div className="col-span-2 space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Event Timeline</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="relative">
                <div className="absolute left-[19px] top-0 bottom-0 w-px bg-border" />
                <div className="space-y-4">
                  {incident.timeline.map((event, i) => (
                    <div key={i} className="relative flex gap-4 pl-0">
                      <div className="relative z-10 flex items-center justify-center w-10 h-10 rounded-full bg-background border">
                        {typeIcon(event.type)}
                      </div>
                      <div
                        className={`flex-1 p-3 rounded-lg transition-colors ${
                          event.details ? 'cursor-pointer hover:bg-secondary/50' : ''
                        } ${expandedEvent === i ? 'bg-secondary/50' : ''}`}
                        onClick={() => event.details && setExpandedEvent(expandedEvent === i ? null : i)}
                      >
                        <div className="flex items-center justify-between">
                          <span className="font-medium text-sm">{event.title}</span>
                          <span className="text-xs text-muted-foreground font-mono">{event.time}</span>
                        </div>
                        <p className="text-sm text-muted-foreground mt-1">{event.description}</p>
                        {event.details && expandedEvent === i && (
                          <pre className="mt-3 p-3 bg-background rounded-md text-xs whitespace-pre-wrap font-mono border">
                            {event.details}
                          </pre>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Sidebar */}
        <div className="space-y-4">
          {/* AI Analysis */}
          <Card className="border-purple-500/20 bg-purple-500/5">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm flex items-center gap-2">
                <Brain className="h-4 w-4 text-purple-500" />
                AI Root Cause Analysis
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm">{incident.rootCause}</p>
              <div className="flex items-center gap-2 mt-3">
                <Badge variant="outline">Confidence: {incident.confidence}</Badge>
              </div>
            </CardContent>
          </Card>

          {/* Safety Results */}
          {incident.safetyResults.length > 0 && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm flex items-center gap-2">
                  <ShieldCheck className="h-4 w-4 text-blue-500" />
                  Safety Check Results
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {incident.safetyResults.map((result, i) => (
                  <div key={i} className="flex items-center gap-2 text-xs">
                    {result.passed ? (
                      <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500 shrink-0" />
                    ) : (
                      <AlertTriangle className="h-3.5 w-3.5 text-red-500 shrink-0" />
                    )}
                    <span className={result.passed ? 'text-muted-foreground' : 'text-red-500 font-medium'}>
                      {result.rule}
                    </span>
                  </div>
                ))}
              </CardContent>
            </Card>
          )}

          {/* Affected Services */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">Affected Services</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {incident.services.map(svc => (
                <Button
                  key={svc}
                  variant="ghost"
                  size="sm"
                  className="w-full justify-start"
                  onClick={() => navigate(`/services/${svc}`)}
                >
                  <ExternalLink className="h-3 w-3 mr-2" />
                  {svc}
                </Button>
              ))}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
