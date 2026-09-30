import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Zap, Play, CheckCircle2, AlertTriangle, Flame, Clock } from 'lucide-react';
import './ChaosControlPanel.css';

interface HealthAssessmentResponse {
  id: string;
  dependency_name: string;
  region: string;
  status: string;
  customer_signal: string;
  synthetic_signal: string;
  vendor_status: string;
  customer_latency_ms: number;
  synthetic_latency_ms: number;
  customer_baseline_p95_ms: number;
  synthetic_baseline_p95_ms: number;
  latency_multiplier: number;
  confidence: string;
  reason_codes: string[];
  recommended_action: string;
  explanation: string;
  timestamp: string;
}

interface ScenarioOption {
  id: string;
  label: string;
  apiValue: string;
  className?: string;
}

const SCENARIOS: ScenarioOption[] = [
  { id: 'normal', label: 'NORMAL', apiValue: 'NORMAL', className: 'chaos-panel__btn--normal' },
  { id: 'degraded', label: 'DEGRADED ~2s', apiValue: 'DEGRADED', className: 'chaos-panel__btn--degraded' },
  { id: 'grey_failure', label: 'GREY FAILURE ~8s', apiValue: 'GREY_FAILURE', className: 'chaos-panel__btn--grey' },
  { id: 'http_500', label: 'HTTP 500', apiValue: 'HTTP_500' },
  { id: 'http_429', label: 'HTTP 429', apiValue: 'HTTP_429' },
  { id: 'timeout', label: 'TIMEOUT', apiValue: 'TIMEOUT' },
];

