import React from 'react';
import type { HealthStatus } from '../../types/domain';
import { getStatusMeta } from '../../utils/status';

interface StatusBadgeProps {
  status: HealthStatus;
  size?: 'sm' | 'md';
  showDot?: boolean;
}

export function StatusBadge({ status, size = 'md', showDot = true }: StatusBadgeProps) {
  const meta = getStatusMeta(status);
  return (
    <span
      className={`badge ${meta.cssClass}`}
      style={size === 'sm' ? { fontSize: 'var(--text-xs)', padding: '1px 6px' } : undefined}
      aria-label={`Status: ${meta.label}`}
    >
      {showDot && <span className={`health-dot ${meta.dotClass}`} aria-hidden />}
      {meta.label}
    </span>
  );
}

interface HealthDotProps {
  status: HealthStatus;
  size?: number;
}

export function HealthDot({ status, size = 8 }: HealthDotProps) {
  const meta = getStatusMeta(status);
  return (
    <span
      className={`health-dot ${meta.dotClass}`}
      style={{ width: size, height: size }}
      aria-label={`Health: ${meta.label}`}
      role="img"
    />
  );
}
