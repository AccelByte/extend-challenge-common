package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// BenchmarkBatchIncrementCrossoverPoint benchmarks BatchIncrementProgress at various sizes
// to find the exact crossover point where optimization would help.
// Based on M2 load test data, production uses ~50-60 records per flush.
func BenchmarkBatchIncrementCrossoverPoint(b *testing.B) {
	db := setupM3BenchDB(b)
	if db == nil {
		return
	}
	defer cleanupM3BenchDB(b, db)

	repo := NewPostgresGoalRepository(db)
	ctx := context.Background()

	// Test sizes around the production range (50-60 records)
	batchSizes := []int{10, 25, 50, 60, 75, 100, 150, 200}

	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			// Setup: Create goals
			setupGoals := make([]*domain.UserGoalProgress, size)
			for i := 0; i < size; i++ {
				now := time.Now()
				setupGoals[i] = &domain.UserGoalProgress{
					UserID:      fmt.Sprintf("cross-user-%d-%d", size, i),
					GoalID:      fmt.Sprintf("cross-goal-%d-%d", size, i),
					ChallengeID: "cross-challenge",
					Namespace:   "test",
					Progress:    5,
					Status:      domain.GoalStatusInProgress,
					IsActive:    true,
					AssignedAt:  &now,
				}
			}

			err := repo.BatchUpsertProgressWithCOPY(ctx, setupGoals)
			if err != nil {
				b.Fatalf("Setup failed: %v", err)
			}

			// Benchmark: Increment
			increments := make([]ProgressIncrement, size)
			for i := 0; i < size; i++ {
				increments[i] = ProgressIncrement{
					UserID:      fmt.Sprintf("cross-user-%d-%d", size, i),
					GoalID:      fmt.Sprintf("cross-goal-%d-%d", size, i),
					ChallengeID: "cross-challenge",
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
			b.ReportMetric(msPerOp/float64(size), "ms/row")
		})
	}
}
