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

### Overview

The repository includes comprehensive benchmarks for all batch operations across M1-M4 milestones. Benchmarks require a running PostgreSQL instance (see integration test setup above).

### Benchmark Files

| File | Focus | Key Metrics |
|------|-------|-------------|
| `postgres_goal_repository_bench_test.go` | M3 assignment control & COPY protocol | Batch operations (100-10,000 rows) |
| `m3_performance_bench_test.go` | M3 event processing performance | High-throughput batch operations |
| `batch_increment_crossover_bench_test.go` | Batch increment optimizations | Crossover point analysis |
| `m4_batch_active_bench_test.go` | **M4 batch goal activation** | Small-medium batches (3-50 goals) |

### Running M4 Benchmarks

**Prerequisites:**
- PostgreSQL test database running on `localhost:5433` (see docker-compose.test.yml)
- User `postgres` with password `postgres` (for benchmarks)
- Database `challenge_db` with schema applied

**Quick Start:**
```bash
# Start test database
cd /path/to/extend-challenge-suite
docker-compose -f docker-compose.test.yml up -d postgres-test

# Create postgres user for benchmarks (if not exists)
docker exec extend-challenge-test-db psql -U testuser -d postgres -c \
  "CREATE USER postgres WITH PASSWORD 'postgres' SUPERUSER;" 2>/dev/null || true

# Create challenge_db and apply schema
docker exec extend-challenge-test-db psql -U testuser -d postgres -c \
  "CREATE DATABASE challenge_db;" 2>/dev/null || true
cat migrations/001_create_user_goal_progress.up.sql | \
  docker exec -i extend-challenge-test-db psql -U testuser -d challenge_db

# Run M4 benchmarks
cd extend-challenge-common
go test -bench="^BenchmarkBatchUpsertGoalActive|^BenchmarkM4_" \
  -benchmem -benchtime=100x -run=^$ ./pkg/repository/
```

**Individual Benchmark Suites:**

```bash
# M4: BatchUpsertGoalActive vs Loop comparison (critical for M4 performance)
go test -bench="BenchmarkBatchUpsertGoalActive_vs_Loop" -benchmem -benchtime=100x -run=^$ ./pkg/repository/

# M4: New record insertion (INSERT path)
go test -bench="BenchmarkBatchUpsertGoalActive_NewRecords" -benchmem -benchtime=100x -run=^$ ./pkg/repository/

# M4: Existing record update (UPDATE path)
go test -bench="BenchmarkBatchUpsertGoalActive_ExistingRecords" -benchmem -benchtime=100x -run=^$ ./pkg/repository/

# M4: Real-world scenarios (random selection, batch activation)
go test -bench="BenchmarkM4_" -benchmem -benchtime=100x -run=^$ ./pkg/repository/

# M3: Assignment control (is_active filtering)
go test -bench="BenchmarkBatchUpsertProgressWithCOPY_AssignmentControl" -benchmem -benchtime=50x -run=^$ ./pkg/repository/

# M3: High-throughput baseline (100-10,000 rows)
go test -bench="BenchmarkBatchUpsertProgressWithCOPY_Baseline" -benchmem -benchtime=50x -run=^$ ./pkg/repository/

# All benchmarks (comprehensive, takes 3-5 minutes)
go test -bench=. -benchmem -benchtime=50x -run=^$ ./pkg/repository/
```

### Benchmark Results

**M4 Performance Summary (2025-11-18):**

See [M4_BENCHMARK_RESULTS.md](./M4_BENCHMARK_RESULTS.md) for comprehensive analysis.

**Key Findings:**
- ✅ **BatchUpsertGoalActive: 1.1-1.5ms** for 5-10 goals (vs 10ms target = **7-9x better**)
- ✅ **Batch vs Loop: 2.7x-4.4x faster** (solves N+1 query problem)
- ✅ **Random selection: 1.8ms** total (deactivate + activate)
- ✅ **Transaction overhead: +0.26ms** (minimal impact)

**Expected Performance by Milestone:**

| Operation | Batch Size | Latency (p95) | Use Case |
|-----------|------------|---------------|----------|
| **M4: BatchUpsertGoalActive** | 3-50 | 1.0-3.7ms | Random/batch goal activation |
| **M3: BatchUpsertProgressWithCOPY** | 100-10,000 | 2.7-21ms | Event processing bulk updates |
| **M3: BatchIncrementProgress** | 100-1,000 | ~15ms | Stat increment events |
| **M1: UpsertProgress** | 1 | 0.6-1.0ms | Single goal updates |

