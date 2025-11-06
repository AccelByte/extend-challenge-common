package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	"github.com/AccelByte/extend-challenge-common/pkg/errors"

	"github.com/lib/pq" // PostgreSQL driver and array support
)

// PostgresGoalRepository implements GoalRepository interface using PostgreSQL.
type PostgresGoalRepository struct {
	db *sql.DB
}

// NewPostgresGoalRepository creates a new PostgreSQL-backed goal repository.
func NewPostgresGoalRepository(db *sql.DB) *PostgresGoalRepository {
	return &PostgresGoalRepository{
		db: db,
	}
}

// GetProgress retrieves a single user's progress for a specific goal.
func (r *PostgresGoalRepository) GetProgress(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error) {
	query := `
		SELECT user_id, goal_id, challenge_id, namespace, progress, status,
		       completed_at, claimed_at, created_at, updated_at,
		       is_active, assigned_at, expires_at
		FROM user_goal_progress
		WHERE user_id = $1 AND goal_id = $2
	`

	var progress domain.UserGoalProgress
	err := r.db.QueryRowContext(ctx, query, userID, goalID).Scan(
		&progress.UserID,
		&progress.GoalID,
		&progress.ChallengeID,
		&progress.Namespace,
		&progress.Progress,
		&progress.Status,
		&progress.CompletedAt,
		&progress.ClaimedAt,
		&progress.CreatedAt,
		&progress.UpdatedAt,
		&progress.IsActive,
		&progress.AssignedAt,
		&progress.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No progress record exists (lazy initialization)
	}

	if err != nil {
		return nil, errors.ErrDatabaseError("get progress", err)
	}

	return &progress, nil
}

// GetUserProgress retrieves all goal progress records for a specific user.
// M3 Phase 4: activeOnly parameter filters to only is_active = true goals.
func (r *PostgresGoalRepository) GetUserProgress(ctx context.Context, userID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	query := `
		SELECT user_id, goal_id, challenge_id, namespace, progress, status,
		       completed_at, claimed_at, created_at, updated_at,
		       is_active, assigned_at, expires_at
		FROM user_goal_progress
		WHERE user_id = $1
	`

	// M3 Phase 4: Add is_active filter when activeOnly is true
	if activeOnly {
		query += " AND is_active = true"
	}

	query += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, errors.ErrDatabaseError("get user progress", err)
	}
	defer func() { _ = rows.Close() }()

	return r.scanProgressRows(rows)
}

// GetChallengeProgress retrieves all goal progress for a user within a specific challenge.
// M3 Phase 4: activeOnly parameter filters to only is_active = true goals.
func (r *PostgresGoalRepository) GetChallengeProgress(ctx context.Context, userID, challengeID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	query := `
		SELECT user_id, goal_id, challenge_id, namespace, progress, status,
		       completed_at, claimed_at, created_at, updated_at,
		       is_active, assigned_at, expires_at
		FROM user_goal_progress
		WHERE user_id = $1 AND challenge_id = $2
	`

	// M3 Phase 4: Add is_active filter when activeOnly is true
	if activeOnly {
		query += " AND is_active = true"
	}

	query += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, userID, challengeID)
	if err != nil {
		return nil, errors.ErrDatabaseError("get challenge progress", err)
	}
	defer func() { _ = rows.Close() }()

	return r.scanProgressRows(rows)
}

// UpsertProgress creates or updates a single goal progress record.
func (r *PostgresGoalRepository) UpsertProgress(ctx context.Context, progress *domain.UserGoalProgress) error {
	query := `
		INSERT INTO user_goal_progress (
			user_id, goal_id, challenge_id, namespace,
			progress, status, completed_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, NOW()
		)
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = EXCLUDED.progress,
			status = EXCLUDED.status,
			completed_at = EXCLUDED.completed_at,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`

	_, err := r.db.ExecContext(ctx, query,
		progress.UserID,
		progress.GoalID,
		progress.ChallengeID,
		progress.Namespace,
		progress.Progress,
		progress.Status,
		progress.CompletedAt,
	)

	if err != nil {
		return errors.ErrDatabaseError("upsert progress", err)
	}

	return nil
}

// BatchUpsertProgress performs batch upsert for multiple progress records in a single query.
// This is the key optimization for buffered event processing (1,000,000x query reduction).
//
// DEPRECATED: Use BatchUpsertProgressWithCOPY for better performance (5-10x faster).
// This method is kept for backwards compatibility and testing.
func (r *PostgresGoalRepository) BatchUpsertProgress(ctx context.Context, updates []*domain.UserGoalProgress) error {
	if len(updates) == 0 {
		return nil
	}

	// Check PostgreSQL parameter limit (65,535 parameters)
	// With 7 parameters per row, max is ~9,000 rows
	if len(updates) > 9000 {
		return fmt.Errorf("batch size exceeds PostgreSQL parameter limit: %d rows (max 9000)", len(updates))
	}

	// Build dynamic query with correct number of placeholders
	valueStrings := make([]string, 0, len(updates))
	valueArgs := make([]interface{}, 0, len(updates)*7)

	for i, update := range updates {
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, NOW())",
			i*7+1, i*7+2, i*7+3, i*7+4, i*7+5, i*7+6, i*7+7,
		))
		valueArgs = append(valueArgs,
			update.UserID,
			update.GoalID,
			update.ChallengeID,
			update.Namespace,
			update.Progress,
			update.Status,
			update.CompletedAt,
		)
	}

	// Safe: fmt.Sprintf only builds the VALUES structure with placeholders ($1, $2, etc.)
	// All actual values are passed via parameterized query (valueArgs), not string interpolation
	// #nosec G201
	query := fmt.Sprintf(`
		INSERT INTO user_goal_progress (
			user_id, goal_id, challenge_id, namespace,
			progress, status, completed_at, updated_at
		) VALUES %s
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = EXCLUDED.progress,
			status = EXCLUDED.status,
			completed_at = EXCLUDED.completed_at,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`, strings.Join(valueStrings, ","))

	_, err := r.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return errors.ErrDatabaseError("batch upsert progress", err)
	}

	return nil
}

