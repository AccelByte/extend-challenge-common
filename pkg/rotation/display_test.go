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

// --- CalculateDisplayedProgress ---

func TestCalculateDisplayedProgress_AbsoluteMode(t *testing.T) {
	goal := &domain.Goal{
		Requirement: domain.Requirement{
			ProgressMode: domain.ProgressModeAbsolute,
			TargetValue:  100,
		},
	}

	row := &domain.UserGoalProgress{Progress: 42}
	assert.Equal(t, 42, CalculateDisplayedProgress(row, goal))
}

func TestCalculateDisplayedProgress_RelativeWithBaseline(t *testing.T) {
	baseline := 50
	goal := &domain.Goal{
		Requirement: domain.Requirement{
			ProgressMode: domain.ProgressModeRelative,
			TargetValue:  100,
		},
	}

	row := &domain.UserGoalProgress{
		Progress:      80,
		BaselineValue: &baseline,
	}
	assert.Equal(t, 30, CalculateDisplayedProgress(row, goal))
}

func TestCalculateDisplayedProgress_RelativeNilBaseline(t *testing.T) {
	goal := &domain.Goal{
		Requirement: domain.Requirement{
			ProgressMode: domain.ProgressModeRelative,
			TargetValue:  100,
		},
	}

	row := &domain.UserGoalProgress{
		Progress:      80,
		BaselineValue: nil,
	}
	assert.Equal(t, 0, CalculateDisplayedProgress(row, goal))
}

func TestCalculateDisplayedProgress_RelativeNegativeDelta(t *testing.T) {
	baseline := 100
	goal := &domain.Goal{
		Requirement: domain.Requirement{
			ProgressMode: domain.ProgressModeRelative,
			TargetValue:  50,
		},
	}

	row := &domain.UserGoalProgress{
		Progress:      80,
		BaselineValue: &baseline,
	}
	// 80 - 100 = -20, clamped to 0
	assert.Equal(t, 0, CalculateDisplayedProgress(row, goal))
}

func TestCalculateDisplayedProgress_NilInputs(t *testing.T) {
	goal := &domain.Goal{
		Requirement: domain.Requirement{ProgressMode: domain.ProgressModeAbsolute},
	}
	row := &domain.UserGoalProgress{Progress: 10}

	assert.Equal(t, 0, CalculateDisplayedProgress(nil, goal))
	assert.Equal(t, 0, CalculateDisplayedProgress(row, nil))
	assert.Equal(t, 0, CalculateDisplayedProgress(nil, nil))
}

func TestCalculateDisplayedProgress_EmptyProgressMode(t *testing.T) {
	// Empty/default progress mode should behave like absolute
	goal := &domain.Goal{
		Requirement: domain.Requirement{ProgressMode: ""},
	}

	row := &domain.UserGoalProgress{Progress: 25}
	assert.Equal(t, 25, CalculateDisplayedProgress(row, goal))
}

// --- ApplyDisplayRotation ---

func TestApplyDisplayRotation_NoRotationOccurred(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)

	row := &domain.UserGoalProgress{
		Progress:  50,
		Status:    domain.GoalStatusInProgress,
		UpdatedAt: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC), // same day, after boundary
	}

	goal := &domain.Goal{
		Requirement: domain.Requirement{ProgressMode: domain.ProgressModeAbsolute},
		Rotation: &domain.RotationConfig{
			Enabled:  true,
			Schedule: domain.RotationScheduleDaily,
			OnExpiry: domain.OnExpiryConfig{ResetProgress: true},
		},
	}

	progress, status, rotated := ApplyDisplayRotation(row, goal, now)
	assert.Equal(t, 50, progress)
	assert.Equal(t, domain.GoalStatusInProgress, status)
	assert.False(t, rotated)
}

func TestApplyDisplayRotation_RotatedWithReset(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)
	yesterday := time.Date(2025, 6, 14, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		status         domain.GoalStatus
		expectProgress int
		expectStatus   domain.GoalStatus
	}{
		{
			name:           "not_started + reset → 0, not_started",
			status:         domain.GoalStatusNotStarted,
			expectProgress: 0,
			expectStatus:   domain.GoalStatusNotStarted,
		},
		{
			name:           "in_progress + reset → 0, not_started",
			status:         domain.GoalStatusInProgress,
			expectProgress: 0,
			expectStatus:   domain.GoalStatusNotStarted,
		},
		{
			name:           "completed + reset → 0, not_started",
			status:         domain.GoalStatusCompleted,
			expectProgress: 0,
			expectStatus:   domain.GoalStatusNotStarted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := &domain.UserGoalProgress{
				Progress:  75,
				Status:    tt.status,
				UpdatedAt: yesterday,
			}

			goal := &domain.Goal{
				Requirement: domain.Requirement{ProgressMode: domain.ProgressModeAbsolute},
				Rotation: &domain.RotationConfig{
					Enabled:  true,
					Schedule: domain.RotationScheduleDaily,
					OnExpiry: domain.OnExpiryConfig{ResetProgress: true},
				},
			}

			progress, status, rotated := ApplyDisplayRotation(row, goal, now)
			assert.Equal(t, tt.expectProgress, progress)
			assert.Equal(t, tt.expectStatus, status)
			assert.True(t, rotated)
		})
	}
}

