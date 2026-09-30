import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Activity, AlertTriangle, Shield, Clock, Globe,
  TrendingUp, Zap, ChevronRight, ArrowUpRight
} from 'lucide-react';
import {
  AreaChart, Area, XAxis, YAxis, Tooltip,
  ResponsiveContainer, CartesianGrid
} from 'recharts';
import { StatusBadge, HealthDot } from '../components/ui/StatusBadge';
import { MetricCard } from '../components/ui/MetricCard';
import { useDependencies } from '../hooks/useDependencies';
import { usePlatformSummary } from '../hooks/usePlatformSummary';
import { useIncidents } from '../hooks/useIncidents';
import { BackendOfflineBanner } from '../components/ui/BackendOfflineBanner';
import { ChaosControlPanel } from '../components/Demo/ChaosControlPanel';
import { KNOWN_REGIONS } from '../services/telemetryService';
import {
  formatLatency, formatRelativeTime, formatUptime,
  getStatusMeta, CIRCUIT_META, SEVERITY_META
} from '../utils/status';
import './Overview.css';

const CustomTooltip = ({ active, payload, label }: any) => {
  if (!active || !payload?.length) return null;
  return (
    <div className="chart-tooltip">
      <p className="chart-tooltip__label">{label}</p>
      {payload.map((entry: any) => (
        <p key={entry.dataKey} className="chart-tooltip__item" style={{ color: entry.color }}>
          {entry.name}: <strong>{formatLatency(entry.value)}</strong>
        </p>
      ))}
    </div>
  );
};

