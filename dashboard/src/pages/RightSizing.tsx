import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { TrendingDown, DollarSign, Cpu, MemoryStick, ArrowRight, CheckCircle2, Info } from 'lucide-react';
import { useState } from 'react';

interface Recommendation {
  deployment: string;
  namespace: string;
  resource: 'cpu' | 'memory';
  currentLimit: string;
  p99ActualUsage: string;
  recommendedLimit: string;
  monthlySaving: number;
  dataDays: number;
  confidence: 'high' | 'medium' | 'low';
  savingsPercent: number;
}

const demoRecommendations: Recommendation[] = [
  { deployment: 'api-gateway', namespace: 'production', resource: 'cpu', currentLimit: '1000m', p99ActualUsage: '280m', recommendedLimit: '400m', monthlySaving: 45, dataDays: 14, confidence: 'high', savingsPercent: 60 },
  { deployment: 'api-gateway', namespace: 'production', resource: 'memory', currentLimit: '1Gi', p99ActualUsage: '320Mi', recommendedLimit: '512Mi', monthlySaving: 32, dataDays: 14, confidence: 'high', savingsPercent: 50 },
  { deployment: 'search-service', namespace: 'production', resource: 'cpu', currentLimit: '2000m', p99ActualUsage: '450m', recommendedLimit: '700m', monthlySaving: 85, dataDays: 14, confidence: 'high', savingsPercent: 65 },
  { deployment: 'search-service', namespace: 'production', resource: 'memory', currentLimit: '2Gi', p99ActualUsage: '680Mi', recommendedLimit: '1Gi', monthlySaving: 55, dataDays: 14, confidence: 'medium', savingsPercent: 50 },
  { deployment: 'user-service', namespace: 'production', resource: 'cpu', currentLimit: '500m', p99ActualUsage: '180m', recommendedLimit: '250m', monthlySaving: 18, dataDays: 14, confidence: 'high', savingsPercent: 50 },
  { deployment: 'payment-service', namespace: 'production', resource: 'memory', currentLimit: '512Mi', p99ActualUsage: '160Mi', recommendedLimit: '256Mi', monthlySaving: 12, dataDays: 14, confidence: 'medium', savingsPercent: 50 },
  { deployment: 'inventory-service', namespace: 'production', resource: 'cpu', currentLimit: '500m', p99ActualUsage: '95m', recommendedLimit: '150m', monthlySaving: 22, dataDays: 14, confidence: 'high', savingsPercent: 70 },
  { deployment: 'recommendation-engine', namespace: 'ml', resource: 'cpu', currentLimit: '4000m', p99ActualUsage: '1200m', recommendedLimit: '2000m', monthlySaving: 130, dataDays: 7, confidence: 'low', savingsPercent: 50 },
  { deployment: 'recommendation-engine', namespace: 'ml', resource: 'memory', currentLimit: '8Gi', p99ActualUsage: '3.2Gi', recommendedLimit: '5Gi', monthlySaving: 95, dataDays: 7, confidence: 'low', savingsPercent: 37.5 },
  { deployment: 'cache-service', namespace: 'production', resource: 'memory', currentLimit: '4Gi', p99ActualUsage: '2.8Gi', recommendedLimit: '3.5Gi', monthlySaving: 15, dataDays: 14, confidence: 'high', savingsPercent: 12.5 },
];

