import React, { useState } from 'react';
import {
  Plug, CheckCircle2, XCircle, AlertCircle, ArrowUpRight,
  Copy, Check, ExternalLink, RefreshCw, Send, ShieldCheck,
  Code2, Terminal, Sliders
} from 'lucide-react';
import { useIntegrations } from '../hooks/useIntegrations';
import { formatRelativeTime } from '../utils/status';
import type { Integration, IntegrationType } from '../types/domain';
import { useToast } from '../context/AppContext';
import './Integrations.css';

export default function Integrations() {
  const { addToast } = useToast();
  const { integrations: integrationsList, toggleConnect, testRelay } = useIntegrations();
  const [copiedSnippet, setCopiedSnippet] = useState(false);
  const [testPayloadLoading, setTestPayloadLoading] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'all' | 'alerts' | 'metrics'>('all');

  const handleToggleConnect = async (intId: string) => {
    const updated = await toggleConnect(intId);
    addToast(`${updated.name} integration ${updated.status}.`, updated.status === 'connected' ? 'success' : 'info');
  };

  const handleTestDispatch = async (integration: Integration) => {
    setTestPayloadLoading(integration.id);
    const res = await testRelay(integration.id);
    setTestPayloadLoading(null);
    if (res.success) {
      addToast(`Test payload dispatched to ${integration.name} successfully (${res.latencyMs}ms, 200 OK).`, 'success');
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedSnippet(true);
    addToast('Configuration snippet copied to clipboard.', 'info');
    setTimeout(() => setCopiedSnippet(false), 2000);
  };

  const prometheusScrapeConfig = `scrape_configs:
  - job_name: 'apipulse'
    scrape_interval: 15s
    scrape_timeout: 10s
    metrics_path: '/api/v1/metrics'
    scheme: 'https'
    bearer_token: '••••••••••••••••••••••••'
    static_configs:
      - targets: ['telemetry.apipulse.io']
        labels:
          environment: 'production'
          cluster: 'global-edge-fleet'`;

  const filteredIntegrations = integrationsList.filter(item => {
    if (activeTab === 'alerts') return item.type === 'slack' || item.type === 'pagerduty' || item.type === 'webhook';
    if (activeTab === 'metrics') return item.type === 'prometheus' || item.type === 'opentelemetry';
    return true;
  });

  return (
    <div className="integrations-page">
      {/* Header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">Ecosystem & Telemetry Integrations</h1>
          <p className="page-subtitle">
            Relay sub-second grey-failure alerts and export Prometheus / OpenTelemetry OTLP metrics.
          </p>
        </div>
        <div className="integrations-header-tabs">
          <button
            className={`btn ${activeTab === 'all' ? 'btn--primary' : 'btn--outline'} btn--sm`}
            onClick={() => setActiveTab('all')}
          >
            All Integrations
          </button>
          <button
            className={`btn ${activeTab === 'alerts' ? 'btn--primary' : 'btn--outline'} btn--sm`}
            onClick={() => setActiveTab('alerts')}
          >
            Incident Alerts
          </button>
          <button
            className={`btn ${activeTab === 'metrics' ? 'btn--primary' : 'btn--outline'} btn--sm`}
            onClick={() => setActiveTab('metrics')}
          >
            Metrics & OTLP
          </button>
        </div>
      </div>

      {/* Zero Trust Security Banner */}
      <div className="card integrations-sec-banner">
        <div className="integrations-sec-icon">
          <ShieldCheck size={24} />
        </div>
        <div className="integrations-sec-content">
          <h3 className="integrations-sec-title">Enterprise Zero-Leak Security Architecture</h3>
          <p className="integrations-sec-text">
            All API keys, Slack Webhooks, and PagerDuty routing keys are encrypted at rest using envelope encryption (AES-256-GCM) in server-side KMS vaults. No raw credentials are ever exposed in frontend payloads or DOM trees.
          </p>
        </div>
      </div>

      {/* Integrations Grid */}
      <div className="integrations-grid">
        {filteredIntegrations.map(int => {
          const isConnected = int.status === 'connected';
          return (
            <div key={int.id} className="card integration-card">
              <div className="integration-top">
                <div className="integration-brand">
                  <div className={`integration-icon-wrap integration-icon-wrap--${int.type}`}>
                    <Plug size={20} />
                  </div>
                  <div>
                    <h3 className="integration-name">{int.name}</h3>
                    <p className="integration-type-label">{int.type.toUpperCase()}</p>
                  </div>
                </div>
                <div className="integration-status-pill">
                  {isConnected ? (
                    <span className="badge badge--success">
                      <CheckCircle2 size={12} /> CONNECTED
                    </span>
                  ) : (
                    <span className="badge badge--neutral">
                      <XCircle size={12} /> DISCONNECTED
                    </span>
                  )}
                </div>
              </div>

              <p className="integration-desc">{int.description}</p>

              {/* Masked Secret Hint */}
              {int.maskedCredentialHint && (
                <div className="integration-secret-row">
                  <span className="secret-label">Vault Key:</span>
                  <code className="secret-hint">{int.maskedCredentialHint}</code>
                </div>
              )}

              {/* Event Subscriptions */}
              <div className="integration-events-box">
                <span className="events-label">Subscribed Events:</span>
                <div className="events-tags">
                  {int.eventTypes.map(evt => (
                    <span key={evt} className="event-tag">{evt}</span>
                  ))}
                </div>
              </div>

              {/* Footer info & actions */}
              <div className="integration-footer">
                <div className="integration-meta-col">
                  {isConnected && int.lastDeliveryAt ? (
                    <span className="delivery-time">
                      Last ping: {formatRelativeTime(int.lastDeliveryAt)}
                    </span>
                  ) : (
                    <span className="delivery-time text-muted">Ready to connect</span>
                  )}
                </div>

                <div className="integration-actions-col">
                  {isConnected && (
                    <button
                      className="btn btn--outline btn--sm"
                      onClick={() => handleTestDispatch(int)}
                      disabled={testPayloadLoading === int.id}
                      title="Send instant simulated webhook"
                    >
                      {testPayloadLoading === int.id ? (
                        <RefreshCw size={12} className="spin" />
                      ) : (
                        <Send size={12} />
                      )}
                      <span>Test Relay</span>
                    </button>
                  )}
                  <button
                    className={`btn ${isConnected ? 'btn--ghost' : 'btn--primary'} btn--sm`}
                    onClick={() => handleToggleConnect(int.id)}
                  >
                    {isConnected ? 'Disconnect' : 'Connect'}
                  </button>
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {/* Prometheus & OpenTelemetry Endpoint Exposer */}
      <div className="card integrations-telemetry-box">
        <div className="telemetry-box-header">
          <div className="telemetry-box-title-group">
            <Terminal size={20} className="text-primary" />
            <div>
              <h2 className="section-title">Prometheus / OpenTelemetry Scraping Configuration</h2>
              <p className="section-subtitle">
                Pull real-time tail percentiles and grey-failure gauges directly into Grafana or Prometheus.
              </p>
            </div>
          </div>
          <button
            className="btn btn--outline btn--sm"
            onClick={() => copyToClipboard(prometheusScrapeConfig)}
          >
            {copiedSnippet ? <Check size={14} className="text-success" /> : <Copy size={14} />}
            <span>{copiedSnippet ? 'Copied YAML' : 'Copy prometheus.yml'}</span>
          </button>
        </div>

        <div className="telemetry-code-block">
          <pre>
            <code>{prometheusScrapeConfig}</code>
          </pre>
        </div>

        <div className="telemetry-sample-metrics">
          <span className="sample-label">Exported Metric Families:</span>
          <div className="sample-metrics-tags">
            <code>apipulse_probe_latency_seconds_bucket&#123;region, dep, quantile&#125;</code>
            <code>apipulse_grey_failure_detected&#123;dep, vendor_status="operational"&#125;</code>
            <code>apipulse_circuit_breaker_tripped_total&#123;dep, region&#125;</code>
            <code>apipulse_vendor_page_status&#123;vendor&#125;</code>
          </div>
        </div>
      </div>
    </div>
  );
}
