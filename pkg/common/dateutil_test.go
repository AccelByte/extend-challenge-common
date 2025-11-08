// Copyright (c) 2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package common

import (
	"testing"
	"time"
)

func TestGetCurrentDateUTC(t *testing.T) {
	result := GetCurrentDateUTC()

	// Verify it's in UTC timezone
	if result.Location() != time.UTC {
		t.Errorf("Expected UTC timezone, got %v", result.Location())
	}

	// Verify time is truncated to midnight (00:00:00)
	if result.Hour() != 0 || result.Minute() != 0 || result.Second() != 0 || result.Nanosecond() != 0 {
		t.Errorf("Expected truncated time (00:00:00.000000000), got %02d:%02d:%02d.%09d",
			result.Hour(), result.Minute(), result.Second(), result.Nanosecond())
	}

	// Verify it returns today's date
	now := time.Now().UTC()
	if result.Year() != now.Year() || result.Month() != now.Month() || result.Day() != now.Day() {
		t.Errorf("Expected today's date %04d-%02d-%02d, got %04d-%02d-%02d",
			now.Year(), now.Month(), now.Day(),
			result.Year(), result.Month(), result.Day())
	}
}

func TestTruncateToDateUTC(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "truncate afternoon time",
			input:    time.Date(2025, 10, 17, 14, 23, 45, 123456789, time.UTC),
			expected: time.Date(2025, 10, 17, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "truncate midnight (already truncated)",
			input:    time.Date(2025, 10, 17, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 10, 17, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "truncate just before midnight",
			input:    time.Date(2025, 10, 17, 23, 59, 59, 999999999, time.UTC),
			expected: time.Date(2025, 10, 17, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "truncate early morning",
			input:    time.Date(2025, 10, 17, 0, 0, 1, 0, time.UTC),
			expected: time.Date(2025, 10, 17, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "truncate with non-UTC timezone (should convert to UTC)",
			input:    time.Date(2025, 10, 17, 14, 23, 45, 0, time.FixedZone("PST", -8*60*60)),
			expected: time.Date(2025, 10, 17, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateToDateUTC(tt.input)

			if !result.Equal(tt.expected) {
				t.Errorf("TruncateToDateUTC(%v) = %v, want %v", tt.input, result, tt.expected)
			}

			// Verify result is in UTC
			if result.Location() != time.UTC {
				t.Errorf("Expected UTC timezone, got %v", result.Location())
			}

			// Verify time is truncated to midnight
			if result.Hour() != 0 || result.Minute() != 0 || result.Second() != 0 || result.Nanosecond() != 0 {
				t.Errorf("Expected truncated time (00:00:00.000000000), got %02d:%02d:%02d.%09d",
					result.Hour(), result.Minute(), result.Second(), result.Nanosecond())
			}
		})
	}
}

func TestTruncateToDateUTC_ConsistencyWithPostgreSQL(t *testing.T) {
	// This test verifies that our truncation logic matches PostgreSQL's DATE() function behavior
	// PostgreSQL DATE() truncates to midnight in the session's timezone (we use UTC)
	testCases := []struct {
		name     string
		input    time.Time
		expected string // Expected date string (YYYY-MM-DD)
	}{
		{
			name:     "PostgreSQL DATE('2025-10-17 14:23:45 UTC')",
			input:    time.Date(2025, 10, 17, 14, 23, 45, 0, time.UTC),
			expected: "2025-10-17",
		},
		{
			name:     "PostgreSQL DATE('2025-10-17 00:00:00 UTC')",
			input:    time.Date(2025, 10, 17, 0, 0, 0, 0, time.UTC),
			expected: "2025-10-17",
		},
		{
			name:     "PostgreSQL DATE('2025-10-17 23:59:59 UTC')",
			input:    time.Date(2025, 10, 17, 23, 59, 59, 0, time.UTC),
			expected: "2025-10-17",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TruncateToDateUTC(tc.input)
			resultStr := result.Format("2006-01-02")

			if resultStr != tc.expected {
				t.Errorf("TruncateToDateUTC(%v).Format('2006-01-02') = %s, want %s",
					tc.input, resultStr, tc.expected)
			}
		})
	}
}

func TestGetCurrentDateUTC_IdempotentOnSameDay(t *testing.T) {
	// Calling GetCurrentDateUTC multiple times on the same day should return the same date
	first := GetCurrentDateUTC()
	time.Sleep(10 * time.Millisecond) // Small delay
	second := GetCurrentDateUTC()

	if !first.Equal(second) {
		t.Errorf("GetCurrentDateUTC() called twice on same day returned different dates: %v vs %v",
			first, second)
	}
}

func TestTruncateToDateUTC_Idempotent(t *testing.T) {
	// Truncating an already-truncated date should return the same date
	input := time.Date(2025, 10, 17, 14, 23, 45, 0, time.UTC)
	first := TruncateToDateUTC(input)
	second := TruncateToDateUTC(first)

	if !first.Equal(second) {
		t.Errorf("TruncateToDateUTC is not idempotent: first=%v, second=%v", first, second)
	}
}
