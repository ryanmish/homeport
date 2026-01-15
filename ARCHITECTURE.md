# Homeport Architecture

A persistent, self-hosted remote development environment that provides browser-based VS Code, managed dev server routing, and secure remote access on hardware you own.

---

## Decisions Summary

### Naming & Branding
- **Name**: Homeport
- **Domain/GitHub**: TBD (gethomeport.com / homeportdev.com available)

### Hardware
- **Server**: Dell OptiPlex 3070 Micro (i5-9500T 6-core, 16GB RAM, SSD)
- **OS**: Ubuntu (to be installed)
- **Role**: Alongside other Docker services (not dedicated)

### Core Tech Stack
- **Daemon**: Go (fast, single binary, great for system-level work)
- **UI**: Minimal, responsive, Cmd+K command palette, light mode only
- **State**: SQLite
- **Deployment**: Docker Compose
- **Proxy**: Caddy

### Remote Access
- **Primary**: Cloudflare Tunnel (path-per-port: `dev.domain.com/3000`)
- **Fallback**: Tailscale for when CF is down
- **SSL**: Cloudflare terminates SSL, internal traffic is HTTP
- **Auth**: Cloudflare Access as base layer, optional password per port

### GitHub Integration
- **Auth**: `gh auth login` (GitHub CLI)
- **Cloning**: On-demand via UI (paste URL or pick from list)
- **Sync**: Pull on demand through UI
- **Git flow**: Claude Code commits and pushes directly

### code-server
- **Management**: Homeport installs, configures, and supervises it
- **Instances**: One shared instance (switch folders as needed)
- **Access**: Opens directly to selected repo

---

## Mental Model

**Repo-first, not port-first.**

```
Homeport Dashboard
├── manifold (running: 8000, 3000)
│   ├── :8000 - backend [private]
│   └── :3000 - frontend [shared, expires 24h]
├── client-website (stopped)
└── my-nextjs-app (running: 5173)
    └── :5173 - dev server [private]
```

Every dev server belongs to a repo. No orphan ports.

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         INTERNET                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Cloudflare Tunnel                             │
│                  (dev.yourdomain.com)                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Caddy                                    │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  /           → Homeport UI (control panel)               │   │
│  │  /code       → code-server                               │   │
│  │  /3000       → localhost:3000 (via homeportd)            │   │
│  │  /5173       → localhost:5173 (via homeportd)            │   │
│  │  /api        → homeportd API                             │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│   Homeport UI   │ │   code-server   │ │   homeportd     │
│   (static SPA)  │ │   (VS Code)     │ │   (daemon)      │
└─────────────────┘ └─────────────────┘ └─────────────────┘
                              │                   │
                              ▼                   ▼
                    ┌─────────────────────────────────────┐
                    │            /srv/homeport            │
                    │  ├── repos/                         │
                    │  │   ├── manifold/                  │
                    │  │   └── my-nextjs-app/             │
                    │  ├── data/                          │
                    │  │   ├── homeport.db (SQLite)       │
                    │  │   └── config.yaml                │
                    │  └── code-server/                   │
                    │      └── (settings, extensions)     │
                    └─────────────────────────────────────┘
```

---

## Components

### 1. homeportd (Go daemon)

The core daemon that manages everything.

**Responsibilities:**
- Port scanning (3000-9999) to auto-detect dev servers
- Process supervision for dev servers started via UI
- Sharing policy enforcement (private/password/public)
- State persistence (repos, ports, sharing config)
- CLI socket for `homeport` commands
- REST API for UI and menu bar client

**API Endpoints:**
```
GET    /api/repos              - List repos
POST   /api/repos              - Add repo (clone from GitHub)
DELETE /api/repos/:id          - Remove repo
GET    /api/repos/:id/servers  - List dev servers for repo
POST   /api/repos/:id/start    - Start dev server (detect from package.json)
POST   /api/repos/:id/stop     - Stop dev server

