#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

# Print banner
clear
echo -e "${BLUE}"
cat << 'EOF'
  _   _                                      _
 | | | | ___  _ __ ___   ___ _ __   ___  _ __| |_
 | |_| |/ _ \| '_ ` _ \ / _ \ '_ \ / _ \| '__| __|
 |  _  | (_) | | | | | |  __/ |_) | (_) | |  | |_
 |_| |_|\___/|_| |_| |_|\___| .__/ \___/|_|   \__|
                            |_|
EOF
echo -e "${NC}"
echo -e "${BOLD}Self-hosted remote development environment${NC}"
echo ""
echo "This script will set up Homeport on your Ubuntu server."
echo "It will install dependencies, configure Cloudflare Tunnel,"
echo "and start all services."
echo ""

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    echo -e "${RED}Please run without sudo. The script will ask for sudo when needed.${NC}"
    exit 1
fi

# Detect OS
if [ ! -f /etc/os-release ]; then
    echo -e "${RED}This script requires Ubuntu/Debian Linux.${NC}"
    exit 1
fi

source /etc/os-release
if [[ "$ID" != "ubuntu" && "$ID" != "debian" ]]; then
    echo -e "${YELLOW}Warning: This script is designed for Ubuntu/Debian.${NC}"
    echo -e "Detected: $ID $VERSION_ID"
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo -e "${BLUE}Step 1/6: Installing system dependencies${NC}"
echo "=========================================="

# Update package list
echo "Updating package list..."
sudo apt-get update -qq

# Install basic dependencies
echo "Installing git, curl, wget..."
sudo apt-get install -y -qq git curl wget ca-certificates gnupg lsb-release

echo -e "${GREEN}✓${NC} System dependencies installed"
echo ""

echo -e "${BLUE}Step 2/6: Installing Docker${NC}"
echo "=========================================="

if command -v docker &> /dev/null; then
    echo -e "${GREEN}✓${NC} Docker already installed ($(docker --version | cut -d' ' -f3 | tr -d ','))"
else
    echo "Installing Docker..."

    # Add Docker's official GPG key
    sudo install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    sudo chmod a+r /etc/apt/keyrings/docker.gpg

    # Add the repository
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
      sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

    # Install Docker
    sudo apt-get update -qq
    sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

    # Add current user to docker group
    sudo usermod -aG docker $USER

    echo -e "${GREEN}✓${NC} Docker installed"
    echo -e "${YELLOW}Note: You may need to log out and back in for docker group to take effect.${NC}"
fi
echo ""

echo -e "${BLUE}Step 3/6: Installing GitHub CLI${NC}"
echo "=========================================="

if command -v gh &> /dev/null; then
    echo -e "${GREEN}✓${NC} GitHub CLI already installed ($(gh --version | head -1 | cut -d' ' -f3))"
else
    echo "Installing GitHub CLI..."

    # Install gh CLI
    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
    sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
    sudo apt-get update -qq
    sudo apt-get install -y -qq gh

    echo -e "${GREEN}✓${NC} GitHub CLI installed"
fi

# Check GitHub auth
if gh auth status &> /dev/null; then
    GH_USER=$(gh api user -q .login 2>/dev/null || echo "authenticated")
    echo -e "${GREEN}✓${NC} GitHub CLI authenticated as ${BOLD}$GH_USER${NC}"
else
    echo ""
    echo -e "${YELLOW}GitHub CLI needs to be authenticated.${NC}"
    echo "This allows Homeport to clone your repositories."
    echo ""
    gh auth login
    echo -e "${GREEN}✓${NC} GitHub CLI authenticated"
fi
echo ""

echo -e "${BLUE}Step 4/6: Installing Cloudflare Tunnel${NC}"
echo "=========================================="

if command -v cloudflared &> /dev/null; then
    echo -e "${GREEN}✓${NC} cloudflared already installed ($(cloudflared --version | head -1 | cut -d' ' -f3))"
else
    echo "Installing cloudflared..."

    # Download and install cloudflared
    curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb -o /tmp/cloudflared.deb
    sudo dpkg -i /tmp/cloudflared.deb
    rm /tmp/cloudflared.deb

    echo -e "${GREEN}✓${NC} cloudflared installed"
fi
echo ""

echo -e "${BLUE}Step 5/6: Cloudflare Tunnel Setup${NC}"
echo "=========================================="

