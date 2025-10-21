# extend-challenge-common

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Shared Go library for the AccelByte Challenge Service - provides domain models, interfaces, and common utilities for implementing challenge systems in games.

## Overview

This library contains the core business logic and interfaces for a challenge/achievement system designed for AccelByte Extend applications. It enables game developers to implement challenge systems (daily missions, seasonal events, quests, achievements) with minimal configuration.

**Key Features:**
- Domain models for challenges, goals, and user progress
- Repository interfaces for data persistence
- In-memory cache for challenge configurations
- Reward client interfaces for AGS Platform Service integration
- Configuration loading and validation
- PostgreSQL database utilities

## Installation

```bash
go get github.com/AccelByte/extend-challenge-common
```

## Package Structure

```
pkg/
├── cache/          # GoalCache interface and in-memory implementation
├── client/         # RewardClient interface for AGS Platform Service
├── common/         # Common utilities (date helpers, etc.)
├── config/         # Configuration loading and validation
├── db/             # PostgreSQL connection and utilities
├── domain/         # Domain models (Challenge, Goal, UserGoalProgress, Reward)
├── errors/         # Error types and codes
└── repository/     # GoalRepository interface and PostgreSQL implementation
```

## Usage

### Domain Models

```go
import "github.com/AccelByte/extend-challenge-common/pkg/domain"

challenge := &domain.Challenge{
    ID:          "daily-login-2024",
    Name:        "Daily Login Challenge",
    Description: "Login every day for rewards!",
    Goals: []domain.Goal{
        {
            ID:          "login-7-days",
            Name:        "Login 7 Days",
            Description: "Login for 7 consecutive days",
            TargetValue: 7,
            Rewards: []domain.Reward{
                {
                    Type:  domain.RewardTypeItem,
                    Code:  "GOLD_COIN",
                    Value: 100,
                },
            },
        },
    },
}
```

### Repository Interface

```go
import (
    "github.com/AccelByte/extend-challenge-common/pkg/repository"
    "github.com/AccelByte/extend-challenge-common/pkg/domain"
)

type GoalRepository interface {
    GetUserGoalProgress(ctx context.Context, userID, goalID string) (*domain.UserGoalProgress, error)
    UpsertUserGoalProgress(ctx context.Context, progress *domain.UserGoalProgress) error
    BatchUpsertUserGoalProgress(ctx context.Context, progressList []*domain.UserGoalProgress) error
    BatchIncrementProgress(ctx context.Context, increments []*domain.ProgressIncrement) error
    GetUserChallengeProgress(ctx context.Context, userID, challengeID string) ([]*domain.UserGoalProgress, error)
    GetAllUserProgress(ctx context.Context, userID string) ([]*domain.UserGoalProgress, error)
}
```

### Configuration Loading

```go
import "github.com/AccelByte/extend-challenge-common/pkg/config"

// Load from JSON file
challenges, err := config.LoadConfig("config/challenges.json")
if err != nil {
    log.Fatal(err)
}

// Validate configuration
if err := config.ValidateConfig(challenges); err != nil {
    log.Fatal(err)
}

// Create in-memory cache
cache := cache.NewInMemoryGoalCache(challenges)
goals := cache.GetGoalsByChallengeID("daily-login-2024")
```

### Database Connection

```go
import "github.com/AccelByte/extend-challenge-common/pkg/db"

// Load config from environment variables
cfg := db.NewPostgresConfigFromEnv()

// Connect to database
db, err := db.ConnectPostgres(cfg)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Create repository
repo := repository.NewPostgresGoalRepository(db)
```

## Environment Variables

```bash
# PostgreSQL
DB_HOST=localhost
DB_PORT=5432
DB_NAME=challenge_db
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSLMODE=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME_MINUTES=30
```

## Testing

The library includes comprehensive unit tests with 80%+ code coverage:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run integration tests (requires PostgreSQL)
docker run -d --name test-postgres -p 5432:5432 \
  -e POSTGRES_PASSWORD=test postgres:15
go test ./pkg/repository/... -v
```

## Database Schema

The library expects the following PostgreSQL table:

```sql
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

    PRIMARY KEY (user_id, goal_id)
);

CREATE INDEX idx_user_goal_progress_user_challenge
    ON user_goal_progress (user_id, challenge_id);
```

## Performance Characteristics

- **Cache Lookups:** O(1) in-memory hash map
- **Batch UPSERT:** ~20ms for 1,000 rows (single query)
- **Batch Increment:** ~15ms for 1,000 updates (optimized query)
- **Repository Methods:** < 50ms (p95) with connection pooling

## Used By

This library is designed for use with:
- **extend-challenge-service** - REST API backend service
- **extend-challenge-event-handler** - gRPC event handler service

Both services are AccelByte Extend applications for implementing challenge systems.

## Development

```bash
# Install dependencies
go mod download

# Run tests
make test

# Run linter
make lint

# Check coverage
make test-coverage
```

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass and coverage remains >80%
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Related Projects

- [extend-challenge-service](https://github.com/AccelByte/extend-challenge-service) - REST API service
- [extend-challenge-event-handler](https://github.com/AccelByte/extend-challenge-event-handler) - Event handler service
- [AccelByte Extend](https://docs.accelbyte.io/extend/) - Platform documentation

## Support

For issues and questions:
- Open an issue on GitHub
- Check the [documentation](https://docs.accelbyte.io/extend/)
- Review the [examples](./examples/) directory
