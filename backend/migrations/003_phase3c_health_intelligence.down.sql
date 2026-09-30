-- Migration: 003_phase3c_health_intelligence.down.sql
-- Reverses Phase 3C additions.

ALTER TABLE incident_events DROP COLUMN IF EXISTS assessment_id;
ALTER TABLE incident_events DROP COLUMN IF EXISTS dependency_name;
ALTER TABLE incident_events DROP COLUMN IF EXISTS region_slug;
ALTER TABLE incident_events DROP COLUMN IF EXISTS reason;

ALTER TABLE incidents DROP COLUMN IF EXISTS deduplication_key;
ALTER TABLE incidents DROP COLUMN IF EXISTS incident_type;
ALTER TABLE incidents DROP COLUMN IF EXISTS assessment_id;
ALTER TABLE incidents DROP COLUMN IF EXISTS dependency_name;
ALTER TABLE incidents DROP COLUMN IF EXISTS region_slug;

DROP TABLE IF EXISTS health_assessments CASCADE;
DROP TABLE IF EXISTS vendor_status_observations CASCADE;
