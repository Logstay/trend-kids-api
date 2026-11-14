DB_URL ?= postgres://trendkids:trendkids@localhost:5433/trendkids?sslmode=disable
TEST_DB_URL ?= postgres://trendkids:trendkids@localhost:5433/trendkids_test?sslmode=disable

up:
	@docker compose -f config/local/docker-compose.yml up -d --wait

down:
	@docker compose -f config/local/docker-compose.yml down

migrate:
	@for f in config/migrations/V*.sql; do psql "$(DB_URL)" -v ON_ERROR_STOP=1 -q -f $$f 2>&1 | grep -v NOTICE; done; true

unit_test:
	@go test ./...

it_test: up
	@docker exec trendkids-postgres psql -U trendkids -d trendkids -tc "SELECT 1 FROM pg_database WHERE datname='trendkids_test'" | grep -q 1 || docker exec trendkids-postgres psql -U trendkids -d trendkids -q -c "CREATE DATABASE trendkids_test"
	@TEST_DATABASE_URL="$(TEST_DB_URL)" go test -p 1 -tags integration ./internal/adapters/...

run:
	@set -a; . ./.env; set +a; go run ./cmd

seed:
	@psql "$(DB_URL)" -v ON_ERROR_STOP=1 -q -f config/seed/dev.sql
