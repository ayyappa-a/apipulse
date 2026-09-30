import React, { useState } from 'react';
import {
  Globe, Radio, Play, Pause, Plus, RefreshCw,
  CheckCircle, AlertCircle, Clock, Shield, ArrowUpRight,
  Server, Sliders, ExternalLink, X, Zap
} from 'lucide-react';
import { useProbes } from '../hooks/useProbes';
import { useDependencies } from '../hooks/useDependencies';
import { ProbeWaterfall } from '../features/probes/ProbeWaterfall';
import { KNOWN_REGIONS } from '../services/telemetryService';
import { formatLatency } from '../utils/status';
import type { ProbeConfig, HttpMethod, RegionCode, ProbeExecutionResult } from '../types/domain';
import { useToast } from '../context/AppContext';
import './Probes.css';

export default function Probes() {
  const { addToast } = useToast();
  const { probes: probesList, activeCount, toggleStatus, runProbe, createProbe } = useProbes();
  const { dependencies } = useDependencies();
  const [selectedProbe, setSelectedProbe] = useState<ProbeConfig | null>(null);
  const [executing, setExecuting] = useState(false);
  const [executionResult, setExecutionResult] = useState<ProbeExecutionResult | null>(null);
  const [testRegion, setTestRegion] = useState<RegionCode>('ap-south-1');
  const [isNewProbeModalOpen, setIsNewProbeModalOpen] = useState(false);

  // Form state for new probe modal
  const [newProbeName, setNewProbeName] = useState('');
  const [newProbeDep, setNewProbeDep] = useState(dependencies[0]?.id || 'dep-stripe-payments');
  const [newProbeUrl, setNewProbeUrl] = useState('https://api.example.com/v1/health');
  const [newProbeMethod, setNewProbeMethod] = useState<HttpMethod>('GET');
  const [newProbeInterval, setNewProbeInterval] = useState(30);

  const toggleProbeStatus = async (id: string) => {
    const updated = await toggleStatus(id);
    addToast(`Probe "${updated.name}" is now ${updated.status}.`, 'info');
  };

  const runTestProbe = async (probe: ProbeConfig) => {
    setSelectedProbe(probe);
    setExecuting(true);
    setExecutionResult(null);

    try {
      const res = await runProbe(probe.id, testRegion);
      setExecutionResult(res);
      addToast(`Probe execution completed from ${testRegion}: ${res.total}ms`, 'success');
    } catch (err: any) {
      addToast(err?.message || 'Probe execution failed', 'error');
    } finally {
      setExecuting(false);
    }
  };

  const handleCreateProbe = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newProbeName.trim()) return;

    await createProbe({
      name: newProbeName,
      dependencyId: newProbeDep || dependencies[0]?.id || 'dep-stripe-payments',
      endpoint: newProbeUrl,
      method: newProbeMethod,
      headers: { 'User-Agent': 'APIPulse-EdgeProbe/2.0' },
      intervalSeconds: newProbeInterval,
      timeoutMs: 5000,
      regions: ['us-east-1', 'eu-west-1', 'ap-south-1'],
      expectedStatusCodes: [200],
      followRedirects: true,
      authType: 'none',
      thresholds: { degradedP95Ms: 450, criticalP95Ms: 1800, errorRatePercent: 1.0 },
      status: 'active',
    });

    setIsNewProbeModalOpen(false);
    setNewProbeName('');
    addToast(`New synthetic probe "${newProbeName}" created and dispatched to edge fleet.`, 'success');
  };

  const activeProbes = probesList.filter(p => p.status === 'active').length;

  return (
    <div className="probes-page">
      {/* Header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">Synthetic Outbound Probes</h1>
          <p className="page-subtitle">
            Distributed continuous edge probes measuring TCP, TLS, and TTFB latencies across 8 global nodes.
          </p>
        </div>
        <div className="probes-header-actions">
          <button
            className="btn btn--primary"
            onClick={() => setIsNewProbeModalOpen(true)}
          >
            <Plus size={16} />
            <span>Configure New Probe</span>
          </button>
        </div>
      </div>

      {/* KPI Stats */}
      <div className="probes-stats-grid">
        <div className="card probes-stat-card">
          <div className="probes-stat-icon probes-stat-icon--active">
            <Radio size={20} />
          </div>
          <div>
            <div className="probes-stat-value">{activeProbes} / {probesList.length}</div>
            <div className="probes-stat-label">Active Probes</div>
          </div>
        </div>

        <div className="card probes-stat-card">
          <div className="probes-stat-icon probes-stat-icon--nodes">
            <Globe size={20} />
          </div>
          <div>
            <div className="probes-stat-value">{KNOWN_REGIONS.length} Edge Points</div>
            <div className="probes-stat-label">Global Fleet Coverage</div>
          </div>
        </div>

        <div className="card probes-stat-card">
          <div className="probes-stat-icon probes-stat-icon--cadence">
            <Clock size={20} />
          </div>
          <div>
            <div className="probes-stat-value">180 / min</div>
            <div className="probes-stat-label">Probe Execution Frequency</div>
          </div>
        </div>

        <div className="card probes-stat-card">
          <div className="probes-stat-icon probes-stat-icon--shield">
            <Shield size={20} />
          </div>
          <div>
            <div className="probes-stat-value">100% Zero-Leak</div>
            <div className="probes-stat-label">Server-side Vault Isolation</div>
          </div>
        </div>
      </div>

      {/* Probes Table / Grid */}
      <div className="card probes-table-card">
        <div className="probes-table-header">
          <h2 className="section-title">Configured Probes ({probesList.length})</h2>
          <span className="probes-table-note">
            Sub-second telemetry dispatched every 30s to 120s
          </span>
        </div>

        <div className="probes-table-wrap">
          <table className="table probes-table">
            <thead>
              <tr>
                <th>Probe Name & Endpoint</th>
                <th>Method</th>
                <th>Interval</th>
                <th>Target Regions</th>
                <th>Latency Thresholds</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {probesList.map(probe => {
                const dep = dependencies.find(d => d.id === probe.dependencyId);
                return (
                  <tr key={probe.id}>
                    <td>
                      <div className="probe-name-cell">
                        <div className="probe-title-row">
                          <span className="probe-name">{probe.name}</span>
                          {dep && <span className="probe-vendor-badge">{dep.vendor.displayName}</span>}
                        </div>
                        <code className="probe-endpoint">{probe.endpoint}</code>
                      </div>
                    </td>
                    <td>
                      <span className={`method-badge method-badge--${probe.method.toLowerCase()}`}>
                        {probe.method}
                      </span>
                    </td>
                    <td>
                      <span className="probe-interval">{probe.intervalSeconds}s</span>
                    </td>
                    <td>
                      <div className="probe-regions-list">
                        {probe.regions.map(r => (
                          <span key={r} className="probe-region-pill" title={r}>
                            {r.split('-')[0]}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td>
                      <div className="probe-thresholds-cell">
                        <span className="threshold-pill threshold-pill--warn">
                          P95 &gt; {formatLatency(probe.thresholds.degradedP95Ms)}
                        </span>
                        <span className="threshold-pill threshold-pill--crit">
                          P95 &gt; {formatLatency(probe.thresholds.criticalP95Ms)}
                        </span>
                      </div>
                    </td>
                    <td>
                      <span className={`badge ${probe.status === 'active' ? 'badge--success' : 'badge--neutral'}`}>
                        {probe.status === 'active' ? 'ACTIVE' : 'PAUSED'}
                      </span>
                    </td>
                    <td>
                      <div className="probe-actions-cell">
                        <button
                          className="btn btn--outline btn--sm"
                          onClick={() => runTestProbe(probe)}
                          title="Run immediate synthetic test probe"
                        >
                          <Play size={12} />
                          <span>Test</span>
                        </button>
                        <button
                          className="btn btn--ghost btn--sm"
                          onClick={() => toggleProbeStatus(probe.id)}
                          title={probe.status === 'active' ? 'Pause probe' : 'Resume probe'}
                        >
                          {probe.status === 'active' ? <Pause size={14} /> : <Play size={14} />}
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {/* Synthetic Probe Diagnostic Runner Drawer / Modal */}
      {selectedProbe && (
        <div className="probe-tester-overlay" onClick={() => !executing && setSelectedProbe(null)}>
          <div className="card probe-tester-modal" onClick={e => e.stopPropagation()}>
            <div className="probe-tester-header">
              <div className="probe-tester-title-group">
                <Radio size={20} className="text-primary" />
                <div>
                  <h3 className="probe-tester-title">Live Synthetic Probe Diagnostic</h3>
                  <p className="probe-tester-sub">
                    Direct execution from APIPulse edge worker to <code>{selectedProbe.endpoint}</code>
                  </p>
                </div>
              </div>
              <button
                className="btn btn--ghost btn--sm"
                onClick={() => setSelectedProbe(null)}
                disabled={executing}
              >
                <X size={18} />
              </button>
            </div>

            <div className="probe-tester-controls">
              <div className="probe-tester-control-group">
                <label className="probe-tester-label">Origin Probe Region:</label>
                <select
                  className="input probe-tester-select"
                  value={testRegion}
                  onChange={e => setTestRegion(e.target.value as RegionCode)}
                  disabled={executing}
                >
                  {KNOWN_REGIONS.map(r => (
                    <option key={r.code} value={r.code}>
                      {r.label} ({r.code})
                    </option>
                  ))}
                </select>
              </div>

              <button
                className="btn btn--primary"
                onClick={() => runTestProbe(selectedProbe)}
                disabled={executing}
              >
                {executing ? (
                  <>
                    <RefreshCw size={14} className="spin" />
                    <span>Executing Outbound Probe...</span>
                  </>
                ) : (
                  <>
                    <Zap size={14} />
                    <span>Trigger Outbound Probe</span>
                  </>
                )}
              </button>
            </div>

            {/* Execution Result Waterfall */}
            {executionResult && (
              <div className="probe-result-box">
                <div className="probe-result-summary">
                  <div className="result-metric">
                    <span className="result-label">Status</span>
                    <span className="result-val text-success">
                      {executionResult.statusCode} OK
                    </span>
                  </div>
                  <div className="result-metric">
                    <span className="result-label">Total Latency</span>
                    <span className="result-val text-primary font-mono">
                      {formatLatency(executionResult.total)}
                    </span>
                  </div>
                  <div className="result-metric">
                    <span className="result-label">Resolved IP</span>
                    <span className="result-val font-mono">{executionResult.ipAddress}</span>
                  </div>
                  <div className="result-metric">
                    <span className="result-label">TLS Cipher</span>
                    <span className="result-val font-mono">{executionResult.certIssuer}</span>
                  </div>
                </div>

                <ProbeWaterfall result={executionResult} />
              </div>
            )}
          </div>
        </div>
      )}

      {/* New Probe Configuration Modal */}
      {isNewProbeModalOpen && (
        <div className="probe-tester-overlay" onClick={() => setIsNewProbeModalOpen(false)}>
          <div className="card probe-modal" onClick={e => e.stopPropagation()}>
            <div className="probe-modal-header">
              <h3 className="section-title">Configure Synthetic Outbound Probe</h3>
              <button className="btn btn--ghost btn--sm" onClick={() => setIsNewProbeModalOpen(false)}>
                <X size={18} />
              </button>
            </div>

            <form onSubmit={handleCreateProbe} className="probe-form">
              <div className="probe-form-group">
                <label className="probe-form-label">Probe Friendly Name</label>
                <input
                  type="text"
                  className="input"
                  placeholder="e.g. OpenAI Models API Health"
                  value={newProbeName}
                  onChange={e => setNewProbeName(e.target.value)}
                  required
                />
              </div>

              <div className="probe-form-row">
                <div className="probe-form-group">
                  <label className="probe-form-label">Target Dependency</label>
                  <select
                    className="input"
                    value={newProbeDep}
                    onChange={e => setNewProbeDep(e.target.value)}
                  >
                    {dependencies.map(d => (
                      <option key={d.id} value={d.id}>
                        {d.vendor.displayName} — {d.name}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="probe-form-group">
                  <label className="probe-form-label">HTTP Method</label>
                  <select
                    className="input"
                    value={newProbeMethod}
                    onChange={e => setNewProbeMethod(e.target.value as HttpMethod)}
                  >
                    <option value="GET">GET</option>
                    <option value="POST">POST</option>
                    <option value="HEAD">HEAD</option>
                  </select>
                </div>
              </div>

              <div className="probe-form-group">
                <label className="probe-form-label">Probe Target Endpoint URL</label>
                <input
                  type="url"
                  className="input font-mono"
                  placeholder="https://api.vendor.com/v1/health"
                  value={newProbeUrl}
                  onChange={e => setNewProbeUrl(e.target.value)}
                  required
                />
              </div>

              <div className="probe-form-row">
                <div className="probe-form-group">
                  <label className="probe-form-label">Interval (seconds)</label>
                  <select
                    className="input"
                    value={newProbeInterval}
                    onChange={e => setNewProbeInterval(Number(e.target.value))}
                  >
                    <option value={15}>15 seconds (High Frequency)</option>
                    <option value={30}>30 seconds (Standard)</option>
                    <option value={60}>60 seconds (Relaxed)</option>
                    <option value={120}>120 seconds (Low Traffic)</option>
                  </select>
                </div>

                <div className="probe-form-group">
                  <label className="probe-form-label">Auth Handling</label>
                  <input
                    type="text"
                    className="input"
                    disabled
                    value="Server Vault Isolated (Zero Frontend Storage)"
                  />
                </div>
              </div>

              <div className="probe-modal-footer">
                <button
                  type="button"
                  className="btn btn--outline"
                  onClick={() => setIsNewProbeModalOpen(false)}
                >
                  Cancel
                </button>
                <button type="submit" className="btn btn--primary">
                  Deploy Probe to Edge
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
