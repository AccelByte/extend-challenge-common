package repository

import (
	"context"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// CopyRow represents a single row for the unified COPY flush path (M5 Phase 2).
// Carries both absolute and increment events through a single temp table and COPY protocol.
// Status/completion is computed in SQL CASE expressions, not in Go code.
type CopyRow struct {
	UserID       string // User ID
	GoalID       string // Goal ID
	ChallengeID  string // Challenge ID
	Namespace    string // Namespace
	Progress     *int   // Absolute stat value (nil for login/increment events)
	ProgressMode string // "absolute" or "relative"
	IncValue     int    // Increment delta (used when Progress is nil)
	TargetValue  int    // Target value for SQL-side completion check

	// M5 Phase 5: Rotation metadata for SQL CASE rotation logic.
	// Zero values are safe — all rotation SQL branches guard on rotation_boundary IS NOT NULL.
	RotationBoundary *time.Time // Last rotation boundary (nil = no rotation)
	NewExpiresAt     *time.Time // Next expiry timestamp (nil = no rotation)
	AllowReselection bool       // Allow claimed goals to reset on rotation
	ResetProgress    bool       // Reset progress on rotation (default true in config)
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
	// Does NOT update records where status is 'claimed'.
	//
	// DEPRECATED: Use BatchUpsertProgressWithCOPY for better performance (5-10x faster).
	// This method is kept for backwards compatibility and testing.
	BatchUpsertProgress(ctx context.Context, updates []*domain.UserGoalProgress) error

	// BatchUpsertProgressWithCOPY performs batch upsert using PostgreSQL COPY protocol (M5 Phase 2: Unified COPY Path).
	// Accepts CopyRow slices carrying both absolute and increment events through a single flush path.
	// Status and completion are computed in SQL CASE expressions, not in Go code.
	//
	// USAGE: Use this for production workloads requiring high throughput (500+ EPS).
	// Performance: 10-20ms for 1,000 records.
	BatchUpsertProgressWithCOPY(ctx context.Context, rows []CopyRow) error

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

	// BulkInsert creates multiple goal progress records in a single parameterized INSERT query.
	// Uses INSERT ... ON CONFLICT DO NOTHING for idempotency.
	// Used by initialization endpoint to create default goal assignments.
	BulkInsert(ctx context.Context, progresses []*domain.UserGoalProgress) error

	// BulkInsertWithCOPY creates multiple goal progress records using PostgreSQL COPY protocol.
	BulkInsertWithCOPY(ctx context.Context, progresses []*domain.UserGoalProgress) error

	// UpsertGoalActive creates or updates a goal's is_active status.
	UpsertGoalActive(ctx context.Context, progress *domain.UserGoalProgress) error

	// M4: Batch goal activation for random/batch selection

	// BatchUpsertGoalActive activates multiple goals in a single database operation.
	BatchUpsertGoalActive(ctx context.Context, progresses []*domain.UserGoalProgress) error

	// M3 Phase 9: Fast path optimization methods

	// GetUserGoalCount returns the total number of goals for a user (active + inactive).
	GetUserGoalCount(ctx context.Context, userID string) (int, error)

	// GetActiveGoals retrieves only active goal progress records for a user.
	GetActiveGoals(ctx context.Context, userID string) ([]*domain.UserGoalProgress, error)

	// M6: Cleanup methods

	// DeleteExpiredRows deletes expired rows in batches using CTE + PK pattern.
	// Deletes rows where expires_at < cutoff, limited to batchSize per call.
	DeleteExpiredRows(ctx context.Context, cutoff time.Time, batchSize int) (int64, error)

	// DeleteUserData deletes all goal progress data for a specific user (GDPR compliance).
	DeleteUserData(ctx context.Context, userID string) (int64, error)
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
