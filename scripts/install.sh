#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}"
echo "  _   _                                      _   "
echo " | | | | ___  _ __ ___   ___ _ __   ___  _ __| |_ "
echo " | |_| |/ _ \| '_ \` _ \ / _ \ '_ \ / _ \| '__| __|"
echo " |  _  | (_) | | | | | |  __/ |_) | (_) | |  | |_ "
echo " |_| |_|\___/|_| |_| |_|\___| .__/ \___/|_|   \__|"
echo "                            |_|                   "
echo -e "${NC}"
echo "Homeport Installation Script"
echo "=============================="
echo ""

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    echo -e "${RED}Error: Please don't run as root. The script will use sudo when needed.${NC}"
    exit 1
fi

# Function to check command exists
check_command() {
    if ! command -v "$1" &> /dev/null; then
        echo -e "${RED}Error: $1 is not installed${NC}"
        return 1
    fi
    echo -e "${GREEN}✓${NC} $1 found"
    return 0
}

# Check prerequisites
echo "Checking prerequisites..."
echo ""

MISSING=0

check_command docker || MISSING=1
check_command docker-compose || check_command "docker compose" || MISSING=1
check_command gh || MISSING=1
check_command cloudflared || {
    echo -e "${YELLOW}! cloudflared not found - will install${NC}"
}

echo ""

if [ $MISSING -eq 1 ]; then
    echo -e "${RED}Please install missing prerequisites and run again.${NC}"
    echo ""
    echo "Install Docker: https://docs.docker.com/engine/install/"
    echo "Install gh CLI: https://cli.github.com/"
    exit 1
fi

# Check GitHub auth
echo "Checking GitHub authentication..."
if ! gh auth status &> /dev/null; then
    echo -e "${YELLOW}GitHub CLI not authenticated. Running 'gh auth login'...${NC}"
    gh auth login
fi
echo -e "${GREEN}✓${NC} GitHub authenticated"
echo ""

# Install cloudflared if not present
if ! command -v cloudflared &> /dev/null; then
    echo "Installing cloudflared..."
    if [ -f /etc/debian_version ]; then
        curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb -o /tmp/cloudflared.deb
        sudo dpkg -i /tmp/cloudflared.deb
        rm /tmp/cloudflared.deb
    elif [ -f /etc/redhat-release ]; then
        curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-x86_64.rpm -o /tmp/cloudflared.rpm
        sudo rpm -i /tmp/cloudflared.rpm
        rm /tmp/cloudflared.rpm
    else
        echo -e "${RED}Unsupported OS. Please install cloudflared manually:${NC}"
        echo "https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation/"
        exit 1
    fi
    echo -e "${GREEN}✓${NC} cloudflared installed"
fi
echo ""

# Get domain
echo -e "${BLUE}Domain Configuration${NC}"
echo "Enter the domain you want to use (e.g., dev.example.com)"
echo "This domain must be managed by Cloudflare."
read -p "Domain: " DOMAIN

if [ -z "$DOMAIN" ]; then
    echo -e "${RED}Error: Domain is required${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}Cloudflare Tunnel Setup${NC}"
echo "This will open a browser to authenticate with Cloudflare."
echo ""
read -p "Press Enter to continue..."

# Login to Cloudflare
cloudflared tunnel login

# Create tunnel
TUNNEL_NAME="homeport-$(hostname)"
echo ""
echo "Creating tunnel: $TUNNEL_NAME"
cloudflared tunnel create "$TUNNEL_NAME"

# Get tunnel ID
TUNNEL_ID=$(cloudflared tunnel list | grep "$TUNNEL_NAME" | awk '{print $1}')
echo "Tunnel ID: $TUNNEL_ID"

# Create tunnel config
CLOUDFLARED_CONFIG_DIR="$HOME/.cloudflared"
mkdir -p "$CLOUDFLARED_CONFIG_DIR"

cat > "$CLOUDFLARED_CONFIG_DIR/config.yml" << EOF
tunnel: $TUNNEL_ID
credentials-file: $CLOUDFLARED_CONFIG_DIR/$TUNNEL_ID.json

ingress:
  - hostname: $DOMAIN
    service: http://localhost:80
  - service: http_status:404
EOF

echo -e "${GREEN}✓${NC} Tunnel config created"

# Route DNS
echo ""
echo "Setting up DNS routing..."
cloudflared tunnel route dns "$TUNNEL_NAME" "$DOMAIN"
echo -e "${GREEN}✓${NC} DNS route created"

# Create .env file
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOMEPORT_DIR="$(dirname "$SCRIPT_DIR")"

cat > "$HOMEPORT_DIR/docker/.env" << EOF
DOMAIN=$DOMAIN
EXTERNAL_URL=https://$DOMAIN
CODE_SERVER_AUTH=none
EOF

echo -e "${GREEN}✓${NC} Environment file created"

# Create systemd service for cloudflared
echo ""
echo "Setting up cloudflared as a system service..."
sudo cloudflared service install
echo -e "${GREEN}✓${NC} cloudflared service installed"

# Build and start services
echo ""
echo -e "${BLUE}Building and starting services...${NC}"
cd "$HOMEPORT_DIR/docker"
docker compose build
docker compose up -d

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Installation complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Your Homeport instance is now available at:"
echo -e "  ${BLUE}https://$DOMAIN${NC}"
echo ""
echo "Services:"
echo "  - Homeport UI: https://$DOMAIN/"
echo "  - Code Server: https://$DOMAIN/code/"
echo "  - API: https://$DOMAIN/api/"
echo ""
echo "To view logs:"
echo "  docker compose -f $HOMEPORT_DIR/docker/docker-compose.yml logs -f"
echo ""
echo "To stop services:"
echo "  docker compose -f $HOMEPORT_DIR/docker/docker-compose.yml down"
echo ""
echo -e "${YELLOW}Note: It may take a few minutes for DNS to propagate.${NC}"
