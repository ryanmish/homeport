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

# Build daemon and CLI
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o homeportd ./cmd/homeportd
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o homeport ./cmd/homeport

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

# Install runtime dependencies including gh CLI
RUN apk add --no-cache \
    ca-certificates \
    git \
    openssh-client \
    github-cli \
    && rm -rf /var/cache/apk/*

# Create app user
RUN adduser -D -h /home/homeport homeport

# Create directories
RUN mkdir -p /srv/homeport/data /srv/homeport/repos /srv/homeport/ui \
    && chown -R homeport:homeport /srv/homeport

WORKDIR /srv/homeport

# Copy binaries
COPY --from=builder /build/homeportd /usr/local/bin/
COPY --from=builder /build/homeport /usr/local/bin/

# Copy built UI
COPY --from=ui-builder /build/ui/dist /srv/homeport/ui/

# Copy default config
COPY docker/homeport.yaml /srv/homeport/config.yaml

# Switch to app user
USER homeport

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/status || exit 1

ENTRYPOINT ["homeportd"]
CMD ["--config", "/srv/homeport/config.yaml"]
