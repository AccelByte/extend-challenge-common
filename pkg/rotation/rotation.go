// Copyright (c) 2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package rotation

import (
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// CalculateLastRotationBoundary returns the start of the current rotation period.
// Daily: today 00:00 UTC. Weekly: last Monday 00:00 UTC. Monthly: 1st of month 00:00 UTC.
// Returns zero time for invalid schedules.
func CalculateLastRotationBoundary(schedule domain.RotationSchedule, now time.Time) time.Time {
	now = now.UTC()

	switch schedule {
	case domain.RotationScheduleDaily:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	case domain.RotationScheduleWeekly:
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		daysFromMonday := (int(now.Weekday()) + 6) % 7
		return today.AddDate(0, 0, -daysFromMonday)
	case domain.RotationScheduleMonthly:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Time{}
	}
}

// CalculateNextRotationBoundary returns when the current period ends (= start of next period).
// Returns zero time for invalid schedules.
func CalculateNextRotationBoundary(schedule domain.RotationSchedule, now time.Time) time.Time {
	last := CalculateLastRotationBoundary(schedule, now)
	if last.IsZero() {
		return time.Time{}
	}

	switch schedule {
	case domain.RotationScheduleDaily:
		return last.Add(24 * time.Hour)
	case domain.RotationScheduleWeekly:
		return last.AddDate(0, 0, 7)
	case domain.RotationScheduleMonthly:
		return last.AddDate(0, 1, 0)
	default:
		return time.Time{}
	}
}

// CalculateNextExpiresAt returns *time.Time for the goal's next expiry.
// Returns nil if goal is nil, rotation is nil, or rotation is disabled.
func CalculateNextExpiresAt(goal *domain.Goal, now time.Time) *time.Time {
	if goal == nil || goal.Rotation == nil || !goal.Rotation.Enabled {
		return nil
	}

	next := CalculateNextRotationBoundary(goal.Rotation.Schedule, now)
	if next.IsZero() {
		return nil
	}

	return &next
}

// HasRotationOccurred returns true if a rotation boundary has passed since the row was last updated.
// Read-only; does not modify the row. Returns false for nil inputs or disabled rotation.
func HasRotationOccurred(row *domain.UserGoalProgress, goal *domain.Goal, now time.Time) bool {
	if row == nil || goal == nil || goal.Rotation == nil || !goal.Rotation.Enabled {
		return false
	}

	lastBoundary := CalculateLastRotationBoundary(goal.Rotation.Schedule, now)
	if lastBoundary.IsZero() {
		return false
	}

	return row.UpdatedAt.Before(lastBoundary)
}

// ApplyRotationReset mutates the row for the new rotation period. Returns true if reset was applied.
//
// Behavior matrix:
//
//	| Status       | reset_progress=true              | reset_progress=false |
//	|--------------|----------------------------------|----------------------|
//	| not_started  | Reset baseline/status            | Update expiry only   |
//	| in_progress  | Reset to not_started             | Update expiry only   |
//	| completed    | Reset to not_started             | Update expiry only   |
//	| claimed      | Reset if allow_reselection=true  | Skip (return false)  |
func ApplyRotationReset(row *domain.UserGoalProgress, goal *domain.Goal, now time.Time) bool {
	if !HasRotationOccurred(row, goal, now) {
		return false
	}

	// Claimed + !allow_reselection → permanent, skip
	if row.Status == domain.GoalStatusClaimed && !goal.Rotation.OnExpiry.AllowReselection {
		return false
	}

	// Claimed + !reset_progress → meaningless combo per spec line 842, skip
	if row.Status == domain.GoalStatusClaimed && !goal.Rotation.OnExpiry.ResetProgress {
		return false
	}

	if goal.Rotation.OnExpiry.ResetProgress {
		row.BaselineValue = nil
		row.Status = domain.GoalStatusNotStarted
		row.Progress = 0
		row.CompletedAt = nil
		row.ClaimedAt = nil
	}

	row.ExpiresAt = CalculateNextExpiresAt(goal, now)
	row.UpdatedAt = now

	return true
}
