import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Globe, Clock, Shield, ChevronRight } from 'lucide-react';
import { StatusBadge, HealthDot } from '../../components/ui/StatusBadge';
import { LatencyMetricRow } from '../../components/ui/MetricCard';
import { formatLatency, formatRelativeTime, formatUptime, formatErrorRate, getStatusMeta, CIRCUIT_META } from '../../utils/status';
import type { Dependency } from '../../types/domain';

interface DependencyCardProps {
  dependency: Dependency;
}

export function DependencyCard({ dependency: dep }: DependencyCardProps) {
  const navigate = useNavigate();
  const meta = getStatusMeta(dep.status);
  const cm = CIRCUIT_META[dep.circuitState];

  return (
    <div
      className="card dep-card"
      onClick={() => navigate(`/dependencies/${dep.id}`)}
      tabIndex={0}
      onKeyDown={e => e.key === 'Enter' && navigate(`/dependencies/${dep.id}`)}
      role="button"
      aria-label={`${dep.name} — ${meta.label}`}
    >
      {/* Card Header */}
      <div className="dep-card__header">
        <div className="flex items-center gap-3">
          <HealthDot status={dep.status} size={10} />
          <div>
            <h3 className="dep-card__vendor">{dep.vendor.displayName}</h3>
            <p className="dep-card__name">{dep.name}</p>
          </div>
        </div>
        <StatusBadge status={dep.status} size="sm" />
      </div>

      {/* Latency Strip */}
      <div className="dep-card__metrics">
        <div className="dep-card__metric">
          <span className="dep-card__metric-label">P50</span>
          <span className="dep-card__metric-value mono">{formatLatency(dep.latency.p50)}</span>
        </div>
        <div className="dep-card__metric">
          <span className="dep-card__metric-label">P95</span>
          <span
            className="dep-card__metric-value mono"
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
        </div>
        <div className="dep-card__metric">
          <span className="dep-card__metric-label">P99</span>
          <span className="dep-card__metric-value mono">{formatLatency(dep.latency.p99)}</span>
        </div>
        <div className="dep-card__metric">
          <span className="dep-card__metric-label">Error Rate</span>
          <span
            className="dep-card__metric-value mono"
            style={{ color: dep.errorRate > 1 ? 'var(--color-critical-text)' : 'var(--color-text-primary)' }}
          >
            {formatErrorRate(dep.errorRate)}
          </span>
        </div>
      </div>

      {/* Card Footer */}
      <div className="dep-card__footer">
        <div className="flex items-center gap-2">
          <span
            className="dep-card__circuit-badge"
            style={{ color: cm.color }}
            title={cm.description}
          >
            <Shield size={11} />
            <span>Circuit: {cm.label}</span>
          </span>
          <span className="text-secondary text-xs">·</span>
          <span className="text-secondary text-xs">{dep.regions.length} regions</span>
        </div>
        <div className="flex items-center gap-1 text-secondary text-xs">
          <span>{formatRelativeTime(dep.lastProbeAt)}</span>
          <ChevronRight size={14} />
        </div>
      </div>
    </div>
  );
}
