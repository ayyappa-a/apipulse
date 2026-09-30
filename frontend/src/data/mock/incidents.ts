/**
 * APIPulse Mock Data — Incidents
 * DEMO DATA ONLY
 */
import type { Incident } from '../../types/domain';

const now = Date.now();
const min = 60 * 1000;
const hr  = 60 * min;

export const MOCK_INCIDENTS: Incident[] = [
  // ── Active: Regional Grey Failure — SendGrid ───────────────
  {
    id: 'inc-001',
    dependencyId: 'dep-sendgrid-email',
    vendorName: 'SendGrid',
    title: 'Regional Grey Failure — AP-South',
    description:
      'APIPulse detected extreme tail latency in AP-South region for SendGrid Email API. P95 has risen 21× above baseline. Vendor status page reports "Operational". Grey failure classification active.',
    status: 'investigating',
    severity: 'high',
    healthStatus: 'regional_degradation',
    affectedRegions: ['ap-south-1'],
    detectedAt: now - 38 * min,
    peakP95Ms: 4820,
    baselineP95Ms: 224,
    vendorReportedStatus: 'Operational',
    circuitAction: 'armed',
    alertsSent: ['slack', 'pagerduty'],
    timeline: [
      {
        id: 'evt-001-1',
        timestamp: now - 38 * min,
        type: 'detected',
        message: 'APIPulse detected P95 latency spike in ap-south-1. P95 = 1.2s (baseline: 224ms).',
        severity: 'medium',
        metadata: { region: 'ap-south-1', p95: 1200, baseline: 224 },
      },
      {
        id: 'evt-001-2',
        timestamp: now - 35 * min,
        type: 'detected',
        message: 'P95 escalating: 2.4s. Error rate rising. Possible regional degradation.',
        severity: 'high',
        metadata: { region: 'ap-south-1', p95: 2400, errorRate: 3.2 },
      },
      {
        id: 'evt-001-3',
        timestamp: now - 32 * min,
        type: 'vendor_status_change',
        message: 'Vendor status page checked: SendGrid reports "Operational". Grey failure pattern confirmed.',
        severity: 'high',
        metadata: { vendorStatus: 'Operational' },
      },
      {
        id: 'evt-001-4',
        timestamp: now - 30 * min,
        type: 'identified',
        message: 'APIPulse classified as REGIONAL GREY FAILURE. P95 = 4.8s (21× baseline). Only ap-south-1 affected.',
        severity: 'high',
        metadata: { p95: 4820, baseline: 224, multiplier: 21.5, region: 'ap-south-1' },
      },
      {
        id: 'evt-001-5',
        timestamp: now - 28 * min,
        type: 'alert_sent',
        message: 'Slack alert dispatched to #alerts-prod. PagerDuty incident P-00481 created.',
        severity: 'info',
        metadata: { slackChannel: '#alerts-prod', pdIncident: 'P-00481' },
      },
      {
        id: 'evt-001-6',
        timestamp: now - 24 * min,
        type: 'circuit_opened',
        message: 'Circuit breaker ARMED for ap-south-1 probes. Threshold: P95 > 2000ms.',
        severity: 'high',
        metadata: { circuitState: 'armed', threshold: 2000 },
      },
      {
        id: 'evt-001-7',
        timestamp: now - 8 * min,
        type: 'update',
        message: 'P95 holding at 4.8s. No vendor acknowledgment. Monitoring continued.',
        severity: 'medium',
      },
    ],
  },

  // ── Active: Degraded — Twilio EU-West ─────────────────────
  {
    id: 'inc-002',
    dependencyId: 'dep-twilio-sms',
    vendorName: 'Twilio',
    title: 'Elevated Latency — EU-West',
    description:
      'Twilio SMS Gateway showing elevated latency in EU-West region. P95 at 1.24s vs 238ms baseline. Error rate at 1.8%. Monitoring for escalation.',
    status: 'monitoring',
    severity: 'medium',
    healthStatus: 'degraded',
    affectedRegions: ['eu-west-1'],
    detectedAt: now - 2 * hr - 14 * min,
    identifiedAt: now - 2 * hr - 5 * min,
    peakP95Ms: 1240,
    baselineP95Ms: 238,
    vendorReportedStatus: 'Operational',
    alertsSent: ['slack'],
    timeline: [
      {
        id: 'evt-002-1',
        timestamp: now - 2 * hr - 14 * min,
        type: 'detected',
        message: 'Elevated P95 detected in eu-west-1 for Twilio SMS. P95 = 580ms (baseline: 238ms).',
        severity: 'medium',
        metadata: { region: 'eu-west-1', p95: 580, baseline: 238 },
      },
      {
        id: 'evt-002-2',
        timestamp: now - 2 * hr - 5 * min,
        type: 'identified',
        message: 'Classified as DEGRADED. P95 = 1.24s (5.2× baseline). Error rate 1.8%.',
        severity: 'medium',
        metadata: { p95: 1240, baseline: 238, errorRate: 1.8 },
      },
      {
        id: 'evt-002-3',
        timestamp: now - 2 * hr,
        type: 'alert_sent',
        message: 'Slack alert dispatched to #alerts-infra.',
        severity: 'info',
      },
      {
        id: 'evt-002-4',
        timestamp: now - 1 * hr - 20 * min,
        type: 'update',
        message: 'Twilio acknowledged elevated error rates in EU region. Investigating.',
        severity: 'medium',
      },
      {
        id: 'evt-002-5',
        timestamp: now - 45 * min,
        type: 'update',
        message: 'Latency slightly improving. P95 = 980ms. Continuing to monitor.',
        severity: 'low',
        metadata: { p95: 980 },
      },
    ],
  },

  // ── Resolved: Stripe Outage (Historical) ──────────────────
  {
    id: 'inc-003',
    dependencyId: 'dep-stripe-payments',
    vendorName: 'Stripe',
    title: 'Stripe API Partial Outage — US-East',
    description:
      'Stripe Payments API experienced a partial outage in US-East. APIPulse detected 3× before vendor acknowledgment.',
    status: 'resolved',
    severity: 'critical',
    healthStatus: 'partial_outage',
    affectedRegions: ['us-east-1'],
    detectedAt: now - 3 * 24 * hr - 4 * hr - 22 * min,
    identifiedAt: now - 3 * 24 * hr - 4 * hr - 10 * min,
    resolvedAt: now - 3 * 24 * hr - 2 * hr - 15 * min,
    duration: 2 * hr + 7 * min,
    peakP95Ms: 8400,
    baselineP95Ms: 195,
    vendorReportedStatus: 'Degraded Performance',
    circuitAction: 'open',
    alertsSent: ['slack', 'pagerduty'],
    timeline: [
      {
        id: 'evt-003-1',
        timestamp: now - 3 * 24 * hr - 4 * hr - 22 * min,
        type: 'detected',
        message: 'APIPulse detected 3× latency spike before Stripe acknowledged.',
        severity: 'critical',
      },
      {
        id: 'evt-003-2',
        timestamp: now - 3 * 24 * hr - 4 * hr - 10 * min,
        type: 'circuit_opened',
        message: 'Circuit breaker OPENED for us-east-1.',
        severity: 'critical',
      },
      {
        id: 'evt-003-3',
        timestamp: now - 3 * 24 * hr - 2 * hr - 15 * min,
        type: 'resolved',
        message: 'Stripe returned to healthy. Circuit closed. Incident resolved.',
        severity: 'info',
      },
    ],
  },
];

export const getIncidentById = (id: string) =>
  MOCK_INCIDENTS.find(i => i.id === id);

export const getActiveIncidents = () =>
  MOCK_INCIDENTS.filter(i => i.status !== 'resolved');

export const getResolvedIncidents = () =>
  MOCK_INCIDENTS.filter(i => i.status === 'resolved');
