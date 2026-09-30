// Package postgres provides PostgreSQL/TimescaleDB persistence for APIPulse.
package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/apipulse/apipulse/internal/config"
	"github.com/apipulse/apipulse/internal/telemetry/model"
)

// PostgresStore implements TelemetryStore using pgxpool for production persistence.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore initializes a connection pool to PostgreSQL/TimescaleDB.
func NewPostgresStore(ctx context.Context, cfg *config.Config) (*PostgresStore, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database URL: %w", err)
	}

	poolConfig.MaxConns = cfg.PostgresMaxConns
	poolConfig.MinConns = cfg.PostgresMinConns
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 15 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres connection pool: %w", err)
	}

	// Verify connectivity
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &PostgresStore{pool: pool}, nil
}

// Ping verifies database connectivity.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close closes all connections in the pool.
func (s *PostgresStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Pool returns the underlying pgxpool.Pool for diagnostics/metrics.
func (s *PostgresStore) Pool() *pgxpool.Pool {
	return s.pool
}

// RunMigrations applies SQL migrations in ascending lexical order, tracking applied versions in schema_migrations.
func (s *PostgresStore) RunMigrations(ctx context.Context, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations directory %s: %w", migrationsDir, err)
	}

	// Ensure schema_migrations table exists for migration metadata tracking
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, filepath.Join(migrationsDir, entry.Name()))
		}
	}
	sort.Strings(upFiles)

	for _, file := range upFiles {
		version := filepath.Base(file)

		// Check if migration version was already applied
		var alreadyApplied bool
		err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&alreadyApplied)
		if err == nil && alreadyApplied {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", file, err)
		}

		// Execute migration statements
		if _, err := s.pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("execute migration %s: %w", version, err)
		}

		// Record applied migration in schema_migrations table
		if _, err := s.pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING`, version); err != nil {
			return fmt.Errorf("record applied migration %s: %w", version, err)
		}
	}

	return nil
}

// ── Writes ────────────────────────────────────────────────────────────────────

func (s *PostgresStore) InsertDependencyObservation(ctx context.Context, obs model.DependencyObservation) error {
	if obs.ID == uuid.Nil {
		obs.ID = uuid.New()
	}
	if obs.Timestamp.IsZero() {
		obs.Timestamp = time.Now().UTC()
	}
	if obs.IngestedAt.IsZero() {
		obs.IngestedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO dependency_observations (
			id, timestamp, service_name, dependency_name, operation,
			region_slug, source, duration_ms, status, status_code,
			error_type, trace_id, environment, ingested_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14
		)
	`
	_, err := s.pool.Exec(ctx, query,
		obs.ID, obs.Timestamp, obs.ServiceName, obs.DependencyName, obs.Operation,
		obs.Region, string(obs.Source), obs.DurationMS, string(obs.Status), obs.StatusCode,
		string(obs.ErrorType), obs.TraceID, obs.Environment, obs.IngestedAt,
	)
	return err
}

