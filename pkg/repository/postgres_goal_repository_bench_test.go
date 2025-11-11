package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	_ "github.com/lib/pq"
)

// BenchmarkBatchUpsertProgressWithCOPY_AssignmentControl benchmarks M3 assignment control
// in the BatchUpsertProgressWithCOPY query. Tests that inactive goals are skipped efficiently.
func BenchmarkBatchUpsertProgressWithCOPY_AssignmentControl(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	db := setupTestDBForBench(b)
	if db == nil {
		return
	}
	defer cleanupTestDBForBench(b, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	// Setup: Create 1,000 goals (500 active, 500 inactive)
	setupGoals := make([]*domain.UserGoalProgress, 1000)
	for i := 0; i < 1000; i++ {
		isActive := i%2 == 0 // Every other goal is active
		now := time.Now()
		setupGoals[i] = &domain.UserGoalProgress{
			UserID:      fmt.Sprintf("bench-user-%d", i),
			GoalID:      fmt.Sprintf("bench-goal-%d", i),
			ChallengeID: "bench-challenge",
			Namespace:   "test",
			Progress:    0,
			Status:      domain.GoalStatusInProgress,
			IsActive:    isActive,
			AssignedAt:  &now,
		}
	}

	err := repo.BatchUpsertProgressWithCOPY(ctx, setupGoals)
	if err != nil {
		b.Fatalf("Setup failed: %v", err)
	}

	// Benchmark: Update all 1,000 goals (only 500 active should update)
	updateGoals := make([]*domain.UserGoalProgress, 1000)
	for i := 0; i < 1000; i++ {
		updateGoals[i] = &domain.UserGoalProgress{
			UserID:      fmt.Sprintf("bench-user-%d", i),
			GoalID:      fmt.Sprintf("bench-goal-%d", i),
			ChallengeID: "bench-challenge",
			Namespace:   "test",
			Progress:    10,
			Status:      domain.GoalStatusCompleted,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := repo.BatchUpsertProgressWithCOPY(ctx, updateGoals)
		if err != nil {
			b.Fatalf("Batch update failed: %v", err)
		}

		// Verify after first run only to check assignment control works
		// Note: nestif disabled for benchmark verification code to keep readable
		if i == 0 { //nolint:nestif
			var activeUpdated, inactiveNotUpdated int
			for j := 0; j < 1000; j++ {
				result, err := repo.GetProgress(ctx, fmt.Sprintf("bench-user-%d", j), fmt.Sprintf("bench-goal-%d", j))
				if err != nil {
					b.Fatalf("GetProgress failed: %v", err)
				}

				isActive := j%2 == 0
				if isActive && result.Progress == 10 {
					activeUpdated++
				} else if !isActive && result.Progress == 0 {
					inactiveNotUpdated++
				}
			}

			if activeUpdated != 500 {
				b.Errorf("Active goals updated = %d, want 500", activeUpdated)
			}
			if inactiveNotUpdated != 500 {
				b.Errorf("Inactive goals NOT updated = %d, want 500", inactiveNotUpdated)
			}
		}
	}
	b.StopTimer()

	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1000000, "ms/op")
}

// BenchmarkBatchIncrementProgress_AssignmentControl benchmarks M3 assignment control
// in the BatchIncrementProgress query using UNNEST pattern.
func BenchmarkBatchIncrementProgress_AssignmentControl(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	db := setupTestDBForBench(b)
	if db == nil {
		return
	}
	defer cleanupTestDBForBench(b, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	// Setup: Create 1,000 goals (500 active, 500 inactive)
	setupGoals := make([]*domain.UserGoalProgress, 1000)
	for i := 0; i < 1000; i++ {
		isActive := i%2 == 0
		now := time.Now()
		setupGoals[i] = &domain.UserGoalProgress{
			UserID:      fmt.Sprintf("bench-user-%d", i),
			GoalID:      fmt.Sprintf("bench-goal-%d", i),
			ChallengeID: "bench-challenge",
			Namespace:   "test",
			Progress:    5,
			Status:      domain.GoalStatusInProgress,
			IsActive:    isActive,
			AssignedAt:  &now,
		}
	}

	err := repo.BatchUpsertProgressWithCOPY(ctx, setupGoals)
	if err != nil {
		b.Fatalf("Setup failed: %v", err)
	}

	// Benchmark: Increment all 1,000 goals by 3 (only 500 active should increment)
	incrementGoals := make([]ProgressIncrement, 1000)
	for i := 0; i < 1000; i++ {
		incrementGoals[i] = ProgressIncrement{
			UserID:      fmt.Sprintf("bench-user-%d", i),
			GoalID:      fmt.Sprintf("bench-goal-%d", i),
			ChallengeID: "bench-challenge",
			Namespace:   "test",
			Delta:       3, // Increment by 3
			TargetValue: 10,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := repo.BatchIncrementProgress(ctx, incrementGoals)
		if err != nil {
			b.Fatalf("Batch increment failed: %v", err)
		}

		// Verify after first run only to check assignment control works
		// Note: nestif disabled for benchmark verification code to keep readable
		if i == 0 { //nolint:nestif
			var activeIncremented, inactiveNotIncremented int
			expectedActiveProgress := 5 + 3 // Initial 5 + first increment of 3
			for j := 0; j < 1000; j++ {
				result, err := repo.GetProgress(ctx, fmt.Sprintf("bench-user-%d", j), fmt.Sprintf("bench-goal-%d", j))
				if err != nil {
					b.Fatalf("GetProgress failed: %v", err)
				}

				isActive := j%2 == 0
				if isActive && result.Progress == expectedActiveProgress {
					activeIncremented++
				} else if !isActive && result.Progress == 5 {
					inactiveNotIncremented++
				}
			}

			if activeIncremented != 500 {
				b.Errorf("Active goals incremented = %d, want 500 (expected progress: %d)", activeIncremented, expectedActiveProgress)
			}
			if inactiveNotIncremented != 500 {
				b.Errorf("Inactive goals NOT incremented = %d, want 500", inactiveNotIncremented)
			}
		}
	}
	b.StopTimer()

	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1000000, "ms/op")
}

// BenchmarkIncrementProgress_AssignmentControl benchmarks M3 assignment control
// in the single-row IncrementProgress query.
func BenchmarkIncrementProgress_AssignmentControl(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	db := setupTestDBForBench(b)
	if db == nil {
		return
	}
	defer cleanupTestDBForBench(b, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	// Setup: Create 2 goals (1 active, 1 inactive)
	now := time.Now()
	activeGoal := &domain.UserGoalProgress{
		UserID:      "bench-user-active",
		GoalID:      "bench-goal-active",
		ChallengeID: "bench-challenge",
		Namespace:   "test",
		Progress:    0,
		Status:      domain.GoalStatusInProgress,
		IsActive:    true,
		AssignedAt:  &now,
	}
	inactiveGoal := &domain.UserGoalProgress{
		UserID:      "bench-user-inactive",
		GoalID:      "bench-goal-inactive",
		ChallengeID: "bench-challenge",
		Namespace:   "test",
		Progress:    0,
		Status:      domain.GoalStatusInProgress,
		IsActive:    false,
		AssignedAt:  &now,
	}

	err := repo.UpsertProgress(ctx, activeGoal)
	if err != nil {
		b.Fatalf("Setup active goal failed: %v", err)
	}
	err = repo.UpsertProgress(ctx, inactiveGoal)
	if err != nil {
		b.Fatalf("Setup inactive goal failed: %v", err)
	}

	// Benchmark: Increment active goal
	b.Run("ActiveGoal", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := repo.IncrementProgress(ctx, "bench-user-active", "bench-goal-active", "bench-challenge", "test", 1, 100, false)
			if err != nil {
				b.Fatalf("IncrementProgress failed: %v", err)
			}
		}
		b.StopTimer()

		// Verify: Active goal was incremented
		result, err := repo.GetProgress(ctx, "bench-user-active", "bench-goal-active")
		if err != nil {
			b.Fatalf("GetProgress failed: %v", err)
		}
		if result.Progress != b.N {
			b.Errorf("Active goal progress = %d, want %d", result.Progress, b.N)
		}

		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1000000, "ms/op")
	})

	// Benchmark: Try to increment inactive goal (should be no-op)
	b.Run("InactiveGoal", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := repo.IncrementProgress(ctx, "bench-user-inactive", "bench-goal-inactive", "bench-challenge", "test", 1, 100, false)
			if err != nil {
				b.Fatalf("IncrementProgress failed: %v", err)
			}
		}
		b.StopTimer()

		// Verify: Inactive goal was NOT incremented
		result, err := repo.GetProgress(ctx, "bench-user-inactive", "bench-goal-inactive")
		if err != nil {
			b.Fatalf("GetProgress failed: %v", err)
		}
		if result.Progress != 0 {
			b.Errorf("Inactive goal progress = %d, want 0 (should not increment)", result.Progress)
		}

		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1000000, "ms/op")
	})
}