export default function Overview() {
  const navigate = useNavigate();
  const [tick, setTick] = useState(0);
  const { dependencies, isOffline: depsOffline, lastSuccessAt: depsLastSuccess, refetch: refetchDeps } = useDependencies();
  const { summary, isOffline: summaryOffline, lastSuccessAt: summaryLastSuccess, refetch: refetchSummary } = usePlatformSummary();
  const { incidents, isOffline: incidentsOffline, lastSuccessAt: incidentsLastSuccess, refetch: refetchIncidents } = useIncidents();

  const isOffline = depsOffline || summaryOffline || incidentsOffline;
  const lastSuccessAt = depsLastSuccess ?? summaryLastSuccess ?? incidentsLastSuccess;

  const handleRetry = () => {
    refetchDeps();
    refetchSummary();
    refetchIncidents();
  };

  const activeIncidents = incidents.filter(i => i.status !== 'resolved');

  // Simulate live clock ticking
  useEffect(() => {
    const id = setInterval(() => setTick(t => t + 1), 15000);
    return () => clearInterval(id);
  }, []);

  const criticalDep = dependencies.find(d => d.status === 'grey_failure' || d.status === 'regional_degradation' || d.status === 'degraded' || d.circuitState === 'armed');
  const degradedDeps = dependencies.filter(d => d.status !== 'healthy' && d.status !== 'unknown');

  const chartData = (criticalDep || dependencies[0])?.latencyHistory?.slice(-24).map(pt => ({
    time: new Date(pt.timestamp).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false }),
    p50:  pt.p50,
    p95:  pt.p95,
    p99:  pt.p99,
    err:  pt.errorRate,
  })) || [];

  const activeDep = criticalDep || dependencies[0];

  // Show full-page offline state when backend is unreachable
  if (isOffline) {
    return <BackendOfflineBanner onRetry={handleRetry} lastSuccessAt={lastSuccessAt} />;
  }

  if (!summary) {
    return (
      <div className="overview fade-in" style={{ padding: '3rem', textAlign: 'center' }}>
        <p className="text-secondary">Loading APIPulse telemetry...</p>
      </div>
    );
  }

  return (
    <div className="overview fade-in">
      {/* Live indicator */}
      <div className="overview__live-row">
        <div className="live-indicator">
          <span className="live-dot" />
          LIVE
        </div>
        <span className="text-secondary text-xs">
          Last updated {formatRelativeTime((summary?.lastUpdatedAt ?? Date.now()) - tick * 10)}
        </span>
      </div>

      {/* Page header */}
      <div className="page-header">
        <h1 className="page-title">Command Center</h1>
        <p className="page-subtitle">
          Real-time reliability intelligence across {summary.totalDependencies} monitored dependencies
        </p>
      </div>

      {/* ── Top metric strip ── */}
      <div className="overview__metric-strip">
        <MetricCard
          label="Platform Uptime"
          value={formatUptime(summary.uptimeOverall)}
          subLabel="30-day aggregate"
          color="var(--color-healthy-text)"
        />
        <MetricCard
          label="P50 Overall"
          value={formatLatency(summary.p50Overall)}
          subLabel="aggregate across all deps"
        />
        <MetricCard
          label="P95 Overall"
          value={formatLatency(summary.p95Overall)}
          trend="up"
          trendValue="+312%"
          trendGoodDirection="down"
          color={summary.p95Overall > 500 ? 'var(--color-critical-text)' : undefined}
        />
        <MetricCard
          label="Active Incidents"
          value={summary.activeIncidents}
          trend={summary.activeIncidents > 0 ? 'up' : 'neutral'}
          trendValue={summary.activeIncidents > 0 ? 'Needs attention' : 'Nominal'}
          trendGoodDirection="down"
          color={summary.activeIncidents > 0 ? 'var(--color-critical-text)' : 'var(--color-healthy-text)'}
        />
        <MetricCard
          label="Monitored Deps"
          value={summary.totalDependencies}
          subLabel={`${summary.healthyCount} healthy, ${summary.degradedCount} degraded`}
        />
        <MetricCard
          label="Circuit Breakers"
          value={`${summary.circuitOpenCount} Open`}
          subLabel="1 armed"
          color="var(--color-degraded-text)"
        />
      </div>

      {/* ── Demo / Chaos Control Panel ── */}
      <ChaosControlPanel />

      {/* ── Grey Failure Alert Banner ── */}
      {criticalDep && (
        <div className="overview__alert-banner" role="alert">
          <div className="overview__alert-icon">
            <AlertTriangle size={20} />
          </div>
          <div className="overview__alert-body">
            <p className="overview__alert-title">
              Regional Grey Failure Detected — {criticalDep.vendor.displayName}
            </p>
            <p className="overview__alert-desc">
              AP-South P95 at {formatLatency(criticalDep.latency.p95)} ({Math.round(criticalDep.latency.p95 / criticalDep.baselineP95)}× baseline) ·{' '}
              Vendor reports <strong>Operational</strong> · Circuit <strong>Armed</strong>
            </p>
          </div>
          <button
            className="btn btn-sm overview__alert-action"
            onClick={() => navigate('/incidents')}
            aria-label="View incident details"
          >
            View Incident <ChevronRight size={14} />
          </button>
        </div>
      )}

      <div className="overview__main-grid">
        {/* ── Left Column ── */}
        <div className="overview__left">
          {/* Dependency Health Table */}
          <section aria-labelledby="deps-heading">
            <div className="flex items-center justify-between mb-4">
              <h2 id="deps-heading" className="section-heading">Dependency Health</h2>
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => navigate('/dependencies')}
              >
                View All <ArrowUpRight size={12} />
              </button>
            </div>
            <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Dependency</th>
                    <th>Status</th>
                    <th>P95</th>
                    <th>Error Rate</th>
                    <th>Last Probe</th>
                  </tr>
                </thead>
                <tbody>
                  {dependencies.map(dep => (
                    <tr
                      key={dep.id}
                      className="overview__dep-row"
                      onClick={() => navigate(`/dependencies/${dep.id}`)}
                      style={{ cursor: 'pointer' }}
                      tabIndex={0}
                      onKeyDown={e => e.key === 'Enter' && navigate(`/dependencies/${dep.id}`)}
                    >
                      <td>
                        <div className="flex items-center gap-2">
                          <HealthDot status={dep.status} />
                          <div>
                            <p className="font-medium text-primary">{dep.vendor.displayName}</p>
                            <p className="text-xs text-secondary">{dep.name}</p>
                          </div>
                        </div>
                      </td>
                      <td><StatusBadge status={dep.status} size="sm" /></td>
                      <td>
                        <span
                          className="mono text-sm"
                          style={{
                            color: dep.latency.p95 > dep.baselineP95 * 3
                              ? 'var(--color-critical-text)'
                              : dep.latency.p95 > dep.baselineP95 * 1.5
                              ? 'var(--color-warning-text)'
                              : 'var(--color-text-primary)',
                          }}
                        >
                          {formatLatency(dep.latency.p95)}
                        </span>
                      </td>
                      <td>
                        <span
                          className="mono text-sm"
                          style={{ color: dep.errorRate > 1 ? 'var(--color-critical-text)' : 'var(--color-text-primary)' }}
                        >
                          {dep.errorRate.toFixed(2)}%
                        </span>
                      </td>
                      <td>
                        <span className="text-secondary text-xs">
                          {formatRelativeTime(dep.lastProbeAt)}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>

          {/* Latency Chart */}
          <section aria-labelledby="chart-heading" className="mt-6">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h2 id="chart-heading" className="section-heading">
                  {criticalDep
                    ? `Grey Failure Timeline — ${activeDep?.vendor?.displayName || 'Dependency'}`
                    : `Telemetry Timeline — ${activeDep?.vendor?.displayName || 'Primary Target'}`}
                </h2>
                <p className="text-xs text-secondary mt-1">
                  Latency timeline from APIPulse assessment records ({activeDep?.regions?.[0] || 'ap-south-1'})
                </p>
              </div>
              <div className="flex items-center gap-3">
                <span className="text-xs" style={{ color: '#60a5fa' }}>— P50</span>
                <span className="text-xs" style={{ color: '#f59e0b' }}>— P95</span>
                <span className="text-xs" style={{ color: '#ef4444' }}>— P99</span>
              </div>
            </div>
            <div className="card">
              <div className="chart-container" style={{ height: 220 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={chartData} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
                    <defs>
                      <linearGradient id="p50Grad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#60a5fa" stopOpacity={0.2} />
                        <stop offset="95%" stopColor="#60a5fa" stopOpacity={0} />
                      </linearGradient>
                      <linearGradient id="p95Grad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.2} />
                        <stop offset="95%" stopColor="#f59e0b" stopOpacity={0} />
                      </linearGradient>
                      <linearGradient id="p99Grad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#ef4444" stopOpacity={0.2} />
                        <stop offset="95%" stopColor="#ef4444" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                    <XAxis dataKey="time" tick={{ fill: 'var(--color-text-tertiary)', fontSize: 10 }} tickLine={false} axisLine={false} />
                    <YAxis tickFormatter={v => formatLatency(v)} tick={{ fill: 'var(--color-text-tertiary)', fontSize: 10 }} tickLine={false} axisLine={false} />
                    <Tooltip content={<CustomTooltip />} />
                    <Area type="monotone" dataKey="p50" name="P50" stroke="#60a5fa" strokeWidth={1.5} fill="url(#p50Grad)" dot={false} />
                    <Area type="monotone" dataKey="p95" name="P95" stroke="#f59e0b" strokeWidth={2} fill="url(#p95Grad)" dot={false} />
                    <Area type="monotone" dataKey="p99" name="P99" stroke="#ef4444" strokeWidth={1.5} fill="url(#p99Grad)" dot={false} />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>
          </section>
        </div>

        {/* ── Right Column ── */}
        <div className="overview__right">
          {/* Active Incidents */}
          <section aria-labelledby="incidents-heading">
            <div className="flex items-center justify-between mb-4">
              <h2 id="incidents-heading" className="section-heading">Active Incidents</h2>
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => navigate('/incidents')}
              >
                All <ArrowUpRight size={12} />
              </button>
            </div>
            <div className="flex flex-col gap-2">
              {activeIncidents.map(inc => {
                const sevMeta = SEVERITY_META[inc.severity];
                const statusMeta = getStatusMeta(inc.healthStatus);
                return (
                  <div
                    key={inc.id}
                    className="card incident-card"
                    style={{ borderLeftColor: sevMeta.color, borderLeftWidth: 3 }}
                    onClick={() => navigate('/incidents')}
                    tabIndex={0}
                    onKeyDown={e => e.key === 'Enter' && navigate('/incidents')}
                    role="button"
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex-1 min-w-0">
                        <p className="font-semibold text-primary truncate">{inc.title}</p>
                        <p className="text-xs text-secondary mt-1">{inc.vendorName}</p>
                      </div>
                      <span className={`badge ${statusMeta.cssClass}`} style={{ flexShrink: 0 }}>
                        {statusMeta.shortLabel}
                      </span>
                    </div>
                    <div className="flex items-center gap-3 mt-3">
                      <span className="badge" style={{ background: sevMeta.bgVar, color: sevMeta.color, border: `1px solid ${sevMeta.color}33` }}>
                        {sevMeta.label}
                      </span>
                      <span className="text-xs text-secondary flex items-center gap-1">
                        <Clock size={10} /> {formatRelativeTime(inc.detectedAt)}
                      </span>
                    </div>
                  </div>
                );
              })}
              {activeIncidents.length === 0 && (
                <div className="card empty-state">
                  <Shield size={32} className="empty-state__icon" />
                  <p className="text-sm text-secondary">No active incidents</p>
                </div>
              )}
            </div>
          </section>

          {/* Regional Health */}
          <section aria-labelledby="regional-heading" className="mt-6">
            <div className="flex items-center justify-between mb-4">
              <h2 id="regional-heading" className="section-heading">Regional Status</h2>
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => navigate('/monitoring')}
              >
                Map <ArrowUpRight size={12} />
              </button>
            </div>
            <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
              {KNOWN_REGIONS.map((region, i) => (
                <div
                  key={region.code}
                  className="regional-row"
                  style={{ borderBottom: i < KNOWN_REGIONS.length - 1 ? '1px solid var(--color-border-subtle)' : 'none' }}
                >
                  <div className="flex items-center gap-2 flex-1 min-w-0">
                    <HealthDot status="unknown" />
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-primary truncate">{region.shortLabel}</p>
                      <p className="text-xs text-secondary">{region.code}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="mono text-xs text-secondary">
                      APIPulse monitored
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </section>

          {/* Circuit Breaker State */}
          <section aria-labelledby="circuit-heading" className="mt-6">
            <h2 id="circuit-heading" className="section-heading mb-4">Circuit Protection</h2>
            <div className="flex flex-col gap-2">
              {dependencies.map(dep => {
                const cm = CIRCUIT_META[dep.circuitState];
                return (
                  <div key={dep.id} className="flex items-center justify-between card" style={{ padding: 'var(--space-3) var(--space-4)' }}>
                    <span className="text-sm text-primary font-medium">{dep.vendor.displayName}</span>
                    <span className="mono text-xs font-semibold" style={{ color: cm.color }}>
                      {dep.circuitState.toUpperCase().replace('_', ' ')}
                    </span>
                  </div>
                );
              })}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
