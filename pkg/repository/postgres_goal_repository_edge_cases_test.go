package repository

import (
	"context"
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// Edge case tests to increase coverage from 82.6% to 90%+
// Focus on error paths and boundary conditions

// TestPostgresTxRepository_Commit_DoubleCommit tests committing an already-committed transaction
func TestPostgresTxRepository_Commit_DoubleCommit(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}

	// Insert some data
	progress := &domain.UserGoalProgress{
		UserID:      "double-commit-user",
		GoalID:      "double-commit-goal",
		ChallengeID: "double-commit-challenge",
		Namespace:   "test",
		Progress:    10,
		Status:      domain.GoalStatusInProgress,
		IsActive:    true,
	}
	if err := tx.UpsertProgress(ctx, progress); err != nil {
		_ = tx.Rollback()
		t.Fatalf("UpsertProgress failed: %v", err)
	}

	// First commit should succeed
	if err := tx.Commit(); err != nil {
		t.Fatalf("First commit failed: %v", err)
	}

	// Second commit should fail (sql.ErrTxDone)
	err = tx.Commit()
	if err == nil {
		t.Fatal("Expected error when committing already-committed transaction, got nil")
	}
	// Error is expected - transaction already committed
}

// TestPostgresTxRepository_Commit_AfterRollback tests committing after rollback
func TestPostgresTxRepository_Commit_AfterRollback(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}

	// Insert some data
	progress := &domain.UserGoalProgress{
		UserID:      "commit-after-rollback-user",
		GoalID:      "commit-after-rollback-goal",
		ChallengeID: "commit-after-rollback-challenge",
		Namespace:   "test",
		Progress:    10,
		Status:      domain.GoalStatusInProgress,
		IsActive:    true,
	}
	if err := tx.UpsertProgress(ctx, progress); err != nil {
		_ = tx.Rollback()
		t.Fatalf("UpsertProgress failed: %v", err)
	}

	// Rollback
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Commit after rollback should fail
	err = tx.Commit()
	if err == nil {
		t.Fatal("Expected error when committing after rollback, got nil")
	}
	// Error is expected - transaction already rolled back
}

// TestPostgresTxRepository_MarkAsClaimed_NotCompleted tests claiming a non-completed goal
func TestPostgresTxRepository_MarkAsClaimed_NotCompleted(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	// Create a goal that's in_progress (not completed)
	progress := &domain.UserGoalProgress{
		UserID:      "claim-not-completed-user",
		GoalID:      "claim-not-completed-goal",
		ChallengeID: "claim-not-completed-challenge",
		Namespace:   "test",
		Progress:    50,
		Status:      domain.GoalStatusInProgress, // Not completed
		IsActive:    true,
	}
	if err := repo.UpsertProgress(ctx, progress); err != nil {
		t.Fatalf("UpsertProgress failed: %v", err)
	}

	// Try to claim in transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = tx.MarkAsClaimed(ctx, "claim-not-completed-user", "claim-not-completed-goal")
	if err == nil {
		t.Fatal("Expected error when claiming non-completed goal, got nil")
	}
	// Error is expected - goal not completed
}

// TestPostgresTxRepository_MarkAsClaimed_AlreadyClaimed tests claiming an already-claimed goal
func TestPostgresTxRepository_MarkAsClaimed_AlreadyClaimed(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	// Create a completed goal
	completedAt := time.Now()
	progress := &domain.UserGoalProgress{
		UserID:      "claim-already-claimed-user",
		GoalID:      "claim-already-claimed-goal",
		ChallengeID: "claim-already-claimed-challenge",
		Namespace:   "test",
		Progress:    100,
		Status:      domain.GoalStatusCompleted,
		CompletedAt: &completedAt,
		IsActive:    true,
	}
	if err := repo.UpsertProgress(ctx, progress); err != nil {
		t.Fatalf("UpsertProgress failed: %v", err)
	}

	// First claim should succeed
	tx1, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}

	if err := tx1.MarkAsClaimed(ctx, "claim-already-claimed-user", "claim-already-claimed-goal"); err != nil {
		_ = tx1.Rollback()
		t.Fatalf("First claim failed: %v", err)
	}

	if err := tx1.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Second claim should fail (already claimed)
	tx2, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer func() { _ = tx2.Rollback() }()

	err = tx2.MarkAsClaimed(ctx, "claim-already-claimed-user", "claim-already-claimed-goal")
	if err == nil {
		t.Fatal("Expected error when claiming already-claimed goal, got nil")
	}
	// Error is expected - goal already claimed
}

