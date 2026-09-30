// ============================================================
// APIPulse Domain Types
// ============================================================

// ─── Health / Status ─────────────────────────────────────────

export type HealthStatus =
  | 'healthy'
  | 'degraded'
  | 'regional_degradation'
  | 'grey_failure'
  | 'partial_outage'
  | 'outage'
  | 'recovering'
  | 'unknown';

export type CircuitState = 'closed' | 'open' | 'half_open' | 'armed';

export type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info';

// ─── Regions ─────────────────────────────────────────────────

export type RegionCode =
  | 'us-east-1'
  | 'us-west-2'
  | 'eu-west-1'
  | 'eu-central-1'
  | 'ap-south-1'
  | 'ap-southeast-1'
  | 'ap-northeast-1'
  | 'sa-east-1';

export interface Region {
  code: RegionCode;
  label: string;
  shortLabel: string;
  lat: number;
  lon: number;
  status: HealthStatus;
  latencyP50: number;
  latencyP95: number;
  latencyP99: number;
  probeCount: number;
  successRate: number;
}

// ─── Telemetry Source & Provenance ───────────────────────────

export type TelemetrySource =
  | 'SYNTHETIC_PROBE'       // Observed by APIPulse edge fleet probes
  | 'CUSTOMER_OTEL'         // Ingested from customer OpenTelemetry spans/RUM
  | 'VENDOR_STATUS'         // Scraped from public vendor status page
  | 'APIPULSE_ASSESSMENT';  // Computed by APIPulse intelligence engine

export interface TelemetryProvenance {
  source: TelemetrySource;
  sourceLabel: string;
  collectedAt: number;
  collectorNode?: string;
  verified: boolean;
}

// ─── Latency Metrics ─────────────────────────────────────────

export interface LatencyMetrics {
  p50: number;  // ms
  p95: number;
  p99: number;
  mean: number;
  min: number;
  max: number;
  source?: TelemetrySource;
}

export interface TimingBreakdown {
  dns: number;
  tcp: number;
  tls: number;
  ttfb: number;
  transfer: number;
  total: number;
}

export interface LatencyDataPoint {
  timestamp: number; // epoch ms
  p50: number;
  p95: number;
  p99: number;
  mean: number;
  errorRate: number;
  source?: TelemetrySource;
}

export interface ProbeActivity {
  id: string;
  region: string;
  vendor: string;
  latency: number;
  status: 'ok' | 'slow' | 'error';
  timestamp: number;
  source: TelemetrySource;
}

export interface ProbeExecutionResult {
  region: RegionCode;
  statusCode: number;
  dns: number;
  tcp: number;
  tls: number;
  ttfb: number;
  transfer: number;
  total: number;
  success: boolean;
  certIssuer: string;
  ipAddress: string;
  source: TelemetrySource;
}

// ─── Vendor / Dependency ─────────────────────────────────────

export type VendorCategory =
  | 'payment'
  | 'auth'
  | 'messaging'
  | 'storage'
  | 'cdn'
  | 'database'
  | 'ml'
  | 'communication'
  | 'analytics'
  | 'infrastructure';

export interface Vendor {
  id: string;
  name: string;
  displayName: string;
  logoUrl?: string;
  category: VendorCategory;
  statusPageUrl?: string;
  vendorReportedStatus: 'operational' | 'degraded_performance' | 'partial_outage' | 'major_outage' | 'maintenance';
  vendorStatusUpdatedAt: number;
}

export interface Dependency {
  id: string;
  vendor: Vendor;
  name: string;
  endpoint: string;
  status: HealthStatus;
  circuitState: CircuitState;
  latency: LatencyMetrics;
  timing: TimingBreakdown;
  errorRate: number;          // 0–100 %
  uptime: number;             // 0–100 %
  lastProbeAt: number;        // epoch ms
  createdAt: number;
  regions: RegionCode[];
  regionalStatus: Partial<Record<RegionCode, HealthStatus>>;
  regionalLatency: Partial<Record<RegionCode, LatencyMetrics>>;
  latencyHistory: LatencyDataPoint[];
  baselineP95: number;        // historical baseline ms
  activeIncidentId?: string;
  tags: string[];
}

// ─── Probes ──────────────────────────────────────────────────

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'HEAD' | 'OPTIONS';
export type ProbeStatus = 'active' | 'paused' | 'error' | 'pending';

