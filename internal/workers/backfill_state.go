package workers

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// BackfillProgress represents the current backfill progress.
type BackfillProgress struct {
	IsActive     bool
	StartedAt    *time.Time
	ActorsDone   int
	ActorsTotal  int
	RecordsDone  int
	ErrorCount   int
	LastError    string
	CurrentActor string
}

// BackfillState tracks the state of an active backfill operation.
type BackfillState struct {
	mu       sync.RWMutex
	progress BackfillProgress

	// Callbacks for state changes
	onStart    func()
	onComplete func()
}

// NewBackfillState creates a new backfill state tracker.
func NewBackfillState() *BackfillState {
	return &BackfillState{}
}

// OnStart sets a callback to be called when a backfill starts.
func (s *BackfillState) OnStart(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onStart = fn
}

// OnComplete sets a callback to be called when a backfill completes.
func (s *BackfillState) OnComplete(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onComplete = fn
}

// Start marks a backfill as started.
func (s *BackfillState) Start(ctx context.Context, totalActors int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.progress.IsActive {
		slog.Warn("Attempted to start backfill while one is already active")
		return false
	}

	now := time.Now()
	s.progress = BackfillProgress{
		IsActive:    true,
		StartedAt:   &now,
		ActorsTotal: totalActors,
	}

	slog.Info("Backfill started", "total_actors", totalActors)

	if s.onStart != nil {
		go s.onStart()
	}

	return true
}

// Complete marks the backfill as completed.
func (s *BackfillState) Complete() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.progress.IsActive {
		return
	}

	duration := time.Duration(0)
	if s.progress.StartedAt != nil {
		duration = time.Since(*s.progress.StartedAt)
	}

	slog.Info("Backfill completed",
		"actors_done", s.progress.ActorsDone,
		"records_done", s.progress.RecordsDone,
		"errors", s.progress.ErrorCount,
		"duration", duration)

	s.progress.IsActive = false

	if s.onComplete != nil {
		go s.onComplete()
	}
}

// UpdateProgress updates the backfill progress.
func (s *BackfillState) UpdateProgress(actorsDone, recordsDone int, currentActor string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.progress.ActorsDone = actorsDone
	s.progress.RecordsDone = recordsDone
	s.progress.CurrentActor = currentActor
}

// RecordError records an error during backfill.
func (s *BackfillState) RecordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.progress.ErrorCount++
	s.progress.LastError = err.Error()
}

// IsActive returns whether a backfill is currently active.
func (s *BackfillState) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.progress.IsActive
}

// Progress returns the current backfill progress.
func (s *BackfillState) Progress() BackfillProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.progress
}

// Reset resets the backfill state (use with caution).
func (s *BackfillState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress = BackfillProgress{}
}
