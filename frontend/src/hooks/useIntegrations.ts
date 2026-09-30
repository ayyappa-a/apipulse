import { useState, useEffect, useCallback } from 'react';
import type { Integration } from '../types/domain';
import { integrationService } from '../services/integrationService';

export function useIntegrations() {
  const [integrations, setIntegrations] = useState<Integration[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  const fetchIntegrations = useCallback(async () => {
    setIsLoading(true);
    const data = await integrationService.getIntegrations();
    setIntegrations(data);
    setIsLoading(false);
  }, []);

  useEffect(() => {
    fetchIntegrations();
  }, [fetchIntegrations]);

  const toggleConnect = async (id: string) => {
    const updated = await integrationService.toggleIntegration(id);
    setIntegrations(prev => prev.map(i => i.id === id ? updated : i));
    return updated;
  };

  const testRelay = async (id: string) => {
    return integrationService.testRelay(id);
  };

  return {
    integrations,
    isLoading,
    toggleConnect,
    testRelay,
    refetch: fetchIntegrations,
  };
}
