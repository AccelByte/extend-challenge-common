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

	// Save original values
	originalValues := make(map[string]string)
	for _, key := range envVars {
		originalValues[key] = os.Getenv(key)
		testUnsetenv(t, key)
	}

	// Restore original values after test
	defer func() {
		for key, value := range originalValues {
			if value != "" {
				testSetenv(t, key, value)
			}
		}
	}()

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
	// Save original values
	originalValues := map[string]string{
		"DB_HOST":               os.Getenv("DB_HOST"),
		"DB_PORT":               os.Getenv("DB_PORT"),
		"DB_NAME":               os.Getenv("DB_NAME"),
		"DB_USER":               os.Getenv("DB_USER"),
		"DB_PASSWORD":           os.Getenv("DB_PASSWORD"),
		"DB_SSLMODE":            os.Getenv("DB_SSLMODE"),
		"DB_MAX_OPEN_CONNS":     os.Getenv("DB_MAX_OPEN_CONNS"),
		"DB_MAX_IDLE_CONNS":     os.Getenv("DB_MAX_IDLE_CONNS"),
		"DB_CONN_MAX_LIFETIME":  os.Getenv("DB_CONN_MAX_LIFETIME"),
		"DB_CONN_MAX_IDLE_TIME": os.Getenv("DB_CONN_MAX_IDLE_TIME"),
	}

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
		// Restore original values
		for key, value := range originalValues {
			if value != "" {
				testSetenv(t, key, value)
			} else {
				testUnsetenv(t, key)
			}
		}
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
	originalValue := os.Getenv("DB_PORT")
	testSetenv(t, "DB_PORT", "invalid")
	defer func() {
		if originalValue != "" {
			testSetenv(t, "DB_PORT", originalValue)
		} else {
			testUnsetenv(t, "DB_PORT")
		}
	}()

	cfg := NewConfigFromEnv()

	// Should fallback to default
	assert.Equal(t, 5432, cfg.Port)
}

func TestNewConfigFromEnv_InvalidInt(t *testing.T) {
	originalValue := os.Getenv("DB_MAX_OPEN_CONNS")
	testSetenv(t, "DB_MAX_OPEN_CONNS", "not_a_number")
	defer func() {
		if originalValue != "" {
			testSetenv(t, "DB_MAX_OPEN_CONNS", originalValue)
		} else {
			testUnsetenv(t, "DB_MAX_OPEN_CONNS")
		}
	}()

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

func TestConnect_SQLOpenError(t *testing.T) {
	// Test with invalid driver/connection string format
	cfg := &Config{
		Host:            "",
		Port:            -1, // Invalid port
		Database:        "",
		User:            "",
		Password:        "",
		SSLMode:         "invalid",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 300 * time.Second,
		ConnMaxIdleTime: 300 * time.Second,
	}

	db, err := Connect(cfg)

	// Should fail to ping with invalid configuration
	assert.Error(t, err)
	assert.Nil(t, db)
}

func TestHealth_WithContext(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	cfg := NewConfigFromEnv()
	db, err := Connect(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Test with proper context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	assert.NoError(t, err)
}

func TestHealth_ContextTimeout(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	cfg := NewConfigFromEnv()
	db, err := Connect(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Test with already-expired context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure context is expired

	err = db.PingContext(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestConnect_ConnectionPoolConfiguration(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	// Test various pool configurations
	tests := []struct {
		name         string
		maxOpenConns int
		maxIdleConns int
	}{
		{
			name:         "standard pool",
			maxOpenConns: 25,
			maxIdleConns: 5,
		},
		{
			name:         "small pool",
			maxOpenConns: 5,
			maxIdleConns: 2,
		},
		{
			name:         "large pool",
			maxOpenConns: 100,
			maxIdleConns: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Host:            getEnv("DB_HOST", "localhost"),
				Port:            getEnvAsInt("DB_PORT", 5432),
				Database:        getEnv("DB_NAME", "postgres"),
				User:            getEnv("DB_USER", "postgres"),
				Password:        getEnv("DB_PASSWORD", "test"),
				SSLMode:         "disable",
				MaxOpenConns:    tt.maxOpenConns,
				MaxIdleConns:    tt.maxIdleConns,
				ConnMaxLifetime: 300 * time.Second,
				ConnMaxIdleTime: 300 * time.Second,
			}

			db, err := Connect(cfg)
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			stats := db.Stats()
			assert.Equal(t, tt.maxOpenConns, stats.MaxOpenConnections)

			// Verify we can execute a query
			var result int
			err = db.QueryRow("SELECT 1").Scan(&result)
			assert.NoError(t, err)
			assert.Equal(t, 1, result)
		})
	}
}

func TestConnect_SSLModes(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	// Test different SSL modes
	sslModes := []string{"disable", "allow", "prefer"}

	for _, sslMode := range sslModes {
		t.Run("sslmode_"+sslMode, func(t *testing.T) {
			cfg := &Config{
				Host:            getEnv("DB_HOST", "localhost"),
				Port:            getEnvAsInt("DB_PORT", 5432),
				Database:        getEnv("DB_NAME", "postgres"),
				User:            getEnv("DB_USER", "postgres"),
				Password:        getEnv("DB_PASSWORD", "test"),
				SSLMode:         sslMode,
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 300 * time.Second,
				ConnMaxIdleTime: 300 * time.Second,
			}

			db, err := Connect(cfg)
			// Some SSL modes may not work depending on PostgreSQL configuration
			// We just verify the connection attempt doesn't panic
			if err != nil {
				assert.Contains(t, err.Error(), "failed to ping database")
			} else {
				defer func() { _ = db.Close() }()
				// Verify we can ping
				err = db.Ping()
				assert.NoError(t, err)
			}
		})
	}
}

func TestHealth_Concurrency(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	cfg := NewConfigFromEnv()
	db, err := Connect(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Test concurrent health checks
	const numGoroutines = 10
	errs := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			errs <- Health(db)
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-errs
		assert.NoError(t, err)
	}
}

func TestConnect_VerifyPoolSettings(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("Skipping integration test: DB_HOST not set")
	}

	cfg := &Config{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnvAsInt("DB_PORT", 5432),
		Database:        getEnv("DB_NAME", "postgres"),
		User:            getEnv("DB_USER", "postgres"),
		Password:        getEnv("DB_PASSWORD", "test"),
		SSLMode:         "disable",
		MaxOpenConns:    15,
		MaxIdleConns:    3,
		ConnMaxLifetime: 120 * time.Second,
		ConnMaxIdleTime: 60 * time.Second,
	}

	db, err := Connect(cfg)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Force some connections to be created
	for i := 0; i < 5; i++ {
		var result int
		err = db.QueryRow("SELECT 1").Scan(&result)
		assert.NoError(t, err)
	}

	// Verify pool statistics
	stats := db.Stats()
	assert.Equal(t, 15, stats.MaxOpenConnections)
	assert.GreaterOrEqual(t, stats.OpenConnections, 1)
}
