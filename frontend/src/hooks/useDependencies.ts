import { useState, useEffect, useCallback, useRef } from 'react';
import type { Dependency } from '../types/domain';
import { dependencyService } from '../services/dependencyService';

export function useDependencies(pollIntervalMs = 5000) {
  const [dependencies, setDependencies] = useState<Dependency[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isOffline, setIsOffline] = useState(false);
  const [lastSuccessAt, setLastSuccessAt] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const isFetchingRef = useRef(false);

  const fetchDeps = useCallback(async (isBackground = false) => {
    if (isFetchingRef.current) return;
    isFetchingRef.current = true;

    if (!isBackground) {
      setIsLoading(true);
    } else {
      setIsRefreshing(true);
    }
    setError(null);

    try {
      const data = await dependencyService.getDependencies();
      setDependencies(data);
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
    fetchDeps(false);

    if (pollIntervalMs > 0) {
      const id = setInterval(() => {
        fetchDeps(true);
      }, pollIntervalMs);
      return () => clearInterval(id);
    }
  }, [fetchDeps, pollIntervalMs]);

  return {
    dependencies,
    isLoading,
    isRefreshing,
    isOffline,
    lastSuccessAt,
    error,
    refetch: () => fetchDeps(false),
  };
}

export function useDependency(id?: string, pollIntervalMs = 5000) {
  const [dependency, setDependency] = useState<Dependency | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isOffline, setIsOffline] = useState(false);
  const [lastSuccessAt, setLastSuccessAt] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const isFetchingRef = useRef(false);

  const fetchDep = useCallback(async (isBackground = false) => {
    if (!id || isFetchingRef.current) return;
    isFetchingRef.current = true;

    if (!isBackground) {
      setIsLoading(true);
    } else {
      setIsRefreshing(true);
    }
    setError(null);

    try {
      const data = await dependencyService.getDependency(id);
      setDependency(data ?? null);
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
  }, [id]);

  useEffect(() => {
    fetchDep(false);

    if (pollIntervalMs > 0) {
      const intervalId = setInterval(() => {
        fetchDep(true);
      }, pollIntervalMs);
      return () => clearInterval(intervalId);
    }
  }, [fetchDep, pollIntervalMs]);

  return {
    dependency,
    isLoading,
    isRefreshing,
    isOffline,
    lastSuccessAt,
    error,
    refetch: () => fetchDep(false),
  };
}
