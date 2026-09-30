/**
 * BackendOfflineBanner
 *
 * Renders a prominent, unmissable "APIPulse Backend Unavailable" state
 * instead of silently showing mock/stale data.
 *
 * Props:
 *   onRetry       — called when the user clicks "Retry"
 *   lastSuccessAt — timestamp of last successful data load (optional)
 *   inline        — if true, renders compact inline banner (default: full page)
 */
import React from 'react';
import { WifiOff, RefreshCw, Clock } from 'lucide-react';

interface Props {
  onRetry: () => void;
  lastSuccessAt?: number | null;
  inline?: boolean;
}

function formatAgo(ts: number): string {
  const secs = Math.floor((Date.now() - ts) / 1000);
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  return `${hrs}h ago`;
}

export function BackendOfflineBanner({ onRetry, lastSuccessAt, inline = false }: Props) {
  if (inline) {
    return (
      <div className="backend-offline-banner backend-offline-banner--inline" role="alert">
        <WifiOff size={16} />
        <span>
          <strong>APIPulse Backend Unavailable</strong>
          {lastSuccessAt && (
            <span className="backend-offline-banner__last-known">
              {' '}— Last known data: <Clock size={12} /> {formatAgo(lastSuccessAt)}
            </span>
          )}
        </span>
        <button
          id="backend-retry-btn-inline"
          className="backend-offline-banner__retry"
          onClick={onRetry}
        >
          <RefreshCw size={14} />
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="backend-offline-page" role="alert">
      <div className="backend-offline-page__card">
        <div className="backend-offline-page__icon">
          <WifiOff size={48} />
        </div>
        <h2 className="backend-offline-page__title">APIPulse Backend Unavailable</h2>
        <p className="backend-offline-page__body">
          The APIPulse API server is not reachable. This page does not display
          mock data — it shows only live, verified telemetry.
        </p>
        {lastSuccessAt && (
          <p className="backend-offline-page__last-known">
            <Clock size={14} />
            Last known data: <strong>{formatAgo(lastSuccessAt)}</strong>
          </p>
        )}
        <button
          id="backend-retry-btn-full"
          className="backend-offline-page__retry"
          onClick={onRetry}
        >
          <RefreshCw size={18} />
          Retry Connection
        </button>
        <p className="backend-offline-page__hint">
          Start the APIPulse backend server:<br />
          <code>cd backend &amp;&amp; go run ./cmd/api</code>
        </p>
      </div>
    </div>
  );
}
