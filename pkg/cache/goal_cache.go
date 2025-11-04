package cache

import "github.com/AccelByte/extend-challenge-common/pkg/domain"

// GoalCache provides O(1) in-memory lookups for goal configurations.
// This cache is built at application startup from the challenges.json config file.
// All lookups are read-only and thread-safe.
type GoalCache interface {
	// GetGoalByID retrieves a goal by its unique ID.
	// Returns nil if goal does not exist.
	// Time complexity: O(1)
	GetGoalByID(goalID string) *domain.Goal

	// GetGoalsByStatCode retrieves all goals that track a specific stat code.
	// Multiple goals can track the same stat (e.g., multiple challenges tracking "login_count").
	// Returns empty slice if no goals track this stat.
	// Time complexity: O(1)
	GetGoalsByStatCode(statCode string) []*domain.Goal

	// GetChallengeByChallengeID retrieves a challenge by its unique ID.
	// Returns nil if challenge does not exist.
	// Time complexity: O(1)
	GetChallengeByChallengeID(challengeID string) *domain.Challenge

	// GetAllChallenges retrieves all configured challenges.
	// Returns all challenges in the order they appear in the config file.
	// Time complexity: O(1)
	GetAllChallenges() []*domain.Challenge

	// GetAllGoals retrieves all configured goals across all challenges.
	// Useful for filtering goals by properties like event_source.
	// Returns all goals flattened from all challenges.
	// Time complexity: O(n) where n is total number of goals
	GetAllGoals() []*domain.Goal

	// M3: GetGoalsWithDefaultAssigned retrieves all goals that have default_assigned = true.
	// Used by initialization endpoint to determine which goals to assign to new players.
	// Returns empty slice if no goals are marked as default assigned.
	// Time complexity: O(n) where n is total number of goals
	GetGoalsWithDefaultAssigned() []*domain.Goal

	// Reload reloads the cache from the config file.
	// In M1, this requires application restart (config is baked into Docker image).
	// Returns error if config file cannot be read or is invalid.
	Reload() error
}
