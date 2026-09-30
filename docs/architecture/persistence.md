# APIPulse Persistence & Storage Architecture

## 1. Storage Tiers

APIPulse employs a dual-tier storage strategy:

| Tier | Engine | Role | Retention |
|---|---|---|---|
| **Historical & Audit** | PostgreSQL + TimescaleDB | Authoritative time-series observations, assessment history, incident audit logs | Configurable (90 days raw, 1 yr assessments, indefinite incidents) |
| **Hot Operational** | Redis (or in-process fallback) | Active health status, latest evaluation cache, hysteresis counters, deduplication | Ephemeral (TTL 5m - 24h) |

## 2. TimescaleDB Hypertables & Indexes

- `dependency_observations`: partitioned on `timestamp` (1-day chunk interval). Indexes on `(dependency_name, timestamp DESC)` and `(region_slug, timestamp DESC)`. Trace IDs are stored for correlation but **never indexed** to prevent unbounded cardinality explosions.
- `probe_observations`: partitioned on `timestamp`. Indexes on `(region_slug, timestamp DESC)` and `(source, timestamp DESC)`.
- `vendor_status_observations`: partitioned on `timestamp`. Indexes on `(dependency_name, timestamp DESC)`.
- `health_assessments`: partitioned on `timestamp`. Composite index on `(dependency_name, region_slug, timestamp DESC)`.
- `incidents`: standard PostgreSQL table with partial index on `(deduplication_key) WHERE status != 'RESOLVED'`.

## 3. Incident Lifecycle & Restart Survival

Incidents survive process restarts without duplication:
- Every degradation episode calculates a deterministic `deduplication_key` (e.g. `stripe-like:ap-south-1:GREY_FAILURE`).
- On restart, `GetActiveIncident` queries the database for existing non-resolved episodes matching this key.
- Updates append events to `incident_events` and update the existing incident record without creating duplicates.
