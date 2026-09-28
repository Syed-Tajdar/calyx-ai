# Build from repo root so catalog.json is available
FROM golang:1.25.14-alpine AS builder
WORKDIR /src
COPY shared/providers/catalog.json ./catalog.json
COPY services/providerconfig/go.mod services/providerconfig/go.sum* ./services/providerconfig/
WORKDIR /src/services/providerconfig
RUN go mod download
COPY services/providerconfig/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /providerconfig ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /providerconfig .
COPY --from=builder /src/catalog.json ./catalog.json
EXPOSE 8084
USER nobody
ENV PROVIDER_CATALOG_PATH=/app/catalog.json
ENTRYPOINT ["./providerconfig"]
