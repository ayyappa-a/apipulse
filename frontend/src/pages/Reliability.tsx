import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { ShieldCheck, AlertTriangle, TrendingUp, ChevronRight, Activity, CheckCircle, RefreshCw } from 'lucide-react';
import { HealthDot, StatusBadge } from '../components/ui/StatusBadge';
import { useDependencies } from '../hooks/useDependencies';
import { useIncidents } from '../hooks/useIncidents';
import { apiClient, type ApiAssessment } from '../services/apiClient';
import { BackendOfflineBanner } from '../components/ui/BackendOfflineBanner';
import { formatLatency, getStatusMeta } from '../utils/status';
import type { HealthStatus } from '../types/domain';
import './Reliability.css';

export default function Reliability() {
  const navigate = useNavigate();
  const { dependencies, isOffline: depsOffline, lastSuccessAt, refetch: refetchDeps } = useDependencies();
  const { incidents, isOffline: incsOffline, refetch: refetchIncidents } = useIncidents();
  const [assessment, setAssessment] = useState<ApiAssessment | null>(null);
  const [loadingAssessment, setLoadingAssessment] = useState(true);
  const [isAssessmentOffline, setIsAssessmentOffline] = useState(false);

  const fetchAssessment = async () => {
    try {
      const a = await apiClient.getDependencyHealth('stripe-like', 'ap-south-1');
      setAssessment(a);
      setIsAssessmentOffline(false);
    } catch {
      setIsAssessmentOffline(true);
    } finally {
      setLoadingAssessment(false);
    }
  };

  useEffect(() => {
    fetchAssessment();
    const interval = setInterval(fetchAssessment, 4000);
    return () => clearInterval(interval);
  }, []);

  const isOffline = depsOffline || incsOffline || isAssessmentOffline;

  const handleRetry = () => {
    refetchDeps();
    refetchIncidents();
    fetchAssessment();
  };

  if (isOffline) {
    return <BackendOfflineBanner onRetry={handleRetry} lastSuccessAt={lastSuccessAt} />;
  }

  // Derive overall status from real dependencies
  const isGreyFailure = assessment?.status === 'GREY_FAILURE' || dependencies.some(d => d.status === 'grey_failure' || d.status === 'regional_degradation');
  const isDegraded = assessment?.status === 'DEGRADED' || dependencies.some(d => d.status === 'degraded');
  const isRecovering = assessment?.status === 'RECOVERING' || dependencies.some(d => d.status === 'recovering');

  const overallStatus: HealthStatus = isGreyFailure
    ? 'grey_failure'
    : isDegraded
    ? 'degraded'
    : isRecovering
    ? 'recovering'
    : 'healthy';

  const meta = getStatusMeta(overallStatus);

  // Active incidents related to stripe-like
  const activeIncidents = incidents.filter(i => i.dependencyId === 'stripe-like' && i.status !== 'resolved');

  const observedLat = Math.round(assessment?.synthetic_latency_ms || assessment?.customer_latency_ms || 0);
  const baselineLat = Math.round(assessment?.synthetic_baseline_p95_ms || assessment?.customer_baseline_p95_ms || 203);
  const multiplier = assessment?.latency_multiplier ? assessment.latency_multiplier.toFixed(2) : '1.00';

  return (
    <div className="reliability fade-in">
      <div className="page-header">
        <div>
          <h1 className="page-title">Reliability & Health Intelligence</h1>
          <p className="page-subtitle">
            Autonomous multi-signal health classification, tail-latency anomaly detection, and circuit control
          </p>
        </div>
      </div>

      {/* Overall Health Classification */}
      <div className={`reliability__classification reliability__classification--${overallStatus}`}>
        <div className="reliability__class-header">
          <div className="flex items-center gap-3">
            <HealthDot status={overallStatus} size={14} />
            <div>
              <p className="reliability__class-label">APIPULSE PLATFORM HEALTH CLASSIFICATION</p>
              <p className="reliability__class-value">{assessment?.status || meta.label.toUpperCase()}</p>
            </div>
          </div>
          <ShieldCheck size={36} style={{ opacity: 0.3 }} />
        </div>
        <p className="reliability__class-desc">
          {assessment?.explanation || meta.description}
        </p>
      </div>

      {/* Real Assessment Intelligence Analysis */}
      <div className="reliability__section">
        <div className="flex items-center justify-between mb-4">
          <h2 className="section-heading">
            Live Health Evaluation — stripe-like (ap-south-1)
          </h2>
          <span className="badge" style={{ background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border)' }}>
            SOURCE: APIPULSE_ASSESSMENT
          </span>
        </div>

        <div className="reliability__analysis-grid">
          {/* Classification Evidence Card */}
          <div className="card reliability__analysis-card">
            <div className="flex items-center gap-2 mb-4">
              {isGreyFailure ? (
                <AlertTriangle size={18} style={{ color: 'var(--color-warning-text)' }} />
              ) : isRecovering ? (
                <Activity size={18} style={{ color: '#60a5fa' }} />
              ) : (
                <CheckCircle size={18} style={{ color: 'var(--color-healthy-text)' }} />
              )}
              <span className="font-semibold" style={{
                color: isGreyFailure ? 'var(--color-warning-text)' : isRecovering ? '#60a5fa' : 'var(--color-healthy-text)'
              }}>
                {assessment?.status || 'HEALTHY'}
              </span>
            </div>

            <div className="reliability__evidence">
              <p className="reliability__evidence-title">Telemetry Signal Evidence</p>
              {[
                { label: 'Synthetic Observed P95', value: formatLatency(observedLat), bad: observedLat > baselineLat * 2 },
                { label: 'Synthetic Baseline P95', value: formatLatency(baselineLat), bad: false },
                { label: 'Latency Multiplier', value: `${multiplier}×`, bad: parseFloat(multiplier) > 2.0 },
                { label: 'Confidence Score', value: assessment?.confidence || 'HIGH', bad: false },
                { label: 'Synthetic Signal', value: assessment?.synthetic_signal || 'HEALTHY', bad: assessment?.synthetic_signal === 'DEGRADED' },
                { label: 'Customer Signal', value: assessment?.customer_signal || 'INSUFFICIENT_DATA', bad: assessment?.customer_signal === 'DEGRADED' },
                { label: 'Sample Count (Synthetic)', value: `${assessment?.synthetic_sample_count ?? 0} samples`, bad: false },
              ].map(e => (
                <div key={e.label} className="reliability__evidence-row">
                  <span className="reliability__evidence-label">{e.label}</span>
                  <span
                    className="mono font-bold text-sm"
                    style={{ color: e.bad ? 'var(--color-critical-text)' : 'var(--color-healthy-text)' }}
                  >
                    {e.value}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Vendor Correlation Card */}
          <div className="card reliability__analysis-card">
            <p className="section-heading mb-4">Vendor Status Cross-Verification</p>
            <div className="reliability__vendor-status">
              <div className="reliability__vendor-row">
                <span className="text-secondary text-sm">Vendor reported status</span>
                <span className={`badge ${assessment?.vendor_status?.toLowerCase() === 'operational' ? 'status-healthy' : 'status-critical'}`}>
                  {assessment?.vendor_status || 'OPERATIONAL'}
                </span>
              </div>
              <div className="reliability__vendor-row mt-3">
                <span className="text-secondary text-sm">APIPulse assessment</span>
                <span className={`badge ${isGreyFailure ? 'status-regional' : 'status-healthy'}`}>
                  {assessment?.status || 'HEALTHY'}
                </span>
              </div>

              {isGreyFailure ? (
                <div className="reliability__vendor-discrepancy mt-3">
                  <AlertTriangle size={14} style={{ color: 'var(--color-warning-text)' }} />
                  <p className="text-sm" style={{ color: 'var(--color-warning-text)' }}>
                    Vendor status page reports Operational while independent synthetic probes observe elevated latency. Grey-failure pattern confirmed.
                  </p>
                </div>
              ) : (
                <div className="reliability__vendor-discrepancy mt-3" style={{ background: 'rgba(34,197,94,0.08)', borderColor: 'rgba(34,197,94,0.2)' }}>
                  <CheckCircle size={14} style={{ color: 'var(--color-healthy-text)' }} />
                  <p className="text-sm" style={{ color: 'var(--color-healthy-text)' }}>
                    Vendor status is consistent with observed telemetry. Nominal baseline verified.
                  </p>
                </div>
              )}

              <div className="mt-4 reliability__vendor-row">
                <span className="text-secondary text-sm">Circuit breaker action</span>
                <span className="mono text-sm font-semibold" style={{
                  color: assessment?.recommended_action === 'ARM_FALLBACK_OR_CIRCUIT'
                    ? 'var(--color-critical-text)'
                    : 'var(--color-healthy-text)'
                }}>
                  {assessment?.recommended_action || 'NO_ACTION'}
                </span>
              </div>

              {activeIncidents.length > 0 && (
                <button
                  className="btn btn-secondary w-full mt-4"
                  onClick={() => navigate('/incidents')}
                >
                  View Incident Timeline ({activeIncidents.length} active) <ChevronRight size={14} />
                </button>
              )}
            </div>
          </div>

          {/* Engine Reason Codes Card */}
          <div className="card reliability__analysis-card">
            <p className="section-heading mb-4">Reason Codes & Heuristics</p>
            <div className="reliability__reasoning">
              {assessment?.reason_codes && assessment.reason_codes.length > 0 ? (
                assessment.reason_codes.map((code, idx) => (
                  <div key={code} className="reliability__reasoning-step">
                    <span className="reliability__reasoning-num">{`0${idx + 1}`}</span>
                    <p className="text-sm text-secondary font-mono">{code}</p>
                  </div>
                ))
              ) : (
                <div className="reliability__reasoning-step">
                  <span className="reliability__reasoning-num">01</span>
                  <p className="text-sm text-secondary">WITHIN_NORMAL_RANGE</p>
                </div>
              )}

              <div className="mt-4 pt-3" style={{ borderTop: '1px solid var(--color-border)' }}>
                <p className="text-xs text-secondary mb-1">Recommended Remediation Action:</p>
                <span className="mono text-sm font-bold" style={{
                  color: assessment?.recommended_action === 'ARM_FALLBACK_OR_CIRCUIT'
                    ? 'var(--color-warning-text)'
                    : 'var(--color-primary-text)'
                }}>
                  {assessment?.recommended_action || 'NO_ACTION'}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Monitored Dependencies Health Summary */}
      <div className="reliability__section">
        <h2 className="section-heading mb-4">Monitored Dependencies Health Summary</h2>
        <div className="grid-3 gap-4">
          {dependencies.map(dep => {
            const m = getStatusMeta(dep.status);
            return (
              <div
                key={dep.id}
                className="card"
                style={{ cursor: 'pointer' }}
                onClick={() => navigate(`/dependencies/${dep.id}`)}
              >
                <div className="flex items-center gap-2 mb-3">
                  <HealthDot status={dep.status} size={10} />
                  <span className="font-semibold text-primary">{dep.vendor.displayName}</span>
                </div>
                <div className="flex items-center justify-between text-xs text-secondary">
                  <span>Region: {dep.regions.join(', ')}</span>
                  <span className={`badge ${m.cssClass}`}>{m.label}</span>
                </div>
                <div className="flex items-center justify-between text-xs mt-3 pt-2" style={{ borderTop: '1px solid var(--color-border)' }}>
                  <span className="text-secondary">P95: {formatLatency(dep.latency.p95)}</span>
                  <span className="text-secondary">Circuit: {dep.circuitState.toUpperCase()}</span>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
