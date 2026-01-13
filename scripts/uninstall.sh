#!/bin/bash
set -e

# Ensure we can read from terminal
if [ ! -t 0 ]; then
    exec < /dev/tty
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

# Get script directory (same method as install.sh)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOMEPORT_DIR="$(dirname "$SCRIPT_DIR")"

echo ""
echo -e "${RED}HOMEPORT DECOMMISSION${NC}"
echo "====================="
echo ""
echo "Initiating station decommission sequence..."
echo "Installation directory: $HOMEPORT_DIR"
echo ""
echo "What will be removed:"
echo "  - Docker containers, images, and volumes"
echo "  - Systemd services (homeport + cloudflared)"
echo "  - Cloudflare tunnel and DNS routes"
echo "  - CLI command (/usr/local/bin/homeport)"
echo "  - Tunnel config (~/.cloudflared)"
echo ""
echo -e "${YELLOW}What will NOT be removed:${NC}"
echo "  - Docker, GitHub CLI, cloudflared (system packages)"
echo "  - Source code directory (delete manually if desired)"
echo ""
read -p "Also jettison cargo (delete Docker volumes with repos/settings)? (y/n) " -n 1 -r
REMOVE_VOLUMES=$REPLY
echo
echo ""
read -p "Confirm station decommission? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Decommission aborted. Station remains operational."
    exit 0
fi

echo ""

# Stop and disable homeport service
echo "Powering down reactor..."
sudo systemctl stop homeport.service 2>/dev/null || true
sudo systemctl disable homeport.service 2>/dev/null || true
sudo rm -f /etc/systemd/system/homeport.service
sudo systemctl daemon-reload
echo -e "${GREEN}[*]${NC} Reactor offline"

# Stop and fully uninstall cloudflared service
echo "Closing tunnel service..."
sudo systemctl stop cloudflared 2>/dev/null || true
sudo systemctl disable cloudflared 2>/dev/null || true
sudo cloudflared service uninstall 2>/dev/null || true
# Kill any orphaned cloudflared processes
pkill -9 cloudflared 2>/dev/null || true
echo -e "${GREEN}[*]${NC} Tunnel service stopped"

# Delete all tunnels and their DNS routes
echo "Destroying Cloudflare tunnels..."
if command -v cloudflared &> /dev/null && [ -f "$HOME/.cloudflared/cert.pem" ]; then
    # Get all tunnels
    TUNNELS=$(cloudflared tunnel list --output json 2>/dev/null | jq -r '.[].name // empty' 2>/dev/null || true)

    for TUNNEL in $TUNNELS; do
        if [ -n "$TUNNEL" ]; then
            echo "  Deleting tunnel: $TUNNEL"

            # Get tunnel ID for DNS cleanup info
            TUNNEL_ID=$(cloudflared tunnel list --output json | jq -r ".[] | select(.name==\"$TUNNEL\") | .id" 2>/dev/null || true)

            # Force delete the tunnel (this also cleans up connections)
            cloudflared tunnel delete -f "$TUNNEL" 2>/dev/null || true
        fi
    done
    echo -e "${GREEN}[*]${NC} Tunnels destroyed"
else
    echo -e "${YELLOW}[!]${NC} No cloudflared auth found, skipping tunnel deletion"
fi

# Remove cloudflared config directory
echo "Removing tunnel config..."
rm -rf ~/.cloudflared
echo -e "${GREEN}[*]${NC} Tunnel config removed"

# Stop Docker containers
if [ -d "$HOMEPORT_DIR/docker" ]; then
    echo "Evacuating modules..."
    cd "$HOMEPORT_DIR/docker"

    # Check if user has docker access
    if docker info &> /dev/null; then
        COMPOSE="docker compose"
        $COMPOSE down --rmi local 2>/dev/null || true
    elif sg docker -c "docker info" &> /dev/null; then
        sg docker -c "docker compose down --rmi local" 2>/dev/null || true
    fi
    echo -e "${GREEN}[*]${NC} Modules evacuated"

    # Remove volumes if requested
    if [[ $REMOVE_VOLUMES =~ ^[Yy]$ ]]; then
        echo "Jettisoning cargo..."
        # Volume names are prefixed with directory name (docker_)
        if docker info &> /dev/null; then
            docker volume rm docker_homeport-data docker_repos docker_code-server-data docker_code-server-config docker_caddy-data docker_caddy-config 2>/dev/null || true
        elif sg docker -c "docker info" &> /dev/null; then
            sg docker -c "docker volume rm docker_homeport-data docker_repos docker_code-server-data docker_code-server-config docker_caddy-data docker_caddy-config" 2>/dev/null || true
        fi
        echo -e "${GREEN}[*]${NC} Cargo jettisoned"
    else
        echo -e "${YELLOW}[!]${NC} Cargo preserved in storage"
    fi
fi

# Remove CLI
echo "Removing mission control interface..."
sudo rm -f /usr/local/bin/homeport
echo -e "${GREEN}[*]${NC} Interface removed"

echo ""
echo -e "${GREEN}DECOMMISSION COMPLETE${NC}"
echo ""
echo -e "${YELLOW}Note:${NC} DNS records in Cloudflare dashboard may need manual deletion:"
echo "  1. Go to https://dash.cloudflare.com"
echo "  2. Select your domain -> DNS -> Records"
echo "  3. Delete any CNAME records pointing to *.cfargotunnel.com"
echo ""
echo "Final cleanup (optional):"
echo "  rm -rf $HOMEPORT_DIR    # Remove source code"
echo ""
echo "Support systems remain installed (Docker, gh, cloudflared packages)."
echo ""
echo "Farewell, Commander."
echo ""
