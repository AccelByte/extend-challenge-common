package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	customerrors "github.com/AccelByte/extend-challenge-common/pkg/errors"

	_ "github.com/lib/pq"
)

// Note: These tests require a PostgreSQL database.
// Run with: docker run -d --name test-postgres -p 5432:5432 -e POSTGRES_PASSWORD=test postgres:15
// Or use docker-compose with a test database

const testDSN = "postgres://testuser:testpass@localhost:5433/testdb?sslmode=disable"

// setupTestDB creates a test database connection and applies schema.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", testDSN)
	if err != nil {
		t.Skipf("Skipping integration test: cannot connect to database: %v", err)
		return nil
	}

	// Check if database is available
	if err := db.Ping(); err != nil {
		t.Skipf("Skipping integration test: database not available: %v", err)
		return nil
	}

	// Create table (M3 schema with is_active, assigned_at, expires_at)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_goal_progress (
			user_id VARCHAR(100) NOT NULL,
			goal_id VARCHAR(100) NOT NULL,
			challenge_id VARCHAR(100) NOT NULL,
			namespace VARCHAR(100) NOT NULL,
			progress INT NOT NULL DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'not_started',
			completed_at TIMESTAMP NULL,
			claimed_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			is_active BOOLEAN NOT NULL DEFAULT true,
			assigned_at TIMESTAMP NULL,
			expires_at TIMESTAMP NULL,
			PRIMARY KEY (user_id, goal_id),
			CONSTRAINT check_status CHECK (status IN ('not_started', 'in_progress', 'completed', 'claimed')),
			CONSTRAINT check_progress_non_negative CHECK (progress >= 0),
			CONSTRAINT check_claimed_implies_completed CHECK (claimed_at IS NULL OR completed_at IS NOT NULL)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Create index
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_user_goal_progress_user_challenge
		ON user_goal_progress(user_id, challenge_id)
	`)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	return db
}

// cleanupTestDB cleans up the test database.
func cleanupTestDB(t *testing.T, db *sql.DB) {
	t.Helper()

	if db == nil {
		return
	}

	// Clean up data
	_, err := db.Exec("TRUNCATE TABLE user_goal_progress")
	if err != nil {
		t.Logf("Warning: failed to truncate table: %v", err)
	}

	_ = db.Close()
}

func TestPostgresGoalRepository_UpsertProgress(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("insert new progress", func(t *testing.T) {
		progress := &domain.UserGoalProgress{
			UserID:      "user1",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
		}

		err := repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		// Verify it was inserted
		retrieved, err := repo.GetProgress(ctx, "user1", "goal1")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected progress to be found")
		}

		if retrieved.Progress != 5 {
			t.Errorf("Progress = %d, want 5", retrieved.Progress)
		}

		if retrieved.Status != domain.GoalStatusInProgress {
			t.Errorf("Status = %s, want %s", retrieved.Status, domain.GoalStatusInProgress)
		}
	})

	t.Run("update existing progress", func(t *testing.T) {
		// Insert initial progress
		progress := &domain.UserGoalProgress{
			UserID:      "user2",
			GoalID:      "goal2",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
		}
		err := repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("Initial UpsertProgress failed: %v", err)
		}

		// Update progress
		progress.Progress = 10
		completedTime := time.Now()
		progress.Status = domain.GoalStatusCompleted
		progress.CompletedAt = &completedTime

		err = repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("Update UpsertProgress failed: %v", err)
		}

		// Verify it was updated
		retrieved, err := repo.GetProgress(ctx, "user2", "goal2")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if retrieved.Progress != 10 {
			t.Errorf("Progress = %d, want 10", retrieved.Progress)
		}

		if retrieved.Status != domain.GoalStatusCompleted {
			t.Errorf("Status = %s, want %s", retrieved.Status, domain.GoalStatusCompleted)
		}
	})

	t.Run("does not update claimed progress", func(t *testing.T) {
		// Insert and claim progress
		progress := &domain.UserGoalProgress{
			UserID:      "user3",
			GoalID:      "goal3",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusClaimed,
		}
		err := repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("Initial UpsertProgress failed: %v", err)
		}

		// Try to update claimed progress
		progress.Progress = 20
		progress.Status = domain.GoalStatusCompleted

		err = repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		// Verify it was NOT updated (status still claimed, progress still 10)
		retrieved, err := repo.GetProgress(ctx, "user3", "goal3")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if retrieved.Progress != 10 {
			t.Errorf("Progress = %d, want 10 (should not have been updated)", retrieved.Progress)
		}

		if retrieved.Status != domain.GoalStatusClaimed {
			t.Errorf("Status = %s, want %s", retrieved.Status, domain.GoalStatusClaimed)
		}
	})
}

func TestPostgresGoalRepository_BatchUpsertProgress(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("batch update multiple active progress records", func(t *testing.T) {
		// M3: BatchUpsertProgress now requires rows to exist (UPDATE-only for lazy materialization)
		// Create initial rows with is_active = true using UpsertProgress
		initial := []*domain.UserGoalProgress{
			{
				UserID:      "user1",
				GoalID:      "goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
			{
				UserID:      "user1",
				GoalID:      "goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
			{
				UserID:      "user2",
				GoalID:      "goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
		}
		for _, p := range initial {
			if err := repo.UpsertProgress(ctx, p); err != nil {
				t.Fatalf("UpsertProgress failed: %v", err)
			}
		}

		// Now update via BatchUpsertProgress
		updates := []*domain.UserGoalProgress{
			{
				UserID:      "user1",
				GoalID:      "goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    5,
				Status:      domain.GoalStatusInProgress,
			},
			{
				UserID:      "user1",
				GoalID:      "goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusCompleted,
			},
			{
				UserID:      "user2",
				GoalID:      "goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    3,
				Status:      domain.GoalStatusInProgress,
			},
		}

		err := repo.BatchUpsertProgress(ctx, updates)
		if err != nil {
			t.Fatalf("BatchUpsertProgress failed: %v", err)
		}

		// Verify all records were updated
		progress1, _ := repo.GetProgress(ctx, "user1", "goal1")
		if progress1 == nil || progress1.Progress != 5 {
			t.Error("user1/goal1 not updated correctly")
		}

		progress2, _ := repo.GetProgress(ctx, "user1", "goal2")
		if progress2 == nil || progress2.Progress != 10 {
			t.Error("user1/goal2 not updated correctly")
		}

		progress3, _ := repo.GetProgress(ctx, "user2", "goal1")
		if progress3 == nil || progress3.Progress != 3 {
			t.Error("user2/goal1 not updated correctly")
		}
	})

	t.Run("batch update existing active records only", func(t *testing.T) {
		// Insert initial active records
		initial := []*domain.UserGoalProgress{
			{
				UserID:      "user3",
				GoalID:      "goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    1,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
			{
				UserID:      "user3",
				GoalID:      "goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    2,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
		}
		for _, p := range initial {
			if err := repo.UpsertProgress(ctx, p); err != nil {
				t.Fatalf("UpsertProgress failed: %v", err)
			}
		}

		// Update records
		updates := []*domain.UserGoalProgress{
			{
				UserID:      "user3",
				GoalID:      "goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    5,
				Status:      domain.GoalStatusInProgress,
			},
			{
				UserID:      "user3",
				GoalID:      "goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusCompleted,
			},
		}
		err := repo.BatchUpsertProgress(ctx, updates)
		if err != nil {
			t.Fatalf("Update BatchUpsertProgress failed: %v", err)
		}

		// Verify updates
		progress1, _ := repo.GetProgress(ctx, "user3", "goal1")
		if progress1.Progress != 5 {
			t.Errorf("user3/goal1 progress = %d, want 5", progress1.Progress)
		}

		progress2, _ := repo.GetProgress(ctx, "user3", "goal2")
		if progress2.Progress != 10 {
			t.Errorf("user3/goal2 progress = %d, want 10", progress2.Progress)
		}
	})

	t.Run("skips updates for inactive goals (M3)", func(t *testing.T) {
		// Create inactive goal
		inactive := &domain.UserGoalProgress{
			UserID:      "user4",
			GoalID:      "inactive_goal",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    0,
			Status:      domain.GoalStatusInProgress,
			IsActive:    false, // Inactive goal
		}
		if err := repo.UpsertProgress(ctx, inactive); err != nil {
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		// Try to update inactive goal via BatchUpsertProgress
		updates := []*domain.UserGoalProgress{
			{
				UserID:      "user4",
				GoalID:      "inactive_goal",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    100, // Should NOT be applied
				Status:      domain.GoalStatusCompleted,
			},
		}
		err := repo.BatchUpsertProgress(ctx, updates)
		if err != nil {
			t.Fatalf("BatchUpsertProgress failed: %v", err)
		}

		// Verify inactive goal was NOT updated (progress should still be 0)
		progress, _ := repo.GetProgress(ctx, "user4", "inactive_goal")
		if progress.Progress != 0 {
			t.Errorf("inactive goal was updated (progress = %d), want 0 (no update)", progress.Progress)
		}
		if progress.Status != domain.GoalStatusInProgress {
			t.Errorf("inactive goal status changed to %s, want in_progress", progress.Status)
		}
	})

	t.Run("empty batch does nothing", func(t *testing.T) {
		err := repo.BatchUpsertProgress(ctx, []*domain.UserGoalProgress{})
		if err != nil {
			t.Fatalf("Empty BatchUpsertProgress should not error: %v", err)
		}
	})
}

func TestPostgresGoalRepository_BatchUpsertProgressWithCOPY(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("batch update existing progress records with COPY", func(t *testing.T) {
		// M3 Phase 9: BatchUpsertProgressWithCOPY is UPDATE-only (lazy materialization)
		// First create initial records using BulkInsert (simulates initialization)
		initial := []*domain.UserGoalProgress{
			{
				UserID:      "copy-user1",
				GoalID:      "copy-goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
			},
			{
				UserID:      "copy-user1",
				GoalID:      "copy-goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
			},
			{
				UserID:      "copy-user2",
				GoalID:      "copy-goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
			},
		}
		err := repo.BulkInsert(ctx, initial)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Now update records using BatchUpsertProgressWithCOPY
		updates := []CopyRow{
			{
				UserID:       "copy-user1",
				GoalID:       "copy-goal1",
				ChallengeID:  "challenge1",
				Namespace:    "test",
				Progress:     intPtr(5),
				ProgressMode: "absolute",
				TargetValue:  10,
			},
			{
				UserID:       "copy-user1",
				GoalID:       "copy-goal2",
				ChallengeID:  "challenge1",
				Namespace:    "test",
				Progress:     intPtr(10),
				ProgressMode: "absolute",
				TargetValue:  10,
			},
			{
				UserID:       "copy-user2",
				GoalID:       "copy-goal1",
				ChallengeID:  "challenge1",
				Namespace:    "test",
				Progress:     intPtr(3),
				ProgressMode: "absolute",
				TargetValue:  10,
			},
		}

		err = repo.BatchUpsertProgressWithCOPY(ctx, updates)
		if err != nil {
			t.Fatalf("BatchUpsertProgressWithCOPY failed: %v", err)
		}

		// Verify all records were updated
		progress1, _ := repo.GetProgress(ctx, "copy-user1", "copy-goal1")
		if progress1 == nil || progress1.Progress != 5 {
			t.Error("copy-user1/copy-goal1 not updated correctly")
		}

		progress2, _ := repo.GetProgress(ctx, "copy-user1", "copy-goal2")
		if progress2 == nil || progress2.Progress != 10 {
			t.Error("copy-user1/copy-goal2 not updated correctly")
		}

		progress3, _ := repo.GetProgress(ctx, "copy-user2", "copy-goal1")
		if progress3 == nil || progress3.Progress != 3 {
			t.Error("copy-user2/copy-goal1 not updated correctly")
		}
	})

	t.Run("batch update existing records with COPY - accumulated updates", func(t *testing.T) {
		// M3 Phase 9: First create records using BulkInsert (simulates initialization)
		initial := []*domain.UserGoalProgress{
			{
				UserID:      "copy-user3",
				GoalID:      "copy-goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    1,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
			{
				UserID:      "copy-user3",
				GoalID:      "copy-goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    2,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
		}
		err := repo.BulkInsert(ctx, initial)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Update records using COPY
		updates := []CopyRow{
			{
				UserID:       "copy-user3",
				GoalID:       "copy-goal1",
				ChallengeID:  "challenge1",
				Namespace:    "test",
				Progress:     intPtr(5),
				ProgressMode: "absolute",
				TargetValue:  10,
			},
			{
				UserID:       "copy-user3",
				GoalID:       "copy-goal2",
				ChallengeID:  "challenge1",
				Namespace:    "test",
				Progress:     intPtr(10),
				ProgressMode: "absolute",
				TargetValue:  10,
			},
		}
		err = repo.BatchUpsertProgressWithCOPY(ctx, updates)
		if err != nil {
			t.Fatalf("Update BatchUpsertProgressWithCOPY failed: %v", err)
		}

		// Verify updates
		progress1, _ := repo.GetProgress(ctx, "copy-user3", "copy-goal1")
		if progress1.Progress != 5 {
			t.Errorf("copy-user3/copy-goal1 progress = %d, want 5", progress1.Progress)
		}

		progress2, _ := repo.GetProgress(ctx, "copy-user3", "copy-goal2")
		if progress2.Progress != 10 {
			t.Errorf("copy-user3/copy-goal2 progress = %d, want 10", progress2.Progress)
		}
	})

	t.Run("empty batch does nothing with COPY", func(t *testing.T) {
		err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{})
		if err != nil {
			t.Fatalf("Empty BatchUpsertProgressWithCOPY should not error: %v", err)
		}
	})

	t.Run("does not update claimed goals with COPY", func(t *testing.T) {
		// M3 Phase 9: Insert a goal using BulkInsert and mark it as claimed
		completedAt := time.Now()
		initial := []*domain.UserGoalProgress{
			{
				UserID:      "copy-user4",
				GoalID:      "copy-goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusCompleted,
				CompletedAt: &completedAt,
				IsActive:    true,
			},
		}
		err := repo.BulkInsert(ctx, initial)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Mark as claimed
		err = repo.MarkAsClaimed(ctx, "copy-user4", "copy-goal1")
		if err != nil {
			t.Fatalf("MarkAsClaimed failed: %v", err)
		}

		// Try to update the claimed goal
		updates := []CopyRow{
			{
				UserID:       "copy-user4",
				GoalID:       "copy-goal1",
				ChallengeID:  "challenge1",
				Namespace:    "test",
				Progress:     intPtr(20), // Try to change progress
				ProgressMode: "absolute",
				TargetValue:  10,
			},
		}
		err = repo.BatchUpsertProgressWithCOPY(ctx, updates)
		if err != nil {
			t.Fatalf("BatchUpsertProgressWithCOPY failed: %v", err)
		}

		// Verify the claimed goal was NOT updated
		progress, _ := repo.GetProgress(ctx, "copy-user4", "copy-goal1")
		if progress.Progress != 10 {
			t.Errorf("Claimed goal was updated (progress = %d), should remain 10", progress.Progress)
		}
		if progress.Status != domain.GoalStatusClaimed {
			t.Errorf("Goal status = %s, want claimed", progress.Status)
		}
	})

	// M3 Phase 5: Assignment control tests
	t.Run("M3: event updates assigned goal (is_active = true)", func(t *testing.T) {
		// 1. Create goal with is_active = true
		now := time.Now()
		initial := &domain.UserGoalProgress{
			UserID:      "m3-user1",
			GoalID:      "m3-goal1",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
			AssignedAt:  &now,
		}
		err := repo.UpsertProgress(ctx, initial)
		if err != nil {
			t.Fatalf("Initial insert failed: %v", err)
		}

		// 2. Simulate event update (progress = 10)
		updates := []CopyRow{
			{
				UserID:       "m3-user1",
				GoalID:       "m3-goal1",
				ChallengeID:  "challenge1",
				Namespace:    "test",
				Progress:     intPtr(10),
				ProgressMode: "absolute",
				TargetValue:  10,
			},
		}
		err = repo.BatchUpsertProgressWithCOPY(ctx, updates)
		if err != nil {
			t.Fatalf("BatchUpsertProgressWithCOPY failed: %v", err)
		}

		// 3. Verify row was updated
		result, err := repo.GetProgress(ctx, "m3-user1", "m3-goal1")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}
		if result.Progress != 10 {
			t.Errorf("Progress = %d, want 10 (should be updated)", result.Progress)
		}
		if result.Status != domain.GoalStatusCompleted {
			t.Errorf("Status = %s, want completed", result.Status)
		}
	})

	t.Run("M3: event does NOT update unassigned goal (is_active = false)", func(t *testing.T) {
		// 1. Create goal with is_active = false
		now := time.Now()
		initial := &domain.UserGoalProgress{
			UserID:      "m3-user2",
			GoalID:      "m3-goal2",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
			IsActive:    false, // ← Unassigned
			AssignedAt:  &now,
		}
		err := repo.UpsertProgress(ctx, initial)
		if err != nil {
			t.Fatalf("Initial insert failed: %v", err)
		}

		// 2. Simulate event update (progress = 10)
		updates := []CopyRow{
			{
				UserID:       "m3-user2",
				GoalID:       "m3-goal2",
				ChallengeID:  "challenge1",
				Namespace:    "test",
				Progress:     intPtr(10),
				ProgressMode: "absolute",
				TargetValue:  10,
			},
		}
		err = repo.BatchUpsertProgressWithCOPY(ctx, updates)
		if err != nil {
			t.Fatalf("BatchUpsertProgressWithCOPY should not error: %v", err)
		}

		// 3. Verify row was NOT updated (progress still 5)
		result, err := repo.GetProgress(ctx, "m3-user2", "m3-goal2")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}
		if result.Progress != 5 {
			t.Errorf("Progress = %d, want 5 (should NOT be updated)", result.Progress)
		}
		if result.Status != domain.GoalStatusInProgress {
			t.Errorf("Status = %s, want in_progress", result.Status)
		}
	})

	t.Run("M3: activate, event updates, deactivate, event does NOT update", func(t *testing.T) {
		// 1. Create assigned goal
		now := time.Now()
		initial := &domain.UserGoalProgress{
			UserID:      "m3-user3",
			GoalID:      "m3-goal3",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    0,
			Status:      domain.GoalStatusNotStarted,
			IsActive:    true,
			AssignedAt:  &now,
		}
		err := repo.UpsertProgress(ctx, initial)
		if err != nil {
			t.Fatalf("Initial insert failed: %v", err)
		}

		// 2. Event updates assigned goal
		updates1 := []CopyRow{
			{
				UserID:       "m3-user3",
				GoalID:       "m3-goal3",
				ChallengeID:  "challenge1",
				Namespace:    "test",
				Progress:     intPtr(5),
				ProgressMode: "absolute",
				TargetValue:  10,
			},
		}
		err = repo.BatchUpsertProgressWithCOPY(ctx, updates1)
		if err != nil {
			t.Fatalf("First update failed: %v", err)
		}

		// Verify update worked
		result, _ := repo.GetProgress(ctx, "m3-user3", "m3-goal3")
		if result.Progress != 5 {
			t.Errorf("After first update: progress = %d, want 5", result.Progress)
		}

		// 3. Deactivate goal
		err = repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:      "m3-user3",
			GoalID:      "m3-goal3",
			ChallengeID: "challenge1",
			Namespace:   "test",
			IsActive:    false,
		})
		if err != nil {
			t.Fatalf("Deactivation failed: %v", err)
		}

		// 4. Event should NOT update unassigned goal
		updates2 := []CopyRow{
			{
				UserID:       "m3-user3",
				GoalID:       "m3-goal3",
				ChallengeID:  "challenge1",
				Namespace:    "test",
				Progress:     intPtr(10),
				ProgressMode: "absolute",
				TargetValue:  10,
			},
		}
		err = repo.BatchUpsertProgressWithCOPY(ctx, updates2)
		if err != nil {
			t.Fatalf("Second update failed: %v", err)
		}

		// Verify update was blocked
		result, _ = repo.GetProgress(ctx, "m3-user3", "m3-goal3")
		if result.Progress != 5 {
			t.Errorf("After deactivation: progress = %d, want 5 (should NOT be updated)", result.Progress)
		}
		if result.Status != domain.GoalStatusInProgress {
			t.Errorf("After deactivation: status = %s, want in_progress", result.Status)
		}
	})
}

func TestPostgresGoalRepository_GetMethods(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	// Insert test data
	testData := []*domain.UserGoalProgress{
		{
			UserID:      "user1",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
		},
		{
			UserID:      "user1",
			GoalID:      "goal2",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusCompleted,
		},
		{
			UserID:      "user1",
			GoalID:      "goal3",
			ChallengeID: "challenge2",
			Namespace:   "test",
			Progress:    3,
			Status:      domain.GoalStatusInProgress,
		},
		{
			UserID:      "user2",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    7,
			Status:      domain.GoalStatusInProgress,
		},
	}
	err := repo.BatchUpsertProgress(ctx, testData)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	t.Run("GetProgress returns nil for non-existent progress", func(t *testing.T) {
		progress, err := repo.GetProgress(ctx, "nonexistent", "goal1")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}
		if progress != nil {
			t.Error("Expected nil for non-existent progress")
		}
	})

	t.Run("GetUserProgress returns all user's progress", func(t *testing.T) {
		progress, err := repo.GetUserProgress(ctx, "user1", false) // activeOnly = false
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(progress) != 3 {
			t.Errorf("Got %d progress records, want 3", len(progress))
		}
	})

	t.Run("GetChallengeProgress returns progress for specific challenge", func(t *testing.T) {
		progress, err := repo.GetChallengeProgress(ctx, "user1", "challenge1", false) // activeOnly = false
		if err != nil {
			t.Fatalf("GetChallengeProgress failed: %v", err)
		}

		if len(progress) != 2 {
			t.Errorf("Got %d progress records, want 2", len(progress))
		}

		// Verify both records belong to challenge1
		for _, p := range progress {
			if p.ChallengeID != "challenge1" {
				t.Errorf("Got challenge_id %s, want challenge1", p.ChallengeID)
			}
		}
	})

	t.Run("GetChallengeProgress returns empty for user with no progress", func(t *testing.T) {
		progress, err := repo.GetChallengeProgress(ctx, "nonexistent", "challenge1", false) // activeOnly = false
		if err != nil {
			t.Fatalf("GetChallengeProgress failed: %v", err)
		}

		if len(progress) != 0 {
			t.Errorf("Got %d progress records, want 0", len(progress))
		}
	})
}

func TestPostgresGoalRepository_MarkAsClaimed(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("marks completed goal as claimed", func(t *testing.T) {
		// Insert completed progress
		completedTime := time.Now()
		progress := &domain.UserGoalProgress{
			UserID:      "user1",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusCompleted,
			CompletedAt: &completedTime,
		}
		err := repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		// Mark as claimed
		err = repo.MarkAsClaimed(ctx, "user1", "goal1")
		if err != nil {
			t.Fatalf("MarkAsClaimed failed: %v", err)
		}

		// Verify status is claimed
		retrieved, err := repo.GetProgress(ctx, "user1", "goal1")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if retrieved.Status != domain.GoalStatusClaimed {
			t.Errorf("Status = %s, want %s", retrieved.Status, domain.GoalStatusClaimed)
		}

		if retrieved.ClaimedAt == nil {
			t.Error("ClaimedAt should not be nil")
		}
	})

	t.Run("fails to mark in_progress goal as claimed", func(t *testing.T) {
		// Insert in-progress
		progress := &domain.UserGoalProgress{
			UserID:      "user2",
			GoalID:      "goal2",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
		}
		err := repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		// Try to mark as claimed
		err = repo.MarkAsClaimed(ctx, "user2", "goal2")
		if err == nil {
			t.Error("Expected error when marking in_progress goal as claimed")
		}
	})

	t.Run("idempotent - marking already claimed goal returns error", func(t *testing.T) {
		// Insert completed progress
		completedTime := time.Now()
		progress := &domain.UserGoalProgress{
			UserID:      "user3",
			GoalID:      "goal3",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusCompleted,
			CompletedAt: &completedTime,
		}
		err := repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		// Mark as claimed first time
		err = repo.MarkAsClaimed(ctx, "user3", "goal3")
		if err != nil {
			t.Fatalf("First MarkAsClaimed failed: %v", err)
		}

		// Try to mark as claimed again
		err = repo.MarkAsClaimed(ctx, "user3", "goal3")
		if err == nil {
			t.Error("Expected error when marking already claimed goal")
		}
	})

	t.Run("fails to mark non-existent goal as claimed", func(t *testing.T) {
		// Try to mark non-existent goal as claimed
		err := repo.MarkAsClaimed(ctx, "nonexistent-user", "nonexistent-goal")

		if err == nil {
			t.Error("Expected error when marking non-existent goal as claimed")
		}

		// Verify it's the correct error type
		var challengeErr *customerrors.ChallengeError
		if errors.As(err, &challengeErr) {
			if challengeErr.Code != customerrors.ErrCodeGoalNotCompleted {
				t.Errorf("Expected ErrCodeGoalNotCompleted, got %s", challengeErr.Code)
			}
		} else {
			t.Error("Expected ChallengeError type")
		}
	})
}

func TestPostgresGoalRepository_Transaction(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("commit transaction persists changes", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		progress := &domain.UserGoalProgress{
			UserID:      "user1",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
		}

		err = tx.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("UpsertProgress in tx failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify changes persisted
		retrieved, err := repo.GetProgress(ctx, "user1", "goal1")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if retrieved == nil {
			t.Fatal("Expected progress to be persisted after commit")
		}
	})

	t.Run("rollback transaction discards changes", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		progress := &domain.UserGoalProgress{
			UserID:      "user2",
			GoalID:      "goal2",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
		}

		err = tx.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("UpsertProgress in tx failed: %v", err)
		}

		err = tx.Rollback()
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify changes were discarded
		retrieved, err := repo.GetProgress(ctx, "user2", "goal2")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if retrieved != nil {
			t.Error("Expected progress to be discarded after rollback")
		}
	})

	t.Run("GetProgressForUpdate locks row", func(t *testing.T) {
		// Insert test data
		progress := &domain.UserGoalProgress{
			UserID:      "user3",
			GoalID:      "goal3",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusCompleted,
		}
		err := repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		// Start transaction and lock row
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		locked, err := tx.GetProgressForUpdate(ctx, "user3", "goal3")
		if err != nil {
			t.Fatalf("GetProgressForUpdate failed: %v", err)
		}

		if locked == nil {
			t.Fatal("Expected progress to be found")
		}

		if locked.Status != domain.GoalStatusCompleted {
			t.Errorf("Status = %s, want %s", locked.Status, domain.GoalStatusCompleted)
		}

		// Commit to release lock
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
	})

	t.Run("transaction methods work correctly", func(t *testing.T) {
		// Insert test data
		completedTime := time.Now()
		testData := []*domain.UserGoalProgress{
			{
				UserID:      "user4",
				GoalID:      "goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    5,
				Status:      domain.GoalStatusInProgress,
			},
			{
				UserID:      "user4",
				GoalID:      "goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusCompleted,
				CompletedAt: &completedTime,
			},
		}
		err := repo.BatchUpsertProgress(ctx, testData)
		if err != nil {
			t.Fatalf("BatchUpsertProgress failed: %v", err)
		}

		// Start transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Test GetProgress in transaction
		progress, err := tx.GetProgress(ctx, "user4", "goal1")
		if err != nil {
			t.Fatalf("GetProgress in tx failed: %v", err)
		}
		if progress == nil || progress.Progress != 5 {
			t.Error("GetProgress in tx did not return correct data")
		}

		// Test GetUserProgress in transaction
		userProgress, err := tx.GetUserProgress(ctx, "user4", false) // activeOnly = false
		if err != nil {
			t.Fatalf("GetUserProgress in tx failed: %v", err)
		}
		if len(userProgress) != 2 {
			t.Errorf("GetUserProgress in tx returned %d records, want 2", len(userProgress))
		}

		// Test GetChallengeProgress in transaction
		challengeProgress, err := tx.GetChallengeProgress(ctx, "user4", "challenge1", false) // activeOnly = false
		if err != nil {
			t.Fatalf("GetChallengeProgress in tx failed: %v", err)
		}
		if len(challengeProgress) != 2 {
			t.Errorf("GetChallengeProgress in tx returned %d records, want 2", len(challengeProgress))
		}

		// Test UpsertProgress in transaction
		newProgress := &domain.UserGoalProgress{
			UserID:      "user4",
			GoalID:      "goal3",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    3,
			Status:      domain.GoalStatusInProgress,
		}
		err = tx.UpsertProgress(ctx, newProgress)
		if err != nil {
			t.Fatalf("UpsertProgress in tx failed: %v", err)
		}

		// Test BatchUpsertProgress in transaction
		batchUpdates := []*domain.UserGoalProgress{
			{
				UserID:      "user4",
				GoalID:      "goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusCompleted,
			},
		}
		err = tx.BatchUpsertProgress(ctx, batchUpdates)
		if err != nil {
			t.Fatalf("BatchUpsertProgress in tx failed: %v", err)
		}

		// Test MarkAsClaimed in transaction
		err = tx.MarkAsClaimed(ctx, "user4", "goal2")
		if err != nil {
			t.Fatalf("MarkAsClaimed in tx failed: %v", err)
		}

		// Commit transaction
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify changes persisted
		claimed, err := repo.GetProgress(ctx, "user4", "goal2")
		if err != nil {
			t.Fatalf("GetProgress after commit failed: %v", err)
		}
		if claimed.Status != domain.GoalStatusClaimed {
			t.Errorf("Status after commit = %s, want claimed", claimed.Status)
		}
	})

	t.Run("nested transaction returns error", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		// Try to start nested transaction
		_, err = tx.BeginTx(ctx)
		if err == nil {
			t.Error("Expected error when starting nested transaction")
		}
	})
}

func TestConfigureDB(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	// Test ConfigureDB doesn't panic
	ConfigureDB(db)

	// Verify settings were applied
	maxOpen := db.Stats().MaxOpenConnections
	if maxOpen != 50 {
		t.Errorf("MaxOpenConnections = %d, want 50", maxOpen)
	}
}

// M3 Phase 4: Test activeOnly filtering

func TestPostgresGoalRepository_GetUserProgress_ActiveOnly(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	userID := "user-123"
	now := time.Now()

	// Create 3 goals: 2 active, 1 inactive
	goals := []*domain.UserGoalProgress{
		{
			UserID:      userID,
			GoalID:      "goal-1",
			ChallengeID: "challenge-1",
			Namespace:   "test-ns",
			Progress:    5,
			Status:      "in_progress",
			IsActive:    true,
			AssignedAt:  &now,
		},
		{
			UserID:      userID,
			GoalID:      "goal-2",
			ChallengeID: "challenge-1",
			Namespace:   "test-ns",
			Progress:    10,
			Status:      "completed",
			IsActive:    true,
			AssignedAt:  &now,
		},
		{
			UserID:      userID,
			GoalID:      "goal-3",
			ChallengeID: "challenge-2",
			Namespace:   "test-ns",
			Progress:    3,
			Status:      "in_progress",
			IsActive:    false, // Inactive goal
			AssignedAt:  &now,
		},
	}

	// Insert all goals
	for _, goal := range goals {
		err := repo.UpsertProgress(ctx, goal)
		if err != nil {
			t.Fatalf("Failed to insert goal: %v", err)
		}
	}

	// Test 1: activeOnly = false (should return all 3 goals)
	allGoals, err := repo.GetUserProgress(ctx, userID, false)
	if err != nil {
		t.Fatalf("GetUserProgress(activeOnly=false) failed: %v", err)
	}

	if len(allGoals) != 3 {
		t.Errorf("GetUserProgress(activeOnly=false) returned %d goals, want 3", len(allGoals))
	}

	// Test 2: activeOnly = true (should return only 2 active goals)
	activeGoals, err := repo.GetUserProgress(ctx, userID, true)
	if err != nil {
		t.Fatalf("GetUserProgress(activeOnly=true) failed: %v", err)
	}

	if len(activeGoals) != 2 {
		t.Errorf("GetUserProgress(activeOnly=true) returned %d goals, want 2", len(activeGoals))
	}

	// Verify all returned goals are active
	for _, goal := range activeGoals {
		if !goal.IsActive {
			t.Errorf("GetUserProgress(activeOnly=true) returned inactive goal: %s", goal.GoalID)
		}
	}

	// Verify the active goals are the correct ones
	activeGoalIDs := make(map[string]bool)
	for _, goal := range activeGoals {
		activeGoalIDs[goal.GoalID] = true
	}

	if !activeGoalIDs["goal-1"] || !activeGoalIDs["goal-2"] {
		t.Errorf("GetUserProgress(activeOnly=true) did not return expected goals")
	}
}

func TestPostgresGoalRepository_GetChallengeProgress_ActiveOnly(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	userID := "user-456"
	challengeID := "challenge-multi"
	now := time.Now()

	// Create 4 goals in same challenge: 3 active, 1 inactive
	goals := []*domain.UserGoalProgress{
		{
			UserID:      userID,
			GoalID:      "goal-a",
			ChallengeID: challengeID,
			Namespace:   "test-ns",
			Progress:    1,
			Status:      "in_progress",
			IsActive:    true,
			AssignedAt:  &now,
		},
		{
			UserID:      userID,
			GoalID:      "goal-b",
			ChallengeID: challengeID,
			Namespace:   "test-ns",
			Progress:    2,
			Status:      "in_progress",
			IsActive:    false, // Inactive
			AssignedAt:  &now,
		},
		{
			UserID:      userID,
			GoalID:      "goal-c",
			ChallengeID: challengeID,
			Namespace:   "test-ns",
			Progress:    3,
			Status:      "completed",
			IsActive:    true,
			AssignedAt:  &now,
		},
		{
			UserID:      userID,
			GoalID:      "goal-d",
			ChallengeID: challengeID,
			Namespace:   "test-ns",
			Progress:    4,
			Status:      "in_progress",
			IsActive:    true,
			AssignedAt:  &now,
		},
	}

	// Insert all goals
	for _, goal := range goals {
		err := repo.UpsertProgress(ctx, goal)
		if err != nil {
			t.Fatalf("Failed to insert goal: %v", err)
		}
	}

	// Test 1: activeOnly = false (should return all 4 goals)
	allGoals, err := repo.GetChallengeProgress(ctx, userID, challengeID, false)
	if err != nil {
		t.Fatalf("GetChallengeProgress(activeOnly=false) failed: %v", err)
	}

	if len(allGoals) != 4 {
		t.Errorf("GetChallengeProgress(activeOnly=false) returned %d goals, want 4", len(allGoals))
	}

	// Test 2: activeOnly = true (should return only 3 active goals)
	activeGoals, err := repo.GetChallengeProgress(ctx, userID, challengeID, true)
	if err != nil {
		t.Fatalf("GetChallengeProgress(activeOnly=true) failed: %v", err)
	}

	if len(activeGoals) != 3 {
		t.Errorf("GetChallengeProgress(activeOnly=true) returned %d goals, want 3", len(activeGoals))
	}

	// Verify all returned goals are active
	for _, goal := range activeGoals {
		if !goal.IsActive {
			t.Errorf("GetChallengeProgress(activeOnly=true) returned inactive goal: %s", goal.GoalID)
		}
	}

	// Verify the inactive goal is not returned
	for _, goal := range activeGoals {
		if goal.GoalID == "goal-b" {
			t.Errorf("GetChallengeProgress(activeOnly=true) should not return goal-b (inactive)")
		}
	}
}
func TestPostgresGoalRepository_GetGoalsByIDs(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("returns multiple goals by IDs", func(t *testing.T) {
		// Setup: Insert 5 goals for a user
		now := time.Now()
		goals := []*domain.UserGoalProgress{
			{
				UserID:      "user-multi-1",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "user-multi-1",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    20,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "user-multi-1",
				GoalID:      "goal-3",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    30,
				Status:      domain.GoalStatusCompleted,
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "user-multi-1",
				GoalID:      "goal-4",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    40,
				Status:      domain.GoalStatusInProgress,
				IsActive:    false,
				AssignedAt:  &now,
			},
			{
				UserID:      "user-multi-1",
				GoalID:      "goal-5",
				ChallengeID: "challenge-2",
				Namespace:   "test",
				Progress:    50,
				Status:      domain.GoalStatusCompleted,
				IsActive:    true,
				AssignedAt:  &now,
			},
		}

		for _, g := range goals {
			err := repo.UpsertProgress(ctx, g)
			if err != nil {
				t.Fatalf("Setup failed: %v", err)
			}
		}

		// Execute: GetGoalsByIDs([goal-1, goal-3, goal-5])
		result, err := repo.GetGoalsByIDs(ctx, "user-multi-1", []string{"goal-1", "goal-3", "goal-5"})
		if err != nil {
			t.Fatalf("GetGoalsByIDs failed: %v", err)
		}

		// Verify: Returns exactly 3 goals
		if len(result) != 3 {
			t.Errorf("Expected 3 goals, got %d", len(result))
		}

		// Verify goal IDs
		goalIDs := make(map[string]bool)
		for _, g := range result {
			goalIDs[g.GoalID] = true
		}

		if !goalIDs["goal-1"] || !goalIDs["goal-3"] || !goalIDs["goal-5"] {
			t.Errorf("Expected goal-1, goal-3, goal-5, got %v", goalIDs)
		}

		// Verify progress values match
		progressMap := make(map[string]int)
		for _, g := range result {
			progressMap[g.GoalID] = g.Progress
		}

		if progressMap["goal-1"] != 10 {
			t.Errorf("goal-1 progress = %d, want 10", progressMap["goal-1"])
		}
		if progressMap["goal-3"] != 30 {
			t.Errorf("goal-3 progress = %d, want 30", progressMap["goal-3"])
		}
		if progressMap["goal-5"] != 50 {
			t.Errorf("goal-5 progress = %d, want 50", progressMap["goal-5"])
		}
	})

	t.Run("returns empty slice when no goals found", func(t *testing.T) {
		// Execute: GetGoalsByIDs for non-existent goals
		result, err := repo.GetGoalsByIDs(ctx, "user-multi-1", []string{"non-existent-1", "non-existent-2"})
		if err != nil {
			t.Fatalf("GetGoalsByIDs failed: %v", err)
		}

		// Verify: Returns empty slice
		if len(result) != 0 {
			t.Errorf("Expected empty slice, got %d goals", len(result))
		}
	})

	t.Run("empty goalIDs slice returns empty slice", func(t *testing.T) {
		// Execute: GetGoalsByIDs with empty array
		result, err := repo.GetGoalsByIDs(ctx, "user-multi-1", []string{})
		if err != nil {
			t.Fatalf("GetGoalsByIDs failed: %v", err)
		}

		// Verify: Returns empty slice immediately (no DB query)
		if result == nil {
			t.Error("Expected non-nil slice, got nil")
		}
		if len(result) != 0 {
			t.Errorf("Expected empty slice, got %d goals", len(result))
		}
	})

	t.Run("filters by user correctly", func(t *testing.T) {
		// Setup: Insert goal for different user
		now := time.Now()
		err := repo.UpsertProgress(ctx, &domain.UserGoalProgress{
			UserID:      "user-multi-2",
			GoalID:      "goal-1",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			Progress:    99,
			Status:      domain.GoalStatusCompleted,
			IsActive:    true,
			AssignedAt:  &now,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Execute: GetGoalsByIDs for user-multi-1
		result, err := repo.GetGoalsByIDs(ctx, "user-multi-1", []string{"goal-1"})
		if err != nil {
			t.Fatalf("GetGoalsByIDs failed: %v", err)
		}

		// Verify: Returns only user-multi-1's goal (progress=10, not 99)
		if len(result) != 1 {
			t.Fatalf("Expected 1 goal, got %d", len(result))
		}
		if result[0].Progress != 10 {
			t.Errorf("Expected progress=10 for user-multi-1, got %d (user-multi-2's progress)", result[0].Progress)
		}
	})

	t.Run("includes all M3 fields", func(t *testing.T) {
		// Execute: GetGoalsByIDs
		result, err := repo.GetGoalsByIDs(ctx, "user-multi-1", []string{"goal-1"})
		if err != nil {
			t.Fatalf("GetGoalsByIDs failed: %v", err)
		}

		// Verify: M3 fields are populated
		if len(result) != 1 {
			t.Fatalf("Expected 1 goal, got %d", len(result))
		}

		g := result[0]
		if !g.IsActive {
			t.Error("Expected is_active=true")
		}
		if g.AssignedAt == nil {
			t.Error("Expected assigned_at to be non-nil")
		}
	})
}

func TestPostgresGoalRepository_BulkInsert(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("inserts multiple goals in single query", func(t *testing.T) {
		// Setup: Prepare 100 UserGoalProgress records
		now := time.Now()
		progresses := make([]*domain.UserGoalProgress, 100)
		for i := 0; i < 100; i++ {
			progresses[i] = &domain.UserGoalProgress{
				UserID:      "bulk-user-1",
				GoalID:      fmt.Sprintf("bulk-goal-%d", i),
				ChallengeID: "bulk-challenge-1",
				Namespace:   "test",
				Progress:    i * 10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		// Execute: BulkInsert(100 records)
		start := time.Now()
		err := repo.BulkInsert(ctx, progresses)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Verify: All 100 records exist in DB
		result, err := repo.GetUserProgress(ctx, "bulk-user-1", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(result) != 100 {
			t.Errorf("Expected 100 records, got %d", len(result))
		}

		// Verify: Performance < 100ms
		if elapsed > 100*time.Millisecond {
			t.Logf("Warning: BulkInsert(100) took %v (expected < 100ms)", elapsed)
		}
	})

	t.Run("handles empty slice without error", func(t *testing.T) {
		// Execute: BulkInsert([])
		err := repo.BulkInsert(ctx, []*domain.UserGoalProgress{})

		// Verify: No error
		if err != nil {
			t.Errorf("Expected no error for empty slice, got %v", err)
		}
	})

	t.Run("ON CONFLICT DO NOTHING preserves existing data", func(t *testing.T) {
		// Setup: Insert goal with progress=100
		now := time.Now()
		err := repo.UpsertProgress(ctx, &domain.UserGoalProgress{
			UserID:      "bulk-user-2",
			GoalID:      "conflict-goal",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			Progress:    100,
			Status:      domain.GoalStatusCompleted,
			IsActive:    true,
			AssignedAt:  &now,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Execute: BulkInsert with same user_id+goal_id but progress=200
		err = repo.BulkInsert(ctx, []*domain.UserGoalProgress{
			{
				UserID:      "bulk-user-2",
				GoalID:      "conflict-goal",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    200,
				Status:      domain.GoalStatusInProgress,
				IsActive:    false,
			},
		})
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Verify: Progress remains 100 (not updated)
		result, err := repo.GetProgress(ctx, "bulk-user-2", "conflict-goal")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if result.Progress != 100 {
			t.Errorf("Expected progress=100 (preserved), got %d (updated)", result.Progress)
		}
		if result.Status != domain.GoalStatusCompleted {
			t.Errorf("Expected status=completed (preserved), got %s", result.Status)
		}
		if !result.IsActive {
			t.Error("Expected is_active=true (preserved), got false")
		}
	})

	t.Run("handles large batches efficiently", func(t *testing.T) {
		// Setup: Prepare 1000 records
		now := time.Now()
		progresses := make([]*domain.UserGoalProgress, 1000)
		for i := 0; i < 1000; i++ {
			progresses[i] = &domain.UserGoalProgress{
				UserID:      "bulk-user-3",
				GoalID:      fmt.Sprintf("large-goal-%d", i),
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    i,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		// Execute: BulkInsert(1000 records)
		start := time.Now()
		err := repo.BulkInsert(ctx, progresses)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Verify: All records inserted
		result, err := repo.GetUserProgress(ctx, "bulk-user-3", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(result) != 1000 {
			t.Errorf("Expected 1000 records, got %d", len(result))
		}

		// Verify: Performance < 150ms (generous limit for 1000 rows)
		if elapsed > 150*time.Millisecond {
			t.Logf("Warning: BulkInsert(1000) took %v (expected < 150ms)", elapsed)
		}
	})

	t.Run("respects M3 fields (is_active, assigned_at, expires_at)", func(t *testing.T) {
		// Setup: Prepare records with various M3 field values
		now := time.Now()
		future := now.Add(24 * time.Hour)

		progresses := []*domain.UserGoalProgress{
			{
				UserID:      "bulk-user-4",
				GoalID:      "m3-goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
				ExpiresAt:   &future,
			},
			{
				UserID:      "bulk-user-4",
				GoalID:      "m3-goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    20,
				Status:      domain.GoalStatusInProgress,
				IsActive:    false,
				AssignedAt:  nil,
				ExpiresAt:   nil,
			},
		}

		// Execute: BulkInsert
		err := repo.BulkInsert(ctx, progresses)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Verify: M3 fields correctly stored
		result1, err := repo.GetProgress(ctx, "bulk-user-4", "m3-goal-1")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if !result1.IsActive {
			t.Error("Expected m3-goal-1 is_active=true")
		}
		if result1.AssignedAt == nil {
			t.Error("Expected m3-goal-1 assigned_at non-nil")
		}
		if result1.ExpiresAt == nil {
			t.Error("Expected m3-goal-1 expires_at non-nil")
		}

		result2, err := repo.GetProgress(ctx, "bulk-user-4", "m3-goal-2")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if result2.IsActive {
			t.Error("Expected m3-goal-2 is_active=false")
		}
		if result2.AssignedAt != nil {
			t.Error("Expected m3-goal-2 assigned_at nil")
		}
		if result2.ExpiresAt != nil {
			t.Error("Expected m3-goal-2 expires_at nil")
		}
	})

	t.Run("handles NULL timestamp fields correctly", func(t *testing.T) {
		// Setup: Record with NULL completed_at and claimed_at
		progresses := []*domain.UserGoalProgress{
			{
				UserID:      "bulk-user-5",
				GoalID:      "null-goal",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    5,
				Status:      domain.GoalStatusInProgress,
				CompletedAt: nil,
				ClaimedAt:   nil,
				IsActive:    true,
			},
		}

		// Execute: BulkInsert
		err := repo.BulkInsert(ctx, progresses)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Verify: NULL fields remain NULL
		result, err := repo.GetProgress(ctx, "bulk-user-5", "null-goal")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if result.CompletedAt != nil {
			t.Error("Expected completed_at=nil, got non-nil")
		}
		if result.ClaimedAt != nil {
			t.Error("Expected claimed_at=nil, got non-nil")
		}
	})
}
func TestPostgresGoalRepository_UpsertGoalActive_EdgeCases(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("INSERT path - creates new row when not exists", func(t *testing.T) {
		// Execute: UpsertGoalActive for non-existent user/goal
		err := repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:      "upsert-user-1",
			GoalID:      "upsert-goal-1",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			IsActive:    true,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		// Verify: Row created with defaults
		result, err := repo.GetProgress(ctx, "upsert-user-1", "upsert-goal-1")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if !result.IsActive {
			t.Error("Expected is_active=true")
		}
		if result.Status != domain.GoalStatusNotStarted {
			t.Errorf("Expected status=not_started, got %s", result.Status)
		}
		if result.Progress != 0 {
			t.Errorf("Expected progress=0, got %d", result.Progress)
		}
		if result.AssignedAt == nil {
			t.Error("Expected assigned_at to be set when is_active=true")
		}
	})

	t.Run("UPDATE path - updates existing row", func(t *testing.T) {
		// Setup: Insert goal with is_active=true, progress=50
		now := time.Now()
		err := repo.UpsertProgress(ctx, &domain.UserGoalProgress{
			UserID:      "upsert-user-2",
			GoalID:      "upsert-goal-2",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			Progress:    50,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
			AssignedAt:  &now,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Execute: UpsertGoalActive with is_active=false
		err = repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:   "upsert-user-2",
			GoalID:   "upsert-goal-2",
			IsActive: false,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		// Verify: is_active updated to false, progress/status unchanged
		result, err := repo.GetProgress(ctx, "upsert-user-2", "upsert-goal-2")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if result.IsActive {
			t.Error("Expected is_active=false after update")
		}
		if result.Progress != 50 {
			t.Errorf("Expected progress=50 (unchanged), got %d", result.Progress)
		}
		if result.Status != domain.GoalStatusInProgress {
			t.Errorf("Expected status=in_progress (unchanged), got %s", result.Status)
		}
	})

	t.Run("assigned_at set to NOW() when activating", func(t *testing.T) {
		// Setup: Create inactive goal
		err := repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:      "upsert-user-3",
			GoalID:      "upsert-goal-3",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			IsActive:    false,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Sleep to ensure timestamp difference
		time.Sleep(100 * time.Millisecond)
		beforeActivation := time.Now()

		// Execute: Activate goal
		err = repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:   "upsert-user-3",
			GoalID:   "upsert-goal-3",
			IsActive: true,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		afterActivation := time.Now()

		// Verify: assigned_at is set to recent timestamp
		result, err := repo.GetProgress(ctx, "upsert-user-3", "upsert-goal-3")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if result.AssignedAt == nil {
			t.Fatal("Expected assigned_at to be non-nil when activating")
		}

		// Verify assigned_at is between beforeActivation and afterActivation
		if result.AssignedAt.Before(beforeActivation) || result.AssignedAt.After(afterActivation) {
			t.Errorf("assigned_at=%v should be between %v and %v",
				result.AssignedAt, beforeActivation, afterActivation)
		}
	})

	t.Run("assigned_at unchanged when deactivating", func(t *testing.T) {
		// Setup: Create active goal with assigned_at = yesterday
		yesterday := time.Now().UTC().Add(-24 * time.Hour)
		err := repo.UpsertProgress(ctx, &domain.UserGoalProgress{
			UserID:      "upsert-user-4",
			GoalID:      "upsert-goal-4",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
			AssignedAt:  &yesterday,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Execute: Deactivate goal
		err = repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:   "upsert-user-4",
			GoalID:   "upsert-goal-4",
			IsActive: false,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		// Verify: assigned_at still = yesterday (unchanged)
		result, err := repo.GetProgress(ctx, "upsert-user-4", "upsert-goal-4")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if result.AssignedAt == nil {
			t.Fatal("Expected assigned_at to remain non-nil")
		}

		// Verify timestamp didn't change (allow 1 second tolerance for test execution)
		// Convert both to UTC for comparison to avoid timezone issues
		diff := result.AssignedAt.UTC().Sub(yesterday.UTC()).Abs()
		if diff > 1*time.Second {
			t.Errorf("assigned_at changed from %v to %v (diff=%v), expected unchanged",
				yesterday.UTC(), result.AssignedAt.UTC(), diff)
		}
	})

	t.Run("INSERT path - creates inactive goal without assigned_at", func(t *testing.T) {
		// Execute: UpsertGoalActive with is_active=false for new goal
		err := repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:      "upsert-user-5",
			GoalID:      "upsert-goal-5",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			IsActive:    false,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		// Verify: Row created with is_active=false, assigned_at=nil
		result, err := repo.GetProgress(ctx, "upsert-user-5", "upsert-goal-5")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if result.IsActive {
			t.Error("Expected is_active=false")
		}
		if result.AssignedAt != nil {
			t.Errorf("Expected assigned_at=nil when is_active=false, got %v", result.AssignedAt)
		}
	})

	t.Run("toggle active multiple times", func(t *testing.T) {
		// Setup: Create inactive goal
		err := repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:      "upsert-user-6",
			GoalID:      "upsert-goal-6",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			IsActive:    false,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Activate
		err = repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:   "upsert-user-6",
			GoalID:   "upsert-goal-6",
			IsActive: true,
		})
		if err != nil {
			t.Fatalf("Activation failed: %v", err)
		}

		result1, _ := repo.GetProgress(ctx, "upsert-user-6", "upsert-goal-6")
		firstAssignedAt := result1.AssignedAt

		// Deactivate
		err = repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:   "upsert-user-6",
			GoalID:   "upsert-goal-6",
			IsActive: false,
		})
		if err != nil {
			t.Fatalf("Deactivation failed: %v", err)
		}

		// Re-activate
		time.Sleep(100 * time.Millisecond) // Ensure timestamp difference
		err = repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:   "upsert-user-6",
			GoalID:   "upsert-goal-6",
			IsActive: true,
		})
		if err != nil {
			t.Fatalf("Re-activation failed: %v", err)
		}

		result2, _ := repo.GetProgress(ctx, "upsert-user-6", "upsert-goal-6")
		secondAssignedAt := result2.AssignedAt

		// Verify: Second activation updates assigned_at to newer timestamp
		if secondAssignedAt.Before(*firstAssignedAt) || secondAssignedAt.Equal(*firstAssignedAt) {
			t.Errorf("Expected second assigned_at (%v) > first assigned_at (%v)",
				secondAssignedAt, firstAssignedAt)
		}
	})
}

// TestPostgresGoalRepository_GetUserGoalCount tests the M3 Phase 9 fast path method
func TestPostgresGoalRepository_GetUserGoalCount(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("returns 0 for user with no goals", func(t *testing.T) {
		count, err := repo.GetUserGoalCount(ctx, "user-no-goals")
		if err != nil {
			t.Fatalf("GetUserGoalCount failed: %v", err)
		}

		if count != 0 {
			t.Errorf("Expected count=0, got %d", count)
		}
	})

	t.Run("returns correct count for user with goals", func(t *testing.T) {
		// Setup: Insert 5 goals for user
		goals := make([]*domain.UserGoalProgress, 5)
		for i := 0; i < 5; i++ {
			goals[i] = &domain.UserGoalProgress{
				UserID:      "user-with-goals",
				GoalID:      fmt.Sprintf("goal-%d", i),
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
			}
		}
		err := repo.BulkInsert(ctx, goals)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		count, err := repo.GetUserGoalCount(ctx, "user-with-goals")
		if err != nil {
			t.Fatalf("GetUserGoalCount failed: %v", err)
		}

		if count != 5 {
			t.Errorf("Expected count=5, got %d", count)
		}
	})

	t.Run("counts both active and inactive goals", func(t *testing.T) {
		// Setup: Insert 3 active and 2 inactive goals
		goals := []*domain.UserGoalProgress{
			{UserID: "user-mixed", GoalID: "goal-1", ChallengeID: "c1", Namespace: "test", IsActive: true, Status: domain.GoalStatusNotStarted},
			{UserID: "user-mixed", GoalID: "goal-2", ChallengeID: "c1", Namespace: "test", IsActive: true, Status: domain.GoalStatusNotStarted},
			{UserID: "user-mixed", GoalID: "goal-3", ChallengeID: "c1", Namespace: "test", IsActive: true, Status: domain.GoalStatusNotStarted},
			{UserID: "user-mixed", GoalID: "goal-4", ChallengeID: "c1", Namespace: "test", IsActive: false, Status: domain.GoalStatusNotStarted},
			{UserID: "user-mixed", GoalID: "goal-5", ChallengeID: "c1", Namespace: "test", IsActive: false, Status: domain.GoalStatusNotStarted},
		}
		err := repo.BulkInsert(ctx, goals)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		count, err := repo.GetUserGoalCount(ctx, "user-mixed")
		if err != nil {
			t.Fatalf("GetUserGoalCount failed: %v", err)
		}

		if count != 5 {
			t.Errorf("Expected count=5 (3 active + 2 inactive), got %d", count)
		}
	})

	t.Run("counts goals across multiple challenges", func(t *testing.T) {
		// Setup: Insert goals from different challenges
		goals := []*domain.UserGoalProgress{
			{UserID: "user-multi", GoalID: "goal-1", ChallengeID: "challenge-1", Namespace: "test", IsActive: true, Status: domain.GoalStatusNotStarted},
			{UserID: "user-multi", GoalID: "goal-2", ChallengeID: "challenge-2", Namespace: "test", IsActive: true, Status: domain.GoalStatusNotStarted},
			{UserID: "user-multi", GoalID: "goal-3", ChallengeID: "challenge-3", Namespace: "test", IsActive: true, Status: domain.GoalStatusNotStarted},
		}
		err := repo.BulkInsert(ctx, goals)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		count, err := repo.GetUserGoalCount(ctx, "user-multi")
		if err != nil {
			t.Fatalf("GetUserGoalCount failed: %v", err)
		}

		if count != 3 {
			t.Errorf("Expected count=3, got %d", count)
		}
	})
}

// TestPostgresGoalRepository_GetActiveGoals tests the M3 Phase 9 fast path method
func TestPostgresGoalRepository_GetActiveGoals(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("returns empty slice for user with no goals", func(t *testing.T) {
		goals, err := repo.GetActiveGoals(ctx, "user-no-active")
		if err != nil {
			t.Fatalf("GetActiveGoals failed: %v", err)
		}

		if len(goals) != 0 {
			t.Errorf("Expected 0 goals, got %d", len(goals))
		}
	})

	t.Run("returns only active goals", func(t *testing.T) {
		// Setup: Insert 3 active and 2 inactive goals
		allGoals := []*domain.UserGoalProgress{
			{UserID: "user-filter", GoalID: "active-1", ChallengeID: "c1", Namespace: "test", Progress: 10, Status: domain.GoalStatusInProgress, IsActive: true},
			{UserID: "user-filter", GoalID: "active-2", ChallengeID: "c1", Namespace: "test", Progress: 5, Status: domain.GoalStatusInProgress, IsActive: true},
			{UserID: "user-filter", GoalID: "active-3", ChallengeID: "c2", Namespace: "test", Progress: 0, Status: domain.GoalStatusNotStarted, IsActive: true},
			{UserID: "user-filter", GoalID: "inactive-1", ChallengeID: "c1", Namespace: "test", Progress: 20, Status: domain.GoalStatusInProgress, IsActive: false},
			{UserID: "user-filter", GoalID: "inactive-2", ChallengeID: "c2", Namespace: "test", Progress: 30, Status: domain.GoalStatusCompleted, IsActive: false},
		}
		err := repo.BulkInsert(ctx, allGoals)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Execute: Get only active goals
		activeGoals, err := repo.GetActiveGoals(ctx, "user-filter")
		if err != nil {
			t.Fatalf("GetActiveGoals failed: %v", err)
		}

		// Verify: Only 3 active goals returned
		if len(activeGoals) != 3 {
			t.Errorf("Expected 3 active goals, got %d", len(activeGoals))
		}

		// Verify all returned goals are active
		for _, goal := range activeGoals {
			if !goal.IsActive {
				t.Errorf("Goal %s should be active", goal.GoalID)
			}
		}

		// Verify correct goals returned
		goalIDs := make(map[string]bool)
		for _, goal := range activeGoals {
			goalIDs[goal.GoalID] = true
		}

		if !goalIDs["active-1"] || !goalIDs["active-2"] || !goalIDs["active-3"] {
			t.Error("Expected active-1, active-2, active-3 to be returned")
		}
		if goalIDs["inactive-1"] || goalIDs["inactive-2"] {
			t.Error("Inactive goals should not be returned")
		}
	})

	t.Run("returns goals ordered by challenge_id, goal_id", func(t *testing.T) {
		// Setup: Insert goals in random order
		goals := []*domain.UserGoalProgress{
			{UserID: "user-order", GoalID: "goal-2", ChallengeID: "challenge-2", Namespace: "test", IsActive: true, Status: domain.GoalStatusNotStarted},
			{UserID: "user-order", GoalID: "goal-1", ChallengeID: "challenge-1", Namespace: "test", IsActive: true, Status: domain.GoalStatusNotStarted},
			{UserID: "user-order", GoalID: "goal-3", ChallengeID: "challenge-1", Namespace: "test", IsActive: true, Status: domain.GoalStatusNotStarted},
		}
		err := repo.BulkInsert(ctx, goals)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Execute: Get active goals
		activeGoals, err := repo.GetActiveGoals(ctx, "user-order")
		if err != nil {
			t.Fatalf("GetActiveGoals failed: %v", err)
		}

		// Verify: Ordered by challenge_id, goal_id
		if len(activeGoals) != 3 {
			t.Fatalf("Expected 3 goals, got %d", len(activeGoals))
		}

		if activeGoals[0].ChallengeID != "challenge-1" || activeGoals[0].GoalID != "goal-1" {
			t.Errorf("First goal should be challenge-1/goal-1, got %s/%s", activeGoals[0].ChallengeID, activeGoals[0].GoalID)
		}
		if activeGoals[1].ChallengeID != "challenge-1" || activeGoals[1].GoalID != "goal-3" {
			t.Errorf("Second goal should be challenge-1/goal-3, got %s/%s", activeGoals[1].ChallengeID, activeGoals[1].GoalID)
		}
		if activeGoals[2].ChallengeID != "challenge-2" || activeGoals[2].GoalID != "goal-2" {
			t.Errorf("Third goal should be challenge-2/goal-2, got %s/%s", activeGoals[2].ChallengeID, activeGoals[2].GoalID)
		}
	})

	t.Run("returns all fields correctly", func(t *testing.T) {
		// Setup: Insert goal with all fields populated
		now := time.Now().UTC()
		completedAt := now.Add(-1 * time.Hour)
		goal := &domain.UserGoalProgress{
			UserID:      "user-fields",
			GoalID:      "goal-complete",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			Progress:    100,
			Status:      domain.GoalStatusCompleted,
			CompletedAt: &completedAt,
			IsActive:    true,
			AssignedAt:  &now,
		}
		err := repo.BulkInsert(ctx, []*domain.UserGoalProgress{goal})
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Execute: Get active goals
		activeGoals, err := repo.GetActiveGoals(ctx, "user-fields")
		if err != nil {
			t.Fatalf("GetActiveGoals failed: %v", err)
		}

		// Verify: All fields returned correctly
		if len(activeGoals) != 1 {
			t.Fatalf("Expected 1 goal, got %d", len(activeGoals))
		}

		result := activeGoals[0]
		if result.UserID != "user-fields" {
			t.Errorf("UserID = %s, want user-fields", result.UserID)
		}
		if result.Progress != 100 {
			t.Errorf("Progress = %d, want 100", result.Progress)
		}
		if result.Status != domain.GoalStatusCompleted {
			t.Errorf("Status = %s, want completed", result.Status)
		}
		if result.CompletedAt == nil {
			t.Error("CompletedAt should not be nil")
		}
		if result.AssignedAt == nil {
			t.Error("AssignedAt should not be nil")
		}
		if !result.IsActive {
			t.Error("IsActive should be true")
		}
	})
}

func TestPostgresTxRepository_BulkInsert(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("transaction bulk insert commit", func(t *testing.T) {
		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Prepare 100 goals
		now := time.Now()
		progresses := make([]*domain.UserGoalProgress, 100)
		for i := 0; i < 100; i++ {
			progresses[i] = &domain.UserGoalProgress{
				UserID:      "tx-bulk-user-1",
				GoalID:      fmt.Sprintf("tx-bulk-goal-%d", i),
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    i,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		// BulkInsert within transaction
		err = txRepo.BulkInsert(ctx, progresses)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Verify data visible within transaction
		txResult, err := txRepo.GetUserProgress(ctx, "tx-bulk-user-1", false)
		if err != nil {
			t.Fatalf("GetUserProgress within tx failed: %v", err)
		}
		if len(txResult) != 100 {
			t.Errorf("Within tx: expected 100 goals, got %d", len(txResult))
		}

		// Commit transaction
		err = txRepo.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify all 100 goals persisted
		result, err := repo.GetUserProgress(ctx, "tx-bulk-user-1", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(result) != 100 {
			t.Errorf("After commit: expected 100 goals, got %d", len(result))
		}
	})

	t.Run("transaction bulk insert rollback", func(t *testing.T) {
		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Prepare 50 goals
		now := time.Now()
		progresses := make([]*domain.UserGoalProgress, 50)
		for i := 0; i < 50; i++ {
			progresses[i] = &domain.UserGoalProgress{
				UserID:      "tx-bulk-user-2",
				GoalID:      fmt.Sprintf("tx-rollback-goal-%d", i),
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    i,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		// BulkInsert within transaction
		err = txRepo.BulkInsert(ctx, progresses)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Rollback transaction
		err = txRepo.Rollback()
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify no goals persisted
		result, err := repo.GetUserProgress(ctx, "tx-bulk-user-2", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(result) != 0 {
			t.Errorf("After rollback: expected 0 goals, got %d", len(result))
		}
	})
}

// TestPostgresGoalRepository_BulkInsertWithCOPY tests the optimized COPY-based bulk insert
func TestPostgresGoalRepository_BulkInsertWithCOPY(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("insert new records with COPY", func(t *testing.T) {
		now := time.Now()
		progresses := make([]*domain.UserGoalProgress, 10)
		for i := 0; i < 10; i++ {
			progresses[i] = &domain.UserGoalProgress{
				UserID:      "copy-user-1",
				GoalID:      fmt.Sprintf("copy-goal-%d", i),
				ChallengeID: "copy-challenge-1",
				Namespace:   "test",
				Progress:    i * 10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		// Use COPY protocol to insert
		err := repo.BulkInsertWithCOPY(ctx, progresses)
		if err != nil {
			t.Fatalf("BulkInsertWithCOPY failed: %v", err)
		}

		// Verify all records inserted
		for i := 0; i < 10; i++ {
			result, err := repo.GetProgress(ctx, "copy-user-1", fmt.Sprintf("copy-goal-%d", i))
			if err != nil {
				t.Errorf("GetProgress for goal %d failed: %v", i, err)
				continue
			}

			if result == nil {
				t.Errorf("Expected goal %d to exist", i)
				continue
			}

			if result.Progress != i*10 {
				t.Errorf("Goal %d: expected progress %d, got %d", i, i*10, result.Progress)
			}

			if result.Status != domain.GoalStatusInProgress {
				t.Errorf("Goal %d: expected status %s, got %s", i, domain.GoalStatusInProgress, result.Status)
			}

			if !result.IsActive {
				t.Errorf("Goal %d: expected is_active=true, got false", i)
			}
		}
	})

	t.Run("ON CONFLICT DO NOTHING - skip existing records", func(t *testing.T) {
		// Insert initial record
		initial := &domain.UserGoalProgress{
			UserID:      "copy-user-2",
			GoalID:      "copy-goal-conflict",
			ChallengeID: "copy-challenge-1",
			Namespace:   "test",
			Progress:    100,
			Status:      domain.GoalStatusCompleted,
		}
		err := repo.UpsertProgress(ctx, initial)
		if err != nil {
			t.Fatalf("Initial insert failed: %v", err)
		}

		// Try to insert same record with different data using COPY
		now := time.Now()
		conflictRecord := &domain.UserGoalProgress{
			UserID:      "copy-user-2",
			GoalID:      "copy-goal-conflict",
			ChallengeID: "copy-challenge-1",
			Namespace:   "test",
			Progress:    50, // Different value
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
			AssignedAt:  &now,
		}

		err = repo.BulkInsertWithCOPY(ctx, []*domain.UserGoalProgress{conflictRecord})
		if err != nil {
			t.Fatalf("BulkInsertWithCOPY failed: %v", err)
		}

		// Verify original record unchanged (ON CONFLICT DO NOTHING)
		result, err := repo.GetProgress(ctx, "copy-user-2", "copy-goal-conflict")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if result.Progress != 100 {
			t.Errorf("Expected progress=100 (unchanged), got %d", result.Progress)
		}

		if result.Status != domain.GoalStatusCompleted {
			t.Errorf("Expected status=%s (unchanged), got %s", domain.GoalStatusCompleted, result.Status)
		}
	})

	t.Run("empty slice - no-op", func(t *testing.T) {
		err := repo.BulkInsertWithCOPY(ctx, []*domain.UserGoalProgress{})
		if err != nil {
			t.Errorf("BulkInsertWithCOPY with empty slice should not error, got: %v", err)
		}
	})

	t.Run("all fields inserted correctly", func(t *testing.T) {
		now := time.Now().UTC() // Use UTC to match PostgreSQL storage
		expiresAt := now.Add(24 * time.Hour)
		completedAt := now.Add(-1 * time.Hour)

		progress := &domain.UserGoalProgress{
			UserID:      "copy-user-3",
			GoalID:      "copy-goal-all-fields",
			ChallengeID: "copy-challenge-1",
			Namespace:   "test-namespace",
			Progress:    75,
			Status:      domain.GoalStatusCompleted,
			CompletedAt: &completedAt,
			ClaimedAt:   nil,
			IsActive:    true,
			AssignedAt:  &now,
			ExpiresAt:   &expiresAt,
		}

		err := repo.BulkInsertWithCOPY(ctx, []*domain.UserGoalProgress{progress})
		if err != nil {
			t.Fatalf("BulkInsertWithCOPY failed: %v", err)
		}

		// Verify all fields
		result, err := repo.GetProgress(ctx, "copy-user-3", "copy-goal-all-fields")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if result.UserID != "copy-user-3" {
			t.Errorf("UserID = %s, want copy-user-3", result.UserID)
		}
		if result.GoalID != "copy-goal-all-fields" {
			t.Errorf("GoalID = %s, want copy-goal-all-fields", result.GoalID)
		}
		if result.ChallengeID != "copy-challenge-1" {
			t.Errorf("ChallengeID = %s, want copy-challenge-1", result.ChallengeID)
		}
		if result.Namespace != "test-namespace" {
			t.Errorf("Namespace = %s, want test-namespace", result.Namespace)
		}
		if result.Progress != 75 {
			t.Errorf("Progress = %d, want 75", result.Progress)
		}
		if result.Status != domain.GoalStatusCompleted {
			t.Errorf("Status = %s, want %s", result.Status, domain.GoalStatusCompleted)
		}
		if !result.IsActive {
			t.Error("IsActive = false, want true")
		}

		// Verify timestamps (compare Unix seconds to ignore timezone differences)
		if result.CompletedAt == nil {
			t.Error("CompletedAt is nil, want non-nil")
		} else if completedAtDiff := result.CompletedAt.Unix() - completedAt.Unix(); completedAtDiff < -1 || completedAtDiff > 1 {
			t.Errorf("CompletedAt = %v (unix: %d), want %v (unix: %d)", result.CompletedAt, result.CompletedAt.Unix(), completedAt, completedAt.Unix())
		}

		if result.AssignedAt == nil {
			t.Error("AssignedAt is nil, want non-nil")
		} else if assignedAtDiff := result.AssignedAt.Unix() - now.Unix(); assignedAtDiff < -1 || assignedAtDiff > 1 {
			t.Errorf("AssignedAt = %v (unix: %d), want %v (unix: %d)", result.AssignedAt, result.AssignedAt.Unix(), now, now.Unix())
		}

		if result.ExpiresAt == nil {
			t.Error("ExpiresAt is nil, want non-nil")
		} else if expiresAtDiff := result.ExpiresAt.Unix() - expiresAt.Unix(); expiresAtDiff < -1 || expiresAtDiff > 1 {
			t.Errorf("ExpiresAt = %v (unix: %d), want %v (unix: %d)", result.ExpiresAt, result.ExpiresAt.Unix(), expiresAt, expiresAt.Unix())
		}

		if result.ClaimedAt != nil {
			t.Errorf("ClaimedAt = %v, want nil", result.ClaimedAt)
		}
	})

	t.Run("large batch - 1000 records", func(t *testing.T) {
		now := time.Now()
		progresses := make([]*domain.UserGoalProgress, 1000)
		for i := 0; i < 1000; i++ {
			progresses[i] = &domain.UserGoalProgress{
				UserID:      fmt.Sprintf("copy-large-user-%d", i),
				GoalID:      "copy-large-goal",
				ChallengeID: "copy-large-challenge",
				Namespace:   "test",
				Progress:    i,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		// Measure performance
		start := time.Now()
		err := repo.BulkInsertWithCOPY(ctx, progresses)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("BulkInsertWithCOPY(1000) failed: %v", err)
		}

		// Performance target: < 50ms for 1000 records
		if elapsed > 50*time.Millisecond {
			t.Logf("Warning: BulkInsertWithCOPY(1000) took %v (target: < 50ms)", elapsed)
		} else {
			t.Logf("Performance: BulkInsertWithCOPY(1000) took %v", elapsed)
		}

		// Spot check a few records
		checkIndices := []int{0, 499, 999}
		for _, idx := range checkIndices {
			result, err := repo.GetProgress(ctx, fmt.Sprintf("copy-large-user-%d", idx), "copy-large-goal")
			if err != nil {
				t.Errorf("GetProgress for user %d failed: %v", idx, err)
				continue
			}

			if result == nil {
				t.Errorf("Expected record for user %d to exist", idx)
				continue
			}

			if result.Progress != idx {
				t.Errorf("User %d: expected progress %d, got %d", idx, idx, result.Progress)
			}
		}
	})
}

// TestPostgresTxRepository_BulkInsertWithCOPY tests transaction-based COPY bulk insert
func TestPostgresTxRepository_BulkInsertWithCOPY(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("transaction commit with COPY", func(t *testing.T) {
		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Prepare 100 goals
		now := time.Now()
		progresses := make([]*domain.UserGoalProgress, 100)
		for i := 0; i < 100; i++ {
			progresses[i] = &domain.UserGoalProgress{
				UserID:      "tx-copy-user-1",
				GoalID:      fmt.Sprintf("tx-copy-goal-%d", i),
				ChallengeID: "tx-copy-challenge-1",
				Namespace:   "test",
				Progress:    i,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		// BulkInsertWithCOPY within transaction
		err = txRepo.BulkInsertWithCOPY(ctx, progresses)
		if err != nil {
			t.Fatalf("BulkInsertWithCOPY failed: %v", err)
		}

		// Commit transaction
		err = txRepo.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify all 100 goals persisted
		for i := 0; i < 100; i++ {
			result, err := repo.GetProgress(ctx, "tx-copy-user-1", fmt.Sprintf("tx-copy-goal-%d", i))
			if err != nil {
				t.Errorf("GetProgress for goal %d failed: %v", i, err)
				continue
			}

			if result == nil {
				t.Errorf("Expected goal %d to exist after commit", i)
				continue
			}

			if result.Progress != i {
				t.Errorf("Goal %d: expected progress %d, got %d", i, i, result.Progress)
			}
		}
	})

	t.Run("transaction rollback with COPY", func(t *testing.T) {
		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Prepare 50 goals
		now := time.Now()
		progresses := make([]*domain.UserGoalProgress, 50)
		for i := 0; i < 50; i++ {
			progresses[i] = &domain.UserGoalProgress{
				UserID:      "tx-copy-rollback-user",
				GoalID:      fmt.Sprintf("tx-copy-rollback-goal-%d", i),
				ChallengeID: "tx-copy-challenge-1",
				Namespace:   "test",
				Progress:    i,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		// BulkInsertWithCOPY within transaction
		err = txRepo.BulkInsertWithCOPY(ctx, progresses)
		if err != nil {
			t.Fatalf("BulkInsertWithCOPY failed: %v", err)
		}

		// Rollback transaction
		err = txRepo.Rollback()
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify no goals persisted
		for i := 0; i < 50; i++ {
			result, err := repo.GetProgress(ctx, "tx-copy-rollback-user", fmt.Sprintf("tx-copy-rollback-goal-%d", i))
			if err != nil {
				t.Errorf("GetProgress for goal %d failed: %v", i, err)
				continue
			}

			if result != nil {
				t.Errorf("Expected goal %d to NOT exist after rollback, but found it", i)
			}
		}
	})

	t.Run("empty slice in transaction - no-op", func(t *testing.T) {
		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = txRepo.Rollback() }()

		// Call with empty slice
		err = txRepo.BulkInsertWithCOPY(ctx, []*domain.UserGoalProgress{})
		if err != nil {
			t.Errorf("BulkInsertWithCOPY with empty slice should not error, got: %v", err)
		}
	})
}

func TestPostgresTxRepository_GetGoalsByIDs(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("reads uncommitted data within transaction", func(t *testing.T) {
		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// BulkInsert goals within transaction
		now := time.Now()
		progresses := []*domain.UserGoalProgress{
			{
				UserID:      "tx-get-user-1",
				GoalID:      "tx-goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "tx-get-user-1",
				GoalID:      "tx-goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    20,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			},
		}

		err = txRepo.BulkInsert(ctx, progresses)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Read uncommitted data via GetGoalsByIDs
		result, err := txRepo.GetGoalsByIDs(ctx, "tx-get-user-1", []string{"tx-goal-1", "tx-goal-2"})
		if err != nil {
			t.Fatalf("GetGoalsByIDs within tx failed: %v", err)
		}

		// Verify: Can read uncommitted data
		if len(result) != 2 {
			t.Errorf("Within tx: expected 2 goals, got %d", len(result))
		}

		// Rollback transaction
		err = txRepo.Rollback()
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify: Data not persisted after rollback
		outsideResult, err := repo.GetGoalsByIDs(ctx, "tx-get-user-1", []string{"tx-goal-1", "tx-goal-2"})
		if err != nil {
			t.Fatalf("GetGoalsByIDs after rollback failed: %v", err)
		}

		if len(outsideResult) != 0 {
			t.Errorf("After rollback: expected 0 goals, got %d", len(outsideResult))
		}
	})

	t.Run("reads committed data", func(t *testing.T) {
		// Setup: Insert goals outside transaction
		now := time.Now()
		err := repo.UpsertProgress(ctx, &domain.UserGoalProgress{
			UserID:      "tx-get-user-2",
			GoalID:      "committed-goal",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			Progress:    100,
			Status:      domain.GoalStatusCompleted,
			IsActive:    true,
			AssignedAt:  &now,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = txRepo.Rollback() }()

		// Read committed data via GetGoalsByIDs
		result, err := txRepo.GetGoalsByIDs(ctx, "tx-get-user-2", []string{"committed-goal"})
		if err != nil {
			t.Fatalf("GetGoalsByIDs failed: %v", err)
		}

		// Verify: Can read committed data
		if len(result) != 1 {
			t.Errorf("Expected 1 committed goal, got %d", len(result))
		}
		if result[0].Progress != 100 {
			t.Errorf("Expected progress=100, got %d", result[0].Progress)
		}
	})
}

func TestPostgresTxRepository_UpsertGoalActive(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("transaction upsert commit", func(t *testing.T) {
		// Setup: Create inactive goal
		err := repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:      "tx-upsert-user-1",
			GoalID:      "tx-upsert-goal-1",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			IsActive:    false,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Activate goal within transaction
		err = txRepo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:   "tx-upsert-user-1",
			GoalID:   "tx-upsert-goal-1",
			IsActive: true,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		// Commit
		err = txRepo.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify: is_active updated
		result, err := repo.GetProgress(ctx, "tx-upsert-user-1", "tx-upsert-goal-1")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if !result.IsActive {
			t.Error("Expected is_active=true after commit")
		}
	})

	t.Run("transaction upsert rollback", func(t *testing.T) {
		// Setup: Create active goal
		now := time.Now()
		err := repo.UpsertProgress(ctx, &domain.UserGoalProgress{
			UserID:      "tx-upsert-user-2",
			GoalID:      "tx-upsert-goal-2",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			Progress:    50,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
			AssignedAt:  &now,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Deactivate goal within transaction
		err = txRepo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:   "tx-upsert-user-2",
			GoalID:   "tx-upsert-goal-2",
			IsActive: false,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		// Rollback
		err = txRepo.Rollback()
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify: is_active unchanged (still true)
		result, err := repo.GetProgress(ctx, "tx-upsert-user-2", "tx-upsert-goal-2")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if !result.IsActive {
			t.Error("Expected is_active=true (unchanged) after rollback")
		}
	})

	t.Run("insert new goal when activating non-existent goal", func(t *testing.T) {
		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = txRepo.Rollback() }()

		// Try to activate a goal that doesn't exist
		err = txRepo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:      "tx-new-user",
			GoalID:      "tx-new-goal",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			IsActive:    true,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		// Commit
		err = txRepo.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify: new row created with is_active=true
		result, err := repo.GetProgress(ctx, "tx-new-user", "tx-new-goal")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}
		if result == nil {
			t.Fatal("Expected new row to be created")
		}
		if !result.IsActive {
			t.Error("Expected is_active=true for new row")
		}
		if result.Progress != 0 {
			t.Errorf("Expected progress=0, got %d", result.Progress)
		}
		if result.Status != domain.GoalStatusNotStarted {
			t.Errorf("Expected status=not_started, got %s", result.Status)
		}
		if result.AssignedAt == nil {
			t.Error("Expected assigned_at to be set for new active goal")
		}
	})

	t.Run("insert new goal when deactivating non-existent goal", func(t *testing.T) {
		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = txRepo.Rollback() }()

		// Try to deactivate a goal that doesn't exist
		err = txRepo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:      "tx-new-inactive-user",
			GoalID:      "tx-new-inactive-goal",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			IsActive:    false,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		// Commit
		err = txRepo.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify: new row created with is_active=false
		result, err := repo.GetProgress(ctx, "tx-new-inactive-user", "tx-new-inactive-goal")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}
		if result == nil {
			t.Fatal("Expected new row to be created")
		}
		if result.IsActive {
			t.Error("Expected is_active=false for new row")
		}
		if result.Progress != 0 {
			t.Errorf("Expected progress=0, got %d", result.Progress)
		}
		if result.Status != domain.GoalStatusNotStarted {
			t.Errorf("Expected status=not_started, got %s", result.Status)
		}
		if result.AssignedAt != nil {
			t.Error("Expected assigned_at to be NULL for new inactive goal")
		}
	})

	t.Run("update preserves assigned_at when deactivating", func(t *testing.T) {
		// Setup: Create active goal with assigned_at
		assignedTime := time.Now().UTC().Add(-24 * time.Hour)
		err := repo.UpsertProgress(ctx, &domain.UserGoalProgress{
			UserID:      "tx-preserve-user",
			GoalID:      "tx-preserve-goal",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
			AssignedAt:  &assignedTime,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = txRepo.Rollback() }()

		// Deactivate goal
		err = txRepo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:   "tx-preserve-user",
			GoalID:   "tx-preserve-goal",
			IsActive: false,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		// Commit
		err = txRepo.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify: assigned_at preserved
		result, err := repo.GetProgress(ctx, "tx-preserve-user", "tx-preserve-goal")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}
		if result.IsActive {
			t.Error("Expected is_active=false")
		}
		if result.AssignedAt == nil {
			t.Fatal("Expected assigned_at to be preserved")
		}
		// Allow small time difference for DB timestamp precision
		if result.AssignedAt.UTC().Unix() != assignedTime.Unix() {
			t.Errorf("Expected assigned_at to be preserved as %v, got %v", assignedTime, result.AssignedAt)
		}
	})

	t.Run("update sets assigned_at when activating", func(t *testing.T) {
		// Setup: Create inactive goal (no assigned_at)
		err := repo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:      "tx-assign-user",
			GoalID:      "tx-assign-goal",
			ChallengeID: "challenge-1",
			Namespace:   "test",
			IsActive:    false,
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify: initially no assigned_at
		initial, err := repo.GetProgress(ctx, "tx-assign-user", "tx-assign-goal")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}
		if initial.AssignedAt != nil {
			t.Error("Expected assigned_at to be NULL initially")
		}

		beforeActivate := time.Now().UTC()
		time.Sleep(10 * time.Millisecond) // Ensure timestamp difference

		// Begin transaction
		txRepo, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = txRepo.Rollback() }()

		// Activate goal
		err = txRepo.UpsertGoalActive(ctx, &domain.UserGoalProgress{
			UserID:   "tx-assign-user",
			GoalID:   "tx-assign-goal",
			IsActive: true,
		})
		if err != nil {
			t.Fatalf("UpsertGoalActive failed: %v", err)
		}

		// Commit
		err = txRepo.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify: assigned_at set to NOW()
		result, err := repo.GetProgress(ctx, "tx-assign-user", "tx-assign-goal")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}
		if !result.IsActive {
			t.Error("Expected is_active=true")
		}
		if result.AssignedAt == nil {
			t.Fatal("Expected assigned_at to be set")
		}
		// assigned_at should be after we started the activation
		if result.AssignedAt.Before(beforeActivate) {
			t.Errorf("Expected assigned_at to be after %v, got %v", beforeActivate, result.AssignedAt)
		}
	})
}

// TestPostgresTxRepository_BatchUpsertProgressWithCOPY tests transaction COPY batch upsert
func TestPostgresTxRepository_BatchUpsertProgressWithCOPY(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("transaction_batch_COPY_commit", func(t *testing.T) {
		// First create 100 initial records using BulkInsertWithCOPY
		initialRecords := make([]*domain.UserGoalProgress, 100)
		for i := 0; i < 100; i++ {
			initialRecords[i] = &domain.UserGoalProgress{
				UserID:      fmt.Sprintf("copy-user-%d", i),
				GoalID:      "copy-goal-1",
				ChallengeID: "copy-challenge-1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
			}
		}
		err := repo.BulkInsertWithCOPY(ctx, initialRecords)
		if err != nil {
			t.Fatalf("BulkInsertWithCOPY setup failed: %v", err)
		}

		// Create 100 CopyRow updates
		rows := make([]CopyRow, 100)
		for i := 0; i < 100; i++ {
			p := 10 + i
			rows[i] = CopyRow{
				UserID:       fmt.Sprintf("copy-user-%d", i),
				GoalID:       "copy-goal-1",
				ChallengeID:  "copy-challenge-1",
				Namespace:    "test",
				Progress:     &p,
				ProgressMode: "absolute",
				TargetValue:  1000,
			}
		}

		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Batch update using COPY
		err = tx.BatchUpsertProgressWithCOPY(ctx, rows)
		if err != nil {
			t.Fatalf("BatchUpsertProgressWithCOPY failed: %v", err)
		}

		// Commit transaction
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify: All 100 records updated
		for i := 0; i < 100; i++ {
			result, err := repo.GetProgress(ctx, fmt.Sprintf("copy-user-%d", i), "copy-goal-1")
			if err != nil {
				t.Errorf("GetProgress for user %d failed: %v", i, err)
				continue
			}

			if result.Progress != 10+i {
				t.Errorf("User %d: expected progress %d, got %d", i, 10+i, result.Progress)
			}
		}
	})

	t.Run("transaction_batch_COPY_rollback", func(t *testing.T) {
		// First create 50 initial records
		initialRecords := make([]*domain.UserGoalProgress, 50)
		for i := 0; i < 50; i++ {
			initialRecords[i] = &domain.UserGoalProgress{
				UserID:      fmt.Sprintf("copy-rollback-user-%d", i),
				GoalID:      "copy-rollback-goal-1",
				ChallengeID: "copy-rollback-challenge-1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
			}
		}
		err := repo.BulkInsertWithCOPY(ctx, initialRecords)
		if err != nil {
			t.Fatalf("BulkInsertWithCOPY setup failed: %v", err)
		}

		// Create CopyRow updates
		rows := make([]CopyRow, 50)
		for i := 0; i < 50; i++ {
			p := 20 + i
			rows[i] = CopyRow{
				UserID:       fmt.Sprintf("copy-rollback-user-%d", i),
				GoalID:       "copy-rollback-goal-1",
				ChallengeID:  "copy-rollback-challenge-1",
				Namespace:    "test",
				Progress:     &p,
				ProgressMode: "absolute",
				TargetValue:  1000,
			}
		}

		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Batch update using COPY
		err = tx.BatchUpsertProgressWithCOPY(ctx, rows)
		if err != nil {
			t.Fatalf("BatchUpsertProgressWithCOPY failed: %v", err)
		}

		// Rollback transaction
		err = tx.Rollback()
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify: Records still have original progress (rollback discarded updates)
		for i := 0; i < 50; i++ {
			result, err := repo.GetProgress(ctx, fmt.Sprintf("copy-rollback-user-%d", i), "copy-rollback-goal-1")
			if err != nil {
				t.Errorf("User %d: GetProgress failed: %v", i, err)
				continue
			}
			if result == nil {
				t.Errorf("User %d: expected record to exist", i)
				continue
			}
			if result.Progress != 0 {
				t.Errorf("User %d: expected progress 0 after rollback, got %d", i, result.Progress)
			}
		}
	})

	t.Run("transaction_COPY_handles_large_batches", func(t *testing.T) {
		// First create 1000 initial records
		initialRecords := make([]*domain.UserGoalProgress, 1000)
		for i := 0; i < 1000; i++ {
			initialRecords[i] = &domain.UserGoalProgress{
				UserID:      fmt.Sprintf("copy-large-user-%d", i),
				GoalID:      "copy-large-goal-1",
				ChallengeID: "copy-large-challenge-1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
			}
		}
		err := repo.BulkInsertWithCOPY(ctx, initialRecords)
		if err != nil {
			t.Fatalf("BulkInsertWithCOPY setup failed: %v", err)
		}

		// Create 1000 CopyRow updates for stress test
		rows := make([]CopyRow, 1000)
		for i := 0; i < 1000; i++ {
			p := i
			rows[i] = CopyRow{
				UserID:       fmt.Sprintf("copy-large-user-%d", i),
				GoalID:       "copy-large-goal-1",
				ChallengeID:  "copy-large-challenge-1",
				Namespace:    "test",
				Progress:     &p,
				ProgressMode: "absolute",
				TargetValue:  10000,
			}
		}

		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Batch update using COPY
		start := time.Now()
		err = tx.BatchUpsertProgressWithCOPY(ctx, rows)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("BatchUpsertProgressWithCOPY failed: %v", err)
		}

		// Commit
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Performance check: 1000 rows should be < 200ms
		if elapsed > 200*time.Millisecond {
			t.Logf("Warning: BatchUpsertProgressWithCOPY(1000) took %v (expected < 200ms)", elapsed)
		}

		// Spot check a few records
		checkIndices := []int{0, 499, 999}
		for _, idx := range checkIndices {
			result, err := repo.GetProgress(ctx, fmt.Sprintf("copy-large-user-%d", idx), "copy-large-goal-1")
			if err != nil {
				t.Errorf("GetProgress for user %d failed: %v", idx, err)
				continue
			}

			if result.Progress != idx {
				t.Errorf("User %d: expected progress %d, got %d", idx, idx, result.Progress)
			}
		}
	})
}

// TestPostgresTxRepository_GetProgress tests transaction GetProgress
func TestPostgresTxRepository_GetProgress(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("reads_data_within_transaction", func(t *testing.T) {
		// Create initial progress
		initial := &domain.UserGoalProgress{
			UserID:      "tx-get-user-1",
			GoalID:      "tx-get-goal-1",
			ChallengeID: "tx-get-challenge-1",
			Namespace:   "test",
			Progress:    50,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
		}
		if err := repo.UpsertProgress(ctx, initial); err != nil {
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		// Read progress within transaction
		result, err := tx.GetProgress(ctx, "tx-get-user-1", "tx-get-goal-1")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		// Verify data
		if result.Progress != 50 {
			t.Errorf("Expected progress 50, got %d", result.Progress)
		}
	})

	t.Run("returns_nil_for_nonexistent_progress", func(t *testing.T) {
		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		// Try to read non-existent progress
		result, err := tx.GetProgress(ctx, "nonexistent-user", "nonexistent-goal")
		if err != nil {
			t.Errorf("Expected no error for nonexistent progress, got %v", err)
		}
		if result != nil {
			t.Error("Expected nil result for nonexistent progress, got non-nil")
		}
	})
}

// TestPostgresTxRepository_GetProgressForUpdate tests transaction GetProgressForUpdate
func TestPostgresTxRepository_GetProgressForUpdate(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("locks_row_for_update", func(t *testing.T) {
		// Create initial progress
		initial := &domain.UserGoalProgress{
			UserID:      "lock-user-1",
			GoalID:      "lock-goal-1",
			ChallengeID: "lock-challenge-1",
			Namespace:   "test",
			Progress:    30,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
		}
		if err := repo.UpsertProgress(ctx, initial); err != nil {
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Lock row for update
		result, err := tx.GetProgressForUpdate(ctx, "lock-user-1", "lock-goal-1")
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("GetProgressForUpdate failed: %v", err)
		}

		// Verify data
		if result.Progress != 30 {
			t.Errorf("Expected progress 30, got %d", result.Progress)
		}

		// Update progress within transaction
		result.Progress = 60
		if err := tx.UpsertProgress(ctx, result); err != nil {
			_ = tx.Rollback()
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		// Commit
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify update persisted
		final, err := repo.GetProgress(ctx, "lock-user-1", "lock-goal-1")
		if err != nil {
			t.Fatalf("GetProgress failed: %v", err)
		}

		if final.Progress != 60 {
			t.Errorf("Expected progress 60 after commit, got %d", final.Progress)
		}
	})

	t.Run("returns_nil_for_nonexistent_progress", func(t *testing.T) {
		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		// Try to lock non-existent progress
		result, err := tx.GetProgressForUpdate(ctx, "nonexistent-user-2", "nonexistent-goal-2")
		if err != nil {
			t.Errorf("Expected no error for nonexistent progress, got %v", err)
		}
		if result != nil {
			t.Error("Expected nil result for nonexistent progress, got non-nil")
		}
	})
}

// TestPostgresTxRepository_BeginTx_CommitRollback tests transaction lifecycle
func TestPostgresTxRepository_BeginTx_CommitRollback(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("multiple_operations_in_transaction", func(t *testing.T) {
		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Operation 1: Insert progress
		progress1 := &domain.UserGoalProgress{
			UserID:      "multi-user-1",
			GoalID:      "multi-goal-1",
			ChallengeID: "multi-challenge-1",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
		}
		if err := tx.UpsertProgress(ctx, progress1); err != nil {
			_ = tx.Rollback()
			t.Fatalf("UpsertProgress 1 failed: %v", err)
		}

		// Operation 2: Insert another progress
		progress2 := &domain.UserGoalProgress{
			UserID:      "multi-user-1",
			GoalID:      "multi-goal-2",
			ChallengeID: "multi-challenge-1",
			Namespace:   "test",
			Progress:    20,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
		}
		if err := tx.UpsertProgress(ctx, progress2); err != nil {
			_ = tx.Rollback()
			t.Fatalf("UpsertProgress 2 failed: %v", err)
		}

		// Operation 3: Batch insert
		batch := []*domain.UserGoalProgress{
			{
				UserID:      "multi-user-2",
				GoalID:      "multi-goal-1",
				ChallengeID: "multi-challenge-1",
				Namespace:   "test",
				Progress:    30,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
			{
				UserID:      "multi-user-2",
				GoalID:      "multi-goal-2",
				ChallengeID: "multi-challenge-1",
				Namespace:   "test",
				Progress:    40,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
		}
		if err := tx.BatchUpsertProgress(ctx, batch); err != nil {
			_ = tx.Rollback()
			t.Fatalf("BatchUpsertProgress failed: %v", err)
		}

		// Commit all operations
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify all operations persisted
		results := []struct {
			userID   string
			goalID   string
			expected int
		}{
			{"multi-user-1", "multi-goal-1", 10},
			{"multi-user-1", "multi-goal-2", 20},
			{"multi-user-2", "multi-goal-1", 30},
			{"multi-user-2", "multi-goal-2", 40},
		}

		for _, r := range results {
			result, err := repo.GetProgress(ctx, r.userID, r.goalID)
			if err != nil {
				t.Errorf("GetProgress(%s, %s) failed: %v", r.userID, r.goalID, err)
				continue
			}

			if result.Progress != r.expected {
				t.Errorf("GetProgress(%s, %s): expected progress %d, got %d",
					r.userID, r.goalID, r.expected, result.Progress)
			}
		}
	})

	t.Run("rollback_discards_all_operations", func(t *testing.T) {
		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Perform multiple operations
		progress1 := &domain.UserGoalProgress{
			UserID:      "rollback-user-1",
			GoalID:      "rollback-goal-1",
			ChallengeID: "rollback-challenge-1",
			Namespace:   "test",
			Progress:    100,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
		}
		if err := tx.UpsertProgress(ctx, progress1); err != nil {
			_ = tx.Rollback()
			t.Fatalf("UpsertProgress failed: %v", err)
		}

		batch := []*domain.UserGoalProgress{
			{
				UserID:      "rollback-user-2",
				GoalID:      "rollback-goal-1",
				ChallengeID: "rollback-challenge-1",
				Namespace:   "test",
				Progress:    200,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
		}
		if err := tx.BatchUpsertProgress(ctx, batch); err != nil {
			_ = tx.Rollback()
			t.Fatalf("BatchUpsertProgress failed: %v", err)
		}

		// Rollback everything
		if err := tx.Rollback(); err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify: No records exist
		result1, err1 := repo.GetProgress(ctx, "rollback-user-1", "rollback-goal-1")
		if err1 != nil {
			t.Errorf("GetProgress for rollback-user-1 failed: %v", err1)
		}
		if result1 != nil {
			t.Error("Expected rollback-user-1 record not found, but found it")
		}

		result2, err2 := repo.GetProgress(ctx, "rollback-user-2", "rollback-goal-1")
		if err2 != nil {
			t.Errorf("GetProgress for rollback-user-2 failed: %v", err2)
		}
		if result2 != nil {
			t.Error("Expected rollback-user-2 record not found, but found it")
		}
	})
}

func TestPostgresTxRepository_GetUserGoalCount(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("returns 0 for user with no goals in transaction", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		count, err := tx.GetUserGoalCount(ctx, "tx-user-no-goals")
		if err != nil {
			t.Fatalf("GetUserGoalCount failed: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected count=0, got %d", count)
		}
	})

	t.Run("returns correct count within transaction", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		// Insert goals within transaction
		goals := []*domain.UserGoalProgress{
			{
				UserID:      "tx-count-user",
				GoalID:      "tx-goal-1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
			{
				UserID:      "tx-count-user",
				GoalID:      "tx-goal-2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    20,
				Status:      domain.GoalStatusInProgress,
				IsActive:    false,
			},
			{
				UserID:      "tx-count-user",
				GoalID:      "tx-goal-3",
				ChallengeID: "challenge2",
				Namespace:   "test",
				Progress:    30,
				Status:      domain.GoalStatusCompleted,
				IsActive:    true,
			},
		}

		err = tx.BulkInsert(ctx, goals)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Count within same transaction
		count, err := tx.GetUserGoalCount(ctx, "tx-count-user")
		if err != nil {
			t.Fatalf("GetUserGoalCount failed: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected count=3, got %d", count)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify count persists after commit
		count2, err := repo.GetUserGoalCount(ctx, "tx-count-user")
		if err != nil {
			t.Fatalf("GetUserGoalCount after commit failed: %v", err)
		}
		if count2 != 3 {
			t.Errorf("Expected count=3 after commit, got %d", count2)
		}
	})

	t.Run("rollback prevents count changes", func(t *testing.T) {
		// Verify initial count is 0
		initialCount, err := repo.GetUserGoalCount(ctx, "tx-rollback-user")
		if err != nil {
			t.Fatalf("Initial GetUserGoalCount failed: %v", err)
		}
		if initialCount != 0 {
			t.Errorf("Expected initial count=0, got %d", initialCount)
		}

		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Insert goals within transaction
		goals := []*domain.UserGoalProgress{
			{
				UserID:      "tx-rollback-user",
				GoalID:      "tx-goal-1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
		}

		err = tx.BulkInsert(ctx, goals)
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Count within transaction should be 1
		txCount, err := tx.GetUserGoalCount(ctx, "tx-rollback-user")
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("GetUserGoalCount in tx failed: %v", err)
		}
		if txCount != 1 {
			t.Errorf("Expected tx count=1, got %d", txCount)
		}

		// Rollback
		if err := tx.Rollback(); err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Count should still be 0 after rollback
		finalCount, err := repo.GetUserGoalCount(ctx, "tx-rollback-user")
		if err != nil {
			t.Fatalf("GetUserGoalCount after rollback failed: %v", err)
		}
		if finalCount != 0 {
			t.Errorf("Expected count=0 after rollback, got %d", finalCount)
		}
	})
}

func TestPostgresTxRepository_GetActiveGoals(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("returns empty slice for user with no goals in transaction", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		goals, err := tx.GetActiveGoals(ctx, "tx-user-no-active")
		if err != nil {
			t.Fatalf("GetActiveGoals failed: %v", err)
		}

		if len(goals) != 0 {
			t.Errorf("Expected 0 goals, got %d", len(goals))
		}
	})

	t.Run("returns only active goals within transaction", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		// Insert mix of active and inactive goals
		allGoals := []*domain.UserGoalProgress{
			{
				UserID:      "tx-filter-user",
				GoalID:      "active-1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
			{
				UserID:      "tx-filter-user",
				GoalID:      "active-2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    20,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
			{
				UserID:      "tx-filter-user",
				GoalID:      "inactive-1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    30,
				Status:      domain.GoalStatusInProgress,
				IsActive:    false,
			},
			{
				UserID:      "tx-filter-user",
				GoalID:      "inactive-2",
				ChallengeID: "challenge2",
				Namespace:   "test",
				Progress:    40,
				Status:      domain.GoalStatusCompleted,
				IsActive:    false,
			},
		}

		err = tx.BulkInsert(ctx, allGoals)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Get active goals
		activeGoals, err := tx.GetActiveGoals(ctx, "tx-filter-user")
		if err != nil {
			t.Fatalf("GetActiveGoals failed: %v", err)
		}

		// Should return only 2 active goals
		if len(activeGoals) != 2 {
			t.Errorf("Expected 2 active goals, got %d", len(activeGoals))
		}

		// Verify all returned goals are active
		for i, goal := range activeGoals {
			if !goal.IsActive {
				t.Errorf("Goal %d is not active: %+v", i, goal)
			}
		}

		// Commit
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
	})

	t.Run("returns goals ordered by challenge_id, goal_id within transaction", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		// Insert goals in non-sorted order
		goals := []*domain.UserGoalProgress{
			{
				UserID:      "tx-order-user",
				GoalID:      "goal-z",
				ChallengeID: "challenge-b",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
			{
				UserID:      "tx-order-user",
				GoalID:      "goal-b",
				ChallengeID: "challenge-a",
				Namespace:   "test",
				Progress:    20,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
			{
				UserID:      "tx-order-user",
				GoalID:      "goal-a",
				ChallengeID: "challenge-a",
				Namespace:   "test",
				Progress:    30,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
		}

		err = tx.BulkInsert(ctx, goals)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Get active goals
		activeGoals, err := tx.GetActiveGoals(ctx, "tx-order-user")
		if err != nil {
			t.Fatalf("GetActiveGoals failed: %v", err)
		}

		if len(activeGoals) != 3 {
			t.Fatalf("Expected 3 goals, got %d", len(activeGoals))
		}

		// Verify ordering: challenge-a/goal-a, challenge-a/goal-b, challenge-b/goal-z
		expected := []struct {
			challengeID string
			goalID      string
		}{
			{"challenge-a", "goal-a"},
			{"challenge-a", "goal-b"},
			{"challenge-b", "goal-z"},
		}

		for i, exp := range expected {
			if activeGoals[i].ChallengeID != exp.challengeID || activeGoals[i].GoalID != exp.goalID {
				t.Errorf("Goal %d: expected (%s, %s), got (%s, %s)",
					i, exp.challengeID, exp.goalID,
					activeGoals[i].ChallengeID, activeGoals[i].GoalID)
			}
		}

		// Commit
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
	})

	t.Run("rollback prevents changes from being visible", func(t *testing.T) {
		// Verify no goals initially
		initialGoals, err := repo.GetActiveGoals(ctx, "tx-rollback-active-user")
		if err != nil {
			t.Fatalf("Initial GetActiveGoals failed: %v", err)
		}
		if len(initialGoals) != 0 {
			t.Errorf("Expected 0 initial goals, got %d", len(initialGoals))
		}

		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Insert goals within transaction
		goals := []*domain.UserGoalProgress{
			{
				UserID:      "tx-rollback-active-user",
				GoalID:      "goal-1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
			},
		}

		err = tx.BulkInsert(ctx, goals)
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Within transaction, should see 1 goal
		txGoals, err := tx.GetActiveGoals(ctx, "tx-rollback-active-user")
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("GetActiveGoals in tx failed: %v", err)
		}
		if len(txGoals) != 1 {
			t.Errorf("Expected 1 goal in tx, got %d", len(txGoals))
		}

		// Rollback
		if err := tx.Rollback(); err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// After rollback, should still be 0 goals
		finalGoals, err := repo.GetActiveGoals(ctx, "tx-rollback-active-user")
		if err != nil {
			t.Fatalf("GetActiveGoals after rollback failed: %v", err)
		}
		if len(finalGoals) != 0 {
			t.Errorf("Expected 0 goals after rollback, got %d", len(finalGoals))
		}
	})
}

// M4 Tests: BatchUpsertGoalActive

func TestPostgresGoalRepository_BatchUpsertGoalActive(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("activates multiple new goals in single operation", func(t *testing.T) {
		// Setup: No existing records
		now := time.Now()
		progresses := []*domain.UserGoalProgress{
			{
				UserID:      "batch-user-1",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-1",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-1",
				GoalID:      "goal-3",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
		}

		// Execute: BatchUpsertGoalActive
		start := time.Now()
		err := repo.BatchUpsertGoalActive(ctx, progresses)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("BatchUpsertGoalActive failed: %v", err)
		}

		// Verify: All 3 goals are active
		result, err := repo.GetUserProgress(ctx, "batch-user-1", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(result) != 3 {
			t.Errorf("Expected 3 records, got %d", len(result))
		}

		for _, p := range result {
			if !p.IsActive {
				t.Errorf("Goal %s should be active", p.GoalID)
			}
			if p.Status != domain.GoalStatusNotStarted {
				t.Errorf("Goal %s should have status 'not_started', got %s", p.GoalID, p.Status)
			}
			if p.AssignedAt == nil {
				t.Errorf("Goal %s should have assigned_at set", p.GoalID)
			}
		}

		// Verify: Performance < 20ms
		if elapsed > 20*time.Millisecond {
			t.Logf("Warning: BatchUpsertGoalActive(3) took %v (expected < 20ms)", elapsed)
		}
	})

	t.Run("updates existing inactive goals to active", func(t *testing.T) {
		// Setup: Create 3 inactive goals
		now := time.Now()
		initialProgresses := []*domain.UserGoalProgress{
			{
				UserID:      "batch-user-2",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    5,
				Status:      domain.GoalStatusInProgress,
				IsActive:    false,
				AssignedAt:  nil,
			},
			{
				UserID:      "batch-user-2",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    false,
				AssignedAt:  nil,
			},
			{
				UserID:      "batch-user-2",
				GoalID:      "goal-3",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    false,
				AssignedAt:  nil,
			},
		}

		err := repo.BulkInsert(ctx, initialProgresses)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Execute: BatchUpsertGoalActive to activate them
		activateProgresses := []*domain.UserGoalProgress{
			{
				UserID:      "batch-user-2",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-2",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-2",
				GoalID:      "goal-3",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
		}

		err = repo.BatchUpsertGoalActive(ctx, activateProgresses)
		if err != nil {
			t.Fatalf("BatchUpsertGoalActive failed: %v", err)
		}

		// Verify: All goals are now active, progress preserved
		result, err := repo.GetUserProgress(ctx, "batch-user-2", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(result) != 3 {
			t.Errorf("Expected 3 records, got %d", len(result))
		}

		for _, p := range result {
			if !p.IsActive {
				t.Errorf("Goal %s should be active", p.GoalID)
			}
			if p.AssignedAt == nil {
				t.Errorf("Goal %s should have assigned_at set", p.GoalID)
			}

			// Verify progress is preserved
			switch p.GoalID {
			case "goal-1":
				if p.Progress != 5 {
					t.Errorf("Goal goal-1 should have progress 5, got %d", p.Progress)
				}
			case "goal-2":
				if p.Progress != 10 {
					t.Errorf("Goal goal-2 should have progress 10, got %d", p.Progress)
				}
			case "goal-3":
				if p.Progress != 0 {
					t.Errorf("Goal goal-3 should have progress 0, got %d", p.Progress)
				}
			}
		}
	})

	t.Run("handles mixed existing and new goals", func(t *testing.T) {
		// Setup: Create 2 existing goals (1 active, 1 inactive)
		now := time.Now()
		initialProgresses := []*domain.UserGoalProgress{
			{
				UserID:      "batch-user-3",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    5,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-3",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    false,
				AssignedAt:  nil,
			},
		}

		err := repo.BulkInsert(ctx, initialProgresses)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Execute: BatchUpsertGoalActive with 3 goals (2 existing + 1 new)
		activateProgresses := []*domain.UserGoalProgress{
			{
				UserID:      "batch-user-3",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-3",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-3",
				GoalID:      "goal-3", // NEW
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
		}

		err = repo.BatchUpsertGoalActive(ctx, activateProgresses)
		if err != nil {
			t.Fatalf("BatchUpsertGoalActive failed: %v", err)
		}

		// Verify: All 3 goals exist and are active
		result, err := repo.GetUserProgress(ctx, "batch-user-3", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(result) != 3 {
			t.Errorf("Expected 3 records, got %d", len(result))
		}

		for _, p := range result {
			if !p.IsActive {
				t.Errorf("Goal %s should be active", p.GoalID)
			}
		}
	})

	t.Run("deactivates multiple active goals in single operation", func(t *testing.T) {
		// Setup: Create 3 active goals with progress
		now := time.Now()
		initialProgresses := []*domain.UserGoalProgress{
			{
				UserID:      "batch-user-deactivate-1",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    5,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-deactivate-1",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-deactivate-1",
				GoalID:      "goal-3",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
				AssignedAt:  &now,
			},
		}

		err := repo.BulkInsert(ctx, initialProgresses)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Execute: BatchUpsertGoalActive to deactivate them
		deactivateProgresses := []*domain.UserGoalProgress{
			{
				UserID:      "batch-user-deactivate-1",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    false,
				AssignedAt:  nil,
			},
			{
				UserID:      "batch-user-deactivate-1",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    false,
				AssignedAt:  nil,
			},
			{
				UserID:      "batch-user-deactivate-1",
				GoalID:      "goal-3",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    false,
				AssignedAt:  nil,
			},
		}

		start := time.Now()
		err = repo.BatchUpsertGoalActive(ctx, deactivateProgresses)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("BatchUpsertGoalActive failed: %v", err)
		}

		// Verify: All goals are now inactive, progress preserved
		result, err := repo.GetUserProgress(ctx, "batch-user-deactivate-1", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(result) != 3 {
			t.Errorf("Expected 3 records, got %d", len(result))
		}

		for _, p := range result {
			if p.IsActive {
				t.Errorf("Goal %s should be inactive", p.GoalID)
			}
			if p.AssignedAt != nil {
				t.Errorf("Goal %s should have assigned_at cleared", p.GoalID)
			}

			// Verify progress is preserved during deactivation
			switch p.GoalID {
			case "goal-1":
				if p.Progress != 5 {
					t.Errorf("Goal goal-1 should have progress 5, got %d", p.Progress)
				}
				if p.Status != domain.GoalStatusInProgress {
					t.Errorf("Goal goal-1 should have status 'in_progress', got %s", p.Status)
				}
			case "goal-2":
				if p.Progress != 10 {
					t.Errorf("Goal goal-2 should have progress 10, got %d", p.Progress)
				}
				if p.Status != domain.GoalStatusInProgress {
					t.Errorf("Goal goal-2 should have status 'in_progress', got %s", p.Status)
				}
			case "goal-3":
				if p.Progress != 0 {
					t.Errorf("Goal goal-3 should have progress 0, got %d", p.Progress)
				}
				if p.Status != domain.GoalStatusNotStarted {
					t.Errorf("Goal goal-3 should have status 'not_started', got %s", p.Status)
				}
			}
		}

		// Verify: Performance < 20ms
		if elapsed > 20*time.Millisecond {
			t.Logf("Warning: BatchUpsertGoalActive(deactivate 3) took %v (expected < 20ms)", elapsed)
		}
	})

	t.Run("deactivates and reactivates goals preserving progress (M4 replace mode)", func(t *testing.T) {
		// Setup: Create 3 active goals with progress
		now := time.Now()
		initialProgresses := []*domain.UserGoalProgress{
			{
				UserID:      "batch-user-replace-1",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    15,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-replace-1",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    8,
				Status:      domain.GoalStatusInProgress,
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "batch-user-replace-1",
				GoalID:      "goal-3",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				Progress:    20,
				Status:      domain.GoalStatusCompleted,
				IsActive:    true,
				AssignedAt:  &now,
			},
		}

		err := repo.BulkInsert(ctx, initialProgresses)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}

		// Step 1: Deactivate all goals (simulate replace mode step 1)
		deactivateProgresses := []*domain.UserGoalProgress{
			{
				UserID:      "batch-user-replace-1",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    false,
				AssignedAt:  nil,
			},
			{
				UserID:      "batch-user-replace-1",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    false,
				AssignedAt:  nil,
			},
			{
				UserID:      "batch-user-replace-1",
				GoalID:      "goal-3",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    false,
				AssignedAt:  nil,
			},
		}

		err = repo.BatchUpsertGoalActive(ctx, deactivateProgresses)
		if err != nil {
			t.Fatalf("Deactivate failed: %v", err)
		}

		// Verify all inactive
		afterDeactivate, err := repo.GetUserProgress(ctx, "batch-user-replace-1", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		for _, p := range afterDeactivate {
			if p.IsActive {
				t.Errorf("Goal %s should be inactive after deactivation", p.GoalID)
			}
		}

		// Step 2: Reactivate some goals (simulate replace mode step 2)
		reactivateNow := time.Now()
		reactivateProgresses := []*domain.UserGoalProgress{
			{
				UserID:      "batch-user-replace-1",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &reactivateNow,
			},
			{
				UserID:      "batch-user-replace-1",
				GoalID:      "goal-3",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &reactivateNow,
			},
		}

		err = repo.BatchUpsertGoalActive(ctx, reactivateProgresses)
		if err != nil {
			t.Fatalf("Reactivate failed: %v", err)
		}

		// Verify: goal-1 and goal-3 active, goal-2 inactive, all progress preserved
		afterReactivate, err := repo.GetUserProgress(ctx, "batch-user-replace-1", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(afterReactivate) != 3 {
			t.Errorf("Expected 3 records, got %d", len(afterReactivate))
		}

		for _, p := range afterReactivate {
			switch p.GoalID {
			case "goal-1":
				if !p.IsActive {
					t.Errorf("Goal goal-1 should be active after reactivation")
				}
				if p.Progress != 15 {
					t.Errorf("Goal goal-1 progress should be preserved (15), got %d", p.Progress)
				}
				if p.Status != domain.GoalStatusInProgress {
					t.Errorf("Goal goal-1 status should be preserved, got %s", p.Status)
				}
			case "goal-2":
				if p.IsActive {
					t.Errorf("Goal goal-2 should remain inactive")
				}
				if p.Progress != 8 {
					t.Errorf("Goal goal-2 progress should be preserved (8), got %d", p.Progress)
				}
			case "goal-3":
				if !p.IsActive {
					t.Errorf("Goal goal-3 should be active after reactivation")
				}
				if p.Progress != 20 {
					t.Errorf("Goal goal-3 progress should be preserved (20), got %d", p.Progress)
				}
				if p.Status != domain.GoalStatusCompleted {
					t.Errorf("Goal goal-3 status should be preserved (completed), got %s", p.Status)
				}
			}
		}
	})

	t.Run("handles empty slice without error", func(t *testing.T) {
		// Execute: BatchUpsertGoalActive([])
		err := repo.BatchUpsertGoalActive(ctx, []*domain.UserGoalProgress{})
		if err != nil {
			t.Fatalf("BatchUpsertGoalActive with empty slice should not error, got: %v", err)
		}
	})

	t.Run("performance: 10 goals < 20ms", func(t *testing.T) {
		// Setup: 10 goals
		now := time.Now()
		progresses := make([]*domain.UserGoalProgress, 10)
		for i := 0; i < 10; i++ {
			progresses[i] = &domain.UserGoalProgress{
				UserID:      "batch-perf-user",
				GoalID:      fmt.Sprintf("goal-%d", i),
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		// Execute: BatchUpsertGoalActive
		start := time.Now()
		err := repo.BatchUpsertGoalActive(ctx, progresses)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("BatchUpsertGoalActive failed: %v", err)
		}

		// Verify: Performance < 20ms
		if elapsed > 20*time.Millisecond {
			t.Errorf("BatchUpsertGoalActive(10) took %v, expected < 20ms", elapsed)
		} else {
			t.Logf("BatchUpsertGoalActive(10) took %v ✓", elapsed)
		}
	})
}

func TestPostgresTxRepository_BatchUpsertGoalActive(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("activates goals within transaction", func(t *testing.T) {
		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Execute: BatchUpsertGoalActive in transaction
		now := time.Now()
		progresses := []*domain.UserGoalProgress{
			{
				UserID:      "tx-batch-user",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
			{
				UserID:      "tx-batch-user",
				GoalID:      "goal-2",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
		}

		err = tx.BatchUpsertGoalActive(ctx, progresses)
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("BatchUpsertGoalActive failed: %v", err)
		}

		// Commit
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify: Goals are active
		result, err := repo.GetUserProgress(ctx, "tx-batch-user", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("Expected 2 records, got %d", len(result))
		}

		for _, p := range result {
			if !p.IsActive {
				t.Errorf("Goal %s should be active", p.GoalID)
			}
		}
	})

	t.Run("rollback undoes batch activation", func(t *testing.T) {
		// Begin transaction
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		// Execute: BatchUpsertGoalActive in transaction
		now := time.Now()
		progresses := []*domain.UserGoalProgress{
			{
				UserID:      "tx-rollback-user",
				GoalID:      "goal-1",
				ChallengeID: "challenge-1",
				Namespace:   "test",
				IsActive:    true,
				AssignedAt:  &now,
			},
		}

		err = tx.BatchUpsertGoalActive(ctx, progresses)
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("BatchUpsertGoalActive failed: %v", err)
		}

		// Rollback
		err = tx.Rollback()
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify: No goals exist
		result, err := repo.GetUserProgress(ctx, "tx-rollback-user", false)
		if err != nil {
			t.Fatalf("GetUserProgress failed: %v", err)
		}

		if len(result) != 0 {
			t.Errorf("Expected 0 records after rollback, got %d", len(result))
		}
	})
}
