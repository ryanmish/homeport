#!/bin/bash

# Homeport Uninstall Script
# Thoroughly removes all Homeport components from the system

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# Parse arguments
FORCE=false
REMOVE_VOLUMES=false
for arg in "$@"; do
    case $arg in
        --force|-f)
            FORCE=true
            REMOVE_VOLUMES=true
            ;;
        --keep-volumes)
            REMOVE_VOLUMES=false
            ;;
        --help|-h)
            echo "Usage: uninstall.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --force, -f      Skip confirmation prompts and remove everything"
            echo "  --keep-volumes   Preserve Docker volumes (repos and settings)"
            echo "  --help, -h       Show this help message"
            exit 0
            ;;
    esac
done

# Ensure we can read from terminal (if not in force mode)
if [ "$FORCE" = false ] && [ ! -t 0 ]; then
    exec < /dev/tty
fi

# Get script directory - handle being run from anywhere
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOMEPORT_DIR="$(dirname "$SCRIPT_DIR")"

# Change to a safe directory (in case we're in HOMEPORT_DIR)
cd /tmp 2>/dev/null || cd ~

# Step counter
STEP=0
TOTAL_STEPS=7

step() {
    STEP=$((STEP + 1))
    echo ""
    echo -e "${BLUE}[Step $STEP/$TOTAL_STEPS]${NC} $1"
    echo "============================================="
}

echo ""
echo -e "${RED}HOMEPORT DECOMMISSION${NC}"
echo "====================="
echo ""
echo "Installation directory: $HOMEPORT_DIR"
echo ""

# Load saved config to get DOMAIN for DNS warning
SAVED_DOMAIN=""
if [ -f ~/.homeport/config ]; then
    source ~/.homeport/config
    SAVED_DOMAIN="$DOMAIN"
fi

if [ "$FORCE" = false ]; then
    echo "What will be removed:"
    echo "  - Docker containers, images, networks, and volumes"
    echo "  - Systemd services (homeport + cloudflared)"
    echo "  - Cloudflare tunnel"
    echo "  - CLI command (/usr/local/bin/homeport)"
    echo "  - All config (~/.cloudflared, ~/.homeport, .env)"
    echo "  - Temp files (/tmp/homeportd-setup)"
    echo ""
    echo -e "${YELLOW}What will NOT be removed:${NC}"
    echo "  - Docker, GitHub CLI, cloudflared packages"
    echo "  - Go runtime"
    echo "  - Source code directory"
    if [ -n "$SAVED_DOMAIN" ]; then
        echo ""
        echo -e "${YELLOW}NOTE:${NC} DNS record for ${BOLD}$SAVED_DOMAIN${NC} may need manual deletion"
        echo "      at https://dash.cloudflare.com"
    fi
    echo ""
    read -p "Delete Docker volumes (repos/settings will be lost)? (y/n) " -n 1 -r
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        REMOVE_VOLUMES=true
    fi
    echo
    echo ""
    read -p "Confirm station decommission? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Decommission aborted."
        exit 0
    fi
fi

# =============================================================================
step "Stopping all processes"
# =============================================================================

echo "  Stopping Docker containers..."
if [ -d "$HOMEPORT_DIR/docker" ]; then
    cd "$HOMEPORT_DIR/docker" 2>/dev/null || true
    if docker info &> /dev/null; then
        docker compose down 2>/dev/null || true
    elif sg docker -c "docker info" &> /dev/null; then
        sg docker -c "docker compose down" 2>/dev/null || true
    fi
    cd /tmp 2>/dev/null || cd ~
fi

# Also stop by container name directly
docker stop homeportd code-server caddy 2>/dev/null || true

echo "  Stopping systemd services..."
sudo systemctl stop homeport.service 2>/dev/null || true
sudo systemctl disable homeport.service 2>/dev/null || true
sudo systemctl stop cloudflared 2>/dev/null || true
sudo systemctl disable cloudflared 2>/dev/null || true
sudo cloudflared service uninstall 2>/dev/null || true

