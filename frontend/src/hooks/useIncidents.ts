import { useState, useEffect, useCallback, useRef } from 'react';
import type { Incident } from '../types/domain';
import { incidentService } from '../services/incidentService';

export function useIncidents(pollIntervalMs = 5000) {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isOffline, setIsOffline] = useState(false);
  const [lastSuccessAt, setLastSuccessAt] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const isFetchingRef = useRef(false);

  const fetchIncidents = useCallback(async (isBackground = false) => {
    if (isFetchingRef.current) return;
    isFetchingRef.current = true;

    if (!isBackground) {
      setIsLoading(true);
    } else {
      setIsRefreshing(true);
    }
    setError(null);

    try {
      const data = await incidentService.getIncidents();
      setIncidents(data);
      setIsOffline(false);
      setLastSuccessAt(Date.now());
    } catch (err: any) {
      setIsOffline(true);
      setError(err?.message || 'APIPulse backend unavailable');
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
      isFetchingRef.current = false;
    }
  }, []);

  useEffect(() => {
    fetchIncidents(false);

    if (pollIntervalMs > 0) {
      const intervalId = setInterval(() => {
        fetchIncidents(true);
      }, pollIntervalMs);
      return () => clearInterval(intervalId);
    }
  }, [fetchIncidents, pollIntervalMs]);

  const acknowledge = async (id: string) => {
    await incidentService.acknowledgeIncident(id);
    setIncidents(prev => prev.map(i => i.id === id ? { ...i, status: 'identified' } : i));
  };

  const activeCount = incidents.filter(i => i.status !== 'resolved').length;
  const criticalCount = incidents.filter(i => i.severity === 'critical').length;
  const resolvedCount = incidents.filter(i => i.status === 'resolved').length;

  return {
    incidents,
    activeCount,
    criticalCount,
    resolvedCount,
    isLoading,
    isRefreshing,
    isOffline,
    lastSuccessAt,
    error,
    acknowledge,
    refetch: () => fetchIncidents(false),
  };
}
