# Trend Kids API — plano por fatias (monólito modular em Go)

Arquitetura espelhada de `tech-challange-execution-service`: `domain → application (commands/queries) → ports → adapters (http, postgres)`.
Escopo atual: **produto simples, sem variantes** (tamanho/cor ficam para depois). Dinheiro em centavos. TDD em tudo.

Deliberadamente fora (ver seção 6 da spec): Kafka, k8s/Helm, New Relic, tipos `null` próprios, tracing (entra se fizer falta).

| Fatia | Entrega | Estado |
| --- | --- | --- |
| 1 | Domínio `catalog.Product` + factory validando campos | feita |
| 2 | Portas + casos de uso `List`/`Get`/`Save` (repositório em memória nos testes) | feita |
| 3 | Postgres: migração `products`, DAO/repository (SQL à mão, sqlx), teste de integração; `docker compose` | feita |
| 4 | HTTP (Gin): `GET /v1/products`, `GET /v1/products/:slug`, `/health`, CORS; bootstrap em `cmd/` | feita |
| 5 | Front: `lib/api.ts` consome a API; `/loja` e vitrines da landing leem do back | feita |
| 6 | Admin: login + CRUD de produto/estoque | próxima |
