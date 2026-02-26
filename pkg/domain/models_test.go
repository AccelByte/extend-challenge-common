package domain

import (
	"encoding/json"
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

func TestProgressMode_IsValid(t *testing.T) {
	tests := []struct {
		name string
		mode ProgressMode
		want bool
	}{
		{name: "absolute is valid", mode: ProgressModeAbsolute, want: true},
		{name: "relative is valid", mode: ProgressModeRelative, want: true},
		{name: "empty is invalid", mode: ProgressMode(""), want: false},
		{name: "increment is invalid", mode: ProgressMode("increment"), want: false},
		{name: "daily is invalid", mode: ProgressMode("daily"), want: false},
		{name: "uppercase ABSOLUTE is invalid", mode: ProgressMode("ABSOLUTE"), want: false},
		{name: "unknown mode is invalid", mode: ProgressMode("weekly"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.IsValid(); got != tt.want {
				t.Errorf("ProgressMode.IsValid() = %v, want %v for mode %q", got, tt.want, tt.mode)
			}
		})
	}
}

func TestRotationSchedule_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		schedule RotationSchedule
		want     bool
	}{
		{name: "daily is valid", schedule: RotationScheduleDaily, want: true},
		{name: "weekly is valid", schedule: RotationScheduleWeekly, want: true},
		{name: "monthly is valid", schedule: RotationScheduleMonthly, want: true},
		{name: "empty is invalid", schedule: RotationSchedule(""), want: false},
		{name: "hourly is invalid", schedule: RotationSchedule("hourly"), want: false},
		{name: "uppercase DAILY is invalid", schedule: RotationSchedule("DAILY"), want: false},
		{name: "yearly is invalid", schedule: RotationSchedule("yearly"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.schedule.IsValid(); got != tt.want {
				t.Errorf("RotationSchedule.IsValid() = %v, want %v for schedule %q", got, tt.want, tt.schedule)
			}
		})
	}
}

func TestRotationType_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		rotType RotationType
		want    bool
	}{
		{name: "global is valid", rotType: RotationTypeGlobal, want: true},
		{name: "empty is invalid", rotType: RotationType(""), want: false},
		{name: "per_user is invalid in M5", rotType: RotationType("per_user"), want: false},
		{name: "uppercase GLOBAL is invalid", rotType: RotationType("GLOBAL"), want: false},
		{name: "unknown is invalid", rotType: RotationType("unknown"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rotType.IsValid(); got != tt.want {
				t.Errorf("RotationType.IsValid() = %v, want %v for type %q", got, tt.want, tt.rotType)
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

func TestUserGoalProgress_BaselineValue(t *testing.T) {
	t.Run("nil by default", func(t *testing.T) {
		p := &UserGoalProgress{
			UserID: "user1",
			GoalID: "goal1",
		}
		if p.BaselineValue != nil {
			t.Errorf("expected BaselineValue to be nil, got %v", *p.BaselineValue)
		}
	})

	t.Run("can be set to a value", func(t *testing.T) {
		val := 150
		p := &UserGoalProgress{
			UserID:        "user1",
			GoalID:        "goal1",
			BaselineValue: &val,
		}
		if p.BaselineValue == nil || *p.BaselineValue != 150 {
			t.Errorf("expected BaselineValue to be 150, got %v", p.BaselineValue)
		}
	})

	t.Run("omitted from JSON when nil", func(t *testing.T) {
		p := &UserGoalProgress{
			UserID:   "user1",
			GoalID:   "goal1",
			Status:   GoalStatusNotStarted,
			IsActive: true,
		}
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if _, exists := m["baselineValue"]; exists {
			t.Error("expected baselineValue to be omitted from JSON when nil")
		}
	})

	t.Run("included in JSON when set", func(t *testing.T) {
		val := 42
		p := &UserGoalProgress{
			UserID:        "user1",
			GoalID:        "goal1",
			Status:        GoalStatusInProgress,
			IsActive:      true,
			BaselineValue: &val,
		}
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		v, exists := m["baselineValue"]
		if !exists {
			t.Fatal("expected baselineValue to be present in JSON")
		}
		if v.(float64) != 42 {
			t.Errorf("expected baselineValue to be 42, got %v", v)
		}
	})
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
