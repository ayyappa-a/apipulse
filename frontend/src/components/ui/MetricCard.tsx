import React from 'react';
import { TrendingUp, TrendingDown, Minus } from 'lucide-react';
import { formatLatency } from '../../utils/status';

interface MetricCardProps {
  label: string;
  value: string | number;
  unit?: string;
  subLabel?: string;
  trend?: 'up' | 'down' | 'neutral';
  trendValue?: string;
  trendGoodDirection?: 'up' | 'down'; // which direction is "good"
  color?: string;
  large?: boolean;
  className?: string;
}

export function MetricCard({
  label,
  value,
  unit,
  subLabel,
  trend,
  trendValue,
  trendGoodDirection = 'down',
  color,
  large,
  className = '',
}: MetricCardProps) {
  const trendGood = trend === trendGoodDirection;
  const trendBad  = trend !== 'neutral' && trend !== undefined && !trendGood;

  return (
    <div className={`card metric-card ${className}`}>
      <p className="metric-label">{label}</p>
      <div className="metric-card__value-row">
        <span
          className={`metric-value ${large ? 'metric-value--lg' : ''}`}
          style={color ? { color } : undefined}
        >
          {value}
        </span>
        {unit && <span className="metric-card__unit">{unit}</span>}
      </div>
      {(trend || subLabel) && (
        <div className="metric-card__footer">
          {trend && trend !== 'neutral' && (
            <span
              className="metric-card__trend"
              style={{ color: trendGood ? 'var(--color-healthy-text)' : 'var(--color-critical-text)' }}
              aria-label={`Trend: ${trend}`}
            >
              {trend === 'up' ? <TrendingUp size={12} /> : <TrendingDown size={12} />}
              {trendValue}
            </span>
          )}
          {trend === 'neutral' && (
            <span className="metric-card__trend" style={{ color: 'var(--color-text-tertiary)' }}>
              <Minus size={12} />
              {trendValue}
            </span>
          )}
          {subLabel && <span className="metric-card__sublabel">{subLabel}</span>}
        </div>
      )}
    </div>
  );
}

interface LatencyMetricRowProps {
  p50: number;
  p95: number;
  p99: number;
  baseline?: number;
}

export function LatencyMetricRow({ p50, p95, p99, baseline }: LatencyMetricRowProps) {
  const p95Color = baseline
    ? p95 > baseline * 5 ? 'var(--color-critical-text)'
    : p95 > baseline * 2 ? 'var(--color-warning-text)'
    : p95 > baseline * 1.5 ? 'var(--color-degraded-text)'
    : 'var(--color-healthy-text)'
    : undefined;

  return (
    <div className="latency-row">
      <div className="latency-row__item">
        <span className="latency-row__label">P50</span>
        <span className="latency-row__value mono">{formatLatency(p50)}</span>
      </div>
      <div className="latency-row__item">
        <span className="latency-row__label">P95</span>
        <span className="latency-row__value mono" style={p95Color ? { color: p95Color } : undefined}>
          {formatLatency(p95)}
        </span>
      </div>
      <div className="latency-row__item">
        <span className="latency-row__label">P99</span>
        <span className="latency-row__value mono">{formatLatency(p99)}</span>
      </div>
    </div>
  );
}
