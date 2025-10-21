package client

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test IsRetryableHTTPStatus

func TestIsRetryableHTTPStatus_400_BadRequest(t *testing.T) {
	assert.False(t, IsRetryableHTTPStatus(400))
}

func TestIsRetryableHTTPStatus_401_Unauthorized(t *testing.T) {
	assert.False(t, IsRetryableHTTPStatus(401))
}

func TestIsRetryableHTTPStatus_403_Forbidden(t *testing.T) {
	assert.False(t, IsRetryableHTTPStatus(403))
}

func TestIsRetryableHTTPStatus_404_NotFound(t *testing.T) {
	assert.False(t, IsRetryableHTTPStatus(404))
}

func TestIsRetryableHTTPStatus_409_Conflict(t *testing.T) {
	assert.False(t, IsRetryableHTTPStatus(409))
}

func TestIsRetryableHTTPStatus_422_UnprocessableEntity(t *testing.T) {
	assert.False(t, IsRetryableHTTPStatus(422))
}

func TestIsRetryableHTTPStatus_408_RequestTimeout(t *testing.T) {
	assert.True(t, IsRetryableHTTPStatus(408))
}

func TestIsRetryableHTTPStatus_429_TooManyRequests(t *testing.T) {
	assert.True(t, IsRetryableHTTPStatus(429))
}

func TestIsRetryableHTTPStatus_500_InternalServerError(t *testing.T) {
	assert.True(t, IsRetryableHTTPStatus(500))
}

func TestIsRetryableHTTPStatus_502_BadGateway(t *testing.T) {
	assert.True(t, IsRetryableHTTPStatus(502))
}

func TestIsRetryableHTTPStatus_503_ServiceUnavailable(t *testing.T) {
	assert.True(t, IsRetryableHTTPStatus(503))
}

func TestIsRetryableHTTPStatus_504_GatewayTimeout(t *testing.T) {
	assert.True(t, IsRetryableHTTPStatus(504))
}

func TestIsRetryableHTTPStatus_405_Unknown4xx(t *testing.T) {
	// Unknown 4xx codes should be non-retryable
	assert.False(t, IsRetryableHTTPStatus(405))
}

func TestIsRetryableHTTPStatus_501_Unknown5xx(t *testing.T) {
	// Unknown 5xx codes should be retryable
	assert.True(t, IsRetryableHTTPStatus(501))
}

// Test AGSError

func TestAGSError_Error(t *testing.T) {
	err := &AGSError{StatusCode: 400, Message: "invalid request"}
	assert.Equal(t, "invalid request", err.Error())
}

func TestAGSError_HTTPStatusCode(t *testing.T) {
	err := &AGSError{StatusCode: 502, Message: "bad gateway"}
	assert.Equal(t, 502, err.HTTPStatusCode())
}

func TestIsRetryableError_AGSError_NonRetryable(t *testing.T) {
	err := &AGSError{StatusCode: 400, Message: "bad request"}
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_AGSError_Retryable(t *testing.T) {
	err := &AGSError{StatusCode: 502, Message: "bad gateway"}
	assert.True(t, IsRetryableError(err))
}

// Test IsRetryableError with typed errors

func TestIsRetryableError_BadRequestError(t *testing.T) {
	err := &BadRequestError{Message: "invalid item ID"}
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_NotFoundError(t *testing.T) {
	err := &NotFoundError{Resource: "item_sword"}
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_ForbiddenError(t *testing.T) {
	err := &ForbiddenError{Message: "namespace mismatch"}
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_AuthenticationError(t *testing.T) {
	err := &AuthenticationError{Message: "invalid token"}
	assert.False(t, IsRetryableError(err))
}

// Test IsRetryableError with error message patterns

func TestIsRetryableError_BadRequestPattern(t *testing.T) {
	err := errors.New("bad request: invalid currency")
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_InvalidArgumentPattern(t *testing.T) {
	err := errors.New("invalid argument: quantity must be positive")
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_NotFoundPattern(t *testing.T) {
	err := errors.New("item not found in inventory")
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_ForbiddenPattern(t *testing.T) {
	err := errors.New("forbidden: insufficient permissions")
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_UnauthorizedPattern(t *testing.T) {
	err := errors.New("unauthorized access")
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_AuthenticationFailedPattern(t *testing.T) {
	err := errors.New("authentication failed: expired token")
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_PermissionDeniedPattern(t *testing.T) {
	err := errors.New("permission denied")
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_InvalidItemPattern(t *testing.T) {
	err := errors.New("invalid item code")
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_InvalidCurrencyPattern(t *testing.T) {
	err := errors.New("invalid currency: XYZ")
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_CurrencyNotFoundPattern(t *testing.T) {
	err := errors.New("currency not found in wallet")
	assert.False(t, IsRetryableError(err))
}

// Test IsRetryableError with retryable errors

func TestIsRetryableError_NetworkTimeout(t *testing.T) {
	err := errors.New("network timeout")
	assert.True(t, IsRetryableError(err))
}

func TestIsRetryableError_ConnectionRefused(t *testing.T) {
	err := errors.New("connection refused")
	assert.True(t, IsRetryableError(err))
}

func TestIsRetryableError_BadGateway(t *testing.T) {
	err := errors.New("502 bad gateway")
	assert.True(t, IsRetryableError(err))
}

func TestIsRetryableError_ServiceUnavailable(t *testing.T) {
	err := errors.New("503 service unavailable")
	assert.True(t, IsRetryableError(err))
}

func TestIsRetryableError_DNSFailure(t *testing.T) {
	err := errors.New("temporary DNS failure")
	assert.True(t, IsRetryableError(err))
}

func TestIsRetryableError_GenericError(t *testing.T) {
	err := errors.New("unexpected error occurred")
	assert.True(t, IsRetryableError(err))
}

// Test IsRetryableError edge cases

func TestIsRetryableError_NilError(t *testing.T) {
	assert.False(t, IsRetryableError(nil))
}

func TestIsRetryableError_CaseInsensitive(t *testing.T) {
	// Should match "not found" regardless of case
	err := errors.New("Item NOT FOUND")
	assert.False(t, IsRetryableError(err))
}

func TestIsRetryableError_WrappedNonRetryable(t *testing.T) {
	baseErr := &BadRequestError{Message: "invalid"}
	wrappedErr := errors.New("wrapped: " + baseErr.Error())
	// Should detect the pattern "bad request" in wrapped error
	assert.False(t, IsRetryableError(wrappedErr))
}

// Test error type implementations

func TestBadRequestError_Error(t *testing.T) {
	err := &BadRequestError{Message: "test message"}
	assert.Equal(t, "bad request: test message", err.Error())
}

func TestNotFoundError_Error(t *testing.T) {
	err := &NotFoundError{Resource: "item_123"}
	assert.Equal(t, "resource not found: item_123", err.Error())
}

func TestForbiddenError_Error(t *testing.T) {
	err := &ForbiddenError{Message: "test message"}
	assert.Equal(t, "forbidden: test message", err.Error())
}

func TestAuthenticationError_Error(t *testing.T) {
	err := &AuthenticationError{Message: "test message"}
	assert.Equal(t, "authentication failed: test message", err.Error())
}
