// Copyright (c) 2025 AccelByte Inc. All Rights Reserved.
// This is licensed software from AccelByte Inc, for limitations
// and restrictions contact your company contract manager.

package common

import "time"

// GetCurrentDateUTC returns the current date in UTC, truncated to midnight (00:00:00).
// This matches PostgreSQL's DATE() function behavior for consistency.
//
// Example:
//   - Input: 2025-10-17 14:23:45 UTC
//   - Output: 2025-10-17 00:00:00 UTC
//
// Usage: Use this for daily increment client-side date checking to ensure
// consistency with SQL DATE() function in BatchIncrementProgress queries.
func GetCurrentDateUTC() time.Time {
	return time.Now().UTC().Truncate(24 * time.Hour)
}

// TruncateToDateUTC truncates the given time to midnight (00:00:00) in UTC.
// This matches PostgreSQL's DATE() function behavior for consistency.
//
// Example:
//   - Input: 2025-10-17 14:23:45 UTC
//   - Output: 2025-10-17 00:00:00 UTC
//
// Usage: Use this for comparing timestamps in bufferIncrementDaily with today's date.
func TruncateToDateUTC(t time.Time) time.Time {
	return t.UTC().Truncate(24 * time.Hour)
}
