package cache

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/AccelByte/extend-challenge-common/pkg/config"
	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

func TestNewInMemoryGoalCache(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := createTestConfig()

	cache := NewInMemoryGoalCache(cfg, "/path/to/config.json", logger)

	if cache == nil {
		t.Fatal("NewInMemoryGoalCache() returned nil")
	}

	// Verify cache was built
	if len(cache.goalsByID) != 3 {
		t.Errorf("expected 3 goals in cache, got %d", len(cache.goalsByID))
	}

	if len(cache.challengesByID) != 2 {
		t.Errorf("expected 2 challenges in cache, got %d", len(cache.challengesByID))
	}

	if len(cache.challenges) != 2 {
		t.Errorf("expected 2 challenges in slice, got %d", len(cache.challenges))
	}
}

func TestInMemoryGoalCache_GetGoalByID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := createTestConfig()
	cache := NewInMemoryGoalCache(cfg, "/path/to/config.json", logger)

	t.Run("existing goal", func(t *testing.T) {
		goal := cache.GetGoalByID("goal-1")

		if goal == nil {
			t.Fatal("GetGoalByID() returned nil for existing goal")
		}

		if goal.ID != "goal-1" {
			t.Errorf("expected goal ID 'goal-1', got %q", goal.ID)
		}

		if goal.Name != "Goal 1" {
			t.Errorf("expected goal name 'Goal 1', got %q", goal.Name)
		}
	})

	t.Run("non-existing goal", func(t *testing.T) {
		goal := cache.GetGoalByID("nonexistent")

		if goal != nil {
			t.Errorf("GetGoalByID() expected nil for non-existing goal, got %v", goal)
		}
	})
}

func TestInMemoryGoalCache_GetGoalsByStatCode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := createTestConfig()
	cache := NewInMemoryGoalCache(cfg, "/path/to/config.json", logger)

	t.Run("existing stat code - single goal", func(t *testing.T) {
		goals := cache.GetGoalsByStatCode("stat_code_2")

		if len(goals) != 1 {
			t.Fatalf("expected 1 goal, got %d", len(goals))
		}

		if goals[0].ID != "goal-2" {
			t.Errorf("expected goal ID 'goal-2', got %q", goals[0].ID)
		}
	})

	t.Run("existing stat code - multiple goals", func(t *testing.T) {
		goals := cache.GetGoalsByStatCode("stat_code_1")

		if len(goals) != 2 {
			t.Fatalf("expected 2 goals, got %d", len(goals))
		}

		// Verify both goals are present (order doesn't matter)
		goalIDs := make(map[string]bool)
		for _, goal := range goals {
			goalIDs[goal.ID] = true
		}

		if !goalIDs["goal-1"] || !goalIDs["goal-3"] {
			t.Errorf("expected goals 'goal-1' and 'goal-3', got %v", goalIDs)
		}
	})

	t.Run("non-existing stat code", func(t *testing.T) {
		goals := cache.GetGoalsByStatCode("nonexistent")

		if len(goals) != 0 {
			t.Errorf("expected empty slice for non-existing stat code, got %d goals", len(goals))
		}
	})
}

func TestInMemoryGoalCache_GetChallengeByChallengeID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := createTestConfig()
	cache := NewInMemoryGoalCache(cfg, "/path/to/config.json", logger)

	t.Run("existing challenge", func(t *testing.T) {
		challenge := cache.GetChallengeByChallengeID("challenge-1")

		if challenge == nil {
			t.Fatal("GetChallengeByChallengeID() returned nil for existing challenge")
		}

		if challenge.ID != "challenge-1" {
			t.Errorf("expected challenge ID 'challenge-1', got %q", challenge.ID)
		}

		if challenge.Name != "Challenge 1" {
			t.Errorf("expected challenge name 'Challenge 1', got %q", challenge.Name)
		}

		if len(challenge.Goals) != 2 {
			t.Errorf("expected 2 goals in challenge, got %d", len(challenge.Goals))
		}
	})

	t.Run("non-existing challenge", func(t *testing.T) {
		challenge := cache.GetChallengeByChallengeID("nonexistent")

		if challenge != nil {
			t.Errorf("GetChallengeByChallengeID() expected nil for non-existing challenge, got %v", challenge)
		}
	})
}

