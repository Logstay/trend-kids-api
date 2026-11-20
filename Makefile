DB_URL ?= postgres://trendkids:trendkids@localhost:5433/trendkids?sslmode=disable
TEST_DB_URL ?= postgres://trendkids:trendkids@localhost:5433/trendkids_test?sslmode=disable

up:
	@docker compose -f config/local/docker-compose.yml up -d --wait

down:
	@docker compose -f config/local/docker-compose.yml down

# Migrações com Flyway (mesmo padrão do tech-challenge-os-service). Os SQLs ficam em config/migrations/sql/V<n>__nome.sql.
# Sem -ti: funciona também fora de terminal interativo.
migrate:
	@docker run --rm \
	--net=host \
	-v $(PWD)/config/migrations:/flyway/conf \
	-v $(PWD)/config/migrations/sql:/flyway/sql \
	flyway/flyway:11 \
	-configFiles=/flyway/conf/flyway.local.conf \
	migrate

migrate_test:
	@docker run --rm \
	--net=host \
	-v $(PWD)/config/migrations:/flyway/conf \
	-v $(PWD)/config/migrations/sql:/flyway/sql \
	flyway/flyway:11 \
	-configFiles=/flyway/conf/flyway.test.conf \
	migrate

unit_test:
	@go test ./...

it_test: up
	@docker exec trendkids-postgres psql -U trendkids -d trendkids -tc "SELECT 1 FROM pg_database WHERE datname='trendkids_test'" | grep -q 1 || docker exec trendkids-postgres psql -U trendkids -d trendkids -q -c "CREATE DATABASE trendkids_test"
	@TEST_DATABASE_URL="$(TEST_DB_URL)" go test -p 1 -tags integration ./internal/adapters/...

run:
	@set -a; . ./.env; set +a; go run ./cmd

seed:
	@psql "$(DB_URL)" -v ON_ERROR_STOP=1 -q -f config/seed/dev.sql

# Cria o dono do painel. Uso: make admin EMAIL=voce@email.com
# A senha NÃO vai na linha de comando (apareceria no histórico do shell): é pedida no terminal, sem eco.
# Mínimo de 12 caracteres. Para scripts, dá para passar ADMIN_PASSWORD no ambiente.
.PHONY: admin
admin:
	@test -n "$(EMAIL)" || { echo "Informe o e-mail: make admin EMAIL=voce@email.com"; exit 1; }
	@test -f .env || { echo "Falta o arquivo .env (copie o .env.example)"; exit 1; }
	@set -a; . ./.env; set +a; go run ./cmd/createadmin -email "$(EMAIL)"
