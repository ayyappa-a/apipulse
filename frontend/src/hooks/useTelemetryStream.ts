import { useState, useEffect, useCallback, useRef } from 'react';
import type { ProbeActivity, Region } from '../types/domain';
import { apiClient } from '../services/apiClient';
import { telemetryService } from '../services/telemetryService';

export function useTelemetryStream() {
  const [probeLog, setProbeLog] = useState<ProbeActivity[]>([]);
  const [probeCount, setProbeCount] = useState<number>(0);
  const [isPaused, setIsPaused] = useState(false);
  const [regions, setRegions] = useState<Region[]>([]);
  const [isOffline, setIsOffline] = useState(false);
  const [lastSuccessAt, setLastSuccessAt] = useState<number | null>(null);
  const isPausedRef = useRef(isPaused);
  isPausedRef.current = isPaused;

  const fetchStreamData = useCallback(async () => {
    try {
      // 1. Fetch real probe observations from backend
      const [obs, regList] = await Promise.all([
        apiClient.getDependencyObservations('stripe-like', 'ap-south-1', 60),
        telemetryService.getRegions(),
      ]);

      const probeObs = obs?.probe_observations || [];
      const activities: ProbeActivity[] = probeObs.map(p => ({
        id: p.id,
        region: p.region === 'ap-south-1' ? 'AP-South' : p.region,
        vendor: 'stripe-like (Vendor Mock)',
        latency: Math.round(p.total_duration_ms),
        status: (!p.success ? 'error' : p.total_duration_ms > 1000 ? 'slow' : 'ok') as ProbeActivity['status'],
        timestamp: new Date(p.timestamp).getTime(),
        source: 'SYNTHETIC_PROBE' as const,
      })).reverse(); // newest first

      setProbeLog(activities);
      setProbeCount(probeObs.length);
      setRegions(regList);
      setIsOffline(false);
      setLastSuccessAt(Date.now());
    } catch {
      setIsOffline(true);
    }
  }, []);

  // Initial load
  useEffect(() => {
    fetchStreamData();
  }, [fetchStreamData]);

  // Periodic polling for genuine backend stream (3s interval)
  useEffect(() => {
    const intervalId = setInterval(() => {
      if (!isPausedRef.current) {
        fetchStreamData();
      }
    }, 3000);

    return () => clearInterval(intervalId);
  }, [fetchStreamData]);

  const togglePause = () => setIsPaused(p => !p);

  return {
    probeLog,
    probeCount,
    isPaused,
    regions,
    isOffline,
    lastSuccessAt,
    togglePause,
    refetch: fetchStreamData,
  };
}
