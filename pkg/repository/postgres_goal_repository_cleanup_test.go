package repository

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestDeleteExpiredRows verifies expired row cleanup with various scenarios.
func TestDeleteExpiredRows(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	retentionDays := 7
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)

	t.Run("deletes rows expired past cutoff", func(t *testing.T) {
		// Insert row expired 10 days ago (should be deleted)
		_, err := db.Exec(`
			INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, expires_at)
			VALUES ('user-exp-1', 'goal-exp-1', 'ch1', 'ns', 0, 'not_started', $1)
		`, now.Add(-10*24*time.Hour))
		if err != nil {
			t.Fatalf("insert expired row: %v", err)
		}

		deleted, err := repo.DeleteExpiredRows(ctx, "ns", cutoff, 1000)
		if err != nil {
			t.Fatalf("DeleteExpiredRows: %v", err)
		}
		if deleted != 1 {
			t.Errorf("expected 1 deleted, got %d", deleted)
		}
	})

	t.Run("does not delete rows within retention window", func(t *testing.T) {
		// Clean up first
		_, _ = db.Exec("TRUNCATE TABLE user_goal_progress")

		// Insert row expired 1 day ago (within 7-day retention, should NOT be deleted)
		_, err := db.Exec(`
			INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, expires_at)
			VALUES ('user-ret-1', 'goal-ret-1', 'ch1', 'ns', 0, 'not_started', $1)
		`, now.Add(-1*24*time.Hour))
		if err != nil {
			t.Fatalf("insert row: %v", err)
		}

		deleted, err := repo.DeleteExpiredRows(ctx, "ns", cutoff, 1000)
		if err != nil {
			t.Fatalf("DeleteExpiredRows: %v", err)
		}
		if deleted != 0 {
			t.Errorf("expected 0 deleted (within retention), got %d", deleted)
		}
	})

	t.Run("never deletes rows with NULL expires_at", func(t *testing.T) {
		_, _ = db.Exec("TRUNCATE TABLE user_goal_progress")

		// Insert row with NULL expires_at (non-rotating goal)
		_, err := db.Exec(`
			INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status)
			VALUES ('user-null-1', 'goal-null-1', 'ch1', 'ns', 0, 'not_started')
		`)
		if err != nil {
			t.Fatalf("insert row: %v", err)
		}

		// Use very aggressive cutoff that would catch everything if NULL wasn't excluded
		deleted, err := repo.DeleteExpiredRows(ctx, "ns", now.Add(24*time.Hour), 1000)
		if err != nil {
			t.Fatalf("DeleteExpiredRows: %v", err)
		}
		if deleted != 0 {
			t.Errorf("expected 0 deleted (NULL expires_at), got %d", deleted)
		}
	})

	t.Run("deletes all statuses equally", func(t *testing.T) {
		_, _ = db.Exec("TRUNCATE TABLE user_goal_progress")

		expired := now.Add(-10 * 24 * time.Hour)
		statuses := []string{"not_started", "in_progress", "completed", "claimed"}
		for i, status := range statuses {
			completedAt := "NULL"
			claimedAt := "NULL"
			if status == "completed" || status == "claimed" {
				completedAt = fmt.Sprintf("'%s'", expired.Format(time.RFC3339))
			}
			if status == "claimed" {
				claimedAt = fmt.Sprintf("'%s'", expired.Format(time.RFC3339))
			}

			//nolint:gosec // Safe: status values are hardcoded, not user input
			query := fmt.Sprintf(`
				INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, completed_at, claimed_at, expires_at)
				VALUES ('user-status-%d', 'goal-status-%d', 'ch1', 'ns', 0, '%s', %s, %s, $1)
			`, i, i, status, completedAt, claimedAt)
			_, err := db.Exec(query, expired)
			if err != nil {
				t.Fatalf("insert row with status %s: %v", status, err)
			}
		}

		deleted, err := repo.DeleteExpiredRows(ctx, "ns", cutoff, 1000)
		if err != nil {
			t.Fatalf("DeleteExpiredRows: %v", err)
		}
		if deleted != 4 {
			t.Errorf("expected 4 deleted (all statuses), got %d", deleted)
		}
	})

	t.Run("boundary: row exactly at cutoff is deleted", func(t *testing.T) {
		_, _ = db.Exec("TRUNCATE TABLE user_goal_progress")

		// Row expires exactly at cutoff minus 1ms (should be deleted since expires_at < cutoff)
		_, err := db.Exec(`
			INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, expires_at)
			VALUES ('user-bound-1', 'goal-bound-1', 'ch1', 'ns', 0, 'not_started', $1)
		`, cutoff.Add(-time.Millisecond))
		if err != nil {
			t.Fatalf("insert boundary row: %v", err)
		}

		// Row expires 1ms after cutoff (should NOT be deleted)
		_, err = db.Exec(`
			INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, expires_at)
			VALUES ('user-bound-2', 'goal-bound-2', 'ch1', 'ns', 0, 'not_started', $1)
		`, cutoff.Add(time.Millisecond))
		if err != nil {
			t.Fatalf("insert boundary row: %v", err)
		}

		deleted, err := repo.DeleteExpiredRows(ctx, "ns", cutoff, 1000)
		if err != nil {
			t.Fatalf("DeleteExpiredRows: %v", err)
		}
		if deleted != 1 {
			t.Errorf("expected 1 deleted (boundary), got %d", deleted)
		}
	})

	t.Run("boundary: row exactly at cutoff is NOT deleted", func(t *testing.T) {
		_, _ = db.Exec("TRUNCATE TABLE user_goal_progress")

		// Row with expires_at exactly at cutoff (SQL uses < $1, so this should NOT be deleted)
		_, err := db.Exec(`
			INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, expires_at)
			VALUES ('user-exact-1', 'goal-exact-1', 'ch1', 'ns', 0, 'not_started', $1)
		`, cutoff)
		if err != nil {
			t.Fatalf("insert exact-cutoff row: %v", err)
		}

		// Row with expires_at 1 microsecond before cutoff (should be deleted)
		_, err = db.Exec(`
			INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, expires_at)
			VALUES ('user-exact-2', 'goal-exact-2', 'ch1', 'ns', 0, 'not_started', $1)
		`, cutoff.Add(-time.Microsecond))
		if err != nil {
			t.Fatalf("insert before-cutoff row: %v", err)
		}

		deleted, err := repo.DeleteExpiredRows(ctx, "ns", cutoff, 1000)
		if err != nil {
			t.Fatalf("DeleteExpiredRows: %v", err)
		}
		if deleted != 1 {
			t.Errorf("expected 1 deleted (only the row before cutoff), got %d", deleted)
		}

		// Verify exact-cutoff row survives
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM user_goal_progress WHERE user_id = 'user-exact-1'").Scan(&count)
		if err != nil {
			t.Fatalf("count exact-cutoff row: %v", err)
		}
		if count != 1 {
			t.Errorf("expected exact-cutoff row to survive, got count=%d", count)
		}
	})

	t.Run("batch limit: respects batchSize", func(t *testing.T) {
		_, _ = db.Exec("TRUNCATE TABLE user_goal_progress")

		// Insert 2500 expired rows
		for i := range 2500 {
			_, err := db.Exec(`
				INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, expires_at)
				VALUES ($1, $2, 'ch1', 'ns', 0, 'not_started', $3)
			`, fmt.Sprintf("user-batch-%d", i), fmt.Sprintf("goal-batch-%d", i), now.Add(-10*24*time.Hour))
			if err != nil {
				t.Fatalf("insert batch row %d: %v", i, err)
			}
		}

		// First batch: should delete exactly 1000
		deleted, err := repo.DeleteExpiredRows(ctx, "ns", cutoff, 1000)
		if err != nil {
			t.Fatalf("DeleteExpiredRows batch 1: %v", err)
		}
		if deleted != 1000 {
			t.Errorf("expected 1000 deleted in first batch, got %d", deleted)
		}

		// Second batch: should delete another 1000
		deleted, err = repo.DeleteExpiredRows(ctx, "ns", cutoff, 1000)
		if err != nil {
			t.Fatalf("DeleteExpiredRows batch 2: %v", err)
		}
		if deleted != 1000 {
			t.Errorf("expected 1000 deleted in second batch, got %d", deleted)
		}

		// Third batch: should delete remaining 500
		deleted, err = repo.DeleteExpiredRows(ctx, "ns", cutoff, 1000)
		if err != nil {
			t.Fatalf("DeleteExpiredRows batch 3: %v", err)
		}
		if deleted != 500 {
			t.Errorf("expected 500 deleted in third batch, got %d", deleted)
		}
	})
}

