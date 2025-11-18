-- Create user_goal_progress table
-- This table tracks user progress for each goal across all challenges
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

    -- M3: User assignment control
    is_active BOOLEAN NOT NULL DEFAULT true,
    assigned_at TIMESTAMP NULL,

    -- M5: System rotation control (added now for forward compatibility)
    expires_at TIMESTAMP NULL,

    PRIMARY KEY (user_id, goal_id),

    CONSTRAINT check_status CHECK (status IN ('not_started', 'in_progress', 'completed', 'claimed')),
    CONSTRAINT check_progress_non_negative CHECK (progress >= 0),
    CONSTRAINT check_claimed_implies_completed CHECK (claimed_at IS NULL OR completed_at IS NOT NULL)
);

-- Create performance index for user + challenge lookups (GET /v1/challenges)
CREATE INDEX idx_user_goal_progress_user_challenge ON user_goal_progress(user_id, challenge_id);

-- M3: Active goal filtering (GET /v1/challenges?active_only=true)
CREATE INDEX idx_user_goal_progress_user_active
ON user_goal_progress(user_id, is_active)
WHERE is_active = true;

-- M3 Phase 9: Fast path optimization for InitializePlayer
-- Used by GetUserGoalCount() to quickly check if user is initialized
CREATE INDEX idx_user_goal_count ON user_goal_progress(user_id);

-- M3 Phase 9: Composite index for fast goal lookups
-- Used by GetGoalsByIDs for faster querying with IN clause
CREATE INDEX idx_user_goal_lookup ON user_goal_progress(user_id, goal_id);

-- M3 Phase 9: Partial index for active-only queries
-- Used by GetActiveGoals() for fast path returning users
CREATE INDEX idx_user_goal_active_only
ON user_goal_progress(user_id)
WHERE is_active = true;

-- Add comments for documentation
COMMENT ON TABLE user_goal_progress IS 'Tracks user progress for challenge goals';
COMMENT ON COLUMN user_goal_progress.user_id IS 'AGS user identifier from JWT';
COMMENT ON COLUMN user_goal_progress.goal_id IS 'Goal identifier from config file';
COMMENT ON COLUMN user_goal_progress.challenge_id IS 'Parent challenge identifier';
COMMENT ON COLUMN user_goal_progress.namespace IS 'For debugging only - each deployment operates in single namespace';
COMMENT ON COLUMN user_goal_progress.progress IS 'Current progress value (e.g., 7 kills out of 10)';
COMMENT ON COLUMN user_goal_progress.status IS 'not_started -> in_progress -> completed -> claimed';
COMMENT ON COLUMN user_goal_progress.completed_at IS 'Timestamp when goal requirement was met';
COMMENT ON COLUMN user_goal_progress.claimed_at IS 'Timestamp when reward was granted';
COMMENT ON COLUMN user_goal_progress.is_active IS 'M3: Whether goal is assigned to user (controls event processing)';
COMMENT ON COLUMN user_goal_progress.assigned_at IS 'M3: When goal was assigned to user';
COMMENT ON COLUMN user_goal_progress.expires_at IS 'M5: When assignment expires (NULL = permanent)';
