# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install git for go mod download
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Version is computed from git tags at build time
# If VERSION arg is provided and not "dev"/"latest", use it (for CI builds)
# Otherwise, get version from git describe
ARG VERSION=auto
RUN if [ "$VERSION" = "auto" ] || [ "$VERSION" = "dev" ] || [ "$VERSION" = "latest" ]; then \
        VERSION=$(git describe --tags --always 2>/dev/null || echo "dev"); \
    fi && \
    echo "Building version: $VERSION" && \
    CGO_ENABLED=0 GOOS=linux go build \
        -ldflags="-s -w -X github.com/gethomeport/homeport/internal/version.Version=${VERSION}" \
        -o homeportd ./cmd/homeportd && \
    CGO_ENABLED=0 GOOS=linux go build \
        -ldflags="-s -w -X github.com/gethomeport/homeport/internal/version.Version=${VERSION}" \
        -o homeport ./cmd/homeport

# Build UI
FROM node:20-alpine AS ui-builder

WORKDIR /build/ui

# Copy UI source
COPY ui/package*.json ./
RUN npm ci

COPY ui/ ./
RUN npm run build

# Final stage
FROM alpine:3.19

# Install runtime dependencies including gh CLI and Node.js
RUN apk add --no-cache \
    ca-certificates \
    git \
    git-lfs \
    openssh-client \
    github-cli \
    nodejs \
    npm \
    bash \
    su-exec \
    icu-data-full \
    curl \
    # Docker CLI for self-upgrade capability
    docker-cli \
    docker-cli-compose \
    # Build tools for native npm modules (sharp, bcrypt, sqlite3, etc.)
    python3 \
    make \
    g++ \
    # Fonts for PDF generation, canvas, etc.
    fontconfig \
    ttf-dejavu \
    && git lfs install \
    && rm -rf /var/cache/apk/*

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Create app user
RUN adduser -D -u 1000 -h /home/homeport homeport

# Create directories
RUN mkdir -p /srv/homeport/data /srv/homeport/repos /srv/homeport/ui \
    && chown -R homeport:homeport /srv/homeport

# Create config directories (Claude, general .config for tools like Astro)
RUN mkdir -p /home/homeport/.claude /home/homeport/.config \
    && chown -R homeport:homeport /home/homeport

# Set environment variables for all processes (including docker exec shells)
ENV HOME=/home/homeport
ENV USER=homeport
ENV SHELL=/bin/bash
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8

WORKDIR /srv/homeport

# Copy binaries
COPY --from=builder /build/homeportd /usr/local/bin/
COPY --from=builder /build/homeport /usr/local/bin/

# Copy built UI
COPY --from=ui-builder /build/ui/dist /srv/homeport/ui/

# Copy default config
COPY docker/homeport.yaml /srv/homeport/config.yaml

# Copy entrypoint script
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/status || exit 1

ENTRYPOINT ["/entrypoint.sh"]
CMD ["--config", "/srv/homeport/config.yaml"]