export const ChaosControlPanel: React.FC = () => {
  const [currentScenario, setCurrentScenario] = useState<string>('NORMAL');
  const [applying, setApplying] = useState<boolean>(false);
  const [statusMessage, setStatusMessage] = useState<string>('');
  const [generatingTraffic, setGeneratingTraffic] = useState<boolean>(false);
  const [assessment, setAssessment] = useState<HealthAssessmentResponse | null>(null);
  const [lastPolledAt, setLastPolledAt] = useState<Date | null>(null);
  const isMounted = useRef<boolean>(true);

  // 1. Fetch current vendor scenario from real vendor mock API
  const fetchScenario = useCallback(async () => {
    try {
      const res = await fetch('/admin/scenario');
      if (res.ok) {
        const data = await res.json();
        if (isMounted.current && data?.scenario) {
          setCurrentScenario(data.scenario.toUpperCase());
        }
      }
    } catch {
      // Ignore network errors on background poll
    }
  }, []);

  // 2. Poll real APIPulse assessment for stripe-like ap-south-1
  const pollAssessment = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/dependencies/stripe-like/health?region=ap-south-1');
      if (res.ok) {
        const data: HealthAssessmentResponse = await res.json();
        if (isMounted.current) {
          setAssessment(data);
          setLastPolledAt(new Date());
        }
      }
    } catch {
      // Ignore poll error
    }
  }, []);

  // Mount pollers
  useEffect(() => {
    isMounted.current = true;
    fetchScenario();
    pollAssessment();

    const intervalId = setInterval(() => {
      pollAssessment();
      fetchScenario();
    }, 3000);

    return () => {
      isMounted.current = false;
      clearInterval(intervalId);
    };
  }, [fetchScenario, pollAssessment]);

  // 3. Apply Scenario
  const handleApplyScenario = async (sc: ScenarioOption) => {
    setApplying(true);
    setStatusMessage('Applying...');
    try {
      const res = await fetch('/admin/scenario', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ scenario: sc.apiValue }),
      });
      if (res.ok) {
        setCurrentScenario(sc.apiValue);
        setStatusMessage(`Scenario applied: ${sc.apiValue}. Emitting real CodeBegun payment traffic...`);
        // Immediately emit real payment traffic so OTel ingests real latency for the new scenario
        try {
          const tRes = await fetch('/demo-traffic', { method: 'POST' });
          if (tRes.ok) {
            const tData = await tRes.json();
            const lat = Math.round(tData?.vendor_latency_ms || 0);
            setStatusMessage(`Active: ${sc.apiValue} · Real CodeBegun payment completed (${lat}ms) · OTel traces ingested by APIPulse`);
          } else {
            setStatusMessage(`Scenario applied: ${sc.apiValue}`);
          }
        } catch {
          setStatusMessage(`Scenario applied: ${sc.apiValue}`);
        }
        setTimeout(() => pollAssessment(), 1200);
      } else {
        const errText = await res.text();
        setStatusMessage(`Failed to apply scenario: ${errText || res.statusText}`);
      }
    } catch (e: any) {
      setStatusMessage(`Network error applying scenario: ${e?.message || 'unknown error'}`);
    } finally {
      if (isMounted.current) {
        setApplying(false);
      }
    }
  };

  // 4. Generate Real Test Traffic via CodeBegun Demo
  const handleGenerateTraffic = async () => {
    setGeneratingTraffic(true);
    setStatusMessage('Generating real payment traffic via CodeBegun Demo...');
    const results: number[] = [];

    try {
      for (let i = 0; i < 3; i++) {
        const t0 = performance.now();
        const res = await fetch('/demo-traffic', { method: 'POST' });
        const t1 = performance.now();
        if (res.ok) {
          const body = await res.json();
          results.push(Math.round(body?.vendor_latency_ms || (t1 - t0)));
        } else {
          results.push(Math.round(t1 - t0));
        }
      }
      setStatusMessage(`Real traffic emitted: 3 payments (latencies: ${results.map(r => `${r}ms`).join(', ')}) — OTel ingested by APIPulse`);
      pollAssessment();
    } catch (e: any) {
      setStatusMessage(`Error generating traffic: ${e?.message || 'Request failed'}`);
    } finally {
      if (isMounted.current) {
        setGeneratingTraffic(false);
      }
    }
  };

  // Status color helpers
  const getAssessmentClass = (st?: string) => {
    switch (st?.toUpperCase()) {
      case 'HEALTHY': return 'val--healthy';
      case 'RECOVERING': return 'val--recovering';
      case 'DEGRADED': return 'val--degraded';
      case 'GREY_FAILURE': return 'val--grey-failure';
      case 'OUTAGE': return 'val--outage';
      default: return 'val--insufficient';
    }
  };

  const getActionClass = (act?: string) => {
    if (act?.includes('ARM') || act?.includes('CIRCUIT')) return 'val--action-arm';
    if (act?.includes('INVESTIGATE') || act?.includes('ESCALATE')) return 'val--action-investigate';
    return 'val--action-none';
  };

  return (
    <div className="chaos-panel">
      {/* Header */}
      <div className="chaos-panel__header">
        <div className="chaos-panel__title-wrap">
          <span className="chaos-panel__badge">Live SRE Harness</span>
          <h2 className="chaos-panel__title">
            <Flame size={18} color="#f97316" />
            Demo / Chaos Control
          </h2>
        </div>

        <div className="chaos-panel__target-info">
          <span>Target:</span>
          <span className="chaos-panel__target-value">stripe-like</span>
          <span>(ap-south-1)</span>
        </div>
      </div>

      {/* Scenario Buttons */}
      <div className="chaos-panel__scenario-row">
        <span className="chaos-panel__label">Scenario:</span>
        {SCENARIOS.map((sc) => {
          const isActive = currentScenario === sc.apiValue;
          return (
            <button
              key={sc.id}
              className={`chaos-panel__btn ${sc.className || ''} ${isActive ? 'chaos-panel__btn--active' : ''}`}
              onClick={() => handleApplyScenario(sc)}
              disabled={applying || generatingTraffic}
              title={`Switch Vendor Mock to ${sc.label}`}
            >
              {isActive && <CheckCircle2 size={12} />}
              {sc.label}
            </button>
          );
        })}

        <button
          className="chaos-panel__btn chaos-panel__btn--traffic"
          onClick={handleGenerateTraffic}
          disabled={applying || generatingTraffic}
          title="Send real payment transactions through CodeBegun Demo to trigger OTel traces"
        >
          {generatingTraffic ? <Clock size={13} className="spin" /> : <Play size={13} />}
          {generatingTraffic ? 'Sending...' : 'Generate Test Traffic'}
        </button>
      </div>

      {/* Status Bar */}
      <div className="chaos-panel__status-bar">
        <div>
          <span>Current Vendor Scenario: </span>
          <strong className="chaos-panel__status-msg">{currentScenario}</strong>
        </div>
        {statusMessage && (
          <div className="text-secondary text-xs">
            {statusMessage}
          </div>
        )}
      </div>

      {/* Live Detection Section */}
      <div className="chaos-panel__detection">
        <div className="chaos-panel__detection-header">
          <div className="chaos-panel__detection-title">
            <span className="chaos-panel__live-pulse" />
            Live Intelligence Detection (Real APIPulse Engine)
          </div>
          {lastPolledAt && (
            <span className="text-muted text-xs">
              Updated {lastPolledAt.toLocaleTimeString()}
            </span>
          )}
        </div>

        <div className="chaos-panel__grid">
          {/* Customer Signal */}
          <div className="chaos-panel__cell">
            <span className="chaos-panel__cell-label">Customer Signal</span>
            <span className={`chaos-panel__cell-value ${getAssessmentClass(assessment?.customer_signal)}`}>
              {assessment?.customer_signal || 'INSUFFICIENT_DATA'}
            </span>
            <span className="chaos-panel__cell-sub">
              {assessment?.customer_latency_ms ? `${Math.round(assessment.customer_latency_ms)}ms P95` : 'no samples'}
            </span>
          </div>

          {/* Synthetic Signal */}
          <div className="chaos-panel__cell">
            <span className="chaos-panel__cell-label">Synthetic Signal</span>
            <span className={`chaos-panel__cell-value ${getAssessmentClass(assessment?.synthetic_signal)}`}>
              {assessment?.synthetic_signal || 'INSUFFICIENT_DATA'}
            </span>
            <span className="chaos-panel__cell-sub">
              {assessment?.synthetic_latency_ms ? `${Math.round(assessment.synthetic_latency_ms)}ms P95` : 'no samples'}
            </span>
          </div>

          {/* Vendor Status */}
          <div className="chaos-panel__cell">
            <span className="chaos-panel__cell-label">Vendor Status</span>
            <span className={`chaos-panel__cell-value ${assessment?.vendor_status === 'OPERATIONAL' ? 'val--healthy' : 'val--degraded'}`}>
              {assessment?.vendor_status || 'UNKNOWN'}
            </span>
            <span className="chaos-panel__cell-sub">vendor status page</span>
          </div>

          {/* APIPulse Assessment */}
          <div className="chaos-panel__cell">
            <span className="chaos-panel__cell-label">APIPulse Assessment</span>
            <span className={`chaos-panel__cell-value ${getAssessmentClass(assessment?.status)}`}>
              {assessment?.status || 'EVALUATING'}
            </span>
            <span className="chaos-panel__cell-sub">engine classification</span>
          </div>

          {/* Confidence */}
          <div className="chaos-panel__cell">
            <span className="chaos-panel__cell-label">Confidence</span>
            <span className="chaos-panel__cell-value" style={{ color: assessment?.confidence === 'HIGH' ? '#a855f7' : '#9ca3af' }}>
              {assessment?.confidence || '—'}
            </span>
            <span className="chaos-panel__cell-sub">correlation model</span>
          </div>

          {/* Latency Multiplier */}
          <div className="chaos-panel__cell">
            <span className="chaos-panel__cell-label">Latency Multiplier</span>
            <span className={`chaos-panel__cell-value ${(assessment?.latency_multiplier || 1) >= 3 ? 'val--grey-failure' : 'val--healthy'}`}>
              {assessment?.latency_multiplier ? `${assessment.latency_multiplier.toFixed(1)}x` : '1.0x'}
            </span>
            <span className="chaos-panel__cell-sub">vs historical baseline</span>
          </div>

          {/* Recommended Action */}
          <div className="chaos-panel__cell">
            <span className="chaos-panel__cell-label">Recommended Action</span>
            <span className={`chaos-panel__cell-value ${getActionClass(assessment?.recommended_action)}`}>
              {assessment?.recommended_action || 'NONE'}
            </span>
            <span className="chaos-panel__cell-sub">SRE mitigation</span>
          </div>
        </div>
      </div>
    </div>
  );
};
