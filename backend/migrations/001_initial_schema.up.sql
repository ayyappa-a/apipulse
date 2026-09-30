-- Migration: 001_initial_schema.up.sql
-- APIPulse initial schema: core entity tables
-- 
-- Design decisions:
--   • All IDs are UUIDs (gen_random_uuid()) — stable, non-sequential, safe to expose in APIs.
--   • All timestamps are TIMESTAMPTZ (UTC enforced at application layer).
--   • TimescaleDB hypertables are used for high-volume telemetry tables (probe_observations,
--     dependency_observations). Do NOT use hypertables for low-volume entity tables.
--   • Composite indexes support the dashboard queries we know we need:
--     dependency+time, region+time, source+time, status+time.
--   • No unbounded high-cardinality label columns (no trace_id indexes, no user_id labels).

-- Enable the TimescaleDB extension.
-- This must be available before any hypertable conversions below.
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- ============================================================
-- VENDORS
-- Third-party SaaS vendors APIPulse monitors (e.g. Stripe, SendGrid).
-- ============================================================
CREATE TABLE IF NOT EXISTS vendors (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,          -- URL-safe identifier e.g. "stripe"
    status_page_url TEXT,                       -- e.g. "https://status.stripe.com"
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vendors_slug ON vendors (slug);

-- ============================================================
-- REGIONS
-- APIPulse monitoring regions (probe origin + customer region labels).
-- ============================================================
CREATE TABLE IF NOT EXISTS regions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT NOT NULL UNIQUE,          -- e.g. "us-east-1"
    display_name TEXT NOT NULL,
    cloud_provider TEXT,                        -- e.g. "aws", "gcp"
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO regions (slug, display_name, cloud_provider) VALUES
    ('us-east-1',      'US East (N. Virginia)',    'aws'),
    ('us-west-2',      'US West (Oregon)',          'aws'),
    ('eu-west-1',      'Europe (Ireland)',          'aws'),
    ('eu-central-1',   'Europe (Frankfurt)',        'aws'),
    ('ap-south-1',     'Asia Pacific (Mumbai)',     'aws'),
    ('ap-southeast-1', 'Asia Pacific (Singapore)', 'aws'),
    ('ap-northeast-1', 'Asia Pacific (Tokyo)',      'aws'),
    ('sa-east-1',      'South America (São Paulo)', 'aws')
ON CONFLICT (slug) DO NOTHING;

-- ============================================================
-- DEPENDENCIES
-- A dependency is a specific API/service endpoint of a vendor.
-- e.g. "Stripe Payments API" under vendor "stripe".
-- ============================================================
CREATE TABLE IF NOT EXISTS dependencies (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vendor_id    UUID NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL,                -- e.g. "stripe-payments-api"
    endpoint_url TEXT,                          -- canonical endpoint being monitored
    environment  TEXT NOT NULL DEFAULT 'production',
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (vendor_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_dependencies_vendor_id ON dependencies (vendor_id);
CREATE INDEX IF NOT EXISTS idx_dependencies_slug ON dependencies (slug);

-- ============================================================
-- PROBES
-- Probe configurations: what to probe, how often, from which regions.
-- ============================================================
CREATE TABLE IF NOT EXISTS probes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dependency_id   UUID NOT NULL REFERENCES dependencies(id) ON DELETE CASCADE,
    region_slug     TEXT NOT NULL,
    probe_url       TEXT NOT NULL,
    method          TEXT NOT NULL DEFAULT 'GET',
    interval_seconds INT NOT NULL DEFAULT 60,
    timeout_ms      INT NOT NULL DEFAULT 10000,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_probes_dependency_id ON probes (dependency_id);
CREATE INDEX IF NOT EXISTS idx_probes_region_slug ON probes (region_slug);
CREATE INDEX IF NOT EXISTS idx_probes_active ON probes (active) WHERE active = TRUE;

-- ============================================================
-- PROBE_OBSERVATIONS
-- High-volume table: one row per synthetic probe execution.
-- Converted to a TimescaleDB hypertable partitioned by timestamp.
--
-- PRIVACY CONTRACT: No request/response bodies. No PII. No customer data.
-- ============================================================
CREATE TABLE IF NOT EXISTS probe_observations (
    id                    UUID NOT NULL DEFAULT gen_random_uuid(),
    probe_id              UUID NOT NULL,                -- references probes(id)
    dependency_id         UUID NOT NULL,                -- denormalized for query performance
    region_slug           TEXT NOT NULL,
    timestamp             TIMESTAMPTZ NOT NULL,
    source                TEXT NOT NULL DEFAULT 'SYNTHETIC_PROBE',
    success               BOOLEAN NOT NULL,
    http_status_code      INT NOT NULL DEFAULT 0,
    dns_duration_ms       DOUBLE PRECISION NOT NULL DEFAULT 0,
    tcp_duration_ms       DOUBLE PRECISION NOT NULL DEFAULT 0,
    tls_duration_ms       DOUBLE PRECISION NOT NULL DEFAULT 0,
    ttfb_duration_ms      DOUBLE PRECISION NOT NULL DEFAULT 0,
    transfer_duration_ms  DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_duration_ms     DOUBLE PRECISION NOT NULL,
    error_type            TEXT NOT NULL DEFAULT '',
    network_error         TEXT,
    timeout               BOOLEAN NOT NULL DEFAULT FALSE,
    response_validation   TEXT,
    ingested_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, timestamp)      -- composite PK required for TimescaleDB
);

-- Convert to TimescaleDB hypertable partitioned by timestamp.
-- chunk_time_interval = 1 day is appropriate for probe volume.
SELECT create_hypertable(
    'probe_observations',
    'timestamp',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Dashboard query indexes:
CREATE INDEX IF NOT EXISTS idx_probe_obs_dep_time    ON probe_observations (dependency_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_probe_obs_region_time ON probe_observations (region_slug, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_probe_obs_source_time ON probe_observations (source, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_probe_obs_success     ON probe_observations (success, timestamp DESC);

-- ============================================================
-- DEPENDENCY_OBSERVATIONS
-- High-volume table: one row per customer OTEL span ingested.
-- Converted to a TimescaleDB hypertable partitioned by timestamp.
--
-- PRIVACY CONTRACT:
--   No request bodies, no response bodies, no auth tokens,
--   no PII, no payment information, no arbitrary query parameters.
-- ============================================================
CREATE TABLE IF NOT EXISTS dependency_observations (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    timestamp       TIMESTAMPTZ NOT NULL,
    service_name    TEXT NOT NULL,
    dependency_name TEXT NOT NULL,
    operation       TEXT NOT NULL,              -- sanitized, cardinality-safe
    region_slug     TEXT NOT NULL,
    source          TEXT NOT NULL DEFAULT 'CUSTOMER_OTEL',
    duration_ms     DOUBLE PRECISION NOT NULL,
    status          TEXT NOT NULL,
    status_code     INT NOT NULL DEFAULT 0,
    error_type      TEXT NOT NULL DEFAULT '',
    trace_id        TEXT,                       -- optional, never indexed
    environment     TEXT NOT NULL DEFAULT 'production',
    ingested_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, timestamp)      -- composite PK required for TimescaleDB
);

-- Convert to TimescaleDB hypertable partitioned by timestamp.
SELECT create_hypertable(
    'dependency_observations',
    'timestamp',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Dashboard query indexes:
CREATE INDEX IF NOT EXISTS idx_dep_obs_dep_time     ON dependency_observations (dependency_name, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_dep_obs_region_time  ON dependency_observations (region_slug, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_dep_obs_source_time  ON dependency_observations (source, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_dep_obs_status_time  ON dependency_observations (status, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_dep_obs_service_time ON dependency_observations (service_name, timestamp DESC);

-- INTENTIONALLY NOT INDEXED: trace_id
-- trace_id is unbounded high-cardinality and must not become a label or index.

-- ============================================================
-- INCIDENTS
-- Detected or manually declared incidents affecting a dependency.
-- ============================================================
CREATE TABLE IF NOT EXISTS incidents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dependency_id   UUID NOT NULL REFERENCES dependencies(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    severity        TEXT NOT NULL,              -- critical | high | medium | low
    status          TEXT NOT NULL DEFAULT 'investigating',
    source          TEXT NOT NULL,              -- which telemetry source triggered this
    started_at      TIMESTAMPTZ NOT NULL,
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_incidents_dependency_id ON incidents (dependency_id);
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents (status);
CREATE INDEX IF NOT EXISTS idx_incidents_started_at ON incidents (started_at DESC);

-- ============================================================
-- INCIDENT_EVENTS
-- Append-only timeline of updates to an incident.
-- ============================================================
CREATE TABLE IF NOT EXISTS incident_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,                  -- status_change | note | resolved | escalated
    message     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_incident_events_incident_id ON incident_events (incident_id, created_at DESC);

-- ============================================================
-- ALERT_POLICIES
-- Per-dependency alerting thresholds.
-- ============================================================
CREATE TABLE IF NOT EXISTS alert_policies (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dependency_id       UUID NOT NULL REFERENCES dependencies(id) ON DELETE CASCADE,
    p95_threshold_ms    DOUBLE PRECISION,       -- alert if p95 latency exceeds this
    p99_threshold_ms    DOUBLE PRECISION,       -- alert if p99 latency exceeds this
    error_rate_pct      DOUBLE PRECISION,       -- alert if error rate exceeds this %
    probe_failure_count INT,                    -- alert after N consecutive probe failures
    active              BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
