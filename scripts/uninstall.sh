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
echo "  - Cloudflare tunnel, DNS records, and Access apps"
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

# Delete Cloudflare Access apps and DNS records if we have API token
if [ -n "$CF_API_TOKEN" ] && [ -n "$ACCOUNT_ID" ]; then
    echo "Removing Cloudflare Access applications..."

    # Get all access apps and delete ones matching our domains
    APPS=$(curl -s -X GET "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/access/apps" \
        -H "Authorization: Bearer $CF_API_TOKEN" \
        -H "Content-Type: application/json" | jq -r '.result[] | @base64')

    for APP in $APPS; do
        APP_JSON=$(echo "$APP" | base64 -d)
        APP_DOMAIN=$(echo "$APP_JSON" | jq -r '.domain // empty')
        APP_ID=$(echo "$APP_JSON" | jq -r '.id')
        APP_NAME=$(echo "$APP_JSON" | jq -r '.name')

        # Delete if it matches our domains
        if [ "$APP_DOMAIN" = "$DOMAIN" ] || [ "$APP_DOMAIN" = "$SSH_DOMAIN" ]; then
            echo "  Deleting Access app: $APP_NAME"
            curl -s -X DELETE "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/access/apps/$APP_ID" \
                -H "Authorization: Bearer $CF_API_TOKEN" > /dev/null
        fi
    done
    echo -e "${GREEN}[*]${NC} Access applications removed"

    # Delete DNS records
    echo "Removing DNS records..."

    # Get zone ID from domain
    BASE_DOMAIN=$(echo "$DOMAIN" | rev | cut -d'.' -f1,2 | rev)
    ZONE_ID=$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones?name=$BASE_DOMAIN" \
        -H "Authorization: Bearer $CF_API_TOKEN" \
        -H "Content-Type: application/json" | jq -r '.result[0].id')

    if [ -n "$ZONE_ID" ] && [ "$ZONE_ID" != "null" ]; then
        # Find and delete CNAME records for our domains
        for TARGET_DOMAIN in "$DOMAIN" "$SSH_DOMAIN"; do
            if [ -n "$TARGET_DOMAIN" ]; then
                RECORD_ID=$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records?name=$TARGET_DOMAIN&type=CNAME" \
                    -H "Authorization: Bearer $CF_API_TOKEN" \
                    -H "Content-Type: application/json" | jq -r '.result[0].id')

                if [ -n "$RECORD_ID" ] && [ "$RECORD_ID" != "null" ]; then
                    echo "  Deleting DNS record: $TARGET_DOMAIN"
                    curl -s -X DELETE "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records/$RECORD_ID" \
                        -H "Authorization: Bearer $CF_API_TOKEN" > /dev/null
                fi
            fi
        done
        echo -e "${GREEN}[*]${NC} DNS records removed"
    else
        echo -e "${YELLOW}[!]${NC} Could not find zone, DNS records may need manual deletion"
    fi
else
    echo -e "${YELLOW}[!]${NC} No API token saved, skipping Access/DNS cleanup"
    echo "    You may need to manually delete DNS records and Access apps"
fi

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
