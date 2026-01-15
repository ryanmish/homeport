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

### Option 1: Direct deploy (dev/testing)
```bash
# 1. Commit and push
git add -A && git commit -m "Your message" && git push

# 2. Deploy on server (builds from source)
ssh ryan@devbox "cd ~/homeport/docker && git -C .. pull && docker compose build && docker compose up -d"
```

### Option 2: Release (production)
```bash
# 1. Commit and push
git add -A && git commit -m "Your message" && git push

# 2. Create a GitHub release (triggers Docker image build)
gh release create v1.2.0 --title "v1.2.0" --notes "What's new in this release"

# 3. Users can now upgrade via Settings → Updates in the UI
```

GitHub Actions automatically builds and pushes the Docker image to `ghcr.io/ryanmish/homeport:v1.2.0`.
Users see a notification dot on Settings when an update is available and can one-click upgrade.

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
