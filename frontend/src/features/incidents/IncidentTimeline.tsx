import React from 'react';
import type { IncidentEvent } from '../../types/domain';

interface IncidentTimelineProps {
  events: IncidentEvent[];
}

export function IncidentTimeline({ events }: IncidentTimelineProps) {
  return (
    <div className="incident-timeline-section">
      <h3 className="section-title">Incident Event Timeline</h3>
      <div className="timeline-container">
        {events.map((evt, idx) => {
          const isLast = idx === events.length - 1;
          return (
            <div key={evt.id} className={`timeline-row ${isLast ? 'timeline-row--last' : ''}`}>
              <div className="timeline-left">
                <div className="timeline-node">
                  <div className="timeline-dot" />
                </div>
                {!isLast && <div className="timeline-connector" />}
              </div>
              <div className="timeline-body">
                <div className="timeline-meta">
                  <span className="timeline-time">
                    {new Date(evt.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                  </span>
                  <span className="timeline-type-badge">{evt.type.replace(/_/g, ' ')}</span>
                </div>
                <p className="timeline-msg">{evt.message}</p>
                {evt.metadata && (
                  <div className="timeline-meta-box">
                    {Object.entries(evt.metadata).map(([k, v]) => (
                      <span key={k} className="meta-tag">
                        {k}: <code>{String(v)}</code>
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
