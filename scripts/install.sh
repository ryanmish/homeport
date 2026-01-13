#!/bin/bash
set -e

# Ensure we can read from terminal (important when script is piped)
if [ ! -t 0 ]; then
    exec < /dev/tty
fi

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
  *  .  *  .  *  .  *  .  *  .  *  .  *  .  *  .  *
  _   _                                      _
 | | | | ___  _ __ ___   ___ _ __   ___  _ __| |_
 | |_| |/ _ \| '_ ` _ \ / _ \ '_ \ / _ \| '__| __|
 |  _  | (_) | | | | | |  __/ |_) | (_) | |  | |_
 |_| |_|\___/|_| |_| |_|\___| .__/ \___/|_|   \__|
                            |_|
  *  .  *  .  *  .  *  .  *  .  *  .  *  .  *  .  *
EOF
echo -e "${NC}"
echo ""
echo -e "${BOLD}Remote Development Environment${NC}"
echo ""
echo "Initiating launch sequence..."
echo "This mission will deploy Homeport to your server."
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

echo -e "${BLUE}[T-6] Loading cargo: System dependencies${NC}"
echo "==========================================="

# Update package list
echo "Updating package list..."
sudo apt-get update -qq

# Install basic dependencies
echo "Installing git, curl, wget, jq..."
sudo apt-get install -y -qq git curl wget ca-certificates gnupg lsb-release jq

echo -e "${GREEN}[*]${NC} Cargo loaded"
echo ""

echo -e "${BLUE}[T-5] Fueling engines: Docker${NC}"
echo "==========================================="

if command -v docker &> /dev/null; then
    echo -e "${GREEN}[*]${NC} Docker already installed ($(docker --version | cut -d' ' -f3 | tr -d ','))"
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

    echo -e "${GREEN}[*]${NC} Docker installed"
fi
echo ""

echo -e "${BLUE}[T-4] Establishing comms: GitHub CLI${NC}"
echo "==========================================="

if command -v gh &> /dev/null; then
    echo -e "${GREEN}[*]${NC} GitHub CLI already installed ($(gh --version | head -1 | cut -d' ' -f3))"
else
    echo "Installing GitHub CLI..."

    # Install gh CLI
    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
    sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
    sudo apt-get update -qq
    sudo apt-get install -y -qq gh

    echo -e "${GREEN}[*]${NC} GitHub CLI installed"
fi

# Check GitHub auth
if gh auth status &> /dev/null; then
    GH_USER=$(gh api user -q .login 2>/dev/null || echo "authenticated")
    echo -e "${GREEN}[*]${NC} GitHub CLI authenticated as ${BOLD}$GH_USER${NC}"
else
    echo ""
    echo -e "${YELLOW}GitHub CLI needs to be authenticated.${NC}"
    echo "This allows Homeport to clone your repositories."
    echo ""
    # Auto-skip the "Press Enter to open browser" prompt (it won't open on headless servers anyway)
    echo | gh auth login -p https -h github.com -w
    echo -e "${GREEN}[*]${NC} GitHub CLI authenticated"
fi

# Make gh config readable by Docker container
if [ -d "$HOME/.config/gh" ]; then
    chmod -R a+r "$HOME/.config/gh"
fi
echo ""

echo -e "${BLUE}[T-3] Navigation systems: Cloudflare Tunnel${NC}"
echo "============================================="

if command -v cloudflared &> /dev/null; then
    echo -e "${GREEN}[*]${NC} cloudflared already installed ($(cloudflared --version | head -1 | cut -d' ' -f3))"
else
    echo "Installing cloudflared..."

    # Detect architecture
    ARCH=$(dpkg --print-architecture)
    case $ARCH in
        amd64) CF_ARCH="amd64" ;;
        arm64) CF_ARCH="arm64" ;;
        armhf) CF_ARCH="arm" ;;
        *)
            echo -e "${RED}Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac

    # Download and install cloudflared
    curl -fsSL "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${CF_ARCH}.deb" -o /tmp/cloudflared.deb
    sudo dpkg -i /tmp/cloudflared.deb
    rm /tmp/cloudflared.deb

    echo -e "${GREEN}[*]${NC} cloudflared installed"
