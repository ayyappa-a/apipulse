/**
 * APIPulse Mock Data — Dependencies
 * DEMO DATA ONLY — Realistic SRE scenario including grey failure
 */
import type { Dependency } from '../../types/domain';
import { MOCK_VENDORS } from './vendors';
import { generateLatencyHistory } from './latencyGenerator';

const now = Date.now();

export const MOCK_DEPENDENCIES: Dependency[] = [
  // ── Stripe — Healthy ───────────────────────────────────────
  {
    id: 'dep-stripe-payments',
    vendor: MOCK_VENDORS.find(v => v.id === 'stripe')!,
    name: 'Stripe Payments API',
    endpoint: 'https://api.stripe.com/v1',
    status: 'healthy',
    circuitState: 'closed',
    latency: { p50: 84, p95: 198, p99: 320, mean: 98, min: 62, max: 380 },
    timing: { dns: 4, tcp: 8, tls: 18, ttfb: 62, transfer: 12, total: 104 },
    errorRate: 0.04,
    uptime: 99.98,
    lastProbeAt: now - 15000,
    createdAt: now - 1000 * 60 * 60 * 24 * 30,
    regions: ['us-east-1', 'us-west-2', 'eu-west-1'],
    regionalStatus: { 'us-east-1': 'healthy', 'us-west-2': 'healthy', 'eu-west-1': 'healthy' },
    regionalLatency: {
      'us-east-1': { p50: 80, p95: 190, p99: 310, mean: 95, min: 60, max: 360 },
      'us-west-2': { p50: 88, p95: 210, p99: 335, mean: 102, min: 65, max: 395 },
      'eu-west-1': { p50: 95, p95: 228, p99: 368, mean: 110, min: 72, max: 420 },
    },
    latencyHistory: generateLatencyHistory({ baseP50: 84, baseP95: 198, baseP99: 320 }),
    baselineP95: 195,
    tags: ['payments', 'critical', 'pci'],
  },

  // ── SendGrid — GREY FAILURE SCENARIO ──────────────────────
  {
    id: 'dep-sendgrid-email',
    vendor: MOCK_VENDORS.find(v => v.id === 'sendgrid')!,
    name: 'SendGrid Email Delivery',
    endpoint: 'https://api.sendgrid.com/v3',
    status: 'regional_degradation',
    circuitState: 'armed',
    latency: { p50: 320, p95: 4820, p99: 8940, mean: 980, min: 88, max: 12400 },
    timing: { dns: 5, tcp: 12, tls: 24, ttfb: 4779, transfer: 22, total: 4842 },
    errorRate: 8.7,
    uptime: 91.3,
    lastProbeAt: now - 8000,
    createdAt: now - 1000 * 60 * 60 * 24 * 45,
    regions: ['us-east-1', 'eu-west-1', 'ap-south-1'],
    regionalStatus: {
      'us-east-1': 'healthy',
      'eu-west-1': 'healthy',
      'ap-south-1': 'regional_degradation',
    },
    regionalLatency: {
      'us-east-1': { p50: 90, p95: 210, p99: 340, mean: 105, min: 68, max: 390 },
      'eu-west-1': { p50: 108, p95: 255, p99: 412, mean: 124, min: 80, max: 468 },
      'ap-south-1': { p50: 480, p95: 4820, p99: 8940, mean: 1840, min: 120, max: 12400 },
    },
    latencyHistory: generateLatencyHistory({
      baseP50: 95, baseP95: 220, baseP99: 380,
      anomalyStartIndex: 40, anomalyDuration: 8, anomalyMultiplier: 22,
      errorRateBase: 0.06, anomalyErrorRate: 8.7,
    }),
    baselineP95: 224,
    activeIncidentId: 'inc-001',
    tags: ['email', 'transactional', 'non-critical'],
  },

  // ── Auth0 — Healthy ────────────────────────────────────────
  {
    id: 'dep-auth0',
    vendor: MOCK_VENDORS.find(v => v.id === 'auth0')!,
    name: 'Auth0 Authentication',
    endpoint: 'https://yourtenant.auth0.com',
    status: 'healthy',
    circuitState: 'closed',
    latency: { p50: 74, p95: 168, p99: 284, mean: 87, min: 52, max: 320 },
    timing: { dns: 2, tcp: 6, tls: 14, ttfb: 54, transfer: 8, total: 84 },
    errorRate: 0.02,
    uptime: 99.99,
    lastProbeAt: now - 12000,
    createdAt: now - 1000 * 60 * 60 * 24 * 60,
    regions: ['us-east-1', 'us-west-2', 'eu-central-1'],
    regionalStatus: { 'us-east-1': 'healthy', 'us-west-2': 'healthy', 'eu-central-1': 'healthy' },
    regionalLatency: {
      'us-east-1': { p50: 70, p95: 162, p99: 275, mean: 82, min: 50, max: 308 },
      'us-west-2': { p50: 78, p95: 178, p99: 294, mean: 92, min: 56, max: 335 },
      'eu-central-1': { p50: 84, p95: 195, p99: 318, mean: 99, min: 60, max: 368 },
    },
    latencyHistory: generateLatencyHistory({ baseP50: 74, baseP95: 168, baseP99: 284 }),
    baselineP95: 168,
    tags: ['auth', 'critical', 'identity'],
  },

  // ── Twilio — Degraded ──────────────────────────────────────
  {
    id: 'dep-twilio-sms',
    vendor: MOCK_VENDORS.find(v => v.id === 'twilio')!,
    name: 'Twilio SMS Gateway',
    endpoint: 'https://api.twilio.com/2010-04-01',
    status: 'degraded',
    circuitState: 'closed',
    latency: { p50: 185, p95: 642, p99: 1240, mean: 280, min: 94, max: 2180 },
    timing: { dns: 5, tcp: 10, tls: 22, ttfb: 605, transfer: 18, total: 660 },
    errorRate: 1.8,
    uptime: 98.2,
    lastProbeAt: now - 18000,
    createdAt: now - 1000 * 60 * 60 * 24 * 20,
    regions: ['us-east-1', 'eu-west-1', 'ap-southeast-1'],
    regionalStatus: {
      'us-east-1': 'healthy',
      'eu-west-1': 'degraded',
      'ap-southeast-1': 'healthy',
    },
    regionalLatency: {
      'us-east-1': { p50: 98, p95: 230, p99: 380, mean: 115, min: 72, max: 430 },
      'eu-west-1': { p50: 320, p95: 1240, p99: 2480, mean: 560, min: 140, max: 3200 },
      'ap-southeast-1': { p50: 126, p95: 295, p99: 488, mean: 148, min: 88, max: 545 },
    },
    latencyHistory: generateLatencyHistory({
      baseP50: 185, baseP95: 642, baseP99: 1240,
      anomalyStartIndex: 35, anomalyDuration: 12, anomalyMultiplier: 3.5,
      errorRateBase: 0.1, anomalyErrorRate: 1.8,
    }),
    baselineP95: 238,
    activeIncidentId: 'inc-002',
    tags: ['sms', 'notifications'],
  },

  // ── AWS S3 — Healthy ───────────────────────────────────────
  {
    id: 'dep-aws-s3',
    vendor: MOCK_VENDORS.find(v => v.id === 'aws-s3')!,
    name: 'AWS S3 Object Storage',
    endpoint: 'https://s3.amazonaws.com',
    status: 'healthy',
    circuitState: 'closed',
    latency: { p50: 48, p95: 124, p99: 218, mean: 58, min: 28, max: 280 },
    timing: { dns: 2, tcp: 5, tls: 10, ttfb: 35, transfer: 10, total: 62 },
    errorRate: 0.01,
    uptime: 99.99,
    lastProbeAt: now - 5000,
    createdAt: now - 1000 * 60 * 60 * 24 * 90,
    regions: ['us-east-1', 'us-west-2', 'eu-west-1', 'ap-south-1'],
    regionalStatus: {
      'us-east-1': 'healthy',
      'us-west-2': 'healthy',
      'eu-west-1': 'healthy',
      'ap-south-1': 'healthy',
    },
    regionalLatency: {
      'us-east-1': { p50: 44, p95: 118, p99: 208, mean: 54, min: 26, max: 265 },
      'us-west-2': { p50: 52, p95: 132, p99: 228, mean: 62, min: 30, max: 295 },
      'eu-west-1': { p50: 64, p95: 158, p99: 268, mean: 76, min: 38, max: 322 },
      'ap-south-1': { p50: 88, p95: 198, p99: 335, mean: 105, min: 52, max: 395 },
    },
    latencyHistory: generateLatencyHistory({ baseP50: 48, baseP95: 124, baseP99: 218 }),
    baselineP95: 124,
    tags: ['storage', 'infrastructure', 'critical'],
  },

  // ── Cloudflare — Healthy ───────────────────────────────────
  {
    id: 'dep-cloudflare-cdn',
    vendor: MOCK_VENDORS.find(v => v.id === 'cloudflare')!,
    name: 'Cloudflare CDN',
    endpoint: 'https://api.cloudflare.com/client/v4',
    status: 'healthy',
    circuitState: 'closed',
    latency: { p50: 22, p95: 68, p99: 112, mean: 28, min: 12, max: 148 },
    timing: { dns: 1, tcp: 3, tls: 6, ttfb: 16, transfer: 4, total: 30 },
    errorRate: 0.0,
    uptime: 100.0,
    lastProbeAt: now - 10000,
    createdAt: now - 1000 * 60 * 60 * 24 * 75,
    regions: ['us-east-1', 'us-west-2', 'eu-west-1', 'eu-central-1', 'ap-southeast-1'],
    regionalStatus: {
      'us-east-1': 'healthy',
      'us-west-2': 'healthy',
      'eu-west-1': 'healthy',
      'eu-central-1': 'healthy',
      'ap-southeast-1': 'healthy',
    },
    regionalLatency: {
      'us-east-1': { p50: 20, p95: 62, p99: 104, mean: 26, min: 10, max: 138 },
      'us-west-2': { p50: 24, p95: 72, p99: 118, mean: 30, min: 14, max: 155 },
      'eu-west-1': { p50: 28, p95: 80, p99: 130, mean: 34, min: 16, max: 168 },
      'eu-central-1': { p50: 26, p95: 76, p99: 124, mean: 32, min: 14, max: 160 },
      'ap-southeast-1': { p50: 34, p95: 92, p99: 148, mean: 40, min: 20, max: 188 },
    },
    latencyHistory: generateLatencyHistory({ baseP50: 22, baseP95: 68, baseP99: 112 }),
    baselineP95: 68,
    tags: ['cdn', 'infrastructure'],
  },

  // ── OpenAI — Healthy ───────────────────────────────────────
  {
    id: 'dep-openai-api',
    vendor: MOCK_VENDORS.find(v => v.id === 'openai')!,
    name: 'OpenAI Completions API',
    endpoint: 'https://api.openai.com/v1',
    status: 'healthy',
    circuitState: 'closed',
    latency: { p50: 820, p95: 2840, p99: 4920, mean: 1100, min: 380, max: 8200 },
    timing: { dns: 3, tcp: 7, tls: 15, ttfb: 795, transfer: 280, total: 1100 },
    errorRate: 0.12,
    uptime: 99.72,
    lastProbeAt: now - 20000,
    createdAt: now - 1000 * 60 * 60 * 24 * 15,
    regions: ['us-east-1', 'eu-west-1'],
    regionalStatus: { 'us-east-1': 'healthy', 'eu-west-1': 'healthy' },
    regionalLatency: {
      'us-east-1': { p50: 800, p95: 2780, p99: 4820, mean: 1060, min: 360, max: 7980 },
      'eu-west-1': { p50: 980, p95: 3200, p99: 5600, mean: 1340, min: 440, max: 9200 },
    },
    latencyHistory: generateLatencyHistory({
      baseP50: 820, baseP95: 2840, baseP99: 4920, jitter: 0.25,
    }),
    baselineP95: 2840,
    tags: ['ml', 'ai', 'inference'],
  },

  // ── Datadog — Healthy ──────────────────────────────────────
  {
    id: 'dep-datadog-api',
    vendor: MOCK_VENDORS.find(v => v.id === 'datadog')!,
    name: 'Datadog Metrics API',
    endpoint: 'https://api.datadoghq.com/api/v2',
    status: 'healthy',
    circuitState: 'closed',
    latency: { p50: 62, p95: 148, p99: 248, mean: 74, min: 40, max: 295 },
    timing: { dns: 3, tcp: 6, tls: 12, ttfb: 45, transfer: 10, total: 76 },
    errorRate: 0.03,
    uptime: 99.97,
    lastProbeAt: now - 7000,
    createdAt: now - 1000 * 60 * 60 * 24 * 50,
    regions: ['us-east-1', 'eu-west-1'],
    regionalStatus: { 'us-east-1': 'healthy', 'eu-west-1': 'healthy' },
    regionalLatency: {
      'us-east-1': { p50: 58, p95: 140, p99: 235, mean: 70, min: 38, max: 280 },
      'eu-west-1': { p50: 72, p95: 168, p99: 278, mean: 85, min: 46, max: 325 },
    },
    latencyHistory: generateLatencyHistory({ baseP50: 62, baseP95: 148, baseP99: 248 }),
    baselineP95: 148,
    tags: ['analytics', 'monitoring'],
  },
];

export const getDependencyById = (id: string): Dependency | undefined =>
  MOCK_DEPENDENCIES.find(d => d.id === id);
