import { useParams } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { LineChart, Line, AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { ArrowLeft, Clock, Activity, AlertTriangle, ExternalLink } from 'lucide-react';
import { useNavigate } from 'react-router-dom';

// Demo time-series data
const generateTimeSeries = (points: number, baseValue: number, variance: number, spike?: { at: number; value: number }) => {
  return Array.from({ length: points }, (_, i) => {
    const time = new Date(Date.now() - (points - i) * 60000);
    let value = baseValue + (Math.random() - 0.5) * variance;
    if (spike && i === spike.at) value = spike.value;
    return {
      time: time.toLocaleTimeString('en', { hour: '2-digit', minute: '2-digit' }),
      value: Math.max(0, value),
    };
  });
};

const latencyData = generateTimeSeries(60, 15, 8);
const latencyP99Data = generateTimeSeries(60, 45, 25, { at: 42, value: 180 });
const requestRateData = generateTimeSeries(60, 650, 120);
const errorRateData = generateTimeSeries(60, 0.5, 0.3, { at: 42, value: 3.8 });

const demoTraces = [
  { traceId: 'abc123', timestamp: '14:23:01', method: 'POST', path: '/api/orders', statusCode: 201, latencyMs: 45, upstream: 'api-gateway' },
  { traceId: 'def456', timestamp: '14:22:58', method: 'POST', path: '/api/orders', statusCode: 500, latencyMs: 4200, upstream: 'api-gateway' },
  { traceId: 'ghi789', timestamp: '14:22:55', method: 'GET', path: '/api/orders/123', statusCode: 200, latencyMs: 12, upstream: 'api-gateway' },
  { traceId: 'jkl012', timestamp: '14:22:51', method: 'POST', path: '/api/orders', statusCode: 201, latencyMs: 38, upstream: 'api-gateway' },
  { traceId: 'mno345', timestamp: '14:22:48', method: 'GET', path: '/api/orders', statusCode: 200, latencyMs: 22, upstream: 'search-service' },
  { traceId: 'pqr678', timestamp: '14:22:45', method: 'POST', path: '/api/orders/123/cancel', statusCode: 200, latencyMs: 95, upstream: 'api-gateway' },
  { traceId: 'stu901', timestamp: '14:22:40', method: 'POST', path: '/api/orders', statusCode: 503, latencyMs: 180, upstream: 'api-gateway' },
  { traceId: 'vwx234', timestamp: '14:22:38', method: 'GET', path: '/api/orders/456', statusCode: 200, latencyMs: 8, upstream: 'api-gateway' },
];

export default function ServiceDetail() {
  const { service } = useParams();
  const navigate = useNavigate();
  const serviceName = service || 'order-service';

  return (
    <div className="space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={() => navigate('/map')}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold">{serviceName}</h1>
            <Badge variant="warning">degraded</Badge>
            <Badge variant="outline">production</Badge>
          </div>
          <p className="text-sm text-muted-foreground mt-1">3 pods · Deployment · Last deployed 2h ago</p>
        </div>
      </div>

      {/* Charts grid */}
      <div className="grid grid-cols-2 gap-4">
        {/* Request Rate */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Request Rate</CardTitle>
            <CardDescription>requests/sec, 60s granularity</CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={requestRateData}>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                <XAxis dataKey="time" tick={{ fontSize: 10 }} stroke="hsl(var(--muted-foreground))" />
                <YAxis tick={{ fontSize: 10 }} stroke="hsl(var(--muted-foreground))" />
                <Tooltip
                  contentStyle={{ background: 'hsl(var(--card))', border: '1px solid hsl(var(--border))', borderRadius: '8px' }}
                  labelStyle={{ color: 'hsl(var(--foreground))' }}
                />
                <Area type="monotone" dataKey="value" stroke="#3b82f6" fill="#3b82f6" fillOpacity={0.1} strokeWidth={2} name="req/s" />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        {/* Latency */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Latency</CardTitle>
            <CardDescription>p50 / p99 with baseline band</CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={200}>
              <LineChart>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                <XAxis dataKey="time" data={latencyData} tick={{ fontSize: 10 }} stroke="hsl(var(--muted-foreground))" />
                <YAxis tick={{ fontSize: 10 }} stroke="hsl(var(--muted-foreground))" unit="ms" />
                <Tooltip contentStyle={{ background: 'hsl(var(--card))', border: '1px solid hsl(var(--border))', borderRadius: '8px' }} />
                <Legend />
                <Line type="monotone" data={latencyData} dataKey="value" stroke="#22c55e" strokeWidth={2} dot={false} name="p50" />
                <Line type="monotone" data={latencyP99Data} dataKey="value" stroke="#ef4444" strokeWidth={2} dot={false} name="p99" />
              </LineChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        {/* Error Rate */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Error Rate</CardTitle>
            <CardDescription>4xx + 5xx percentage</CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={errorRateData}>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                <XAxis dataKey="time" tick={{ fontSize: 10 }} stroke="hsl(var(--muted-foreground))" />
                <YAxis tick={{ fontSize: 10 }} stroke="hsl(var(--muted-foreground))" unit="%" />
                <Tooltip contentStyle={{ background: 'hsl(var(--card))', border: '1px solid hsl(var(--border))', borderRadius: '8px' }} />
                <Area type="monotone" dataKey="value" stroke="#ef4444" fill="#ef4444" fillOpacity={0.1} strokeWidth={2} name="error %" />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        {/* Dependencies */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Dependencies</CardTitle>
            <CardDescription>Services this service calls</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {[
              { name: 'payment-service', health: 'healthy', latency: '22ms' },
              { name: 'inventory-service', health: 'healthy', latency: '10ms' },
              { name: 'notification-service', health: 'critical', latency: '4200ms' },
              { name: 'postgres', health: 'healthy', latency: '2ms' },
            ].map(dep => (
              <div key={dep.name} className="flex items-center justify-between p-2 rounded-lg hover:bg-secondary/50 transition-colors cursor-pointer"
                   onClick={() => navigate(`/services/${dep.name}`)}>
                <div className="flex items-center gap-2">
                  <div className={`w-2 h-2 rounded-full ${dep.health === 'healthy' ? 'bg-emerald-500' : dep.health === 'critical' ? 'bg-red-500' : 'bg-amber-500'}`} />
                  <span className="text-sm">{dep.name}</span>
                </div>
                <span className={`text-sm font-mono ${dep.health === 'critical' ? 'text-red-500' : 'text-muted-foreground'}`}>{dep.latency}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>

      {/* Traces table */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="text-sm font-medium">Recent Traces</CardTitle>
            <div className="flex gap-2">
              <Button variant="outline" size="sm">Filter by status</Button>
              <Button variant="outline" size="sm">Sort by latency</Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-muted-foreground">
                  <th className="text-left py-2 px-3 font-medium">Time</th>
                  <th className="text-left py-2 px-3 font-medium">Method</th>
                  <th className="text-left py-2 px-3 font-medium">Path</th>
                  <th className="text-left py-2 px-3 font-medium">Status</th>
                  <th className="text-left py-2 px-3 font-medium">Latency</th>
                  <th className="text-left py-2 px-3 font-medium">Upstream</th>
                </tr>
              </thead>
              <tbody>
                {demoTraces.map(trace => (
                  <tr key={trace.traceId} className="border-b border-border/50 hover:bg-secondary/30 transition-colors cursor-pointer">
                    <td className="py-2 px-3 font-mono text-xs">{trace.timestamp}</td>
                    <td className="py-2 px-3">
                      <Badge variant="outline" className="font-mono text-xs">{trace.method}</Badge>
                    </td>
                    <td className="py-2 px-3 font-mono text-xs">{trace.path}</td>
                    <td className="py-2 px-3">
                      <Badge variant={trace.statusCode >= 500 ? 'error' : trace.statusCode >= 400 ? 'warning' : 'success'}>
                        {trace.statusCode}
                      </Badge>
                    </td>
                    <td className={`py-2 px-3 font-mono text-xs ${trace.latencyMs > 1000 ? 'text-red-500 font-bold' : trace.latencyMs > 100 ? 'text-amber-500' : ''}`}>
                      {trace.latencyMs}ms
                    </td>
                    <td className="py-2 px-3 text-xs text-muted-foreground">{trace.upstream}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
