// Copyright (c) 2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package rotation

import (
	"testing"
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
	"github.com/stretchr/testify/assert"
)

// --- CalculateLastRotationBoundary ---

func TestCalculateLastRotationBoundary_Daily(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		expected time.Time
	}{
		{
			name:     "mid-day",
			now:      time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "exactly at boundary (midnight)",
			now:      time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "just after midnight",
			now:      time.Date(2025, 6, 15, 0, 0, 0, 1, time.UTC),
			expected: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "just before midnight",
			now:      time.Date(2025, 6, 15, 23, 59, 59, 999999999, time.UTC),
			expected: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "cross-year boundary (Jan 1)",
			now:      time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "leap year Feb 29",
			now:      time.Date(2024, 2, 29, 18, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "non-UTC input converted to UTC",
			now:      time.Date(2025, 6, 15, 3, 0, 0, 0, time.FixedZone("EST", -5*60*60)),
			expected: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), // 3AM EST = 8AM UTC → boundary is June 15 00:00 UTC
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateLastRotationBoundary(domain.RotationScheduleDaily, tt.now)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateLastRotationBoundary_Weekly(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		expected time.Time
	}{
		{
			name:     "Monday (boundary day)",
			now:      time.Date(2025, 6, 16, 10, 0, 0, 0, time.UTC), // Monday
			expected: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Tuesday",
			now:      time.Date(2025, 6, 17, 10, 0, 0, 0, time.UTC), // Tuesday
			expected: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),  // prev Monday
		},
		{
			name:     "Wednesday",
			now:      time.Date(2025, 6, 18, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Thursday",
			now:      time.Date(2025, 6, 19, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Friday",
			now:      time.Date(2025, 6, 20, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Saturday",
			now:      time.Date(2025, 6, 21, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Sunday",
			now:      time.Date(2025, 6, 22, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Monday at midnight",
			now:      time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "cross-year: Sunday Dec 28 to Monday Dec 29",
			now:      time.Date(2025, 12, 31, 10, 0, 0, 0, time.UTC), // Wednesday
			expected: time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC),  // Monday
		},
		{
			name:     "cross-year: Jan 1 2026 is Thursday",
			now:      time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),  // Thursday
			expected: time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC), // prev Monday (in 2025!)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateLastRotationBoundary(domain.RotationScheduleWeekly, tt.now)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateLastRotationBoundary_Monthly(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		expected time.Time
	}{
		{
			name:     "mid-month",
			now:      time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "first of month (boundary)",
			now:      time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "last day of month",
			now:      time.Date(2025, 6, 30, 23, 59, 59, 0, time.UTC),
			expected: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "February in leap year",
			now:      time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "February in non-leap year",
			now:      time.Date(2025, 2, 28, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "January 1 (cross-year)",
			now:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "December 31",
			now:      time.Date(2025, 12, 31, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateLastRotationBoundary(domain.RotationScheduleMonthly, tt.now)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateLastRotationBoundary_Invalid(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	result := CalculateLastRotationBoundary(domain.RotationSchedule("invalid"), now)
	assert.True(t, result.IsZero())

	result = CalculateLastRotationBoundary(domain.RotationSchedule(""), now)
	assert.True(t, result.IsZero())
}

// --- CalculateNextRotationBoundary ---

func TestCalculateNextRotationBoundary_Daily(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		expected time.Time
	}{
		{
			name:     "mid-day",
			now:      time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "exactly at boundary",
			now:      time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "cross-month (June 30 → July 1)",
			now:      time.Date(2025, 6, 30, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "cross-year (Dec 31 → Jan 1)",
			now:      time.Date(2025, 12, 31, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "leap year Feb 28 → Feb 29",
			now:      time.Date(2024, 2, 28, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNextRotationBoundary(domain.RotationScheduleDaily, tt.now)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateNextRotationBoundary_Weekly(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		expected time.Time
	}{
		{
			name:     "Monday → next Monday",
			now:      time.Date(2025, 6, 16, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 23, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Sunday → next Monday",
			now:      time.Date(2025, 6, 22, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 23, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "cross-year week",
			now:      time.Date(2025, 12, 31, 10, 0, 0, 0, time.UTC), // Wed
			expected: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),    // next Monday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNextRotationBoundary(domain.RotationScheduleWeekly, tt.now)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateNextRotationBoundary_Monthly(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		expected time.Time
	}{
		{
			name:     "mid-month → next month",
			now:      time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "first of month → next month",
			now:      time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "December → January (cross-year)",
			now:      time.Date(2025, 12, 15, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Jan 31 in leap year → Feb 1",
			now:      time.Date(2024, 1, 31, 10, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNextRotationBoundary(domain.RotationScheduleMonthly, tt.now)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateNextRotationBoundary_Invalid(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	result := CalculateNextRotationBoundary(domain.RotationSchedule("invalid"), now)
	assert.True(t, result.IsZero())
}

// --- CalculateNextExpiresAt ---

func TestCalculateNextExpiresAt(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC) // Sunday

	tests := []struct {
		name     string
		goal     *domain.Goal
		expected *time.Time
	}{
		{
			name: "daily rotation enabled",
			goal: &domain.Goal{
				Rotation: &domain.RotationConfig{
					Enabled:  true,
					Schedule: domain.RotationScheduleDaily,
				},
			},
			expected: timePtr(time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC)),
		},
		{
			name: "weekly rotation enabled",
			goal: &domain.Goal{
				Rotation: &domain.RotationConfig{
					Enabled:  true,
					Schedule: domain.RotationScheduleWeekly,
				},
			},
			expected: timePtr(time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC)), // next Monday
		},
		{
			name: "monthly rotation enabled",
			goal: &domain.Goal{
				Rotation: &domain.RotationConfig{
					Enabled:  true,
					Schedule: domain.RotationScheduleMonthly,
				},
			},
			expected: timePtr(time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)),
		},
		{
			name:     "nil goal",
			goal:     nil,
			expected: nil,
		},
		{
			name:     "nil rotation config",
			goal:     &domain.Goal{Rotation: nil},
			expected: nil,
		},
		{
			name: "rotation disabled",
			goal: &domain.Goal{
				Rotation: &domain.RotationConfig{
					Enabled:  false,
					Schedule: domain.RotationScheduleDaily,
				},
			},
			expected: nil,
		},
		{
			name: "invalid schedule returns nil",
			goal: &domain.Goal{
				Rotation: &domain.RotationConfig{
					Enabled:  true,
					Schedule: domain.RotationSchedule("invalid"),
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNextExpiresAt(tt.goal, now)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

// --- HasRotationOccurred ---

func TestHasRotationOccurred(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC) // Sunday
	dailyBoundary := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	dailyGoal := &domain.Goal{
		Rotation: &domain.RotationConfig{
			Enabled:  true,
			Schedule: domain.RotationScheduleDaily,
		},
	}

	tests := []struct {
		name     string
		row      *domain.UserGoalProgress
		goal     *domain.Goal
		expected bool
	}{
		{
			name:     "updatedAt before boundary → true",
			row:      &domain.UserGoalProgress{UpdatedAt: dailyBoundary.Add(-1 * time.Hour)},
			goal:     dailyGoal,
			expected: true,
		},
		{
			name:     "updatedAt exactly at boundary → false",
			row:      &domain.UserGoalProgress{UpdatedAt: dailyBoundary},
			goal:     dailyGoal,
			expected: false,
		},
		{
			name:     "updatedAt after boundary → false",
			row:      &domain.UserGoalProgress{UpdatedAt: dailyBoundary.Add(1 * time.Hour)},
			goal:     dailyGoal,
			expected: false,
		},
		{
			name:     "multiple missed rotations (3 days old) → true",
			row:      &domain.UserGoalProgress{UpdatedAt: now.AddDate(0, 0, -3)},
			goal:     dailyGoal,
			expected: true,
		},
		{
			name:     "nil row → false",
			row:      nil,
			goal:     dailyGoal,
			expected: false,
		},
		{
			name:     "nil goal → false",
			row:      &domain.UserGoalProgress{UpdatedAt: dailyBoundary.Add(-1 * time.Hour)},
			goal:     nil,
			expected: false,
		},
		{
			name:     "nil rotation config → false",
			row:      &domain.UserGoalProgress{UpdatedAt: dailyBoundary.Add(-1 * time.Hour)},
			goal:     &domain.Goal{Rotation: nil},
			expected: false,
		},
		{
			name: "rotation disabled → false",
			row:  &domain.UserGoalProgress{UpdatedAt: dailyBoundary.Add(-1 * time.Hour)},
			goal: &domain.Goal{
				Rotation: &domain.RotationConfig{
					Enabled:  false,
					Schedule: domain.RotationScheduleDaily,
				},
			},
			expected: false,
		},
		{
			name: "weekly: updatedAt before Monday boundary → true",
			row:  &domain.UserGoalProgress{UpdatedAt: time.Date(2025, 6, 8, 10, 0, 0, 0, time.UTC)}, // prev week
			goal: &domain.Goal{
				Rotation: &domain.RotationConfig{
					Enabled:  true,
					Schedule: domain.RotationScheduleWeekly,
				},
			},
			expected: true,
		},
		{
			name: "monthly: updatedAt before 1st boundary → true",
			row:  &domain.UserGoalProgress{UpdatedAt: time.Date(2025, 5, 31, 10, 0, 0, 0, time.UTC)},
			goal: &domain.Goal{
				Rotation: &domain.RotationConfig{
					Enabled:  true,
					Schedule: domain.RotationScheduleMonthly,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasRotationOccurred(tt.row, tt.goal, now)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- ApplyRotationReset ---

func TestApplyRotationReset_BehaviorMatrix(t *testing.T) {
	// now is June 15 14:00 UTC (Sunday). Daily boundary = June 15 00:00 UTC.
	// All rows have updatedAt = yesterday (before boundary) to trigger rotation.
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)
	yesterday := time.Date(2025, 6, 14, 10, 0, 0, 0, time.UTC)
	completedTime := time.Date(2025, 6, 14, 8, 0, 0, 0, time.UTC)
	claimedTime := time.Date(2025, 6, 14, 9, 0, 0, 0, time.UTC)
	expectedExpiry := time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC)
	baselineVal := 42

	makeGoal := func(resetProgress, allowReselection bool) *domain.Goal {
		return &domain.Goal{
			Rotation: &domain.RotationConfig{
				Enabled:  true,
				Schedule: domain.RotationScheduleDaily,
				OnExpiry: domain.OnExpiryConfig{
					ResetProgress:    resetProgress,
					AllowReselection: allowReselection,
				},
			},
		}
	}

	tests := []struct {
		name             string
		status           domain.GoalStatus
		resetProgress    bool
		allowReselection bool
		expectApplied    bool
		expectStatus     domain.GoalStatus
		expectProgress   int
		expectBaseline   *int
		expectCompleted  *time.Time
		expectClaimed    *time.Time
		expectExpiry     *time.Time
	}{
		// reset_progress=true cases
		{
			name:           "not_started + reset_progress=true",
			status:         domain.GoalStatusNotStarted,
			resetProgress:  true,
			expectApplied:  true,
			expectStatus:   domain.GoalStatusNotStarted,
			expectProgress: 0,
			expectBaseline: nil,
			expectExpiry:   &expectedExpiry,
		},
		{
			name:           "in_progress + reset_progress=true",
			status:         domain.GoalStatusInProgress,
			resetProgress:  true,
			expectApplied:  true,
			expectStatus:   domain.GoalStatusNotStarted,
			expectProgress: 0,
			expectBaseline: nil,
			expectExpiry:   &expectedExpiry,
		},
		{
			name:           "completed + reset_progress=true",
			status:         domain.GoalStatusCompleted,
			resetProgress:  true,
			expectApplied:  true,
			expectStatus:   domain.GoalStatusNotStarted,
			expectProgress: 0,
			expectBaseline: nil,
			expectExpiry:   &expectedExpiry,
		},
		{
			name:             "claimed + reset_progress=true + allow_reselection=true",
			status:           domain.GoalStatusClaimed,
			resetProgress:    true,
			allowReselection: true,
			expectApplied:    true,
			expectStatus:     domain.GoalStatusNotStarted,
			expectProgress:   0,
			expectBaseline:   nil,
			expectExpiry:     &expectedExpiry,
		},
		{
			name:             "claimed + reset_progress=true + allow_reselection=false → skip",
			status:           domain.GoalStatusClaimed,
			resetProgress:    true,
			allowReselection: false,
			expectApplied:    false,
		},

		// reset_progress=false cases
		{
			name:            "not_started + reset_progress=false",
			status:          domain.GoalStatusNotStarted,
			resetProgress:   false,
			expectApplied:   true,
			expectStatus:    domain.GoalStatusNotStarted,
			expectProgress:  0,
			expectBaseline:  &baselineVal,
			expectCompleted: nil,
			expectClaimed:   nil,
			expectExpiry:    &expectedExpiry,
		},
		{
			name:            "in_progress + reset_progress=false",
			status:          domain.GoalStatusInProgress,
			resetProgress:   false,
			expectApplied:   true,
			expectStatus:    domain.GoalStatusInProgress,
			expectProgress:  50,
			expectBaseline:  &baselineVal,
			expectCompleted: nil,
			expectClaimed:   nil,
			expectExpiry:    &expectedExpiry,
		},
		{
			name:            "completed + reset_progress=false",
			status:          domain.GoalStatusCompleted,
			resetProgress:   false,
			expectApplied:   true,
			expectStatus:    domain.GoalStatusCompleted,
			expectProgress:  100,
			expectBaseline:  &baselineVal,
			expectCompleted: &completedTime,
			expectClaimed:   nil,
			expectExpiry:    &expectedExpiry,
		},
		{
			name:             "claimed + reset_progress=false + allow_reselection=true → skip (meaningless combo)",
			status:           domain.GoalStatusClaimed,
			resetProgress:    false,
			allowReselection: true,
			expectApplied:    false,
		},
		{
			name:             "claimed + reset_progress=false + allow_reselection=false → skip",
			status:           domain.GoalStatusClaimed,
			resetProgress:    false,
			allowReselection: false,
			expectApplied:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := &domain.UserGoalProgress{
				UserID:        "user1",
				GoalID:        "goal1",
				ChallengeID:   "challenge1",
				Namespace:     "test",
				Status:        tt.status,
				UpdatedAt:     yesterday,
				BaselineValue: &baselineVal,
			}

			// Set progress based on status for realistic test data
			switch tt.status {
			case domain.GoalStatusNotStarted:
				row.Progress = 0
			case domain.GoalStatusInProgress:
				row.Progress = 50
			case domain.GoalStatusCompleted:
				row.Progress = 100
				row.CompletedAt = &completedTime
			case domain.GoalStatusClaimed:
				row.Progress = 100
				row.CompletedAt = &completedTime
				row.ClaimedAt = &claimedTime
			}

			goal := makeGoal(tt.resetProgress, tt.allowReselection)
			result := ApplyRotationReset(row, goal, now)
			assert.Equal(t, tt.expectApplied, result)

			if !tt.expectApplied {
				return
			}

			assert.Equal(t, tt.expectStatus, row.Status)
			assert.Equal(t, tt.expectProgress, row.Progress)
			assert.Equal(t, tt.expectBaseline, row.BaselineValue)
			assert.Equal(t, tt.expectCompleted, row.CompletedAt)
			assert.Equal(t, tt.expectClaimed, row.ClaimedAt)
			assert.Equal(t, now, row.UpdatedAt)

			if tt.expectExpiry != nil {
				assert.NotNil(t, row.ExpiresAt)
				assert.Equal(t, *tt.expectExpiry, *row.ExpiresAt)
			} else {
				assert.Nil(t, row.ExpiresAt)
			}
		})
	}
}

func TestApplyRotationReset_NoRotationOccurred(t *testing.T) {
	// updatedAt is after the boundary → no rotation
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)

	row := &domain.UserGoalProgress{
		Status:    domain.GoalStatusInProgress,
		Progress:  50,
		UpdatedAt: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC), // same day, after boundary
	}

	goal := &domain.Goal{
		Rotation: &domain.RotationConfig{
			Enabled:  true,
			Schedule: domain.RotationScheduleDaily,
			OnExpiry: domain.OnExpiryConfig{ResetProgress: true},
		},
	}

	result := ApplyRotationReset(row, goal, now)
	assert.False(t, result)
	assert.Equal(t, domain.GoalStatusInProgress, row.Status) // unchanged
	assert.Equal(t, 50, row.Progress)                        // unchanged
}

func TestApplyRotationReset_NilSafety(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)

	// nil row (via HasRotationOccurred)
	assert.False(t, ApplyRotationReset(nil, &domain.Goal{}, now))

	// nil goal (via HasRotationOccurred)
	row := &domain.UserGoalProgress{
		UpdatedAt: time.Date(2025, 6, 14, 10, 0, 0, 0, time.UTC),
	}
	assert.False(t, ApplyRotationReset(row, nil, now))
}

func TestApplyRotationReset_VerifyExpiresAtAndUpdatedAt(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC) // Sunday

	// June 15 (Sunday): last Monday boundary = June 9. UpdatedAt must be before June 9.
	row := &domain.UserGoalProgress{
		Status:    domain.GoalStatusInProgress,
		Progress:  25,
		UpdatedAt: time.Date(2025, 6, 8, 10, 0, 0, 0, time.UTC), // previous Sunday (before Jun 9 boundary)
	}

	goal := &domain.Goal{
		Rotation: &domain.RotationConfig{
			Enabled:  true,
			Schedule: domain.RotationScheduleWeekly,
			OnExpiry: domain.OnExpiryConfig{ResetProgress: false},
		},
	}

	result := ApplyRotationReset(row, goal, now)
	assert.True(t, result)

	// Weekly next boundary: June 15 is Sunday, last Monday was June 9, next Monday is June 16
	expectedExpiry := time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC)
	assert.NotNil(t, row.ExpiresAt)
	assert.Equal(t, expectedExpiry, *row.ExpiresAt)
	assert.Equal(t, now, row.UpdatedAt)

	// Progress preserved (reset_progress=false)
	assert.Equal(t, domain.GoalStatusInProgress, row.Status)
	assert.Equal(t, 25, row.Progress)
}

func TestCalculateLastRotationBoundary_NonUTCInput(t *testing.T) {
	// 3 AM EST on June 15 = 8 AM UTC on June 15
	est := time.FixedZone("EST", -5*60*60)
	now := time.Date(2025, 6, 15, 3, 0, 0, 0, est)

	result := CalculateLastRotationBoundary(domain.RotationScheduleDaily, now)
	expected := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, result)

	// 1 AM EST on June 15 = 6 AM UTC on June 15
	now2 := time.Date(2025, 6, 15, 1, 0, 0, 0, est)
	result2 := CalculateLastRotationBoundary(domain.RotationScheduleDaily, now2)
	assert.Equal(t, expected, result2)
}

// --- Helpers ---

func timePtr(t time.Time) *time.Time {
	return &t
}
