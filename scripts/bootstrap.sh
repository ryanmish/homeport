#!/bin/bash
# Bootstrap script for curl | bash installation
# Usage: curl -fsSL https://raw.githubusercontent.com/ryanmish/homeport/main/scripts/bootstrap.sh | bash

set -e

REPO_URL="https://github.com/ryanmish/homeport.git"
INSTALL_DIR="$HOME/homeport"

echo ""
echo "  *  .  *  .  *  .  *  .  *  .  *"
echo "       HOMEPORT BOOTSTRAP"
echo "  *  .  *  .  *  .  *  .  *  .  *"
echo ""
echo "  Preparing for launch..."
echo ""

# Check if git is installed
if ! command -v git &> /dev/null; then
    echo "  Loading navigation systems (git)..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq git
fi

# Clone or update repo
if [ -d "$INSTALL_DIR" ]; then
    echo "  Updating flight computer..."
    cd "$INSTALL_DIR"
    git pull --quiet
else
    echo "  Downloading mission files..."
    git clone --quiet "$REPO_URL" "$INSTALL_DIR"
    cd "$INSTALL_DIR"
fi

# Run the main install script
echo ""
echo "  Initiating main launch sequence..."
echo ""
exec bash "$INSTALL_DIR/scripts/install.sh"
