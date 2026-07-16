# Build from repo root: context `..`, dockerfile `services/ingestion/Dockerfile`
FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY packages/pingdedup ./packages/pingdedup
COPY services/ingestion/go.mod services/ingestion/go.sum* ./services/ingestion/
WORKDIR /src/services/ingestion
RUN go mod download

COPY services/ingestion/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /ingestion ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /ingestion .

EXPOSE 8080

USER nobody

ENTRYPOINT ["./ingestion"]
