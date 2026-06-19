.PHONY: dev-up dev-down build-ingestion build-consumer build-scoring build-enrichment build-providerconfig build-compliance build-ask-ai build-attribution build-api build-web tidy test smoke-test demo-seed verify-presentation step1b-test

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

build-providerconfig:
	cd services/providerconfig && go build -o ../../bin/providerconfig ./cmd/server

build-compliance:
	cd services/compliance-intelligence && go build -o ../../bin/compliance-intelligence ./cmd/server

build-ask-ai:
	cd services/ask-ai && go build -o ../../bin/ask-ai ./cmd/server

build-attribution:
	cd services/ad-attribution && go build -o ../../bin/ad-attribution ./cmd/server

build-api:
	cd services/api && npm install && npm run build

build-web:
	cd web && npm install && npm run build

tidy:
	cd services/ingestion && go mod tidy
	cd services/consumer && go mod tidy
	cd services/scoring && go mod tidy
	cd services/enrichment && go mod tidy
	cd services/providerconfig && go mod tidy
	cd services/compliance-intelligence && go mod tidy
	cd services/ask-ai && go mod tidy
	cd services/ad-attribution && go mod tidy

test:
	cd services/ingestion && go test ./...
	cd services/consumer && go test ./...
	cd services/scoring && go test ./...
	cd services/enrichment && go test ./...
	cd services/providerconfig && go test ./...
	cd services/compliance-intelligence && go test ./...
	cd services/ask-ai && go test ./...
	cd services/ad-attribution && go test ./...
	cd services/api && npm test

step1b-test:
	cd services/providerconfig && go test ./internal/loader/... ./internal/catalog/... ./internal/config/... -v

smoke-test:
	powershell -ExecutionPolicy Bypass -File scripts/smoke-test.ps1

demo-seed:
	powershell -ExecutionPolicy Bypass -File scripts/demo-seed.ps1

verify-presentation:
	powershell -ExecutionPolicy Bypass -File scripts/verify-presentation.ps1
