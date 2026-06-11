.PHONY: dev-up dev-down build-ingestion build-consumer build-api build-web tidy

dev-up:
	docker compose -f deploy/docker-compose.yml up -d

dev-down:
	docker compose -f deploy/docker-compose.yml down

build-ingestion:
	cd services/ingestion && go build -o ../../bin/ingestion ./cmd/server

build-consumer:
	cd services/consumer && go build -o ../../bin/consumer ./cmd/server

build-api:
	cd services/api && npm run build

build-web:
	cd web && npm run build

tidy:
	cd services/ingestion && go mod tidy
	cd services/consumer && go mod tidy
