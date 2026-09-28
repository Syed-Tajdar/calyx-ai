FROM golang:1.25.14-alpine AS builder
WORKDIR /src/services/ask-ai
COPY services/ask-ai/go.mod services/ask-ai/go.sum* ./
RUN go mod download
COPY services/ask-ai/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /ask-ai ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /ask-ai .
EXPOSE 8086
USER nobody
ENTRYPOINT ["./ask-ai"]
