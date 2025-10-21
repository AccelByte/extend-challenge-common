package config

import (
	"errors"
	"fmt"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// Validator validates challenge configuration files.
// It ensures all business rules are met before the application starts.
type Validator struct{}

// NewValidator creates a new Validator instance.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate performs comprehensive validation of the configuration.
// It checks for:
// - At least one challenge exists
// - All challenge IDs are unique
// - All goal IDs are globally unique
// - All prerequisites reference valid goals
// - All requirements and rewards are valid
//
// Returns an error describing the first validation failure encountered.
func (v *Validator) Validate(config *Config) error {
	if len(config.Challenges) == 0 {
		return errors.New("config must have at least one challenge")
	}

	// Track unique IDs
	challengeIDs := make(map[string]bool)
	goalIDs := make(map[string]bool)
	allGoals := make(map[string]*domain.Goal)

	// First pass: collect all IDs and goals
	for _, challenge := range config.Challenges {
		// Validate challenge
		if err := v.validateChallenge(challenge); err != nil {
			return fmt.Errorf("invalid challenge '%s': %w", challenge.ID, err)
		}

		// Check duplicate challenge ID
		if challengeIDs[challenge.ID] {
			return fmt.Errorf("duplicate challenge ID: %s", challenge.ID)
		}
		challengeIDs[challenge.ID] = true

		// Validate goals
		for _, goal := range challenge.Goals {
			if err := v.validateGoal(goal); err != nil {
				return fmt.Errorf("invalid goal '%s' in challenge '%s': %w", goal.ID, challenge.ID, err)
			}

			// Check duplicate goal ID
			if goalIDs[goal.ID] {
				return fmt.Errorf("duplicate goal ID: %s", goal.ID)
			}
			goalIDs[goal.ID] = true

			allGoals[goal.ID] = goal
		}
	}

	// Second pass: validate prerequisites
	for _, goal := range allGoals {
		for _, prereqID := range goal.Prerequisites {
			if _, exists := allGoals[prereqID]; !exists {
				return fmt.Errorf("goal '%s' has invalid prerequisite: '%s' does not exist", goal.ID, prereqID)
			}
		}
	}

	return nil
}

// validateChallenge validates a single challenge.
func (v *Validator) validateChallenge(challenge *domain.Challenge) error {
	if challenge.ID == "" {
		return errors.New("challenge ID cannot be empty")
	}
	if challenge.Name == "" {
		return errors.New("challenge name cannot be empty")
	}
	if len(challenge.Goals) == 0 {
		return errors.New("challenge must have at least one goal")
	}
	return nil
}

// validateGoal validates a single goal.
func (v *Validator) validateGoal(goal *domain.Goal) error {
	if goal.ID == "" {
		return errors.New("goal ID cannot be empty")
	}
	if goal.Name == "" {
		return errors.New("goal name cannot be empty")
	}

	// Validate goal type
	if goal.Type != "" && !goal.Type.IsValid() {
		return fmt.Errorf("invalid goal type '%s' (must be 'absolute', 'increment', or 'daily')", goal.Type)
	}

	// Validate event source (required field)
	if goal.EventSource == "" {
		return errors.New("event_source cannot be empty")
	}
	if !goal.EventSource.IsValid() {
		return fmt.Errorf("invalid event_source '%s' (must be 'login' or 'statistic')", goal.EventSource)
	}

	// Validate daily flag (only valid for increment type)
	if goal.Daily && goal.Type != domain.GoalTypeIncrement {
		return fmt.Errorf("daily flag can only be true for increment-type goals (current type: '%s')", goal.Type)
	}

	// Validate requirement
	if goal.Requirement.StatCode == "" {
		return errors.New("stat_code cannot be empty")
	}
	if goal.Requirement.Operator != ">=" {
		return fmt.Errorf("unsupported operator '%s' (only '>=' supported)", goal.Requirement.Operator)
	}
	if goal.Requirement.TargetValue <= 0 {
		return errors.New("target_value must be positive")
	}

	// Validate reward
	if goal.Reward.Type != "ITEM" && goal.Reward.Type != "WALLET" {
		return fmt.Errorf("unsupported reward type '%s' (only 'ITEM' or 'WALLET' allowed)", goal.Reward.Type)
	}
	if goal.Reward.RewardID == "" {
		return errors.New("reward_id cannot be empty")
	}
	if goal.Reward.Quantity <= 0 {
		return errors.New("reward quantity must be positive")
	}

	return nil
}
