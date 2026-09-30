import React, { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Activity, ChevronRight, Search, Filter, Globe,
  Clock, Shield, ArrowLeft, Radio
} from 'lucide-react';
import {
  AreaChart, Area, XAxis, YAxis, Tooltip,
  ResponsiveContainer, CartesianGrid, BarChart, Bar
} from 'recharts';
import { StatusBadge, HealthDot } from '../components/ui/StatusBadge';
import { LatencyMetricRow } from '../components/ui/MetricCard';
import { useDependencies, useDependency } from '../hooks/useDependencies';
import { KNOWN_REGIONS } from '../services/telemetryService';
import { BackendOfflineBanner } from '../components/ui/BackendOfflineBanner';
import {
  formatLatency, formatRelativeTime, formatUptime, formatErrorRate,
  getStatusMeta, CIRCUIT_META
} from '../utils/status';
import type { HealthStatus, RegionCode } from '../types/domain';
import './Dependencies.css';

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

// ─── Dependency List View ─────────────────────────────────────

export default function Dependencies() {
  const { id } = useParams<{ id: string }>();
  if (id) return <DependencyDetail id={id} />;
  return <DependencyList />;
}

function DependencyList() {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<HealthStatus | 'all'>('all');
  const { dependencies, isOffline, lastSuccessAt, refetch } = useDependencies();

  if (isOffline) {
    return <BackendOfflineBanner onRetry={refetch} lastSuccessAt={lastSuccessAt} />;
  }

  const filtered = dependencies.filter(dep => {
    const matchSearch = !search ||
      dep.name.toLowerCase().includes(search.toLowerCase()) ||
      dep.vendor.displayName.toLowerCase().includes(search.toLowerCase());
    const matchStatus = statusFilter === 'all' ||
      dep.status === statusFilter ||
      (statusFilter === 'regional_degradation' && dep.status === 'grey_failure') ||
      (statusFilter === 'grey_failure' && dep.status === 'regional_degradation');
    return matchSearch && matchStatus;
  });

  return (
    <div className="fade-in">
      <div className="page-header">
        <h1 className="page-title">Dependencies</h1>
        <p className="page-subtitle">Manage and monitor all third-party API dependencies</p>
      </div>

      {/* Filter Bar */}
      <div className="deps__filter-bar">
        <div className="deps__search-wrap">
          <Search size={15} className="deps__search-icon" />
          <input
            className="deps__search-input"
            placeholder="Search dependencies..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            aria-label="Search dependencies"
          />
        </div>
        <div className="flex items-center gap-2">
          <Filter size={14} style={{ color: 'var(--color-text-tertiary)' }} />
          {(['all', 'healthy', 'degraded', 'grey_failure', 'regional_degradation'] as const).map(s => (
            <button
              key={s}
              className={`btn btn-sm ${statusFilter === s ? 'btn-secondary' : 'btn-ghost'}`}
              onClick={() => setStatusFilter(s)}
            >
              {s === 'all' ? 'All' : getStatusMeta(s as HealthStatus).label}
            </button>
          ))}
        </div>
      </div>

      {/* Summary Pills */}
      <div className="deps__summary-row">
        {(['healthy', 'degraded', 'grey_failure'] as const).map(s => {
          const count = dependencies.filter(d => d.status === s || (s === 'grey_failure' && d.status === 'regional_degradation')).length;
          const meta = getStatusMeta(s);
          return (
            <div key={s} className={`deps__summary-pill badge ${meta.cssClass}`}>
              <HealthDot status={s} />
              <span>{count} {meta.label}</span>
            </div>
          );
        })}
      </div>

      {/* Dependency Cards Grid */}
      <div className="deps__grid">
        {filtered.map(dep => {
          const meta = getStatusMeta(dep.status);
          const cm = CIRCUIT_META[dep.circuitState];
          return (
            <div
              key={dep.id}
              className="card dep-card"
              onClick={() => navigate(`/dependencies/${dep.id}`)}
              tabIndex={0}
              onKeyDown={e => e.key === 'Enter' && navigate(`/dependencies/${dep.id}`)}
              role="button"
              aria-label={`${dep.name} — ${meta.label}`}
            >
              {/* Card Header */}
              <div className="dep-card__header">
                <div className="flex items-center gap-2 flex-1 min-w-0">
                  <HealthDot status={dep.status} size={10} />
                  <div className="min-w-0">
                    <p className="dep-card__vendor">{dep.vendor.displayName}</p>
                    <p className="dep-card__name truncate">{dep.name}</p>
                  </div>
                </div>
                <ChevronRight size={16} style={{ color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
              </div>

              {/* Status + Circuit */}
              <div className="flex items-center gap-2 mt-3">
                <StatusBadge status={dep.status} size="sm" />
                <span className="mono text-xs font-semibold" style={{ color: cm.color }}>
                  {dep.circuitState.toUpperCase().replace('_', ' ')}
                </span>
              </div>

              {/* Latency */}
              <div className="mt-4">
                <LatencyMetricRow
                  p50={dep.latency.p50}
                  p95={dep.latency.p95}
                  p99={dep.latency.p99}
                  baseline={dep.baselineP95}
                />
              </div>

              {/* Footer */}
              <div className="dep-card__footer">
                <div className="flex items-center gap-1 text-secondary text-xs">
                  <Globe size={11} />
                  {dep.regions.length} regions
                </div>
                <div className="text-xs text-secondary">
                  ERR {formatErrorRate(dep.errorRate)}
                </div>
                <div className="flex items-center gap-1 text-xs text-secondary">
                  <Clock size={11} />
                  {formatRelativeTime(dep.lastProbeAt)}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ─── Dependency Detail View ───────────────────────────────────

function DependencyDetail({ id }: { id: string }) {
  const navigate = useNavigate();
  const { dependency: dep, isLoading } = useDependency(id);
  const [activeTab, setActiveTab] = useState<'overview' | 'latency' | 'regional' | 'timing'>('overview');

  if (isLoading) {
    return (
      <div className="empty-state fade-in">
        <Activity size={40} className="empty-state__icon spin" />
        <h2 className="state-container__title">Loading dependency telemetry...</h2>
      </div>
    );
  }

  if (!dep) return (
    <div className="empty-state fade-in">
      <Activity size={40} className="empty-state__icon" />
      <h2 className="state-container__title">Dependency not found</h2>
      <button className="btn btn-primary" onClick={() => navigate('/dependencies')}>Back to Dependencies</button>
    </div>
  );

  const meta = getStatusMeta(dep.status);
  const cm = CIRCUIT_META[dep.circuitState];

  const chartData = dep.latencyHistory.slice(-24).map(pt => ({
    time: new Date(pt.timestamp).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false }),
    p50: pt.p50, p95: pt.p95, p99: pt.p99, err: pt.errorRate,
  }));

  const regionalData = dep.regions.map(r => {
    const region = KNOWN_REGIONS.find(rg => rg.code === r);
    const latency = dep.regionalLatency[r];
    return {
      name: region?.shortLabel ?? r,
      p50: latency?.p50 ?? 0,
      p95: latency?.p95 ?? 0,
      status: dep.regionalStatus[r] ?? 'unknown',
    };
  });

  const timingData = [
    { name: 'DNS',      value: dep.timing.dns,      color: '#60a5fa' },
    { name: 'TCP',      value: dep.timing.tcp,      color: '#34d399' },
    { name: 'TLS',      value: dep.timing.tls,      color: '#a78bfa' },
    { name: 'TTFB',     value: dep.timing.ttfb,     color: '#f59e0b' },
    { name: 'Transfer', value: dep.timing.transfer, color: '#22d3ee' },
  ];

  return (
    <div className="fade-in">
      {/* Back + Title */}
      <button
        className="btn btn-ghost btn-sm mb-4"
        onClick={() => navigate('/dependencies')}
        aria-label="Back to dependencies"
      >
        <ArrowLeft size={14} /> Dependencies
      </button>

      <div className="dep-detail__header">
        <div className="flex items-center gap-3">
          <HealthDot status={dep.status} size={14} />
          <div>
            <h1 className="page-title">{dep.vendor.displayName}</h1>
            <p className="page-subtitle">{dep.name}</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <StatusBadge status={dep.status} />
          <span className="mono text-sm font-semibold" style={{ color: cm.color }}>
            Circuit: {dep.circuitState.toUpperCase().replace('_', ' ')}
          </span>
        </div>
      </div>

      {/* Metric Strip */}
      <div className="dep-detail__metrics">
        {[
          { label: 'P50', value: formatLatency(dep.latency.p50) },
          { label: 'P95', value: formatLatency(dep.latency.p95), color: dep.latency.p95 > dep.baselineP95 * 3 ? 'var(--color-critical-text)' : dep.latency.p95 > dep.baselineP95 * 1.5 ? 'var(--color-warning-text)' : undefined },
          { label: 'P99', value: formatLatency(dep.latency.p99) },
          { label: 'Error Rate', value: formatErrorRate(dep.errorRate), color: dep.errorRate > 1 ? 'var(--color-critical-text)' : undefined },
          { label: 'Uptime', value: formatUptime(dep.uptime), color: 'var(--color-healthy-text)' },
          { label: 'Baseline P95', value: formatLatency(dep.baselineP95), color: 'var(--color-text-secondary)' },
        ].map(m => (
          <div key={m.label} className="card">
            <p className="metric-label">{m.label}</p>
            <p className="metric-value metric-value--sm mono" style={m.color ? { color: m.color } : undefined}>{m.value}</p>
          </div>
        ))}
      </div>

      {/* Tabs */}
      <div className="dep-detail__tabs">
        {(['overview', 'latency', 'regional', 'timing'] as const).map(tab => (
          <button
            key={tab}
            className={`dep-detail__tab ${activeTab === tab ? 'dep-detail__tab--active' : ''}`}
            onClick={() => setActiveTab(tab)}
          >
            {tab.charAt(0).toUpperCase() + tab.slice(1)}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      {activeTab === 'overview' && (
        <div className="dep-detail__tab-content fade-in">
          <div className="grid-2 gap-4">
            <div className="card">
              <p className="section-heading mb-3">Endpoint</p>
              <code className="code-label" style={{ wordBreak: 'break-all', display: 'block' }}>{dep.endpoint}</code>
              <div className="mt-4 flex flex-col gap-2">
                {dep.tags.map(tag => (
                  <span key={tag} className="tag" style={{ display: 'inline-block', width: 'fit-content' }}>#{tag}</span>
                ))}
              </div>
            </div>
            <div className="card">
              <p className="section-heading mb-3">Vendor Status</p>
              <div className="flex items-center gap-2">
                <span className={`badge ${dep.vendor.vendorReportedStatus === 'operational' ? 'status-healthy' : 'status-degraded'}`}>
                  Vendor: {dep.vendor.vendorReportedStatus.replace('_', ' ').toUpperCase()}
                </span>
              </div>
              <p className="text-xs text-secondary mt-2">
                Last checked {formatRelativeTime(dep.vendor.vendorStatusUpdatedAt)}
              </p>
              {dep.vendor.statusPageUrl && (
                <a href={dep.vendor.statusPageUrl} target="_blank" rel="noopener noreferrer" className="text-xs text-accent mt-2 block">
                  {dep.vendor.statusPageUrl} ↗
                </a>
              )}
            </div>
          </div>
        </div>
      )}

      {activeTab === 'latency' && (
        <div className="card dep-detail__tab-content fade-in">
          <p className="section-heading mb-4">Latency History — Last 24h</p>
          <div style={{ height: 280 }}>
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData} margin={{ top: 4, right: 8, left: -10, bottom: 0 }}>
                <defs>
                  <linearGradient id="p95GradDep" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.25} />
                    <stop offset="95%" stopColor="#f59e0b" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                <XAxis dataKey="time" tick={{ fill: 'var(--color-text-tertiary)', fontSize: 10 }} tickLine={false} axisLine={false} />
                <YAxis tickFormatter={v => formatLatency(v)} tick={{ fill: 'var(--color-text-tertiary)', fontSize: 10 }} tickLine={false} axisLine={false} />
                <Tooltip content={<CustomTooltip />} />
                <Area type="monotone" dataKey="p50" name="P50" stroke="#60a5fa" strokeWidth={1.5} fill="none" dot={false} />
                <Area type="monotone" dataKey="p95" name="P95" stroke="#f59e0b" strokeWidth={2} fill="url(#p95GradDep)" dot={false} />
                <Area type="monotone" dataKey="p99" name="P99" stroke="#ef4444" strokeWidth={1.5} fill="none" dot={false} />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}

      {activeTab === 'regional' && (
        <div className="dep-detail__tab-content fade-in">
          <div className="card">
            <p className="section-heading mb-4">Regional P95 Comparison</p>
            <div style={{ height: 240 }}>
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={regionalData} margin={{ top: 4, right: 8, left: -10, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                  <XAxis dataKey="name" tick={{ fill: 'var(--color-text-tertiary)', fontSize: 11 }} tickLine={false} axisLine={false} />
                  <YAxis tickFormatter={v => formatLatency(v)} tick={{ fill: 'var(--color-text-tertiary)', fontSize: 11 }} tickLine={false} axisLine={false} />
                  <Tooltip content={<CustomTooltip />} />
                  <Bar dataKey="p95" name="P95" fill="#f59e0b" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>
          <div className="flex flex-col gap-2 mt-4">
            {regionalData.map(r => {
              const s = r.status as HealthStatus;
              const m = getStatusMeta(s);
              return (
                <div key={r.name} className="card flex items-center justify-between" style={{ padding: 'var(--space-3) var(--space-4)' }}>
                  <div className="flex items-center gap-2">
                    <HealthDot status={s} />
                    <span className="text-sm text-primary font-medium">{r.name}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="mono text-sm">P95 {formatLatency(r.p95)}</span>
                    <StatusBadge status={s} size="sm" />
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {activeTab === 'timing' && (
        <div className="dep-detail__tab-content fade-in">
          <div className="card">
            <p className="section-heading mb-4">Request Timing Breakdown</p>
            <div className="timing-breakdown">
              {timingData.map(t => (
                <div key={t.name} className="timing-row">
                  <span className="timing-row__label">{t.name}</span>
                  <div className="timing-row__bar-wrap">
                    <div
                      className="timing-row__bar"
                      style={{
                        width: `${Math.min((t.value / dep.timing.total) * 100, 100)}%`,
                        background: t.color,
                      }}
                    />
                  </div>
                  <span className="timing-row__value mono">{t.value}ms</span>
                </div>
              ))}
              <div className="timing-row timing-row--total">
                <span className="timing-row__label">Total</span>
                <div className="timing-row__bar-wrap" />
                <span className="timing-row__value mono">{dep.timing.total}ms</span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
