FROM golang:1.22-alpine AS builder
WORKDIR /src/services/ad-attribution
COPY services/ad-attribution/go.mod services/ad-attribution/go.sum* ./
RUN go mod download
COPY services/ad-attribution/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /ad-attribution ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /ad-attribution .
EXPOSE 8087
USER nobody
ENTRYPOINT ["./ad-attribution"]
