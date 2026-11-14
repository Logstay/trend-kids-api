DB_URL ?= postgres://trendkids:trendkids@localhost:5433/trendkids?sslmode=disable

up:
	@docker compose -f config/local/docker-compose.yml up -d --wait

down:
	@docker compose -f config/local/docker-compose.yml down

migrate:
	@for f in config/migrations/V*.sql; do psql "$(DB_URL)" -v ON_ERROR_STOP=1 -q -f $$f; done

unit_test:
	@go test ./...

it_test: up
	@TEST_DATABASE_URL="$(DB_URL)" go test -tags integration ./internal/adapters/...
