/**
 * APIPulse Mock Data — Platform Summary
 * DEMO DATA ONLY
 */
import type { PlatformSummary, TimeRange } from '../../types/domain';

export const MOCK_PLATFORM_SUMMARY: PlatformSummary = {
  totalDependencies: 8,
  healthyCount: 5,
  degradedCount: 2,
  outageCount: 0,
  activeIncidents: 2,
  p50Overall: 184,
  p95Overall: 1082,
  p99Overall: 2384,
  uptimeOverall: 99.12,
  circuitOpenCount: 0,
  lastUpdatedAt: Date.now(),
};

export const TIME_RANGES: TimeRange[] = [
  { label: '1h',  value: '1h',  durationMs: 60 * 60 * 1000 },
  { label: '6h',  value: '6h',  durationMs: 6 * 60 * 60 * 1000 },
  { label: '24h', value: '24h', durationMs: 24 * 60 * 60 * 1000 },
  { label: '7d',  value: '7d',  durationMs: 7 * 24 * 60 * 60 * 1000 },
  { label: '30d', value: '30d', durationMs: 30 * 24 * 60 * 60 * 1000 },
];