func (s *PostgresStore) InsertProbeObservation(ctx context.Context, obs model.ProbeObservation) error {
	if obs.ID == uuid.Nil {
		obs.ID = uuid.New()
	}
	if obs.Timestamp.IsZero() {
		obs.Timestamp = time.Now().UTC()
	}
	if obs.IngestedAt.IsZero() {
		obs.IngestedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO probe_observations (
			id, probe_id, dependency_id, region_slug, timestamp,
			source, success, http_status_code, dns_duration_ms, tcp_duration_ms,
			tls_duration_ms, ttfb_duration_ms, transfer_duration_ms, total_duration_ms,
			error_type, network_error, timeout, response_validation, ingested_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14,
			$15, $16, $17, $18, $19
		)
	`
	_, err := s.pool.Exec(ctx, query,
		obs.ID, obs.ProbeID, obs.DependencyID, obs.Region, obs.Timestamp,
		string(obs.Source), obs.Success, obs.HTTPStatusCode, obs.DNSDurationMS, obs.TCPDurationMS,
		obs.TLSDurationMS, obs.TTFBDurationMS, obs.TransferDurationMS, obs.TotalDurationMS,
		string(obs.ErrorType), obs.NetworkError, obs.Timeout, obs.ResponseValidationResult, obs.IngestedAt,
	)
	return err
}

func (s *PostgresStore) InsertVendorStatus(ctx context.Context, rec model.VendorStatusRecord) error {
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now().UTC()
	}
	if rec.IngestedAt.IsZero() {
		rec.IngestedAt = time.Now().UTC()
	}
	if rec.Region == "" {
		rec.Region = "global"
	}

	query := `
		INSERT INTO vendor_status_observations (
			id, vendor_name, dependency_name, region_slug, timestamp,
			source, status, status_text, incident_reference, ingested_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10
		)
	`
	_, err := s.pool.Exec(ctx, query,
		rec.ID, rec.VendorName, rec.DependencyName, rec.Region, rec.Timestamp,
		string(rec.Source), string(rec.Status), rec.StatusText, rec.IncidentReference, rec.IngestedAt,
	)
	return err
}

func (s *PostgresStore) InsertAssessment(ctx context.Context, a model.HealthAssessment) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.Timestamp.IsZero() {
		a.Timestamp = time.Now().UTC()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}

	reasonCodesStr := a.ReasonCodesStr
	if reasonCodesStr == "" && len(a.ReasonCodes) > 0 {
		var codes []string
		for _, c := range a.ReasonCodes {
			codes = append(codes, string(c))
		}
		reasonCodesStr = strings.Join(codes, ",")
	}

	query := `
		INSERT INTO health_assessments (
			id, dependency_id, dependency_name, region_slug, timestamp,
			source, status, customer_signal, synthetic_signal, vendor_status,
			customer_latency_ms, synthetic_latency_ms, customer_baseline_p95_ms, synthetic_baseline_p95_ms,
			latency_multiplier, confidence, reason_codes, explanation, recommended_action,
			customer_sample_count, synthetic_sample_count, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14,
			$15, $16, $17, $18, $19,
			$20, $21, $22
		)
	`
	_, err := s.pool.Exec(ctx, query,
		a.ID, a.DependencyID, a.DependencyName, a.Region, a.Timestamp,
		string(a.Source), string(a.Status), string(a.CustomerSignal), string(a.SyntheticSignal), string(a.VendorStatus),
		a.CustomerLatencyMS, a.SyntheticLatencyMS, a.CustomerBaselineP95MS, a.SyntheticBaselineP95MS,
		a.LatencyMultiplier, string(a.Confidence), reasonCodesStr, a.Explanation, string(a.RecommendedAction),
		a.CustomerSampleCount, a.SyntheticSampleCount, a.CreatedAt,
	)
	return err
}

func (s *PostgresStore) UpsertIncident(ctx context.Context, inc model.Incident) error {
	if inc.ID == uuid.Nil {
		inc.ID = uuid.New()
	}
	if inc.StartedAt.IsZero() {
		inc.StartedAt = time.Now().UTC()
	}
	if inc.CreatedAt.IsZero() {
		inc.CreatedAt = time.Now().UTC()
	}
	inc.UpdatedAt = time.Now().UTC()

	// Check if an incident already exists for this deduplication key
	if inc.DeduplicationKey != "" {
		existing, err := s.GetActiveIncident(ctx, inc.DeduplicationKey)
		if err == nil && existing != nil {
			// Update existing incident
			updateQuery := `
				UPDATE incidents
				SET status = $1, resolved_at = $2, updated_at = $3, assessment_id = $4, severity = $5
				WHERE id = $6
			`
			_, err = s.pool.Exec(ctx, updateQuery,
				string(inc.Status), inc.ResolvedAt, inc.UpdatedAt, inc.AssessmentID, inc.Severity,
				existing.ID,
			)
			return err
		}
	}

	query := `
		INSERT INTO incidents (
			id, title, severity, status, source,
			started_at, resolved_at, created_at, updated_at,
			deduplication_key, incident_type, assessment_id, dependency_name, region_slug
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13, $14
		)
	`
	_, err := s.pool.Exec(ctx, query,
		inc.ID, inc.Title, inc.Severity, string(inc.Status), "APIPULSE_ASSESSMENT",
		inc.StartedAt, inc.ResolvedAt, inc.CreatedAt, inc.UpdatedAt,
		inc.DeduplicationKey, string(inc.IncidentType), inc.AssessmentID, inc.DependencyName, inc.Region,
	)
	return err
}

func (s *PostgresStore) InsertIncidentEvent(ctx context.Context, evt model.IncidentEvent) error {
	if evt.ID == uuid.Nil {
		evt.ID = uuid.New()
	}
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO incident_events (
			id, incident_id, event_type, message, reason,
			assessment_id, dependency_name, region_slug, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9
		)
	`
	_, err := s.pool.Exec(ctx, query,
		evt.ID, evt.IncidentID, string(evt.EventType), evt.Reason, evt.Reason,
		evt.AssessmentID, evt.DependencyName, evt.Region, evt.CreatedAt,
	)
	return err
}

// ── Reads ─────────────────────────────────────────────────────────────────────

func (s *PostgresStore) GetRecentDependencyObservations(
	ctx context.Context,
	dependencyName, region string,
	since time.Time,
) ([]model.DependencyObservation, error) {
	query := `
		SELECT id, timestamp, service_name, dependency_name, operation,
		       region_slug, source, duration_ms, status, status_code,
		       error_type, COALESCE(trace_id, ''), environment, ingested_at
		FROM dependency_observations
		WHERE dependency_name = $1 AND region_slug = $2 AND timestamp >= $3
		ORDER BY timestamp ASC
	`
	rows, err := s.pool.Query(ctx, query, dependencyName, region, since)
	if err != nil {
		return nil, fmt.Errorf("query dependency observations: %w", err)
	}
	defer rows.Close()

	var result []model.DependencyObservation
	for rows.Next() {
		var obs model.DependencyObservation
		var sourceStr, statusStr, errorTypeStr string
		err := rows.Scan(
			&obs.ID, &obs.Timestamp, &obs.ServiceName, &obs.DependencyName, &obs.Operation,
			&obs.Region, &sourceStr, &obs.DurationMS, &statusStr, &obs.StatusCode,
			&errorTypeStr, &obs.TraceID, &obs.Environment, &obs.IngestedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan dependency observation: %w", err)
		}
		obs.Source = model.TelemetrySource(sourceStr)
		obs.Status = model.ObservationStatus(statusStr)
		obs.ErrorType = model.ErrorType(errorTypeStr)
		result = append(result, obs)
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetRecentProbeObservations(
	ctx context.Context,
	dependencyName, region string,
	since time.Time,
) ([]model.ProbeObservation, error) {
	query := `
		SELECT id, probe_id, dependency_id, region_slug, timestamp,
		       source, success, http_status_code, dns_duration_ms, tcp_duration_ms,
		       tls_duration_ms, ttfb_duration_ms, transfer_duration_ms, total_duration_ms,
		       error_type, COALESCE(network_error, ''), timeout, COALESCE(response_validation, ''), ingested_at
		FROM probe_observations
		WHERE region_slug = $1 AND timestamp >= $2
		ORDER BY timestamp ASC
	`
	rows, err := s.pool.Query(ctx, query, region, since)
	if err != nil {
		return nil, fmt.Errorf("query probe observations: %w", err)
	}
	defer rows.Close()

	var result []model.ProbeObservation
	for rows.Next() {
		var obs model.ProbeObservation
		var sourceStr, errorTypeStr string
		err := rows.Scan(
			&obs.ID, &obs.ProbeID, &obs.DependencyID, &obs.Region, &obs.Timestamp,
			&sourceStr, &obs.Success, &obs.HTTPStatusCode, &obs.DNSDurationMS, &obs.TCPDurationMS,
			&obs.TLSDurationMS, &obs.TTFBDurationMS, &obs.TransferDurationMS, &obs.TotalDurationMS,
			&errorTypeStr, &obs.NetworkError, &obs.Timeout, &obs.ResponseValidationResult, &obs.IngestedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan probe observation: %w", err)
		}
		obs.Source = model.TelemetrySource(sourceStr)
		obs.ErrorType = model.ErrorType(errorTypeStr)
		result = append(result, obs)
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetLatestVendorStatus(
	ctx context.Context,
	vendorName, dependencyName string,
) (*model.VendorStatusRecord, error) {
	query := `
		SELECT id, vendor_name, dependency_name, region_slug, timestamp,
		       source, status, COALESCE(status_text, ''), COALESCE(incident_reference, ''), ingested_at
		FROM vendor_status_observations
		WHERE dependency_name = $1
		ORDER BY timestamp DESC
		LIMIT 1
	`
	var rec model.VendorStatusRecord
	var sourceStr, statusStr string
	err := s.pool.QueryRow(ctx, query, dependencyName).Scan(
		&rec.ID, &rec.VendorName, &rec.DependencyName, &rec.Region, &rec.Timestamp,
		&sourceStr, &statusStr, &rec.StatusText, &rec.IncidentReference, &rec.IngestedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query latest vendor status: %w", err)
	}
	rec.Source = model.TelemetrySource(sourceStr)
	rec.Status = model.VendorStatus(statusStr)
	return &rec, nil
}

func (s *PostgresStore) GetLatestAssessment(
	ctx context.Context,
	dependencyName, region string,
) (*model.HealthAssessment, error) {
	query := `
		SELECT id, dependency_id, dependency_name, region_slug, timestamp,
		       source, status, customer_signal, synthetic_signal, vendor_status,
		       customer_latency_ms, synthetic_latency_ms, customer_baseline_p95_ms, synthetic_baseline_p95_ms,
		       latency_multiplier, confidence, reason_codes, explanation, recommended_action,
		       customer_sample_count, synthetic_sample_count, created_at
		FROM health_assessments
		WHERE dependency_name = $1 AND region_slug = $2
		ORDER BY timestamp DESC
		LIMIT 1
	`
	var a model.HealthAssessment
	var sourceStr, statusStr, custSig, synthSig, vendStat, confStr, recAction string
	err := s.pool.QueryRow(ctx, query, dependencyName, region).Scan(
		&a.ID, &a.DependencyID, &a.DependencyName, &a.Region, &a.Timestamp,
		&sourceStr, &statusStr, &custSig, &synthSig, &vendStat,
		&a.CustomerLatencyMS, &a.SyntheticLatencyMS, &a.CustomerBaselineP95MS, &a.SyntheticBaselineP95MS,
		&a.LatencyMultiplier, &confStr, &a.ReasonCodesStr, &a.Explanation, &recAction,
		&a.CustomerSampleCount, &a.SyntheticSampleCount, &a.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query latest assessment: %w", err)
	}

	a.Source = model.TelemetrySource(sourceStr)
	a.Status = model.AssessmentStatus(statusStr)
	a.CustomerSignal = model.SignalState(custSig)
	a.SyntheticSignal = model.SignalState(synthSig)
	a.VendorStatus = model.VendorStatus(vendStat)
	a.Confidence = model.ConfidenceLevel(confStr)
	a.RecommendedAction = model.RecommendedAction(recAction)
	if a.ReasonCodesStr != "" {
		for _, part := range strings.Split(a.ReasonCodesStr, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				a.ReasonCodes = append(a.ReasonCodes, model.ReasonCode(trimmed))
			}
		}
	}
	return &a, nil
}

func (s *PostgresStore) ListAssessments(
	ctx context.Context,
	dependencyName string,
	limit int,
) ([]model.HealthAssessment, error) {
	if limit <= 0 {
		limit = 50
	}

	var rows pgx.Rows
	var err error
	if dependencyName != "" {
		query := `
			SELECT id, dependency_id, dependency_name, region_slug, timestamp,
			       source, status, customer_signal, synthetic_signal, vendor_status,
			       customer_latency_ms, synthetic_latency_ms, customer_baseline_p95_ms, synthetic_baseline_p95_ms,
			       latency_multiplier, confidence, reason_codes, explanation, recommended_action,
			       customer_sample_count, synthetic_sample_count, created_at
			FROM health_assessments
			WHERE dependency_name = $1
			ORDER BY timestamp DESC
			LIMIT $2
		`
		rows, err = s.pool.Query(ctx, query, dependencyName, limit)
	} else {
		query := `
			SELECT id, dependency_id, dependency_name, region_slug, timestamp,
			       source, status, customer_signal, synthetic_signal, vendor_status,
			       customer_latency_ms, synthetic_latency_ms, customer_baseline_p95_ms, synthetic_baseline_p95_ms,
			       latency_multiplier, confidence, reason_codes, explanation, recommended_action,
			       customer_sample_count, synthetic_sample_count, created_at
			FROM health_assessments
			ORDER BY timestamp DESC
			LIMIT $1
		`
		rows, err = s.pool.Query(ctx, query, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list assessments: %w", err)
	}
	defer rows.Close()

	var result []model.HealthAssessment
	for rows.Next() {
		var a model.HealthAssessment
		var sourceStr, statusStr, custSig, synthSig, vendStat, confStr, recAction string
		err := rows.Scan(
			&a.ID, &a.DependencyID, &a.DependencyName, &a.Region, &a.Timestamp,
			&sourceStr, &statusStr, &custSig, &synthSig, &vendStat,
			&a.CustomerLatencyMS, &a.SyntheticLatencyMS, &a.CustomerBaselineP95MS, &a.SyntheticBaselineP95MS,
			&a.LatencyMultiplier, &confStr, &a.ReasonCodesStr, &a.Explanation, &recAction,
			&a.CustomerSampleCount, &a.SyntheticSampleCount, &a.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan assessment: %w", err)
		}
		a.Source = model.TelemetrySource(sourceStr)
		a.Status = model.AssessmentStatus(statusStr)
		a.CustomerSignal = model.SignalState(custSig)
		a.SyntheticSignal = model.SignalState(synthSig)
		a.VendorStatus = model.VendorStatus(vendStat)
		a.Confidence = model.ConfidenceLevel(confStr)
		a.RecommendedAction = model.RecommendedAction(recAction)
		if a.ReasonCodesStr != "" {
			for _, part := range strings.Split(a.ReasonCodesStr, ",") {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					a.ReasonCodes = append(a.ReasonCodes, model.ReasonCode(trimmed))
				}
			}
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetActiveIncident(
	ctx context.Context,
	deduplicationKey string,
) (*model.Incident, error) {
	query := `
		SELECT id, COALESCE(dependency_name, ''), COALESCE(region_slug, ''), deduplication_key,
		       COALESCE(incident_type, 'DEGRADED'), status, title, severity,
		       COALESCE(assessment_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       started_at, resolved_at, created_at, updated_at
		FROM incidents
		WHERE deduplication_key = $1 AND status != 'RESOLVED'
		LIMIT 1
	`
	var inc model.Incident
	var statusStr, incTypeStr string
	err := s.pool.QueryRow(ctx, query, deduplicationKey).Scan(
		&inc.ID, &inc.DependencyName, &inc.Region, &inc.DeduplicationKey,
		&incTypeStr, &statusStr, &inc.Title, &inc.Severity,
		&inc.AssessmentID,
		&inc.StartedAt, &inc.ResolvedAt, &inc.CreatedAt, &inc.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query active incident: %w", err)
	}
	inc.Status = model.IncidentStatus(statusStr)
	inc.IncidentType = model.AssessmentStatus(incTypeStr)
	return &inc, nil
}

func (s *PostgresStore) ListIncidents(
	ctx context.Context,
	limit int,
) ([]model.Incident, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, COALESCE(dependency_name, ''), COALESCE(region_slug, ''), COALESCE(deduplication_key, ''),
		       COALESCE(incident_type, 'DEGRADED'), status, title, severity,
		       COALESCE(assessment_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       started_at, resolved_at, created_at, updated_at
		FROM incidents
		ORDER BY started_at DESC
		LIMIT $1
	`
	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var result []model.Incident
	for rows.Next() {
		var inc model.Incident
		var statusStr, incTypeStr string
		err := rows.Scan(
			&inc.ID, &inc.DependencyName, &inc.Region, &inc.DeduplicationKey,
			&incTypeStr, &statusStr, &inc.Title, &inc.Severity,
			&inc.AssessmentID,
			&inc.StartedAt, &inc.ResolvedAt, &inc.CreatedAt, &inc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		inc.Status = model.IncidentStatus(statusStr)
		inc.IncidentType = model.AssessmentStatus(incTypeStr)
		result = append(result, inc)
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetIncident(
	ctx context.Context,
	id string,
) (*model.Incident, error) {
	parsedUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid incident id %s: %w", id, err)
	}

	query := `
		SELECT id, COALESCE(dependency_name, ''), COALESCE(region_slug, ''), COALESCE(deduplication_key, ''),
		       COALESCE(incident_type, 'DEGRADED'), status, title, severity,
		       COALESCE(assessment_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       started_at, resolved_at, created_at, updated_at
		FROM incidents
		WHERE id = $1
	`
	var inc model.Incident
	var statusStr, incTypeStr string
	err = s.pool.QueryRow(ctx, query, parsedUUID).Scan(
		&inc.ID, &inc.DependencyName, &inc.Region, &inc.DeduplicationKey,
		&incTypeStr, &statusStr, &inc.Title, &inc.Severity,
		&inc.AssessmentID,
		&inc.StartedAt, &inc.ResolvedAt, &inc.CreatedAt, &inc.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("incident %s not found", id)
		}
		return nil, fmt.Errorf("query incident %s: %w", id, err)
	}
	inc.Status = model.IncidentStatus(statusStr)
	inc.IncidentType = model.AssessmentStatus(incTypeStr)

	// Fetch timeline events
	eventQuery := `
		SELECT id, incident_id, event_type, COALESCE(reason, message, ''),
		       COALESCE(assessment_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       COALESCE(dependency_name, ''), COALESCE(region_slug, ''), created_at
		FROM incident_events
		WHERE incident_id = $1
		ORDER BY created_at ASC
	`
	eventRows, err := s.pool.Query(ctx, eventQuery, inc.ID)
	if err == nil {
		defer eventRows.Close()
		for eventRows.Next() {
			var evt model.IncidentEvent
			var evtTypeStr string
			if scanErr := eventRows.Scan(
				&evt.ID, &evt.IncidentID, &evtTypeStr, &evt.Reason,
				&evt.AssessmentID, &evt.DependencyName, &evt.Region, &evt.CreatedAt,
			); scanErr == nil {
				evt.EventType = model.IncidentEventType(evtTypeStr)
				inc.Events = append(inc.Events, evt)
			}
		}
	}

	return &inc, nil
}

// GetActiveTargets returns distinct (dependency_name, region) tuples discovered
// from ingested observations or assessments.
func (s *PostgresStore) GetActiveTargets(ctx context.Context) ([]Target, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT dependency_name, region_slug FROM (
			SELECT dependency_name, region_slug FROM dependency_observations
			UNION
			SELECT dependency_name, region_slug FROM health_assessments
		) combined
		WHERE dependency_name != '' AND region_slug != ''
		ORDER BY dependency_name, region_slug
	`)
	if err != nil {
		return nil, fmt.Errorf("get active targets: %w", err)
	}
	defer rows.Close()

	var targets []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.DependencyName, &t.Region); err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}
