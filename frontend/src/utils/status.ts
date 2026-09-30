/**
 * APIPulse — Status Utilities
 * Centralized status color/label system
 */
import type { HealthStatus, CircuitState, Severity } from '../types/domain';

// ─── Status Display Metadata ──────────────────────────────────

export interface StatusMeta {
  label: string;
  shortLabel: string;
  cssClass: string;
  dotClass: string;
  description: string;
}

export const STATUS_META: Record<HealthStatus, StatusMeta> = {
  healthy: {
    label: 'Healthy',
    shortLabel: 'OK',
    cssClass: 'status-healthy',
    dotClass: 'health-dot--healthy',
    description: 'All systems operating within normal parameters',
  },
  degraded: {
    label: 'Degraded',
    shortLabel: 'DEG',
    cssClass: 'status-degraded',
    dotClass: 'health-dot--degraded',
    description: 'Elevated latency or error rate detected',
  },
  regional_degradation: {
    label: 'Regional Grey Failure',
    shortLabel: 'REG',
    cssClass: 'status-regional',
    dotClass: 'health-dot--regional',
    description: 'Degradation confined to specific geographic region(s)',
  },
  grey_failure: {
    label: 'Grey Failure',
    shortLabel: 'GREY',
    cssClass: 'status-critical',
    dotClass: 'health-dot--critical',
    description: 'Silent grey failure: severe latency while vendor declares operational',
  },
  partial_outage: {
    label: 'Partial Outage',
    shortLabel: 'PRT',
    cssClass: 'status-partial_outage',
    dotClass: 'health-dot--partial_outage',
    description: 'Subset of endpoints or regions are unavailable',
  },
  outage: {
    label: 'Outage',
    shortLabel: 'OUT',
    cssClass: 'status-outage',
    dotClass: 'health-dot--outage',
    description: 'Complete service disruption detected',
  },
  recovering: {
    label: 'Recovering',
    shortLabel: 'RCV',
    cssClass: 'status-recovering',
    dotClass: 'health-dot--recovering',
    description: 'Metrics trending toward recovery',
  },
  unknown: {
    label: 'Unknown',
    shortLabel: '???',
    cssClass: 'status-unknown',
    dotClass: 'health-dot--unknown',
    description: 'Status could not be determined',
  },
};

export const getStatusMeta = (status: HealthStatus): StatusMeta =>
  STATUS_META[status] ?? STATUS_META.unknown;

// ─── Circuit State ────────────────────────────────────────────

export const CIRCUIT_META: Record<CircuitState, { label: string; color: string; description: string }> = {
  closed: {
    label: 'Closed',
    color: 'var(--color-healthy-text)',
    description: 'Normal operation — requests flowing',
  },
  open: {
    label: 'Open',
    color: 'var(--color-critical-text)',
    description: 'Circuit open — requests blocked',
  },
  half_open: {
    label: 'Half-Open',
    color: 'var(--color-warning-text)',
    description: 'Trial requests allowed — testing recovery',
  },
  armed: {
    label: 'Armed',
    color: 'var(--color-degraded-text)',
    description: 'Circuit armed — ready to open on threshold breach',
  },
};

// ─── Severity ─────────────────────────────────────────────────

export const SEVERITY_META: Record<Severity, { label: string; color: string; bgVar: string; cssClass: string }> = {
  critical: { label: 'Critical', color: 'var(--color-critical-text)',   bgVar: 'var(--color-critical-dim)',    cssClass: 'badge--critical' },
  high:     { label: 'High',     color: 'var(--color-warning-text)',    bgVar: 'var(--color-warning-dim)',     cssClass: 'badge--warning' },
  medium:   { label: 'Medium',   color: 'var(--color-degraded-text)',   bgVar: 'var(--color-degraded-dim)',    cssClass: 'badge--degraded' },
  low:      { label: 'Low',      color: 'var(--color-accent-bright)',   bgVar: 'var(--color-accent-subtle)',   cssClass: 'badge--info' },
  info:     { label: 'Info',     color: 'var(--color-text-secondary)',  bgVar: 'var(--color-bg-elevated)',     cssClass: 'badge--neutral' },
};

// ─── Latency Formatting ───────────────────────────────────────

export function formatLatency(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

export function formatUptime(pct: number): string {
  return `${pct.toFixed(2)}%`;
}

export function formatErrorRate(rate: number): string {
  return `${rate.toFixed(2)}%`;
}

export function getLatencyColor(ms: number, p95Baseline: number): string {
  const ratio = ms / p95Baseline;
  if (ratio >= 10) return 'var(--color-critical-text)';
  if (ratio >= 3)  return 'var(--color-warning-text)';
  if (ratio >= 1.5)return 'var(--color-degraded-text)';
  return 'var(--color-healthy-text)';
}

// ─── Time Formatting ──────────────────────────────────────────

export function formatRelativeTime(timestamp: number): string {
  const diff = Date.now() - timestamp;
  const s = Math.floor(diff / 1000);
  if (s < 60)  return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60)  return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24)  return `${h}h ago`;
  const d = Math.floor(h / 24);
  return `${d}d ago`;
}

export function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60)   return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60)   return `${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

export function formatTimestamp(ts: number): string {
  return new Date(ts).toLocaleString('en-US', {
    month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
    hour12: false,
  });
}
