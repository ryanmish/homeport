#!/bin/bash
#
# Homeport Self-Upgrade Script
# Runs as a detached process to pull new images and restart containers.
# Status is written to a JSON file for the frontend to poll.
#

set -e

# Configuration
VERSION="${1:-latest}"
IMAGE="ghcr.io/ryanmish/homeport:$VERSION"
# When running inside container, compose dir is mounted at /homeport-compose
# When running on host, it's the current directory or specified via env
COMPOSE_DIR="${COMPOSE_DIR:-/homeport-compose}"
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
}

# Cleanup lock file on exit
cleanup() {
    rm -f "$LOCK_FILE"
}

# Check if another upgrade is in progress
if [ -f "$LOCK_FILE" ]; then
    pid=$(cat "$LOCK_FILE" 2>/dev/null || echo "")
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        write_status "error" "Upgrade already in progress" true false
        exit 1
    fi
    # Stale lock file, remove it
    rm -f "$LOCK_FILE"
fi

# Set up lock and cleanup trap
trap cleanup EXIT
echo $$ > "$LOCK_FILE"

# Clear previous log
> "$LOG_FILE"

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

# Pull new image with retry logic
write_status "pulling" "Downloading update..." false false
echo "Pulling image: $IMAGE" >> "$LOG_FILE"
for i in $(seq 1 $MAX_RETRIES); do
    if docker pull "$IMAGE" >> "$LOG_FILE" 2>&1; then
        echo "Pull successful on attempt $i" >> "$LOG_FILE"
        break
    fi
    if [ $i -eq $MAX_RETRIES ]; then
        write_status "error" "Failed to download update after $MAX_RETRIES attempts" true false
        echo "ERROR: Pull failed after $MAX_RETRIES attempts" >> "$LOG_FILE"
        exit 1
    fi
    echo "Pull attempt $i failed, retrying in 5s..." >> "$LOG_FILE"
    sleep 5
done

# Tag current image for rollback (best effort)
echo "Tagging current image for rollback..." >> "$LOG_FILE"
CURRENT_IMAGE=$(docker compose -f "$COMPOSE_DIR/docker-compose.yml" images homeportd -q 2>/dev/null | head -1 || true)
if [ -n "$CURRENT_IMAGE" ]; then
    docker tag "$CURRENT_IMAGE" "ghcr.io/ryanmish/homeport:rollback" 2>/dev/null || true
    echo "Tagged $CURRENT_IMAGE as rollback" >> "$LOG_FILE"
fi

# Update .env file with new version (persists across reboots)
write_status "restarting" "Restarting services..." false false
echo "Updating HOMEPORT_VERSION in .env to $VERSION" >> "$LOG_FILE"
cd "$COMPOSE_DIR"
if [ -f ".env" ]; then
    # Replace existing HOMEPORT_VERSION line or add it if missing
    if grep -q "^HOMEPORT_VERSION=" .env; then
        sed -i "s/^HOMEPORT_VERSION=.*/HOMEPORT_VERSION=$VERSION/" .env
    else
        echo "HOMEPORT_VERSION=$VERSION" >> .env
    fi
else
    echo "HOMEPORT_VERSION=$VERSION" > .env
fi

# Restart with new image
echo "Restarting with HOMEPORT_VERSION=$VERSION" >> "$LOG_FILE"
export HOMEPORT_VERSION="$VERSION"
docker compose up -d >> "$LOG_FILE" 2>&1

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
        docker images "ghcr.io/ryanmish/homeport" --format "{{.Tag}}" 2>/dev/null | \
            grep -v -E "^($VERSION|rollback|latest)$" | \
            xargs -I {} docker rmi "ghcr.io/ryanmish/homeport:{}" >> "$LOG_FILE" 2>&1 || true
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
if docker image inspect "ghcr.io/ryanmish/homeport:rollback" > /dev/null 2>&1; then
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
