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

# Get script directory
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
echo "  - Cloudflare tunnel and DNS records"
echo "  - CLI command (/usr/local/bin/homeport)"
echo "  - All config (~/.cloudflared, ~/.homeport)"
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

# Load saved config if exists
if [ -f ~/.homeport/config ]; then
    source ~/.homeport/config
    echo -e "${GREEN}[*]${NC} Loaded saved configuration"
fi

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
pkill -9 cloudflared 2>/dev/null || true
echo -e "${GREEN}[*]${NC} Tunnel service stopped"

# Note: DNS records are managed by cloudflared and will be cleaned up when tunnel is deleted

# Delete all tunnels
echo "Destroying Cloudflare tunnels..."
if command -v cloudflared &> /dev/null && [ -f "$HOME/.cloudflared/cert.pem" ]; then
    TUNNELS=$(cloudflared tunnel list --output json 2>/dev/null | jq -r '.[].name // empty' 2>/dev/null || true)

    for TUNNEL in $TUNNELS; do
        if [ -n "$TUNNEL" ]; then
            echo "  Deleting tunnel: $TUNNEL"
            cloudflared tunnel delete -f "$TUNNEL" 2>/dev/null || true
        fi
    done
    echo -e "${GREEN}[*]${NC} Tunnels destroyed"
else
    echo -e "${YELLOW}[!]${NC} No cloudflared auth found, skipping tunnel deletion"
fi

# Remove all config directories
echo "Removing configuration..."
rm -rf ~/.cloudflared
rm -rf ~/.homeport
echo -e "${GREEN}[*]${NC} Configuration removed"

# Stop Docker containers
if [ -d "$HOMEPORT_DIR/docker" ]; then
    echo "Evacuating modules..."
    cd "$HOMEPORT_DIR/docker"

    if docker info &> /dev/null; then
        docker compose down --rmi local 2>/dev/null || true
    elif sg docker -c "docker info" &> /dev/null; then
        sg docker -c "docker compose down --rmi local" 2>/dev/null || true
    fi
    echo -e "${GREEN}[*]${NC} Modules evacuated"

    if [[ $REMOVE_VOLUMES =~ ^[Yy]$ ]]; then
        echo "Jettisoning cargo..."
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
echo "Final cleanup (optional):"
echo "  rm -rf $HOMEPORT_DIR    # Remove source code"
echo ""
echo "Support systems remain installed (Docker, gh, cloudflared packages)."
echo ""
echo "Farewell, Commander."
echo ""
