import React, { useState } from 'react';
import {
  AlertTriangle, CheckCircle, Clock, Shield, Search,
  ChevronDown, ChevronUp, Bell, ExternalLink, Zap, ArrowRight,
  Filter, AlertOctagon, Info, Activity
} from 'lucide-react';
import { StatusBadge, HealthDot } from '../components/ui/StatusBadge';
import { useIncidents } from '../hooks/useIncidents';
import {
  formatLatency, formatRelativeTime, getStatusMeta,
  SEVERITY_META, CIRCUIT_META
} from '../utils/status';
import type { Incident, Severity, IncidentStatus } from '../types/domain';
import { useToast } from '../context/AppContext';
import './Incidents.css';

export default function Incidents() {
  const { addToast } = useToast();
  const { incidents, activeCount, criticalCount, resolvedCount, acknowledge } = useIncidents();
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [severityFilter, setSeverityFilter] = useState<string>('all');
  const [selectedIncidentId, setSelectedIncidentId] = useState<string>('');
  const [ackList, setAckList] = useState<Record<string, boolean>>({});

  const filteredIncidents = incidents.filter(inc => {
    if (statusFilter === 'active' && inc.status === 'resolved') return false;
    if (statusFilter === 'resolved' && inc.status !== 'resolved') return false;
    if (statusFilter !== 'all' && statusFilter !== 'active' && statusFilter !== 'resolved' && inc.status !== statusFilter) return false;
    if (severityFilter !== 'all' && inc.severity !== severityFilter) return false;
    if (search.trim()) {
      const q = search.toLowerCase();
      const matchTitle = inc.title.toLowerCase().includes(q);
      const matchVendor = inc.vendorName.toLowerCase().includes(q);
      const matchDesc = inc.description.toLowerCase().includes(q);
      if (!matchTitle && !matchVendor && !matchDesc) return false;
    }
    return true;
  });

  const selectedIncident = (selectedIncidentId ? incidents.find(i => i.id === selectedIncidentId) : filteredIncidents[0]) || filteredIncidents[0];

  const handleAcknowledge = async (incId: string) => {
    setAckList(prev => ({ ...prev, [incId]: true }));
    await acknowledge(incId);
    addToast(`Incident ${incId} acknowledged by on-call engineer.`, 'success');
  };

  const handleRunDiagnostics = (depId: string) => {
    addToast(`Dispatched synthetic probe sweep across 8 edge regions for ${depId}.`, 'info');
  };

  return (
    <div className="incidents-page">
      {/* Header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">Incidents & Grey Failure Alerts</h1>
          <p className="page-subtitle">
            Autonomous detection of tail latency degradation, vendor status divergence, and regional outages.
          </p>
        </div>
        <div className="incidents-header-actions">
          <button
            className="btn btn--outline"
            onClick={() => addToast('Incident post-mortem report exported to JSON/Markdown.', 'info')}
          >
            Export SRE Log
          </button>
        </div>
      </div>

      {/* KPI Stats Strip */}
      <div className="incidents-stats-grid">
        <div className="card incidents-stat-card">
          <div className="incidents-stat-icon incidents-stat-icon--active">
            <AlertTriangle size={20} />
          </div>
          <div>
            <div className="incidents-stat-value">{activeCount}</div>
            <div className="incidents-stat-label">Active Incidents</div>
          </div>
        </div>
        <div className="card incidents-stat-card">
          <div className="incidents-stat-icon incidents-stat-icon--critical">
            <AlertOctagon size={20} />
          </div>
          <div>
            <div className="incidents-stat-value">{criticalCount}</div>
            <div className="incidents-stat-label">Critical Severity</div>
          </div>
        </div>
        <div className="card incidents-stat-card">
          <div className="incidents-stat-icon incidents-stat-icon--resolved">
            <CheckCircle size={20} />
          </div>
          <div>
            <div className="incidents-stat-value">{resolvedCount}</div>
            <div className="incidents-stat-label">Resolved (Past 7d)</div>
          </div>
        </div>
        <div className="card incidents-stat-card">
          <div className="incidents-stat-icon incidents-stat-icon--speed">
            <Zap size={20} />
          </div>
          <div>
            <div className="incidents-stat-value">4.2 min</div>
            <div className="incidents-stat-label">Mean Detection Time</div>
          </div>
        </div>
      </div>

      {/* Filters */}
      <div className="incidents-filters card">
        <div className="incidents-search-box">
          <Search size={16} className="incidents-search-icon" />
          <input
            type="text"
            className="input incidents-search-input"
            placeholder="Search incidents by title, vendor, or keywords..."
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
        </div>

        <div className="incidents-filter-group">
          <label className="incidents-filter-label">Status:</label>
          <div className="incidents-tabs">
            {['all', 'active', 'resolved'].map(s => (
              <button
                key={s}
                className={`incidents-tab ${statusFilter === s ? 'incidents-tab--active' : ''}`}
                onClick={() => setStatusFilter(s)}
              >
                {s.charAt(0).toUpperCase() + s.slice(1)}
              </button>
            ))}
          </div>
        </div>

        <div className="incidents-filter-group">
          <label className="incidents-filter-label">Severity:</label>
          <select
            className="input incidents-select"
            value={severityFilter}
            onChange={e => setSeverityFilter(e.target.value)}
          >
            <option value="all">All Severities</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
        </div>
      </div>

      {/* Main 2-Column Split: Incidents List & Deep Dive Inspector */}
      <div className="incidents-main-layout">
        {/* Left Column: List */}
        <div className="incidents-list-col">
          <div className="incidents-list-header">
            <span>Incidents ({filteredIncidents.length})</span>
            <span className="incidents-list-sub">Click to inspect timeline</span>
          </div>

          <div className="incidents-items-scroll">
            {filteredIncidents.length === 0 ? (
              <div className="incidents-empty card">
                <AlertTriangle size={32} />
                <p>No incidents found matching current filters.</p>
              </div>
            ) : (
              filteredIncidents.map(inc => {
                const isSelected = selectedIncident?.id === inc.id;
                const isAcked = ackList[inc.id];
                const sevMeta = SEVERITY_META[inc.severity];
                const statusMeta = getStatusMeta(inc.healthStatus);

                return (
                  <div
                    key={inc.id}
                    className={`card incident-item-card ${isSelected ? 'incident-item-card--selected' : ''}`}
                    onClick={() => setSelectedIncidentId(inc.id)}
                  >
                    <div className="incident-item-top">
                      <div className="incident-item-meta">
                        <span className={`badge ${sevMeta.cssClass}`}>
                          {sevMeta.label}
                        </span>
                        <StatusBadge status={inc.healthStatus} size="sm" />
                        <span className="incident-item-time">{formatRelativeTime(inc.detectedAt)}</span>
                      </div>
                      {isAcked && <span className="incident-acked-tag">Acknowledged</span>}
                    </div>

                    <h3 className="incident-item-title">{inc.title}</h3>
                    <p className="incident-item-desc">{inc.description}</p>

                    <div className="incident-item-footer">
                      <div className="incident-item-metrics">
                        <span className="metric-tag">
                          Peak: <strong>{formatLatency(inc.peakP95Ms)}</strong>
                        </span>
                        <span className="metric-tag">
                          Base: <strong>{formatLatency(inc.baselineP95Ms)}</strong>
                        </span>
                        {inc.peakP95Ms && inc.baselineP95Ms ? (
                          <span className="metric-tag metric-tag--spike">
                            +{Math.round((inc.peakP95Ms / inc.baselineP95Ms) * 10) / 10}x
                          </span>
                        ) : null}
                      </div>
                      <span className="incident-regions-count">
                        {inc.affectedRegions.length} {inc.affectedRegions.length === 1 ? 'Region' : 'Regions'}
                      </span>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>

        {/* Right Column: Deep-Dive Inspector */}
        <div className="incidents-detail-col">
          {selectedIncident ? (
            <div className="card incident-detail-card">
              {/* Header */}
              <div className="incident-detail-header">
                <div className="incident-detail-headline">
                  <div className="incident-detail-tags">
                    <span className={`badge ${SEVERITY_META[selectedIncident.severity].cssClass}`}>
                      {SEVERITY_META[selectedIncident.severity].label} Severity
                    </span>
                    <StatusBadge status={selectedIncident.healthStatus} />
                    <span className="incident-status-pill">
                      State: {selectedIncident.status.toUpperCase()}
                    </span>
                  </div>
                  <h2 className="incident-detail-title">{selectedIncident.title}</h2>
                  <div className="incident-detail-sub">
                    <span>Vendor: <strong>{selectedIncident.vendorName}</strong></span>
                    <span>•</span>
                    <span>Detected: {new Date(selectedIncident.detectedAt).toLocaleString()}</span>
                    {selectedIncident.duration && (
                      <>
                        <span>•</span>
                        <span>Duration: {Math.round(selectedIncident.duration / 60000)} minutes</span>
                      </>
                    )}
                  </div>
                </div>

                <div className="incident-detail-actions">
                  {!ackList[selectedIncident.id] && selectedIncident.status !== 'resolved' && (
                    <button
                      className="btn btn--primary"
                      onClick={() => handleAcknowledge(selectedIncident.id)}
                    >
                      Acknowledge Incident
                    </button>
                  )}
                  <button
                    className="btn btn--outline"
                    onClick={() => handleRunDiagnostics(selectedIncident.dependencyId)}
                  >
                    Run Sweep Probes
                  </button>
                </div>
              </div>

              {/* Grey Failure & Root-Cause Callout */}
              <div className="grey-failure-insight-box">
                <div className="grey-failure-insight-title">
                  <AlertTriangle size={18} className="text-warning" />
                  <span>APIPulse Observability Intelligence</span>
                </div>
                <div className="grey-failure-insight-grid">
                  <div className="insight-metric">
                    <span className="insight-metric-label">Vendor Reported Status</span>
                    <span className="insight-metric-val insight-metric-val--vendor">
                      "{selectedIncident.vendorReportedStatus}"
                    </span>
                    <span className="insight-metric-note">Public status page response</span>
                  </div>

                  <div className="insight-metric">
                    <span className="insight-metric-label">APIPulse Ground Truth</span>
                    <span className="insight-metric-val insight-metric-val--real">
                      {getStatusMeta(selectedIncident.healthStatus).label}
                    </span>
                    <span className="insight-metric-note">
                      Confirmed via edge synthetic probes
                    </span>
                  </div>

                  <div className="insight-metric">
                    <span className="insight-metric-label">Circuit Breaker</span>
                    <span className="insight-metric-val insight-metric-val--circuit">
                      {selectedIncident.circuitAction
                        ? CIRCUIT_META[selectedIncident.circuitAction].label
                        : 'Standby'}
                    </span>
                    <span className="insight-metric-note">Autonomous protection layer</span>
                  </div>

                  <div className="insight-metric">
                    <span className="insight-metric-label">Notification Channels</span>
                    <div className="insight-channels-row">
                      {selectedIncident.alertsSent.map(ch => (
                        <span key={ch} className="channel-pill">
                          <Bell size={12} /> {ch.toUpperCase()}
                        </span>
                      ))}
                    </div>
                  </div>
                </div>

                <p className="grey-failure-analysis-text">
                  <strong>Diagnostic Verdict:</strong> Vendor's centralized health check passed, but APIPulse regional probes registered severe tail degradation in <code>{selectedIncident.affectedRegions.join(', ')}</code>. Latency spiked from baseline {formatLatency(selectedIncident.baselineP95Ms)} to {formatLatency(selectedIncident.peakP95Ms)}.
                </p>
              </div>

              {/* Timeline Section */}
              <div className="incident-timeline-section">
                <h3 className="section-title">Incident Event Timeline</h3>
                <div className="timeline-container">
                  {selectedIncident.timeline.map((evt, idx) => {
                    const isLast = idx === selectedIncident.timeline.length - 1;
                    return (
                      <div key={evt.id} className={`timeline-row ${isLast ? 'timeline-row--last' : ''}`}>
                        <div className="timeline-left">
                          <div className="timeline-node">
                            <div className="timeline-dot" />
                          </div>
                          {!isLast && <div className="timeline-connector" />}
                        </div>
                        <div className="timeline-body">
                          <div className="timeline-meta">
                            <span className="timeline-time">
                              {new Date(evt.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                            </span>
                            <span className="timeline-type-badge">{evt.type.replace(/_/g, ' ')}</span>
                          </div>
                          <p className="timeline-msg">{evt.message}</p>
                          {evt.metadata && (
                            <div className="timeline-meta-box">
                              {Object.entries(evt.metadata).map(([k, v]) => (
                                <span key={k} className="meta-tag">
                                  {k}: <code>{String(v)}</code>
                                </span>
                              ))}
                            </div>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          ) : (
            <div className="card incident-empty-detail">
              <Info size={32} />
              <p>Select an incident from the left column to view its diagnostic timeline.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
