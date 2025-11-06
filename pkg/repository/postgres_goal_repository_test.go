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

const testDSN = "postgres://postgres:test@localhost:5432/postgres?sslmode=disable"

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

	t.Run("batch insert multiple progress records", func(t *testing.T) {
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

		// Verify all records were inserted
		progress1, _ := repo.GetProgress(ctx, "user1", "goal1")
		if progress1 == nil || progress1.Progress != 5 {
			t.Error("user1/goal1 not inserted correctly")
		}

		progress2, _ := repo.GetProgress(ctx, "user1", "goal2")
		if progress2 == nil || progress2.Progress != 10 {
			t.Error("user1/goal2 not inserted correctly")
		}

		progress3, _ := repo.GetProgress(ctx, "user2", "goal1")
		if progress3 == nil || progress3.Progress != 3 {
			t.Error("user2/goal1 not inserted correctly")
		}
	})

	t.Run("batch update existing records", func(t *testing.T) {
		// Insert initial records
		initial := []*domain.UserGoalProgress{
			{
				UserID:      "user3",
				GoalID:      "goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    1,
				Status:      domain.GoalStatusInProgress,
			},
			{
				UserID:      "user3",
				GoalID:      "goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    2,
				Status:      domain.GoalStatusInProgress,
			},
		}
		err := repo.BatchUpsertProgress(ctx, initial)
		if err != nil {
			t.Fatalf("Initial BatchUpsertProgress failed: %v", err)
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
		err = repo.BatchUpsertProgress(ctx, updates)
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

	t.Run("batch insert multiple progress records with COPY", func(t *testing.T) {
		updates := []*domain.UserGoalProgress{
			{
				UserID:      "copy-user1",
				GoalID:      "copy-goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    5,
				Status:      domain.GoalStatusInProgress,
			},
			{
				UserID:      "copy-user1",
				GoalID:      "copy-goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusCompleted,
			},
			{
				UserID:      "copy-user2",
				GoalID:      "copy-goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    3,
				Status:      domain.GoalStatusInProgress,
			},
		}

		err := repo.BatchUpsertProgressWithCOPY(ctx, updates)
		if err != nil {
			t.Fatalf("BatchUpsertProgressWithCOPY failed: %v", err)
		}

		// Verify all records were inserted
		progress1, _ := repo.GetProgress(ctx, "copy-user1", "copy-goal1")
		if progress1 == nil || progress1.Progress != 5 {
			t.Error("copy-user1/copy-goal1 not inserted correctly")
		}

		progress2, _ := repo.GetProgress(ctx, "copy-user1", "copy-goal2")
		if progress2 == nil || progress2.Progress != 10 {
			t.Error("copy-user1/copy-goal2 not inserted correctly")
		}

		progress3, _ := repo.GetProgress(ctx, "copy-user2", "copy-goal1")
		if progress3 == nil || progress3.Progress != 3 {
			t.Error("copy-user2/copy-goal1 not inserted correctly")
		}
	})

	t.Run("batch update existing records with COPY", func(t *testing.T) {
		// Insert initial records using COPY
		initial := []*domain.UserGoalProgress{
			{
				UserID:      "copy-user3",
				GoalID:      "copy-goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    1,
				Status:      domain.GoalStatusInProgress,
			},
			{
				UserID:      "copy-user3",
				GoalID:      "copy-goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    2,
				Status:      domain.GoalStatusInProgress,
			},
		}
		err := repo.BatchUpsertProgressWithCOPY(ctx, initial)
		if err != nil {
			t.Fatalf("Initial BatchUpsertProgressWithCOPY failed: %v", err)
		}

		// Update records using COPY
		updates := []*domain.UserGoalProgress{
			{
				UserID:      "copy-user3",
				GoalID:      "copy-goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    5,
				Status:      domain.GoalStatusInProgress,
			},
			{
				UserID:      "copy-user3",
				GoalID:      "copy-goal2",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    10,
				Status:      domain.GoalStatusCompleted,
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
		err := repo.BatchUpsertProgressWithCOPY(ctx, []*domain.UserGoalProgress{})
		if err != nil {
			t.Fatalf("Empty BatchUpsertProgressWithCOPY should not error: %v", err)
		}
	})

	t.Run("does not update claimed goals with COPY", func(t *testing.T) {
		// Insert a goal and mark it as claimed
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
			},
		}
		err := repo.BatchUpsertProgressWithCOPY(ctx, initial)
		if err != nil {
			t.Fatalf("Initial insert failed: %v", err)
		}

		// Mark as claimed
		err = repo.MarkAsClaimed(ctx, "copy-user4", "copy-goal1")
		if err != nil {
			t.Fatalf("MarkAsClaimed failed: %v", err)
		}

		// Try to update the claimed goal
		updates := []*domain.UserGoalProgress{
			{
				UserID:      "copy-user4",
				GoalID:      "copy-goal1",
				ChallengeID: "challenge1",
				Namespace:   "test",
				Progress:    20, // Try to change progress
				Status:      domain.GoalStatusInProgress,
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

func TestPostgresGoalRepository_IncrementProgress(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("regular increment - basic increment (delta=1)", func(t *testing.T) {
		err := repo.IncrementProgress(ctx, "user1", "goal1", "challenge1", "test", 1, 10, false)
		if err != nil {
			t.Fatalf("IncrementProgress failed: %v", err)
		}

		progress, _ := repo.GetProgress(ctx, "user1", "goal1")
		if progress == nil {
			t.Fatal("Expected progress to exist")
		}
		if progress.Progress != 1 {
			t.Errorf("Progress = %d, want 1", progress.Progress)
		}
		if progress.Status != domain.GoalStatusInProgress {
			t.Errorf("Status = %s, want %s", progress.Status, domain.GoalStatusInProgress)
		}
	})

	t.Run("regular increment - accumulated delta (delta=5)", func(t *testing.T) {
		// First increment by 3
		err := repo.IncrementProgress(ctx, "user2", "goal2", "challenge1", "test", 3, 10, false)
		if err != nil {
			t.Fatalf("First IncrementProgress failed: %v", err)
		}

		// Second increment by 5
		err = repo.IncrementProgress(ctx, "user2", "goal2", "challenge1", "test", 5, 10, false)
		if err != nil {
			t.Fatalf("Second IncrementProgress failed: %v", err)
		}

		progress, _ := repo.GetProgress(ctx, "user2", "goal2")
		if progress.Progress != 8 {
			t.Errorf("Progress = %d, want 8 (3+5)", progress.Progress)
		}
	})

	t.Run("regular increment - zero delta (no-op)", func(t *testing.T) {
		// Initial increment
		err := repo.IncrementProgress(ctx, "user3", "goal3", "challenge1", "test", 5, 10, false)
		if err != nil {
			t.Fatalf("Initial IncrementProgress failed: %v", err)
		}

		// Zero delta
		err = repo.IncrementProgress(ctx, "user3", "goal3", "challenge1", "test", 0, 10, false)
		if err != nil {
			t.Fatalf("Zero delta IncrementProgress failed: %v", err)
		}

		progress, _ := repo.GetProgress(ctx, "user3", "goal3")
		if progress.Progress != 5 {
			t.Errorf("Progress = %d, want 5 (unchanged)", progress.Progress)
		}
	})

	t.Run("regular increment - overflow beyond target", func(t *testing.T) {
		// Initial progress
		err := repo.IncrementProgress(ctx, "user4", "goal4", "challenge1", "test", 4, 5, false)
		if err != nil {
			t.Fatalf("Initial IncrementProgress failed: %v", err)
		}

		// Increment beyond target (4 + 100 = 104 > 5)
		err = repo.IncrementProgress(ctx, "user4", "goal4", "challenge1", "test", 100, 5, false)
		if err != nil {
			t.Fatalf("Overflow IncrementProgress failed: %v", err)
		}

		progress, _ := repo.GetProgress(ctx, "user4", "goal4")
		if progress.Progress != 104 {
			t.Errorf("Progress = %d, want 104 (allows overflow)", progress.Progress)
		}
		if progress.Status != domain.GoalStatusCompleted {
			t.Errorf("Status = %s, want completed", progress.Status)
		}
	})

	t.Run("regular increment - status transition to completed at threshold", func(t *testing.T) {
		// Start at 8, target 10
		err := repo.IncrementProgress(ctx, "user5", "goal5", "challenge1", "test", 8, 10, false)
		if err != nil {
			t.Fatalf("Initial IncrementProgress failed: %v", err)
		}

		progress, _ := repo.GetProgress(ctx, "user5", "goal5")
		if progress.Status != domain.GoalStatusInProgress {
			t.Errorf("Initial status = %s, want in_progress", progress.Status)
		}

		// Increment by 2 to reach target (8 + 2 = 10)
		err = repo.IncrementProgress(ctx, "user5", "goal5", "challenge1", "test", 2, 10, false)
		if err != nil {
			t.Fatalf("Final IncrementProgress failed: %v", err)
		}

		progress, _ = repo.GetProgress(ctx, "user5", "goal5")
		if progress.Progress != 10 {
			t.Errorf("Progress = %d, want 10", progress.Progress)
		}
		if progress.Status != domain.GoalStatusCompleted {
			t.Errorf("Status = %s, want completed", progress.Status)
		}
		if progress.CompletedAt == nil {
			t.Error("CompletedAt should be set when status becomes completed")
		}
	})

	t.Run("daily increment - first day increment", func(t *testing.T) {
		err := repo.IncrementProgress(ctx, "user6", "goal6", "challenge1", "test", 1, 7, true)
		if err != nil {
			t.Fatalf("First day IncrementProgress failed: %v", err)
		}

		progress, _ := repo.GetProgress(ctx, "user6", "goal6")
		if progress.Progress != 1 {
			t.Errorf("Progress = %d, want 1", progress.Progress)
		}
	})

	t.Run("daily increment - same day no-op (progress unchanged)", func(t *testing.T) {
		// First increment today
		err := repo.IncrementProgress(ctx, "user7", "goal7", "challenge1", "test", 1, 7, true)
		if err != nil {
			t.Fatalf("First increment failed: %v", err)
		}

		progress1, _ := repo.GetProgress(ctx, "user7", "goal7")
		time1 := progress1.UpdatedAt

		// Second increment same day (should be no-op)
		time.Sleep(10 * time.Millisecond) // Small delay to ensure timestamp would change if updated
		err = repo.IncrementProgress(ctx, "user7", "goal7", "challenge1", "test", 1, 7, true)
		if err != nil {
			t.Fatalf("Second increment failed: %v", err)
		}

		progress2, _ := repo.GetProgress(ctx, "user7", "goal7")
		if progress2.Progress != 1 {
			t.Errorf("Progress = %d, want 1 (unchanged)", progress2.Progress)
		}
		// Note: updated_at will change even if progress doesn't (by design - tracks last attempt)
		if !progress2.UpdatedAt.After(time1) {
			t.Error("UpdatedAt should be updated even for same-day no-op")
		}
	})

	t.Run("claimed protection - no update when status=claimed", func(t *testing.T) {
		// Insert and claim progress
		completedTime := time.Now()
		claimedTime := time.Now()
		progress := &domain.UserGoalProgress{
			UserID:      "user8",
			GoalID:      "goal8",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusClaimed,
			CompletedAt: &completedTime,
			ClaimedAt:   &claimedTime,
		}
		err := repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("Initial UpsertProgress failed: %v", err)
		}

		// Try to increment claimed goal
		err = repo.IncrementProgress(ctx, "user8", "goal8", "challenge1", "test", 5, 10, false)
		if err != nil {
			t.Fatalf("IncrementProgress on claimed goal failed: %v", err)
		}

		// Verify it was NOT updated (progress still 10)
		retrieved, _ := repo.GetProgress(ctx, "user8", "goal8")
		if retrieved.Progress != 10 {
			t.Errorf("Progress = %d, want 10 (should not have been updated)", retrieved.Progress)
		}
		if retrieved.Status != domain.GoalStatusClaimed {
			t.Errorf("Status = %s, want claimed", retrieved.Status)
		}
	})
}

func TestPostgresGoalRepository_BatchIncrementProgress(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("batch increment - empty slice (no-op)", func(t *testing.T) {
		err := repo.BatchIncrementProgress(ctx, []ProgressIncrement{})
		if err != nil {
			t.Fatalf("Empty BatchIncrementProgress should not error: %v", err)
		}
	})

	t.Run("batch increment - mixed regular and daily increments", func(t *testing.T) {
		increments := []ProgressIncrement{
			// Regular increments
			{UserID: "user1", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 1, TargetValue: 10, IsDailyIncrement: false},
			{UserID: "user1", GoalID: "goal2", ChallengeID: "challenge1", Namespace: "test", Delta: 5, TargetValue: 10, IsDailyIncrement: false},
			{UserID: "user2", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 3, TargetValue: 5, IsDailyIncrement: false},
			// Daily increments
			{UserID: "user3", GoalID: "goal3", ChallengeID: "challenge1", Namespace: "test", Delta: 1, TargetValue: 7, IsDailyIncrement: true},
			{UserID: "user3", GoalID: "goal4", ChallengeID: "challenge1", Namespace: "test", Delta: 1, TargetValue: 7, IsDailyIncrement: true},
		}

		err := repo.BatchIncrementProgress(ctx, increments)
		if err != nil {
			t.Fatalf("BatchIncrementProgress failed: %v", err)
		}

		// Verify regular increments
		p1, _ := repo.GetProgress(ctx, "user1", "goal1")
		if p1 == nil || p1.Progress != 1 {
			t.Error("user1/goal1 not incremented correctly")
		}

		p2, _ := repo.GetProgress(ctx, "user1", "goal2")
		if p2 == nil || p2.Progress != 5 {
			t.Error("user1/goal2 not incremented correctly")
		}

		p3, _ := repo.GetProgress(ctx, "user2", "goal1")
		if p3 == nil || p3.Progress != 3 {
			t.Error("user2/goal1 not incremented correctly")
		}

		// Verify daily increments
		p4, _ := repo.GetProgress(ctx, "user3", "goal3")
		if p4 == nil || p4.Progress != 1 {
			t.Error("user3/goal3 (daily) not incremented correctly")
		}

		p5, _ := repo.GetProgress(ctx, "user3", "goal4")
		if p5 == nil || p5.Progress != 1 {
			t.Error("user3/goal4 (daily) not incremented correctly")
		}
	})

	t.Run("batch increment - accumulation on existing progress", func(t *testing.T) {
		// Initial increments
		initial := []ProgressIncrement{
			{UserID: "user4", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 3, TargetValue: 10, IsDailyIncrement: false},
			{UserID: "user4", GoalID: "goal2", ChallengeID: "challenge1", Namespace: "test", Delta: 2, TargetValue: 10, IsDailyIncrement: false},
		}
		err := repo.BatchIncrementProgress(ctx, initial)
		if err != nil {
			t.Fatalf("Initial BatchIncrementProgress failed: %v", err)
		}

		// Additional increments
		additional := []ProgressIncrement{
			{UserID: "user4", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 5, TargetValue: 10, IsDailyIncrement: false},
			{UserID: "user4", GoalID: "goal2", ChallengeID: "challenge1", Namespace: "test", Delta: 4, TargetValue: 10, IsDailyIncrement: false},
		}
		err = repo.BatchIncrementProgress(ctx, additional)
		if err != nil {
			t.Fatalf("Additional BatchIncrementProgress failed: %v", err)
		}

		// Verify accumulation
		p1, _ := repo.GetProgress(ctx, "user4", "goal1")
		if p1.Progress != 8 {
			t.Errorf("user4/goal1 progress = %d, want 8 (3+5)", p1.Progress)
		}

		p2, _ := repo.GetProgress(ctx, "user4", "goal2")
		if p2.Progress != 6 {
			t.Errorf("user4/goal2 progress = %d, want 6 (2+4)", p2.Progress)
		}
	})

	t.Run("batch increment - status transitions to completed", func(t *testing.T) {
		// Start with progress near target
		initial := []ProgressIncrement{
			{UserID: "user5", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 8, TargetValue: 10, IsDailyIncrement: false},
			{UserID: "user5", GoalID: "goal2", ChallengeID: "challenge1", Namespace: "test", Delta: 3, TargetValue: 5, IsDailyIncrement: false},
		}
		err := repo.BatchIncrementProgress(ctx, initial)
		if err != nil {
			t.Fatalf("Initial BatchIncrementProgress failed: %v", err)
		}

		// Increment to complete both goals
		completing := []ProgressIncrement{
			{UserID: "user5", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 2, TargetValue: 10, IsDailyIncrement: false}, // 8+2=10
			{UserID: "user5", GoalID: "goal2", ChallengeID: "challenge1", Namespace: "test", Delta: 5, TargetValue: 5, IsDailyIncrement: false},  // 3+5=8>5
		}
		err = repo.BatchIncrementProgress(ctx, completing)
		if err != nil {
			t.Fatalf("Completing BatchIncrementProgress failed: %v", err)
		}

		// Verify both are completed
		p1, _ := repo.GetProgress(ctx, "user5", "goal1")
		if p1.Status != domain.GoalStatusCompleted {
			t.Errorf("user5/goal1 status = %s, want completed", p1.Status)
		}
		if p1.CompletedAt == nil {
			t.Error("user5/goal1 should have completed_at set")
		}

		p2, _ := repo.GetProgress(ctx, "user5", "goal2")
		if p2.Status != domain.GoalStatusCompleted {
			t.Errorf("user5/goal2 status = %s, want completed", p2.Status)
		}
		if p2.CompletedAt == nil {
			t.Error("user5/goal2 should have completed_at set")
		}
	})

	t.Run("batch increment - daily increment same day no-op", func(t *testing.T) {
		// First batch with daily increment
		first := []ProgressIncrement{
			{UserID: "user6", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 1, TargetValue: 7, IsDailyIncrement: true},
		}
		err := repo.BatchIncrementProgress(ctx, first)
		if err != nil {
			t.Fatalf("First BatchIncrementProgress failed: %v", err)
		}

		p1, _ := repo.GetProgress(ctx, "user6", "goal1")
		initialProgress := p1.Progress

		// Second batch same day (should be no-op)
		second := []ProgressIncrement{
			{UserID: "user6", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 1, TargetValue: 7, IsDailyIncrement: true},
		}
		err = repo.BatchIncrementProgress(ctx, second)
		if err != nil {
			t.Fatalf("Second BatchIncrementProgress failed: %v", err)
		}

		p2, _ := repo.GetProgress(ctx, "user6", "goal1")
		if p2.Progress != initialProgress {
			t.Errorf("Progress = %d, want %d (same day no-op)", p2.Progress, initialProgress)
		}
	})

	t.Run("batch increment - large batch (100 increments)", func(t *testing.T) {
		increments := make([]ProgressIncrement, 100)
		for i := 0; i < 100; i++ {
			increments[i] = ProgressIncrement{
				UserID:           "batchuser",
				GoalID:           fmt.Sprintf("goal%d", i),
				ChallengeID:      "challenge1",
				Namespace:        "test",
				Delta:            1,
				TargetValue:      10,
				IsDailyIncrement: i%2 == 0, // Alternating regular/daily
			}
		}

		err := repo.BatchIncrementProgress(ctx, increments)
		if err != nil {
			t.Fatalf("Large BatchIncrementProgress failed: %v", err)
		}

		// Verify a few random goals were created
		p1, _ := repo.GetProgress(ctx, "batchuser", "goal0")
		if p1 == nil || p1.Progress != 1 {
			t.Error("goal0 not created correctly")
		}

		p50, _ := repo.GetProgress(ctx, "batchuser", "goal50")
		if p50 == nil || p50.Progress != 1 {
			t.Error("goal50 not created correctly")
		}

		p99, _ := repo.GetProgress(ctx, "batchuser", "goal99")
		if p99 == nil || p99.Progress != 1 {
			t.Error("goal99 not created correctly")
		}
	})

	t.Run("batch increment - claimed protection", func(t *testing.T) {
		// Insert claimed goal
		completedTime := time.Now()
		claimedTime := time.Now()
		progress := &domain.UserGoalProgress{
			UserID:      "user7",
			GoalID:      "goal1",
			ChallengeID: "challenge1",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusClaimed,
			CompletedAt: &completedTime,
			ClaimedAt:   &claimedTime,
		}
		err := repo.UpsertProgress(ctx, progress)
		if err != nil {
			t.Fatalf("Initial UpsertProgress failed: %v", err)
		}

		// Try to increment claimed goal in batch
		increments := []ProgressIncrement{
			{UserID: "user7", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 5, TargetValue: 10, IsDailyIncrement: false},
		}
		err = repo.BatchIncrementProgress(ctx, increments)
		if err != nil {
			t.Fatalf("BatchIncrementProgress failed: %v", err)
		}

		// Verify it was NOT updated
		retrieved, _ := repo.GetProgress(ctx, "user7", "goal1")
		if retrieved.Progress != 10 {
			t.Errorf("Progress = %d, want 10 (claimed goals should not be updated)", retrieved.Progress)
		}
	})
}

func TestPostgresTxRepository_IncrementProgress(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("transaction - regular increment", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		err = tx.IncrementProgress(ctx, "txuser1", "goal1", "challenge1", "test", 5, 10, false)
		if err != nil {
			t.Fatalf("IncrementProgress in tx failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify increment persisted
		progress, _ := repo.GetProgress(ctx, "txuser1", "goal1")
		if progress == nil || progress.Progress != 5 {
			t.Error("Increment in transaction did not persist correctly")
		}
	})

	t.Run("transaction - daily increment", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		err = tx.IncrementProgress(ctx, "txuser2", "goal2", "challenge1", "test", 1, 7, true)
		if err != nil {
			t.Fatalf("IncrementProgress (daily) in tx failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify increment persisted
		progress, _ := repo.GetProgress(ctx, "txuser2", "goal2")
		if progress == nil || progress.Progress != 1 {
			t.Error("Daily increment in transaction did not persist correctly")
		}
	})

	t.Run("transaction - rollback discards increment", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		err = tx.IncrementProgress(ctx, "txuser3", "goal3", "challenge1", "test", 10, 10, false)
		if err != nil {
			t.Fatalf("IncrementProgress in tx failed: %v", err)
		}

		err = tx.Rollback()
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify increment was discarded
		progress, _ := repo.GetProgress(ctx, "txuser3", "goal3")
		if progress != nil {
			t.Errorf("Increment should have been discarded after rollback, got progress=%d", progress.Progress)
		}
	})
}

func TestPostgresTxRepository_BatchIncrementProgress(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	t.Run("transaction - batch increment commit", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		increments := []ProgressIncrement{
			{UserID: "txuser4", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 3, TargetValue: 10, IsDailyIncrement: false},
			{UserID: "txuser4", GoalID: "goal2", ChallengeID: "challenge1", Namespace: "test", Delta: 1, TargetValue: 7, IsDailyIncrement: true},
			{UserID: "txuser5", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 5, TargetValue: 10, IsDailyIncrement: false},
		}

		err = tx.BatchIncrementProgress(ctx, increments)
		if err != nil {
			t.Fatalf("BatchIncrementProgress in tx failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify all increments persisted
		p1, _ := repo.GetProgress(ctx, "txuser4", "goal1")
		if p1 == nil || p1.Progress != 3 {
			t.Error("txuser4/goal1 not incremented correctly in transaction")
		}

		p2, _ := repo.GetProgress(ctx, "txuser4", "goal2")
		if p2 == nil || p2.Progress != 1 {
			t.Error("txuser4/goal2 (daily) not incremented correctly in transaction")
		}

		p3, _ := repo.GetProgress(ctx, "txuser5", "goal1")
		if p3 == nil || p3.Progress != 5 {
			t.Error("txuser5/goal1 not incremented correctly in transaction")
		}
	})

	t.Run("transaction - batch increment rollback", func(t *testing.T) {
		tx, err := repo.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx failed: %v", err)
		}

		increments := []ProgressIncrement{
			{UserID: "txuser6", GoalID: "goal1", ChallengeID: "challenge1", Namespace: "test", Delta: 3, TargetValue: 10, IsDailyIncrement: false},
			{UserID: "txuser6", GoalID: "goal2", ChallengeID: "challenge1", Namespace: "test", Delta: 5, TargetValue: 10, IsDailyIncrement: false},
		}

		err = tx.BatchIncrementProgress(ctx, increments)
		if err != nil {
			t.Fatalf("BatchIncrementProgress in tx failed: %v", err)
		}

		err = tx.Rollback()
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify batch increments were discarded
		p1, _ := repo.GetProgress(ctx, "txuser6", "goal1")
		if p1 != nil {
			t.Error("Batch increments should have been discarded after rollback")
		}

		p2, _ := repo.GetProgress(ctx, "txuser6", "goal2")
		if p2 != nil {
			t.Error("Batch increments should have been discarded after rollback")
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
