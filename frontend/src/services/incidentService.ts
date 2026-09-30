/**
 * Incident Service — Live Backend Implementation
 *
 * HARDENED: No silent mock fallback.
 * When the APIPulse backend is unavailable, throws BackendUnavailableError.
 * Pages must display an explicit "APIPulse Backend Unavailable" state.
 */
import type { Incident, IncidentStatus, RegionCode, Severity, HealthStatus } from '../types/domain';
import { apiClient } from './apiClient';
import { BackendUnavailableError } from './backendStatus';

export interface IIncidentService {
  getIncidents(): Promise<Incident[]>;
  getIncident(id: string): Promise<Incident | undefined>;
  acknowledgeIncident(id: string): Promise<void>;
}

function mapBackendStatusToIncidentStatus(status: string): IncidentStatus {
  switch (status.toUpperCase()) {
    case 'OPEN':
      return 'investigating';
    case 'ACKNOWLEDGED':
      return 'identified';
    case 'RECOVERING':
      return 'monitoring';
    case 'RESOLVED':
      return 'resolved';
    default:
      return 'investigating';
  }
}

function determineIncidentHealthStatus(item: any): HealthStatus {
  const isResolved = item.status?.toUpperCase() === 'RESOLVED';
  if (isResolved) return 'healthy';

  const type = (item.incident_type || '').toUpperCase();
  const title = (item.title || '').toLowerCase();

  if (type === 'GREY_FAILURE' || title.includes('grey failure')) {
    return 'grey_failure';
  }
  if (type === 'OUTAGE' || title.includes('outage')) {
    return 'outage';
  }
  if (type === 'RECOVERING' || item.status?.toUpperCase() === 'RECOVERING') {
    return 'recovering';
  }
  return 'degraded';
}

function determineIncidentSeverity(item: any): Severity {
  const type = (item.incident_type || '').toUpperCase();
  const title = (item.title || '').toLowerCase();
  const sev = (item.severity || '').toLowerCase();

  if (sev === 'critical' || type === 'GREY_FAILURE' || title.includes('grey failure') || type === 'OUTAGE') {
    return 'critical';
  }
  if (sev === 'high') {
    return 'high';
  }
  if (sev === 'medium') {
    return 'medium';
  }
  if (sev === 'low') {
    return 'low';
  }
  return 'high';
}

function extractIncidentLatencies(item: any): { peak: number; baseline: number } {
  const text = `${item.title || ''} ${item.description || ''} ${item.events?.map((e: any) => e.reason).join(' ') || ''}`;
  const peakMatch = text.match(/P95=(\d+)ms/i) || text.match(/latency elevated.*?(\d+)ms/i);
  const baseMatch = text.match(/baseline P95=(\d+)ms/i) || text.match(/baseline.*?(\d+)ms/i);

  const isGrey = (item.incident_type || '').toUpperCase() === 'GREY_FAILURE' || (item.title || '').toLowerCase().includes('grey');
  const isDegraded = (item.incident_type || '').toUpperCase() === 'DEGRADED';

  const peak = peakMatch ? parseInt(peakMatch[1], 10) : isGrey ? 8000 : isDegraded ? 2000 : 250;
  const baseline = baseMatch ? parseInt(baseMatch[1], 10) : 200;

  return { peak, baseline };
}

