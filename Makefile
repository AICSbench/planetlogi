.PHONY: dev-up dev-down api-run api-worker api-test api-vet api-build api-migrate-up api-migrate-down api-sqlc api-seed api-openapi web-dev web-build check

DB_URL ?= postgres://planetlogi:planetlogi_dev@localhost:5432/planetlogi?sslmode=disable

dev-up:
	docker compose -f docker/docker-compose.yml up -d

dev-down:
	docker compose -f docker/docker-compose.yml down

api-run:
	cd apps/api && go run ./cmd/server

api-worker:
	cd apps/api && go run ./cmd/worker

api-test:
	cd apps/api && go test -race ./...

api-vet:
	cd apps/api && go vet ./...

api-build:
	cd apps/api && go build -o bin/server ./cmd/server && go build -o bin/worker ./cmd/worker && go build -o bin/seed ./cmd/seed

api-migrate-up:
	cd apps/api && goose -dir db/migrations postgres "$(DB_URL)" up

api-migrate-down:
	cd apps/api && goose -dir db/migrations postgres "$(DB_URL)" down

api-sqlc:
	cd apps/api && sqlc generate

api-seed:
	cd apps/api && DATABASE_URL="$(DB_URL)" go run ./cmd/seed

api-openapi:
	cd apps/api && go run ./cmd/openapi > ../../packages/api-client/openapi.json && cd ../.. && pnpm api:gen

web-dev:
	pnpm --filter @planetlogi/web dev

web-build:
	pnpm build

check:
	pnpm i18n:check
	pnpm typecheck
	pnpm lint
	pnpm test
	pnpm build
	cd apps/api && go vet ./... && go test -race ./...