// TestPostgresTxRepository_MarkAsClaimed_NonexistentGoal tests claiming a goal that doesn't exist
func TestPostgresTxRepository_MarkAsClaimed_NonexistentGoal(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = tx.MarkAsClaimed(ctx, "nonexistent-user", "nonexistent-goal")
	if err == nil {
		t.Fatal("Expected error when claiming nonexistent goal, got nil")
	}
	// Error is expected - goal doesn't exist
}

// TestPostgresTxRepository_GetGoalsByIDs_EmptyList tests GetGoalsByIDs with empty list
func TestPostgresTxRepository_GetGoalsByIDs_EmptyList(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	results, err := tx.GetGoalsByIDs(ctx, "any-user", []string{})
	if err != nil {
		t.Fatalf("GetGoalsByIDs with empty list failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty goal IDs, got %d", len(results))
	}
}

// TestPostgresTxRepository_GetGoalsByIDs_NilList tests GetGoalsByIDs with nil list
func TestPostgresTxRepository_GetGoalsByIDs_NilList(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	results, err := tx.GetGoalsByIDs(ctx, "any-user", nil)
	if err != nil {
		t.Fatalf("GetGoalsByIDs with nil list failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results for nil goal IDs, got %d", len(results))
	}
}

// TestPostgresTxRepository_GetGoalsByIDs_PartialMatch tests GetGoalsByIDs with some existing and some missing goals
func TestPostgresTxRepository_GetGoalsByIDs_PartialMatch(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	// Create one goal
	progress := &domain.UserGoalProgress{
		UserID:      "partial-match-user",
		GoalID:      "existing-goal",
		ChallengeID: "partial-match-challenge",
		Namespace:   "test",
		Progress:    10,
		Status:      domain.GoalStatusInProgress,
		IsActive:    true,
	}
	if err := repo.UpsertProgress(ctx, progress); err != nil {
		t.Fatalf("UpsertProgress failed: %v", err)
	}

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Query for both existing and nonexistent goals
	results, err := tx.GetGoalsByIDs(ctx, "partial-match-user", []string{"existing-goal", "nonexistent-goal", "another-missing"})
	if err != nil {
		t.Fatalf("GetGoalsByIDs failed: %v", err)
	}

	// Should return only the existing goal
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].GoalID != "existing-goal" {
		t.Errorf("Expected goal ID 'existing-goal', got '%s'", results[0].GoalID)
	}
}

// TestPostgresTxRepository_BatchUpsertProgressWithCOPY_EmptyBatch tests COPY with empty batch
func TestPostgresTxRepository_BatchUpsertProgressWithCOPY_EmptyBatch(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = tx.BatchUpsertProgressWithCOPY(ctx, []*domain.UserGoalProgress{})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY with empty batch failed: %v", err)
	}
}

// TestPostgresTxRepository_BatchUpsertProgressWithCOPY_NilBatch tests COPY with nil batch
func TestPostgresTxRepository_BatchUpsertProgressWithCOPY_NilBatch(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = tx.BatchUpsertProgressWithCOPY(ctx, nil)
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY with nil batch failed: %v", err)
	}
}

// TestPostgresTxRepository_BatchUpsertProgressWithCOPY_SingleItem tests COPY with single item
func TestPostgresTxRepository_BatchUpsertProgressWithCOPY_SingleItem(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}

	batch := []*domain.UserGoalProgress{
		{
			UserID:      "copy-single-user",
			GoalID:      "copy-single-goal",
			ChallengeID: "copy-single-challenge",
			Namespace:   "test",
			Progress:    15,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
		},
	}

	err = tx.BatchUpsertProgressWithCOPY(ctx, batch)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("BatchUpsertProgressWithCOPY failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify
	result, err := repo.GetProgress(ctx, "copy-single-user", "copy-single-goal")
	if err != nil {
		t.Fatalf("GetProgress failed: %v", err)
	}

	if result.Progress != 15 {
		t.Errorf("Expected progress 15, got %d", result.Progress)
	}
}

