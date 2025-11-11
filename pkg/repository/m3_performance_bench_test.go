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

// BenchmarkM3_BatchUpsertCOPY_MixedActiveInactive benchmarks batch UPSERT performance
// with a realistic mix of 50% active and 50% inactive goals to ensure M3 filtering
// doesn't degrade performance.
func BenchmarkM3_BatchUpsertCOPY_MixedActiveInactive(b *testing.B) {
	db := setupM3BenchDB(b)
	if db == nil {
		return
	}
	defer cleanupM3BenchDB(b, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	batchSizes := []int{100, 500, 1000}
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			// Setup: Create goals with alternating is_active status
			setupGoals := make([]*domain.UserGoalProgress, size)
			for i := 0; i < size; i++ {
				now := time.Now()
				setupGoals[i] = &domain.UserGoalProgress{
					UserID:      fmt.Sprintf("m3-user-%d-%d", size, i),
					GoalID:      fmt.Sprintf("m3-goal-%d-%d", size, i),
					ChallengeID: "m3-challenge",
					Namespace:   "test",
					Progress:    0,
					Status:      domain.GoalStatusInProgress,
					IsActive:    i%2 == 0, // 50% active, 50% inactive
					AssignedAt:  &now,
				}
			}

			err := repo.BatchUpsertProgressWithCOPY(ctx, setupGoals)
			if err != nil {
				b.Fatalf("Setup failed: %v", err)
			}

			// Benchmark: Update all goals (only active should update)
			updateGoals := make([]*domain.UserGoalProgress, size)
			for i := 0; i < size; i++ {
				updateGoals[i] = &domain.UserGoalProgress{
					UserID:      fmt.Sprintf("m3-user-%d-%d", size, i),
					GoalID:      fmt.Sprintf("m3-goal-%d-%d", size, i),
					ChallengeID: "m3-challenge",
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

			// Report performance
			msPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N) / 1000000
			rowsPerSec := float64(size) * 1000000000 / (float64(b.Elapsed().Nanoseconds()) / float64(b.N))

			b.ReportMetric(msPerOp, "ms/op")
			b.ReportMetric(rowsPerSec, "rows/sec")

			// Verify (only check a sample to avoid overhead)
			// Note: nestif disabled for benchmark verification code to keep readable
			if b.N == 1 { //nolint:nestif
				activeCount := 0
				inactiveCount := 0
				for i := 0; i < size; i++ {
					result, err := repo.GetProgress(ctx, fmt.Sprintf("m3-user-%d-%d", size, i), fmt.Sprintf("m3-goal-%d-%d", size, i))
					if err != nil {
						b.Fatalf("GetProgress failed: %v", err)
					}
					if result.IsActive && result.Progress == 10 {
						activeCount++
					} else if !result.IsActive && result.Progress == 0 {
						inactiveCount++
					}
				}

				expectedActive := size / 2
				expectedInactive := size - expectedActive
				if activeCount != expectedActive {
					b.Logf("WARNING: Active goals updated = %d, expected %d", activeCount, expectedActive)
				}
				if inactiveCount != expectedInactive {
					b.Logf("WARNING: Inactive goals NOT updated = %d, expected %d", inactiveCount, expectedInactive)
				}
			}
		})
	}
}

// BenchmarkM3_BatchIncrement_MixedActiveInactive benchmarks batch increment performance
func BenchmarkM3_BatchIncrement_MixedActiveInactive(b *testing.B) {
	db := setupM3BenchDB(b)
	if db == nil {
		return
	}
	defer cleanupM3BenchDB(b, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	batchSizes := []int{100, 500, 1000}
	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			// Setup: Create goals with alternating is_active status
			setupGoals := make([]*domain.UserGoalProgress, size)
			for i := 0; i < size; i++ {
				now := time.Now()
				setupGoals[i] = &domain.UserGoalProgress{
					UserID:      fmt.Sprintf("inc-user-%d-%d", size, i),
					GoalID:      fmt.Sprintf("inc-goal-%d-%d", size, i),
					ChallengeID: "inc-challenge",
					Namespace:   "test",
					Progress:    5,
					Status:      domain.GoalStatusInProgress,
					IsActive:    i%2 == 0, // 50% active, 50% inactive
					AssignedAt:  &now,
				}
			}

			err := repo.BatchUpsertProgressWithCOPY(ctx, setupGoals)
			if err != nil {
				b.Fatalf("Setup failed: %v", err)
			}

			// Benchmark: Increment all goals (only active should increment)
			increments := make([]ProgressIncrement, size)
			for i := 0; i < size; i++ {
				increments[i] = ProgressIncrement{
					UserID:      fmt.Sprintf("inc-user-%d-%d", size, i),
					GoalID:      fmt.Sprintf("inc-goal-%d-%d", size, i),
					ChallengeID: "inc-challenge",
					Namespace:   "test",
					Delta:       3,
					TargetValue: 100,
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := repo.BatchIncrementProgress(ctx, increments)
				if err != nil {
					b.Fatalf("Batch increment failed: %v", err)
				}
			}
			b.StopTimer()

			// Report performance
			msPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N) / 1000000
			rowsPerSec := float64(size) * 1000000000 / (float64(b.Elapsed().Nanoseconds()) / float64(b.N))

			b.ReportMetric(msPerOp, "ms/op")
			b.ReportMetric(rowsPerSec, "rows/sec")
		})
	}
}