// BatchUpsertProgressWithCOPY performs batch upsert using PostgreSQL COPY protocol.
// This is 5-10x faster than BatchUpsertProgress (10-20ms vs 62-105ms for 1,000 records).
//
// Implementation:
// 1. Creates temporary table (session-local, auto-dropped)
// 2. Uses COPY FROM STDIN to bulk load data (bypasses query parser)
// 3. Merges temp table into main table using INSERT ... SELECT with ON CONFLICT
// 4. Maintains claimed protection logic (does not update claimed goals)
//
// This method solves the Phase 1 database bottleneck by reducing flush time from
// 62-105ms to 10-20ms, allowing the system to handle 500+ EPS with <1% data loss.
func (r *PostgresGoalRepository) BatchUpsertProgressWithCOPY(ctx context.Context, updates []*domain.UserGoalProgress) error {
	if len(updates) == 0 {
		return nil
	}

	// Start transaction for temp table + merge operation
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.ErrDatabaseError("begin transaction for COPY", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Step 1: Create temporary table (session-local, automatically dropped at end of session)
	_, err = tx.ExecContext(ctx, `
		CREATE TEMP TABLE IF NOT EXISTS temp_user_goal_progress (
			user_id VARCHAR(100) NOT NULL,
			goal_id VARCHAR(100) NOT NULL,
			challenge_id VARCHAR(100) NOT NULL,
			namespace VARCHAR(100) NOT NULL,
			progress INT NOT NULL,
			status VARCHAR(20) NOT NULL,
			completed_at TIMESTAMP NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		) ON COMMIT DROP
	`)
	if err != nil {
		return errors.ErrDatabaseError("create temp table for COPY", err)
	}

	// Step 2: Prepare COPY statement
	stmt, err := tx.PrepareContext(ctx, pq.CopyIn(
		"temp_user_goal_progress",
		"user_id", "goal_id", "challenge_id", "namespace",
		"progress", "status", "completed_at", "updated_at",
	))
	if err != nil {
		return errors.ErrDatabaseError("prepare COPY statement", err)
	}
	defer func() { _ = stmt.Close() }()

	// Step 3: Bulk load data into temp table using COPY
	now := time.Now()
	for _, update := range updates {
		_, err = stmt.ExecContext(ctx,
			update.UserID,
			update.GoalID,
			update.ChallengeID,
			update.Namespace,
			update.Progress,
			update.Status,
			update.CompletedAt,
			now,
		)
		if err != nil {
			return errors.ErrDatabaseError("execute COPY row", err)
		}
	}

	// Step 4: Execute COPY (flush buffered rows to temp table)
	_, err = stmt.ExecContext(ctx)
	if err != nil {
		return errors.ErrDatabaseError("flush COPY to temp table", err)
	}

	// Step 5: Merge temp table into main table with ON CONFLICT logic
	// This maintains the same upsert semantics as BatchUpsertProgress
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_goal_progress (
			user_id, goal_id, challenge_id, namespace,
			progress, status, completed_at, updated_at
		)
		SELECT
			user_id, goal_id, challenge_id, namespace,
			progress, status, completed_at, NOW()
		FROM temp_user_goal_progress
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = EXCLUDED.progress,
			status = EXCLUDED.status,
			completed_at = EXCLUDED.completed_at,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`)
	if err != nil {
		return errors.ErrDatabaseError("merge temp table into user_goal_progress", err)
	}

	// Step 6: Commit transaction (temp table automatically dropped)
	err = tx.Commit()
	if err != nil {
		return errors.ErrDatabaseError("commit COPY transaction", err)
	}

	return nil
}

// IncrementProgress atomically increments a user's progress by a delta value.
func (r *PostgresGoalRepository) IncrementProgress(ctx context.Context, userID, goalID, challengeID, namespace string, delta, targetValue int, isDailyIncrement bool) error {
	if isDailyIncrement {
		return r.incrementProgressDaily(ctx, userID, goalID, challengeID, namespace, delta, targetValue)
	}
	return r.incrementProgressRegular(ctx, userID, goalID, challengeID, namespace, delta, targetValue)
}

// incrementProgressRegular handles regular increments (always adds delta)
func (r *PostgresGoalRepository) incrementProgressRegular(ctx context.Context, userID, goalID, challengeID, namespace string, delta, targetValue int) error {
	query := `
		INSERT INTO user_goal_progress (
			user_id,
			goal_id,
			challenge_id,
			namespace,
			progress,
			status,
			completed_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5::INT,
			CASE WHEN $5::INT >= $6::INT THEN 'completed' ELSE 'in_progress' END,
			CASE WHEN $5::INT >= $6::INT THEN NOW() ELSE NULL END,
			NOW()
		)
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = user_goal_progress.progress + $5::INT,
			status = CASE
				WHEN user_goal_progress.progress + $5::INT >= $6::INT THEN 'completed'
				ELSE 'in_progress'
			END,
			completed_at = CASE
				WHEN user_goal_progress.progress + $5::INT >= $6::INT AND user_goal_progress.completed_at IS NULL
					THEN NOW()
				ELSE user_goal_progress.completed_at
			END,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`

	_, err := r.db.ExecContext(ctx, query, userID, goalID, challengeID, namespace, delta, targetValue)
	if err != nil {
		return errors.ErrDatabaseError("increment progress (regular)", err)
	}

	return nil
}

// incrementProgressDaily handles daily increments (only once per day)
// Uses timezone-safe date comparison to prevent timezone-related bugs
func (r *PostgresGoalRepository) incrementProgressDaily(ctx context.Context, userID, goalID, challengeID, namespace string, delta, targetValue int) error {
	query := `
		INSERT INTO user_goal_progress (
			user_id,
			goal_id,
			challenge_id,
			namespace,
			progress,
			status,
			completed_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, 1,  -- Initial progress = 1 for first day
			CASE WHEN 1 >= $6::INT THEN 'completed' ELSE 'in_progress' END,
			CASE WHEN 1 >= $6::INT THEN NOW() ELSE NULL END,
			NOW()
		)
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = CASE
				-- Same day (UTC): don't increment
				WHEN DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC')
					THEN user_goal_progress.progress
				-- New day: increment by delta
				ELSE user_goal_progress.progress + $5::INT
			END,
			status = CASE
				-- Calculate new progress first, then check threshold
				WHEN DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC') THEN
					-- Same day, progress unchanged
					CASE WHEN user_goal_progress.progress >= $6::INT THEN 'completed' ELSE 'in_progress' END
				ELSE
					-- New day, check incremented progress
					CASE WHEN user_goal_progress.progress + $5::INT >= $6::INT THEN 'completed' ELSE 'in_progress' END
			END,
			completed_at = CASE
				WHEN DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC') THEN
					user_goal_progress.completed_at  -- Same day, keep existing
				WHEN user_goal_progress.progress + $5::INT >= $6::INT AND user_goal_progress.completed_at IS NULL THEN
					NOW()  -- New day and just completed
				ELSE
					user_goal_progress.completed_at  -- Keep existing
			END,
			updated_at = NOW()  -- Always update timestamp (for daily tracking)
		WHERE user_goal_progress.status != 'claimed'
	`

	_, err := r.db.ExecContext(ctx, query, userID, goalID, challengeID, namespace, delta, targetValue)
	if err != nil {
		return errors.ErrDatabaseError("increment progress (daily)", err)
	}

	return nil
}

// BatchIncrementProgress performs batch atomic increment for multiple progress records.
// Uses PostgreSQL UNNEST for efficient batch processing (50x faster than individual calls).
func (r *PostgresGoalRepository) BatchIncrementProgress(ctx context.Context, increments []ProgressIncrement) error {
	if len(increments) == 0 {
		return nil
	}

	// Build arrays for UNNEST
	userIDs := make([]string, len(increments))
	goalIDs := make([]string, len(increments))
	challengeIDs := make([]string, len(increments))
	namespaces := make([]string, len(increments))
	deltas := make([]int, len(increments))
	targetValues := make([]int, len(increments))
	isDailyFlags := make([]bool, len(increments))

	for i, inc := range increments {
		userIDs[i] = inc.UserID
		goalIDs[i] = inc.GoalID
		challengeIDs[i] = inc.ChallengeID
		namespaces[i] = inc.Namespace
		deltas[i] = inc.Delta
		targetValues[i] = inc.TargetValue
		isDailyFlags[i] = inc.IsDailyIncrement
	}

	// Complex query using UNNEST for batch operations with daily increment support
	// Uses timezone-safe date comparison (AT TIME ZONE 'UTC') to prevent timezone bugs
	query := `
		INSERT INTO user_goal_progress (
			user_id,
			goal_id,
			challenge_id,
			namespace,
			progress,
			status,
			completed_at,
			updated_at
		)
		SELECT
			t.user_id,
			t.goal_id,
			t.challenge_id,
			t.namespace,
			t.delta,  -- Initial progress for INSERT
			initial.status,
			initial.completed_at,
			NOW()
		FROM UNNEST(
			$1::VARCHAR(100)[],  -- user_ids
			$2::VARCHAR(100)[],  -- goal_ids
			$3::VARCHAR(100)[],  -- challenge_ids
			$4::VARCHAR(100)[],  -- namespaces
			$5::INT[],           -- deltas (initial progress for INSERT)
			$6::INT[],           -- target_values
			$7::BOOLEAN[]        -- is_daily_increment flags
		) AS t(user_id, goal_id, challenge_id, namespace, delta, target_value, is_daily)
		-- Determine initial status and completed_at for INSERT (first-time progress)
		CROSS JOIN LATERAL (
			SELECT
				CASE WHEN t.delta >= t.target_value THEN 'completed' ELSE 'in_progress' END as status,
				CASE WHEN t.delta >= t.target_value THEN NOW() ELSE NULL END as completed_at
		) AS initial
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = CASE
				-- Daily increment: check if same day (UTC)
				WHEN (SELECT is_daily FROM UNNEST($7::BOOLEAN[], $2::VARCHAR(100)[]) AS u(is_daily, gid)
				      WHERE u.gid = user_goal_progress.goal_id LIMIT 1) = true
				     AND DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC')
					THEN user_goal_progress.progress  -- Same day, no increment
				ELSE
					user_goal_progress.progress + (
						SELECT delta FROM UNNEST($5::INT[], $2::VARCHAR(100)[]) AS u(delta, gid)
						WHERE u.gid = user_goal_progress.goal_id LIMIT 1
					)  -- Different day or regular increment
			END,
			status = CASE
				-- Calculate based on new progress value
				WHEN (SELECT is_daily FROM UNNEST($7::BOOLEAN[], $2::VARCHAR(100)[]) AS u(is_daily, gid)
				      WHERE u.gid = user_goal_progress.goal_id LIMIT 1) = true
				     AND DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC') THEN
					-- Same day: status based on current progress
					CASE WHEN user_goal_progress.progress >= (
						SELECT target_value FROM UNNEST($6::INT[], $2::VARCHAR(100)[]) AS u(target_value, gid)
						WHERE u.gid = user_goal_progress.goal_id LIMIT 1
					) THEN 'completed' ELSE 'in_progress' END
				ELSE
					-- New day or regular: status based on incremented progress
					CASE WHEN user_goal_progress.progress + (
						SELECT delta FROM UNNEST($5::INT[], $2::VARCHAR(100)[]) AS u(delta, gid)
						WHERE u.gid = user_goal_progress.goal_id LIMIT 1
					) >= (
						SELECT target_value FROM UNNEST($6::INT[], $2::VARCHAR(100)[]) AS u(target_value, gid)
						WHERE u.gid = user_goal_progress.goal_id LIMIT 1
					) THEN 'completed' ELSE 'in_progress' END
			END,
			completed_at = CASE
				WHEN (SELECT is_daily FROM UNNEST($7::BOOLEAN[], $2::VARCHAR(100)[]) AS u(is_daily, gid)
				      WHERE u.gid = user_goal_progress.goal_id LIMIT 1) = true
				     AND DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC') THEN
					user_goal_progress.completed_at  -- Same day, keep existing
				WHEN user_goal_progress.progress + (
					SELECT delta FROM UNNEST($5::INT[], $2::VARCHAR(100)[]) AS u(delta, gid)
					WHERE u.gid = user_goal_progress.goal_id LIMIT 1
				) >= (
					SELECT target_value FROM UNNEST($6::INT[], $2::VARCHAR(100)[]) AS u(target_value, gid)
					WHERE u.gid = user_goal_progress.goal_id LIMIT 1
				) AND user_goal_progress.completed_at IS NULL THEN
					NOW()  -- Just completed
				ELSE
					user_goal_progress.completed_at  -- Keep existing
			END,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`

	_, err := r.db.ExecContext(ctx, query,
		pq.Array(userIDs),
		pq.Array(goalIDs),
		pq.Array(challengeIDs),
		pq.Array(namespaces),
		pq.Array(deltas),
		pq.Array(targetValues),
		pq.Array(isDailyFlags),
	)

	if err != nil {
		return errors.ErrDatabaseError("batch increment progress", err)
	}

	return nil
}

// MarkAsClaimed updates a goal's status to 'claimed' and sets claimed_at timestamp.
func (r *PostgresGoalRepository) MarkAsClaimed(ctx context.Context, userID, goalID string) error {
	query := `
		UPDATE user_goal_progress
		SET status = 'claimed',
			claimed_at = NOW(),
			updated_at = NOW()
		WHERE user_id = $1 AND goal_id = $2
		AND status = 'completed'
		AND claimed_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, userID, goalID)
	if err != nil {
		return errors.ErrDatabaseError("mark as claimed", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.ErrDatabaseError("check rows affected", err)
	}

	if rowsAffected == 0 {
		// No rows updated - goal either doesn't exist, not completed, or already claimed
		// Caller should check progress status to determine specific error
		return errors.ErrGoalNotCompleted(goalID)
	}

	return nil
}

// M3: Goal assignment control methods

// GetGoalsByIDs retrieves goal progress records for a user across multiple goal IDs.
func (r *PostgresGoalRepository) GetGoalsByIDs(ctx context.Context, userID string, goalIDs []string) ([]*domain.UserGoalProgress, error) {
	if len(goalIDs) == 0 {
		return []*domain.UserGoalProgress{}, nil
	}

	query := `
		SELECT user_id, goal_id, challenge_id, namespace, progress, status,
		       completed_at, claimed_at, created_at, updated_at,
		       is_active, assigned_at, expires_at
		FROM user_goal_progress
		WHERE user_id = $1 AND goal_id = ANY($2)
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userID, pq.Array(goalIDs))
	if err != nil {
		return nil, errors.ErrDatabaseError("get goals by IDs", err)
	}
	defer func() { _ = rows.Close() }()

	return r.scanProgressRows(rows)
}

// BulkInsert creates multiple goal progress records in a single query.
func (r *PostgresGoalRepository) BulkInsert(ctx context.Context, progresses []*domain.UserGoalProgress) error {
	if len(progresses) == 0 {
		return nil
	}

	// Build values for bulk insert
	valueStrings := make([]string, 0, len(progresses))
	valueArgs := make([]interface{}, 0, len(progresses)*13)

	for i, p := range progresses {
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, NOW(), NOW(), $%d, $%d, $%d)",
			i*13+1, i*13+2, i*13+3, i*13+4, i*13+5, i*13+6, i*13+7, i*13+8, i*13+9, i*13+10, i*13+11,
		))

		valueArgs = append(valueArgs,
			p.UserID,
			p.GoalID,
			p.ChallengeID,
			p.Namespace,
			p.Progress,
			p.Status,
			p.CompletedAt,
			p.ClaimedAt,
			p.IsActive,
			p.AssignedAt,
			p.ExpiresAt,
		)
	}

	//nolint:gosec // Safe: valueStrings contains only parameterized placeholders like "($1, $2, $3)", not user input
	query := fmt.Sprintf(`
		INSERT INTO user_goal_progress (
			user_id, goal_id, challenge_id, namespace,
			progress, status, completed_at, claimed_at,
			created_at, updated_at,
			is_active, assigned_at, expires_at
		) VALUES %s
		ON CONFLICT (user_id, goal_id) DO NOTHING
	`, strings.Join(valueStrings, ","))

	_, err := r.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return errors.ErrDatabaseError("bulk insert goals", err)
	}

	return nil
}

// UpsertGoalActive creates or updates a goal's is_active status.
func (r *PostgresGoalRepository) UpsertGoalActive(ctx context.Context, progress *domain.UserGoalProgress) error {
	query := `
		INSERT INTO user_goal_progress (
			user_id, goal_id, challenge_id, namespace,
			progress, status, is_active, assigned_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
		)
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			is_active = EXCLUDED.is_active,
			assigned_at = CASE
				WHEN EXCLUDED.is_active = true THEN NOW()
				ELSE user_goal_progress.assigned_at
			END,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query,
		progress.UserID,
		progress.GoalID,
		progress.ChallengeID,
		progress.Namespace,
		progress.Progress,
		progress.Status,
		progress.IsActive,
		progress.AssignedAt,
	)

	if err != nil {
		return errors.ErrDatabaseError("upsert goal active", err)
	}

	return nil
}

