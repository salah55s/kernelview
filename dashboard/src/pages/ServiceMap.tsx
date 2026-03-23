import { useEffect, useRef, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import * as d3 from 'd3';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Activity, AlertTriangle, Clock, Filter, Maximize2, ZoomIn, ZoomOut } from 'lucide-react';
import type { ServiceMapNode, ServiceMapEdge } from '@/api/client';

// Demo data for the service map (replaced by API in production)
const DEMO_NODES: ServiceMapNode[] = [
  { service: { name: 'api-gateway', namespace: 'production', podCount: 3 }, requestRate: 2450, errorRate: 0.5, latencyP50: 12, latencyP99: 45, health: 'healthy', hasAnomaly: false, hasRemediation: false },
  { service: { name: 'user-service', namespace: 'production', podCount: 2 }, requestRate: 800, errorRate: 1.2, latencyP50: 8, latencyP99: 35, health: 'healthy', hasAnomaly: false, hasRemediation: false },
  { service: { name: 'order-service', namespace: 'production', podCount: 3 }, requestRate: 650, errorRate: 3.8, latencyP50: 15, latencyP99: 180, health: 'degraded', hasAnomaly: true, hasRemediation: false },
  { service: { name: 'payment-service', namespace: 'production', podCount: 2 }, requestRate: 320, errorRate: 0.1, latencyP50: 22, latencyP99: 95, health: 'healthy', hasAnomaly: false, hasRemediation: false },
  { service: { name: 'inventory-service', namespace: 'production', podCount: 2 }, requestRate: 450, errorRate: 0.3, latencyP50: 10, latencyP99: 42, health: 'healthy', hasAnomaly: false, hasRemediation: false },
  { service: { name: 'notification-service', namespace: 'production', podCount: 1 }, requestRate: 200, errorRate: 12.5, latencyP50: 45, latencyP99: 4200, health: 'critical', hasAnomaly: true, hasRemediation: true },
  { service: { name: 'search-service', namespace: 'production', podCount: 3 }, requestRate: 1200, errorRate: 0.2, latencyP50: 6, latencyP99: 28, health: 'healthy', hasAnomaly: false, hasRemediation: false },
  { service: { name: 'recommendation-engine', namespace: 'ml', podCount: 2 }, requestRate: 180, errorRate: 0.8, latencyP50: 120, latencyP99: 450, health: 'healthy', hasAnomaly: false, hasRemediation: false },
  { service: { name: 'cache-service', namespace: 'production', podCount: 3 }, requestRate: 5200, errorRate: 0.01, latencyP50: 1, latencyP99: 3, health: 'healthy', hasAnomaly: false, hasRemediation: false },
  { service: { name: 'auth-service', namespace: 'production', podCount: 2 }, requestRate: 950, errorRate: 0.4, latencyP50: 5, latencyP99: 15, health: 'healthy', hasAnomaly: false, hasRemediation: false },
  { service: { name: 'email-worker', namespace: 'workers', podCount: 1 }, requestRate: 50, errorRate: 2.0, latencyP50: 200, latencyP99: 800, health: 'learning', hasAnomaly: false, hasRemediation: false },
  { service: { name: 'postgres', namespace: 'data', podCount: 3 }, requestRate: 3800, errorRate: 0.05, latencyP50: 2, latencyP99: 12, health: 'healthy', hasAnomaly: false, hasRemediation: false },
];

const DEMO_EDGES: ServiceMapEdge[] = [
  { source: 'api-gateway', target: 'user-service', requestRate: 800, errorRate: 0.5 },
  { source: 'api-gateway', target: 'order-service', requestRate: 650, errorRate: 1.2 },
  { source: 'api-gateway', target: 'search-service', requestRate: 1200, errorRate: 0.1 },
  { source: 'api-gateway', target: 'auth-service', requestRate: 950, errorRate: 0.2 },
  { source: 'order-service', target: 'payment-service', requestRate: 320, errorRate: 0.3 },
  { source: 'order-service', target: 'inventory-service', requestRate: 450, errorRate: 0.5 },
  { source: 'order-service', target: 'notification-service', requestRate: 200, errorRate: 8.0 },
  { source: 'user-service', target: 'postgres', requestRate: 1500, errorRate: 0.02 },
  { source: 'order-service', target: 'postgres', requestRate: 1200, errorRate: 0.05 },
  { source: 'search-service', target: 'cache-service', requestRate: 3000, errorRate: 0.01 },
  { source: 'api-gateway', target: 'recommendation-engine', requestRate: 180, errorRate: 0.3 },
  { source: 'auth-service', target: 'cache-service', requestRate: 2200, errorRate: 0.01 },
  { source: 'notification-service', target: 'email-worker', requestRate: 50, errorRate: 1.0 },
  { source: 'inventory-service', target: 'postgres', requestRate: 1100, errorRate: 0.03 },
];

