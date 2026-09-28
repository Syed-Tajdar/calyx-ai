# Build from repo root: context `..`, dockerfile `services/platform-sync/Dockerfile`
FROM golang:1.25.14-alpine AS builder

WORKDIR /src

COPY packages/pingdedup ./packages/pingdedup
COPY services/platform-sync/go.mod services/platform-sync/go.sum* ./services/platform-sync/
WORKDIR /src/services/platform-sync
RUN go mod download

COPY services/platform-sync/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X main.dedupMode=content_hash" -o /platform-sync ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
  && adduser -D -H -u 65532 nonroot
WORKDIR /app
COPY --from=builder /platform-sync .
RUN chown nonroot:nonroot /app/platform-sync
USER nonroot
EXPOSE 8088
CMD ["./platform-sync"]