// BenchmarkBatchUpsertProgressWithCOPY_Baseline benchmarks the COPY protocol baseline
// performance without assignment control concerns (all goals active).
func BenchmarkBatchUpsertProgressWithCOPY_Baseline(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	db := setupTestDBForBench(b)
	if db == nil {
		return
	}
	defer cleanupTestDBForBench(b, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	sizes := []int{100, 500, 1000, 5000, 10000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			// Setup: Create N goals (all active)
			setupGoals := make([]*domain.UserGoalProgress, size)
			for i := 0; i < size; i++ {
				now := time.Now()
				setupGoals[i] = &domain.UserGoalProgress{
					UserID:      fmt.Sprintf("baseline-user-%d", i),
					GoalID:      fmt.Sprintf("baseline-goal-%d", i),
					ChallengeID: "baseline-challenge",
					Namespace:   "test",
					Progress:    0,
					Status:      domain.GoalStatusInProgress,
					IsActive:    true,
					AssignedAt:  &now,
				}
			}

			err := repo.BatchUpsertProgressWithCOPY(ctx, setupGoals)
			if err != nil {
				b.Fatalf("Setup failed: %v", err)
			}

			// Benchmark: Update all N goals
			updateGoals := make([]*domain.UserGoalProgress, size)
			for i := 0; i < size; i++ {
				updateGoals[i] = &domain.UserGoalProgress{
					UserID:      fmt.Sprintf("baseline-user-%d", i),
					GoalID:      fmt.Sprintf("baseline-goal-%d", i),
					ChallengeID: "baseline-challenge",
					Namespace:   "test",
					Progress:    10,
					Status:      domain.GoalStatusCompleted,
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := repo.BatchUpsertProgressWithCOPY(ctx, updateGoals)
				if err != nil {
					b.Fatalf("Batch update failed: %v", err)
				}
			}
			b.StopTimer()

			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1000000, "ms/op")
			b.ReportMetric(float64(size)*1000000000/float64(b.Elapsed().Nanoseconds())*float64(b.N), "rows/sec")
		})
	}
}

// setupTestDBForBench creates a test database connection for benchmarks
func setupTestDBForBench(b *testing.B) *sql.DB {
	b.Helper()

	// Use the same DSN as tests - postgres container should be running on port 5433
	const benchDSN = "postgres://postgres:postgres@localhost:5433/challenge_db?sslmode=disable"

	db, err := sql.Open("postgres", benchDSN)
	if err != nil {
		b.Skipf("Skipping benchmark: cannot connect to database: %v", err)
		return nil
	}

	// Check if database is available
	if err := db.Ping(); err != nil {
		b.Skipf("Skipping benchmark: database not available: %v", err)
		return nil
	}

	return db
}

// cleanupTestDBForBench cleans up the benchmark database
func cleanupTestDBForBench(b *testing.B, db *sql.DB) {
	b.Helper()

	if db == nil {
		return
	}

	// Clean up data
	_, err := db.Exec("TRUNCATE TABLE user_goal_progress")
	if err != nil {
		b.Logf("Warning: failed to truncate table: %v", err)
	}

	_ = db.Close()
}