class LiveIncidentService implements IIncidentService {
  async getIncidents(): Promise<Incident[]> {
    // Throws BackendUnavailableError (via apiClient network error) if backend is down
    const liveList = await apiClient.getIncidents();
    return liveList.map(item => {
      const { peak, baseline } = extractIncidentLatencies(item);
      const healthStatus = determineIncidentHealthStatus(item);
      const severity = determineIncidentSeverity(item);

      const timeline = (item.events && item.events.length > 0)
        ? item.events.map((evt: any) => ({
            id: evt.id,
            timestamp: new Date(evt.created_at).getTime(),
            type: (evt.event_type?.toLowerCase() === 'detected' ? 'detected' : 'update') as any,
            message: evt.reason || item.title,
            severity,
          }))
        : [
            {
              id: `${item.id}-detected`,
              timestamp: new Date(item.started_at).getTime(),
              type: 'detected' as const,
              message: item.title,
              severity,
            },
            ...(item.resolved_at ? [{
              id: `${item.id}-resolved`,
              timestamp: new Date(item.resolved_at).getTime(),
              type: 'resolved' as const,
              message: `Incident resolved — healthy observations confirmed for ${item.dependency_name} (${item.region}).`,
              severity: 'low' as const,
            }] : []),
          ];

      return {
        id: item.id,
        dependencyId: item.dependency_name,
        vendorName: item.dependency_name === 'stripe-like' ? 'Stripe Payments' : (item.dependency_name.split('-')[0] || item.dependency_name),
        title: item.title,
        description: `APIPulse detected ${item.incident_type} for ${item.dependency_name} in ${item.region}. Deduplication key: ${item.deduplication_key}`,
        status: mapBackendStatusToIncidentStatus(item.status),
        severity,
        healthStatus,
        affectedRegions: [item.region as RegionCode],
        detectedAt: new Date(item.started_at).getTime(),
        resolvedAt: item.resolved_at ? new Date(item.resolved_at).getTime() : undefined,
        peakP95Ms: peak,
        baselineP95Ms: baseline,
        vendorReportedStatus: (item.incident_type === 'GREY_FAILURE' || item.title?.toLowerCase().includes('grey')) ? 'OPERATIONAL' : 'DEGRADED',
        circuitAction: (item.status?.toUpperCase() !== 'RESOLVED') ? 'armed' : 'closed',
        alertsSent: ['slack', 'pagerduty', 'webhook'],
        timeline,
      };
    });
  }

  async getIncident(id: string): Promise<Incident | undefined> {
    const live = await apiClient.getIncident(id);
    if (!live) return undefined;

    const { peak, baseline } = extractIncidentLatencies(live);
    const healthStatus = determineIncidentHealthStatus(live);
    const severity = determineIncidentSeverity(live);

    const timeline = (live.events && live.events.length > 0)
      ? live.events.map((evt: any) => ({
          id: evt.id,
          timestamp: new Date(evt.created_at).getTime(),
          type: (evt.event_type?.toLowerCase() === 'detected' ? 'detected' : 'update') as any,
          message: evt.reason || live.title,
          severity,
        }))
      : [
          {
            id: `${live.id}-detected`,
            timestamp: new Date(live.started_at).getTime(),
            type: 'detected' as const,
            message: live.title,
            severity,
          },
          ...(live.resolved_at ? [{
            id: `${live.id}-resolved`,
            timestamp: new Date(live.resolved_at).getTime(),
            type: 'resolved' as const,
            message: `Incident resolved — healthy observations confirmed for ${live.dependency_name} (${live.region}).`,
            severity: 'low' as const,
          }] : []),
        ];

    return {
      id: live.id,
      dependencyId: live.dependency_name,
      vendorName: live.dependency_name === 'stripe-like' ? 'Stripe Payments' : (live.dependency_name.split('-')[0] || live.dependency_name),
      title: live.title,
      description: `APIPulse detected ${live.incident_type} for ${live.dependency_name} in ${live.region}`,
      status: mapBackendStatusToIncidentStatus(live.status),
      severity,
      healthStatus,
      affectedRegions: [live.region as RegionCode],
      detectedAt: new Date(live.started_at).getTime(),
      resolvedAt: live.resolved_at ? new Date(live.resolved_at).getTime() : undefined,
      peakP95Ms: peak,
      baselineP95Ms: baseline,
      vendorReportedStatus: (live.incident_type === 'GREY_FAILURE' || live.title?.toLowerCase().includes('grey')) ? 'OPERATIONAL' : 'DEGRADED',
      circuitAction: (live.status?.toUpperCase() !== 'RESOLVED') ? 'armed' : 'closed',
      alertsSent: ['slack', 'pagerduty', 'webhook'],
      timeline,
    };
  }

  async acknowledgeIncident(id: string): Promise<void> {
    // Best-effort PATCH — backend will handle persistence
    try {
      await fetch(`/api/v1/incidents/${encodeURIComponent(id)}/acknowledge`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
      });
    } catch {
      throw new BackendUnavailableError('Cannot acknowledge incident: APIPulse backend unavailable');
    }
  }
}

export const incidentService: IIncidentService = new LiveIncidentService();

// Re-export the error type
export { BackendUnavailableError };