echo "  Killing cloudflared processes..."
# Multiple methods to ensure all processes are killed
sudo pkill -9 -x cloudflared 2>/dev/null || true
sudo pkill -9 -f "cloudflared tunnel" 2>/dev/null || true
sudo pkill -9 -f "cloudflared --config" 2>/dev/null || true
pkill -9 -x cloudflared 2>/dev/null || true
sudo killall -9 cloudflared 2>/dev/null || true
killall -9 cloudflared 2>/dev/null || true

# Wait and do a final check
sleep 2
REMAINING=$(pgrep -x cloudflared 2>/dev/null || true)
if [ -n "$REMAINING" ]; then
    echo -e "  ${YELLOW}Killing stubborn processes...${NC}"
    for pid in $REMAINING; do
        sudo kill -9 "$pid" 2>/dev/null || true
    done
    sleep 1
fi

echo -e "${GREEN}[*]${NC} Processes stopped"

# =============================================================================
step "Deleting Cloudflare tunnel"
# =============================================================================

if command -v cloudflared &> /dev/null && [ -f "$HOME/.cloudflared/cert.pem" ]; then
    TUNNELS=$(cloudflared tunnel list --output json 2>/dev/null | jq -r '.[].name // empty' 2>/dev/null || true)

    if [ -n "$TUNNELS" ]; then
        for TUNNEL in $TUNNELS; do
            if [ -n "$TUNNEL" ]; then
                echo "  Deleting tunnel: $TUNNEL"
                cloudflared tunnel cleanup "$TUNNEL" 2>/dev/null || true
                cloudflared tunnel delete -f "$TUNNEL" 2>/dev/null || true
            fi
        done
        echo -e "${GREEN}[*]${NC} Tunnels deleted"
    else
        echo -e "${DIM}  No tunnels found${NC}"
    fi
else
    echo -e "${YELLOW}[!]${NC} No cloudflared auth - manual tunnel deletion may be needed"
    echo "    Visit: https://one.dash.cloudflare.com → Zero Trust → Networks → Tunnels"
fi

# =============================================================================
step "Removing Docker resources"
# =============================================================================

if docker info &> /dev/null || sg docker -c "docker info" &> /dev/null 2>&1; then
    echo "  Removing containers..."
    docker rm -f homeportd code-server caddy 2>/dev/null || true

    echo "  Removing networks..."
    docker network rm docker_default 2>/dev/null || true
    docker network rm homeport_default 2>/dev/null || true
    # Prune any orphaned networks from this project
    docker network ls --filter "name=docker_" --filter "name=homeport_" -q 2>/dev/null | xargs -r docker network rm 2>/dev/null || true

    if [ "$REMOVE_VOLUMES" = true ]; then
        echo "  Removing volumes..."
        # Try both naming conventions
        docker volume rm docker_homeport-data docker_repos docker_code-server-data docker_code-server-config docker_caddy-data docker_caddy-config 2>/dev/null || true
        docker volume rm homeport-data repos code-server-data code-server-config caddy-data caddy-config 2>/dev/null || true
        echo -e "${GREEN}[*]${NC} Volumes removed"
    else
        echo -e "${YELLOW}[!]${NC} Volumes preserved"
    fi

    echo "  Removing images..."
    docker rmi codercom/code-server:latest caddy:2-alpine 2>/dev/null || true
    # Remove any homeport images
    docker images --filter "reference=*homeport*" -q 2>/dev/null | xargs -r docker rmi 2>/dev/null || true

    echo -e "${GREEN}[*]${NC} Docker resources cleaned"
else
    echo -e "${YELLOW}[!]${NC} Docker not accessible, skipping"
fi

# =============================================================================
step "Removing configuration files"
# =============================================================================