// TestDeleteUserData verifies GDPR user data deletion.
func TestDeleteUserData(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("deletes all rows for target user", func(t *testing.T) {
		// Insert rows for user-A and user-B
		for i := range 3 {
			_, err := db.Exec(`
				INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status)
				VALUES ('user-A', $1, 'ch1', 'ns', 0, 'not_started')
			`, fmt.Sprintf("goal-A-%d", i))
			if err != nil {
				t.Fatalf("insert user-A row: %v", err)
			}
		}
		for i := range 2 {
			_, err := db.Exec(`
				INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status)
				VALUES ('user-B', $1, 'ch1', 'ns', 0, 'not_started')
			`, fmt.Sprintf("goal-B-%d", i))
			if err != nil {
				t.Fatalf("insert user-B row: %v", err)
			}
		}

		// Delete user-A data
		deleted, err := repo.DeleteUserData(ctx, "ns", "user-A")
		if err != nil {
			t.Fatalf("DeleteUserData: %v", err)
		}
		if deleted != 3 {
			t.Errorf("expected 3 deleted, got %d", deleted)
		}

		// Verify user-B rows still exist
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM user_goal_progress WHERE user_id = 'user-B'").Scan(&count)
		if err != nil {
			t.Fatalf("count user-B rows: %v", err)
		}
		if count != 2 {
			t.Errorf("expected 2 user-B rows remaining, got %d", count)
		}

		// Verify user-A rows are gone
		err = db.QueryRow("SELECT COUNT(*) FROM user_goal_progress WHERE user_id = 'user-A'").Scan(&count)
		if err != nil {
			t.Fatalf("count user-A rows: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0 user-A rows, got %d", count)
		}
	})

	t.Run("empty user returns 0 and no error", func(t *testing.T) {
		deleted, err := repo.DeleteUserData(ctx, "ns", "nonexistent-user")
		if err != nil {
			t.Fatalf("DeleteUserData: %v", err)
		}
		if deleted != 0 {
			t.Errorf("expected 0 deleted, got %d", deleted)
		}
	})
}

