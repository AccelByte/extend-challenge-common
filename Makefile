# Copyright (c) 2025 AccelByte Inc. All Rights Reserved.
# This is licensed software from AccelByte Inc, for limitations
# and restrictions contact your company contract manager.

SHELL := /bin/bash

# Database configuration for tests and benchmarks
TEST_DB_CONTAINER := extend-challenge-common-test-db
TEST_DB_HOST := localhost
TEST_DB_PORT := 5433
TEST_DB_USER := postgres
TEST_DB_PASSWORD := postgres
TEST_DB_NAME := challenge_db
TEST_DB_DSN := postgres://$(TEST_DB_USER):$(TEST_DB_PASSWORD)@$(TEST_DB_HOST):$(TEST_DB_PORT)/$(TEST_DB_NAME)?sslmode=disable

# Migration file location (local to this repo)
MIGRATION_FILE := migrations/001_create_user_goal_progress.up.sql

.PHONY: lint lint-fix test test-coverage test-all
.PHONY: db-setup db-teardown db-status db-clean
.PHONY: bench bench-m4 bench-m3 bench-all bench-compare

# Linting targets
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

lint-fix:
	@echo "Running golangci-lint with auto-fix..."
	@golangci-lint run --fix ./...

# Testing targets
test:
	@echo "Running tests..."
	@go test ./... -v

test-coverage:
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=coverage.out
	@go tool cover -func=coverage.out | grep total

# Run all checks (lint + test with coverage + benchmarks) with automatic DB setup/teardown
test-all:
	@echo "🚀 Starting comprehensive test suite..."
	@echo ""
	@echo "Step 1/5: Setting up test database..."
	@$(MAKE) db-setup
	@echo ""
	@echo "Step 2/5: Running linter..."
	@$(MAKE) lint || (echo "❌ Linting failed!" && $(MAKE) db-teardown && exit 1)
	@echo ""
	@echo "Step 3/5: Running tests with coverage..."
	@$(MAKE) test-coverage || (echo "❌ Tests failed!" && $(MAKE) db-teardown && exit 1)
	@echo ""
	@echo "Step 4/5: Running benchmarks..."
	@$(MAKE) bench || (echo "❌ Benchmarks failed!" && $(MAKE) db-teardown && exit 1)
	@echo ""
	@echo "Step 5/5: Tearing down test database..."
	@$(MAKE) db-teardown
	@echo ""
	@echo "✅ All checks passed!"
	@echo ""
	@echo "Summary:"
	@echo "  ✓ Linting"
	@echo "  ✓ Tests with coverage"
	@echo "  ✓ Benchmarks"
	@echo "  ✓ Database cleanup"

# Database setup for integration tests and benchmarks
db-setup:
	@echo "🐘 Setting up PostgreSQL test database..."
	@if [ ! -f "$(MIGRATION_FILE)" ]; then \
		echo "❌ Migration file not found: $(MIGRATION_FILE)"; \
		echo "   Please ensure the migration file exists in the migrations/ directory"; \
		exit 1; \
	fi
	@docker-compose up -d postgres-test
	@echo "⏳ Waiting for PostgreSQL to be ready..."
	@sleep 5
	@echo "👤 Creating postgres superuser (password: 'postgres')..."
	@docker exec $(TEST_DB_CONTAINER) psql -U testuser -d postgres -c \
		"CREATE USER postgres WITH PASSWORD 'postgres' SUPERUSER;" 2>/dev/null || \
		echo "   (User 'postgres' already exists, skipping)"
	@echo "🗄️  Creating challenge_db database (for benchmarks)..."
	@docker exec $(TEST_DB_CONTAINER) psql -U testuser -d postgres -c \
		"CREATE DATABASE $(TEST_DB_NAME);" 2>/dev/null || \
		echo "   (Database 'challenge_db' already exists, skipping)"
	@echo "📋 Applying database schema to 'postgres' database (for tests)..."
	@cat $(MIGRATION_FILE) | \
		docker exec -i $(TEST_DB_CONTAINER) psql -U postgres -d postgres 2>&1 | grep -v "already exists" || true
	@echo "📋 Applying database schema to 'challenge_db' database (for benchmarks)..."
	@cat $(MIGRATION_FILE) | \
		docker exec -i $(TEST_DB_CONTAINER) psql -U postgres -d $(TEST_DB_NAME) 2>&1 | grep -v "already exists" || true
	@echo "✅ Database setup complete!"
	@echo ""
	@echo "Database connection details:"
	@echo "  For integration tests:"
	@echo "    DSN: postgres://postgres:postgres@$(TEST_DB_HOST):$(TEST_DB_PORT)/postgres?sslmode=disable"
	@echo "  For benchmarks:"
	@echo "    DSN: postgres://postgres:postgres@$(TEST_DB_HOST):$(TEST_DB_PORT)/$(TEST_DB_NAME)?sslmode=disable"

db-teardown:
	@echo "🧹 Tearing down PostgreSQL test database..."
	@docker-compose down -v
	@echo "✅ Database teardown complete!"

db-status:
	@echo "📊 PostgreSQL test database status:"
	@docker ps --filter "name=$(TEST_DB_CONTAINER)" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" || \
		echo "❌ Test database container not running"

db-clean:
	@echo "🧹 Cleaning test database (truncating tables)..."
	@docker exec $(TEST_DB_CONTAINER) psql -U $(TEST_DB_USER) -d $(TEST_DB_NAME) -c \
		"TRUNCATE TABLE user_goal_progress;" 2>/dev/null && \
		echo "✅ Tables truncated successfully!" || \
		echo "❌ Failed to truncate tables (database may not be running)"