// Utility functions
const healthColor = (health: string) => {
  switch (health) {
    case 'healthy': return '#22c55e';
    case 'degraded': return '#eab308';
    case 'critical': return '#ef4444';
    case 'learning': return '#3b82f6';
    default: return '#6b7280';
  }
};

const edgeColor = (errorRate: number) => {
  if (errorRate < 1) return '#22c55e';
  if (errorRate < 5) return '#eab308';
  return '#ef4444';
};

export default function ServiceMap() {
  const svgRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [selectedNode, setSelectedNode] = useState<ServiceMapNode | null>(null);
  const [timeRange, setTimeRange] = useState('1h');
  const navigate = useNavigate();

  const drawGraph = useCallback(() => {
    if (!svgRef.current || !containerRef.current) return;

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    const rect = containerRef.current.getBoundingClientRect();
    const width = rect.width;
    const height = rect.height;

    svg.attr('width', width).attr('height', height);

    // Zoom behavior
    const g = svg.append('g');
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.3, 4])
      .on('zoom', (event) => {
        g.attr('transform', event.transform);
      });
    svg.call(zoom);

    // Arrow marker
    svg.append('defs').append('marker')
      .attr('id', 'arrowhead')
      .attr('viewBox', '0 -5 10 10')
      .attr('refX', 20)
      .attr('refY', 0)
      .attr('markerWidth', 6)
      .attr('markerHeight', 6)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M0,-5L10,0L0,5')
      .attr('fill', '#6b7280');

    // Force simulation
    const nodes = DEMO_NODES.map(n => ({ ...n, id: n.service.name }));
    const links = DEMO_EDGES.map(e => ({ ...e }));

    const simulation = d3.forceSimulation(nodes as any)
      .force('link', d3.forceLink(links as any).id((d: any) => d.id).distance(160))
      .force('charge', d3.forceManyBody().strength(-600))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collision', d3.forceCollide().radius(50));

    // Draw edges
    const link = g.append('g')
      .selectAll('line')
      .data(links)
      .join('line')
      .attr('stroke', d => edgeColor(d.errorRate))
      .attr('stroke-opacity', 0.6)
      .attr('stroke-width', d => Math.max(1, Math.log2(d.requestRate / 100 + 1) * 2))
      .attr('marker-end', 'url(#arrowhead)');

    // Draw nodes
    const node = g.append('g')
      .selectAll('g')
      .data(nodes)
      .join('g')
      .attr('cursor', 'pointer')
      .call(d3.drag<SVGGElement, any>()
        .on('start', (event, d: any) => {
          if (!event.active) simulation.alphaTarget(0.3).restart();
          d.fx = d.x;
          d.fy = d.y;
        })
        .on('drag', (event, d: any) => {
          d.fx = event.x;
          d.fy = event.y;
        })
        .on('end', (event, d: any) => {
          if (!event.active) simulation.alphaTarget(0);
          d.fx = null;
          d.fy = null;
        })
      );

    // Node circles
    node.append('circle')
      .attr('r', d => Math.max(14, Math.log2(d.requestRate + 1) * 4))
      .attr('fill', d => healthColor(d.health))
      .attr('fill-opacity', 0.15)
      .attr('stroke', d => healthColor(d.health))
      .attr('stroke-width', 2)
      .attr('class', d => d.hasAnomaly ? 'animate-pulse-anomaly' : '');

    // Inner dot
    node.append('circle')
      .attr('r', 4)
      .attr('fill', d => healthColor(d.health));

    // Wrench icon for remediation
    node.filter(d => d.hasRemediation)
      .append('text')
      .attr('text-anchor', 'middle')
      .attr('dy', -20)
      .attr('font-size', '14px')
      .text('🔧');

    // Labels
    node.append('text')
      .attr('dy', d => Math.max(14, Math.log2(d.requestRate + 1) * 4) + 16)
      .attr('text-anchor', 'middle')
      .attr('fill', 'currentColor')
      .attr('font-size', '11px')
      .attr('font-weight', '500')
      .attr('class', 'text-foreground')
      .text(d => d.service.name);

    // Click handler
    node.on('click', (_event, d) => {
      setSelectedNode(d);
    });

    // Tick
    simulation.on('tick', () => {
      link
        .attr('x1', (d: any) => d.source.x)
        .attr('y1', (d: any) => d.source.y)
        .attr('x2', (d: any) => d.target.x)
        .attr('y2', (d: any) => d.target.y);

      node.attr('transform', (d: any) => `translate(${d.x},${d.y})`);
    });
  }, []);

  useEffect(() => {
    drawGraph();
    window.addEventListener('resize', drawGraph);
    return () => window.removeEventListener('resize', drawGraph);
  }, [drawGraph]);

  // Stats
  const totalServices = DEMO_NODES.length;
  const activeAnomalies = DEMO_NODES.filter(n => n.hasAnomaly).length;
  const totalRPS = DEMO_NODES.reduce((sum, n) => sum + n.requestRate, 0);
  const avgLatency = DEMO_NODES.reduce((sum, n) => sum + n.latencyP99, 0) / DEMO_NODES.length;

  return (
    <div className="space-y-4">
      {/* Stats bar */}
      <div className="grid grid-cols-4 gap-4">
        <Card className="bg-card/50 backdrop-blur-sm">
          <CardContent className="p-4">
            <div className="flex items-center gap-2">
              <Activity className="h-4 w-4 text-blue-500" />
              <span className="text-sm text-muted-foreground">Services</span>
            </div>
            <p className="text-2xl font-bold mt-1">{totalServices}</p>
          </CardContent>
        </Card>
        <Card className="bg-card/50 backdrop-blur-sm">
          <CardContent className="p-4">
            <div className="flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 text-amber-500" />
              <span className="text-sm text-muted-foreground">Active Anomalies</span>
            </div>
            <p className="text-2xl font-bold mt-1">{activeAnomalies}</p>
          </CardContent>
        </Card>
        <Card className="bg-card/50 backdrop-blur-sm">
          <CardContent className="p-4">
            <div className="flex items-center gap-2">
              <Activity className="h-4 w-4 text-emerald-500" />
              <span className="text-sm text-muted-foreground">Cluster RPS</span>
            </div>
            <p className="text-2xl font-bold mt-1">{totalRPS.toLocaleString()}</p>
          </CardContent>
        </Card>
        <Card className="bg-card/50 backdrop-blur-sm">
          <CardContent className="p-4">
            <div className="flex items-center gap-2">
              <Clock className="h-4 w-4 text-purple-500" />
              <span className="text-sm text-muted-foreground">Avg p99 Latency</span>
            </div>
            <p className="text-2xl font-bold mt-1">{avgLatency.toFixed(0)}ms</p>
          </CardContent>
        </Card>
      </div>

      <div className="flex gap-4 h-[calc(100vh-220px)]">
        {/* Graph */}
        <Card className="flex-1 relative overflow-hidden">
          <div className="absolute top-3 right-3 z-10 flex gap-2">
            {['15m', '1h', '6h', '24h'].map(t => (
              <Button
                key={t}
                variant={timeRange === t ? 'default' : 'outline'}
                size="sm"
                onClick={() => setTimeRange(t)}
              >
                {t}
              </Button>
            ))}
            <Button variant="outline" size="icon" className="h-8 w-8">
              <Filter className="h-3 w-3" />
            </Button>
          </div>
          <div ref={containerRef} className="w-full h-full">
            <svg ref={svgRef} className="w-full h-full" />
          </div>
        </Card>

        {/* Detail drawer */}
        {selectedNode && (
          <Card className="w-80 flex flex-col glass-panel">
            <CardHeader className="pb-3">
              <div className="flex items-center justify-between">
                <CardTitle className="text-base">{selectedNode.service.name}</CardTitle>
                <Badge variant={
                  selectedNode.health === 'healthy' ? 'success' :
                  selectedNode.health === 'degraded' ? 'warning' :
                  selectedNode.health === 'critical' ? 'error' : 'secondary'
                }>
                  {selectedNode.health}
                </Badge>
              </div>
              <p className="text-xs text-muted-foreground">{selectedNode.service.namespace} · {selectedNode.service.podCount} pods</p>
            </CardHeader>
            <CardContent className="space-y-4 flex-1">
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">Request Rate</p>
                  <p className="text-lg font-semibold">{selectedNode.requestRate}/s</p>
                </div>
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">Error Rate</p>
                  <p className={`text-lg font-semibold ${selectedNode.errorRate > 5 ? 'text-red-500' : selectedNode.errorRate > 1 ? 'text-amber-500' : 'text-emerald-500'}`}>
                    {selectedNode.errorRate}%
                  </p>
                </div>
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">p50 Latency</p>
                  <p className="text-lg font-semibold">{selectedNode.latencyP50}ms</p>
                </div>
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">p99 Latency</p>
                  <p className={`text-lg font-semibold ${selectedNode.latencyP99 > 1000 ? 'text-red-500' : ''}`}>
                    {selectedNode.latencyP99}ms
                  </p>
                </div>
              </div>

              {selectedNode.hasAnomaly && (
                <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3">
                  <div className="flex items-center gap-2 text-red-500 text-sm font-medium">
                    <AlertTriangle className="h-4 w-4" />
                    Active Anomaly
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">
                    {selectedNode.health === 'critical' ? 'Error rate exceeds threshold' : 'Latency degradation detected'}
                  </p>
                </div>
              )}

              {selectedNode.hasRemediation && (
                <div className="bg-blue-500/10 border border-blue-500/20 rounded-lg p-3">
                  <div className="flex items-center gap-2 text-blue-500 text-sm font-medium">
                    🔧 Auto-Remediation Active
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">
                    CPU throttle applied — auto-revert in 4m 23s
                  </p>
                </div>
              )}

              <Button
                className="w-full mt-4"
                onClick={() => navigate(`/services/${selectedNode.service.name}`)}
              >
                View Details
              </Button>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}