// TestBatchUpsertProgressWithCOPY_SkipsExpiredRows verifies that the COPY-based upsert
// does not update rows that have already expired (expires_at in the past).
func TestBatchUpsertProgressWithCOPY_SkipsExpiredRows(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()
	pastTime := time.Now().Add(-24 * time.Hour)

	// Insert a row with expires_at in the past
	_, err := db.Exec(`
		INSERT INTO user_goal_progress (user_id, goal_id, challenge_id, namespace, progress, status, is_active, expires_at)
		VALUES ('user-expired', 'goal-expired', 'ch1', 'ns', 5, 'in_progress', true, $1)
	`, pastTime)
	if err != nil {
		t.Fatalf("insert expired row: %v", err)
	}

	// Try to update the expired row via BatchUpsertProgressWithCOPY
	targetVal := 10
	rows := []CopyRow{
		{
			UserID:       "user-expired",
			GoalID:       "goal-expired",
			ChallengeID:  "ch1",
			Namespace:    "ns",
			Progress:     &targetVal,
			ProgressMode: "absolute",
			TargetValue:  100,
		},
	}

	err = repo.BatchUpsertProgressWithCOPY(ctx, rows)
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	// Verify the row was NOT updated (progress should still be 5)
	var progress int
	err = db.QueryRow("SELECT progress FROM user_goal_progress WHERE user_id = 'user-expired' AND goal_id = 'goal-expired'").Scan(&progress)
	if err != nil {
		t.Fatalf("query progress: %v", err)
	}
	if progress != 5 {
		t.Errorf("expected progress=5 (unchanged), got %d — expired row should not be updated", progress)
	}
}
