-- Migration: 002_add_circuit_state.up.sql
-- Adds the circuit_state table for dependency circuit breaker state tracking.
-- This is separate from incidents — circuit state is a transient operational flag,
-- not an incident lifecycle record.

CREATE TABLE IF NOT EXISTS circuit_states (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dependency_id   UUID NOT NULL REFERENCES dependencies(id) ON DELETE CASCADE,
    state           TEXT NOT NULL DEFAULT 'closed',  -- closed | open | half_open
    triggered_by    TEXT NOT NULL,                    -- SYNTHETIC_PROBE | CUSTOMER_OTEL | APIPULSE_ASSESSMENT
    reason          TEXT,
    opened_at       TIMESTAMPTZ,
    closed_at       TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (dependency_id)                            -- One active circuit state per dependency
);

CREATE INDEX IF NOT EXISTS idx_circuit_states_state ON circuit_states (state);
CREATE INDEX IF NOT EXISTS idx_circuit_states_dep    ON circuit_states (dependency_id);

COMMENT ON TABLE circuit_states IS
    'Operational circuit breaker state per dependency. '
    'APIPulse recommends circuit state but does NOT enforce it in the customer production path. '
    'Customers implement circuit breaking in their own application code.';
