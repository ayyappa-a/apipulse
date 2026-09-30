/**
 * APIPulse Mock Data — Latency History Generator
 * DEMO DATA ONLY — Generates realistic time-series telemetry
 */
import type { LatencyDataPoint } from '../../types/domain';

/**
 * Generate a realistic latency history for the given number of data points.
 * Supports injecting an anomaly (grey failure) period.
 */
export function generateLatencyHistory(
  options: {
    points?: number;
    intervalMs?: number;
    baseP50?: number;
    baseP95?: number;
    baseP99?: number;
    anomalyStartIndex?: number;
    anomalyDuration?: number;
    anomalyMultiplier?: number;
    errorRateBase?: number;
    anomalyErrorRate?: number;
    jitter?: number;
  } = {}
): LatencyDataPoint[] {
  const {
    points = 48,
    intervalMs = 30 * 60 * 1000,  // 30-min buckets
    baseP50 = 120,
    baseP95 = 280,
    baseP99 = 490,
    anomalyStartIndex,
    anomalyDuration = 6,
    anomalyMultiplier = 12,
    errorRateBase = 0.05,
    anomalyErrorRate = 4.2,
    jitter = 0.15,
  } = options;

  const now = Date.now();
  const history: LatencyDataPoint[] = [];

  for (let i = 0; i < points; i++) {
    const timestamp = now - (points - i) * intervalMs;
    const inAnomaly =
      anomalyStartIndex !== undefined &&
      i >= anomalyStartIndex &&
      i < anomalyStartIndex + anomalyDuration;

    const jitterFactor = 1 + (Math.random() - 0.5) * 2 * jitter;
    const multiplier = inAnomaly ? anomalyMultiplier * (0.8 + Math.random() * 0.4) : 1;

    history.push({
      timestamp,
      p50:       Math.round(baseP50  * jitterFactor * multiplier),
      p95:       Math.round(baseP95  * jitterFactor * multiplier),
      p99:       Math.round(baseP99  * jitterFactor * multiplier),
      mean:      Math.round(baseP50  * jitterFactor * multiplier * 1.1),
      errorRate: inAnomaly
        ? parseFloat((anomalyErrorRate * (0.7 + Math.random() * 0.6)).toFixed(2))
        : parseFloat((errorRateBase   * (0.5 + Math.random())).toFixed(2)),
    });
  }

  return history;
}

/**
 * Generate a short sparkline of the last N data points
 */
export function generateSparkline(
  points: number,
  base: number,
  jitter = 0.2,
  trend: 'up' | 'down' | 'flat' | 'spike' = 'flat'
): number[] {
  return Array.from({ length: points }, (_, i) => {
    const j = 1 + (Math.random() - 0.5) * 2 * jitter;
    let trendFactor = 1;
    if (trend === 'up')   trendFactor = 1 + (i / points) * 0.5;
    if (trend === 'down') trendFactor = 1 - (i / points) * 0.3;
    if (trend === 'spike' && i === Math.floor(points * 0.7)) trendFactor = 4;
    return Math.round(base * j * trendFactor);
  });
}
