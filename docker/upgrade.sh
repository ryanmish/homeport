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
COMPOSE_DIR="${COMPOSE_DIR:-/home/ryan/homeport/docker}"
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

# Restart with new image
write_status "restarting" "Restarting services..." false false
echo "Restarting with HOMEPORT_VERSION=$VERSION" >> "$LOG_FILE"
cd "$COMPOSE_DIR"
export HOMEPORT_VERSION="$VERSION"
docker compose up -d >> "$LOG_FILE" 2>&1

# Health check (wait up to 30s for new container to be healthy)
write_status "verifying" "Verifying upgrade..." false false
echo "Waiting for health check..." >> "$LOG_FILE"
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/api/status > /dev/null 2>&1; then
        write_status "complete" "Upgrade complete!" false true
        echo "Health check passed on attempt $i" >> "$LOG_FILE"
        echo "=== Upgrade completed successfully at $(date) ===" >> "$LOG_FILE"
        exit 0
    fi
    sleep 1
done

# Health check failed
write_status "error" "New version failed health check. Check logs." true false
echo "ERROR: Health check failed after 30 seconds" >> "$LOG_FILE"
exit 1
