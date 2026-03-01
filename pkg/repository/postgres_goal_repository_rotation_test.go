package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// setupRotationTestDB creates a test database with the full M5 schema including baseline_value.
func setupRotationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", testDSN)
	if err != nil {
		t.Skipf("Skipping integration test: cannot connect to database: %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping integration test: database not available: %v", err)
		return nil
	}

	// Drop and recreate to ensure full M5 schema
	_, _ = db.Exec(`DROP TABLE IF EXISTS user_goal_progress`)

	_, err = db.Exec(`
		CREATE TABLE user_goal_progress (
			user_id VARCHAR(100) NOT NULL,
			goal_id VARCHAR(100) NOT NULL,
			challenge_id VARCHAR(100) NOT NULL,
			namespace VARCHAR(100) NOT NULL,
			progress INT NOT NULL DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'not_started',
			completed_at TIMESTAMP NULL,
			claimed_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			is_active BOOLEAN NOT NULL DEFAULT true,
			assigned_at TIMESTAMP NULL,
			expires_at TIMESTAMP NULL,
			baseline_value INT NULL,
			PRIMARY KEY (user_id, goal_id),
			CONSTRAINT check_status CHECK (status IN ('not_started', 'in_progress', 'completed', 'claimed')),
			CONSTRAINT check_progress_non_negative CHECK (progress >= 0)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_user_goal_progress_user_challenge
		ON user_goal_progress(user_id, challenge_id)
	`)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	return db
}

// rotationVerifyRow holds the fields we need to verify after a rotation UPDATE.
type rotationVerifyRow struct {
	Progress      int
	Status        string
	BaselineValue sql.NullInt64
	UpdatedAt     time.Time
	CompletedAt   sql.NullTime
	ClaimedAt     sql.NullTime
	ExpiresAt     sql.NullTime
}