// BeginTx starts a database transaction and returns a transactional repository.
func (r *PostgresGoalRepository) BeginTx(ctx context.Context) (TxRepository, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.ErrDatabaseError("begin transaction", err)
	}

	return &PostgresTxRepository{
		tx:     tx,
		parent: r,
	}, nil
}

// scanProgressRows is a helper to scan multiple progress rows.
func (r *PostgresGoalRepository) scanProgressRows(rows *sql.Rows) ([]*domain.UserGoalProgress, error) {
	var results []*domain.UserGoalProgress

	for rows.Next() {
		var progress domain.UserGoalProgress
		err := rows.Scan(
			&progress.UserID,
			&progress.GoalID,
			&progress.ChallengeID,
			&progress.Namespace,
			&progress.Progress,
			&progress.Status,
			&progress.CompletedAt,
			&progress.ClaimedAt,
			&progress.CreatedAt,
			&progress.UpdatedAt,
			&progress.IsActive,
			&progress.AssignedAt,
			&progress.ExpiresAt,
		)
		if err != nil {
			return nil, errors.ErrDatabaseError("scan progress row", err)
		}
		results = append(results, &progress)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.ErrDatabaseError("iterate progress rows", err)
	}

	return results, nil
}

// PostgresTxRepository implements TxRepository interface for transactional operations.
type PostgresTxRepository struct {
	tx     *sql.Tx
	parent *PostgresGoalRepository
}