export interface ProbeConfig {
  id: string;
  name: string;
  dependencyId: string;
  endpoint: string;
  method: HttpMethod;
  headers: Record<string, string>;
  body?: string;
  intervalSeconds: number;
  timeoutMs: number;
  regions: RegionCode[];
  expectedStatusCodes: number[];
  expectedBodyContains?: string;
  followRedirects: boolean;
  authType: 'none' | 'bearer' | 'basic' | 'api_key';
  // NOTE: auth credentials are NEVER stored in frontend — server-side only
  thresholds: {
    degradedP95Ms: number;
    criticalP95Ms: number;
    errorRatePercent: number;
  };
  status: ProbeStatus;
  createdAt: number;
  updatedAt: number;
}

export interface ProbeResult {
  id: string;
  probeId: string;
  dependencyId: string;
  region: RegionCode;
  timestamp: number;
  statusCode: number;
  latency: TimingBreakdown;
  success: boolean;
  errorMessage?: string;
}

// ─── Incidents ───────────────────────────────────────────────

export type IncidentStatus = 'investigating' | 'identified' | 'monitoring' | 'resolved';

export interface IncidentEvent {
  id: string;
  timestamp: number;
  type: 'detected' | 'identified' | 'alert_sent' | 'circuit_opened' | 'circuit_closed' | 'resolved' | 'update' | 'vendor_status_change';
  message: string;
  severity?: Severity;
  metadata?: Record<string, string | number | boolean>;
}

export interface Incident {
  id: string;
  dependencyId: string;
  vendorName: string;
  title: string;
  description: string;
  status: IncidentStatus;
  severity: Severity;
  healthStatus: HealthStatus;
  affectedRegions: RegionCode[];
  detectedAt: number;
  identifiedAt?: number;
  resolvedAt?: number;
  duration?: number;         // ms, if resolved
  peakP95Ms: number;
  baselineP95Ms: number;
  vendorReportedStatus: string;
  circuitAction?: CircuitState;
  alertsSent: string[];      // e.g. ['slack', 'pagerduty']
  timeline: IncidentEvent[];
}

// ─── Integrations ────────────────────────────────────────────

export type IntegrationType = 'slack' | 'pagerduty' | 'prometheus' | 'opentelemetry' | 'webhook';
export type IntegrationStatus = 'connected' | 'disconnected' | 'error' | 'pending';

export interface Integration {
  id: string;
  type: IntegrationType;
  name: string;
  description: string;
  status: IntegrationStatus;
  lastDeliveryAt?: number;
  lastDeliverySuccess?: boolean;
  configuredAt?: number;
  eventTypes: string[];
  // SECURITY: Credentials never stored in frontend — server-side only
  // Display masked token hint only
  maskedCredentialHint?: string;
}

// ─── Alert Policies ──────────────────────────────────────────

export type AlertChannel = 'slack' | 'pagerduty' | 'email' | 'webhook';

export interface AlertPolicy {
  id: string;
  name: string;
  dependencyIds: string[];  // empty = all
  conditions: {
    healthStatus?: HealthStatus[];
    p95ThresholdMs?: number;
    errorRatePercent?: number;
    regions?: RegionCode[];
  };
  channels: AlertChannel[];
  severity: Severity;
  enabled: boolean;
  createdAt: number;
}

// ─── Analytics / Summary ─────────────────────────────────────

export interface PlatformSummary {
  totalDependencies: number;
  healthyCount: number;
  degradedCount: number;
  outageCount: number;
  activeIncidents: number;
  p50Overall: number;
  p95Overall: number;
  p99Overall: number;
  uptimeOverall: number;   // %
  circuitOpenCount: number;
  lastUpdatedAt: number;
}

export interface TimeRange {
  label: string;
  value: '1h' | '6h' | '24h' | '7d' | '30d';
  durationMs: number;
}

// ─── UI State ────────────────────────────────────────────────

export interface FilterState {
  search: string;
  status: HealthStatus | 'all';
  region: RegionCode | 'all';
  vendor: string | 'all';
  timeRange: TimeRange;
}

export const TIME_RANGES: TimeRange[] = [
  { label: '1h',  value: '1h',  durationMs: 60 * 60 * 1000 },
  { label: '6h',  value: '6h',  durationMs: 6 * 60 * 60 * 1000 },
  { label: '24h', value: '24h', durationMs: 24 * 60 * 60 * 1000 },
  { label: '7d',  value: '7d',  durationMs: 7 * 24 * 60 * 60 * 1000 },
  { label: '30d', value: '30d', durationMs: 30 * 24 * 60 * 60 * 1000 },
];
