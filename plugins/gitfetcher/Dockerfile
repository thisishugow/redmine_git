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

# Configure Git to trust all repositories (fix dubious ownership issue)
RUN git config --global --add safe.directory '*'

# Disable SSH strict host key checking and create wrapper script
RUN mkdir -p /root/.ssh && \
    echo "Host *" > /root/.ssh/config && \
    echo "    StrictHostKeyChecking no" >> /root/.ssh/config && \
    echo "    UserKnownHostsFile=/dev/null" >> /root/.ssh/config && \
    chmod 700 /root/.ssh && \
    chmod 600 /root/.ssh/config

# Create entrypoint script to fix SSH key permissions and Git config at runtime
RUN echo '#!/bin/sh' > /entrypoint.sh && \
    echo 'set -e' >> /entrypoint.sh && \
    echo '' >> /entrypoint.sh && \
    echo '# Set HOME to /tmp for writable config files' >> /entrypoint.sh && \
    echo 'export HOME=/tmp' >> /entrypoint.sh && \
    echo '' >> /entrypoint.sh && \
    echo '# Configure Git safe.directory in /tmp/.gitconfig' >> /entrypoint.sh && \
    echo 'git config --global --add safe.directory "*"' >> /entrypoint.sh && \
    echo '' >> /entrypoint.sh && \
    echo '# Fix SSH key permissions if mounted' >> /entrypoint.sh && \
    echo 'if [ -d /root/.ssh ]; then' >> /entrypoint.sh && \
    echo '  # Copy SSH keys to writable location' >> /entrypoint.sh && \
    echo '  mkdir -p /tmp/.ssh' >> /entrypoint.sh && \
    echo '  if [ -f /root/.ssh/id_rsa ]; then' >> /entrypoint.sh && \
    echo '    cp /root/.ssh/id_rsa /tmp/.ssh/id_rsa' >> /entrypoint.sh && \
    echo '    chmod 600 /tmp/.ssh/id_rsa' >> /entrypoint.sh && \
    echo '  fi' >> /entrypoint.sh && \
    echo '  if [ -f /root/.ssh/id_ed25519 ]; then' >> /entrypoint.sh && \
    echo '    cp /root/.ssh/id_ed25519 /tmp/.ssh/id_ed25519' >> /entrypoint.sh && \
    echo '    chmod 600 /tmp/.ssh/id_ed25519' >> /entrypoint.sh && \
    echo '  fi' >> /entrypoint.sh && \
    echo '  # Create SSH config in writable location' >> /entrypoint.sh && \
    echo '  echo "Host *" > /tmp/.ssh/config' >> /entrypoint.sh && \
    echo '  echo "    StrictHostKeyChecking no" >> /tmp/.ssh/config' >> /entrypoint.sh && \
    echo '  echo "    UserKnownHostsFile=/dev/null" >> /tmp/.ssh/config' >> /entrypoint.sh && \
    echo '  chmod 600 /tmp/.ssh/config' >> /entrypoint.sh && \
    echo '  chmod 700 /tmp/.ssh' >> /entrypoint.sh && \
    echo 'fi' >> /entrypoint.sh && \
    echo '' >> /entrypoint.sh && \
    echo 'exec "$@"' >> /entrypoint.sh && \
    chmod +x /entrypoint.sh

# Expose HTTP port
EXPOSE 8080

# Use entrypoint to fix permissions at runtime
ENTRYPOINT ["/entrypoint.sh"]

# Run the application
CMD ["/app/gitfetcher", "-config", "/app/config.yaml"]
