-- Migration: 003_phase3c_health_intelligence.up.sql
-- APIPulse Phase 3C: Health Intelligence Engine persistence layer
--
-- New tables:
--   vendor_status_observations   — VENDOR_STATUS time-series
--   health_assessments           — APIPULSE_ASSESSMENT time-series
--   incidents                    — Incident lifecycle (extends Phase 2 incidents table)
--   incident_events              — Append-only incident timeline
--
-- The incidents and incident_events tables already exist from migration 001.
-- This migration adds the vendor_status_observations and health_assessments tables,
-- and adds missing columns to the existing incidents table for Phase 3C.
--
-- Retention policy (documented, not silently applied):
--   vendor_status_observations: retain 90 days (high-volume time-series)
--   health_assessments:         retain 1 year (medium-volume, valuable for audit)
--   incidents:                  indefinite (low-volume, audit/history)
--
-- To apply retention, operators should configure TimescaleDB data_node policies:
--   SELECT add_retention_policy('vendor_status_observations', INTERVAL '90 days');
--   SELECT add_retention_policy('health_assessments', INTERVAL '365 days');
-- These commands are NOT included here to keep migration idempotent.

-- ============================================================
-- VENDOR_STATUS_OBSERVATIONS
-- Time-series of vendor-declared status for each dependency.
-- One row per polling interval per vendor/dependency.
--
-- PRIVACY CONTRACT: Vendor status pages are public data.
-- No customer data is involved.
-- ============================================================
CREATE TABLE IF NOT EXISTS vendor_status_observations (
    id                  UUID NOT NULL DEFAULT gen_random_uuid(),
    vendor_name         TEXT NOT NULL,
    dependency_name     TEXT NOT NULL,
    region_slug         TEXT NOT NULL DEFAULT 'global',
    timestamp           TIMESTAMPTZ NOT NULL,
    source              TEXT NOT NULL DEFAULT 'VENDOR_STATUS',
    status              TEXT NOT NULL,          -- OPERATIONAL | DEGRADED | PARTIAL_OUTAGE | OUTAGE | UNKNOWN
    status_text         TEXT,
    incident_reference  TEXT,
    ingested_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, timestamp)                  -- composite PK required for TimescaleDB
);

