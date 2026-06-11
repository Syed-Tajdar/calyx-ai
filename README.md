# Call Loom Intelligence Platform

Pay-per-call analytics and intelligence — Kaliper competitor, part of the Call Loom ecosystem.

## Repository layout

```
├── .cursor/rules/          # Cursor AI rules
├── services/
│   ├── ingestion/          # Go — webhook ingestion (Phase 1 first build)
│   ├── api/                # NestJS — REST API
│   └── consumer/           # Go — Kafka → ClickHouse writer
├── web/                    # Next.js dashboard
├── shared/                 # Cross-service schemas and types
├── db/                     # PostgreSQL and ClickHouse migrations
├── deploy/                 # docker-compose, Terraform, Kubernetes
└── scripts/                # Dev helper scripts
```

## Local development

1. Copy environment template:

   ```bash
   cp .env.example .env
   ```

2. Start infrastructure and services:

   ```bash
   make dev-up
   ```

3. Health checks:

   | Service   | URL                        |
   |-----------|----------------------------|
   | Ingestion | http://localhost:8080/healthz |
   | Consumer  | http://localhost:8081/healthz |
   | API       | http://localhost:3000/healthz |
   | Web       | http://localhost:3001        |

## Tech stack

- **Ingestion / Consumer:** Go 1.22+
- **API:** Node.js + TypeScript (NestJS)
- **Web:** React + TypeScript (Next.js)
- **Data:** PostgreSQL 16, ClickHouse, Redis 7, Kafka

See `.cursor/rules/project.mdc` for full architecture rules.