func TestApplyDisplayRotation_RotatedClaimedPermanent(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)
	yesterday := time.Date(2025, 6, 14, 10, 0, 0, 0, time.UTC)

	row := &domain.UserGoalProgress{
		Progress:  100,
		Status:    domain.GoalStatusClaimed,
		UpdatedAt: yesterday,
	}

	goal := &domain.Goal{
		Requirement: domain.Requirement{ProgressMode: domain.ProgressModeAbsolute},
		Rotation: &domain.RotationConfig{
			Enabled:  true,
			Schedule: domain.RotationScheduleDaily,
			OnExpiry: domain.OnExpiryConfig{
				ResetProgress:    true,
				AllowReselection: false, // permanent claim
			},
		},
	}

	progress, status, rotated := ApplyDisplayRotation(row, goal, now)
	assert.Equal(t, 100, progress) // keep as-is
	assert.Equal(t, domain.GoalStatusClaimed, status)
	assert.True(t, rotated)
}

func TestApplyDisplayRotation_RotatedClaimedAllowReselection(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)
	yesterday := time.Date(2025, 6, 14, 10, 0, 0, 0, time.UTC)

	row := &domain.UserGoalProgress{
		Progress:  100,
		Status:    domain.GoalStatusClaimed,
		UpdatedAt: yesterday,
	}

	goal := &domain.Goal{
		Requirement: domain.Requirement{ProgressMode: domain.ProgressModeAbsolute},
		Rotation: &domain.RotationConfig{
			Enabled:  true,
			Schedule: domain.RotationScheduleDaily,
			OnExpiry: domain.OnExpiryConfig{
				ResetProgress:    true,
				AllowReselection: true, // can be re-attempted
			},
		},
	}

	progress, status, rotated := ApplyDisplayRotation(row, goal, now)
	assert.Equal(t, 0, progress)
	assert.Equal(t, domain.GoalStatusNotStarted, status)
	assert.True(t, rotated)
}

func TestApplyDisplayRotation_RotatedNoReset(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)
	yesterday := time.Date(2025, 6, 14, 10, 0, 0, 0, time.UTC)

	row := &domain.UserGoalProgress{
		Progress:  50,
		Status:    domain.GoalStatusInProgress,
		UpdatedAt: yesterday,
	}

	goal := &domain.Goal{
		Requirement: domain.Requirement{ProgressMode: domain.ProgressModeAbsolute},
		Rotation: &domain.RotationConfig{
			Enabled:  true,
			Schedule: domain.RotationScheduleDaily,
			OnExpiry: domain.OnExpiryConfig{ResetProgress: false},
		},
	}

	progress, status, rotated := ApplyDisplayRotation(row, goal, now)
	assert.Equal(t, 50, progress)                        // kept
	assert.Equal(t, domain.GoalStatusInProgress, status) // kept
	assert.True(t, rotated)
}

func TestApplyDisplayRotation_NilInputs(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)
	goal := &domain.Goal{}
	row := &domain.UserGoalProgress{}

	p, s, r := ApplyDisplayRotation(nil, goal, now)
	assert.Equal(t, 0, p)
	assert.Equal(t, domain.GoalStatusNotStarted, s)
	assert.False(t, r)

	p, s, r = ApplyDisplayRotation(row, nil, now)
	assert.Equal(t, 0, p)
	assert.Equal(t, domain.GoalStatusNotStarted, s)
	assert.False(t, r)
}

func TestApplyDisplayRotation_NoRotationConfig(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)

	row := &domain.UserGoalProgress{
		Progress:  50,
		Status:    domain.GoalStatusInProgress,
		UpdatedAt: time.Date(2025, 6, 14, 10, 0, 0, 0, time.UTC),
	}

	goal := &domain.Goal{
		Requirement: domain.Requirement{ProgressMode: domain.ProgressModeAbsolute},
		Rotation:    nil, // no rotation
	}

	progress, status, rotated := ApplyDisplayRotation(row, goal, now)
	assert.Equal(t, 50, progress)
	assert.Equal(t, domain.GoalStatusInProgress, status)
	assert.False(t, rotated)
}

func TestApplyDisplayRotation_RelativeModeWithRotation(t *testing.T) {
	now := time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC)
	baseline := 50

	row := &domain.UserGoalProgress{
		Progress:      80,
		BaselineValue: &baseline,
		Status:        domain.GoalStatusInProgress,
		UpdatedAt:     time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC), // no rotation
	}

	goal := &domain.Goal{
		Requirement: domain.Requirement{
			ProgressMode: domain.ProgressModeRelative,
			TargetValue:  100,
		},
		Rotation: &domain.RotationConfig{
			Enabled:  true,
			Schedule: domain.RotationScheduleDaily,
			OnExpiry: domain.OnExpiryConfig{ResetProgress: true},
		},
	}

	progress, status, rotated := ApplyDisplayRotation(row, goal, now)
	assert.Equal(t, 30, progress) // 80 - 50 = 30 (relative)
	assert.Equal(t, domain.GoalStatusInProgress, status)
	assert.False(t, rotated)
}
