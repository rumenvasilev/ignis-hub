# Build stage
FROM golang:1.25.5-alpine AS builder

ARG PREBUILT=false
ARG VERSION=dev
ARG TARGETARCH

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# If using prebuilt binary, copy it; otherwise build from source
COPY go.mod go.sum ./
RUN if [ "$PREBUILT" = "false" ]; then go mod download; fi

COPY . .

# Build or use pre-built binary
RUN if [ "$PREBUILT" = "true" ]; then \
      cp dist/ignis-hub-linux-${TARGETARCH} ignis-hub && chmod +x ignis-hub; \
    else \
      CGO_ENABLED=0 GOOS=linux go build \
        -a \
        -ldflags="-s -w -X main.Version=${VERSION}" \
        -o ignis-hub .; \
    fi

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates wget

# Create non-root user
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/ignis-hub .

# Copy configuration file template
COPY --from=builder /app/config.yaml ./config.yaml.example

# Change ownership
RUN chown -R app:app /app

# Switch to non-root user
USER app

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["./ignis-hub"]
