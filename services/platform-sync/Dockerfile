# Build from repo root: context `..`, dockerfile `services/platform-sync/Dockerfile`
FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY packages/pingdedup ./packages/pingdedup
COPY services/platform-sync/go.mod services/platform-sync/go.sum* ./services/platform-sync/
WORKDIR /src/services/platform-sync
RUN go mod download

COPY services/platform-sync/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X main.dedupMode=content_hash" -o /platform-sync ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /platform-sync .
EXPOSE 8088
CMD ["./platform-sync"]