fi
echo ""

echo -e "${BLUE}[T-2] Plotting course: Tunnel Configuration${NC}"
echo "============================================="

# Clean up any existing broken cloudflared service first
sudo systemctl stop cloudflared 2>/dev/null || true
sudo cloudflared service uninstall 2>/dev/null || true

# Check if we have valid Cloudflare auth
NEED_AUTH=true
if [ -f "$HOME/.cloudflared/cert.pem" ]; then
    # Test if auth is actually valid by trying to list tunnels
    if cloudflared tunnel list --output json &>/dev/null; then
        echo -e "${GREEN}[*]${NC} Cloudflare already authenticated"
        NEED_AUTH=false

        # Check for existing tunnels
        EXISTING_TUNNEL=$(cloudflared tunnel list --output json 2>/dev/null | jq -r '.[0].name // empty')
        if [ -n "$EXISTING_TUNNEL" ]; then
            # Check if tunnel credentials file exists
            EXISTING_ID=$(cloudflared tunnel list --output json | jq -r '.[0].id // empty')
            if [ -f "$HOME/.cloudflared/${EXISTING_ID}.json" ]; then
                echo -e "${GREEN}[*]${NC} Found existing tunnel: $EXISTING_TUNNEL"
                read -p "Use existing tunnel? (y/n) " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    TUNNEL_NAME=$EXISTING_TUNNEL
                fi
            else
                echo -e "${YELLOW}[!]${NC} Found tunnel '$EXISTING_TUNNEL' but credentials missing"
                echo "Deleting orphaned tunnel..."
                cloudflared tunnel delete "$EXISTING_TUNNEL" 2>/dev/null || true
            fi
        fi
    else
        echo -e "${YELLOW}[!]${NC} Cloudflare auth expired or invalid, re-authenticating..."
        rm -rf "$HOME/.cloudflared"
    fi
fi

if [ "$NEED_AUTH" = true ]; then
    echo ""
    echo "You need to authenticate with Cloudflare."
    echo "A URL will be displayed - open it in your browser to log in."
    echo ""
    cloudflared tunnel login
    echo -e "${GREEN}[*]${NC} Cloudflare authenticated"
fi

# Create tunnel if needed
if [ -z "$TUNNEL_NAME" ]; then
    echo ""
    TUNNEL_NAME="homeport-$(hostname)"
    echo "Creating tunnel: $TUNNEL_NAME"

    # Delete any existing tunnel with same name first (in case of partial cleanup)
    cloudflared tunnel delete "$TUNNEL_NAME" 2>/dev/null || true

    cloudflared tunnel create "$TUNNEL_NAME"
fi

# Get tunnel ID using JSON for reliable parsing
TUNNEL_ID=$(cloudflared tunnel list --output json | jq -r ".[] | select(.name==\"$TUNNEL_NAME\") | .id")

if [ -z "$TUNNEL_ID" ]; then
    echo -e "${RED}Failed to get tunnel ID. Please check Cloudflare configuration.${NC}"
    exit 1
fi
echo -e "${GREEN}[*]${NC} Tunnel ID: $TUNNEL_ID"

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

# Ask about SSH access
echo ""
echo -e "${BOLD}Enable SSH access via Cloudflare Tunnel?${NC}"
echo "This allows you to SSH into your server from anywhere"
echo "without exposing port 22 to the internet."
echo ""
read -p "Enable SSH tunnel? (y/n) " -n 1 -r
ENABLE_SSH=$REPLY
echo ""