// GetProgress retrieves progress within a transaction.
func (r *PostgresTxRepository) GetProgress(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error) {
	query := `
		SELECT user_id, goal_id, challenge_id, namespace, progress, status,
		       completed_at, claimed_at, created_at, updated_at,
		       is_active, assigned_at, expires_at
		FROM user_goal_progress
		WHERE user_id = $1 AND goal_id = $2
	`

	var progress domain.UserGoalProgress
	err := r.tx.QueryRowContext(ctx, query, userID, goalID).Scan(
		&progress.UserID,
		&progress.GoalID,
		&progress.ChallengeID,
		&progress.Namespace,
		&progress.Progress,
		&progress.Status,
		&progress.CompletedAt,
		&progress.ClaimedAt,
		&progress.CreatedAt,
		&progress.UpdatedAt,
		&progress.IsActive,
		&progress.AssignedAt,
		&progress.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, errors.ErrDatabaseError("get progress in transaction", err)
	}

	return &progress, nil
}

// GetProgressForUpdate retrieves progress with SELECT ... FOR UPDATE (row-level lock).
func (r *PostgresTxRepository) GetProgressForUpdate(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error) {
	query := `
		SELECT user_id, goal_id, challenge_id, namespace, progress, status,
		       completed_at, claimed_at, created_at, updated_at,
		       is_active, assigned_at, expires_at
		FROM user_goal_progress
		WHERE user_id = $1 AND goal_id = $2
		FOR UPDATE
	`

	var progress domain.UserGoalProgress
	err := r.tx.QueryRowContext(ctx, query, userID, goalID).Scan(
		&progress.UserID,
		&progress.GoalID,
		&progress.ChallengeID,
		&progress.Namespace,
		&progress.Progress,
		&progress.Status,
		&progress.CompletedAt,
		&progress.ClaimedAt,
		&progress.CreatedAt,
		&progress.UpdatedAt,
		&progress.IsActive,
		&progress.AssignedAt,
		&progress.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, errors.ErrDatabaseError("get progress for update", err)
	}

	return &progress, nil
}

