import { useState, useEffect, useCallback } from 'react';
import type { ProbeConfig, ProbeExecutionResult, RegionCode } from '../types/domain';
import { probeService } from '../services/probeService';

export function useProbes() {
  const [probes, setProbes] = useState<ProbeConfig[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchProbes = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await probeService.getProbes();
      setProbes(data);
    } catch (err: any) {
      setError(err?.message || 'Failed to load probes');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchProbes();
  }, [fetchProbes]);

  const toggleStatus = async (id: string) => {
    const updated = await probeService.toggleProbeStatus(id);
    setProbes(prev => prev.map(p => p.id === id ? updated : p));
    return updated;
  };

  const runProbe = async (probeId: string, region: RegionCode): Promise<ProbeExecutionResult> => {
    return probeService.executeProbe(probeId, region);
  };

  const createProbe = async (data: Omit<ProbeConfig, 'id' | 'createdAt' | 'updatedAt'>) => {
    const created = await probeService.createProbe(data);
    setProbes(prev => [created, ...prev]);
    return created;
  };

  const activeCount = probes.filter(p => p.status === 'active').length;

  return {
    probes,
    activeCount,
    isLoading,
    error,
    toggleStatus,
    runProbe,
    createProbe,
    refetch: fetchProbes,
  };
}
