package config

import (
	"strings"
	"testing"

	"extend-challenge-common/pkg/domain"
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
								Type:        domain.GoalTypeAbsolute,
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
								Type:        domain.GoalTypeAbsolute,
								EventSource: domain.EventSourceStatistic,
								Requirement: domain.Requirement{
									StatCode:    "stat_code",
									Operator:    ">=",
									TargetValue: 10,
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
		// Goal type validation tests
		{
			name: "valid goal type - absolute",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        domain.GoalTypeAbsolute,
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
			wantErr: false,
		},
		{
			name: "valid goal type - increment",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        domain.GoalTypeIncrement,
								EventSource: domain.EventSourceLogin,
								Requirement: domain.Requirement{
									StatCode:    "login_count",
									Operator:    ">=",
									TargetValue: 7,
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
			wantErr: false,
		},
		{
			name: "valid goal type - daily",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        domain.GoalTypeDaily,
								EventSource: domain.EventSourceLogin,
								Requirement: domain.Requirement{
									StatCode:    "daily_login",
									Operator:    ">=",
									TargetValue: 7,
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
			wantErr: false,
		},
		{
			name: "invalid goal type - unknown",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        "unknown",
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
			errMsg:  "invalid goal type 'unknown'",
		},
		{
			name: "invalid goal type - weekly",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        "weekly",
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
			errMsg:  "invalid goal type 'weekly'",
		},
		{
			name: "invalid goal type - streak",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        "streak",
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
			errMsg:  "invalid goal type 'streak'",
		},
		{
			name: "invalid goal type - typo absolut",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        "absolut",
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
			errMsg:  "invalid goal type 'absolut'",
		},
		{
			name: "invalid goal type - typo incremen",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        "incremen",
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
			errMsg:  "invalid goal type 'incremen'",
		},
		{
			name: "invalid goal type - typo daly",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        "daly",
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
			errMsg:  "invalid goal type 'daly'",
		},
		{
			name: "invalid goal type - empty string is valid (defaults to absolute)",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        "",
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
			wantErr: false,
		},
		{
			name: "invalid goal type - uppercase ABSOLUTE",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        "ABSOLUTE",
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
			errMsg:  "invalid goal type 'ABSOLUTE'",
		},
		{
			name: "invalid goal type - mixed case Increment",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        "Increment",
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
			errMsg:  "invalid goal type 'Increment'",
		},
		// Daily flag validation tests
		{
			name: "valid daily flag - increment with daily=true",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        domain.GoalTypeIncrement,
								EventSource: domain.EventSourceLogin,
								Daily:       true,
								Requirement: domain.Requirement{
									StatCode:    "login_count",
									Operator:    ">=",
									TargetValue: 7,
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
			wantErr: false,
		},
		{
			name: "valid daily flag - increment with daily=false",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        domain.GoalTypeIncrement,
								EventSource: domain.EventSourceLogin,
								Daily:       false,
								Requirement: domain.Requirement{
									StatCode:    "login_count",
									Operator:    ">=",
									TargetValue: 100,
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
			wantErr: false,
		},
		{
			name: "invalid daily flag - absolute with daily=true",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        domain.GoalTypeAbsolute,
								EventSource: domain.EventSourceStatistic,
								Daily:       true,
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
			errMsg:  "daily flag can only be true for increment-type goals",
		},
		{
			name: "invalid daily flag - daily type with daily=true",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        domain.GoalTypeDaily,
								EventSource: domain.EventSourceLogin,
								Daily:       true,
								Requirement: domain.Requirement{
									StatCode:    "login_daily",
									Operator:    ">=",
									TargetValue: 1,
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
			errMsg:  "daily flag can only be true for increment-type goals",
		},
		{
			name: "valid - absolute with daily=false (ignored)",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        domain.GoalTypeAbsolute,
								EventSource: domain.EventSourceStatistic,
								Daily:       false,
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
			wantErr: false,
		},
		{
			name: "valid - daily type with daily=false (ignored)",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						ID:   "challenge-1",
						Name: "Challenge 1",
						Goals: []*domain.Goal{
							{
								ID:          "goal-1",
								Name:        "Goal 1",
								Type:        domain.GoalTypeDaily,
								EventSource: domain.EventSourceLogin,
								Daily:       false,
								Requirement: domain.Requirement{
									StatCode:    "login_daily",
									Operator:    ">=",
									TargetValue: 1,
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