// GetUserProgress retrieves all user progress within a transaction.
// M3 Phase 4: activeOnly parameter filters to only is_active = true goals.
func (r *PostgresTxRepository) GetUserProgress(ctx context.Context, userID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	query := `
		SELECT user_id, goal_id, challenge_id, namespace, progress, status,
		       completed_at, claimed_at, created_at, updated_at,
		       is_active, assigned_at, expires_at
		FROM user_goal_progress
		WHERE user_id = $1
	`

	// M3 Phase 4: Add is_active filter when activeOnly is true
	if activeOnly {
		query += " AND is_active = true"
	}

	query += " ORDER BY created_at ASC"

	rows, err := r.tx.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, errors.ErrDatabaseError("get user progress in transaction", err)
	}
	defer func() { _ = rows.Close() }()

	return r.parent.scanProgressRows(rows)
}

// GetChallengeProgress retrieves challenge progress within a transaction.
// M3 Phase 4: activeOnly parameter filters to only is_active = true goals.
func (r *PostgresTxRepository) GetChallengeProgress(ctx context.Context, userID, challengeID string, activeOnly bool) ([]*domain.UserGoalProgress, error) {
	query := `
		SELECT user_id, goal_id, challenge_id, namespace, progress, status,
		       completed_at, claimed_at, created_at, updated_at,
		       is_active, assigned_at, expires_at
		FROM user_goal_progress
		WHERE user_id = $1 AND challenge_id = $2
	`

	// M3 Phase 4: Add is_active filter when activeOnly is true
	if activeOnly {
		query += " AND is_active = true"
	}

	query += " ORDER BY created_at ASC"

	rows, err := r.tx.QueryContext(ctx, query, userID, challengeID)
	if err != nil {
		return nil, errors.ErrDatabaseError("get challenge progress in transaction", err)
	}
	defer func() { _ = rows.Close() }()

	return r.parent.scanProgressRows(rows)
}

// UpsertProgress upserts progress within a transaction.
func (r *PostgresTxRepository) UpsertProgress(ctx context.Context, progress *domain.UserGoalProgress) error {
	query := `
		INSERT INTO user_goal_progress (
			user_id, goal_id, challenge_id, namespace,
			progress, status, completed_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, NOW()
		)
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = EXCLUDED.progress,
			status = EXCLUDED.status,
			completed_at = EXCLUDED.completed_at,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`

	_, err := r.tx.ExecContext(ctx, query,
		progress.UserID,
		progress.GoalID,
		progress.ChallengeID,
		progress.Namespace,
		progress.Progress,
		progress.Status,
		progress.CompletedAt,
	)

	if err != nil {
		return errors.ErrDatabaseError("upsert progress in transaction", err)
	}

	return nil
}

