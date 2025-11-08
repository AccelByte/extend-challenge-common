# Repository Tests

This directory contains integration tests for the PostgreSQL repository implementation.

## Running Integration Tests

### Prerequisites

Integration tests require a PostgreSQL database accessible at `localhost:5432` with the following credentials:

```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=test
DB_NAME=postgres
```

**Important:** Tests will fail if environment variables point to Docker-only hostnames (e.g., `DB_HOST=postgres`) since tests run on the host machine, not inside Docker.

### Option 1: Using Existing test-postgres Container (Recommended)

If you already have `test-postgres` running from project setup:

```bash
# Verify container is running
docker ps | grep test-postgres
# Should show: test-postgres   Up ...   0.0.0.0:5432->5432/tcp

# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=test
export DB_NAME=postgres

# Run all common module tests
go test ./... -v -coverprofile=coverage.out

# Check coverage
go tool cover -func=coverage.out | grep total
```

### Option 2: Start Fresh test-postgres Container

If you don't have test-postgres running:

```bash
docker run -d --name test-postgres \
  -p 5432:5432 \
  -e POSTGRES_PASSWORD=test \
  postgres:15

# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=test
export DB_NAME=postgres

# Run tests
go test -v ./pkg/repository/...

# Clean up (optional)
docker stop test-postgres && docker rm test-postgres
```

### Option 3: Using Docker Compose

If you have a docker-compose.yml in the project root with a PostgreSQL service:

```bash
docker-compose up -d postgres

# IMPORTANT: Use localhost, not the Docker service name
export DB_HOST=localhost
export DB_PORT=5432  # or 5433 if your compose file maps to a different port
export DB_USER=postgres
export DB_PASSWORD=test  # or your compose file's password
export DB_NAME=postgres

# Run tests
go test -v ./pkg/repository/...

# Clean up
docker-compose down
```

### Running All Common Module Tests

To run the complete test suite for `extend-challenge-common`:

```bash
# From extend-challenge-common directory
DB_HOST=localhost \
DB_PORT=5432 \
DB_USER=postgres \
DB_PASSWORD=test \
DB_NAME=postgres \
go test ./... -coverprofile=coverage.out -v

# View coverage summary
go tool cover -func=coverage.out | grep total

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html
```

### Troubleshooting

#### Error: "lookup postgres on 127.0.0.53:53: server misbehaving"

**Cause:** Environment variable `DB_HOST=postgres` (Docker hostname)
**Solution:** Use `DB_HOST=localhost` instead

```bash
export DB_HOST=localhost  # NOT 'postgres'
```

#### Error: "password authentication failed for user postgres"

**Cause:** Wrong password for the test database
**Solution:** Use the correct password from your container

```bash
# Check test-postgres password
docker inspect test-postgres | grep POSTGRES_PASSWORD

# Common passwords:
export DB_PASSWORD=test        # test-postgres
export DB_PASSWORD=postgres    # challenge-postgres
```

#### Error: "connection refused"

**Cause:** PostgreSQL container not running or wrong port
**Solution:** Verify container is running and check port mapping

```bash
# Check running containers
docker ps | grep postgres

# Verify port mapping (should show 0.0.0.0:5432->5432/tcp)
docker port test-postgres
```

### Test Behavior

- Tests will be **skipped** if `DB_HOST` environment variable is not set
- Tests use connection from environment variables (see Prerequisites above)
- Tests automatically clean up data after each run
- Tests create temporary tables for isolation

## Test Coverage

The test suite covers:

- **UpsertProgress**: Single row insert and update
- **BatchUpsertProgress**: Batch operations (1-1000 rows)
- **GetProgress**: Single progress retrieval
- **GetUserProgress**: All progress for a user
- **GetChallengeProgress**: All progress for a challenge
- **MarkAsClaimed**: Claim flow with validations
- **Transactions**: Commit, rollback, and row-level locking
- **Claimed Protection**: Ensures claimed goals cannot be overwritten

## Performance Benchmarks

To run performance benchmarks:

```bash
# Benchmark batch upsert with different sizes
go test -bench=BenchmarkBatchUpsert -benchmem ./pkg/repository/...
```

Expected performance (on typical hardware):
- Single UPSERT: < 5ms
- Batch UPSERT (100 rows): < 10ms
- Batch UPSERT (1000 rows): < 20ms
