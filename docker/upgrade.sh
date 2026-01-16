#!/bin/bash
#
# Homeport Self-Upgrade Script
# Runs as a detached process to build from source and restart containers.
# Status is written to a JSON file for the frontend to poll.
#

set -e

# Configuration
VERSION="${1:-latest}"
# When running inside container, compose dir is mounted at /homeport-compose
# When running on host, it's the current directory or specified via env
COMPOSE_DIR="${COMPOSE_DIR:-/homeport-compose}"
REPO_DIR="${REPO_DIR:-/homeport-repo}"

# Docker API compatibility - Alpine's docker-cli may be older than host's dockerd
export DOCKER_API_VERSION=1.44

# Compose project name must match the original install (from ~/homeport/docker)
export COMPOSE_PROJECT_NAME=docker
DATA_DIR="${DATA_DIR:-/srv/homeport/data}"
STATUS_FILE="$DATA_DIR/upgrade-status.json"
LOG_FILE="$DATA_DIR/upgrade.log"
LOCK_FILE="$DATA_DIR/upgrade.lock"
MAX_RETRIES=3

# Write status to JSON file
write_status() {
    local step="$1"
    local message="$2"
    local error="${3:-false}"
    local completed="${4:-false}"
    echo "{\"step\": \"$step\", \"message\": \"$message\", \"error\": $error, \"completed\": $completed, \"version\": \"$VERSION\"}" > "$STATUS_FILE"
    # Ensure homeportd (uid 1000) can read/write the status file
    chown 1000:1000 "$STATUS_FILE" 2>/dev/null || true
}

# Cleanup lock file on exit
cleanup() {
    rm -f "$LOCK_FILE"
}

# Set up lock and cleanup trap
trap cleanup EXIT
echo $$ > "$LOCK_FILE"

# Clear previous log and status
> "$LOG_FILE"
rm -f "$STATUS_FILE"

# Ensure log file is writable by homeportd
chown 1000:1000 "$LOG_FILE" 2>/dev/null || true

# Write initial status immediately
write_status "starting" "Starting upgrade to $VERSION..." false false

echo "=== Homeport Upgrade to $VERSION ===" >> "$LOG_FILE"
echo "Started at: $(date)" >> "$LOG_FILE"

# Pre-flight: Check disk space (need ~1GB free for images)
write_status "checking" "Checking disk space..." false false
FREE_SPACE=$(df -BG /var/lib/docker 2>/dev/null | tail -1 | awk '{print $4}' | tr -d 'G' || echo "10")
if [ "$FREE_SPACE" -lt 1 ]; then
    write_status "error" "Insufficient disk space (need 1GB free, have ${FREE_SPACE}GB)" true false
    echo "ERROR: Insufficient disk space" >> "$LOG_FILE"
    exit 1
fi
echo "Disk space check passed: ${FREE_SPACE}GB available" >> "$LOG_FILE"

# Fetch latest code from GitHub
write_status "pulling" "Downloading update..." false false
echo "Fetching version: $VERSION" >> "$LOG_FILE"
cd "$REPO_DIR"

# Mark repo as safe (running as root but repo owned by different user)
git config --global --add safe.directory "$REPO_DIR"
for i in $(seq 1 $MAX_RETRIES); do
    if git fetch --tags >> "$LOG_FILE" 2>&1; then
        echo "Fetch successful on attempt $i" >> "$LOG_FILE"
        break
    fi
    if [ $i -eq $MAX_RETRIES ]; then
        write_status "error" "Failed to fetch updates after $MAX_RETRIES attempts" true false
        echo "ERROR: Fetch failed after $MAX_RETRIES attempts" >> "$LOG_FILE"
        exit 1
    fi
    echo "Fetch attempt $i failed, retrying in 5s..." >> "$LOG_FILE"
    sleep 5
done

