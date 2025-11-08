package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

func TestConfigLoader_LoadConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("successful load", func(t *testing.T) {
		// Create temp file with valid config
		tmpFile := createTempConfigFile(t, `{
			"challenges": [
				{
					"challengeId": "challenge-1",
					"name": "Challenge 1",
					"description": "Description",
					"goals": [
						{
							"goalId": "goal-1",
							"name": "Goal 1",
							"description": "Description",
							"type": "absolute",
							"eventSource": "statistic",
							"requirement": {
								"statCode": "stat_code",
								"operator": ">=",
								"targetValue": 10
							},
							"reward": {
								"type": "ITEM",
								"rewardId": "item_1",
								"quantity": 1
							},
							"prerequisites": []
						}
					]
				}
			]
		}`)
		defer func() { _ = os.Remove(tmpFile) }()

		loader := NewConfigLoader(tmpFile, logger)
		config, err := loader.LoadConfig()

		if err != nil {
			t.Fatalf("LoadConfig() unexpected error = %v", err)
		}

		if config == nil {
			t.Fatal("LoadConfig() returned nil config")
		}

		if len(config.Challenges) != 1 {
			t.Errorf("expected 1 challenge, got %d", len(config.Challenges))
		}

		// Verify ChallengeID is populated
		if config.Challenges[0].Goals[0].ChallengeID != "challenge-1" {
			t.Errorf("expected ChallengeID to be 'challenge-1', got %q", config.Challenges[0].Goals[0].ChallengeID)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		loader := NewConfigLoader("/nonexistent/file.json", logger)
		_, err := loader.LoadConfig()

		if err == nil {
			t.Fatal("LoadConfig() expected error, got nil")
		}

		if !strings.Contains(err.Error(), "failed to read config file") {
			t.Errorf("expected 'failed to read config file' error, got %v", err)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tmpFile := createTempConfigFile(t, `{invalid json}`)
		defer func() { _ = os.Remove(tmpFile) }()

		loader := NewConfigLoader(tmpFile, logger)
		_, err := loader.LoadConfig()

		if err == nil {
			t.Fatal("LoadConfig() expected error, got nil")
		}

		if !strings.Contains(err.Error(), "failed to parse config JSON") {
			t.Errorf("expected 'failed to parse config JSON' error, got %v", err)
		}
	})

	t.Run("validation failure - empty challenges", func(t *testing.T) {
		tmpFile := createTempConfigFile(t, `{"challenges": []}`)
		defer func() { _ = os.Remove(tmpFile) }()

		loader := NewConfigLoader(tmpFile, logger)
		_, err := loader.LoadConfig()

		if err == nil {
			t.Fatal("LoadConfig() expected error, got nil")
		}

		if !strings.Contains(err.Error(), "config validation failed") {
			t.Errorf("expected 'config validation failed' error, got %v", err)
		}

		if !strings.Contains(err.Error(), "config must have at least one challenge") {
			t.Errorf("expected validation error message, got %v", err)
		}
	})

	t.Run("validation failure - duplicate goal IDs", func(t *testing.T) {
		tmpFile := createTempConfigFile(t, `{
			"challenges": [
				{
					"challengeId": "challenge-1",
					"name": "Challenge 1",
					"description": "Description",
					"goals": [
						{
							"goalId": "goal-1",
							"name": "Goal 1",
							"description": "Description",
							"type": "absolute",
							"eventSource": "statistic",
							"requirement": {
								"statCode": "stat_code",
								"operator": ">=",
								"targetValue": 10
							},
							"reward": {
								"type": "ITEM",
								"rewardId": "item_1",
								"quantity": 1
							},
							"prerequisites": []
						},
						{
							"goalId": "goal-1",
							"name": "Goal 2",
							"description": "Description",
							"type": "absolute",
							"eventSource": "statistic",
							"requirement": {
								"statCode": "stat_code",
								"operator": ">=",
								"targetValue": 10
							},
							"reward": {
								"type": "ITEM",
								"rewardId": "item_1",
								"quantity": 1
							},
							"prerequisites": []
						}
					]
				}
			]
		}`)
		defer func() { _ = os.Remove(tmpFile) }()

		loader := NewConfigLoader(tmpFile, logger)
		_, err := loader.LoadConfig()

		if err == nil {
			t.Fatal("LoadConfig() expected error, got nil")
		}

		if !strings.Contains(err.Error(), "duplicate goal ID") {
			t.Errorf("expected 'duplicate goal ID' error, got %v", err)
		}
	})

	t.Run("multiple challenges with goals", func(t *testing.T) {
		tmpFile := createTempConfigFile(t, `{
			"challenges": [
				{
					"challengeId": "challenge-1",
					"name": "Challenge 1",
					"description": "Description",
					"goals": [
						{
							"goalId": "goal-1",
							"name": "Goal 1",
							"description": "Description",
							"type": "absolute",
							"eventSource": "statistic",
							"requirement": {
								"statCode": "stat_code_1",
								"operator": ">=",
								"targetValue": 10
							},
							"reward": {
								"type": "ITEM",
								"rewardId": "item_1",
								"quantity": 1
							},
							"prerequisites": []
						},
						{
							"goalId": "goal-2",
							"name": "Goal 2",
							"description": "Description",
							"type": "absolute",
							"eventSource": "statistic",
							"requirement": {
								"statCode": "stat_code_2",
								"operator": ">=",
								"targetValue": 20
							},
							"reward": {
								"type": "WALLET",
								"rewardId": "GOLD",
								"quantity": 100
							},
							"prerequisites": ["goal-1"]
						}
					]
				},
				{
					"challengeId": "challenge-2",
					"name": "Challenge 2",
					"description": "Description",
					"goals": [
						{
							"goalId": "goal-3",
							"name": "Goal 3",
							"description": "Description",
							"type": "absolute",
							"eventSource": "statistic",
							"requirement": {
								"statCode": "stat_code_3",
								"operator": ">=",
								"targetValue": 30
							},
							"reward": {
								"type": "ITEM",
								"rewardId": "item_3",
								"quantity": 1
							},
							"prerequisites": []
						}
					]
				}
			]
		}`)
		defer func() { _ = os.Remove(tmpFile) }()

		loader := NewConfigLoader(tmpFile, logger)
		config, err := loader.LoadConfig()

		if err != nil {
			t.Fatalf("LoadConfig() unexpected error = %v", err)
		}

		if len(config.Challenges) != 2 {
			t.Errorf("expected 2 challenges, got %d", len(config.Challenges))
		}

		// Verify ChallengeID is populated for all goals
		if config.Challenges[0].Goals[0].ChallengeID != "challenge-1" {
			t.Errorf("expected goal-1 ChallengeID to be 'challenge-1', got %q", config.Challenges[0].Goals[0].ChallengeID)
		}
		if config.Challenges[0].Goals[1].ChallengeID != "challenge-1" {
			t.Errorf("expected goal-2 ChallengeID to be 'challenge-1', got %q", config.Challenges[0].Goals[1].ChallengeID)
		}
		if config.Challenges[1].Goals[0].ChallengeID != "challenge-2" {
			t.Errorf("expected goal-3 ChallengeID to be 'challenge-2', got %q", config.Challenges[1].Goals[0].ChallengeID)
		}

		// Verify prerequisite is maintained
		if len(config.Challenges[0].Goals[1].Prerequisites) != 1 {
			t.Errorf("expected 1 prerequisite for goal-2, got %d", len(config.Challenges[0].Goals[1].Prerequisites))
		}
		if config.Challenges[0].Goals[1].Prerequisites[0] != "goal-1" {
			t.Errorf("expected prerequisite 'goal-1', got %q", config.Challenges[0].Goals[1].Prerequisites[0])
		}
	})

	t.Run("backward compatibility - config without type field", func(t *testing.T) {
		tmpFile := createTempConfigFile(t, `{
			"challenges": [
				{
					"challengeId": "challenge-1",
					"name": "Challenge 1",
					"description": "Description",
					"goals": [
						{
							"goalId": "goal-1",
							"name": "Goal 1",
							"description": "Description",
							"eventSource": "statistic",
							"requirement": {
								"statCode": "stat_code",
								"operator": ">=",
								"targetValue": 10
							},
							"reward": {
								"type": "ITEM",
								"rewardId": "item_1",
								"quantity": 1
							},
							"prerequisites": []
						}
					]
				}
			]
		}`)
		defer func() { _ = os.Remove(tmpFile) }()

		loader := NewConfigLoader(tmpFile, logger)
		config, err := loader.LoadConfig()

		if err != nil {
			t.Fatalf("LoadConfig() unexpected error = %v", err)
		}

		if config == nil {
			t.Fatal("LoadConfig() returned nil config")
		}

		// Verify type defaults to "absolute"
		if config.Challenges[0].Goals[0].Type != domain.GoalTypeAbsolute {
			t.Errorf("expected type to default to 'absolute', got %q", config.Challenges[0].Goals[0].Type)
		}
	})

	t.Run("default behavior - empty type field defaults to absolute", func(t *testing.T) {
		tmpFile := createTempConfigFile(t, `{
			"challenges": [
				{
					"challengeId": "challenge-1",
					"name": "Challenge 1",
					"description": "Description",
					"goals": [
						{
							"goalId": "goal-1",
							"name": "Goal 1",
							"description": "Description",
							"type": "",
							"eventSource": "statistic",
							"requirement": {
								"statCode": "stat_code",
								"operator": ">=",
								"targetValue": 10
							},
							"reward": {
								"type": "ITEM",
								"rewardId": "item_1",
								"quantity": 1
							},
							"prerequisites": []
						}
					]
				}
			]
		}`)
		defer func() { _ = os.Remove(tmpFile) }()

		loader := NewConfigLoader(tmpFile, logger)
		config, err := loader.LoadConfig()

		if err != nil {
			t.Fatalf("LoadConfig() unexpected error = %v", err)
		}

		// Verify empty type defaults to "absolute"
		if config.Challenges[0].Goals[0].Type != domain.GoalTypeAbsolute {
			t.Errorf("expected empty type to default to 'absolute', got %q", config.Challenges[0].Goals[0].Type)
		}
	})
}

func TestConfigLoader_countGoals(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	loader := NewConfigLoader("/dummy/path", logger)

	tests := []struct {
		name     string
		config   *Config
		expected int
	}{
		{
			name: "no challenges",
			config: &Config{
				Challenges: []*domain.Challenge{},
			},
			expected: 0,
		},
		{
			name: "single challenge with one goal",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						Goals: []*domain.Goal{
							{ID: "goal-1"},
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "multiple challenges with multiple goals",
			config: &Config{
				Challenges: []*domain.Challenge{
					{
						Goals: []*domain.Goal{
							{ID: "goal-1"},
							{ID: "goal-2"},
						},
					},
					{
						Goals: []*domain.Goal{
							{ID: "goal-3"},
						},
					},
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := loader.countGoals(tt.config)
			if result != tt.expected {
				t.Errorf("countGoals() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// Helper function to create a temporary config file for testing
func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "challenges.json")

	err := os.WriteFile(tmpFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	return tmpFile
}
