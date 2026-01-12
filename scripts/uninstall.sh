#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

echo ""
echo -e "${RED}Homeport Uninstaller${NC}"
echo "===================="
echo ""
echo "This will remove Homeport from your system."
echo ""
echo "What will be removed:"
echo "  - Docker containers and images"
echo "  - Systemd services"
echo "  - CLI command (/usr/local/bin/homeport)"
echo ""
echo -e "${YELLOW}What will NOT be removed:${NC}"
echo "  - Docker, GitHub CLI, cloudflared (system packages)"
echo "  - Cloudflare Tunnel (you can delete it from dashboard)"
echo "  - ~/homeport directory (delete manually if desired)"
echo "  - Your cloned repositories"
echo ""
read -p "Are you sure you want to uninstall? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Uninstall cancelled."
    exit 0
fi

echo ""

# Stop and disable services
echo "Stopping services..."
sudo systemctl stop homeport.service 2>/dev/null || true
sudo systemctl disable homeport.service 2>/dev/null || true
sudo rm -f /etc/systemd/system/homeport.service
sudo systemctl daemon-reload
echo -e "${GREEN}✓${NC} Homeport service removed"

# Stop cloudflared (but don't uninstall - user may want it)
echo "Stopping Cloudflare Tunnel..."
sudo systemctl stop cloudflared 2>/dev/null || true
echo -e "${GREEN}✓${NC} Tunnel stopped (service still installed)"

# Stop Docker containers
HOMEPORT_DIR="$HOME/homeport"
if [ -d "$HOMEPORT_DIR/docker" ]; then
    echo "Stopping Docker containers..."
    cd "$HOMEPORT_DIR/docker"
    if docker compose version &> /dev/null; then
        docker compose down --rmi local 2>/dev/null || true
    else
        docker-compose down --rmi local 2>/dev/null || true
    fi
    echo -e "${GREEN}✓${NC} Docker containers removed"
fi

# Remove CLI
echo "Removing CLI..."
sudo rm -f /usr/local/bin/homeport
echo -e "${GREEN}✓${NC} CLI removed"

echo ""
echo -e "${GREEN}Uninstall complete!${NC}"
echo ""
echo "To fully clean up:"
echo "  rm -rf ~/homeport              # Remove source code"
echo "  cloudflared tunnel delete ...  # Delete tunnel from Cloudflare"
echo ""
echo "Docker, GitHub CLI, and cloudflared were left installed."
echo "Remove them with: sudo apt remove docker-ce gh cloudflared"
echo ""
