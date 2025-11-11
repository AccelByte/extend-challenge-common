package repository

import (
	"context"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// ProgressIncrement represents a single atomic increment operation for batch processing.
// Used by BatchIncrementProgress to perform multiple increments in a single query.
type ProgressIncrement struct {
	UserID           string // User ID
	GoalID           string // Goal ID
	ChallengeID      string // Challenge ID
	Namespace        string // Namespace
	Delta            int    // Amount to increment progress by
	TargetValue      int    // Target value for completion check
	IsDailyIncrement bool   // If true, only increments once per day (based on updated_at date)
}

// GoalRepository defines the interface for managing user goal progress in the database.
// This interface abstracts database operations to allow for testing and different implementations.
type GoalRepository interface {
	// GetProgress retrieves a single user's progress for a specific goal.
	// Returns nil if no progress record exists (lazy initialization).
	GetProgress(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error)

	// GetUserProgress retrieves all goal progress records for a specific user.
	// Returns empty slice if user has no progress records.
	// M3 Phase 4: activeOnly parameter filters to only is_active = true goals.
	GetUserProgress(ctx context.Context, userID string, activeOnly bool) ([]*domain.UserGoalProgress, error)

	// GetChallengeProgress retrieves all goal progress for a user within a specific challenge.
	// Returns empty slice if user has no progress for this challenge.
	// M3 Phase 4: activeOnly parameter filters to only is_active = true goals.
	GetChallengeProgress(ctx context.Context, userID, challengeID string, activeOnly bool) ([]*domain.UserGoalProgress, error)

	// UpsertProgress creates or updates a single goal progress record.
	// Uses INSERT ... ON CONFLICT (user_id, goal_id) DO UPDATE.
	// Does NOT update if status is 'claimed' (protection against overwrites).
	UpsertProgress(ctx context.Context, progress *domain.UserGoalProgress) error

	// BatchUpsertProgress performs batch upsert for multiple progress records in a single query.
	// This is the key optimization for the buffered event processing (1,000,000x query reduction).
	// Does NOT update records where status is 'claimed'.
	//
	// DEPRECATED: Use BatchUpsertProgressWithCOPY for better performance (5-10x faster).
	// This method is kept for backwards compatibility and testing.
	//
	// USAGE: Use this for absolute goal types where you have the complete progress value.
	// For increment goals, use BatchIncrementProgress instead.
	BatchUpsertProgress(ctx context.Context, updates []*domain.UserGoalProgress) error

	// BatchUpsertProgressWithCOPY performs batch upsert using PostgreSQL COPY protocol.
	// This is 5-10x faster than BatchUpsertProgress (10-20ms vs 62-105ms for 1,000 records).
	// Does NOT update records where status is 'claimed'.
	//
	// USAGE: Use this for production workloads requiring high throughput (500+ EPS).
	// This method solves the Phase 1 database bottleneck by reducing flush time from
	// 62-105ms to 10-20ms, allowing the system to handle 500+ EPS with <1% data loss.
	BatchUpsertProgressWithCOPY(ctx context.Context, updates []*domain.UserGoalProgress) error

	// IncrementProgress atomically increments a user's progress by a delta value.
	// This is used for increment and daily goal types where progress accumulates.
	//
	// For regular increment goals (isDailyIncrement=false):
	//   - Atomically adds delta to current progress (handles concurrency safely)
	//   - Multiple concurrent increments won't lose updates
	//   - Example: progress=5, delta=3 → progress=8
	//
	// For daily increment goals (isDailyIncrement=true):
	//   - Only increments once per day (uses updated_at date from DB)
	//   - If updated_at is today, this is a no-op (progress unchanged)
	//   - If updated_at is before today, increments by delta and updates timestamp
	//   - Example: Day 1 progress=3 → increment(1) → progress=4
	//              Same day → increment(1) → progress=4 (no change)
	//              Next day → increment(1) → progress=5
	//
	// The targetValue parameter is used for status determination:
	//   - If progress >= targetValue, status becomes 'completed'
	//   - Sets completed_at timestamp when threshold reached
	//
	// USAGE: Use this for single increment operations during event processing.
	// For batch operations (flush), use BatchIncrementProgress instead for better performance.
	//
	// Does NOT update if status is 'claimed'.
	IncrementProgress(ctx context.Context, userID, goalID, challengeID, namespace string,
		delta, targetValue int, isDailyIncrement bool) error

	// BatchIncrementProgress performs batch atomic increment for multiple progress records.
	// This is the key optimization for buffered increment event processing (50x better than individual calls).
	//
	// For regular increment goals (IsDailyIncrement=false):
	//   - Atomically adds delta to current progress for each record
	//   - Single SQL query using UNNEST for all increments
	//
	// For daily increment goals (IsDailyIncrement=true):
	//   - Only increments if updated_at date is before today
	//   - Uses DATE(updated_at AT TIME ZONE 'UTC') for timezone-safe comparison
	//   - Updates updated_at timestamp after increment
	//
	// USAGE: Use this during periodic flush to batch all accumulated increments.
	// This reduces 1,000 individual queries to 1 single batch query (50x performance gain).
	//
	// Performance: 1,000 increments in ~20ms (vs 1,000ms for individual calls)
	//
	// Does NOT update if status is 'claimed'.
	BatchIncrementProgress(ctx context.Context, increments []ProgressIncrement) error

	// MarkAsClaimed updates a goal's status to 'claimed' and sets claimed_at timestamp.
	// Used after successfully granting rewards via AGS Platform Service.
	// Returns error if goal is not in 'completed' status or already claimed.
	MarkAsClaimed(ctx context.Context, userID, goalID string) error

	// BeginTx starts a database transaction and returns a transactional repository.
	// Used for claim flow to ensure atomicity (check status + mark claimed + verify).
	BeginTx(ctx context.Context) (TxRepository, error)

	// M3: Goal assignment control methods

	// GetGoalsByIDs retrieves goal progress records for a user across multiple goal IDs.
	// Returns empty slice if none of the goals have progress records.
	// Used by initialization endpoint to check which default goals already exist.
	GetGoalsByIDs(ctx context.Context, userID string, goalIDs []string) ([]*domain.UserGoalProgress, error)

	// BulkInsert creates multiple goal progress records in a single query.
	// Uses INSERT ... ON CONFLICT DO NOTHING for idempotency.
	// Used by initialization endpoint to create default goal assignments.
	BulkInsert(ctx context.Context, progresses []*domain.UserGoalProgress) error

	// UpsertGoalActive creates or updates a goal's is_active status.
	// If row doesn't exist, creates it with is_active and assigned_at fields.
	// If row exists, updates is_active and assigned_at (only when activating).
	// Used by manual activation/deactivation endpoint.
	UpsertGoalActive(ctx context.Context, progress *domain.UserGoalProgress) error

	// M3 Phase 9: Fast path optimization methods

	// GetUserGoalCount returns the total number of goals for a user (active + inactive).
	// Used by initialization endpoint's fast path to quickly check if user is initialized.
	// If count > 0, user has been initialized → use GetActiveGoals() instead of full init.
	// Performance: < 1ms using idx_user_goal_count index.
	GetUserGoalCount(ctx context.Context, userID string) (int, error)

	// GetActiveGoals retrieves only active goal progress records for a user.
	// Returns empty slice if user has no active goals.
	// Used by initialization endpoint's fast path to avoid querying all 500 goal IDs.
	// Performance: < 5ms using idx_user_goal_active_only partial index.
	GetActiveGoals(ctx context.Context, userID string) ([]*domain.UserGoalProgress, error)
}

// TxRepository represents a transactional repository that supports commit/rollback.
// This ensures the claim flow is atomic (prevents double claims via row-level locking).
type TxRepository interface {
	GoalRepository

	// GetProgressForUpdate retrieves progress with SELECT ... FOR UPDATE (row-level lock).
	// This prevents concurrent claim attempts for the same goal.
	GetProgressForUpdate(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error)

	// Commit commits the transaction.
	Commit() error

	// Rollback rolls back the transaction.
	Rollback() error
}
