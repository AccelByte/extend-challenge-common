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

// intPtr is a helper to create *int values for CopyRow.Progress
func intPtr(v int) *int {
	return &v
}

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

	// Use BulkInsertWithCOPY instead of BatchUpsertProgressWithCOPY because the latter
	// only UPDATES existing rows (M3 Phase 9 lazy materialization) - it won't create new rows
	err := repo.BulkInsertWithCOPY(ctx, setupGoals)
	if err != nil {
		b.Fatalf("Setup failed: %v", err)
	}

	// Benchmark: Update all 1,000 goals (only 500 active should update)
	updateRows := make([]CopyRow, 1000)
	for i := 0; i < 1000; i++ {
		updateRows[i] = CopyRow{
			UserID:       fmt.Sprintf("bench-user-%d", i),
			GoalID:       fmt.Sprintf("bench-goal-%d", i),
			ChallengeID:  "bench-challenge",
			Namespace:    "test",
			Progress:     intPtr(10),
			ProgressMode: "absolute",
			TargetValue:  10,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := repo.BatchUpsertProgressWithCOPY(ctx, updateRows)
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
				if result == nil {
					b.Fatalf("GetProgress returned nil for user %d goal %d", j, j)
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
			// Setup: Create N goals (all active) using BulkInsertWithCOPY
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

			err := repo.BulkInsertWithCOPY(ctx, setupGoals)
			if err != nil {
				b.Fatalf("Setup failed: %v", err)
			}

			// Benchmark: Update all N goals
			updateRows := make([]CopyRow, size)
			for i := 0; i < size; i++ {
				updateRows[i] = CopyRow{
					UserID:       fmt.Sprintf("baseline-user-%d", i),
					GoalID:       fmt.Sprintf("baseline-goal-%d", i),
					ChallengeID:  "baseline-challenge",
					Namespace:    "test",
					Progress:     intPtr(10),
					ProgressMode: "absolute",
					TargetValue:  10,
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := repo.BatchUpsertProgressWithCOPY(ctx, updateRows)
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

// BenchmarkBulkInsert_OLD benchmarks the old BulkInsert implementation (parameterized INSERT).
// This serves as a baseline for comparison with the COPY protocol implementation.
func BenchmarkBulkInsert_OLD(b *testing.B) {
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

	// Test with 10 records (typical initialization use case)
	b.Run("10_records", func(b *testing.B) {
		records := make([]*domain.UserGoalProgress, 10)
		for i := 0; i < 10; i++ {
			now := time.Now()
			records[i] = &domain.UserGoalProgress{
				UserID:      fmt.Sprintf("old-user-%d", i),
				GoalID:      fmt.Sprintf("old-goal-%d", i),
				ChallengeID: "old-challenge",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Update user IDs to avoid conflicts between iterations
			for j := 0; j < 10; j++ {
				records[j].UserID = fmt.Sprintf("old-user-%d-%d", j, i)
			}

			err := repo.BulkInsert(ctx, records)
			if err != nil {
				b.Fatalf("BulkInsert failed: %v", err)
			}
		}
		b.StopTimer()

		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1000000, "ms/op")
	})

	// Clean between subtests
	_, _ = db.Exec("TRUNCATE TABLE user_goal_progress")

	// Test with 100 records (stress test)
	b.Run("100_records", func(b *testing.B) {
		records := make([]*domain.UserGoalProgress, 100)
		for i := 0; i < 100; i++ {
			now := time.Now()
			records[i] = &domain.UserGoalProgress{
				UserID:      fmt.Sprintf("old-user-%d", i),
				GoalID:      fmt.Sprintf("old-goal-%d", i),
				ChallengeID: "old-challenge",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Update user IDs to avoid conflicts between iterations
			for j := 0; j < 100; j++ {
				records[j].UserID = fmt.Sprintf("old-user-%d-%d", j, i)
			}

			err := repo.BulkInsert(ctx, records)
			if err != nil {
				b.Fatalf("BulkInsert failed: %v", err)
			}
		}
		b.StopTimer()

		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1000000, "ms/op")
	})
}

// BenchmarkBulkInsertWithCOPY_NEW benchmarks the new COPY protocol implementation.
// Expected to be 3-5x faster than BulkInsert (old).
func BenchmarkBulkInsertWithCOPY_NEW(b *testing.B) {
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

	// Test with 10 records (typical initialization use case)
	b.Run("10_records", func(b *testing.B) {
		records := make([]*domain.UserGoalProgress, 10)
		for i := 0; i < 10; i++ {
			now := time.Now()
			records[i] = &domain.UserGoalProgress{
				UserID:      fmt.Sprintf("new-user-%d", i),
				GoalID:      fmt.Sprintf("new-goal-%d", i),
				ChallengeID: "new-challenge",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Update user IDs to avoid conflicts between iterations
			for j := 0; j < 10; j++ {
				records[j].UserID = fmt.Sprintf("new-user-%d-%d", j, i)
			}

			err := repo.BulkInsertWithCOPY(ctx, records)
			if err != nil {
				b.Fatalf("BulkInsertWithCOPY failed: %v", err)
			}
		}
		b.StopTimer()

		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1000000, "ms/op")
	})

	// Clean between subtests
	_, _ = db.Exec("TRUNCATE TABLE user_goal_progress")

	// Test with 100 records (stress test)
	b.Run("100_records", func(b *testing.B) {
		records := make([]*domain.UserGoalProgress, 100)
		for i := 0; i < 100; i++ {
			now := time.Now()
			records[i] = &domain.UserGoalProgress{
				UserID:      fmt.Sprintf("new-user-%d", i),
				GoalID:      fmt.Sprintf("new-goal-%d", i),
				ChallengeID: "new-challenge",
				Namespace:   "test",
				Progress:    0,
				Status:      domain.GoalStatusNotStarted,
				IsActive:    true,
				AssignedAt:  &now,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Update user IDs to avoid conflicts between iterations
			for j := 0; j < 100; j++ {
				records[j].UserID = fmt.Sprintf("new-user-%d-%d", j, i)
			}

			err := repo.BulkInsertWithCOPY(ctx, records)
			if err != nil {
				b.Fatalf("BulkInsertWithCOPY failed: %v", err)
			}
		}
		b.StopTimer()

		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1000000, "ms/op")
	})
}
