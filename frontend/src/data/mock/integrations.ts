/**
 * APIPulse Mock Data — Integrations
 * DEMO DATA ONLY — No real credentials
 */
import type { Integration } from '../../types/domain';

const now = Date.now();

export const MOCK_INTEGRATIONS: Integration[] = [
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
    description: 'Expose metrics via /metrics endpoint for Prometheus scraping',
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
    description: 'Export traces and metrics via OTLP to your observability backend',
    status: 'disconnected',
    configuredAt: undefined,
    eventTypes: ['traces.probe', 'metrics.latency', 'metrics.error_rate'],
    maskedCredentialHint: undefined,
  },
];
