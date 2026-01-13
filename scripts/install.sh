#!/bin/bash
# Homeport Install Script
# Usage: curl -fsSL https://raw.githubusercontent.com/ryanmish/homeport/main/scripts/install.sh | bash

main() {
    # Note: We don't use set -e because we need graceful error handling

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

        # Determine distro for Docker repo (ubuntu or debian)
        DOCKER_DISTRO="$ID"
        if [[ "$ID" != "ubuntu" && "$ID" != "debian" ]]; then
            DOCKER_DISTRO="ubuntu"  # Fallback for derivatives
        fi

        # Add Docker's official GPG key
        sudo install -m 0755 -d /etc/apt/keyrings
        curl -fsSL "https://download.docker.com/linux/${DOCKER_DISTRO}/gpg" | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        sudo chmod a+r /etc/apt/keyrings/docker.gpg

        # Add the repository
        echo \
          "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/${DOCKER_DISTRO} \
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
        gh auth login -p https -h github.com -w
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
        if cloudflared tunnel list --output json &>/dev/null; then
            echo -e "${GREEN}[*]${NC} Cloudflare already authenticated"
            NEED_AUTH=false

            # Auto-use existing homeport tunnel if it exists with valid credentials
            EXISTING_TUNNEL=$(cloudflared tunnel list --output json 2>/dev/null | jq -r '.[] | select(.name | startswith("homeport-")) | .name' | head -1)
            if [ -n "$EXISTING_TUNNEL" ]; then
                EXISTING_ID=$(cloudflared tunnel list --output json | jq -r ".[] | select(.name==\"$EXISTING_TUNNEL\") | .id")
                if [ -f "$HOME/.cloudflared/${EXISTING_ID}.json" ]; then
                    echo -e "${GREEN}[*]${NC} Using existing tunnel: $EXISTING_TUNNEL"
                    TUNNEL_NAME=$EXISTING_TUNNEL
                else
                    cloudflared tunnel delete "$EXISTING_TUNNEL" 2>/dev/null || true
                fi
            fi
        else
            rm -rf "$HOME/.cloudflared"
        fi
    fi

    if [ "$NEED_AUTH" = true ]; then
        echo ""
        echo "Authenticate with Cloudflare (opens browser)..."
        cloudflared tunnel login
        echo -e "${GREEN}[*]${NC} Cloudflare authenticated"
    fi

    # Create tunnel if needed
    if [ -z "$TUNNEL_NAME" ]; then
        TUNNEL_NAME="homeport-$(hostname)"
        cloudflared tunnel delete "$TUNNEL_NAME" 2>/dev/null || true
        echo "Creating tunnel: $TUNNEL_NAME"
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
    echo -e "${BOLD}Enter your domain${NC} (must be managed by Cloudflare)"
    echo "Example: dev.yourdomain.com"
    echo ""
    read -p "Domain: " DOMAIN

    if [ -z "$DOMAIN" ]; then
        echo -e "${RED}Domain is required${NC}"
        exit 1
    fi

    # Create tunnel config (HTTP only - SSH removed for security without CF Access)
    mkdir -p "$HOME/.cloudflared"

    cat > "$HOME/.cloudflared/config.yml" << EOF
tunnel: $TUNNEL_ID
credentials-file: $HOME/.cloudflared/$TUNNEL_ID.json

ingress:
  - hostname: $DOMAIN
    service: http://localhost:80
  - service: http_status:404
EOF

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

    # Set up password authentication
    echo ""
    echo -e "${BLUE}[T-1.5] Security shields: Password Authentication${NC}"
    echo "=================================================="
    echo ""
    echo -e "${BOLD}Set a password to protect your Homeport installation.${NC}"
    echo ""
    echo "Options:"
    echo "  1) Enter your own password"
    echo "  2) Generate a secure password (recommended)"
    echo ""
    read -p "Choose [1/2]: " PASSWORD_CHOICE

    # Clone or update homeport repo
    HOMEPORT_DIR="$HOME/homeport"
    if [ -d "$HOMEPORT_DIR/.git" ]; then
        echo "Updating Homeport..."
        cd "$HOMEPORT_DIR"
        git pull --quiet
    else
        echo "Downloading Homeport..."
        rm -rf "$HOMEPORT_DIR"
        git clone --quiet https://github.com/ryanmish/homeport.git "$HOMEPORT_DIR"
    fi
    cd "$HOMEPORT_DIR"

    # Required Go version
    GO_REQUIRED_MAJOR=1
    GO_REQUIRED_MINOR=22
    GO_INSTALL_VERSION="1.22.10"

    # Function to check if Go version is sufficient
    check_go_version() {
        local go_path="$1"
        local version_output=$("$go_path" version 2>/dev/null)
        local version=$(echo "$version_output" | sed -n 's/.*go\([0-9]*\)\.\([0-9]*\).*/\1.\2/p')

        if [ -z "$version" ]; then
            return 1
        fi

        local major=$(echo "$version" | cut -d. -f1)
        local minor=$(echo "$version" | cut -d. -f2)

        if [ "$major" -gt "$GO_REQUIRED_MAJOR" ]; then
            return 0
        elif [ "$major" -eq "$GO_REQUIRED_MAJOR" ] && [ "$minor" -ge "$GO_REQUIRED_MINOR" ]; then
            return 0
        fi
        return 1
    }

    # Check if Go is available with sufficient version
    GO_CMD=""
    for go_path in "go" "/usr/local/go/bin/go"; do
        if command -v "$go_path" &>/dev/null 2>&1 || [ -x "$go_path" ]; then
            if check_go_version "$go_path"; then
                GO_CMD="$go_path"
                break
            fi
        fi
    done

    if [ -z "$GO_CMD" ]; then
        # Need to install or upgrade Go
        if [ -d "/usr/local/go" ]; then
            echo "Upgrading Go (need >= $GO_REQUIRED_MAJOR.$GO_REQUIRED_MINOR)..."
            sudo rm -rf /usr/local/go
        else
            echo "Installing Go..."
        fi

        ARCH=$(dpkg --print-architecture)
        if ! curl -fsSL "https://go.dev/dl/go${GO_INSTALL_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz; then
            echo -e "${RED}Failed to download Go${NC}"
            exit 1
        fi
        if ! sudo tar -C /usr/local -xzf /tmp/go.tar.gz; then
            echo -e "${RED}Failed to install Go${NC}"
            exit 1
        fi
        rm -f /tmp/go.tar.gz
        GO_CMD="/usr/local/go/bin/go"
        echo -e "${GREEN}[*]${NC} Go $GO_INSTALL_VERSION installed"
    fi

    # Build the password hashing tool
    echo "Building security module..."
    # GOTOOLCHAIN=local prevents Go from trying to download a different toolchain
    if ! GOTOOLCHAIN=local $GO_CMD build -o /tmp/homeportd-setup ./cmd/homeportd; then
        echo -e "${RED}Failed to build homeportd. Check Go installation.${NC}"
        exit 1
    fi

    if [ "$PASSWORD_CHOICE" = "2" ]; then
        # Generate password
        GENERATED=$(/tmp/homeportd-setup generate-password)
        ADMIN_PASSWORD=$(echo "$GENERATED" | head -1)
        ADMIN_PASSWORD_HASH=$(echo "$GENERATED" | tail -1)

        echo ""
        echo -e "${GREEN}Generated password:${NC}"
        echo ""
        echo -e "  ${BOLD}$ADMIN_PASSWORD${NC}"
        echo ""
        echo -e "${YELLOW}Save this password now! It won't be shown again.${NC}"
        echo ""
        read -p "Press Enter when you've saved it..."
    else
        # Manual password entry
        while true; do
            echo ""
            read -s -p "Enter password (min 8 characters): " ADMIN_PASSWORD
            echo
            if [ ${#ADMIN_PASSWORD} -lt 8 ]; then
                echo -e "${RED}Password must be at least 8 characters${NC}"
                continue
            fi
            read -s -p "Confirm password: " ADMIN_PASSWORD_CONFIRM
            echo
            if [ "$ADMIN_PASSWORD" != "$ADMIN_PASSWORD_CONFIRM" ]; then
                echo -e "${RED}Passwords don't match${NC}"
                continue
            fi
            break
        done

        # Hash using Go (secure - password never in command line)
        ADMIN_PASSWORD_HASH=$(echo "$ADMIN_PASSWORD" | /tmp/homeportd-setup hash-password)
    fi

    # Clear password from memory
    unset ADMIN_PASSWORD ADMIN_PASSWORD_CONFIRM
    rm -f /tmp/homeportd-setup

    if [ -z "$ADMIN_PASSWORD_HASH" ]; then
        echo -e "${RED}Failed to hash password.${NC}"
        exit 1
    fi

    # Save config
    mkdir -p ~/.homeport
    cat > ~/.homeport/config << CFGEOF
DOMAIN=$DOMAIN
CFGEOF
    chmod 600 ~/.homeport/config

    echo -e "${GREEN}[*]${NC} Password configured"
    echo ""

    echo -e "${BLUE}[T-1] Ignition: Starting Homeport${NC}"
    echo "==========================================="

    # Generate cookie secret for persistent sessions
    COOKIE_SECRET=$(openssl rand -hex 32)

    # Create .env file
    # Escape $ in bcrypt hash (Docker Compose interprets $ as variable references)
    # Use bracket expression [$] for portable matching, && doubles the match
    ESCAPED_HASH=$(echo "$ADMIN_PASSWORD_HASH" | sed 's/[$]/&&/g')
    {
        echo "DOMAIN=$DOMAIN"
        echo "EXTERNAL_URL=https://$DOMAIN"
        echo "CODE_SERVER_AUTH=none"
        echo "COOKIE_SECRET=$COOKIE_SECRET"
        echo "ADMIN_PASSWORD_HASH=$ESCAPED_HASH"
    } > "$HOMEPORT_DIR/docker/.env"

    echo "Building Docker images (this may take a few minutes)..."
    cd "$HOMEPORT_DIR/docker"

    # Determine docker compose command
    if docker compose version >/dev/null 2>&1; then
        COMPOSE="docker compose"
    elif command -v docker-compose >/dev/null 2>&1; then
        COMPOSE="docker-compose"
    else
        COMPOSE="docker compose"
    fi

    # Run docker commands - use sg if user was just added to docker group
    run_docker() {
        if docker info >/dev/null 2>&1; then
            "$@"
        else
            # User was just added to docker group - use sg to run with that group
            sg docker -c "$*"
        fi
    }

    if ! run_docker $COMPOSE build; then
        echo -e "${RED}Docker build failed.${NC}"
        exit 1
    fi
    echo -e "${GREEN}[*]${NC} Docker images built"

    echo "Starting services..."
    if ! run_docker $COMPOSE up -d; then
        echo -e "${RED}Failed to start services.${NC}"
        exit 1
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
    echo -e "${YELLOW}Note: DNS may take a few minutes to propagate.${NC}"
    echo ""
    echo "Mission control commands:"
    echo "  View logs:     cd $HOMEPORT_DIR/docker && docker compose logs -f"
    echo "  Power down:    cd $HOMEPORT_DIR/docker && docker compose down"
    echo "  Reboot:        cd $HOMEPORT_DIR/docker && docker compose restart"
    echo ""
    echo "Safe travels, Commander."
    echo ""
}

# Run main function - this pattern ensures the entire script is parsed before execution
# which is required for curl|bash to work properly
main "$@"