SSH_DOMAIN=""
if [[ $ENABLE_SSH =~ ^[Yy]$ ]]; then
    # Extract base domain from the provided domain
    # e.g., dev.example.com -> example.com
    BASE_DOMAIN=$(echo "$DOMAIN" | rev | cut -d'.' -f1,2 | rev)
    DEFAULT_SSH="ssh.$BASE_DOMAIN"

    echo ""
    echo "Enter subdomain for SSH access"
    echo -e "Default: ${BOLD}$DEFAULT_SSH${NC}"
    read -p "SSH Domain [$DEFAULT_SSH]: " SSH_DOMAIN
    SSH_DOMAIN=${SSH_DOMAIN:-$DEFAULT_SSH}
fi

# Create tunnel config
mkdir -p "$HOME/.cloudflared"

if [ -n "$SSH_DOMAIN" ]; then
    # Config with SSH
    cat > "$HOME/.cloudflared/config.yml" << EOF
tunnel: $TUNNEL_ID
credentials-file: $HOME/.cloudflared/$TUNNEL_ID.json

ingress:
  - hostname: $SSH_DOMAIN
    service: ssh://localhost:22
  - hostname: $DOMAIN
    service: http://localhost:80
  - service: http_status:404
EOF
else
    # Config without SSH
    cat > "$HOME/.cloudflared/config.yml" << EOF
tunnel: $TUNNEL_ID
credentials-file: $HOME/.cloudflared/$TUNNEL_ID.json

ingress:
  - hostname: $DOMAIN
    service: http://localhost:80
  - service: http_status:404
EOF
fi

echo -e "${GREEN}[*]${NC} Tunnel config created"

# Set up DNS
echo ""
echo "Setting up DNS routing..."

# Add DNS route - if it fails, show the error
if cloudflared tunnel route dns "$TUNNEL_NAME" "$DOMAIN" 2>&1; then
    echo -e "${GREEN}[*]${NC} DNS configured for $DOMAIN"
else
    echo -e "${YELLOW}[!]${NC} DNS route for $DOMAIN may need manual setup in Cloudflare dashboard"
    echo "    Add a CNAME record: $DOMAIN -> $TUNNEL_ID.cfargotunnel.com"
fi

if [ -n "$SSH_DOMAIN" ]; then
    if cloudflared tunnel route dns "$TUNNEL_NAME" "$SSH_DOMAIN" 2>&1; then
        echo -e "${GREEN}[*]${NC} DNS configured for $SSH_DOMAIN (SSH)"
    else
        echo -e "${YELLOW}[!]${NC} DNS route for $SSH_DOMAIN may need manual setup in Cloudflare dashboard"
        echo "    Add a CNAME record: $SSH_DOMAIN -> $TUNNEL_ID.cfargotunnel.com"
    fi
fi

# Set up Cloudflare Access for authentication
echo ""
echo -e "${BLUE}[T-1.5] Security shields: Cloudflare Access${NC}"
echo "============================================="
echo ""
echo -e "${BOLD}Cloudflare Access protects your tunnel with authentication.${NC}"
echo ""
echo "To set this up, you need a Cloudflare API token."
echo "Create one at: https://dash.cloudflare.com/profile/api-tokens"
echo ""
echo "Required permissions:"
echo "  - Account > Access: Apps and Policies > Edit"
echo "  - Account > Account Settings > Read"
echo ""
read -p "Enter your Cloudflare API token (or press Enter to skip): " CF_API_TOKEN