func TestInMemoryGoalCache_GetAllChallenges(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := createTestConfig()
	cache := NewInMemoryGoalCache(cfg, "/path/to/config.json", logger)

	challenges := cache.GetAllChallenges()

	if len(challenges) != 2 {
		t.Fatalf("expected 2 challenges, got %d", len(challenges))
	}

	// Verify challenges are in order
	if challenges[0].ID != "challenge-1" {
		t.Errorf("expected first challenge ID 'challenge-1', got %q", challenges[0].ID)
	}

	if challenges[1].ID != "challenge-2" {
		t.Errorf("expected second challenge ID 'challenge-2', got %q", challenges[1].ID)
	}
}

func TestInMemoryGoalCache_GetAllGoals(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := createTestConfig()
	cache := NewInMemoryGoalCache(cfg, "/path/to/config.json", logger)

	goals := cache.GetAllGoals()

	if len(goals) != 3 {
		t.Fatalf("expected 3 goals, got %d", len(goals))
	}

	// Verify all goals are present (order not guaranteed since it comes from map)
	goalIDs := make(map[string]bool)
	for _, goal := range goals {
		goalIDs[goal.ID] = true
	}

	expectedIDs := []string{"goal-1", "goal-2", "goal-3"}
	for _, expectedID := range expectedIDs {
		if !goalIDs[expectedID] {
			t.Errorf("expected goal %q to be in results", expectedID)
		}
	}

	// Verify goals have correct challenge references
	for _, goal := range goals {
		if goal.ChallengeID == "" {
			t.Errorf("goal %q has empty challenge_id", goal.ID)
		}
	}
}

func TestInMemoryGoalCache_Reload(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("successful reload", func(t *testing.T) {
		// Create temp config file
		tmpFile := createTempConfigFile(t, `{
			"challenges": [
				{
					"id": "challenge-new",
					"name": "New Challenge",
					"description": "Description",
					"goals": [
						{
							"id": "goal-new",
							"name": "New Goal",
							"description": "Description",
							"type": "absolute",
							"event_source": "statistic",
							"requirement": {
								"stat_code": "new_stat",
								"operator": ">=",
								"target_value": 100
							},
							"reward": {
								"type": "ITEM",
								"reward_id": "new_item",
								"quantity": 1
							},
							"prerequisites": []
						}
					]
				}
			]
		}`)
		defer func() { _ = os.Remove(tmpFile) }()

		// Create cache with initial config
		cfg := createTestConfig()
		cache := NewInMemoryGoalCache(cfg, tmpFile, logger)

		// Verify initial state
		if cache.GetGoalByID("goal-new") != nil {
			t.Error("goal-new should not exist before reload")
		}

		// Reload
		err := cache.Reload()
		if err != nil {
			t.Fatalf("Reload() unexpected error = %v", err)
		}

		// Verify new config is loaded
		goal := cache.GetGoalByID("goal-new")
		if goal == nil {
			t.Fatal("goal-new should exist after reload")
		}

		if goal.Name != "New Goal" {
			t.Errorf("expected goal name 'New Goal', got %q", goal.Name)
		}

		// Verify old goals are gone
		if cache.GetGoalByID("goal-1") != nil {
			t.Error("goal-1 should not exist after reload")
		}
	})

	t.Run("failed reload - file not found", func(t *testing.T) {
		cfg := createTestConfig()
		cache := NewInMemoryGoalCache(cfg, "/nonexistent/file.json", logger)

		err := cache.Reload()
		if err == nil {
			t.Error("Reload() expected error for non-existent file, got nil")
		}

		// Verify cache still has original data
		if cache.GetGoalByID("goal-1") == nil {
			t.Error("goal-1 should still exist after failed reload")
		}
	})

	t.Run("failed reload - invalid JSON", func(t *testing.T) {
		tmpFile := createTempConfigFile(t, `{invalid json}`)
		defer func() { _ = os.Remove(tmpFile) }()

		cfg := createTestConfig()
		cache := NewInMemoryGoalCache(cfg, tmpFile, logger)

		err := cache.Reload()
		if err == nil {
			t.Error("Reload() expected error for invalid JSON, got nil")
		}

		// Verify cache still has original data
		if cache.GetGoalByID("goal-1") == nil {
			t.Error("goal-1 should still exist after failed reload")
		}
	})
}

