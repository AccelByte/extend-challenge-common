package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// ConfigLoader loads and validates challenge configuration from a JSON file.
// It performs file reading, JSON parsing, and comprehensive validation.
type ConfigLoader struct {
	configPath string
	validator  *Validator
	logger     *slog.Logger
}

// NewConfigLoader creates a new ConfigLoader instance.
//
// Parameters:
//   - configPath: Path to the challenges.json file
//   - logger: Structured logger for operational logging
func NewConfigLoader(configPath string, logger *slog.Logger) *ConfigLoader {
	return &ConfigLoader{
		configPath: configPath,
		validator:  NewValidator(),
		logger:     logger,
	}
}

// LoadConfig loads the configuration file and returns a validated Config.
// This method performs three steps:
// 1. Read the config file from disk
// 2. Parse JSON into Config struct
// 3. Validate all business rules
//
// If any step fails, returns an error and the application should exit.
// This is a "fail fast" operation - invalid config prevents startup.
//
// Returns:
//   - *Config: Valid configuration ready for use
//   - error: Descriptive error if loading or validation fails
func (l *ConfigLoader) LoadConfig() (*Config, error) {
	// Step 1: Read file
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Step 2: Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Step 3: Populate ChallengeID and default ProgressMode
	// This links each goal to its parent challenge for easier lookups
	// and provides a sensible default for configs without explicit progressMode
	for _, challenge := range config.Challenges {
		for _, goal := range challenge.Goals {
			goal.ChallengeID = challenge.ID
			// Default to "absolute" if progressMode is empty
			if goal.Requirement.ProgressMode == "" {
				goal.Requirement.ProgressMode = domain.ProgressModeAbsolute
			}
		}
	}

	// Step 4: Validate
	if err := l.validator.Validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Log success
	totalGoals, rotatingGoals := l.countGoals(&config)
	l.logger.Info("Config loaded successfully",
		"challenges", len(config.Challenges),
		"total_goals", totalGoals,
		"rotating_goals", rotatingGoals,
		"config_path", l.configPath,
	)

	return &config, nil
}

// countGoals counts the total number of goals and rotating goals across all challenges.
func (l *ConfigLoader) countGoals(config *Config) (total int, rotating int) {
	for _, challenge := range config.Challenges {
		for _, goal := range challenge.Goals {
			total++
			if goal.Rotation != nil && goal.Rotation.Enabled {
				rotating++
			}
		}
	}
	return total, rotating
}
