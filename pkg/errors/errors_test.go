package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestChallengeError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *ChallengeError
		wantMsg string
	}{
		{
			name: "error without wrapped error",
			err: &ChallengeError{
				Code:    ErrCodeGoalNotFound,
				Message: "goal not found: test-goal",
				Err:     nil,
			},
			wantMsg: "GOAL_NOT_FOUND: goal not found: test-goal",
		},
		{
			name: "error with wrapped error",
			err: &ChallengeError{
				Code:    ErrCodeDatabaseError,
				Message: "database error during query",
				Err:     errors.New("connection timeout"),
			},
			wantMsg: "DATABASE_ERROR: database error during query: connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantMsg {
				t.Errorf("ChallengeError.Error() = %v, want %v", got, tt.wantMsg)
			}
		})
	}
}

func TestChallengeError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	err := &ChallengeError{
		Code:    ErrCodeDatabaseError,
		Message: "test error",
		Err:     originalErr,
	}

	unwrapped := err.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Unwrap() returned %v, want %v", unwrapped, originalErr)
	}
}

func TestErrGoalNotFound(t *testing.T) {
	goalID := "test-goal-123"
	err := ErrGoalNotFound(goalID)

	if err.Code != ErrCodeGoalNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeGoalNotFound)
	}

	if !strings.Contains(err.Message, goalID) {
		t.Errorf("Message should contain goal ID %v, got %v", goalID, err.Message)
	}
}

func TestErrChallengeNotFound(t *testing.T) {
	challengeID := "test-challenge-456"
	err := ErrChallengeNotFound(challengeID)

	if err.Code != ErrCodeChallengeNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeChallengeNotFound)
	}

	if !strings.Contains(err.Message, challengeID) {
		t.Errorf("Message should contain challenge ID %v, got %v", challengeID, err.Message)
	}
}

func TestErrGoalAlreadyClaimed(t *testing.T) {
	goalID := "claimed-goal"
	err := ErrGoalAlreadyClaimed(goalID)

	if err.Code != ErrCodeGoalAlreadyClaimed {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeGoalAlreadyClaimed)
	}

	if !strings.Contains(err.Message, goalID) {
		t.Errorf("Message should contain goal ID %v, got %v", goalID, err.Message)
	}
}

func TestErrGoalNotCompleted(t *testing.T) {
	goalID := "incomplete-goal"
	err := ErrGoalNotCompleted(goalID)

	if err.Code != ErrCodeGoalNotCompleted {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeGoalNotCompleted)
	}

	if !strings.Contains(err.Message, goalID) {
		t.Errorf("Message should contain goal ID %v, got %v", goalID, err.Message)
	}
}

func TestErrDatabaseError(t *testing.T) {
	operation := "batch upsert"
	originalErr := errors.New("connection lost")
	err := ErrDatabaseError(operation, originalErr)

	if err.Code != ErrCodeDatabaseError {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeDatabaseError)
	}

	if !strings.Contains(err.Message, operation) {
		t.Errorf("Message should contain operation %v, got %v", operation, err.Message)
	}

	if err.Err != originalErr {
		t.Errorf("Wrapped error = %v, want %v", err.Err, originalErr)
	}
}

func TestErrConfigInvalid(t *testing.T) {
	reason := "duplicate goal IDs"
	err := ErrConfigInvalid(reason)

	if err.Code != ErrCodeConfigInvalid {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeConfigInvalid)
	}

	if !strings.Contains(err.Message, reason) {
		t.Errorf("Message should contain reason %v, got %v", reason, err.Message)
	}
}

func TestErrRewardGrantFailed(t *testing.T) {
	rewardType := "ITEM"
	rewardID := "winter_sword"
	originalErr := errors.New("AGS service unavailable")
	err := ErrRewardGrantFailed(rewardType, rewardID, originalErr)

	if err.Code != ErrCodeRewardGrantFailed {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeRewardGrantFailed)
	}

	if !strings.Contains(err.Message, rewardType) {
		t.Errorf("Message should contain reward type %v, got %v", rewardType, err.Message)
	}

	if !strings.Contains(err.Message, rewardID) {
		t.Errorf("Message should contain reward ID %v, got %v", rewardID, err.Message)
	}

	if err.Err != originalErr {
		t.Errorf("Wrapped error = %v, want %v", err.Err, originalErr)
	}
}

func TestErrValidationFailed(t *testing.T) {
	field := "target_value"
	reason := "must be positive"
	err := ErrValidationFailed(field, reason)

	if err.Code != ErrCodeValidationFailed {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeValidationFailed)
	}

	if !strings.Contains(err.Message, field) {
		t.Errorf("Message should contain field %v, got %v", field, err.Message)
	}

	if !strings.Contains(err.Message, reason) {
		t.Errorf("Message should contain reason %v, got %v", reason, err.Message)
	}
}

func TestNewChallengeError(t *testing.T) {
	code := "TEST_CODE"
	message := "test message"
	originalErr := errors.New("wrapped error")

	err := NewChallengeError(code, message, originalErr)

	if err.Code != code {
		t.Errorf("Code = %v, want %v", err.Code, code)
	}

	if err.Message != message {
		t.Errorf("Message = %v, want %v", err.Message, message)
	}

	if err.Err != originalErr {
		t.Errorf("Wrapped error = %v, want %v", err.Err, originalErr)
	}
}

func TestErrorWrapping(t *testing.T) {
	// Test that errors.Is and errors.As work with ChallengeError
	originalErr := errors.New("database connection failed")
	challengeErr := ErrDatabaseError("query", originalErr)

	// Test error wrapping
	if !errors.Is(challengeErr, originalErr) {
		t.Error("errors.Is should recognize wrapped error")
	}

	// Test error unwrapping
	var unwrapped error = challengeErr
	for unwrapped != nil {
		if unwrapped == originalErr {
			break
		}
		unwrapped = errors.Unwrap(unwrapped)
	}

	if unwrapped != originalErr {
		t.Error("Should be able to unwrap to original error")
	}
}
