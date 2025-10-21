package client

import (
	"context"
	"errors"
	"strings"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// Error types for AGS Platform Service errors
// These indicate non-retryable errors that should fail immediately.

// AGSError represents an error response from AGS Platform Service.
// It includes the HTTP status code for proper error classification.
type AGSError struct {
	StatusCode int
	Message    string
}

func (e *AGSError) Error() string {
	return e.Message
}

// HTTPStatusCode returns the HTTP status code from the AGS response.
func (e *AGSError) HTTPStatusCode() int {
	return e.StatusCode
}

// Convenience constructors for common error types

// BadRequestError indicates invalid request parameters (400).
// Examples: invalid item ID, invalid currency code, invalid quantity
type BadRequestError struct {
	Message string
}

func (e *BadRequestError) Error() string {
	return "bad request: " + e.Message
}

func (e *BadRequestError) HTTPStatusCode() int {
	return 400
}

// NotFoundError indicates resource not found (404).
// Examples: item doesn't exist, currency not configured
type NotFoundError struct {
	Resource string
}

func (e *NotFoundError) Error() string {
	return "resource not found: " + e.Resource
}

func (e *NotFoundError) HTTPStatusCode() int {
	return 404
}

// ForbiddenError indicates insufficient permissions (403).
// Examples: namespace mismatch, service account lacks permissions
type ForbiddenError struct {
	Message string
}

func (e *ForbiddenError) Error() string {
	return "forbidden: " + e.Message
}

func (e *ForbiddenError) HTTPStatusCode() int {
	return 403
}

// AuthenticationError indicates authentication failure (401).
// Examples: invalid/expired token, invalid client credentials
type AuthenticationError struct {
	Message string
}

func (e *AuthenticationError) Error() string {
	return "authentication failed: " + e.Message
}

func (e *AuthenticationError) HTTPStatusCode() int {
	return 401
}

// HTTPStatusCodeError is an interface for errors that include HTTP status codes.
type HTTPStatusCodeError interface {
	error
	HTTPStatusCode() int
}

// IsRetryableHTTPStatus determines if an HTTP status code should be retried.
//
// Non-retryable status codes (4xx client errors):
//   - 400 Bad Request - invalid parameters
//   - 401 Unauthorized - authentication failed
//   - 403 Forbidden - insufficient permissions
//   - 404 Not Found - resource doesn't exist
//   - 409 Conflict - resource conflict
//   - 422 Unprocessable Entity - validation failed
//
// Retryable status codes:
//   - 408 Request Timeout
//   - 429 Too Many Requests
//   - 500 Internal Server Error
//   - 502 Bad Gateway
//   - 503 Service Unavailable
//   - 504 Gateway Timeout
func IsRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case 400, 401, 403, 404, 409, 422:
		// Client errors - non-retryable
		return false
	case 408, 429, 500, 502, 503, 504:
		// Timeouts and server errors - retryable
		return true
	default:
		// For unknown codes, treat 4xx as non-retryable, 5xx as retryable
		if statusCode >= 400 && statusCode < 500 {
			return false
		}
		return true
	}
}

// IsRetryableError determines if an error from RewardClient should be retried.
//
// Classification strategy:
// 1. If error implements HTTPStatusCodeError, check status code (most reliable)
// 2. If error is a known typed error, use its status code
// 3. Fallback to error message pattern matching (for generic errors)
//
// Non-retryable errors (fail immediately):
//   - HTTP 400 Bad Request - invalid item/currency configuration
//   - HTTP 401 Unauthorized - invalid credentials
//   - HTTP 403 Forbidden - insufficient permissions
//   - HTTP 404 Not Found - item/currency doesn't exist
//   - HTTP 409 Conflict - resource conflict
//   - HTTP 422 Unprocessable Entity - validation failed
//
// Retryable errors (retry with exponential backoff):
//   - HTTP 408 Request Timeout
//   - HTTP 429 Too Many Requests
//   - HTTP 500 Internal Server Error
//   - HTTP 502 Bad Gateway
//   - HTTP 503 Service Unavailable
//   - HTTP 504 Gateway Timeout
//   - Network timeouts, connection refused, DNS failures
//
// Usage in retry logic:
//
//	err := rewardClient.GrantReward(...)
//	if err != nil && !IsRetryableError(err) {
//	    return err  // Fail immediately
//	}
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Strategy 1: Check for HTTP status code (most reliable)
	var httpErr HTTPStatusCodeError
	if errors.As(err, &httpErr) {
		return IsRetryableHTTPStatus(httpErr.HTTPStatusCode())
	}

	// Strategy 2: Check for known typed errors
	var badRequest *BadRequestError
	if errors.As(err, &badRequest) {
		return IsRetryableHTTPStatus(badRequest.HTTPStatusCode())
	}

	var notFound *NotFoundError
	if errors.As(err, &notFound) {
		return IsRetryableHTTPStatus(notFound.HTTPStatusCode())
	}

	var forbidden *ForbiddenError
	if errors.As(err, &forbidden) {
		return IsRetryableHTTPStatus(forbidden.HTTPStatusCode())
	}

	var authErr *AuthenticationError
	if errors.As(err, &authErr) {
		return IsRetryableHTTPStatus(authErr.HTTPStatusCode())
	}

	var agsErr *AGSError
	if errors.As(err, &agsErr) {
		return IsRetryableHTTPStatus(agsErr.HTTPStatusCode())
	}

	// Strategy 3: Fallback to pattern matching for generic errors
	// This handles cases where the AGS SDK returns generic errors
	errMsg := strings.ToLower(err.Error())

	// Non-retryable patterns (4xx-like errors)
	nonRetryablePatterns := []string{
		"bad request",
		"invalid argument",
		"not found",
		"forbidden",
		"unauthorized",
		"authentication failed",
		"permission denied",
		"invalid item",
		"invalid currency",
		"item not found",
		"currency not found",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return false
		}
	}

	// All other errors are considered retryable
	// (network timeouts, 502/503, connection refused, etc.)
	return true
}

// RewardClient integrates with AccelByte Gaming Services (AGS) Platform Service
// to grant rewards to users when they claim completed goals.
//
// This client abstracts the AGS SDK calls and provides retry logic for reliability.
type RewardClient interface {
	// GrantItemReward grants an item entitlement to a user.
	// This is used for rewards of type "ITEM".
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - namespace: AGS namespace
	//   - userID: User's unique identifier
	//   - itemID: Item code from AGS inventory
	//   - quantity: Number of items to grant
	//
	// Returns error if grant fails after retries.
	GrantItemReward(ctx context.Context, namespace, userID, itemID string, quantity int) error

	// GrantWalletReward credits a user's wallet with virtual currency.
	// This is used for rewards of type "WALLET".
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - namespace: AGS namespace
	//   - userID: User's unique identifier
	//   - currencyCode: Currency code from AGS wallet (e.g., "GOLD", "GEMS")
	//   - amount: Amount of currency to credit
	//
	// Returns error if grant fails after retries.
	GrantWalletReward(ctx context.Context, namespace, userID, currencyCode string, amount int) error

	// GrantReward is a convenience method that dispatches to the appropriate grant method
	// based on the reward type (ITEM or WALLET).
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - namespace: AGS namespace
	//   - userID: User's unique identifier
	//   - reward: Reward configuration from goal
	//
	// Returns error if reward type is unsupported or grant fails after retries.
	GrantReward(ctx context.Context, namespace, userID string, reward domain.Reward) error
}
