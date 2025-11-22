package errors

import "fmt"

// Error codes for the challenge service.
const (
	// Domain errors
	ErrCodeGoalNotFound       = "GOAL_NOT_FOUND"
	ErrCodeChallengeNotFound  = "CHALLENGE_NOT_FOUND"
	ErrCodeGoalAlreadyClaimed = "GOAL_ALREADY_CLAIMED"
	ErrCodeGoalNotCompleted   = "GOAL_NOT_COMPLETED"
	ErrCodeInvalidStatus      = "INVALID_STATUS"

	// Database errors
	ErrCodeDatabaseError     = "DATABASE_ERROR"
	ErrCodeTransactionFailed = "TRANSACTION_FAILED"

	// Config errors
	ErrCodeConfigInvalid  = "CONFIG_INVALID"
	ErrCodeConfigNotFound = "CONFIG_NOT_FOUND"

	// AGS integration errors
	ErrCodeRewardGrantFailed = "REWARD_GRANT_FAILED"
	ErrCodeAuthFailed        = "AUTH_FAILED"

	// Validation errors
	ErrCodeValidationFailed = "VALIDATION_FAILED"
	ErrCodeInvalidInput     = "INVALID_INPUT"

	// M4: Goal selection errors
	ErrCodeInsufficientGoals = "INSUFFICIENT_GOALS"
)

// ChallengeError represents an error in the challenge service.
type ChallengeError struct {
	Code    string
	Message string
	Err     error
}

func (e *ChallengeError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ChallengeError) Unwrap() error {
	return e.Err
}

// NewChallengeError creates a new ChallengeError.
func NewChallengeError(code, message string, err error) *ChallengeError {
	return &ChallengeError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Domain-specific error constructors

// ErrGoalNotFound returns an error when a goal is not found.
func ErrGoalNotFound(goalID string) *ChallengeError {
	return &ChallengeError{
		Code:    ErrCodeGoalNotFound,
		Message: fmt.Sprintf("goal not found: %s", goalID),
		Err:     nil,
	}
}

// ErrChallengeNotFound returns an error when a challenge is not found.
func ErrChallengeNotFound(challengeID string) *ChallengeError {
	return &ChallengeError{
		Code:    ErrCodeChallengeNotFound,
		Message: fmt.Sprintf("challenge not found: %s", challengeID),
		Err:     nil,
	}
}

// ErrGoalAlreadyClaimed returns an error when attempting to claim an already claimed goal.
func ErrGoalAlreadyClaimed(goalID string) *ChallengeError {
	return &ChallengeError{
		Code:    ErrCodeGoalAlreadyClaimed,
		Message: fmt.Sprintf("goal already claimed: %s", goalID),
		Err:     nil,
	}
}

// ErrGoalNotCompleted returns an error when attempting to claim an incomplete goal.
func ErrGoalNotCompleted(goalID string) *ChallengeError {
	return &ChallengeError{
		Code:    ErrCodeGoalNotCompleted,
		Message: fmt.Sprintf("goal not completed: %s", goalID),
		Err:     nil,
	}
}

// ErrDatabaseError wraps database errors.
func ErrDatabaseError(operation string, err error) *ChallengeError {
	return &ChallengeError{
		Code:    ErrCodeDatabaseError,
		Message: fmt.Sprintf("database error during %s", operation),
		Err:     err,
	}
}

// ErrConfigInvalid returns an error for invalid configuration.
func ErrConfigInvalid(reason string) *ChallengeError {
	return &ChallengeError{
		Code:    ErrCodeConfigInvalid,
		Message: fmt.Sprintf("invalid configuration: %s", reason),
		Err:     nil,
	}
}

// ErrRewardGrantFailed returns an error when reward grant fails.
func ErrRewardGrantFailed(rewardType, rewardID string, err error) *ChallengeError {
	return &ChallengeError{
		Code:    ErrCodeRewardGrantFailed,
		Message: fmt.Sprintf("failed to grant %s reward: %s", rewardType, rewardID),
		Err:     err,
	}
}

// ErrValidationFailed returns a validation error.
func ErrValidationFailed(field, reason string) *ChallengeError {
	return &ChallengeError{
		Code:    ErrCodeValidationFailed,
		Message: fmt.Sprintf("validation failed for %s: %s", field, reason),
		Err:     nil,
	}
}

// ErrInsufficientGoals returns an error when not enough goals are available for selection.
func ErrInsufficientGoals(available, requested int) *ChallengeError {
	return &ChallengeError{
		Code:    ErrCodeInsufficientGoals,
		Message: fmt.Sprintf("no goals available for selection (available: %d, requested: %d)", available, requested),
		Err:     nil,
	}
}
