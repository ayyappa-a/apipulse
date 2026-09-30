import React, { useState } from 'react';
import {
  Settings as SettingsIcon, Shield, Sliders, Bell, Database,
  Cpu, RefreshCw, Save, Check, AlertTriangle, Zap, Download
} from 'lucide-react';
import { useToast } from '../context/AppContext';
import './Settings.css';

export default function Settings() {
  const { addToast } = useToast();
  const [sensitivityMultiplier, setSensitivityMultiplier] = useState(4.0);
  const [circuitTripP95, setCircuitTripP95] = useState(2500);
  const [circuitErrorRate, setCircuitErrorRate] = useState(5.0);
  const [probeInterval, setProbeInterval] = useState(30);
  const [autoArmCircuit, setAutoArmCircuit] = useState(true);
  const [slackChannel, setSlackChannel] = useState('#alerts-sre');
  const [pagerdutyService, setPagerdutyService] = useState('PD-TIER-1-ESCALATION');
  const [retentionDays, setRetentionDays] = useState(90);
  const [saved, setSaved] = useState(false);

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    setSaved(true);
    addToast('APIPulse SRE policies updated across all 8 global probe nodes.', 'success');
    setTimeout(() => setSaved(false), 2500);
  };

  const handleInjectChaos = () => {
    addToast('Simulated tail-latency degradation injected into SendGrid ap-south-1 probe stream.', 'warning');
  };

  const handleResetData = () => {
    addToast('Telemetry caches flushed and reset to reference baseline state.', 'info');
  };

  return (
    <div className="settings-page">
      {/* Header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">SRE Reliability & Circuit Settings</h1>
          <p className="page-subtitle">
            Configure autonomous grey-failure trip thresholds, edge probe cadence, and circuit breakers.
          </p>
        </div>
        <div className="settings-header-actions">
          <button
            type="button"
            className="btn btn--outline"
            onClick={handleResetData}
          >
            <RefreshCw size={14} />
            <span>Reset Demo Baseline</span>
          </button>
        </div>
      </div>

      <form onSubmit={handleSave} className="settings-form">
        {/* Grey-Failure Detection Engine Settings */}
        <div className="card settings-section">
          <div className="settings-section-header">
            <div className="settings-section-icon settings-section-icon--primary">
              <Zap size={20} />
            </div>
            <div>
              <h2 className="section-title">Autonomous Grey-Failure Heuristics</h2>
              <p className="section-subtitle">
                Mathematical detection parameters for vendor-status divergence and tail latency inflation.
              </p>
            </div>
          </div>

          <div className="settings-grid">
            <div className="settings-field">
              <label className="settings-label">
                P95 Latency Spike Multiplier Threshold
                <span className="settings-sublabel">
                  Flag as grey-failure when regional P95 exceeds baseline by this factor
                </span>
              </label>
              <div className="settings-range-wrap">
                <input
                  type="range"
                  min="2.0"
                  max="10.0"
                  step="0.5"
                  value={sensitivityMultiplier}
                  onChange={e => setSensitivityMultiplier(parseFloat(e.target.value))}
                  className="settings-range"
                />
                <span className="range-display font-mono">{sensitivityMultiplier.toFixed(1)}x Baseline</span>
              </div>
            </div>

            <div className="settings-field">
              <label className="settings-label">
                Vendor Status Cross-Verification
                <span className="settings-sublabel">
                  Query vendor status page RSS / API every probe cycle for divergence verification
                </span>
              </label>
              <div className="settings-toggle-row">
                <input
                  type="checkbox"
                  id="auto-verify-vendor"
                  defaultChecked
                  className="settings-checkbox"
                />
                <label htmlFor="auto-verify-vendor" className="settings-toggle-label">
                  Active (Continuous Correlation Engine Enabled)
                </label>
              </div>
            </div>
          </div>
        </div>

        {/* Proactive Circuit Breaker Settings */}
        <div className="card settings-section">
          <div className="settings-section-header">
            <div className="settings-section-icon settings-section-icon--shield">
              <Shield size={20} />
            </div>
            <div>
              <h2 className="section-title">Proactive Circuit Breaker Rules</h2>
              <p className="section-subtitle">
                Autonomous protection trips to stop hanging downstream connection pools.
              </p>
            </div>
          </div>

          <div className="settings-grid">
            <div className="settings-field">
              <label className="settings-label">
                Auto-Trip P95 Latency Ceiling
                <span className="settings-sublabel">Trip circuit when regional P95 exceeds this absolute value</span>
              </label>
              <div className="settings-input-group">
                <input
                  type="number"
                  className="input font-mono"
                  value={circuitTripP95}
                  onChange={e => setCircuitTripP95(parseInt(e.target.value) || 0)}
                  min={500}
                  max={15000}
                  step={100}
                />
                <span className="input-affix">ms</span>
              </div>
            </div>

            <div className="settings-field">
              <label className="settings-label">
                Auto-Trip Error Rate Floor
                <span className="settings-sublabel">Trip circuit when probe HTTP 5xx errors exceed</span>
              </label>
              <div className="settings-input-group">
                <input
                  type="number"
                  className="input font-mono"
                  value={circuitErrorRate}
                  onChange={e => setCircuitErrorRate(parseFloat(e.target.value) || 0)}
                  min={0.5}
                  max={50}
                  step={0.5}
                />
                <span className="input-affix">%</span>
              </div>
            </div>

            <div className="settings-field">
              <label className="settings-label">
                Autonomous Regional Arming
                <span className="settings-sublabel">
                  Arm circuit automatically when grey failure is detected before vendor acknowledges
                </span>
              </label>
              <div className="settings-toggle-row">
                <input
                  type="checkbox"
                  id="auto-arm"
                  checked={autoArmCircuit}
                  onChange={e => setAutoArmCircuit(e.target.checked)}
                  className="settings-checkbox"
                />
                <label htmlFor="auto-arm" className="settings-toggle-label">
                  Enable Proactive Circuit Arming
                </label>
              </div>
            </div>
          </div>
        </div>

        {/* Dispatch & Notification Routing */}
        <div className="card settings-section">
          <div className="settings-section-header">
            <div className="settings-section-icon settings-section-icon--bell">
              <Bell size={20} />
            </div>
            <div>
              <h2 className="section-title">Notification Channels & Escalation</h2>
              <p className="section-subtitle">
                Default outbound destinations for alerts and degraded SLA notifications.
              </p>
            </div>
          </div>

          <div className="settings-grid">
            <div className="settings-field">
              <label className="settings-label">
                Slack Alert Channel
                <span className="settings-sublabel">Channel for real-time grey-failure and circuit alerts</span>
              </label>
              <input
                type="text"
                className="input font-mono"
                value={slackChannel}
                onChange={e => setSlackChannel(e.target.value)}
              />
            </div>

            <div className="settings-field">
              <label className="settings-label">
                PagerDuty Service ID
                <span className="settings-sublabel">Target service for Sev-1 / Sev-2 on-call paging</span>
              </label>
              <input
                type="text"
                className="input font-mono"
                value={pagerdutyService}
                onChange={e => setPagerdutyService(e.target.value)}
              />
            </div>
          </div>
        </div>

        {/* Demo & Hackathon Chaos Simulator */}
        <div className="card settings-section settings-section--chaos">
          <div className="settings-section-header">
            <div className="settings-section-icon settings-section-icon--chaos">
              <AlertTriangle size={20} />
            </div>
            <div>
              <h2 className="section-title">APIPulse Live Chaos Simulator</h2>
              <p className="section-subtitle">
                Inject synthetic grey failures into real-time probe streams to demonstrate latency tail defense.
              </p>
            </div>
          </div>

          <div className="chaos-controls-box">
            <p className="chaos-desc">
              Demonstrates APIPulse's core value proposition: A dependency (e.g. SendGrid or Stripe) continues returning HTTP 200 OK while latency inflates to 4,800ms in AP-South. APIPulse catches it within 2 probe cycles and trips the circuit.
            </p>
            <div className="chaos-actions">
              <button
                type="button"
                className="btn btn--outline"
                onClick={handleInjectChaos}
              >
                <Zap size={14} className="text-warning" />
                <span>Inject SendGrid AP-South Grey Failure</span>
              </button>
            </div>
          </div>
        </div>

        {/* Save Bar */}
        <div className="settings-footer">
          <button type="submit" className="btn btn--primary">
            {saved ? <Check size={16} /> : <Save size={16} />}
            <span>{saved ? 'Settings Saved' : 'Apply Configuration'}</span>
          </button>
        </div>
      </form>
    </div>
  );
}
