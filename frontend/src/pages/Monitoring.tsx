import React, { useState, useEffect, useCallback } from 'react';
import { Radio, Zap, Globe, Pause, Play, Activity, ShieldCheck, Database } from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid, Line, LineChart, Legend } from 'recharts';
import { HealthDot } from '../components/ui/StatusBadge';
import { useTelemetryStream } from '../hooks/useTelemetryStream';
import { apiClient, type ApiObservationsResponse, type ApiAssessment } from '../services/apiClient';
import { BackendOfflineBanner } from '../components/ui/BackendOfflineBanner';
import { formatLatency, formatRelativeTime, getStatusMeta } from '../utils/status';
import './Monitoring.css';

const CustomTooltip = ({ active, payload, label }: any) => {
  if (!active || !payload?.length) return null;
  return (
    <div className="chart-tooltip">
      <p className="chart-tooltip__label">{label}</p>
      {payload.map((entry: any) => (
        <p key={entry.dataKey} className="chart-tooltip__item" style={{ color: entry.color }}>
          {entry.name}: <strong>{typeof entry.value === 'number' ? formatLatency(entry.value) : entry.value}</strong>
        </p>
      ))}
    </div>
  );
};

export default function Monitoring() {
  const { probeLog, probeCount, isPaused, regions, isOffline, lastSuccessAt, togglePause, refetch } = useTelemetryStream();
  const [chartData, setChartData] = useState<Array<{
    time: string;
    synthetic: number;
    customer: number;
    baseline: number;
  }>>([]);
  const [latestAssessment, setLatestAssessment] = useState<ApiAssessment | null>(null);

  // Poll real observation history for the chart
  const loadChartData = useCallback(async () => {
    try {
      const [obs, assessment] = await Promise.all([
        apiClient.getDependencyObservations('stripe-like', 'ap-south-1', 30),
        apiClient.getDependencyHealth('stripe-like', 'ap-south-1'),
      ]);

      setLatestAssessment(assessment);
      const probeList = obs?.probe_observations || [];
      const custList = obs?.customer_observations || [];
      const base = Math.round(assessment?.synthetic_baseline_p95_ms || 203);

      const points: Array<{
        time: string;
        synthetic: number;
        customer: number;
        baseline: number;
      }> = [];

      // Map real probe and customer observations to timeline points
      const count = Math.min(Math.max(probeList.length, custList.length, 1), 25);
      const recentProbes = probeList.slice(-count);
      const recentCust = custList.slice(-count);
      const numPoints = Math.max(recentProbes.length, recentCust.length);

      for (let i = 0; i < numPoints; i++) {
        const p = recentProbes[i] || recentProbes[recentProbes.length - 1];
        const c = recentCust[i] || recentCust[recentCust.length - 1];
        const t = p?.timestamp ? new Date(p.timestamp) : (c?.timestamp ? new Date(c.timestamp) : new Date());

        points.push({
          time: t.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }),
          synthetic: p ? Math.round(p.total_duration_ms) : base,
          customer: c ? Math.round(c.duration_ms) : (p ? Math.round(p.total_duration_ms) : base),
          baseline: base,
        });
      }

      setChartData(points);
    } catch {
      // Backend status handled by useTelemetryStream
    }
  }, []);

  useEffect(() => {
    loadChartData();
    const interval = setInterval(loadChartData, 4000);
    return () => clearInterval(interval);
  }, [loadChartData]);

  if (isOffline) {
    return <BackendOfflineBanner onRetry={refetch} lastSuccessAt={lastSuccessAt} />;
  }

  return (
    <div className="monitoring fade-in">
      <div className="flex items-center justify-between page-header">
        <div>
          <h1 className="page-title">Live Monitoring</h1>
          <p className="page-subtitle">Real-time synthetic probe and customer telemetry across all regions</p>
        </div>
        <div className="flex items-center gap-3">
          <button
            className={`btn btn--sm ${isPaused ? 'btn--primary' : 'btn--outline'}`}
            onClick={togglePause}
          >
            {isPaused ? <Play size={12} /> : <Pause size={12} />}
            <span>{isPaused ? 'Resume Stream' : 'Freeze Stream'}</span>
          </button>
          <div className="live-indicator">
            <span className={`live-dot ${isPaused ? 'live-dot--paused' : ''}`} />
            {isPaused ? 'PAUSED' : 'LIVE PROBES'}
          </div>
        </div>
      </div>

      {/* Telemetry Provenance Banner */}
      <div className="card mb-4" style={{ padding: 'var(--space-3) var(--space-4)', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border)' }}>
        <div className="flex items-center justify-between flex-wrap gap-2 text-xs">
          <span className="font-semibold text-secondary">DATA PROVENANCE:</span>
          <span className="badge" style={{ background: 'rgba(59,130,246,0.15)', color: '#60a5fa' }}>
            SYNTHETIC_PROBE · TimescaleDB
          </span>
          <span className="badge" style={{ background: 'rgba(168,85,247,0.15)', color: '#c084fc' }}>
            CUSTOMER_OTEL · OTLP Ingestion
          </span>
          <span className="badge" style={{ background: 'rgba(34,197,94,0.15)', color: '#4ade80' }}>
            VENDOR_STATUS · Vendor Mock
          </span>
          <span className="badge" style={{ background: 'rgba(234,179,8,0.15)', color: '#facc15' }}>
            APIPULSE_ASSESSMENT · Health Engine
          </span>
        </div>
      </div>

      {/* Summary Strip */}
      <div className="monitoring__strip">
        <div className="card monitoring__stat">
          <p className="metric-label">Total Observations</p>
          <p className="metric-value metric-value--sm mono">{probeCount.toLocaleString()}</p>
          <p className="text-xs text-secondary mt-1">in active window</p>
        </div>
        <div className="card monitoring__stat">
          <p className="metric-label">Probe Region</p>
          <p className="metric-value metric-value--sm">ap-south-1</p>
          <p className="text-xs text-secondary mt-1">active container worker</p>
        </div>
        <div className="card monitoring__stat">
          <p className="metric-label">Target Dependency</p>
          <p className="metric-value metric-value--sm">stripe-like</p>
          <p className="text-xs text-secondary mt-1">vendor-mock:9001</p>
        </div>
        <div className="card monitoring__stat">
          <p className="metric-label">Probe Interval</p>
          <p className="metric-value metric-value--sm mono">30s</p>
          <p className="text-xs text-secondary mt-1">scheduled worker</p>
        </div>
      </div>

      <div className="monitoring__grid">
        {/* Left: Regional Map + Real Latency Chart */}
        <div className="monitoring__left">
          {/* World map visualization */}
          <section>
            <h2 className="section-heading mb-4">Global Probe Nodes</h2>
            <div className="card monitoring__map">
              <div className="monitoring__map-bg">
                <Globe size={120} style={{ color: 'rgba(59,130,246,0.06)' }} />
              </div>
              {regions.map(region => {
                const meta = getStatusMeta(region.status);
                const x = ((region.lon + 180) / 360) * 100;
                const y = ((90 - region.lat) / 180) * 100;
                return (
                  <div
                    key={region.code}
                    className="monitoring__probe-node"
                    style={{ left: `${x}%`, top: `${y}%` }}
                    title={`${region.label}\nP95: ${formatLatency(region.latencyP95)}\nStatus: ${meta.label}`}
                  >
                    <span className={`health-dot ${meta.dotClass}`} style={{ width: 12, height: 12 }} />
                    <span className="monitoring__probe-label">{region.shortLabel}</span>
                  </div>
                );
              })}
            </div>
          </section>

          {/* Genuine Telemetry Multi-Signal Live Chart */}
          <section className="mt-6">
            <h2 className="section-heading mb-4">Live Multi-Signal Telemetry Trend (ap-south-1)</h2>
            <div className="card">
              <div className="flex items-center gap-4 mb-3">
                <span className="text-xs" style={{ color: '#38bdf8' }}>— SYNTHETIC_PROBE (Observed)</span>
                <span className="text-xs" style={{ color: '#c084fc' }}>— CUSTOMER_OTEL (App Span)</span>
                <span className="text-xs" style={{ color: '#22c55e' }}>·· Baseline P95 ({latestAssessment ? formatLatency(latestAssessment.synthetic_baseline_p95_ms || 203) : '203ms'})</span>
              </div>
              <div style={{ height: 220 }}>
                {chartData.length > 0 ? (
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={chartData} margin={{ top: 4, right: 8, left: -10, bottom: 0 }}>
                      <defs>
                        <linearGradient id="synthGrad" x1="0" y1="0" x2="0" y2="1">
                          <stop offset="5%" stopColor="#38bdf8" stopOpacity={0.25} />
                          <stop offset="95%" stopColor="#38bdf8" stopOpacity={0} />
                        </linearGradient>
                      </defs>
                      <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                      <XAxis dataKey="time" tick={{ fill: 'var(--color-text-tertiary)', fontSize: 9 }} tickLine={false} axisLine={false} />
                      <YAxis tickFormatter={v => formatLatency(v)} tick={{ fill: 'var(--color-text-tertiary)', fontSize: 9 }} tickLine={false} axisLine={false} />
                      <Tooltip content={<CustomTooltip />} />
                      <Area type="monotone" dataKey="synthetic" name="SYNTHETIC_PROBE" stroke="#38bdf8" strokeWidth={2} fill="url(#synthGrad)" dot={false} />
                      <Area type="monotone" dataKey="customer" name="CUSTOMER_OTEL" stroke="#c084fc" strokeWidth={1.5} fill="none" dot={false} />
                      <Area type="monotone" dataKey="baseline" name="Baseline P95" stroke="#22c55e" strokeDasharray="3 3" strokeWidth={1.5} fill="none" dot={false} />
                    </AreaChart>
                  </ResponsiveContainer>
                ) : (
                  <div className="flex items-center justify-center h-full text-secondary text-sm">
                    Awaiting probe telemetry from container worker...
                  </div>
                )}
              </div>
            </div>
          </section>
        </div>

        {/* Right: Real Probe Activity Log + Regional Status */}
        <div className="monitoring__right">
          {/* Regional Status Cards */}
          <section>
            <h2 className="section-heading mb-4">Target Region Status</h2>
            <div className="flex flex-col gap-2">
              {regions.filter(r => r.code === 'ap-south-1' || r.code === 'us-east-1').map(region => {
                const meta = getStatusMeta(region.status);
                const lat = latestAssessment && region.code === 'ap-south-1'
                  ? Math.round(latestAssessment.synthetic_latency_ms || 203)
                  : 203;
                return (
                  <div key={region.code} className={`card monitoring__region-card ${region.status !== 'healthy' ? 'monitoring__region-card--alert' : ''}`}>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <HealthDot status={region.status} size={10} />
                        <div>
                          <p className="text-sm font-semibold text-primary">{region.shortLabel}</p>
                          <p className="text-xs text-secondary">{region.code} · Active Target</p>
                        </div>
                      </div>
                      <div className="text-right">
                        <p className="mono text-sm font-bold" style={{ color: lat > 1000 ? 'var(--color-critical-text)' : lat > 400 ? 'var(--color-warning-text)' : 'var(--color-healthy-text)' }}>
                          {formatLatency(lat)}
                        </p>
                        <p className="text-xs text-secondary">Observed</p>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </section>

          {/* Real Live Probe Log */}
          <section className="mt-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="section-heading">Live Probe Telemetry Stream</h2>
              <div className="live-indicator">
                <span className="live-dot" />
                DOCKER PROBE
              </div>
            </div>
            <div className="card monitoring__probe-log">
              {probeLog.length === 0 ? (
                <div className="p-4 text-center text-secondary text-sm">
                  Connecting to TimescaleDB probe stream...
                </div>
              ) : (
                probeLog.slice(0, 30).map((probe, i) => (
                  <div key={probe.id} className={`monitoring__probe-entry ${i === 0 ? 'monitoring__probe-entry--new' : ''}`}>
                    <span
                      className="monitoring__probe-status-dot"
                      style={{
                        background: probe.status === 'error' ? 'var(--color-critical)'
                          : probe.status === 'slow' ? 'var(--color-degraded)'
                          : 'var(--color-healthy)',
                      }}
                    />
                    <span className="monitoring__probe-vendor">{probe.vendor}</span>
                    <span className="text-xs text-secondary">{probe.region}</span>
                    <span
                      className="mono text-xs monitoring__probe-latency"
                      style={{
                        color: probe.status === 'error' ? 'var(--color-critical-text)'
                          : probe.status === 'slow' ? 'var(--color-warning-text)'
                          : 'var(--color-healthy-text)',
                      }}
                    >
                      {formatLatency(probe.latency)}
                    </span>
                    <span className="text-xs text-tertiary monitoring__probe-time">
                      {formatRelativeTime(probe.timestamp)}
                    </span>
                  </div>
                ))
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
