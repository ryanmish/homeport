#!/bin/bash
set -e

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
echo "  - Systemd services"
echo "  - CLI command (/usr/local/bin/homeport)"
echo ""
echo -e "${YELLOW}What will NOT be removed:${NC}"
echo "  - Docker, GitHub CLI, cloudflared (system packages)"
echo "  - Cloudflare Tunnel (you can delete it from dashboard)"
echo "  - Source code directory (delete manually if desired)"
echo ""
read -p "Also jettison cargo (Docker volumes with repos/settings)? (y/n) " -n 1 -r
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

# Stop and disable services
echo "Powering down reactor..."
sudo systemctl stop homeport.service 2>/dev/null || true
sudo systemctl disable homeport.service 2>/dev/null || true
sudo rm -f /etc/systemd/system/homeport.service
sudo systemctl daemon-reload
echo -e "${GREEN}[*]${NC} Reactor offline"

# Stop cloudflared (but don't uninstall - user may want it)
echo "Closing tunnel..."
sudo systemctl stop cloudflared 2>/dev/null || true
echo -e "${GREEN}[*]${NC} Tunnel sealed"

# Stop Docker containers
if [ -d "$HOMEPORT_DIR/docker" ]; then
    echo "Evacuating modules..."
    cd "$HOMEPORT_DIR/docker"
    if docker compose version &> /dev/null; then
        COMPOSE="docker compose"
    else
        COMPOSE="docker-compose"
    fi

    $COMPOSE down --rmi local 2>/dev/null || true
    echo -e "${GREEN}[*]${NC} Modules evacuated"

    # Remove volumes if requested
    if [[ $REMOVE_VOLUMES =~ ^[Yy]$ ]]; then
        echo "Jettisoning cargo..."
        # Volume names are prefixed with directory name (docker_)
        docker volume rm docker_homeport-data 2>/dev/null || true
        docker volume rm docker_repos 2>/dev/null || true
        docker volume rm docker_code-server-data 2>/dev/null || true
        docker volume rm docker_code-server-config 2>/dev/null || true
        docker volume rm docker_caddy-data 2>/dev/null || true
        docker volume rm docker_caddy-config 2>/dev/null || true
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
echo "  rm -rf $HOMEPORT_DIR           # Remove station blueprints"
echo "  cloudflared tunnel delete ...  # Close Cloudflare tunnel"
echo ""
echo "Support systems remain installed (Docker, gh, cloudflared)."
echo ""
echo "Farewell, Commander."
echo ""
