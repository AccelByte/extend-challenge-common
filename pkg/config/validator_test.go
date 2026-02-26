package config

import (
	"strings"
	"testing"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

func TestValidator_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:          "challenge-1",
						Name:        "Challenge 1",
						Description: "Description",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Description: "Description",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:     "stat_code",
									Operator:     ">=",
									TargetValue:  10,
									ProgressMode: domain.ProgressModeAbsolute,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
								Prerequisites: []string{},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty challenges",
			config:  &Config{Challenges: []*domain.Challenge{}},
			wantErr: true,
			errMsg:  "config must have at least one challenge",
		},
		{
			name: "empty challenge ID",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "challenge ID cannot be empty",
		},
		{
			name: "empty challenge name",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "challenge name cannot be empty",
		},
		{
			name: "challenge with no goals",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:    "challenge-1",
						Name:  "Challenge 1",
						Goals: []*domain.Goal{},
					},
				},
			},
			wantErr: true,
			errMsg:  "challenge must have at least one goal",
		},
		{
			name: "duplicate challenge IDs",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
					{
						ID:   "challenge-1",
						Name: "Challenge 2",
						Goals: []*domain.Goal{
							{
								ID:          "goal-2",
								Name:        "Goal 2",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "duplicate challenge ID: challenge-1",
		},
		{
			name: "empty goal ID",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "goal ID cannot be empty",
		},
		{
			name: "empty goal name",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "goal name cannot be empty",
		},
		{
			name: "empty stat_code",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "stat_code cannot be empty",
		},
		{
			name: "invalid operator",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    "==",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "unsupported operator '==' (only '>=' supported)",
		},
		{
			name: "zero target_value",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 0,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "target_value must be positive",
		},
		{
			name: "negative target_value",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: -10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "target_value must be positive",
		},
		{
			name: "invalid reward type",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "UNKNOWN",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "unsupported reward type 'UNKNOWN' (only 'ITEM' or 'WALLET' allowed)",
		},
		{
			name: "empty reward_id",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "reward_id cannot be empty",
		},
		{
			name: "zero reward quantity",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 0,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "reward quantity must be positive",
		},
		{
			name: "negative reward quantity",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: -5,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "reward quantity must be positive",
		},
		{
			name: "duplicate goal IDs",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
							{
								ID:          "goal-1",
								Name:        "Goal 2",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "duplicate goal ID: goal-1",
		},
		{
			name: "invalid prerequisite",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
								Prerequisites: []string{"nonexistent-goal"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "goal 'goal-1' has invalid prerequisite: 'nonexistent-goal' does not exist",
		},
		{
			name: "valid prerequisites",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
								Prerequisites: []string{},
							},
							{
								ID:          "goal-2",
								Name:        "Goal 2",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 20,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_2",
									Quantity: 1,
								},
								Prerequisites: []string{"goal-1"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "WALLET reward type",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:     "stat_code",
									Operator:     ">=",
									TargetValue:  10,
									ProgressMode: domain.ProgressModeAbsolute,
								},
								Reward: domain.Reward{
									Type:     "WALLET",
									RewardID: "GOLD",
									Quantity: 100,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			err := v.Validate(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// validGoalWithRotation returns a minimal valid goal config with an optional rotation override.
func validGoalWithRotation(rotation *domain.RotationConfig, progressMode domain.ProgressMode) *Config {
	return &Config{
		Challenges: []*domain.Challenge{
			{
				ID:   "challenge-1",
				Name: "Challenge 1",
				Goals: []*domain.Goal{
					{
						ID:          "goal-1",
						Name:        "Goal 1",
						EventSource: domain.EventSourceStatistic,
						Requirement: domain.Requirement{
							StatCode:     "stat_code",
							Operator:     ">=",
							TargetValue:  10,
							ProgressMode: progressMode,
						},
						Reward: domain.Reward{
							Type:     "ITEM",
							RewardID: "item_1",
							Quantity: 1,
						},
						Rotation: rotation,
					},
				},
			},
		},
	}
}

func TestValidator_Rotation(t *testing.T) {
	tests := []struct {
		name         string
		rotation     *domain.RotationConfig
		progressMode domain.ProgressMode
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "nil rotation is allowed",
			rotation:     nil,
			progressMode: domain.ProgressModeAbsolute,
			wantErr:      false,
		},
		{
			name: "disabled rotation is allowed",
			rotation: &domain.RotationConfig{
				Enabled: false,
			},
			progressMode: domain.ProgressModeAbsolute,
			wantErr:      false,
		},
		{
			name: "valid daily global rotation",
			rotation: &domain.RotationConfig{
				Enabled:  true,
				Type:     domain.RotationTypeGlobal,
				Schedule: domain.RotationScheduleDaily,
				OnExpiry: domain.OnExpiryConfig{ResetProgress: true, AllowReselection: true},
			},
			progressMode: domain.ProgressModeRelative,
			wantErr:      false,
		},
		{
			name: "valid weekly global rotation",
			rotation: &domain.RotationConfig{
				Enabled:  true,
				Type:     domain.RotationTypeGlobal,
				Schedule: domain.RotationScheduleWeekly,
				OnExpiry: domain.OnExpiryConfig{ResetProgress: true, AllowReselection: false},
			},
			progressMode: domain.ProgressModeRelative,
			wantErr:      false,
		},
		{
			name: "valid monthly global rotation",
			rotation: &domain.RotationConfig{
				Enabled:  true,
				Type:     domain.RotationTypeGlobal,
				Schedule: domain.RotationScheduleMonthly,
				OnExpiry: domain.OnExpiryConfig{ResetProgress: false, AllowReselection: true},
			},
			progressMode: domain.ProgressModeRelative,
			wantErr:      false,
		},
		{
			name: "invalid schedule rejected",
			rotation: &domain.RotationConfig{
				Enabled:  true,
				Type:     domain.RotationTypeGlobal,
				Schedule: domain.RotationSchedule("hourly"),
			},
			progressMode: domain.ProgressModeRelative,
			wantErr:      true,
			errMsg:       "invalid rotation schedule 'hourly'",
		},
		{
			name: "invalid type rejected",
			rotation: &domain.RotationConfig{
				Enabled:  true,
				Type:     domain.RotationType("per_user"),
				Schedule: domain.RotationScheduleDaily,
			},
			progressMode: domain.ProgressModeRelative,
			wantErr:      true,
			errMsg:       "invalid rotation type 'per_user'",
		},
		{
			name: "absolute with rotation rejected",
			rotation: &domain.RotationConfig{
				Enabled:  true,
				Type:     domain.RotationTypeGlobal,
				Schedule: domain.RotationScheduleDaily,
			},
			progressMode: domain.ProgressModeAbsolute,
			wantErr:      true,
			errMsg:       "rotation requires progressMode 'relative', got 'absolute'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validGoalWithRotation(tt.rotation, tt.progressMode)
			v := NewValidator()
			err := v.Validate(config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidator_ProgressMode_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		mode   domain.ProgressMode
		errMsg string
	}{
		{"invalid progressMode - unknown", domain.ProgressMode("unknown"), "invalid progressMode 'unknown'"},
		{"invalid progressMode - increment", domain.ProgressMode("increment"), "invalid progressMode 'increment'"},
		{"invalid progressMode - daily", domain.ProgressMode("daily"), "invalid progressMode 'daily'"},
		{"invalid progressMode - ABSOLUTE uppercase", domain.ProgressMode("ABSOLUTE"), "invalid progressMode 'ABSOLUTE'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:     "stat_code",
									Operator:     ">=",
									TargetValue:  10,
									ProgressMode: tt.mode,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			}

			v := NewValidator()
			err := v.Validate(config)
			if err == nil {
				t.Errorf("Validate() expected error for progressMode %q, got nil", tt.mode)
				return
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
			}
		})
	}
}

func TestValidator_ProgressMode_Valid(t *testing.T) {
	tests := []struct {
		name string
		mode domain.ProgressMode
	}{
		{"valid progressMode - absolute", domain.ProgressModeAbsolute},
		{"valid progressMode - relative", domain.ProgressModeRelative},
		{"valid progressMode - empty (not validated)", domain.ProgressMode("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:     "stat_code",
									Operator:     ">=",
									TargetValue:  10,
									ProgressMode: tt.mode,
								},
								Reward: domain.Reward{
									Type:     "ITEM",
									RewardID: "item_1",
									Quantity: 1,
								},
							},
						},
					},
				},
			}

			v := NewValidator()
			if err := v.Validate(config); err != nil {
				t.Errorf("Validate() unexpected error for progressMode %q: %v", tt.mode, err)
			}
		})
	}
}