if [ -n "$CF_API_TOKEN" ]; then
    # Get user's email for access policy
    echo ""
    read -p "Enter your email (for access policy): " USER_EMAIL

    if [ -z "$USER_EMAIL" ]; then
        echo -e "${YELLOW}[!]${NC} Email required for access policy, skipping Access setup"
    else
        # Get account ID
        echo "Configuring Cloudflare Access..."
        ACCOUNT_ID=$(curl -s -X GET "https://api.cloudflare.com/client/v4/accounts" \
            -H "Authorization: Bearer $CF_API_TOKEN" \
            -H "Content-Type: application/json" | jq -r '.result[0].id')

        if [ -z "$ACCOUNT_ID" ] || [ "$ACCOUNT_ID" = "null" ]; then
            echo -e "${YELLOW}[!]${NC} Could not get account ID. Check your API token permissions."
        else
            # Create Access application for main domain
            APP_RESPONSE=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/access/apps" \
                -H "Authorization: Bearer $CF_API_TOKEN" \
                -H "Content-Type: application/json" \
                --data "{
                    \"name\": \"Homeport - $DOMAIN\",
                    \"domain\": \"$DOMAIN\",
                    \"type\": \"self_hosted\",
                    \"session_duration\": \"24h\",
                    \"auto_redirect_to_identity\": true
                }")

            APP_ID=$(echo "$APP_RESPONSE" | jq -r '.result.id')

            if [ -n "$APP_ID" ] && [ "$APP_ID" != "null" ]; then
                # Create access policy - allow only this email
                curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/access/apps/$APP_ID/policies" \
                    -H "Authorization: Bearer $CF_API_TOKEN" \
                    -H "Content-Type: application/json" \
                    --data "{
                        \"name\": \"Allow owner\",
                        \"decision\": \"allow\",
                        \"include\": [{\"email\": {\"email\": \"$USER_EMAIL\"}}],
                        \"precedence\": 1
                    }" > /dev/null

                echo -e "${GREEN}[*]${NC} Access configured for $DOMAIN (allowed: $USER_EMAIL)"

                # Also protect SSH domain if set
                if [ -n "$SSH_DOMAIN" ]; then
                    SSH_APP_RESPONSE=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/access/apps" \
                        -H "Authorization: Bearer $CF_API_TOKEN" \
                        -H "Content-Type: application/json" \
                        --data "{
                            \"name\": \"Homeport SSH - $SSH_DOMAIN\",
                            \"domain\": \"$SSH_DOMAIN\",
                            \"type\": \"ssh\",
                            \"session_duration\": \"24h\",
                            \"auto_redirect_to_identity\": true
                        }")

                    SSH_APP_ID=$(echo "$SSH_APP_RESPONSE" | jq -r '.result.id')

                    if [ -n "$SSH_APP_ID" ] && [ "$SSH_APP_ID" != "null" ]; then
                        curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/access/apps/$SSH_APP_ID/policies" \
                            -H "Authorization: Bearer $CF_API_TOKEN" \
                            -H "Content-Type: application/json" \
                            --data "{
                                \"name\": \"Allow owner\",
                                \"decision\": \"allow\",
                                \"include\": [{\"email\": {\"email\": \"$USER_EMAIL\"}}],
                                \"precedence\": 1
                            }" > /dev/null

                        echo -e "${GREEN}[*]${NC} Access configured for $SSH_DOMAIN (allowed: $USER_EMAIL)"
                    fi
                fi
            else
                ERROR_MSG=$(echo "$APP_RESPONSE" | jq -r '.errors[0].message // "Unknown error"')
                echo -e "${YELLOW}[!]${NC} Failed to create Access app: $ERROR_MSG"
            fi
        fi
    fi
else
    echo ""
    echo -e "${RED}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║  WARNING: YOUR TUNNEL IS PUBLICLY ACCESSIBLE WITHOUT AUTH!  ║${NC}"
    echo -e "${RED}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Set up Cloudflare Access manually:"
    echo "  1. Go to https://one.dash.cloudflare.com"
    echo "  2. Access > Applications > Add application"
    echo "  3. Protect: $DOMAIN"
    if [ -n "$SSH_DOMAIN" ]; then
        echo "  4. Also protect: $SSH_DOMAIN"
    fi
    echo ""
    read -p "Press Enter to continue (at your own risk)..."
fi
echo ""

echo -e "${BLUE}[T-1] Ignition: Starting Homeport${NC}"
echo "==========================================="

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOMEPORT_DIR="$(dirname "$SCRIPT_DIR")"

# Generate cookie secret for persistent sessions
COOKIE_SECRET=$(openssl rand -hex 32)