GET    /api/ports              - List detected ports
POST   /api/ports/:port/share  - Set sharing mode
GET    /api/ports/:port/logs   - Access logs for shared port

GET    /api/status             - System status
POST   /api/proxy/:port/*      - Proxy request to localhost:port
```

**Port Scanning:**
- Scan every 5 seconds
- Range: 3000-9999
- Method: `ss` or `/proc/net/tcp`
- Associate ports with repos by checking CWD of process

**Sharing Modes:**
```go
type ShareMode struct {
    Mode         string    // "private" | "password" | "public"
    Password     string    // bcrypt hash (if mode=password)
    ExpiresAt    *time.Time // nil = never
    AccessLog    []AccessEntry
}
```

**State Persistence:**
- SQLite database at `/srv/homeport/data/homeport.db`
- Restore running servers on reboot
- Configurable idle timeout to stop servers

### 2. Homeport CLI

Installed on the server, usable by Claude Code and via SSH.

```bash
# Repo management
homeport repos                    # List repos
homeport clone <github-url>       # Clone a repo
homeport open <repo>              # Open repo in code-server (returns URL)

# Server management
homeport start <repo>             # Start dev server (detects from package.json)
homeport stop <repo>              # Stop dev server
homeport restart <repo>           # Restart dev server
homeport list                     # List all running servers

# Sharing
homeport share <port>             # Make port accessible (private by default)
homeport share <port> --password  # Password-protect (prompts for password)
homeport share <port> --public    # Make fully public
homeport share <port> --expires 24h  # Set expiration
homeport unshare <port>           # Remove sharing
homeport url <port>               # Get shareable URL

# Info
homeport status                   # Overall status
homeport logs <port>              # Show access logs for port
```

### 3. Homeport UI (Web Dashboard)

Minimal, clean, light-mode SPA.

**Views:**
1. **Dashboard** - List of repos with running/stopped status + system stats (CPU/RAM/disk)
2. **Repo Detail** - Dev servers for this repo, start/stop, sharing controls
3. **Share Modal** - Set mode, password, expiration
4. **Settings** - Port ranges, timeouts, GitHub connection
5. **Logs** - Access logs for shared ports

**Features:**
- Responsive (works on iPad and phone)
- Cmd+K command palette
- Real-time updates (WebSocket)
- One-click open code-server for any repo
- Quick share with link copy

**Tech:**
- Vanilla JS or Preact (keep it minimal)
- Tailwind CSS
- Built into single static bundle

### 4. code-server

Browser-based VS Code.

**Configuration:**
- Managed by homeportd
- Extensions: Claude Code pre-installed
- Auth: Handled by Cloudflare Access (code-server auth disabled)
- Workspace: Opens to `/srv/homeport/repos`

### 5. Caddy (Reverse Proxy)

**Routes:**
```
dev.yourdomain.com/         → Homeport UI
dev.yourdomain.com/code/*   → code-server
dev.yourdomain.com/api/*    → homeportd API
dev.yourdomain.com/:port/*  → homeportd proxy → localhost:port
```

**Dynamic routing:**
- Caddy config is static
- Port routing goes through homeportd which handles auth and proxies
- WebSocket connections proxied for hot reload (Vite HMR, Next.js Fast Refresh)

### 6. Cloudflare Tunnel + Access

**Tunnel:**
- Single tunnel to Caddy
- Ingress rule: `dev.yourdomain.com` → `localhost:80`

**Access Policy:**
- Application: `dev.yourdomain.com/*`
- Policy: Email = your-email@domain.com (or one-time PIN)
- Bypass: None (everything behind Access)

**Per-port passwords are additional:**
- Cloudflare Access gets you to the proxy
- homeportd enforces port-level passwords

---

## Data Model

```sql
-- Repos
CREATE TABLE repos (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    github_url TEXT,
    start_command TEXT,  -- Auto-detected or manual override
    auto_start BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- Detected/registered ports
CREATE TABLE ports (
    port INTEGER PRIMARY KEY,
    repo_id TEXT REFERENCES repos(id),
    pid INTEGER,
    share_mode TEXT DEFAULT 'private',  -- private, password, public
    password_hash TEXT,
    expires_at TIMESTAMP,
    first_seen TIMESTAMP,
    last_seen TIMESTAMP
);

-- Access logs
CREATE TABLE access_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    port INTEGER,
    ip TEXT,
    user_agent TEXT,
    timestamp TIMESTAMP,
    authenticated BOOLEAN
);

-- Server state (for restore on reboot)
CREATE TABLE server_state (
    repo_id TEXT PRIMARY KEY,
    was_running BOOLEAN,
    ports TEXT,  -- JSON array of ports
    stopped_at TIMESTAMP
);
```

---

## File Structure

```
/srv/homeport/
├── repos/                    # Cloned repositories
│   ├── manifold/
│   └── my-nextjs-app/
├── data/
│   ├── homeport.db          # SQLite database
│   └── config.yaml          # Configuration
└── code-server/
    ├── config.yaml          # code-server config
    └── extensions/          # Installed extensions

~/Desktop/ClaudeFolder/homeport/  # Source code
├── cmd/
│   ├── homeportd/           # Daemon entrypoint
│   └── homeport/            # CLI entrypoint
├── internal/
│   ├── daemon/              # Core daemon logic
│   ├── port/                # Port scanning
│   ├── proxy/               # HTTP proxy
│   ├── repo/                # Repo management
│   ├── share/               # Sharing logic
│   └── store/               # SQLite storage
├── ui/                      # Web UI source
│   ├── index.html
│   ├── app.js
│   └── style.css
├── docker/
│   ├── Dockerfile
│   ├── docker-compose.yml
│   └── Caddyfile
├── scripts/
│   └── install.sh           # CLI installer
├── go.mod
├── go.sum
├── ARCHITECTURE.md
└── README.md
```

---

## Installation Flow

```bash
# 1. Clone and run installer
git clone https://github.com/gethomeport/homeport
cd homeport
./scripts/install.sh

# 2. Installer prompts:
#    - Cloudflare Tunnel setup (browser auth)
#    - Domain to use
#    - GitHub auth (runs `gh auth login`)
#    - Port range (default 3000-9999)

# 3. Starts Docker Compose
docker compose up -d

# 4. Access at https://dev.yourdomain.com
```

---

## MVP Scope

**Must have for v0.1:**
1. homeportd daemon with port scanning
2. Basic web UI showing repos and running servers
3. code-server integration (open repo in browser VS Code)
4. Cloudflare Tunnel setup
5. Share port with optional password
6. CLI: `homeport list`, `homeport share`, `homeport url`

**Deferred:**
- Auto-detect start command from package.json
- UI "Start" button (v0.1 relies on manual `npm run dev`)
- Access logs UI
- Cmd+K command palette
- Tailscale fallback
- Reboot state restoration
- Menu bar macOS client

---

## Security Considerations

1. **No inbound ports** - All access via Cloudflare Tunnel
2. **Cloudflare Access** - Identity-based auth for everything
3. **Password hashing** - bcrypt for port passwords
4. **Port range restriction** - Only scan/proxy configured ranges
5. **No privilege escalation** - Daemon runs as non-root
6. **Secrets in .env** - Never in code or database

---

## Performance Goals

- Port scan: < 100ms
- Proxy latency: < 10ms added
- UI load: < 500ms
- Dev server access: No perceptible delay vs localhost

---

## Future Phases

**Phase 2: Temporary Instances**
- One-click WordPress, Ghost, etc.
- Docker-based app templates
- Isolated environments per instance

**Phase 3: macOS Menu Bar Client**
- Swift/SwiftUI native app
- List repos and running servers
- Start/stop/share controls
- One-click open URLs
- Auth via API token

---

## Resolved Questions

1. **Hot reload**: Yes, must work. Proxy WebSocket connections for Vite/Next.js HMR.
2. **Process management**: Let servers die when terminal closes. No supervision needed. Homeport just detects what's running.
3. **URL structure**: Paths only (dev.domain.com/3000). Simpler setup, one wildcard cert.
4. **System stats**: Yes, show CPU/RAM/disk on dashboard.

---

## Updates & Releases

Homeport supports one-click self-upgrades from the UI.

### How It Works

**Publishing (maintainer):**
1. Commit and push changes to GitHub
2. Create a release: `gh release create v1.2.0 --title "v1.2.0" --notes "What's new"`
3. GitHub Actions automatically builds and pushes Docker image to `ghcr.io/ryanmish/homeport:v1.2.0`

**Upgrading (user):**
1. Dashboard checks for updates on page load
2. Blue notification dot appears on Settings button when update available
3. Open Settings → Updates section shows version comparison and release notes
4. Click "Upgrade" → Downloading (~30s) → Restarting (~5s) → Verifying → Done
5. Page auto-reloads with new version

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  User clicks "Upgrade" in Settings                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  POST /api/upgrade                                              │
│  Backend triggers docker/upgrade.sh                             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  upgrade.sh (runs detached, survives container restart)         │
│  1. Pre-flight: Check disk space (need 1GB free)                │
│  2. docker pull ghcr.io/ryanmish/homeport:v1.2.0 (retry 3x)     │
│  3. Tag current image as :rollback                              │
│  4. docker compose up -d (restart with new image)               │
│  5. Health check: curl /api/status (30s timeout)                │
│  6. Write status to /srv/homeport/data/upgrade-status.json      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Frontend polls GET /api/upgrade/status                         │
│  Shows progress: Downloading → Restarting → Verifying → Done    │
│  Auto-reloads page on completion                                │
└─────────────────────────────────────────────────────────────────┘
```

### Key Files

| File | Purpose |
|------|---------|
| `.github/workflows/release.yml` | Builds and pushes Docker image on GitHub release |
| `docker/upgrade.sh` | Upgrade script with retries, health check, rollback tagging |
| `internal/api/upgrade.go` | API handlers for `/api/upgrade` and `/api/upgrade/status` |
| `internal/version/version.go` | Version checking against GitHub releases |

### Version Injection

Version is injected at build time via Go ldflags:

```dockerfile
ARG VERSION=dev
RUN go build -ldflags="-s -w -X .../version.Version=${VERSION}" -o homeportd
```

GitHub Actions passes `VERSION=${{ github.ref_name }}` (e.g., `v1.2.0`).

### Resilience

| Scenario | Handling |
|----------|----------|
| Network error during pull | Retry up to 3 times with 5s delay |
| Disk space insufficient | Pre-flight check blocks upgrade |
| Concurrent upgrade attempts | Lock file prevents race condition |
| New container crashes | Health check detects, reports error |
| User closes browser | Upgrade continues, reload shows result |

### Manual Rollback

If an upgrade fails, the previous image is tagged as `:rollback`:

```bash
docker tag ghcr.io/ryanmish/homeport:rollback ghcr.io/ryanmish/homeport:latest
cd ~/homeport/docker && docker compose up -d
```

### API Endpoints

```
GET  /api/updates         - Check for updates (compares with GitHub releases)
POST /api/upgrade         - Start upgrade (body: { "version": "v1.2.0" })
GET  /api/upgrade/status  - Poll upgrade progress
```

**Update response:**
```json
{
  "current_version": "1.1.0",
  "latest_version": "1.2.0",
  "update_available": true,
  "release_notes": "## What's New\n- Feature X\n- Bug fix Y",
  "release_url": "https://github.com/ryanmish/homeport/releases/tag/v1.2.0"
}
```

**Status response:**
```json
{
  "step": "pulling",
  "message": "Downloading update...",
  "error": false,
  "completed": false,
  "version": "1.2.0"
}
```
