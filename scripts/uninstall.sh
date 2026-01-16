#!/bin/bash

# Homeport Uninstall Script
# Thoroughly removes all Homeport components from the system

main() {
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

    # Get homeport directory
    HOMEPORT_DIR="$HOME/homeport"

    # Change to a safe directory
    cd /tmp 2>/dev/null || cd ~

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
        echo "  - Docker containers, images (including ghcr.io), networks, and volumes"
        echo "  - Systemd services (homeport + cloudflared)"
        echo "  - Cloudflare tunnel"
        echo "  - CLI command (/usr/local/bin/homeport)"
        echo "  - All config (~/.cloudflared, ~/.homeport, .env)"
        echo "  - Upgrade status files and logs (in Docker volumes)"
        echo "  - Source code directory ($HOMEPORT_DIR)"
        echo ""
        echo -e "${YELLOW}What will NOT be removed:${NC}"
        echo "  - Docker, GitHub CLI, cloudflared packages"
        echo "  - Go runtime"
        if [ -n "$SAVED_DOMAIN" ]; then
            echo ""
            echo -e "${YELLOW}NOTE:${NC} DNS record for ${BOLD}$SAVED_DOMAIN${NC} may need manual deletion"
        fi
        echo ""
        read -p "Delete Docker volumes (repos/settings will be lost)? (y/n) " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            REMOVE_VOLUMES=true
        fi
        read -p "Confirm decommission? (y/n) " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "Decommission aborted."
            exit 0
        fi
    fi

    echo ""
    echo -e "${BLUE}[Step 1/7]${NC} Stopping all processes"
    echo "============================================="

    echo "Stopping Docker containers..."
    if [ -d "$HOMEPORT_DIR/docker" ]; then
        cd "$HOMEPORT_DIR/docker" 2>/dev/null || true
        docker compose down >/dev/null 2>&1 || true
        cd /tmp 2>/dev/null || cd ~
    fi
    docker stop homeportd code-server caddy code-server-init homeportd-init homeport-upgrader homeport-rollback 2>/dev/null || true

    echo "Stopping systemd services..."
    sudo systemctl stop homeport.service >/dev/null 2>&1 || true
    sudo systemctl disable homeport.service >/dev/null 2>&1 || true
    sudo systemctl stop cloudflared >/dev/null 2>&1 || true
    sudo systemctl disable cloudflared >/dev/null 2>&1 || true
    sudo cloudflared service uninstall >/dev/null 2>&1 || true

    # Stop and remove user-level cloudflared service
    if [ -f "$HOME/.config/systemd/user/cloudflared.service" ]; then
        echo "Stopping user cloudflared service..."
        systemctl --user stop cloudflared 2>/dev/null || true
        systemctl --user disable cloudflared 2>/dev/null || true
        rm -f "$HOME/.config/systemd/user/cloudflared.service"
        systemctl --user daemon-reload 2>/dev/null || true
    fi

    echo "Killing cloudflared processes..."
    sudo pkill -9 -x cloudflared >/dev/null 2>&1 || true
    pkill -9 -x cloudflared >/dev/null 2>&1 || true
    sleep 1

    echo -e "${GREEN}[*]${NC} Processes stopped"

    echo ""
    echo -e "${BLUE}[Step 2/7]${NC} Deleting Cloudflare tunnel"
    echo "============================================="

    if command -v cloudflared &> /dev/null && [ -f "$HOME/.cloudflared/cert.pem" ]; then
        TUNNELS=$(cloudflared tunnel list --output json 2>/dev/null | jq -r '.[].name // empty' 2>/dev/null || true)
        if [ -n "$TUNNELS" ]; then
            for TUNNEL in $TUNNELS; do
                if [ -n "$TUNNEL" ]; then
                    echo "Deleting tunnel: $TUNNEL"
                    cloudflared tunnel cleanup "$TUNNEL" >/dev/null 2>&1 || true
                    cloudflared tunnel delete -f "$TUNNEL" >/dev/null 2>&1 || true
                fi
            done
            echo -e "${GREEN}[*]${NC} Tunnels deleted"
        else
            echo -e "${DIM}No tunnels found${NC}"
        fi
    else
        echo -e "${YELLOW}[!]${NC} No cloudflared auth - manual tunnel deletion may be needed"
    fi

    echo ""
    echo -e "${BLUE}[Step 3/7]${NC} Removing Docker resources"
    echo "============================================="

    if docker info >/dev/null 2>&1; then
        echo "Removing containers..."
        docker rm -f homeportd code-server caddy code-server-init homeportd-init homeport-upgrader homeport-rollback 2>/dev/null || true

        echo "Removing networks..."
        docker network rm docker_default homeport_default >/dev/null 2>&1 || true

        if [ "$REMOVE_VOLUMES" = true ]; then
            echo "Removing volumes (includes repos, settings, upgrade logs)..."
            docker volume rm docker_homeport-data docker_repos docker_code-server-data docker_code-server-config docker_caddy-data docker_caddy-config docker_claude-config docker_claude-config-homeport >/dev/null 2>&1 || true
            docker volume rm homeport-data repos code-server-data code-server-config caddy-data caddy-config claude-config claude-config-homeport >/dev/null 2>&1 || true
            echo -e "${GREEN}[*]${NC} Volumes removed"
        else
            echo -e "${YELLOW}[!]${NC} Volumes preserved (includes upgrade status/logs)"
        fi

        echo "Removing images..."
        # Remove locally built images
        docker rmi docker-homeportd docker-code-server >/dev/null 2>&1 || true
        # Remove ghcr.io pre-built images (all versions)
        docker images "ghcr.io/ryanmish/homeport" -q 2>/dev/null | xargs -r docker rmi >/dev/null 2>&1 || true
        # Remove base images
        docker rmi codercom/code-server:latest caddy:2-alpine alpine:latest >/dev/null 2>&1 || true
        # Remove any other homeport-related images
        docker images --filter "reference=*homeport*" -q 2>/dev/null | xargs -r docker rmi >/dev/null 2>&1 || true

        echo -e "${GREEN}[*]${NC} Docker resources cleaned"
    else
        echo -e "${YELLOW}[!]${NC} Docker not accessible, skipping"
    fi

    echo ""
    echo -e "${BLUE}[Step 4/7]${NC} Removing configuration files"
    echo "============================================="

    if [ -f "$HOMEPORT_DIR/docker/.env" ]; then
        echo "Removing .env file..."
        rm -f "$HOMEPORT_DIR/docker/.env"
    fi

    echo "Removing ~/.cloudflared/"
    rm -rf "$HOME/.cloudflared"

    echo "Removing ~/.homeport/"
    rm -rf "$HOME/.homeport"

    echo "Removing /etc/cloudflared/"
    sudo rm -rf /etc/cloudflared 2>/dev/null || true

    echo "Removing systemd service file..."
    sudo rm -f /etc/systemd/system/homeport.service
    sudo systemctl daemon-reload >/dev/null 2>&1 || true

    echo -e "${GREEN}[*]${NC} Configuration removed"

    echo ""
    echo -e "${BLUE}[Step 5/7]${NC} Removing CLI and temp files"
    echo "============================================="

    echo "Removing /usr/local/bin/homeport..."
    sudo rm -f /usr/local/bin/homeport

    echo "Removing temp files..."
    rm -f /tmp/homeportd-setup /tmp/cloudflared.deb 2>/dev/null || true

    echo -e "${GREEN}[*]${NC} CLI removed"

    echo ""
    echo -e "${BLUE}[Step 5.5/7]${NC} Removing source code directory"
    echo "============================================="

    if [ -d "$HOMEPORT_DIR" ]; then
        echo "Removing $HOMEPORT_DIR..."
        rm -rf "$HOMEPORT_DIR"
        echo -e "${GREEN}[*]${NC} Source directory removed"
    else
        echo -e "${DIM}Directory already removed${NC}"
    fi

    echo ""
    echo -e "${BLUE}[Step 6/7]${NC} Final verification"
    echo "============================================="

    ISSUES=0

    CF_PROCS=$(pgrep -x cloudflared 2>/dev/null | wc -l)
    if [ "$CF_PROCS" -gt 0 ]; then
        echo -e "${RED}[!]${NC} cloudflared still running"
        ISSUES=1
    fi

    CONTAINERS=$(docker ps -a --format '{{.Names}}' 2>/dev/null | grep -E "^(homeportd|code-server|caddy|code-server-init|homeportd-init|homeport-upgrader|homeport-rollback)$" || true)
    if [ -n "$CONTAINERS" ]; then
        echo -e "${RED}[!]${NC} Containers still exist: $CONTAINERS"
        ISSUES=1
    fi

    [ -d "$HOME/.cloudflared" ] && echo -e "${RED}[!]${NC} ~/.cloudflared still exists" && ISSUES=1
    [ -d "$HOME/.homeport" ] && echo -e "${RED}[!]${NC} ~/.homeport still exists" && ISSUES=1
    [ -f "/usr/local/bin/homeport" ] && echo -e "${RED}[!]${NC} CLI still exists" && ISSUES=1
    [ -d "$HOMEPORT_DIR" ] && echo -e "${RED}[!]${NC} $HOMEPORT_DIR still exists" && ISSUES=1

    if [ "$ISSUES" -eq 0 ]; then
        echo -e "${GREEN}[*]${NC} All components removed successfully"
    fi

    echo ""
    echo -e "${BLUE}[Step 7/7]${NC} Summary"
    echo "============================================="

    echo ""
    if [ "$ISSUES" -eq 0 ]; then
        echo -e "${GREEN}DECOMMISSION COMPLETE${NC}"
    else
        echo -e "${YELLOW}DECOMMISSION COMPLETE (with warnings)${NC}"
    fi

    echo ""
    echo "Remaining cleanup (optional - these may be used by other tools):"
    echo "  sudo apt remove cloudflared"
    echo "  sudo rm -rf /usr/local/go"

    if [ -n "$SAVED_DOMAIN" ]; then
        echo ""
        echo -e "${YELLOW}IMPORTANT:${NC} Delete DNS record for ${BOLD}$SAVED_DOMAIN${NC}"
        echo "  1. Go to https://dash.cloudflare.com"
        echo "  2. Select your domain -> DNS -> Records"
        echo "  3. Delete the CNAME record for $SAVED_DOMAIN"
    fi

    echo ""
    echo "Farewell, Commander."
    echo ""
}

main "$@"
