/**
 * APIPulse Core API Client
 * Provides typed, resilient access to the APIPulse backend.
 * Handles timeouts, network failures, and provides explicit availability detection.
 */

export interface BackendStatus {
  isAvailable: boolean;
  status: string;
  postgres: string;
  redis: string;
  healthEngine: string;
  lastChecked: number;
}

export interface ApiAssessment {
  id: string;
  dependency_name: string;
  region: string;
  timestamp: string;
  source: string;
  status: string;
  customer_signal: string;
  synthetic_signal: string;
  vendor_status: string;
  customer_latency_ms: number;
  synthetic_latency_ms: number;
  customer_baseline_p95_ms: number;
  synthetic_baseline_p95_ms: number;
  latency_multiplier: number;
  confidence: string;
  reason_codes: string[];
  explanation: string;
  recommended_action: string;
  customer_sample_count: number;
  synthetic_sample_count: number;
}

export interface ApiIncident {
  id: string;
  dependency_name: string;
  region: string;
  incident_type: string;
  status: string;
  title: string;
  severity: string;
  assessment_id: string;
  started_at: string;
  resolved_at?: string;
  created_at: string;
  updated_at: string;
  deduplication_key: string;
  events?: Array<{
    id: string;
    event_type: string;
    reason: string;
    dependency_name: string;
    region: string;
    created_at: string;
  }>;
}

export interface ApiCustomerObservation {
  id: string;
  timestamp: string;
  duration_ms: number;
  status: string;
  status_code: number;
  source: string;
  region: string;
  dependency_name: string;
}

export interface ApiProbeObservation {
  id: string;
  timestamp: string;
  total_duration_ms: number;
  dns_duration_ms: number;
  tcp_duration_ms: number;
  tls_duration_ms: number;
  ttfb_duration_ms: number;
  transfer_duration_ms: number;
  success: boolean;
  http_status_code: number;
  reused_connection: boolean;
  source: string;
  region: string;
}

export interface ApiObservationsResponse {
  dependency_name: string;
  region: string;
  customer_observations: ApiCustomerObservation[];
  probe_observations: ApiProbeObservation[];
}

import { BackendUnavailableError } from './backendStatus';

export const DEFAULT_API_BASE_URL = (import.meta.env.VITE_API_URL as string) || '';

class ApiClient {
  private baseUrl = DEFAULT_API_BASE_URL;

  getBaseUrl(): string {
    return this.baseUrl;
  }

  setBaseUrl(url: string) {
    this.baseUrl = url;
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 4000);

    try {
      const resp = await fetch(`${this.baseUrl}${endpoint}`, {
        ...options,
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
          ...(options.headers || {}),
        },
      });

      if (resp.status === 502 || resp.status === 503 || resp.status === 504) {
        throw new BackendUnavailableError(`APIPulse backend is unavailable (HTTP ${resp.status})`);
      }

      if (!resp.ok) {
        throw new Error(`API error HTTP ${resp.status}: ${resp.statusText}`);
      }

      return (await resp.json()) as T;
    } catch (err: any) {
      if (err instanceof BackendUnavailableError) {
        throw err;
      }
      if (
        err?.name === 'AbortError' ||
        err?.message?.includes('fetch') ||
        err?.message?.includes('network') ||
        err?.message?.includes('Failed to fetch') ||
        err?.message?.includes('NetworkError')
      ) {
        throw new BackendUnavailableError('APIPulse backend is unreachable');
      }
      throw err;
    } finally {
      clearTimeout(timeoutId);
    }
  }

  async checkReadiness(): Promise<BackendStatus> {
    try {
      const data = await this.request<Record<string, string>>('/ready');
      const isOk = data.status === 'ready' || data.status === 'ok';
      return {
        isAvailable: isOk,
        status: data.status || 'unknown',
        postgres: data.postgres || 'unknown',
        redis: data.redis || 'unknown',
        healthEngine: data.health_engine || 'unknown',
        lastChecked: Date.now(),
      };
    } catch {
      return {
        isAvailable: false,
        status: 'unavailable',
        postgres: 'unavailable',
        redis: 'unavailable',
        healthEngine: 'unavailable',
        lastChecked: Date.now(),
      };
    }
  }

  async getDependencies(): Promise<Array<{
    dependency_name: string;
    region: string;
    status: string;
    vendor_status?: string;
    customer_signal?: string;
    synthetic_signal?: string;
    confidence?: string;
    recommended_action?: string;
    timestamp?: string;
  }>> {
    const res = await this.request<{
      count: number;
      dependencies: Array<{
        dependency_name: string;
        region: string;
        status: string;
        vendor_status?: string;
        customer_signal?: string;
        synthetic_signal?: string;
        confidence?: string;
        recommended_action?: string;
        timestamp?: string;
      }>;
    }>('/api/v1/dependencies');
    return res.dependencies || [];
  }

  async getDependencyHealth(name: string, region = 'ap-south-1'): Promise<ApiAssessment> {
    return this.request<ApiAssessment>(`/api/v1/dependencies/${encodeURIComponent(name)}/health?region=${encodeURIComponent(region)}`);
  }

  async getDependencyAssessments(name: string, limit = 50): Promise<ApiAssessment[]> {
    const res = await this.request<{
      count: number;
      assessments: ApiAssessment[];
    }>(`/api/v1/dependencies/${encodeURIComponent(name)}/assessments?limit=${limit}`);
    return res.assessments || [];
  }

  async getDependencyObservations(name: string, region = 'ap-south-1', minutes = 30): Promise<ApiObservationsResponse> {
    return this.request<ApiObservationsResponse>(
      `/api/v1/dependencies/${encodeURIComponent(name)}/observations?region=${encodeURIComponent(region)}&minutes=${minutes}`
    );
  }

  async getIncidents(limit = 50): Promise<ApiIncident[]> {
    const res = await this.request<{
      count: number;
      incidents: ApiIncident[];
    }>(`/api/v1/incidents?limit=${limit}`);
    return res.incidents || [];
  }

  async getIncident(id: string): Promise<ApiIncident> {
    return this.request<ApiIncident>(`/api/v1/incidents/${encodeURIComponent(id)}`);
  }
}

export const apiClient = new ApiClient();
