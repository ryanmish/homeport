# Homeport - Project Instructions

## Overview
Homeport is a self-hosted remote development environment. See `ARCHITECTURE.md` for full system design.

## Tech Stack
- **Daemon**: Go
- **UI**: Vanilla JS + Tailwind (minimal)
- **Database**: SQLite
- **Proxy**: Caddy
- **Deployment**: Docker Compose
- **Remote Access**: Cloudflare Tunnel

## Development

### Running locally
```bash
# Run daemon in dev mode
go run ./cmd/homeportd

# Build
go build -o bin/homeportd ./cmd/homeportd
go build -o bin/homeport ./cmd/homeport
```

### Key directories
- `cmd/homeportd/` - Daemon entrypoint
- `cmd/homeport/` - CLI entrypoint
- `internal/` - Core packages
- `ui/` - Web dashboard source
- `docker/` - Docker Compose and Caddy config

## Conventions
- Keep the UI minimal and fast
- Light mode only
- Repo-first mental model (ports belong to repos)
- CLI commands should be usable by Claude Code