-- Convert to TimescaleDB hypertable partitioned by timestamp.
SELECT create_hypertable(
    'vendor_status_observations',
    'timestamp',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Dashboard query indexes:
CREATE INDEX IF NOT EXISTS idx_vso_dep_time    ON vendor_status_observations (dependency_name, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_vso_vendor_time ON vendor_status_observations (vendor_name, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_vso_status_time ON vendor_status_observations (status, timestamp DESC);

-- ============================================================
-- HEALTH_ASSESSMENTS
-- APIPULSE_ASSESSMENT: computed health states from the Health Intelligence Engine.
-- One row per evaluation cycle per dependency+region.
--
-- Contains enough information to explain WHY the assessment was made.
-- Source provenance (CUSTOMER_OTEL, SYNTHETIC_PROBE, VENDOR_STATUS signals)
-- is always preserved and never conflated.
-- ============================================================
CREATE TABLE IF NOT EXISTS health_assessments (
    id                      UUID NOT NULL DEFAULT gen_random_uuid(),
    dependency_id           TEXT NOT NULL,              -- normalized dependency name or UUID ref
    dependency_name         TEXT NOT NULL,
    region_slug             TEXT NOT NULL,
    timestamp               TIMESTAMPTZ NOT NULL,
    source                  TEXT NOT NULL DEFAULT 'APIPULSE_ASSESSMENT',

    -- Overall health assessment
    status                  TEXT NOT NULL,              -- HEALTHY|DEGRADED|GREY_FAILURE|OUTAGE|RECOVERING|UNKNOWN|INSUFFICIENT_DATA

    -- Per-signal evaluations
    customer_signal         TEXT NOT NULL DEFAULT 'UNKNOWN',
    synthetic_signal        TEXT NOT NULL DEFAULT 'UNKNOWN',
    vendor_status           TEXT NOT NULL DEFAULT 'UNKNOWN',

    -- Observed latency values (evaluation window P95)
    customer_latency_ms     DOUBLE PRECISION NOT NULL DEFAULT 0,
    synthetic_latency_ms    DOUBLE PRECISION NOT NULL DEFAULT 0,

    -- Baseline latency values (rolling window P95)
    customer_baseline_p95_ms  DOUBLE PRECISION NOT NULL DEFAULT 0,
    synthetic_baseline_p95_ms DOUBLE PRECISION NOT NULL DEFAULT 0,

    -- Max ratio of observed to baseline latency (key signal for grey failure)
    latency_multiplier      DOUBLE PRECISION NOT NULL DEFAULT 0,

    -- Categorical confidence — LOW | MEDIUM | HIGH
    confidence              TEXT NOT NULL DEFAULT 'LOW',

    -- Structured reason codes (comma-separated enumeration, not free-form)
    reason_codes            TEXT NOT NULL DEFAULT '',

    -- Human-readable explanation derived from reason codes
    explanation             TEXT NOT NULL DEFAULT '',

    -- Recommended action (APIPulse recommends; customer implements)
    recommended_action      TEXT NOT NULL DEFAULT 'NONE',

    -- Sample counts for this evaluation window
    customer_sample_count   INT NOT NULL DEFAULT 0,
    synthetic_sample_count  INT NOT NULL DEFAULT 0,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (id, timestamp)      -- composite PK required for TimescaleDB
);

-- Convert to TimescaleDB hypertable partitioned by timestamp.
SELECT create_hypertable(
    'health_assessments',
    'timestamp',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Dashboard query indexes:
CREATE INDEX IF NOT EXISTS idx_ha_dep_time    ON health_assessments (dependency_name, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_ha_region_time ON health_assessments (region_slug, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_ha_status_time ON health_assessments (status, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_ha_dep_region  ON health_assessments (dependency_name, region_slug, timestamp DESC);

-- INTENTIONALLY NOT INDEXED: trace_id, customer IDs, request IDs.

COMMENT ON TABLE health_assessments IS
    'APIPULSE_ASSESSMENT records from the Health Intelligence Engine. '
    'Contains provenance for all three signal sources (CUSTOMER_OTEL, SYNTHETIC_PROBE, VENDOR_STATUS). '
    'Source: APIPULSE_ASSESSMENT. Never conflate with raw CUSTOMER_OTEL or SYNTHETIC_PROBE observations.';

-- ============================================================
-- INCIDENTS — Phase 3C additions
-- The incidents table was created in migration 001.
-- Add missing columns for Phase 3C deduplication and enhanced lifecycle.
-- ============================================================
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS deduplication_key TEXT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS incident_type TEXT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS assessment_id UUID;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS dependency_name TEXT;
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS region_slug TEXT;
ALTER TABLE incidents ALTER COLUMN dependency_id DROP NOT NULL;
ALTER TABLE incidents DROP CONSTRAINT IF EXISTS incidents_dependency_id_fkey;

CREATE INDEX IF NOT EXISTS idx_incidents_deduplicate ON incidents (deduplication_key)
    WHERE status != 'RESOLVED';
CREATE INDEX IF NOT EXISTS idx_incidents_dep_region ON incidents (dependency_name, region_slug);

-- ============================================================
-- INCIDENT_EVENTS — Phase 3C additions
-- The incident_events table was created in migration 001.
-- Add missing Phase 3C columns.
-- ============================================================
ALTER TABLE incident_events ADD COLUMN IF NOT EXISTS assessment_id UUID;
ALTER TABLE incident_events ADD COLUMN IF NOT EXISTS dependency_name TEXT;
ALTER TABLE incident_events ADD COLUMN IF NOT EXISTS region_slug TEXT;
ALTER TABLE incident_events ADD COLUMN IF NOT EXISTS reason TEXT;
ALTER TABLE incident_events ALTER COLUMN message DROP NOT NULL;

COMMENT ON TABLE incidents IS
    'Incident lifecycle records for health degradation episodes. '
    'Deduplicated per dependency+region+type — only one active incident per degradation episode. '
    'APIPulse recommends actions but does NOT enforce circuit breaking in customer request paths.';
