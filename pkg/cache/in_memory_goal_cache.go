package cache

import (
	"log/slog"
	"sync"

	"github.com/AccelByte/extend-challenge-common/pkg/config"
	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// InMemoryGoalCache provides O(1) in-memory lookups for goal configurations.
// All maps are built at startup and provide thread-safe read access.
// This cache is immutable after construction (reload requires application restart in M1).
type InMemoryGoalCache struct {
	goalsByID       map[string]*domain.Goal      // "goal-id" -> Goal
	goalsByStatCode map[string][]*domain.Goal    // "stat_code" -> [Goals]
	challengesByID  map[string]*domain.Challenge // "challenge-id" -> Challenge
	challenges      []*domain.Challenge          // All challenges (ordered)
	configPath      string                       // Path to config file (for reload)
	mu              sync.RWMutex                 // Protects all maps
	logger          *slog.Logger
}

// NewInMemoryGoalCache creates a new cache from the provided configuration.
// The cache is immediately built and ready for lookups.
//
// Parameters:
//   - cfg: Validated configuration containing challenges and goals
//   - configPath: Path to config file (used for reload operation)
//   - logger: Structured logger for operational logging
//
// Returns:
//   - *InMemoryGoalCache: Ready-to-use cache with all indexes built
func NewInMemoryGoalCache(cfg *config.Config, configPath string, logger *slog.Logger) *InMemoryGoalCache {
	cache := &InMemoryGoalCache{
		goalsByID:       make(map[string]*domain.Goal),
		goalsByStatCode: make(map[string][]*domain.Goal),
		challengesByID:  make(map[string]*domain.Challenge),
		challenges:      make([]*domain.Challenge, 0, len(cfg.Challenges)),
		configPath:      configPath,
		logger:          logger,
	}

	cache.buildCache(cfg)

	return cache
}

// buildCache constructs all cache indexes from the configuration.
// This method is called during construction and reload.
// It replaces all existing cache data.
func (c *InMemoryGoalCache) buildCache(cfg *config.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear existing cache
	c.goalsByID = make(map[string]*domain.Goal)
	c.goalsByStatCode = make(map[string][]*domain.Goal)
	c.challengesByID = make(map[string]*domain.Challenge)
	c.challenges = make([]*domain.Challenge, 0, len(cfg.Challenges))

	// Build indexes
	for _, challenge := range cfg.Challenges {
		// Index challenge by ID
		c.challengesByID[challenge.ID] = challenge
		c.challenges = append(c.challenges, challenge)

		for _, goal := range challenge.Goals {
			// Index goal by ID
			c.goalsByID[goal.ID] = goal

			// Index goal by stat code (multiple goals can track same stat)
			statCode := goal.Requirement.StatCode
			c.goalsByStatCode[statCode] = append(c.goalsByStatCode[statCode], goal)
		}
	}

	c.logger.Info("Cache built successfully",
		"challenges", len(c.challenges),
		"goals", len(c.goalsByID),
		"stat_codes", len(c.goalsByStatCode),
	)
}

// GetGoalByID retrieves a goal by its unique ID.
// Returns nil if the goal does not exist.
// Time complexity: O(1)
func (c *InMemoryGoalCache) GetGoalByID(goalID string) *domain.Goal {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.goalsByID[goalID]
}

// GetGoalsByStatCode retrieves all goals that track a specific stat code.
// Multiple goals can track the same stat (e.g., multiple challenges tracking "login_count").
// Returns an empty slice if no goals track this stat.
// Time complexity: O(1)
func (c *InMemoryGoalCache) GetGoalsByStatCode(statCode string) []*domain.Goal {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to prevent external modification
	goals := c.goalsByStatCode[statCode]
	if goals == nil {
		return []*domain.Goal{}
	}

	// Return the slice directly - it's safe because Goals are immutable
	return goals
}

// GetChallengeByChallengeID retrieves a challenge by its unique ID.
// Returns nil if the challenge does not exist.
// Time complexity: O(1)
func (c *InMemoryGoalCache) GetChallengeByChallengeID(challengeID string) *domain.Challenge {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.challengesByID[challengeID]
}

// GetAllChallenges retrieves all configured challenges.
// Returns all challenges in the order they appear in the config file.
// Time complexity: O(1)
func (c *InMemoryGoalCache) GetAllChallenges() []*domain.Challenge {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return the slice directly - it's safe because Challenges are immutable
	return c.challenges
}

// GetAllGoals retrieves all configured goals across all challenges.
// This is useful for filtering goals by properties like event_source.
// Returns all goals flattened from all challenges.
// Time complexity: O(n) where n is total number of goals
func (c *InMemoryGoalCache) GetAllGoals() []*domain.Goal {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Flatten goals from all challenges
	allGoals := make([]*domain.Goal, 0, len(c.goalsByID))
	for _, goal := range c.goalsByID {
		allGoals = append(allGoals, goal)
	}

	return allGoals
}

// Reload reloads the cache from the config file.
// In M1, this requires application restart (config is baked into Docker image).
// This method is provided for future use when hot-reload is supported.
//
// Returns:
//   - error: If config file cannot be read or validation fails
func (c *InMemoryGoalCache) Reload() error {
	// Load config from file
	loader := config.NewConfigLoader(c.configPath, c.logger)
	newConfig, err := loader.LoadConfig()
	if err != nil {
		return err
	}

	// Rebuild cache
	c.buildCache(newConfig)

	c.logger.Info("Cache reloaded successfully")

	return nil
}