# Benchmark targets
bench: db-status
	@echo "📊 Running all benchmarks (this may take 3-5 minutes)..."
	@go test -bench=. -benchmem -benchtime=50x -run=^$$ ./pkg/repository/
	@echo "✅ Benchmarks complete!"

bench-m4: db-status
	@echo "📊 Running M4 benchmarks (BatchUpsertGoalActive)..."
	@echo ""
	@echo "🔹 1/4: Batch vs Loop comparison..."
	@go test -bench="BenchmarkBatchUpsertGoalActive_vs_Loop" -benchmem -benchtime=100x -run=^$$ ./pkg/repository/
	@echo ""
	@echo "🔹 2/4: New records (INSERT path)..."
	@go test -bench="BenchmarkBatchUpsertGoalActive_NewRecords" -benchmem -benchtime=100x -run=^$$ ./pkg/repository/
	@echo ""
	@echo "🔹 3/4: Existing records (UPDATE path)..."
	@go test -bench="BenchmarkBatchUpsertGoalActive_ExistingRecords" -benchmem -benchtime=100x -run=^$$ ./pkg/repository/
	@echo ""
	@echo "🔹 4/4: Real-world scenarios..."
	@go test -bench="BenchmarkM4_" -benchmem -benchtime=100x -run=^$$ ./pkg/repository/
	@echo ""
	@echo "✅ M4 benchmarks complete!"
	@echo ""
	@echo "📖 For detailed analysis, see: pkg/repository/M4_BENCHMARK_RESULTS.md"

bench-m3: db-status
	@echo "📊 Running M3 benchmarks (assignment control & COPY protocol)..."
	@go test -bench="BenchmarkBatchUpsertProgressWithCOPY_AssignmentControl" -benchmem -benchtime=50x -run=^$$ ./pkg/repository/
	@go test -bench="BenchmarkBatchIncrementProgress_AssignmentControl" -benchmem -benchtime=50x -run=^$$ ./pkg/repository/
	@go test -bench="BenchmarkBatchUpsertProgressWithCOPY_Baseline" -benchmem -benchtime=50x -run=^$$ ./pkg/repository/
	@echo "✅ M3 benchmarks complete!"

bench-all: db-status
	@echo "📊 Running comprehensive benchmark suite..."
	@echo "⏱️  This will take 5-10 minutes..."
	@echo ""
	@go test -bench=. -benchmem -benchtime=100x -run=^$$ ./pkg/repository/ | tee benchmark_results_$$(date +%Y%m%d_%H%M%S).txt
	@echo ""
	@echo "✅ All benchmarks complete!"
	@echo "📁 Results saved to: benchmark_results_*.txt"

bench-compare: db-status
	@echo "📊 Running benchmarks for regression detection..."
	@echo "💾 Saving baseline results to: benchmark_baseline.txt"
	@go test -bench=. -benchmem -benchtime=100x -run=^$$ ./pkg/repository/ > benchmark_baseline.txt
	@echo ""
	@echo "✅ Baseline saved!"
	@echo ""
	@echo "To compare after making changes:"
	@echo "  1. Make your code changes"
	@echo "  2. Run: go test -bench=. -benchmem -benchtime=100x -run=^$$ ./pkg/repository/ > benchmark_new.txt"
	@echo "  3. Run: benchstat benchmark_baseline.txt benchmark_new.txt"
	@echo ""
	@echo "Install benchstat if needed: go install golang.org/x/perf/cmd/benchstat@latest"

# Quick benchmark (fast iteration during development)
bench-quick: db-status
	@echo "📊 Running quick benchmarks (10 iterations)..."
	@go test -bench="BenchmarkBatchUpsertGoalActive_vs_Loop" -benchmem -benchtime=10x -run=^$$ ./pkg/repository/
	@echo "✅ Quick benchmark complete!"

# Help target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Linting:"
	@echo "  make lint           - Run golangci-lint"
	@echo "  make lint-fix       - Run golangci-lint with auto-fix"
	@echo ""
	@echo "Testing:"
	@echo "  make test           - Run all tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make test-all       - Run lint + tests + benchmarks (auto setup/teardown DB)"
	@echo ""
	@echo "Database Management:"
	@echo "  make db-setup       - Start PostgreSQL test database (required for tests/benchmarks)"
	@echo "  make db-teardown    - Stop and remove test database"
	@echo "  make db-status      - Check test database status"
	@echo "  make db-clean       - Truncate all tables in test database"
	@echo ""
	@echo "Benchmarking:"
	@echo "  make bench          - Run all benchmarks (3-5 minutes)"
	@echo "  make bench-m4       - Run M4 benchmarks only (BatchUpsertGoalActive)"
	@echo "  make bench-m3       - Run M3 benchmarks only (assignment control)"
	@echo "  make bench-all      - Run comprehensive benchmark suite (5-10 minutes)"
	@echo "  make bench-compare  - Save baseline for regression detection"
	@echo "  make bench-quick    - Quick benchmark (10 iterations, for development)"
	@echo ""
	@echo "Workflow Examples:"
	@echo "  make test-all                     - Run complete test suite (auto DB setup/teardown)"
	@echo "  make db-setup && make bench-m4    - Setup DB and run M4 benchmarks"
	@echo "  make db-clean && make bench-quick - Clean DB and run quick benchmark"
	@echo "  make db-teardown                  - Clean up after testing"
