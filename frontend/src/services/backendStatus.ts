/**
 * BackendUnavailableError — thrown when the APIPulse backend is unreachable.
 *
 * By throwing a typed error instead of silently returning mock data,
 * all pages can display an explicit "APIPulse Backend Unavailable" state
 * with a Retry button instead of misleading stale mock data.
 */
export class BackendUnavailableError extends Error {
  public readonly lastKnownAt?: number;

  constructor(message = 'APIPulse backend is unavailable', lastKnownAt?: number) {
    super(message);
    this.name = 'BackendUnavailableError';
    this.lastKnownAt = lastKnownAt;
  }
}

/** Shared hook helper return type for backend-aware state */
export interface BackendAwareState<T> {
  data: T | null;
  isOffline: boolean;
  lastSuccessAt: number | null;
}