# Securely delete .env (contains password hash)
if [ -f "$HOMEPORT_DIR/docker/.env" ]; then
    echo "  Securely removing .env file..."
    if command -v shred &> /dev/null; then
        shred -u "$HOMEPORT_DIR/docker/.env" 2>/dev/null || rm -f "$HOMEPORT_DIR/docker/.env"
    else
        rm -f "$HOMEPORT_DIR/docker/.env"
    fi
fi

echo "  Removing ~/.cloudflared/"
rm -rf "$HOME/.cloudflared"

echo "  Removing ~/.homeport/"
rm -rf "$HOME/.homeport"

echo "  Removing /etc/cloudflared/"
sudo rm -rf /etc/cloudflared 2>/dev/null || true

echo "  Removing systemd service file..."
sudo rm -f /etc/systemd/system/homeport.service
sudo systemctl daemon-reload 2>/dev/null || true

echo -e "${GREEN}[*]${NC} Configuration removed"

# =============================================================================
step "Removing CLI and temp files"
# =============================================================================

echo "  Removing /usr/local/bin/homeport..."
sudo rm -f /usr/local/bin/homeport

echo "  Removing temp files..."
rm -f /tmp/homeportd-setup 2>/dev/null || true
rm -f /tmp/cloudflared.deb 2>/dev/null || true

echo -e "${GREEN}[*]${NC} CLI removed"

# =============================================================================
step "Final verification"
# =============================================================================

ISSUES=0

# Check processes
CF_PROCS=$(pgrep -x cloudflared 2>/dev/null | wc -l)
if [ "$CF_PROCS" -gt 0 ]; then
    echo -e "${RED}[!]${NC} cloudflared still running (PIDs: $(pgrep -x cloudflared | tr '\n' ' '))"
    ISSUES=1
fi

# Check containers
CONTAINERS=$(docker ps -a --format '{{.Names}}' 2>/dev/null | grep -E "^(homeportd|code-server|caddy)$" || true)
if [ -n "$CONTAINERS" ]; then
    echo -e "${RED}[!]${NC} Containers still exist: $CONTAINERS"
    ISSUES=1
fi

# Check config dirs
[ -d "$HOME/.cloudflared" ] && echo -e "${RED}[!]${NC} ~/.cloudflared still exists" && ISSUES=1
[ -d "$HOME/.homeport" ] && echo -e "${RED}[!]${NC} ~/.homeport still exists" && ISSUES=1
[ -f "$HOMEPORT_DIR/docker/.env" ] && echo -e "${RED}[!]${NC} .env still exists" && ISSUES=1
[ -f "/usr/local/bin/homeport" ] && echo -e "${RED}[!]${NC} CLI still exists" && ISSUES=1

if [ "$ISSUES" -eq 0 ]; then
    echo -e "${GREEN}[*]${NC} All components removed successfully"
fi

# =============================================================================
step "Summary"
# =============================================================================

echo ""
if [ "$ISSUES" -eq 0 ]; then
    echo -e "${GREEN}DECOMMISSION COMPLETE${NC}"
else
    echo -e "${YELLOW}DECOMMISSION COMPLETE (with warnings)${NC}"
fi

echo ""
echo "Remaining cleanup (optional):"
echo "  rm -rf $HOMEPORT_DIR           # Remove source code"
echo "  sudo apt remove cloudflared    # Remove cloudflared package"
echo "  sudo rm -rf /usr/local/go      # Remove Go (if installed by Homeport)"

if [ -n "$SAVED_DOMAIN" ]; then
    echo ""
    echo -e "${YELLOW}IMPORTANT:${NC} Delete DNS record for ${BOLD}$SAVED_DOMAIN${NC}"
    echo "  1. Go to https://dash.cloudflare.com"
    echo "  2. Select your domain → DNS → Records"
    echo "  3. Delete the CNAME record for $SAVED_DOMAIN"
fi

echo ""
echo "Farewell, Commander."
echo ""