# Create .env file
cat > "$HOMEPORT_DIR/docker/.env" << EOF
DOMAIN=$DOMAIN
EXTERNAL_URL=https://$DOMAIN
CODE_SERVER_AUTH=none
COOKIE_SECRET=$COOKIE_SECRET
EOF

echo "Building Docker images (this may take a few minutes)..."
cd "$HOMEPORT_DIR/docker"

# Check if user can access Docker directly
if docker info &> /dev/null; then
    # User has docker access
    if docker compose version &> /dev/null; then
        COMPOSE="docker compose"
    else
        COMPOSE="docker-compose"
    fi
    $COMPOSE build --quiet
    echo -e "${GREEN}[*]${NC} Docker images built"
    echo "Starting services..."
    $COMPOSE up -d
else
    # User needs docker group - use sg to run with docker group
    echo "Activating docker group..."
    COMPOSE="docker compose"
    sg docker -c "$COMPOSE build --quiet"
    echo -e "${GREEN}[*]${NC} Docker images built"
    echo "Starting services..."
    sg docker -c "$COMPOSE up -d"
fi
echo -e "${GREEN}[*]${NC} Services started"

# Install systemd service for auto-start on boot
echo "Setting up auto-start on boot..."

# Create systemd service for homeport
sudo tee /etc/systemd/system/homeport.service > /dev/null << EOF
[Unit]
Description=Homeport Development Environment
Documentation=https://github.com/ryanmish/homeport
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
User=$USER
WorkingDirectory=$HOMEPORT_DIR/docker
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
ExecReload=/usr/bin/docker compose restart
TimeoutStartSec=300

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable homeport.service
echo -e "${GREEN}[*]${NC} Homeport will start on boot"

# Start cloudflared tunnel as a systemd service
echo "Starting Cloudflare Tunnel..."
sudo cloudflared service install 2>/dev/null || true
sudo systemctl enable cloudflared 2>/dev/null || true
if ! sudo systemctl start cloudflared 2>/dev/null; then
    # Fallback: run in background with no output
    nohup cloudflared tunnel run "$TUNNEL_NAME" > /dev/null 2>&1 &
    disown
fi
echo -e "${GREEN}[*]${NC} Tunnel running (will auto-start on boot)"

# Install CLI
echo "Installing homeport CLI..."
sudo cp "$HOMEPORT_DIR/scripts/homeport-cli.sh" /usr/local/bin/homeport
sudo chmod +x /usr/local/bin/homeport
echo -e "${GREEN}[*]${NC} CLI installed (run 'homeport' from anywhere)"

echo ""
echo -e "${GREEN}==========================================="
echo -e "       LAUNCH SUCCESSFUL"
echo -e "===========================================${NC}"
echo ""
echo -e "Your Homeport is now available at:"
echo ""
echo -e "  ${BOLD}https://$DOMAIN${NC}              Dashboard"
echo -e "  ${BOLD}https://$DOMAIN/code/${NC}        VS Code"
echo ""
echo -e "Docking bays for dev servers:"
echo -e "  ${BOLD}https://$DOMAIN/3000/${NC}        (example)"
echo ""
if [ -n "$SSH_DOMAIN" ]; then
echo -e "Remote command access:"
echo -e "  ${BOLD}https://$SSH_DOMAIN${NC}          SSH (browser)"
echo ""
echo "  Or from terminal:"
echo "  cloudflared access ssh --hostname $SSH_DOMAIN"
echo ""
fi
echo -e "${YELLOW}Note: DNS may take a few minutes to propagate.${NC}"
echo ""
echo "Mission control commands:"
echo "  View logs:     cd $HOMEPORT_DIR/docker && $COMPOSE logs -f"
echo "  Power down:    cd $HOMEPORT_DIR/docker && $COMPOSE down"
echo "  Reboot:        cd $HOMEPORT_DIR/docker && $COMPOSE restart"
echo ""
echo "Safe travels, Commander."
echo ""