func insertRotationSeed(t *testing.T, ctx context.Context, db *sql.DB,
	userID, goalID string, progress int, status string, baseline *int, updatedAt time.Time, completedAt *time.Time) {
	t.Helper()

	_, _ = db.ExecContext(ctx,
		`DELETE FROM user_goal_progress WHERE user_id = $1 AND goal_id = $2`,
		userID, goalID)

	now := time.Now().UTC()
	var baselineSQL any
	if baseline != nil {
		baselineSQL = *baseline
	}

	var claimedAt any
	if status == "claimed" {
		claimedAt = now.Add(-7 * 24 * time.Hour)
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO user_goal_progress
			(user_id, goal_id, challenge_id, namespace, progress, status,
			 completed_at, claimed_at, created_at, updated_at,
			 is_active, assigned_at, baseline_value)
		VALUES ($1, $2, 'test-challenge', 'test-ns', $3, $4, $5, $6, $7, $8, true, $7, $9)
	`, userID, goalID, progress, status, completedAt, claimedAt, now, updatedAt, baselineSQL)
	if err != nil {
		t.Fatalf("insert seed row %s/%s: %v", userID, goalID, err)
	}
}

func readRotationRow(t *testing.T, ctx context.Context, db *sql.DB, userID, goalID string) rotationVerifyRow {
	t.Helper()
	var row rotationVerifyRow
	err := db.QueryRowContext(ctx, `
		SELECT progress, status, baseline_value, updated_at, completed_at, claimed_at, expires_at
		FROM user_goal_progress
		WHERE user_id = $1 AND goal_id = $2
	`, userID, goalID).Scan(
		&row.Progress, &row.Status, &row.BaselineValue,
		&row.UpdatedAt, &row.CompletedAt, &row.ClaimedAt, &row.ExpiresAt)
	if err != nil {
		t.Fatalf("read row %s/%s: %v", userID, goalID, err)
	}
	return row
}

func todayMidnight() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func intP(v int) *int {
	return &v
}

// TestRotation_RotatedRowGetsNewBaseline verifies baseline reset when row is stale.
func TestRotation_RotatedRowGetsNewBaseline(t *testing.T) {
	db := setupRotationTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	repo := NewPostgresGoalRepository(db)

	userID := "rot-user-001"
	goalID := "rot-goal-001"
	staleTime := todayMidnight().Add(-2 * time.Hour)
	insertRotationSeed(t, ctx, db, userID, goalID, 105, "in_progress", intP(100), staleTime, nil)

	boundary := todayMidnight()
	expires := todayMidnight().Add(24 * time.Hour)
	progress := 108
	err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{{
		UserID: userID, GoalID: goalID, ChallengeID: "test-challenge", Namespace: "test-ns",
		Progress: &progress, ProgressMode: "relative", IncValue: 3, TargetValue: 10,
		RotationBoundary: &boundary, NewExpiresAt: &expires, ResetProgress: true,
	}})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	row := readRotationRow(t, ctx, db, userID, goalID)
	if row.Progress != 108 {
		t.Errorf("progress = %d, want 108", row.Progress)
	}
	if !row.BaselineValue.Valid || row.BaselineValue.Int64 != 105 {
		t.Errorf("baseline_value = %v, want 105 (108-3)", row.BaselineValue)
	}
	if row.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress (3 < 10)", row.Status)
	}
}

// TestRotation_CompletedGoalRotated verifies completed+stale goals reset on rotation.
func TestRotation_CompletedGoalRotated(t *testing.T) {
	db := setupRotationTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	repo := NewPostgresGoalRepository(db)

	userID := "rot-user-002"
	goalID := "rot-goal-002"
	completedAt := todayMidnight().Add(-25 * time.Hour)
	staleTime := todayMidnight().Add(-2 * time.Hour)
	insertRotationSeed(t, ctx, db, userID, goalID, 110, "completed", intP(100), staleTime, &completedAt)

	boundary := todayMidnight()
	expires := todayMidnight().Add(24 * time.Hour)
	progress := 115
	err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{{
		UserID: userID, GoalID: goalID, ChallengeID: "test-challenge", Namespace: "test-ns",
		Progress: &progress, ProgressMode: "relative", IncValue: 5, TargetValue: 10,
		RotationBoundary: &boundary, NewExpiresAt: &expires, ResetProgress: true,
	}})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	row := readRotationRow(t, ctx, db, userID, goalID)
	if row.Progress != 115 {
		t.Errorf("progress = %d, want 115", row.Progress)
	}
	if !row.BaselineValue.Valid || row.BaselineValue.Int64 != 110 {
		t.Errorf("baseline_value = %v, want 110 (115-5)", row.BaselineValue)
	}
	if row.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress (inc=5 < target=10)", row.Status)
	}
	if row.CompletedAt.Valid {
		t.Errorf("completed_at = %v, want NULL (cleared on rotation)", row.CompletedAt.Time)
	}
}

// TestRotation_ClaimedGoalUntouched verifies claimed goals are skipped when allow_reselection=false.
func TestRotation_ClaimedGoalUntouched(t *testing.T) {
	db := setupRotationTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	repo := NewPostgresGoalRepository(db)

	userID := "rot-user-003"
	goalID := "rot-goal-003"
	staleTime := todayMidnight().Add(-7 * 24 * time.Hour)
	insertRotationSeed(t, ctx, db, userID, goalID, 110, "claimed", intP(100), staleTime, nil)

	boundary := todayMidnight()
	expires := todayMidnight().Add(24 * time.Hour)
	progress := 120
	err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{{
		UserID: userID, GoalID: goalID, ChallengeID: "test-challenge", Namespace: "test-ns",
		Progress: &progress, ProgressMode: "relative", IncValue: 5, TargetValue: 10,
		RotationBoundary: &boundary, NewExpiresAt: &expires,
		ResetProgress: true, AllowReselection: false,
	}})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	row := readRotationRow(t, ctx, db, userID, goalID)
	if row.Status != "claimed" {
		t.Errorf("status = %q, want claimed (untouched)", row.Status)
	}
	if row.Progress != 110 {
		t.Errorf("progress = %d, want 110 (unchanged)", row.Progress)
	}
}

// TestRotation_AbsoluteBaselineStaysNull verifies absolute goals never set baseline.
func TestRotation_AbsoluteBaselineStaysNull(t *testing.T) {
	db := setupRotationTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	repo := NewPostgresGoalRepository(db)

	userID := "rot-user-004"
	goalID := "rot-goal-004"
	insertRotationSeed(t, ctx, db, userID, goalID, 5, "in_progress", nil,
		time.Now().UTC().Add(-1*time.Hour), nil)

	progress := 8
	err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{{
		UserID: userID, GoalID: goalID, ChallengeID: "test-challenge", Namespace: "test-ns",
		Progress: &progress, ProgressMode: "absolute", IncValue: 3, TargetValue: 20,
	}})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	row := readRotationRow(t, ctx, db, userID, goalID)
	if row.BaselineValue.Valid {
		t.Errorf("baseline_value = %d, want NULL for absolute goal", row.BaselineValue.Int64)
	}
	if row.Progress != 8 {
		t.Errorf("progress = %d, want 8", row.Progress)
	}
	if row.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress (8 < 20)", row.Status)
	}
}

// TestRotation_FirstEventInitializesBaseline verifies first relative event sets baseline.
func TestRotation_FirstEventInitializesBaseline(t *testing.T) {
	db := setupRotationTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	repo := NewPostgresGoalRepository(db)

	userID := "rot-user-005"
	goalID := "rot-goal-005"
	insertRotationSeed(t, ctx, db, userID, goalID, 0, "not_started", nil,
		time.Now().UTC().Add(-5*time.Minute), nil)

	progress := 153
	err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{{
		UserID: userID, GoalID: goalID, ChallengeID: "test-challenge", Namespace: "test-ns",
		Progress: &progress, ProgressMode: "relative", IncValue: 3, TargetValue: 10,
	}})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	row := readRotationRow(t, ctx, db, userID, goalID)
	if !row.BaselineValue.Valid || row.BaselineValue.Int64 != 150 {
		t.Errorf("baseline_value = %v, want 150 (153-3)", row.BaselineValue)
	}
	if row.Progress != 153 {
		t.Errorf("progress = %d, want 153", row.Progress)
	}
	if row.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress (3 < 10)", row.Status)
	}
}

// TestRotation_CompletedPreservedResetProgressFalse verifies completed+stale stays completed when reset_progress=false.
func TestRotation_CompletedPreservedResetProgressFalse(t *testing.T) {
	db := setupRotationTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	repo := NewPostgresGoalRepository(db)

	userID := "rot-user-006"
	goalID := "rot-goal-006"
	completedAt := todayMidnight().Add(-25 * time.Hour)
	staleTime := todayMidnight().Add(-2 * time.Hour)
	insertRotationSeed(t, ctx, db, userID, goalID, 110, "completed", intP(100), staleTime, &completedAt)

	boundary := todayMidnight()
	expires := todayMidnight().Add(24 * time.Hour)
	progress := 115
	err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{{
		UserID: userID, GoalID: goalID, ChallengeID: "test-challenge", Namespace: "test-ns",
		Progress: &progress, ProgressMode: "relative", IncValue: 5, TargetValue: 10,
		RotationBoundary: &boundary, NewExpiresAt: &expires,
		ResetProgress: false, AllowReselection: false,
	}})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	row := readRotationRow(t, ctx, db, userID, goalID)
	if row.Progress != 115 {
		t.Errorf("progress = %d, want 115", row.Progress)
	}
	if !row.BaselineValue.Valid || row.BaselineValue.Int64 != 100 {
		t.Errorf("baseline_value = %v, want 100 (preserved)", row.BaselineValue)
	}
	if row.Status != "completed" {
		t.Errorf("status = %q, want completed (preserved with reset_progress=false)", row.Status)
	}
	if !row.CompletedAt.Valid {
		t.Error("completed_at should be preserved, got NULL")
	}
}

// TestRotation_ClaimedGoalResetWithAllowReselection verifies claimed+stale resets when allow_reselection=true.
func TestRotation_ClaimedGoalResetWithAllowReselection(t *testing.T) {
	db := setupRotationTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	repo := NewPostgresGoalRepository(db)

	userID := "rot-user-007"
	goalID := "rot-goal-007"
	staleTime := todayMidnight().Add(-7 * 24 * time.Hour)
	insertRotationSeed(t, ctx, db, userID, goalID, 110, "claimed", intP(100), staleTime, nil)

	boundary := todayMidnight()
	expires := todayMidnight().Add(24 * time.Hour)
	progress := 120
	err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{{
		UserID: userID, GoalID: goalID, ChallengeID: "test-challenge", Namespace: "test-ns",
		Progress: &progress, ProgressMode: "relative", IncValue: 5, TargetValue: 10,
		RotationBoundary: &boundary, NewExpiresAt: &expires,
		ResetProgress: true, AllowReselection: true,
	}})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	row := readRotationRow(t, ctx, db, userID, goalID)
	if row.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress (reset with allow_reselection, inc_value > 0)", row.Status)
	}
	if !row.BaselineValue.Valid || row.BaselineValue.Int64 != 115 {
		t.Errorf("baseline_value = %v, want 115 (120-5)", row.BaselineValue)
	}
	if row.ClaimedAt.Valid {
		t.Errorf("claimed_at = %v, want NULL (cleared on reselection)", row.ClaimedAt.Time)
	}
	if row.CompletedAt.Valid {
		t.Errorf("completed_at = %v, want NULL (cleared on reselection)", row.CompletedAt.Time)
	}
}

// TestRotation_InProgressPreservedResetProgressFalse verifies baseline preserved when reset_progress=false.
func TestRotation_InProgressPreservedResetProgressFalse(t *testing.T) {
	db := setupRotationTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	repo := NewPostgresGoalRepository(db)

	userID := "rot-user-008"
	goalID := "rot-goal-008"
	staleTime := todayMidnight().Add(-2 * time.Hour)
	insertRotationSeed(t, ctx, db, userID, goalID, 105, "in_progress", intP(100), staleTime, nil)

	boundary := todayMidnight()
	expires := todayMidnight().Add(24 * time.Hour)
	progress := 108
	err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{{
		UserID: userID, GoalID: goalID, ChallengeID: "test-challenge", Namespace: "test-ns",
		Progress: &progress, ProgressMode: "relative", IncValue: 3, TargetValue: 10,
		RotationBoundary: &boundary, NewExpiresAt: &expires,
		ResetProgress: false, AllowReselection: false,
	}})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	row := readRotationRow(t, ctx, db, userID, goalID)
	if row.Progress != 108 {
		t.Errorf("progress = %d, want 108", row.Progress)
	}
	if !row.BaselineValue.Valid || row.BaselineValue.Int64 != 100 {
		t.Errorf("baseline_value = %v, want 100 (preserved with reset_progress=false)", row.BaselineValue)
	}
	if row.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress (8 < 10)", row.Status)
	}
}

// TestRotation_LoginEvent_NilProgress_WithRotation verifies nil Progress (login) with rotation.
func TestRotation_LoginEvent_NilProgress_WithRotation(t *testing.T) {
	db := setupRotationTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	repo := NewPostgresGoalRepository(db)

	userID := "rot-user-009"
	goalID := "rot-goal-009"
	staleTime := todayMidnight().Add(-2 * time.Hour)
	insertRotationSeed(t, ctx, db, userID, goalID, 5, "in_progress", intP(3), staleTime, nil)

	boundary := todayMidnight()
	expires := todayMidnight().Add(24 * time.Hour)
	// Login event: nil Progress, IncValue=1
	err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{{
		UserID: userID, GoalID: goalID, ChallengeID: "test-challenge", Namespace: "test-ns",
		Progress: nil, ProgressMode: "relative", IncValue: 1, TargetValue: 5,
		RotationBoundary: &boundary, NewExpiresAt: &expires,
		ResetProgress: true, AllowReselection: false,
	}})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	row := readRotationRow(t, ctx, db, userID, goalID)
	// progress = old(5) + inc(1) = 6
	if row.Progress != 6 {
		t.Errorf("progress = %d, want 6 (5+1)", row.Progress)
	}
	// baseline reset: COALESCE(nil, 5+1) - 1 = 5
	if !row.BaselineValue.Valid || row.BaselineValue.Int64 != 5 {
		t.Errorf("baseline_value = %v, want 5 (6-1)", row.BaselineValue)
	}
	// relative progress = 6 - 5 = 1, target = 5, so in_progress
	if row.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress (1 < 5)", row.Status)
	}
}

// TestRotation_LoginEvent_NilProgress_NoRotation verifies nil Progress (login) without rotation.
func TestRotation_LoginEvent_NilProgress_NoRotation(t *testing.T) {
	db := setupRotationTestDB(t)
	defer cleanupTestDB(t, db)

	ctx := context.Background()
	repo := NewPostgresGoalRepository(db)

	userID := "rot-user-010"
	goalID := "rot-goal-010"
	recentTime := time.Now().UTC().Add(-5 * time.Minute)
	insertRotationSeed(t, ctx, db, userID, goalID, 3, "in_progress", intP(0), recentTime, nil)

	// Login event without rotation: nil Progress, IncValue=1, no boundary
	err := repo.BatchUpsertProgressWithCOPY(ctx, []CopyRow{{
		UserID: userID, GoalID: goalID, ChallengeID: "test-challenge", Namespace: "test-ns",
		Progress: nil, ProgressMode: "relative", IncValue: 1, TargetValue: 5,
	}})
	if err != nil {
		t.Fatalf("BatchUpsertProgressWithCOPY: %v", err)
	}

	row := readRotationRow(t, ctx, db, userID, goalID)
	// progress = old(3) + inc(1) = 4
	if row.Progress != 4 {
		t.Errorf("progress = %d, want 4 (3+1)", row.Progress)
	}
	// baseline preserved (no rotation, already initialized)
	if !row.BaselineValue.Valid || row.BaselineValue.Int64 != 0 {
		t.Errorf("baseline_value = %v, want 0 (preserved)", row.BaselineValue)
	}
	// relative progress = 4 - 0 = 4, target = 5, so in_progress
	if row.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress (4 < 5)", row.Status)
	}
}
