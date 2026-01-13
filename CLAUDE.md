# Homeport - Project Instructions

## Overview
Homeport is a self-hosted remote development environment. See `ARCHITECTURE.md` for full system design.

## Tech Stack
- **Daemon**: Go
- **UI**: React + Tailwind
- **Database**: SQLite
- **Proxy**: Caddy
- **Deployment**: Docker Compose
- **Remote Access**: Cloudflare Tunnel

## Deployment

### Deploy changes to server
```bash
# 1. Commit and push
git add -A && git commit -m "Your message" && git push

# 2. Deploy on server (run this one command)
ssh ryan@devbox "cd ~/homeport/docker && git -C .. pull && docker compose build && docker compose up -d"
```

### Quick deploy alias (add to ~/.zshrc)
```bash
alias homeport-deploy='cd /Users/ryanmish/Desktop/ClaudeFolder/homeport && git push && ssh ryan@devbox "cd ~/homeport/docker && git -C .. pull && docker compose build && docker compose up -d"'
```

## Development

### Running locally
```bash
# Run daemon in dev mode
go run ./cmd/homeportd

# Build UI
cd ui && npm run build

# Build Go
go build -o bin/homeportd ./cmd/homeportd
go build -o bin/homeport ./cmd/homeport
```

### Key directories
- `cmd/homeportd/` - Daemon entrypoint
- `cmd/homeport/` - CLI entrypoint
- `internal/` - Core packages
- `ui/` - Web dashboard source (React)
- `ui/src/components/ShareMenu.tsx` - Shared share modal component
- `docker/` - Docker Compose and Caddy config

## Conventions
- Keep the UI minimal and fast
- Light mode only
- Repo-first mental model (ports belong to repos)
- CLI commands should be usable by Claude Code
- Share modal is a shared React component used by both dashboard and VS Code header
