#!/bin/bash
# Local development script for Homeport
# Works on Mac with Colima

set -e

cd "$(dirname "$0")/../docker"

# Safety check: fail if homeportd is running outside Docker
if pgrep -x homeportd > /dev/null 2>&1; then
    echo "ERROR: homeportd is already running outside Docker!"
    echo "Kill it first: pkill homeportd"
    exit 1
fi

# Safety check: fail if port 8080 is in use by non-Docker process
if lsof -i :8080 -sTCP:LISTEN 2>/dev/null | grep -v docker > /dev/null; then
    echo "ERROR: Port 8080 is in use by a non-Docker process!"
    echo "Check with: lsof -i :8080"
    exit 1
fi

cmd="${1:-up}"
shift 2>/dev/null || true

case "$cmd" in
  up)
    echo "Starting Homeport dev environment..."
    docker compose -f docker-compose.dev.yml up --build "$@"
    ;;
  down)
    echo "Stopping Homeport dev environment..."
    docker compose -f docker-compose.dev.yml down
    ;;
  clean)
    echo "Cleaning up dev environment..."
    docker compose -f docker-compose.dev.yml down
    rm -rf .dev-data
    echo "Done. All dev data removed."
    ;;
  logs)
    docker compose -f docker-compose.dev.yml logs -f
    ;;
  rebuild)
    echo "Full rebuild (no cache)..."
    docker compose -f docker-compose.dev.yml build --no-cache
    docker compose -f docker-compose.dev.yml up
    ;;
  *)
    echo "Usage: $0 {up|down|clean|logs|rebuild}"
    echo ""
    echo "  up      - Start dev environment (default)"
    echo "  down    - Stop containers"
    echo "  clean   - Stop and remove all dev data"
    echo "  logs    - Follow container logs"
    echo "  rebuild - Full rebuild with --no-cache"
    exit 1
    ;;
esac