### Benchmark Methodology

**Test Environment:**
- CPU: AMD Ryzen 7 5825U (16 cores)
- Database: PostgreSQL 15-alpine (Docker testcontainer)
- Connection: Local (localhost:5433), minimal network latency
- Iterations: 50-100x per benchmark for statistical significance

**Benchmark Types:**

1. **Latency Benchmarks** (ms/op):
   - Measure end-to-end operation time
   - Include database round-trip, query execution, result parsing
   - Reported as milliseconds per operation

2. **Throughput Benchmarks** (rows/sec or goals/sec):
   - Calculate items processed per second
   - Useful for capacity planning
   - Derived from latency: `throughput = batch_size / (latency / 1000)`

3. **Memory Benchmarks** (B/op, allocs/op):
   - Track memory allocations per operation
   - Identify memory-intensive code paths
   - Compare INSERT vs UPDATE efficiency

**Reading Benchmark Output:**

```
BenchmarkBatchUpsertGoalActive_NewRecords/Size5-16  100  1.040 ms/op  4809 goals/sec  6411 B/op  85 allocs/op
                                               │     │    │            │               │          │
                                               │     │    │            │               │          └─ Allocations per operation
                                               │     │    │            │               └─ Bytes allocated per operation
                                               │     │    │            └─ Throughput (goals/second)
                                               │     │    └─ Latency (milliseconds per operation)
                                               │     └─ Iterations run
                                               └─ CPU cores available (-16 = 16 cores)
```

### Interpreting Results

**Performance Targets:**

- ✅ **Good:** Operation completes within target latency (e.g., < 10ms for M4 batch ops)
- ✅ **Excellent:** Operation exceeds target by 2x+ (e.g., 1-2ms for M4 = 5-10x better)
- ⚠️ **Review:** Operation approaches target limit (e.g., 8-9ms for 10ms target)
- ❌ **Optimize:** Operation exceeds target (requires investigation)

**Comparing Implementations:**

When comparing two approaches (e.g., Batch vs Loop), look for:
1. **Speedup Factor:** How many times faster is the optimized approach?
2. **Memory Efficiency:** Does the faster approach use more memory?
3. **Scalability:** Does the speedup increase with batch size?

Example from M4 results:
```
Batch_Size5:  1.12ms (batch) vs 3.05ms (loop) = 2.7x faster ✅
Batch_Size10: 1.44ms (batch) vs 4.74ms (loop) = 3.3x faster ✅
Batch_Size20: 2.41ms (batch) vs 10.53ms (loop) = 4.4x faster ✅ (scales better!)
```

### Troubleshooting Benchmark Issues

**Benchmark Skipped:**
```bash
# Cause: Database not available or short mode enabled
# Solution: Ensure test DB running and use -short=false (default)
go test -bench=BenchmarkName -run=^$ ./pkg/repository/
```

**Inconsistent Results:**
```bash
# Cause: Insufficient iterations for statistical significance
# Solution: Increase benchtime
go test -bench=BenchmarkName -benchtime=100x -run=^$ ./pkg/repository/
```

**Out of Memory:**
```bash
# Cause: Large batch size benchmark exhausting memory
# Solution: Run specific benchmarks, not all at once
go test -bench="BenchmarkName/Size10-" -benchmem -run=^$ ./pkg/repository/
```

### Performance Regression Detection

To detect regressions between code changes:

```bash
# Baseline (before changes)
go test -bench=. -benchmem -benchtime=100x -run=^$ ./pkg/repository/ > old.txt

# After changes
go test -bench=. -benchmem -benchtime=100x -run=^$ ./pkg/repository/ > new.txt

# Compare (requires benchstat tool)
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

**Example Output:**
```
name                                    old time/op  new time/op  delta
BatchUpsertGoalActive_NewRecords/Size5  1.04ms ± 2%  1.12ms ± 3%  +7.69%
```
- **Positive delta:** Slower (regression)
- **Negative delta:** Faster (improvement)
- **± percentage:** Statistical variance (lower is better)