func TestInMemoryGoalCache_ThreadSafety(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := createTestConfig()
	cache := NewInMemoryGoalCache(cfg, "/path/to/config.json", logger)

	// Run concurrent reads to verify thread-safety
	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(4)

		go func() {
			defer wg.Done()
			_ = cache.GetGoalByID("goal-1")
		}()

		go func() {
			defer wg.Done()
			_ = cache.GetGoalsByStatCode("stat_code_1")
		}()

		go func() {
			defer wg.Done()
			_ = cache.GetChallengeByChallengeID("challenge-1")
		}()

		go func() {
			defer wg.Done()
			_ = cache.GetAllChallenges()
		}()
	}

	wg.Wait()

	// If we reach here without deadlock or panic, thread-safety test passed
	t.Log("Thread-safety test completed successfully")
}

// Helper function to create a test config
func createTestConfig() *config.Config {
	return &config.Config{
		Challenges: []*domain.Challenge{
			{
				ID:          "challenge-1",
				Name:        "Challenge 1",
				Description: "Description 1",
				Goals: []*domain.Goal{
					{
						ID:          "goal-1",
						Name:        "Goal 1",
						Description: "Description",
						ChallengeID: "challenge-1",
						Type:        domain.GoalTypeAbsolute,
						EventSource: domain.EventSourceStatistic,
						Requirement: domain.Requirement{
							StatCode:    "stat_code_1",
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
						Description: "Description",
						ChallengeID: "challenge-1",
						Type:        domain.GoalTypeAbsolute,
						EventSource: domain.EventSourceStatistic,
						Requirement: domain.Requirement{
							StatCode:    "stat_code_2",
							Operator:    ">=",
							TargetValue: 20,
						},
						Reward: domain.Reward{
							Type:     "WALLET",
							RewardID: "GOLD",
							Quantity: 100,
						},
						Prerequisites: []string{"goal-1"},
					},
				},
			},
			{
				ID:          "challenge-2",
				Name:        "Challenge 2",
				Description: "Description 2",
				Goals: []*domain.Goal{
					{
						ID:          "goal-3",
						Name:        "Goal 3",
						Description: "Description",
						ChallengeID: "challenge-2",
						Type:        domain.GoalTypeAbsolute,
						EventSource: domain.EventSourceStatistic,
						Requirement: domain.Requirement{
							StatCode:    "stat_code_1", // Same stat code as goal-1
							Operator:    ">=",
							TargetValue: 30,
						},
						Reward: domain.Reward{
							Type:     "ITEM",
							RewardID: "item_3",
							Quantity: 1,
						},
						Prerequisites: []string{},
					},
				},
			},
		},
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

// M3: Test GetGoalsWithDefaultAssigned method
func TestInMemoryGoalCache_GetGoalsWithDefaultAssigned(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create config with mix of default_assigned goals
	cfg := &config.Config{
		Challenges: []*domain.Challenge{
			{
				ID:          "challenge-1",
				Name:        "Challenge 1",
				Description: "Description",
				Goals: []*domain.Goal{
					{
						ID:              "goal-1-default",
						Name:            "Default Goal 1",
						Description:     "Assigned by default",
						ChallengeID:     "challenge-1",
						Type:            domain.GoalTypeAbsolute,
						EventSource:     domain.EventSourceStatistic,
						DefaultAssigned: true, // M3: Default assigned
						Requirement: domain.Requirement{
							StatCode:    "stat_code_1",
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
						ID:              "goal-2-manual",
						Name:            "Manual Goal",
						Description:     "Not assigned by default",
						ChallengeID:     "challenge-1",
						Type:            domain.GoalTypeAbsolute,
						EventSource:     domain.EventSourceStatistic,
						DefaultAssigned: false, // M3: Not default assigned
						Requirement: domain.Requirement{
							StatCode:    "stat_code_2",
							Operator:    ">=",
							TargetValue: 20,
						},
						Reward: domain.Reward{
							Type:     "WALLET",
							RewardID: "GOLD",
							Quantity: 100,
						},
					},
				},
			},
			{
				ID:          "challenge-2",
				Name:        "Challenge 2",
				Description: "Description",
				Goals: []*domain.Goal{
					{
						ID:              "goal-3-default",
						Name:            "Default Goal 2",
						Description:     "Also assigned by default",
						ChallengeID:     "challenge-2",
						Type:            domain.GoalTypeIncrement,
						EventSource:     domain.EventSourceLogin,
						DefaultAssigned: true, // M3: Default assigned
						Requirement: domain.Requirement{
							StatCode:    "login_count",
							Operator:    ">=",
							TargetValue: 7,
						},
						Reward: domain.Reward{
							Type:     "ITEM",
							RewardID: "item_3",
							Quantity: 1,
						},
					},
				},
			},
		},
	}

	cache := NewInMemoryGoalCache(cfg, "/path/to/config.json", logger)

	t.Run("returns only default assigned goals", func(t *testing.T) {
		defaultGoals := cache.GetGoalsWithDefaultAssigned()

		// Should return only goals with DefaultAssigned = true
		if len(defaultGoals) != 2 {
			t.Fatalf("expected 2 default goals, got %d", len(defaultGoals))
		}

		// Verify the goals are the ones marked as default_assigned
		foundGoal1 := false
		foundGoal3 := false

		for _, goal := range defaultGoals {
			if !goal.DefaultAssigned {
				t.Errorf("goal %s should have DefaultAssigned=true", goal.ID)
			}

			if goal.ID == "goal-1-default" {
				foundGoal1 = true
			}

			if goal.ID == "goal-3-default" {
				foundGoal3 = true
			}

			// Ensure manual goal is not included
			if goal.ID == "goal-2-manual" {
				t.Errorf("manual goal %s should not be in default goals list", goal.ID)
			}
		}

		if !foundGoal1 {
			t.Error("expected to find goal-1-default in default goals list")
		}

		if !foundGoal3 {
			t.Error("expected to find goal-3-default in default goals list")
		}
	})

	t.Run("returns empty slice when no default goals", func(t *testing.T) {
		// Create config with no default assigned goals
		cfgNoDefaults := &config.Config{
			Challenges: []*domain.Challenge{
				{
					ID:          "challenge-1",
					Name:        "Challenge 1",
					Description: "Description",
					Goals: []*domain.Goal{
						{
							ID:              "goal-1",
							Name:            "Goal 1",
							Description:     "Not default",
							ChallengeID:     "challenge-1",
							Type:            domain.GoalTypeAbsolute,
							EventSource:     domain.EventSourceStatistic,
							DefaultAssigned: false,
							Requirement: domain.Requirement{
								StatCode:    "stat_code_1",
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
		}

		cacheNoDefaults := NewInMemoryGoalCache(cfgNoDefaults, "/path/to/config.json", logger)
		defaultGoals := cacheNoDefaults.GetGoalsWithDefaultAssigned()

		if len(defaultGoals) != 0 {
			t.Errorf("expected 0 default goals, got %d", len(defaultGoals))
		}
	})
}
