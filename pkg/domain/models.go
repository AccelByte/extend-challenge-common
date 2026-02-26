package domain

import "time"

// Challenge represents a collection of goals that users can complete.
// A challenge groups related goals together (e.g., "Winter Challenge", "Daily Quests").
type Challenge struct {
	ID          string  `json:"challengeId"`
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

// Goal represents a single objective that users can complete to earn rewards.
// Goals track progress via stat codes from AGS events.
type Goal struct {
	ID              string      `json:"goalId"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	ChallengeID     string      `json:"challengeId"`     // Parent challenge ID
	EventSource     EventSource `json:"eventSource"`     // Which event stream triggers this goal (login, statistic)
	DefaultAssigned bool        `json:"defaultAssigned"` // M3: Whether goal is assigned by default to new players
	Requirement     Requirement `json:"requirement"`
	Reward          Reward      `json:"reward"`
	Prerequisites   []string    `json:"prerequisites"` // Goal IDs that must be completed first
}

// ProgressMode defines how progress is tracked for a goal's requirement.
//
// Usage in Event Processing:
//   - absolute: Set progress = event.statValue (e.g., kills = 100)
//   - relative: Track incremental progress from a baseline (future: Phase 5 rotation)
//
// In Phase 0.5, both modes route to the same absolute handler.
// The distinction only matters in Phase 5+ when baseline/rotation computation is added.
type ProgressMode string

const (
	// ProgressModeAbsolute tracks progress with absolute stat values.
	// Progress is set to the exact value from the event.
	ProgressModeAbsolute ProgressMode = "absolute"

	// ProgressModeRelative tracks progress relative to a baseline.
	// In Phase 0.5, behaves identically to absolute.
	// In Phase 5+, progress = currentStat - baseline.
	ProgressModeRelative ProgressMode = "relative"
)

// IsValid returns true if the progress mode is a valid type.
func (m ProgressMode) IsValid() bool {
	switch m {
	case ProgressModeAbsolute, ProgressModeRelative:
		return true
	default:
		return false
	}
}

// StatUpdate represents a stat value update from an AGS event.
// Value is the absolute stat value (nil for login events which have no stat).
// Inc is the incremental change (always >= 1), extracted from AGS stat events for future baseline computation.
type StatUpdate struct {
	Value *int // Absolute stat value (nil for login events)
	Inc   int  // Incremental change (always >= 1)
}

// Requirement defines the condition that must be met to complete a goal.
type Requirement struct {
	StatCode     string       `json:"statCode"`               // Event field to track (e.g., "snowman_kills")
	Operator     string       `json:"operator"`               // Comparison operator (only ">=" in M1)
	TargetValue  int          `json:"targetValue"`            // Goal threshold
	ProgressMode ProgressMode `json:"progressMode,omitempty"` // How progress is tracked (absolute, relative)
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
	Type     string `json:"type"`     // "ITEM" or "WALLET"
	RewardID string `json:"rewardId"` // Item code or currency code
	Quantity int    `json:"quantity"` // Amount to grant
}

// UserGoalProgress tracks a user's progress toward completing a specific goal.
// Rows are lazily initialized (created on-demand when progress is first updated).
type UserGoalProgress struct {
	UserID      string     `json:"userId" db:"user_id"`
	GoalID      string     `json:"goalId" db:"goal_id"`
	ChallengeID string     `json:"challengeId" db:"challenge_id"`
	Namespace   string     `json:"namespace" db:"namespace"`
	Progress    int        `json:"progress" db:"progress"`
	Status      GoalStatus `json:"status" db:"status"`
	CompletedAt *time.Time `json:"completedAt,omitempty" db:"completed_at"`
	ClaimedAt   *time.Time `json:"claimedAt,omitempty" db:"claimed_at"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time  `json:"updatedAt" db:"updated_at"`

	// M3: User assignment control
	IsActive   bool       `json:"isActive" db:"is_active"`
	AssignedAt *time.Time `json:"assignedAt,omitempty" db:"assigned_at"`

	// M5: System rotation control (added now for forward compatibility)
	ExpiresAt *time.Time `json:"expiresAt,omitempty" db:"expires_at"`
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
