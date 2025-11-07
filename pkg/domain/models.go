package domain

import "time"

// Challenge represents a collection of goals that users can complete.
// A challenge groups related goals together (e.g., "Winter Challenge", "Daily Quests").
type Challenge struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Goals       []*Goal `json:"goals"`
}

// EventSource defines which event stream triggers progress updates for a goal.
type EventSource string

const (
	// EventSourceLogin indicates the goal is triggered by IAM login events.
	// Event: {namespace}.iam.account.v1.userLoggedIn
	// Use cases: Daily login rewards, login streaks, total login count
	EventSourceLogin EventSource = "login"

	// EventSourceStatistic indicates the goal is triggered by statistic update events.
	// Event: {namespace}.social.statistic.v1.statItemUpdated
	// Use cases: Kills, wins, score, level, etc.
	EventSourceStatistic EventSource = "statistic"
)

// IsValid returns true if the event source is a valid type.
func (e EventSource) IsValid() bool {
	switch e {
	case EventSourceLogin, EventSourceStatistic:
		return true
	default:
		return false
	}
}

// GoalType defines how progress is tracked for a goal.
//
// Usage in Event Processing:
//   - absolute: Set progress = event.statValue (e.g., kills = 100)
//   - increment (daily=false): progress = progress + 1 (every event)
//   - increment (daily=true): progress = progress + 1 (once per day)
//   - daily: Set completed_at = NOW() (progress unused)
//
// Usage in Claim Validation:
//   - absolute/increment: Check progress >= target_value
//   - daily: Check DATE(completed_at) = TODAY
type GoalType string

const (
	// GoalTypeAbsolute tracks progress with absolute values (e.g., "kill 100 enemies").
	// Progress is set to the exact value from the event.
	// Example: Stat "kills" = 50 → progress = 50, then "kills" = 100 → progress = 100.
	GoalTypeAbsolute GoalType = "absolute"

	// GoalTypeIncrement tracks progress with incremental updates (e.g., "login 7 times").
	// Progress is incremented by a delta value each time.
	// When Daily flag is true: Only increments once per day (e.g., "login 7 different days").
	// When Daily flag is false: Increments on every event (e.g., "login 100 total times").
	GoalTypeIncrement GoalType = "increment"

	// GoalTypeDaily tracks whether an event occurred today (e.g., "daily login reward").
	// Stores last event timestamp in completed_at. Claim validates if completed_at date equals today.
	// Progress value is not used for completion check. Suitable for daily repeating rewards.
	GoalTypeDaily GoalType = "daily"
)

// IsValid returns true if the goal type is a valid type.
func (t GoalType) IsValid() bool {
	switch t {
	case GoalTypeAbsolute, GoalTypeIncrement, GoalTypeDaily:
		return true
	default:
		return false
	}
}

// Goal represents a single objective that users can complete to earn rewards.
// Goals track progress via stat codes from AGS events.
type Goal struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	ChallengeID     string      `json:"challenge_id"`     // Parent challenge ID
	Type            GoalType    `json:"type"`             // How progress is tracked (absolute, increment, daily)
	EventSource     EventSource `json:"event_source"`     // Which event stream triggers this goal (login, statistic)
	Daily           bool        `json:"daily"`            // For increment type: true = count once per day, false = count every occurrence
	DefaultAssigned bool        `json:"default_assigned"` // M3: Whether goal is assigned by default to new players
	Requirement     Requirement `json:"requirement"`
	Reward          Reward      `json:"reward"`
	Prerequisites   []string    `json:"prerequisites"` // Goal IDs that must be completed first
}

// Requirement defines the condition that must be met to complete a goal.
type Requirement struct {
	StatCode    string `json:"stat_code"`    // Event field to track (e.g., "snowman_kills")
	Operator    string `json:"operator"`     // Comparison operator (only ">=" in M1)
	TargetValue int    `json:"target_value"` // Goal threshold
}

// RewardType defines the type of reward granted to the user.
type RewardType string

const (
	// RewardTypeItem grants an item from the Platform Service item catalog.
	RewardTypeItem RewardType = "ITEM"

	// RewardTypeWallet grants currency to the user's wallet via Platform Service.
	RewardTypeWallet RewardType = "WALLET"
)

// Reward defines what the user receives upon claiming a completed goal.
type Reward struct {
	Type     string `json:"type"`      // "ITEM" or "WALLET"
	RewardID string `json:"reward_id"` // Item code or currency code
	Quantity int    `json:"quantity"`  // Amount to grant
}

// UserGoalProgress tracks a user's progress toward completing a specific goal.
// Rows are lazily initialized (created on-demand when progress is first updated).
type UserGoalProgress struct {
	UserID      string     `json:"user_id" db:"user_id"`
	GoalID      string     `json:"goal_id" db:"goal_id"`
	ChallengeID string     `json:"challenge_id" db:"challenge_id"`
	Namespace   string     `json:"namespace" db:"namespace"`
	Progress    int        `json:"progress" db:"progress"`
	Status      GoalStatus `json:"status" db:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	ClaimedAt   *time.Time `json:"claimed_at,omitempty" db:"claimed_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`

	// M3: User assignment control
	IsActive   bool       `json:"is_active" db:"is_active"`
	AssignedAt *time.Time `json:"assigned_at,omitempty" db:"assigned_at"`

	// M5: System rotation control (added now for forward compatibility)
	ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`
}

// GoalStatus represents the current state of a user's progress on a goal.
type GoalStatus string

const (
	// GoalStatusNotStarted indicates the user has not made any progress.
	GoalStatusNotStarted GoalStatus = "not_started"

	// GoalStatusInProgress indicates the user is actively working on the goal.
	GoalStatusInProgress GoalStatus = "in_progress"

	// GoalStatusCompleted indicates the goal requirement has been met but reward not claimed.
	GoalStatusCompleted GoalStatus = "completed"

	// GoalStatusClaimed indicates the goal is completed and reward has been granted.
	GoalStatusClaimed GoalStatus = "claimed"
)

// IsValid returns true if the status is a valid goal status.
func (s GoalStatus) IsValid() bool {
	switch s {
	case GoalStatusNotStarted, GoalStatusInProgress, GoalStatusCompleted, GoalStatusClaimed:
		return true
	default:
		return false
	}
}

// IsCompleted returns true if the goal is in completed or claimed status.
func (p *UserGoalProgress) IsCompleted() bool {
	return p.Status == GoalStatusCompleted || p.Status == GoalStatusClaimed
}

// IsClaimed returns true if the reward has been claimed.
func (p *UserGoalProgress) IsClaimed() bool {
	return p.Status == GoalStatusClaimed
}

// CanClaim returns true if the goal can be claimed (completed but not yet claimed).
// M3 Phase 6: Goal must be active and completed to claim.
func (p *UserGoalProgress) CanClaim() bool {
	return p.IsActive && p.Status == GoalStatusCompleted
}

// MeetsRequirement returns true if the current progress meets the goal's requirement.
func (p *UserGoalProgress) MeetsRequirement(requirement Requirement) bool {
	// In M1, only ">=" operator is supported
	if requirement.Operator == ">=" {
		return p.Progress >= requirement.TargetValue
	}
	return false
}