// BenchmarkM3_SingleIncrement_ActiveVsInactive compares performance of incrementing active vs inactive goals
func BenchmarkM3_SingleIncrement_ActiveVsInactive(b *testing.B) {
	db := setupM3BenchDB(b)
	if db == nil {
		return
	}
	defer cleanupM3BenchDB(b, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	// Setup active goal
	now := time.Now()
	activeGoal := &domain.UserGoalProgress{
		UserID:      "single-user-active",
		GoalID:      "single-goal-active",
		ChallengeID: "single-challenge",
		Namespace:   "test",
		Progress:    0,
		Status:      domain.GoalStatusInProgress,
		IsActive:    true,
		AssignedAt:  &now,
	}

	// Setup inactive goal
	inactiveGoal := &domain.UserGoalProgress{
		UserID:      "single-user-inactive",
		GoalID:      "single-goal-inactive",
		ChallengeID: "single-challenge",
		Namespace:   "test",
		Progress:    0,
		Status:      domain.GoalStatusInProgress,
		IsActive:    false,
		AssignedAt:  &now,
	}

	err := repo.UpsertProgress(ctx, activeGoal)
	if err != nil {
		b.Fatalf("Setup active failed: %v", err)
	}

	err = repo.UpsertProgress(ctx, inactiveGoal)
	if err != nil {
		b.Fatalf("Setup inactive failed: %v", err)
	}

	// Benchmark active goal increment
	b.Run("ActiveGoal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := repo.IncrementProgress(ctx, "single-user-active", "single-goal-active", "single-challenge", "test", 1, 1000000, false)
			if err != nil {
				b.Fatalf("IncrementProgress failed: %v", err)
			}
		}

		msPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N) / 1000000
		b.ReportMetric(msPerOp, "ms/op")
	})

	// Benchmark inactive goal increment (should be no-op)
	b.Run("InactiveGoal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := repo.IncrementProgress(ctx, "single-user-inactive", "single-goal-inactive", "single-challenge", "test", 1, 1000000, false)
			if err != nil {
				b.Fatalf("IncrementProgress failed: %v", err)
			}
		}

		msPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N) / 1000000
		b.ReportMetric(msPerOp, "ms/op")

		// Verify inactive goal was NOT incremented
		result, err := repo.GetProgress(ctx, "single-user-inactive", "single-goal-inactive")
		if err != nil {
			b.Fatalf("GetProgress failed: %v", err)
		}
		if result.Progress != 0 {
			b.Logf("WARNING: Inactive goal was incremented (progress=%d), should be 0", result.Progress)
		}
	})
}

// setupM3BenchDB creates a test database for M3 benchmarks
func setupM3BenchDB(b *testing.B) *sql.DB {
	b.Helper()

	const benchDSN = "postgres://postgres:postgres@localhost:5433/challenge_db?sslmode=disable"

	db, err := sql.Open("postgres", benchDSN)
	if err != nil {
		b.Skipf("Skipping benchmark: cannot connect to database: %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		b.Skipf("Skipping benchmark: database not available: %v", err)
		return nil
	}

	// Truncate to ensure clean state
	_, err = db.Exec("TRUNCATE TABLE user_goal_progress")
	if err != nil {
		b.Logf("Warning: failed to truncate table: %v", err)
	}

	return db
}

// cleanupM3BenchDB cleans up the benchmark database
func cleanupM3BenchDB(b *testing.B, db *sql.DB) {
	b.Helper()

	if db == nil {
		return
	}

	_, err := db.Exec("TRUNCATE TABLE user_goal_progress")
	if err != nil {
		b.Logf("Warning: failed to truncate table: %v", err)
	}

	_ = db.Close()
}
