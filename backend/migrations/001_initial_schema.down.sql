-- Migration: 001_initial_schema.down.sql
-- Reverses 001_initial_schema.up.sql
-- Run this only in development to reset the schema.
-- WARNING: This drops all data.

DROP TABLE IF EXISTS alert_policies CASCADE;
DROP TABLE IF EXISTS incident_events CASCADE;
DROP TABLE IF EXISTS incidents CASCADE;
DROP TABLE IF EXISTS dependency_observations CASCADE;
DROP TABLE IF EXISTS probe_observations CASCADE;
DROP TABLE IF EXISTS probes CASCADE;
DROP TABLE IF EXISTS dependencies CASCADE;
DROP TABLE IF EXISTS regions CASCADE;
DROP TABLE IF EXISTS vendors CASCADE;
