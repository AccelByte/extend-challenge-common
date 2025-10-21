# Repository Tests

This directory contains integration tests for the PostgreSQL repository implementation.

## Running Integration Tests

### Option 1: Using Docker

Start a PostgreSQL container for testing:

```bash
docker run -d --name test-postgres \
  -p 5432:5432 \
  -e POSTGRES_PASSWORD=test \
  postgres:15

# Run tests
go test -v ./pkg/repository/...

# Clean up
docker stop test-postgres && docker rm test-postgres
```

### Option 2: Using Docker Compose

If you have a docker-compose.yml in the project root with a PostgreSQL service:

```bash
docker-compose up -d postgres

# Run tests
go test -v ./pkg/repository/...

# Clean up
docker-compose down
```

### Test Behavior

- Tests will be **skipped** if PostgreSQL is not available
- Tests use the DSN: `postgres://postgres:test@localhost:5432/postgres?sslmode=disable`
- Tests automatically clean up data after each run

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