# Checkout the requested version
# Add 'v' prefix if version looks like a semver without it (e.g., 1.0.8 -> v1.0.8)
if [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    VERSION="v$VERSION"
fi

if [ "$VERSION" = "latest" ]; then
    # Get the latest tag
    VERSION=$(git describe --tags --abbrev=0 origin/main 2>/dev/null || echo "main")
    echo "Latest version is: $VERSION" >> "$LOG_FILE"
fi
echo "Checking out $VERSION..." >> "$LOG_FILE"
if ! git checkout "$VERSION" >> "$LOG_FILE" 2>&1; then
    write_status "error" "Failed to checkout version $VERSION" true false
    echo "ERROR: Failed to checkout $VERSION" >> "$LOG_FILE"
    exit 1
fi

# Tag current image for rollback (best effort)
echo "Tagging current image for rollback..." >> "$LOG_FILE"
CURRENT_IMAGE=$(docker compose -f "$COMPOSE_DIR/docker-compose.yml" images homeportd -q 2>/dev/null | head -1 || true)
if [ -n "$CURRENT_IMAGE" ]; then
    docker tag "$CURRENT_IMAGE" "homeport:rollback" 2>/dev/null || true
    echo "Tagged $CURRENT_IMAGE as rollback" >> "$LOG_FILE"
fi

# Build from source
write_status "building" "Building update..." false false
echo "Building from source with VERSION=$VERSION..." >> "$LOG_FILE"
cd "$COMPOSE_DIR"

# Set HOMEPORT_VERSION env var - docker-compose.yml uses this for build args
export HOMEPORT_VERSION="$VERSION"

if ! docker compose build >> "$LOG_FILE" 2>&1; then
    write_status "error" "Failed to build update" true false
    echo "ERROR: Build failed" >> "$LOG_FILE"
    exit 1
fi
echo "Build complete" >> "$LOG_FILE"

# Update .env with new version (persists across reboots)
echo "Updating .env with HOMEPORT_VERSION=$VERSION" >> "$LOG_FILE"
if [ -f ".env" ]; then
    if grep -q "^HOMEPORT_VERSION=" .env; then
        sed -i "s/^HOMEPORT_VERSION=.*/HOMEPORT_VERSION=$VERSION/" .env
    else
        echo "HOMEPORT_VERSION=$VERSION" >> .env
    fi
else
    echo "HOMEPORT_VERSION=$VERSION" > .env
fi

# Restart with new build
write_status "restarting" "Restarting services..." false false
echo "Restarting services..." >> "$LOG_FILE"

# Start services - retry if needed to handle transient failures
for attempt in 1 2 3; do
    docker compose up -d >> "$LOG_FILE" 2>&1

    # Verify critical services are running (not just created)
    sleep 2
    HOMEPORTD_STATUS=$(docker inspect -f '{{.State.Status}}' homeportd 2>/dev/null || echo "missing")
    CADDY_STATUS=$(docker inspect -f '{{.State.Status}}' caddy 2>/dev/null || echo "missing")

    if [ "$HOMEPORTD_STATUS" = "running" ] && [ "$CADDY_STATUS" = "running" ]; then
        echo "Critical services started on attempt $attempt" >> "$LOG_FILE"
        break
    fi

    echo "Services not fully started (homeportd=$HOMEPORTD_STATUS, caddy=$CADDY_STATUS), retrying..." >> "$LOG_FILE"
    sleep 3
done

# Health check (wait up to 30s for new container to be healthy)
write_status "verifying" "Verifying upgrade..." false false
echo "Waiting for health check..." >> "$LOG_FILE"
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/api/status > /dev/null 2>&1; then
        write_status "complete" "Upgrade complete!" false true
        echo "Health check passed on attempt $i" >> "$LOG_FILE"

        # Clean up old images to save disk space (keep rollback image for safety)
        echo "Cleaning up old images..." >> "$LOG_FILE"
        # Remove dangling images
        docker image prune -f >> "$LOG_FILE" 2>&1 || true
        # Remove old homeport images except current and rollback
        docker images "homeport" --format "{{.Tag}}" 2>/dev/null | \
            grep -v -E "^($VERSION|rollback|latest)$" | \
            xargs -I {} docker rmi "homeport:{}" >> "$LOG_FILE" 2>&1 || true
        echo "Cleanup complete" >> "$LOG_FILE"

        echo "=== Upgrade completed successfully at $(date) ===" >> "$LOG_FILE"
        exit 0
    fi
    sleep 1
done

# Health check failed - attempt automatic rollback
echo "ERROR: Health check failed after 30 seconds" >> "$LOG_FILE"
echo "Attempting automatic rollback..." >> "$LOG_FILE"
write_status "rolling_back" "Upgrade failed, rolling back..." false false

# Check if rollback image exists
if docker image inspect "homeport:rollback" > /dev/null 2>&1; then
    # Restore previous version in .env
    if grep -q "^HOMEPORT_VERSION=" .env; then
        sed -i "s/^HOMEPORT_VERSION=.*/HOMEPORT_VERSION=rollback/" .env
    else
        echo "HOMEPORT_VERSION=rollback" >> .env
    fi

    # Restart with rollback image
    export HOMEPORT_VERSION="rollback"
    docker compose up -d >> "$LOG_FILE" 2>&1

    # Wait for rollback to be healthy
    echo "Waiting for rollback health check..." >> "$LOG_FILE"
    for i in $(seq 1 30); do
        if curl -sf http://localhost:8080/api/status > /dev/null 2>&1; then
            write_status "rolled_back" "Upgrade failed. Rolled back to previous version." true false
            echo "Rollback successful on attempt $i" >> "$LOG_FILE"
            echo "=== Rolled back successfully at $(date) ===" >> "$LOG_FILE"
            exit 1
        fi
        sleep 1
    done

    # Rollback also failed
    write_status "error" "Upgrade failed and rollback failed. Manual intervention required." true false
    echo "ERROR: Rollback also failed" >> "$LOG_FILE"
    exit 1
else
    # No rollback image available
    write_status "error" "Upgrade failed. No rollback image available." true false
    echo "ERROR: No rollback image available" >> "$LOG_FILE"
    exit 1
fi