// TestPostgresTxRepository_BulkInsertWithCOPY_EmptyBatch tests BulkInsertWithCOPY with empty batch
func TestPostgresTxRepository_BulkInsertWithCOPY_EmptyBatch(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = tx.BulkInsertWithCOPY(ctx, []*domain.UserGoalProgress{})
	if err != nil {
		t.Fatalf("BulkInsertWithCOPY with empty batch failed: %v", err)
	}
}

// TestPostgresTxRepository_BulkInsertWithCOPY_NilBatch tests BulkInsertWithCOPY with nil batch
func TestPostgresTxRepository_BulkInsertWithCOPY_NilBatch(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = tx.BulkInsertWithCOPY(ctx, nil)
	if err != nil {
		t.Fatalf("BulkInsertWithCOPY with nil batch failed: %v", err)
	}
}

// TestPostgresTxRepository_BulkInsertWithCOPY_SingleItem tests BulkInsertWithCOPY with single item
func TestPostgresTxRepository_BulkInsertWithCOPY_SingleItem(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}

	batch := []*domain.UserGoalProgress{
		{
			UserID:      "bulk-copy-single-user",
			GoalID:      "bulk-copy-single-goal",
			ChallengeID: "bulk-copy-single-challenge",
			Namespace:   "test",
			Progress:    0,
			Status:      domain.GoalStatusNotStarted,
			IsActive:    true,
		},
	}

	err = tx.BulkInsertWithCOPY(ctx, batch)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("BulkInsertWithCOPY failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify
	result, err := repo.GetProgress(ctx, "bulk-copy-single-user", "bulk-copy-single-goal")
	if err != nil {
		t.Fatalf("GetProgress failed: %v", err)
	}

	if result.Status != domain.GoalStatusNotStarted {
		t.Errorf("Expected status not_started, got %s", result.Status)
	}
}

// TestPostgresTxRepository_BulkInsertWithCOPY_DuplicateHandling tests BulkInsertWithCOPY handles duplicates with ON CONFLICT DO NOTHING
func TestPostgresTxRepository_BulkInsertWithCOPY_DuplicateHandling(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	batch := []*domain.UserGoalProgress{
		{
			UserID:      "bulk-dup-handling-user",
			GoalID:      "bulk-dup-handling-goal",
			ChallengeID: "bulk-dup-handling-challenge",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusInProgress,
			IsActive:    true,
		},
	}

	// First insert
	tx1, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}

	err = tx1.BulkInsertWithCOPY(ctx, batch)
	if err != nil {
		_ = tx1.Rollback()
		t.Fatalf("First BulkInsertWithCOPY failed: %v", err)
	}

	if err := tx1.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Second insert with different progress value (should be ignored due to ON CONFLICT DO NOTHING)
	batchUpdated := []*domain.UserGoalProgress{
		{
			UserID:      "bulk-dup-handling-user",
			GoalID:      "bulk-dup-handling-goal",
			ChallengeID: "bulk-dup-handling-challenge",
			Namespace:   "test",
			Progress:    99, // Different value - should be ignored
			Status:      domain.GoalStatusCompleted,
			IsActive:    false,
		},
	}

	tx2, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}

	// Should NOT error - ON CONFLICT DO NOTHING
	err = tx2.BulkInsertWithCOPY(ctx, batchUpdated)
	if err != nil {
		_ = tx2.Rollback()
		t.Fatalf("Second BulkInsertWithCOPY failed: %v", err)
	}

	if err := tx2.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify original data is unchanged (ON CONFLICT DO NOTHING preserves original)
	result, err := repo.GetProgress(ctx, "bulk-dup-handling-user", "bulk-dup-handling-goal")
	if err != nil {
		t.Fatalf("GetProgress failed: %v", err)
	}

	if result.Progress != 10 {
		t.Errorf("Expected progress 10 (original value), got %d", result.Progress)
	}

	if result.Status != domain.GoalStatusInProgress {
		t.Errorf("Expected status in_progress (original value), got %s", result.Status)
	}
}

// Tests for non-transaction COPY operations to increase coverage

// TestPostgresGoalRepository_BatchUpsertProgressWithCOPY_EmptyBatch tests non-tx COPY with empty batch
func TestPostgresGoalRepository_BatchUpsertProgressWithCOPY_EmptyBatch(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	err := repo.BatchUpsertProgressWithCOPY(ctx, []*domain.UserGoalProgress{})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY with empty batch failed: %v", err)
	}
}
