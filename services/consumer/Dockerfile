# Build stage
FROM golang:1.24.13-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /consumer ./cmd/server

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /consumer .

EXPOSE 8081

USER nobody

ENTRYPOINT ["./consumer"]
