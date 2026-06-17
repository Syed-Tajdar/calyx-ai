FROM golang:1.22-alpine AS builder
WORKDIR /src/services/compliance-intelligence
COPY services/compliance-intelligence/go.mod services/compliance-intelligence/go.sum* ./
RUN go mod download
COPY services/compliance-intelligence/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /compliance-intelligence ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /compliance-intelligence .
EXPOSE 8085
USER nobody
ENTRYPOINT ["./compliance-intelligence"]