// BatchUpsertProgress batch upserts within a transaction.
// DEPRECATED: Use BatchUpsertProgressWithCOPY for better performance.
func (r *PostgresTxRepository) BatchUpsertProgress(ctx context.Context, updates []*domain.UserGoalProgress) error {
	if len(updates) == 0 {
		return nil
	}

	if len(updates) > 9000 {
		return fmt.Errorf("batch size exceeds PostgreSQL parameter limit: %d rows (max 9000)", len(updates))
	}

	valueStrings := make([]string, 0, len(updates))
	valueArgs := make([]interface{}, 0, len(updates)*7)

	for i, update := range updates {
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, NOW())",
			i*7+1, i*7+2, i*7+3, i*7+4, i*7+5, i*7+6, i*7+7,
		))
		valueArgs = append(valueArgs,
			update.UserID,
			update.GoalID,
			update.ChallengeID,
			update.Namespace,
			update.Progress,
			update.Status,
			update.CompletedAt,
		)
	}

	// Safe: fmt.Sprintf only builds the VALUES structure with placeholders ($1, $2, etc.)
	// All actual values are passed via parameterized query (valueArgs), not string interpolation
	// #nosec G201
	query := fmt.Sprintf(`
		INSERT INTO user_goal_progress (
			user_id, goal_id, challenge_id, namespace,
			progress, status, completed_at, updated_at
		) VALUES %s
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = EXCLUDED.progress,
			status = EXCLUDED.status,
			completed_at = EXCLUDED.completed_at,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`, strings.Join(valueStrings, ","))

	_, err := r.tx.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return errors.ErrDatabaseError("batch upsert progress in transaction", err)
	}

	return nil
}

// BatchUpsertProgressWithCOPY performs batch upsert using COPY protocol within a transaction.
// This is 5-10x faster than BatchUpsertProgress.
func (r *PostgresTxRepository) BatchUpsertProgressWithCOPY(ctx context.Context, updates []*domain.UserGoalProgress) error {
	if len(updates) == 0 {
		return nil
	}

	// Note: We're already in a transaction (r.tx), so we don't need to BEGIN/COMMIT
	// The temp table will be dropped when the parent transaction commits/rollbacks

	// Step 1: Create temporary table
	_, err := r.tx.ExecContext(ctx, `
		CREATE TEMP TABLE IF NOT EXISTS temp_user_goal_progress (
			user_id VARCHAR(100) NOT NULL,
			goal_id VARCHAR(100) NOT NULL,
			challenge_id VARCHAR(100) NOT NULL,
			namespace VARCHAR(100) NOT NULL,
			progress INT NOT NULL,
			status VARCHAR(20) NOT NULL,
			completed_at TIMESTAMP NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		) ON COMMIT DROP
	`)
	if err != nil {
		return errors.ErrDatabaseError("create temp table for COPY in transaction", err)
	}

	// Step 2: Prepare COPY statement
	stmt, err := r.tx.PrepareContext(ctx, pq.CopyIn(
		"temp_user_goal_progress",
		"user_id", "goal_id", "challenge_id", "namespace",
		"progress", "status", "completed_at", "updated_at",
	))
	if err != nil {
		return errors.ErrDatabaseError("prepare COPY statement in transaction", err)
	}
	defer func() { _ = stmt.Close() }()

	// Step 3: Bulk load data
	now := time.Now()
	for _, update := range updates {
		_, err = stmt.ExecContext(ctx,
			update.UserID,
			update.GoalID,
			update.ChallengeID,
			update.Namespace,
			update.Progress,
			update.Status,
			update.CompletedAt,
			now,
		)
		if err != nil {
			return errors.ErrDatabaseError("execute COPY row in transaction", err)
		}
	}

	// Step 4: Execute COPY
	_, err = stmt.ExecContext(ctx)
	if err != nil {
		return errors.ErrDatabaseError("flush COPY to temp table in transaction", err)
	}

	// Step 5: Merge temp table into main table
	_, err = r.tx.ExecContext(ctx, `
		INSERT INTO user_goal_progress (
			user_id, goal_id, challenge_id, namespace,
			progress, status, completed_at, updated_at
		)
		SELECT
			user_id, goal_id, challenge_id, namespace,
			progress, status, completed_at, NOW()
		FROM temp_user_goal_progress
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = EXCLUDED.progress,
			status = EXCLUDED.status,
			completed_at = EXCLUDED.completed_at,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`)
	if err != nil {
		return errors.ErrDatabaseError("merge temp table into user_goal_progress in transaction", err)
	}

	return nil
}

// IncrementProgress atomically increments progress within a transaction.
func (r *PostgresTxRepository) IncrementProgress(ctx context.Context, userID, goalID, challengeID, namespace string, delta, targetValue int, isDailyIncrement bool) error {
	if isDailyIncrement {
		return r.incrementProgressDaily(ctx, userID, goalID, challengeID, namespace, delta, targetValue)
	}
	return r.incrementProgressRegular(ctx, userID, goalID, challengeID, namespace, delta, targetValue)
}

// incrementProgressRegular handles regular increments within a transaction
func (r *PostgresTxRepository) incrementProgressRegular(ctx context.Context, userID, goalID, challengeID, namespace string, delta, targetValue int) error {
	query := `
		INSERT INTO user_goal_progress (
			user_id,
			goal_id,
			challenge_id,
			namespace,
			progress,
			status,
			completed_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5::INT,
			CASE WHEN $5::INT >= $6::INT THEN 'completed' ELSE 'in_progress' END,
			CASE WHEN $5::INT >= $6::INT THEN NOW() ELSE NULL END,
			NOW()
		)
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = user_goal_progress.progress + $5::INT,
			status = CASE
				WHEN user_goal_progress.progress + $5::INT >= $6::INT THEN 'completed'
				ELSE 'in_progress'
			END,
			completed_at = CASE
				WHEN user_goal_progress.progress + $5::INT >= $6::INT AND user_goal_progress.completed_at IS NULL
					THEN NOW()
				ELSE user_goal_progress.completed_at
			END,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`

	_, err := r.tx.ExecContext(ctx, query, userID, goalID, challengeID, namespace, delta, targetValue)
	if err != nil {
		return errors.ErrDatabaseError("increment progress (regular) in transaction", err)
	}

	return nil
}

