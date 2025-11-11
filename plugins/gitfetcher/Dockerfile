# Build stage
FROM golang:1.23.1-alpine AS builder

WORKDIR /build

# Install git (needed for go mod download)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o gitfetcher -ldflags="-w -s" .

# Runtime stage
FROM alpine:latest

# Install git and openssh
RUN apk add --no-cache git openssh-client

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/gitfetcher /app/gitfetcher

# Create directories
RUN mkdir -p /repos /root/.ssh /app/logs

# Set proper SSH permissions
RUN chmod 700 /root/.ssh

# Expose HTTP port
EXPOSE 8080

# Run the application
CMD ["/app/gitfetcher", "-config", "/app/config.yaml"]
