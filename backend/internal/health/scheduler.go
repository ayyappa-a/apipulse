// Package health provides the background health evaluation scheduler for APIPulse.
package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apipulse/apipulse/pkg/logger"
	"github.com/apipulse/apipulse/pkg/metrics"
)

// Scheduler coordinates continuous background health evaluations for active dependencies.
// It enforces bounded concurrency, prevents overlapping evaluations for the same target,
// respects context cancellation, and exposes Prometheus metrics.
type Scheduler struct {
	engine   *Engine
	targets  []EvaluationTarget
	interval time.Duration
	metrics  *metrics.Metrics
	logger   *logger.Logger

	mu            sync.Mutex
	activeTargets map[string]bool
	stopCh        chan struct{}
	wg            sync.WaitGroup
	running       bool
}

// NewScheduler creates a background health evaluation scheduler.
func NewScheduler(
	engine *Engine,
	targets []EvaluationTarget,
	interval time.Duration,
	m *metrics.Metrics,
	log *logger.Logger,
) *Scheduler {
	if interval < 1*time.Second {
		interval = 5 * time.Second
	}

	// Default targets for the demonstration environment if none provided
	if len(targets) == 0 {
		targets = []EvaluationTarget{
			{
				DependencyName: "stripe-like",
				DependencyID:   "stripe-payments",
				VendorName:     "stripe",
				Region:         "ap-south-1",
			},
			{
				DependencyName: "stripe-like",
				DependencyID:   "stripe-payments",
				VendorName:     "stripe",
				Region:         "us-east-1",
			},
		}
	}

	return &Scheduler{
		engine:        engine,
		targets:       targets,
		interval:      interval,
		metrics:       m,
		logger:        log,
		activeTargets: make(map[string]bool),
		stopCh:        make(chan struct{}),
	}
}

// Start begins the continuous evaluation loop in a background goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	if s.metrics != nil {
		s.metrics.SchedulerActive.Set(1)
	}
	if s.logger != nil {
		s.logger.Info(fmt.Sprintf("health evaluation scheduler started: interval=%v targets=%d", s.interval, len(s.targets)))
	}

	s.wg.Add(1)
	go s.runLoop(ctx)
}

// Stop signals the scheduler to stop and waits for all active evaluations to complete.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	if s.metrics != nil {
		s.metrics.SchedulerActive.Set(0)
	}

	s.wg.Wait()
	if s.logger != nil {
		s.logger.Info("health evaluation scheduler stopped cleanly")
	}
}

// IsRunning returns whether the scheduler is currently active.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// AddTarget dynamically registers a new target for evaluation.
func (s *Scheduler) AddTarget(target EvaluationTarget) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.targets {
		if t.DependencyName == target.DependencyName && t.Region == target.Region {
			return
		}
	}
	s.targets = append(s.targets, target)
}

// runLoop executes health evaluations on every tick.
func (s *Scheduler) runLoop(ctx context.Context) {
	defer s.wg.Done()

	// Run an immediate evaluation cycle on startup
	s.evaluateAll(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.evaluateAll(ctx)
		}
	}
}

// evaluateAll iterates over registered targets with bounded concurrency (max 4 concurrent).
func (s *Scheduler) evaluateAll(ctx context.Context) {
	s.mu.Lock()
	targetsCopy := make([]EvaluationTarget, len(s.targets))
	copy(targetsCopy, s.targets)
	s.mu.Unlock()

	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for _, target := range targetsCopy {
		targetKey := fmt.Sprintf("%s:%s", target.DependencyName, target.Region)

		s.mu.Lock()
		if s.activeTargets[targetKey] {
			s.mu.Unlock()
			continue // Skip if previous evaluation for this target is still running
		}
		s.activeTargets[targetKey] = true
		s.mu.Unlock()

		wg.Add(1)
		go func(t EvaluationTarget, key string) {
			defer wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.activeTargets, key)
				s.mu.Unlock()
			}()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			}

			start := time.Now()
			evalCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			assessment, err := s.engine.Evaluate(evalCtx, t)
			duration := time.Since(start)

			if s.metrics != nil {
				s.metrics.SchedulerEvaluationsTotal.WithLabelValues(t.DependencyName, t.Region).Inc()
			}

			if err != nil {
				if s.metrics != nil {
					s.metrics.SchedulerEvaluationFailuresTotal.WithLabelValues(t.DependencyName, t.Region).Inc()
				}
				if s.logger != nil {
					s.logger.Warn(fmt.Sprintf("scheduler evaluation failed for %s (%s): %v", t.DependencyName, t.Region, err))
				}
				return
			}

			if s.logger != nil && assessment != nil {
				s.logger.Debug(fmt.Sprintf("scheduler evaluated %s (%s) -> %s in %v",
					t.DependencyName, t.Region, assessment.Status, duration))
			}
		}(target, targetKey)
	}

	wg.Wait()
}