// incrementProgressDaily handles daily increments within a transaction
func (r *PostgresTxRepository) incrementProgressDaily(ctx context.Context, userID, goalID, challengeID, namespace string, delta, targetValue int) error {
	query := `
		INSERT INTO user_goal_progress (
			user_id,
			goal_id,
			challenge_id,
			namespace,
			progress,
			status,
			completed_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, 1,
			CASE WHEN 1 >= $6::INT THEN 'completed' ELSE 'in_progress' END,
			CASE WHEN 1 >= $6::INT THEN NOW() ELSE NULL END,
			NOW()
		)
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = CASE
				WHEN DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC')
					THEN user_goal_progress.progress
				ELSE user_goal_progress.progress + $5::INT
			END,
			status = CASE
				WHEN DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC') THEN
					CASE WHEN user_goal_progress.progress >= $6::INT THEN 'completed' ELSE 'in_progress' END
				ELSE
					CASE WHEN user_goal_progress.progress + $5::INT >= $6::INT THEN 'completed' ELSE 'in_progress' END
			END,
			completed_at = CASE
				WHEN DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC') THEN
					user_goal_progress.completed_at
				WHEN user_goal_progress.progress + $5::INT >= $6::INT AND user_goal_progress.completed_at IS NULL THEN
					NOW()
				ELSE
					user_goal_progress.completed_at
			END,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`

	_, err := r.tx.ExecContext(ctx, query, userID, goalID, challengeID, namespace, delta, targetValue)
	if err != nil {
		return errors.ErrDatabaseError("increment progress (daily) in transaction", err)
	}

	return nil
}

// BatchIncrementProgress performs batch atomic increment within a transaction.
func (r *PostgresTxRepository) BatchIncrementProgress(ctx context.Context, increments []ProgressIncrement) error {
	if len(increments) == 0 {
		return nil
	}

	// Build arrays for UNNEST
	userIDs := make([]string, len(increments))
	goalIDs := make([]string, len(increments))
	challengeIDs := make([]string, len(increments))
	namespaces := make([]string, len(increments))
	deltas := make([]int, len(increments))
	targetValues := make([]int, len(increments))
	isDailyFlags := make([]bool, len(increments))

	for i, inc := range increments {
		userIDs[i] = inc.UserID
		goalIDs[i] = inc.GoalID
		challengeIDs[i] = inc.ChallengeID
		namespaces[i] = inc.Namespace
		deltas[i] = inc.Delta
		targetValues[i] = inc.TargetValue
		isDailyFlags[i] = inc.IsDailyIncrement
	}

	query := `
		INSERT INTO user_goal_progress (
			user_id,
			goal_id,
			challenge_id,
			namespace,
			progress,
			status,
			completed_at,
			updated_at
		)
		SELECT
			t.user_id,
			t.goal_id,
			t.challenge_id,
			t.namespace,
			t.delta,
			initial.status,
			initial.completed_at,
			NOW()
		FROM UNNEST(
			$1::VARCHAR(100)[],
			$2::VARCHAR(100)[],
			$3::VARCHAR(100)[],
			$4::VARCHAR(100)[],
			$5::INT[],
			$6::INT[],
			$7::BOOLEAN[]
		) AS t(user_id, goal_id, challenge_id, namespace, delta, target_value, is_daily)
		CROSS JOIN LATERAL (
			SELECT
				CASE WHEN t.delta >= t.target_value THEN 'completed' ELSE 'in_progress' END as status,
				CASE WHEN t.delta >= t.target_value THEN NOW() ELSE NULL END as completed_at
		) AS initial
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			progress = CASE
				WHEN (SELECT is_daily FROM UNNEST($7::BOOLEAN[], $2::VARCHAR(100)[]) AS u(is_daily, gid)
				      WHERE u.gid = user_goal_progress.goal_id LIMIT 1) = true
				     AND DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC')
					THEN user_goal_progress.progress
				ELSE
					user_goal_progress.progress + (
						SELECT delta FROM UNNEST($5::INT[], $2::VARCHAR(100)[]) AS u(delta, gid)
						WHERE u.gid = user_goal_progress.goal_id LIMIT 1
					)
			END,
			status = CASE
				WHEN (SELECT is_daily FROM UNNEST($7::BOOLEAN[], $2::VARCHAR(100)[]) AS u(is_daily, gid)
				      WHERE u.gid = user_goal_progress.goal_id LIMIT 1) = true
				     AND DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC') THEN
					CASE WHEN user_goal_progress.progress >= (
						SELECT target_value FROM UNNEST($6::INT[], $2::VARCHAR(100)[]) AS u(target_value, gid)
						WHERE u.gid = user_goal_progress.goal_id LIMIT 1
					) THEN 'completed' ELSE 'in_progress' END
				ELSE
					CASE WHEN user_goal_progress.progress + (
						SELECT delta FROM UNNEST($5::INT[], $2::VARCHAR(100)[]) AS u(delta, gid)
						WHERE u.gid = user_goal_progress.goal_id LIMIT 1
					) >= (
						SELECT target_value FROM UNNEST($6::INT[], $2::VARCHAR(100)[]) AS u(target_value, gid)
						WHERE u.gid = user_goal_progress.goal_id LIMIT 1
					) THEN 'completed' ELSE 'in_progress' END
			END,
			completed_at = CASE
				WHEN (SELECT is_daily FROM UNNEST($7::BOOLEAN[], $2::VARCHAR(100)[]) AS u(is_daily, gid)
				      WHERE u.gid = user_goal_progress.goal_id LIMIT 1) = true
				     AND DATE(user_goal_progress.updated_at AT TIME ZONE 'UTC') = DATE(NOW() AT TIME ZONE 'UTC') THEN
					user_goal_progress.completed_at
				WHEN user_goal_progress.progress + (
					SELECT delta FROM UNNEST($5::INT[], $2::VARCHAR(100)[]) AS u(delta, gid)
					WHERE u.gid = user_goal_progress.goal_id LIMIT 1
				) >= (
					SELECT target_value FROM UNNEST($6::INT[], $2::VARCHAR(100)[]) AS u(target_value, gid)
					WHERE u.gid = user_goal_progress.goal_id LIMIT 1
				) AND user_goal_progress.completed_at IS NULL THEN
					NOW()
				ELSE
					user_goal_progress.completed_at
			END,
			updated_at = NOW()
		WHERE user_goal_progress.status != 'claimed'
	`

	_, err := r.tx.ExecContext(ctx, query,
		pq.Array(userIDs),
		pq.Array(goalIDs),
		pq.Array(challengeIDs),
		pq.Array(namespaces),
		pq.Array(deltas),
		pq.Array(targetValues),
		pq.Array(isDailyFlags),
	)

	if err != nil {
		return errors.ErrDatabaseError("batch increment progress in transaction", err)
	}

	return nil
}

