.PHONY: dev-up dev-down build-ingestion build-consumer build-scoring build-enrichment build-api build-web tidy test smoke-test

dev-up:
	docker compose -f deploy/docker-compose.yml up -d --build

dev-down:
	docker compose -f deploy/docker-compose.yml down

build-ingestion:
	cd services/ingestion && go build -o ../../bin/ingestion ./cmd/server

build-consumer:
	cd services/consumer && go build -o ../../bin/consumer ./cmd/server

build-scoring:
	cd services/scoring && go build -o ../../bin/scoring ./cmd/server

build-enrichment:
	cd services/enrichment && go build -o ../../bin/enrichment ./cmd/server

build-api:
	cd services/api && npm install && npm run build

build-web:
	cd web && npm install && npm run build

tidy:
	cd services/ingestion && go mod tidy
	cd services/consumer && go mod tidy
	cd services/scoring && go mod tidy
	cd services/enrichment && go mod tidy

test:
	cd services/ingestion && go test ./...
	cd services/consumer && go test ./...
	cd services/scoring && go test ./...
	cd services/enrichment && go test ./...
	cd services/api && npm test

smoke-test:
	powershell -ExecutionPolicy Bypass -File scripts/smoke-test.ps1
