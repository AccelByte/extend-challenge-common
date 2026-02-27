// Copyright (c) 2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package rotation

import (
	"time"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// CalculateDisplayedProgress computes the progress value shown to users.
//
// For absolute mode: returns raw progress as-is.
// For relative mode: returns progress minus baseline (delta since rotation start).
// Returns 0 if baseline is not yet set (relative mode, first event pending).
func CalculateDisplayedProgress(row *domain.UserGoalProgress, goal *domain.Goal) int {
	if row == nil || goal == nil {
		return 0
	}

	if goal.Requirement.ProgressMode == domain.ProgressModeRelative {
		if row.BaselineValue == nil {
			return 0
		}
		delta := row.Progress - *row.BaselineValue
		if delta < 0 {
			return 0
		}
		return delta
	}

	return row.Progress
}

// ApplyDisplayRotation returns display-adjusted values for API responses without mutating the row.
//
// This is the read-only counterpart to ApplyRotationReset. It detects rotation in-memory
// and returns what the user should see, without writing to the database.
//
// Returns:
//   - progress: displayed progress value
//   - status: displayed goal status
//   - rotated: whether a rotation boundary has been crossed
func ApplyDisplayRotation(row *domain.UserGoalProgress, goal *domain.Goal, now time.Time) (progress int, status domain.GoalStatus, rotated bool) {
	if row == nil || goal == nil {
		return 0, domain.GoalStatusNotStarted, false
	}

	if !HasRotationOccurred(row, goal, now) {
		return CalculateDisplayedProgress(row, goal), row.Status, false
	}

	// Rotation occurred
	if !goal.Rotation.OnExpiry.ResetProgress {
		// No reset: keep current displayed progress and status
		return CalculateDisplayedProgress(row, goal), row.Status, true
	}

	// reset_progress=true: check claimed special cases
	if row.Status == domain.GoalStatusClaimed && !goal.Rotation.OnExpiry.AllowReselection {
		// Permanent claim: show as-is (claimed forever)
		return CalculateDisplayedProgress(row, goal), row.Status, true
	}

	// Rotated + reset_progress=true → show reset state
	return 0, domain.GoalStatusNotStarted, true
}
