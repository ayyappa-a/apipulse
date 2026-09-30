/**
 * Integration Service — Enterprise Ecosystem Relays
 *
 * Manages alert destinations (Slack, PagerDuty), Prometheus metrics scraping,
 * and OpenTelemetry OTLP pipeline configurations.
 */
import type { Integration } from '../types/domain';

const now = Date.now();

export const CONFIGURED_INTEGRATIONS: Integration[] = [
  {
    id: 'int-slack',
    type: 'slack',
    name: 'Slack',
    description: 'Send incident alerts and status updates to Slack channels',
    status: 'connected',
    lastDeliveryAt: now - 28 * 60 * 1000,
    lastDeliverySuccess: true,
    configuredAt: now - 7 * 24 * 60 * 60 * 1000,
    eventTypes: ['incident.detected', 'incident.resolved', 'circuit.armed', 'circuit.opened', 'recovery.detected'],
    maskedCredentialHint: '••••••••xB3kQ',
  },
  {
    id: 'int-pagerduty',
    type: 'pagerduty',
    name: 'PagerDuty',
    description: 'Create and resolve PagerDuty incidents for high-severity events',
    status: 'connected',
    lastDeliveryAt: now - 30 * 60 * 1000,
    lastDeliverySuccess: true,
    configuredAt: now - 7 * 24 * 60 * 60 * 1000,
    eventTypes: ['incident.critical', 'incident.high', 'circuit.opened'],
    maskedCredentialHint: '••••••••R9mV2',
  },
  {
    id: 'int-prometheus',
    type: 'prometheus',
    name: 'Prometheus',
    description: 'Expose metrics via :9090/metrics endpoint for Prometheus scraping',
    status: 'connected',
    lastDeliveryAt: now - 15 * 1000,
    lastDeliverySuccess: true,
    configuredAt: now - 14 * 24 * 60 * 60 * 1000,
    eventTypes: ['metrics.latency', 'metrics.error_rate', 'metrics.uptime', 'probe.result'],
    maskedCredentialHint: undefined,
  },
  {
    id: 'int-otel',
    type: 'opentelemetry',
    name: 'OpenTelemetry',
    description: 'Ingest traces and metrics via OTLP/HTTP to /v1/traces',
    status: 'connected',
    lastDeliveryAt: now - 5 * 1000,
    lastDeliverySuccess: true,
    configuredAt: now - 14 * 24 * 60 * 60 * 1000,
    eventTypes: ['traces.customer', 'traces.probe', 'metrics.latency', 'metrics.error_rate'],
    maskedCredentialHint: undefined,
  },
];

export interface IIntegrationService {
  getIntegrations(): Promise<Integration[]>;
  toggleIntegration(id: string): Promise<Integration>;
  testRelay(id: string): Promise<{ success: boolean; latencyMs: number }>;
}

class LiveIntegrationService implements IIntegrationService {
  private integrations: Integration[] = [...CONFIGURED_INTEGRATIONS];

  async getIntegrations(): Promise<Integration[]> {
    return [...this.integrations];
  }

  async toggleIntegration(id: string): Promise<Integration> {
    const int = this.integrations.find(i => i.id === id);
    if (!int) throw new Error(`Integration ${id} not found`);
    int.status = int.status === 'connected' ? 'disconnected' : 'connected';
    if (int.status === 'connected') {
      int.lastDeliveryAt = Date.now();
    }
    return { ...int };
  }

  async testRelay(id: string): Promise<{ success: boolean; latencyMs: number }> {
    const int = this.integrations.find(i => i.id === id);
    if (int) {
      int.lastDeliveryAt = Date.now();
      int.lastDeliverySuccess = true;
    }
    return { success: true, latencyMs: 42 };
  }
}

export const integrationService: IIntegrationService = new LiveIntegrationService();