// MarkAsClaimed marks a goal as claimed within a transaction.
func (r *PostgresTxRepository) MarkAsClaimed(ctx context.Context, userID, goalID string) error {
	query := `
		UPDATE user_goal_progress
		SET status = 'claimed',
			claimed_at = NOW(),
			updated_at = NOW()
		WHERE user_id = $1 AND goal_id = $2
		AND status = 'completed'
		AND claimed_at IS NULL
	`

	result, err := r.tx.ExecContext(ctx, query, userID, goalID)
	if err != nil {
		return errors.ErrDatabaseError("mark as claimed in transaction", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.ErrDatabaseError("check rows affected", err)
	}

	if rowsAffected == 0 {
		return errors.ErrGoalNotCompleted(goalID)
	}

	return nil
}

// M3: Goal assignment control methods

// GetGoalsByIDs retrieves goal progress records within a transaction.
func (r *PostgresTxRepository) GetGoalsByIDs(ctx context.Context, userID string, goalIDs []string) ([]*domain.UserGoalProgress, error) {
	if len(goalIDs) == 0 {
		return []*domain.UserGoalProgress{}, nil
	}

	query := `
		SELECT user_id, goal_id, challenge_id, namespace, progress, status,
		       completed_at, claimed_at, created_at, updated_at,
		       is_active, assigned_at, expires_at
		FROM user_goal_progress
		WHERE user_id = $1 AND goal_id = ANY($2)
		ORDER BY created_at ASC
	`

	rows, err := r.tx.QueryContext(ctx, query, userID, pq.Array(goalIDs))
	if err != nil {
		return nil, errors.ErrDatabaseError("get goals by IDs in transaction", err)
	}
	defer func() { _ = rows.Close() }()

	return r.parent.scanProgressRows(rows)
}

// BulkInsert creates multiple goal progress records within a transaction.
func (r *PostgresTxRepository) BulkInsert(ctx context.Context, progresses []*domain.UserGoalProgress) error {
	if len(progresses) == 0 {
		return nil
	}

	// Build values for bulk insert
	valueStrings := make([]string, 0, len(progresses))
	valueArgs := make([]interface{}, 0, len(progresses)*13)

	for i, p := range progresses {
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, NOW(), NOW(), $%d, $%d, $%d)",
			i*13+1, i*13+2, i*13+3, i*13+4, i*13+5, i*13+6, i*13+7, i*13+8, i*13+9, i*13+10, i*13+11,
		))

		valueArgs = append(valueArgs,
			p.UserID,
			p.GoalID,
			p.ChallengeID,
			p.Namespace,
			p.Progress,
			p.Status,
			p.CompletedAt,
			p.ClaimedAt,
			p.IsActive,
			p.AssignedAt,
			p.ExpiresAt,
		)
	}

	//nolint:gosec // Safe: valueStrings contains only parameterized placeholders like "($1, $2, $3)", not user input
	query := fmt.Sprintf(`
		INSERT INTO user_goal_progress (
			user_id, goal_id, challenge_id, namespace,
			progress, status, completed_at, claimed_at,
			created_at, updated_at,
			is_active, assigned_at, expires_at
		) VALUES %s
		ON CONFLICT (user_id, goal_id) DO NOTHING
	`, strings.Join(valueStrings, ","))

	_, err := r.tx.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return errors.ErrDatabaseError("bulk insert goals in transaction", err)
	}

	return nil
}

// UpsertGoalActive creates or updates a goal's is_active status within a transaction.
func (r *PostgresTxRepository) UpsertGoalActive(ctx context.Context, progress *domain.UserGoalProgress) error {
	query := `
		INSERT INTO user_goal_progress (
			user_id, goal_id, challenge_id, namespace,
			progress, status, is_active, assigned_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
		)
		ON CONFLICT (user_id, goal_id) DO UPDATE SET
			is_active = EXCLUDED.is_active,
			assigned_at = CASE
				WHEN EXCLUDED.is_active = true THEN NOW()
				ELSE user_goal_progress.assigned_at
			END,
			updated_at = NOW()
	`

	_, err := r.tx.ExecContext(ctx, query,
		progress.UserID,
		progress.GoalID,
		progress.ChallengeID,
		progress.Namespace,
		progress.Progress,
		progress.Status,
		progress.IsActive,
		progress.AssignedAt,
	)

	if err != nil {
		return errors.ErrDatabaseError("upsert goal active in transaction", err)
	}

	return nil
}

// BeginTx is not supported within a transaction.
func (r *PostgresTxRepository) BeginTx(ctx context.Context) (TxRepository, error) {
	return nil, fmt.Errorf("cannot begin nested transaction")
}

// Commit commits the transaction.
func (r *PostgresTxRepository) Commit() error {
	err := r.tx.Commit()
	if err != nil {
		return errors.ErrDatabaseError("commit transaction", err)
	}
	return nil
}

// Rollback rolls back the transaction.
func (r *PostgresTxRepository) Rollback() error {
	err := r.tx.Rollback()
	if err != nil {
		return errors.ErrDatabaseError("rollback transaction", err)
	}
	return nil
}

// ConfigureDB configures database connection pool settings.
func ConfigureDB(db *sql.DB) {
	// Maximum open connections (includes idle + in-use)
	db.SetMaxOpenConns(50)

	// Maximum idle connections in pool
	db.SetMaxIdleConns(10)

	// Maximum lifetime of connection
	db.SetConnMaxLifetime(30 * time.Minute)

	// Maximum idle time for connection
	db.SetConnMaxIdleTime(5 * time.Minute)
}
