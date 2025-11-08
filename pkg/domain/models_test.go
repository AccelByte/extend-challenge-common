package domain

import (
	"testing"
	"time"
)

func TestEventSource_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		source EventSource
		want   bool
	}{
		{
			name:   "login is valid",
			source: EventSourceLogin,
			want:   true,
		},
		{
			name:   "statistic is valid",
			source: EventSourceStatistic,
			want:   true,
		},
		{
			name:   "invalid source",
			source: EventSource("invalid"),
			want:   false,
		},
		{
			name:   "empty source",
			source: EventSource(""),
			want:   false,
		},
		{
			name:   "random source",
			source: EventSource("random"),
			want:   false,
		},
		{
			name:   "uppercase LOGIN",
			source: EventSource("LOGIN"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.source.IsValid(); got != tt.want {
				t.Errorf("EventSource.IsValid() = %v, want %v for source %q", got, tt.want, tt.source)
			}
		})
	}
}

func TestGoalType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		goalType GoalType
		want     bool
	}{
		{
			name:     "absolute is valid",
			goalType: GoalTypeAbsolute,
			want:     true,
		},
		{
			name:     "increment is valid",
			goalType: GoalTypeIncrement,
			want:     true,
		},
		{
			name:     "daily is valid",
			goalType: GoalTypeDaily,
			want:     true,
		},
		{
			name:     "invalid type",
			goalType: GoalType("invalid"),
			want:     false,
		},
		{
			name:     "empty type",
			goalType: GoalType(""),
			want:     false,
		},
		{
			name:     "weekly (not supported)",
			goalType: GoalType("weekly"),
			want:     false,
		},
		{
			name:     "streak (not supported)",
			goalType: GoalType("streak"),
			want:     false,
		},
		{
			name:     "uppercase ABSOLUTE",
			goalType: GoalType("ABSOLUTE"),
			want:     false,
		},
		{
			name:     "mixed case Increment",
			goalType: GoalType("Increment"),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.goalType.IsValid(); got != tt.want {
				t.Errorf("GoalType.IsValid() = %v, want %v for type %q", got, tt.want, tt.goalType)
			}
		})
	}
}

func TestGoalStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status GoalStatus
		want   bool
	}{
		{
			name:   "not_started is valid",
			status: GoalStatusNotStarted,
			want:   true,
		},
		{
			name:   "in_progress is valid",
			status: GoalStatusInProgress,
			want:   true,
		},
		{
			name:   "completed is valid",
			status: GoalStatusCompleted,
			want:   true,
		},
		{
			name:   "claimed is valid",
			status: GoalStatusClaimed,
			want:   true,
		},
		{
			name:   "invalid status",
			status: GoalStatus("invalid"),
			want:   false,
		},
		{
			name:   "empty status",
			status: GoalStatus(""),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("GoalStatus.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserGoalProgress_IsCompleted(t *testing.T) {
	tests := []struct {
		name     string
		progress *UserGoalProgress
		want     bool
	}{
		{
			name: "not_started is not completed",
			progress: &UserGoalProgress{
				Status: GoalStatusNotStarted,
			},
			want: false,
		},
		{
			name: "in_progress is not completed",
			progress: &UserGoalProgress{
				Status: GoalStatusInProgress,
			},
			want: false,
		},
		{
			name: "completed is completed",
			progress: &UserGoalProgress{
				Status: GoalStatusCompleted,
			},
			want: true,
		},
		{
			name: "claimed is completed",
			progress: &UserGoalProgress{
				Status: GoalStatusClaimed,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.progress.IsCompleted(); got != tt.want {
				t.Errorf("UserGoalProgress.IsCompleted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserGoalProgress_IsClaimed(t *testing.T) {
	tests := []struct {
		name     string
		progress *UserGoalProgress
		want     bool
	}{
		{
			name: "not_started is not claimed",
			progress: &UserGoalProgress{
				Status: GoalStatusNotStarted,
			},
			want: false,
		},
		{
			name: "in_progress is not claimed",
			progress: &UserGoalProgress{
				Status: GoalStatusInProgress,
			},
			want: false,
		},
		{
			name: "completed is not claimed",
			progress: &UserGoalProgress{
				Status: GoalStatusCompleted,
			},
			want: false,
		},
		{
			name: "claimed is claimed",
			progress: &UserGoalProgress{
				Status: GoalStatusClaimed,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.progress.IsClaimed(); got != tt.want {
				t.Errorf("UserGoalProgress.IsClaimed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserGoalProgress_CanClaim(t *testing.T) {
	tests := []struct {
		name     string
		progress *UserGoalProgress
		want     bool
	}{
		{
			name: "not_started cannot claim",
			progress: &UserGoalProgress{
				Status:   GoalStatusNotStarted,
				IsActive: true,
			},
			want: false,
		},
		{
			name: "in_progress cannot claim",
			progress: &UserGoalProgress{
				Status:   GoalStatusInProgress,
				IsActive: true,
			},
			want: false,
		},
		{
			name: "completed and active can claim",
			progress: &UserGoalProgress{
				Status:   GoalStatusCompleted,
				IsActive: true,
			},
			want: true,
		},
		{
			name: "claimed cannot claim",
			progress: &UserGoalProgress{
				Status:   GoalStatusClaimed,
				IsActive: true,
			},
			want: false,
		},
		// M3 Phase 6: Test is_active validation
		{
			name: "completed but inactive cannot claim",
			progress: &UserGoalProgress{
				Status:   GoalStatusCompleted,
				IsActive: false,
			},
			want: false,
		},
		{
			name: "in_progress and inactive cannot claim",
			progress: &UserGoalProgress{
				Status:   GoalStatusInProgress,
				IsActive: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.progress.CanClaim(); got != tt.want {
				t.Errorf("UserGoalProgress.CanClaim() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserGoalProgress_MeetsRequirement(t *testing.T) {
	tests := []struct {
		name        string
		progress    *UserGoalProgress
		requirement Requirement
		want        bool
	}{
		{
			name: "meets requirement - exact match",
			progress: &UserGoalProgress{
				Progress: 10,
			},
			requirement: Requirement{
				StatCode:    "kills",
				Operator:    ">=",
				TargetValue: 10,
			},
			want: true,
		},
		{
			name: "meets requirement - exceeds target",
			progress: &UserGoalProgress{
				Progress: 15,
			},
			requirement: Requirement{
				StatCode:    "kills",
				Operator:    ">=",
				TargetValue: 10,
			},
			want: true,
		},
		{
			name: "does not meet requirement",
			progress: &UserGoalProgress{
				Progress: 5,
			},
			requirement: Requirement{
				StatCode:    "kills",
				Operator:    ">=",
				TargetValue: 10,
			},
			want: false,
		},
		{
			name: "zero progress does not meet requirement",
			progress: &UserGoalProgress{
				Progress: 0,
			},
			requirement: Requirement{
				StatCode:    "kills",
				Operator:    ">=",
				TargetValue: 1,
			},
			want: false,
		},
		{
			name: "unsupported operator returns false",
			progress: &UserGoalProgress{
				Progress: 10,
			},
			requirement: Requirement{
				StatCode:    "kills",
				Operator:    "==",
				TargetValue: 10,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.progress.MeetsRequirement(tt.requirement); got != tt.want {
				t.Errorf("UserGoalProgress.MeetsRequirement() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserGoalProgress_StatusTransitions(t *testing.T) {
	now := time.Now()

	// Test typical status flow: not_started -> in_progress -> completed -> claimed
	progress := &UserGoalProgress{
		UserID:      "user123",
		GoalID:      "goal456",
		ChallengeID: "challenge789",
		Namespace:   "test",
		Progress:    0,
		Status:      GoalStatusNotStarted,
		IsActive:    true, // M3 Phase 6: Goal must be active to claim
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Initial state
	if progress.IsCompleted() {
		t.Error("new progress should not be completed")
	}
	if progress.IsClaimed() {
		t.Error("new progress should not be claimed")
	}
	if progress.CanClaim() {
		t.Error("new progress should not be claimable")
	}

	// Transition to in_progress
	progress.Progress = 5
	progress.Status = GoalStatusInProgress
	progress.UpdatedAt = now.Add(1 * time.Minute)

	if progress.IsCompleted() {
		t.Error("in_progress should not be completed")
	}
	if progress.CanClaim() {
		t.Error("in_progress should not be claimable")
	}

	// Transition to completed
	progress.Progress = 10
	progress.Status = GoalStatusCompleted
	completedTime := now.Add(2 * time.Minute)
	progress.CompletedAt = &completedTime
	progress.UpdatedAt = completedTime

	if !progress.IsCompleted() {
		t.Error("completed progress should be completed")
	}
	if !progress.CanClaim() {
		t.Error("completed progress should be claimable")
	}
	if progress.IsClaimed() {
		t.Error("completed progress should not be claimed yet")
	}

	// Transition to claimed
	progress.Status = GoalStatusClaimed
	claimedTime := now.Add(3 * time.Minute)
	progress.ClaimedAt = &claimedTime
	progress.UpdatedAt = claimedTime

	if !progress.IsCompleted() {
		t.Error("claimed progress should be completed")
	}
	if !progress.IsClaimed() {
		t.Error("claimed progress should be claimed")
	}
	if progress.CanClaim() {
		t.Error("claimed progress should not be claimable again")
	}
}
