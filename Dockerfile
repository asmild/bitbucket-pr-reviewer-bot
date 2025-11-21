# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o bb-pr-reviewer ./cmd/server/main.go

# Runtime stage
FROM node:20-alpine

RUN apk add --no-cache git bash ca-certificates && \
    npm install -g @anthropic-ai/claude-code

WORKDIR /app

COPY --from=builder /build/bb-pr-reviewer .
COPY templates ./templates

RUN adduser -D appuser && \
    mkdir -p projects logs metrics-storage && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./bb-pr-reviewer"]
