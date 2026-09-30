import { useState, useEffect, useCallback, useRef } from 'react';
import type { PlatformSummary } from '../types/domain';
import { telemetryService } from '../services/telemetryService';

export function usePlatformSummary(pollIntervalMs = 5000) {
  const [summary, setSummary] = useState<PlatformSummary | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isOffline, setIsOffline] = useState(false);
  const [lastSuccessAt, setLastSuccessAt] = useState<number | null>(null);
  const isFetchingRef = useRef(false);

  const fetchSummary = useCallback(async (isBackground = false) => {
    if (isFetchingRef.current) return;
    isFetchingRef.current = true;

    if (!isBackground) {
      setIsLoading(true);
    } else {
      setIsRefreshing(true);
    }

    try {
      const data = await telemetryService.getPlatformSummary();
      setSummary(data);
      setIsOffline(false);
      setLastSuccessAt(Date.now());
    } catch {
      setIsOffline(true);
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
      isFetchingRef.current = false;
    }
  }, []);

  useEffect(() => {
    fetchSummary(false);

    if (pollIntervalMs > 0) {
      const intervalId = setInterval(() => {
        fetchSummary(true);
      }, pollIntervalMs);
      return () => clearInterval(intervalId);
    }
  }, [fetchSummary, pollIntervalMs]);

  return { summary, isLoading, isRefreshing, isOffline, lastSuccessAt, refetch: () => fetchSummary(false) };
}
