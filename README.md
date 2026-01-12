# Homeport

Self-hosted remote development environment. Access your dev servers from anywhere.

## What is this?

Homeport turns any Linux server into a remote development environment:

- **Browser-based VS Code** via code-server
- **Dev server access** - Your `localhost:3000` becomes `https://dev.yourdomain.com/3000/`
- **Secure by default** - Cloudflare Tunnel + Access for authentication
- **CLI control** - Manage everything from the terminal

Perfect for coding from an iPad, phone, or any device with a browser.

## Quick Start

### Prerequisites

- Fresh Ubuntu server (22.04+ recommended)
- Domain managed by Cloudflare
- That's it! The installer handles everything else.

### Install

```bash
# One-liner install (works on fresh Ubuntu)
curl -fsSL https://raw.githubusercontent.com/ryanmish/homeport/main/scripts/bootstrap.sh | bash
```

Or clone first:
```bash
git clone https://github.com/ryanmish/homeport.git
cd homeport
./scripts/install.sh
```

The installer will:
1. Install Docker, GitHub CLI, cloudflared
2. Authenticate with GitHub and Cloudflare
3. Create and configure your tunnel
4. Build and start all services

### Access

- `https://yourdomain.com/` - Homeport dashboard
- `https://yourdomain.com/code/` - VS Code in browser
- `https://yourdomain.com/3000/` - Your dev server on port 3000

## CLI

```bash
homeport list                    # Show detected ports
homeport share 3000 --public     # Make port publicly accessible
homeport share 3000 --password   # Require password
homeport unshare 3000            # Back to private
homeport url 3000                # Get shareable URL
homeport status                  # Daemon status
homeport repos                   # List cloned repos
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Cloudflare Tunnel                     │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                        Caddy                             │
│  /code/* → code-server    /* → homeportd                │
└─────────────────────────┬───────────────────────────────┘
                          │
         ┌────────────────┼────────────────┐
         │                │                │
         ▼                ▼                ▼
   ┌──────────┐    ┌──────────┐    ┌──────────┐
   │homeportd │    │code-server│   │ dev servers│
   │  :8080   │    │  :8443   │    │ :3000-9999│
   └──────────┘    └──────────┘    └──────────┘
```

## Development

```bash
# Run daemon locally (Mac or Linux)
go run ./cmd/homeportd --dev

# Build
go build -o bin/homeportd ./cmd/homeportd
go build -o bin/homeport ./cmd/homeport

# Build UI
cd ui && npm install && npm run build
```

## License

MIT with Commons Clause - Free to use, modify, and distribute. Cannot be sold.