export default function RightSizing() {
  const [filter, setFilter] = useState<'all' | 'cpu' | 'memory'>('all');
  const [sortBy, setSortBy] = useState<'savings' | 'confidence'>('savings');
  const [appliedIds, setAppliedIds] = useState<Set<string>>(new Set());

  const filtered = demoRecommendations
    .filter(r => filter === 'all' || r.resource === filter)
    .sort((a, b) => {
      if (sortBy === 'savings') return b.monthlySaving - a.monthlySaving;
      const conf = { high: 3, medium: 2, low: 1 };
      return conf[b.confidence] - conf[a.confidence];
    });

  const totalMonthlySavings = demoRecommendations.reduce((sum, r) => sum + r.monthlySaving, 0);
  const totalAnnualSavings = totalMonthlySavings * 12;
  const highConfidenceCount = demoRecommendations.filter(r => r.confidence === 'high').length;

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      <div>
        <h1 className="text-2xl font-bold">Right-Sizing Recommendations</h1>
        <p className="text-muted-foreground mt-1">Based on 14 days of actual resource usage captured via eBPF</p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-4 gap-4">
        <Card className="bg-gradient-to-br from-emerald-500/10 to-emerald-500/5 border-emerald-500/20">
          <CardContent className="p-4">
            <div className="flex items-center gap-2 text-emerald-500">
              <DollarSign className="h-4 w-4" />
              <span className="text-sm font-medium">Monthly Savings</span>
            </div>
            <p className="text-3xl font-bold mt-2">${totalMonthlySavings}</p>
          </CardContent>
        </Card>
        <Card className="bg-gradient-to-br from-blue-500/10 to-blue-500/5 border-blue-500/20">
          <CardContent className="p-4">
            <div className="flex items-center gap-2 text-blue-500">
              <TrendingDown className="h-4 w-4" />
              <span className="text-sm font-medium">Annual Savings</span>
            </div>
            <p className="text-3xl font-bold mt-2">${totalAnnualSavings.toLocaleString()}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-2 text-muted-foreground">
              <CheckCircle2 className="h-4 w-4" />
              <span className="text-sm font-medium">High Confidence</span>
            </div>
            <p className="text-3xl font-bold mt-2">{highConfidenceCount}</p>
            <p className="text-xs text-muted-foreground">of {demoRecommendations.length} recommendations</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-2 text-muted-foreground">
              <Info className="h-4 w-4" />
              <span className="text-sm font-medium">Data Window</span>
            </div>
            <p className="text-3xl font-bold mt-2">14d</p>
            <p className="text-xs text-muted-foreground">of continuous monitoring</p>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <div className="flex gap-1 bg-secondary rounded-lg p-1">
          {(['all', 'cpu', 'memory'] as const).map(f => (
            <Button
              key={f}
              variant={filter === f ? 'default' : 'ghost'}
              size="sm"
              onClick={() => setFilter(f)}
              className="capitalize"
            >
              {f === 'cpu' ? <Cpu className="h-3 w-3 mr-1" /> : f === 'memory' ? <MemoryStick className="h-3 w-3 mr-1" /> : null}
              {f}
            </Button>
          ))}
        </div>
        <div className="flex gap-1 bg-secondary rounded-lg p-1">
          <Button variant={sortBy === 'savings' ? 'default' : 'ghost'} size="sm" onClick={() => setSortBy('savings')}>
            By Savings
          </Button>
          <Button variant={sortBy === 'confidence' ? 'default' : 'ghost'} size="sm" onClick={() => setSortBy('confidence')}>
            By Confidence
          </Button>
        </div>
      </div>

      {/* Recommendations table */}
      <Card>
        <CardContent className="p-0">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-secondary/30">
                <th className="text-left py-3 px-4 font-medium">Deployment</th>
                <th className="text-left py-3 px-4 font-medium">Resource</th>
                <th className="text-left py-3 px-4 font-medium">Current Limit</th>
                <th className="text-left py-3 px-4 font-medium">p99 Actual Usage</th>
                <th className="text-left py-3 px-4 font-medium">Recommended</th>
                <th className="text-left py-3 px-4 font-medium">Confidence</th>
                <th className="text-right py-3 px-4 font-medium">Monthly Savings</th>
                <th className="text-center py-3 px-4 font-medium">Action</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((rec, i) => {
                const key = `${rec.deployment}-${rec.resource}`;
                const isApplied = appliedIds.has(key);
                return (
                  <tr key={key} className="border-b border-border/50 hover:bg-secondary/20 transition-colors">
                    <td className="py-3 px-4">
                      <div>
                        <span className="font-medium">{rec.deployment}</span>
                        <span className="text-xs text-muted-foreground ml-2">{rec.namespace}</span>
                      </div>
                    </td>
                    <td className="py-3 px-4">
                      <Badge variant="outline" className="font-mono text-xs">
                        {rec.resource === 'cpu' ? <Cpu className="h-3 w-3 mr-1" /> : <MemoryStick className="h-3 w-3 mr-1" />}
                        {rec.resource}
                      </Badge>
                    </td>
                    <td className="py-3 px-4 font-mono text-xs">{rec.currentLimit}</td>
                    <td className="py-3 px-4 font-mono text-xs text-emerald-500">{rec.p99ActualUsage}</td>
                    <td className="py-3 px-4">
                      <div className="flex items-center gap-1.5 font-mono text-xs">
                        <span className="text-muted-foreground line-through">{rec.currentLimit}</span>
                        <ArrowRight className="h-3 w-3 text-muted-foreground" />
                        <span className="text-blue-500 font-semibold">{rec.recommendedLimit}</span>
                      </div>
                    </td>
                    <td className="py-3 px-4">
                      <Badge variant={rec.confidence === 'high' ? 'success' : rec.confidence === 'medium' ? 'warning' : 'secondary'}>
                        {rec.confidence}
                      </Badge>
                    </td>
                    <td className="py-3 px-4 text-right font-semibold text-emerald-500">${rec.monthlySaving}</td>
                    <td className="py-3 px-4 text-center">
                      {isApplied ? (
                        <Badge variant="success">
                          <CheckCircle2 className="h-3 w-3 mr-1" />
                          Applied
                        </Badge>
                      ) : (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setAppliedIds(prev => new Set([...prev, key]))}
                        >
                          Apply
                        </Button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </CardContent>
      </Card>

      <p className="text-xs text-muted-foreground text-center">
        Recommendations are based on p99 actual resource usage over {demoRecommendations[0]?.dataDays || 14} days.
        A 20% safety buffer is included in all recommended limits.
        Apply recommendations via <code className="bg-secondary px-1 rounded">kubectl patch</code> or click "Apply" for auto-patching (Enterprise).
      </p>
    </div>
  );
}
