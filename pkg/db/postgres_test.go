package db

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers to avoid linter errors for unchecked os.Setenv/Unsetenv
func testSetenv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Failed to set env var %s: %v", key, err)
	}
}

func testUnsetenv(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Failed to unset env var %s: %v", key, err)
	}
}

func TestNewConfigFromEnv_AllDefaults(t *testing.T) {
	// Clear all environment variables
	envVars := []string{
		"DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD",
		"DB_SSLMODE", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS",
		"DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME",
	}
	for _, key := range envVars {
		testUnsetenv(t, key)
	}

	cfg := NewConfigFromEnv()

	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 5432, cfg.Port)
	assert.Equal(t, "challenge_service", cfg.Database)
	assert.Equal(t, "postgres", cfg.User)
	assert.Equal(t, "", cfg.Password)
	assert.Equal(t, "disable", cfg.SSLMode)
	assert.Equal(t, 25, cfg.MaxOpenConns)
	assert.Equal(t, 5, cfg.MaxIdleConns)
	assert.Equal(t, 300*time.Second, cfg.ConnMaxLifetime)
	assert.Equal(t, 300*time.Second, cfg.ConnMaxIdleTime)
}

func TestNewConfigFromEnv_CustomValues(t *testing.T) {
	// Set custom environment variables
	testSetenv(t, "DB_HOST", "db.example.com")
	testSetenv(t, "DB_PORT", "5433")
	testSetenv(t, "DB_NAME", "test_db")
	testSetenv(t, "DB_USER", "testuser")
	testSetenv(t, "DB_PASSWORD", "testpass")
	testSetenv(t, "DB_SSLMODE", "require")
	testSetenv(t, "DB_MAX_OPEN_CONNS", "50")
	testSetenv(t, "DB_MAX_IDLE_CONNS", "10")
	testSetenv(t, "DB_CONN_MAX_LIFETIME", "600")
	testSetenv(t, "DB_CONN_MAX_IDLE_TIME", "120")

	defer func() {
		// Clean up
		testUnsetenv(t, "DB_HOST")
		testUnsetenv(t, "DB_PORT")
		testUnsetenv(t, "DB_NAME")
		testUnsetenv(t, "DB_USER")
		testUnsetenv(t, "DB_PASSWORD")
		testUnsetenv(t, "DB_SSLMODE")
		testUnsetenv(t, "DB_MAX_OPEN_CONNS")
		testUnsetenv(t, "DB_MAX_IDLE_CONNS")
		testUnsetenv(t, "DB_CONN_MAX_LIFETIME")
		testUnsetenv(t, "DB_CONN_MAX_IDLE_TIME")
	}()

	cfg := NewConfigFromEnv()

	assert.Equal(t, "db.example.com", cfg.Host)
	assert.Equal(t, 5433, cfg.Port)
	assert.Equal(t, "test_db", cfg.Database)
	assert.Equal(t, "testuser", cfg.User)
	assert.Equal(t, "testpass", cfg.Password)
	assert.Equal(t, "require", cfg.SSLMode)
	assert.Equal(t, 50, cfg.MaxOpenConns)
	assert.Equal(t, 10, cfg.MaxIdleConns)
	assert.Equal(t, 600*time.Second, cfg.ConnMaxLifetime)
	assert.Equal(t, 120*time.Second, cfg.ConnMaxIdleTime)
}

func TestNewConfigFromEnv_InvalidPort(t *testing.T) {
	testSetenv(t, "DB_PORT", "invalid")
	defer testUnsetenv(t, "DB_PORT")

	cfg := NewConfigFromEnv()

	// Should fallback to default
	assert.Equal(t, 5432, cfg.Port)
}

func TestNewConfigFromEnv_InvalidInt(t *testing.T) {
	testSetenv(t, "DB_MAX_OPEN_CONNS", "not_a_number")
	defer testUnsetenv(t, "DB_MAX_OPEN_CONNS")

	cfg := NewConfigFromEnv()

	// Should fallback to default
	assert.Equal(t, 25, cfg.MaxOpenConns)
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "environment variable set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "environment variable not set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "empty string environment variable",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				testSetenv(t, tt.key, tt.envValue)
				defer testUnsetenv(t, tt.key)
			} else {
				testUnsetenv(t, tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvAsInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		expected     int
	}{
		{
			name:         "valid integer",
			key:          "TEST_INT",
			defaultValue: 100,
			envValue:     "200",
			expected:     200,
		},
		{
			name:         "invalid integer",
			key:          "TEST_INT",
			defaultValue: 100,
			envValue:     "not_a_number",
			expected:     100,
		},
		{
			name:         "empty string",
			key:          "TEST_INT",
			defaultValue: 100,
			envValue:     "",
			expected:     100,
		},
		{
			name:         "zero value",
			key:          "TEST_INT",
			defaultValue: 100,
			envValue:     "0",
			expected:     0,
		},
		{
			name:         "negative value",
			key:          "TEST_INT",
			defaultValue: 100,
			envValue:     "-50",
			expected:     -50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				testSetenv(t, tt.key, tt.envValue)
				defer testUnsetenv(t, tt.key)
			} else {
				testUnsetenv(t, tt.key)
			}

			result := getEnvAsInt(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConnect_InvalidDSN(t *testing.T) {
	cfg := &Config{
		Host:            "nonexistent.example.com",
		Port:            5432,
		Database:        "test",
		User:            "test",
		Password:        "test",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 300 * time.Second,
		ConnMaxIdleTime: 300 * time.Second,
	}

	db, err := Connect(cfg)

	// Should fail to connect to nonexistent host
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "failed to ping database")
}

func TestHealth_NilDB(t *testing.T) {
	var db *sql.DB

	err := Health(db)

	assert.Error(t, err)
}

func TestHealth_ClosedDB(t *testing.T) {
	// Skip if no database available
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	cfg := NewConfigFromEnv()
	db, err := Connect(cfg)
	require.NoError(t, err)

	// Close the connection
	_ = db.Close()

	// Health check should fail on closed connection
	err = Health(db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database unhealthy")
}

// Integration test - only runs if database is available
func TestConnect_Success(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	cfg := NewConfigFromEnv()
	db, err := Connect(cfg)

	require.NoError(t, err)
	require.NotNil(t, db)
	defer func() { _ = db.Close() }()

	// Verify connection pool settings
	stats := db.Stats()
	assert.LessOrEqual(t, stats.MaxOpenConnections, cfg.MaxOpenConns)

	// Verify we can ping the database
	err = db.Ping()
	assert.NoError(t, err)
}

// Integration test - only runs if database is available
func TestHealth_Success(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	cfg := NewConfigFromEnv()
	db, err := Connect(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	err = Health(db)
	assert.NoError(t, err)
}

// Integration test - health check timeout
func TestHealth_Timeout(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	cfg := NewConfigFromEnv()
	db, err := Connect(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Use PingContext with canceled context to simulate timeout
	err = db.PingContext(ctx)
	assert.Error(t, err)
}

func TestConfig_ConnectionPoolSettings(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	cfg := &Config{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnvAsInt("DB_PORT", 5432),
		Database:        getEnv("DB_NAME", "challenge_service"),
		User:            getEnv("DB_USER", "postgres"),
		Password:        getEnv("DB_PASSWORD", ""),
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: 60 * time.Second,
		ConnMaxIdleTime: 30 * time.Second,
	}

	db, err := Connect(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Verify pool settings are applied
	stats := db.Stats()
	assert.Equal(t, 10, stats.MaxOpenConnections)
}