# Check if tunnel already exists
if [ -f "$HOME/.cloudflared/cert.pem" ]; then
    echo -e "${GREEN}✓${NC} Cloudflare already authenticated"

    # Check for existing tunnels
    EXISTING_TUNNEL=$(cloudflared tunnel list 2>/dev/null | grep -v "ID" | head -1 | awk '{print $2}')
    if [ -n "$EXISTING_TUNNEL" ]; then
        echo -e "${GREEN}✓${NC} Found existing tunnel: $EXISTING_TUNNEL"
        read -p "Use existing tunnel? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            TUNNEL_NAME=$EXISTING_TUNNEL
        fi
    fi
else
    echo ""
    echo "You need to authenticate with Cloudflare."
    echo "This will open a browser window to log in."
    echo ""
    read -p "Press Enter to open Cloudflare login..."
    cloudflared tunnel login
    echo -e "${GREEN}✓${NC} Cloudflare authenticated"
fi

# Create tunnel if needed
if [ -z "$TUNNEL_NAME" ]; then
    echo ""
    TUNNEL_NAME="homeport-$(hostname)"
    echo "Creating tunnel: $TUNNEL_NAME"
    cloudflared tunnel create "$TUNNEL_NAME" 2>/dev/null || true
fi

TUNNEL_ID=$(cloudflared tunnel list | grep "$TUNNEL_NAME" | awk '{print $1}')
echo -e "${GREEN}✓${NC} Tunnel ID: $TUNNEL_ID"

# Get domain
echo ""
echo -e "${BOLD}Enter your domain${NC}"
echo "This domain must be managed by Cloudflare."
echo "Example: dev.yourdomain.com"
echo ""
read -p "Domain: " DOMAIN

if [ -z "$DOMAIN" ]; then
    echo -e "${RED}Domain is required${NC}"
    exit 1
fi

# Create tunnel config
mkdir -p "$HOME/.cloudflared"
cat > "$HOME/.cloudflared/config.yml" << EOF
tunnel: $TUNNEL_ID
credentials-file: $HOME/.cloudflared/$TUNNEL_ID.json

ingress:
  - hostname: $DOMAIN
    service: http://localhost:80
  - service: http_status:404
EOF

echo -e "${GREEN}✓${NC} Tunnel config created"

# Set up DNS
echo ""
echo "Setting up DNS routing..."
cloudflared tunnel route dns "$TUNNEL_NAME" "$DOMAIN" 2>/dev/null || echo "DNS route may already exist"
echo -e "${GREEN}✓${NC} DNS configured for $DOMAIN"
echo ""

echo -e "${BLUE}Step 6/6: Starting Homeport${NC}"
echo "=========================================="

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOMEPORT_DIR="$(dirname "$SCRIPT_DIR")"

# Create .env file
cat > "$HOMEPORT_DIR/docker/.env" << EOF
DOMAIN=$DOMAIN
EXTERNAL_URL=https://$DOMAIN
CODE_SERVER_AUTH=none
EOF

echo "Building Docker images (this may take a few minutes)..."
cd "$HOMEPORT_DIR/docker"

# Use docker compose (v2) or docker-compose (v1)
if docker compose version &> /dev/null; then
    COMPOSE="docker compose"
else
    COMPOSE="docker-compose"
fi

$COMPOSE build --quiet
echo -e "${GREEN}✓${NC} Docker images built"

echo "Starting services..."
$COMPOSE up -d
echo -e "${GREEN}✓${NC} Services started"

# Start cloudflared tunnel
echo "Starting Cloudflare Tunnel..."
sudo cloudflared service install 2>/dev/null || true
sudo systemctl start cloudflared 2>/dev/null || cloudflared tunnel run "$TUNNEL_NAME" &
echo -e "${GREEN}✓${NC} Tunnel running"

echo ""
echo -e "${GREEN}=========================================="
echo -e "        Installation Complete!"
echo -e "==========================================${NC}"
echo ""
echo -e "Your Homeport is now available at:"
echo ""
echo -e "  ${BOLD}https://$DOMAIN${NC}              Dashboard"
echo -e "  ${BOLD}https://$DOMAIN/code/${NC}        VS Code"
echo ""
echo -e "Detected ports will appear at:"
echo -e "  ${BOLD}https://$DOMAIN/3000/${NC}        (example)"
echo ""
echo -e "${YELLOW}Note: DNS may take a few minutes to propagate.${NC}"
echo ""
echo "Useful commands:"
echo "  View logs:     cd $HOMEPORT_DIR/docker && $COMPOSE logs -f"
echo "  Stop:          cd $HOMEPORT_DIR/docker && $COMPOSE down"
echo "  Restart:       cd $HOMEPORT_DIR/docker && $COMPOSE restart"
echo ""
