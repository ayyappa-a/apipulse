import React from 'react';
import { formatLatency } from '../../utils/status';
import type { ProbeExecutionResult } from '../../types/domain';

interface ProbeWaterfallProps {
  result: ProbeExecutionResult;
}

export function ProbeWaterfall({ result }: ProbeWaterfallProps) {
  return (
    <div className="timing-breakdown-waterfall">
      <div className="flex items-center justify-between mb-2">
        <h4 className="timing-waterfall-title">Telemetry Protocol Waterfall</h4>
        <span className="badge badge--neutral text-xs">SOURCE: {result.source}</span>
      </div>

      <div className="waterfall-rows">
        <div className="waterfall-item">
          <div className="waterfall-label-group">
            <span className="waterfall-name">1. DNS Lookup</span>
            <span className="waterfall-time">{formatLatency(result.dns)}</span>
          </div>
          <div className="waterfall-track">
            <div
              className="waterfall-fill waterfall-fill--dns"
              style={{ width: `${Math.max(5, (result.dns / result.total) * 100)}%` }}
            />
          </div>
        </div>

        <div className="waterfall-item">
          <div className="waterfall-label-group">
            <span className="waterfall-name">2. TCP Handshake (SYN/ACK)</span>
            <span className="waterfall-time">{formatLatency(result.tcp)}</span>
          </div>
          <div className="waterfall-track">
            <div
              className="waterfall-fill waterfall-fill--tcp"
              style={{ width: `${Math.max(5, (result.tcp / result.total) * 100)}%` }}
            />
          </div>
        </div>

        <div className="waterfall-item">
          <div className="waterfall-label-group">
            <span className="waterfall-name">3. TLS 1.3 Negotiation</span>
            <span className="waterfall-time">{formatLatency(result.tls)}</span>
          </div>
          <div className="waterfall-track">
            <div
              className="waterfall-fill waterfall-fill--tls"
              style={{ width: `${Math.max(5, (result.tls / result.total) * 100)}%` }}
            />
          </div>
        </div>

        <div className="waterfall-item">
          <div className="waterfall-label-group">
            <span className="waterfall-name">4. Server TTFB (First Byte)</span>
            <span className="waterfall-time">{formatLatency(result.ttfb)}</span>
          </div>
          <div className="waterfall-track">
            <div
              className="waterfall-fill waterfall-fill--ttfb"
              style={{ width: `${Math.max(10, (result.ttfb / result.total) * 100)}%` }}
            />
          </div>
        </div>

        <div className="waterfall-item">
          <div className="waterfall-label-group">
            <span className="waterfall-name">5. Payload Transfer</span>
            <span className="waterfall-time">{formatLatency(result.transfer)}</span>
          </div>
          <div className="waterfall-track">
            <div
              className="waterfall-fill waterfall-fill--transfer"
              style={{ width: `${Math.max(5, (result.transfer / result.total) * 100)}%` }}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
