#!/bin/bash
#
# Homeport Rollback Script
# Rolls back to the previous version (tagged as :rollback)
#

set -e

# Configuration
COMPOSE_DIR="${COMPOSE_DIR:-/homeport-compose}"

# Docker API compatibility - Alpine's docker-cli may be older than host's dockerd
export DOCKER_API_VERSION=1.44

# Compose project name must match the original install (from ~/homeport/docker)
export COMPOSE_PROJECT_NAME=docker
DATA_DIR="${DATA_DIR:-/srv/homeport/data}"
STATUS_FILE="$DATA_DIR/upgrade-status.json"
LOG_FILE="$DATA_DIR/upgrade.log"
LOCK_FILE="$DATA_DIR/upgrade.lock"

# Write status to JSON file
write_status() {
    local step="$1"
    local message="$2"
    local error="${3:-false}"
    local completed="${4:-false}"
    echo "{\"step\": \"$step\", \"message\": \"$message\", \"error\": $error, \"completed\": $completed, \"version\": \"rollback\"}" > "$STATUS_FILE"
}

# Cleanup lock file on exit
cleanup() {
    rm -f "$LOCK_FILE"
}

# Check if another operation is in progress
if [ -f "$LOCK_FILE" ]; then
    pid=$(cat "$LOCK_FILE" 2>/dev/null || echo "")
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        write_status "error" "Another operation in progress" true false
        exit 1
    fi
    rm -f "$LOCK_FILE"
fi

# Set up lock and cleanup trap
trap cleanup EXIT
echo $$ > "$LOCK_FILE"

echo "=== Homeport Rollback ===" >> "$LOG_FILE"
echo "Started at: $(date)" >> "$LOG_FILE"

# Check if rollback image exists
write_status "checking" "Checking for rollback image..." false false
if ! docker image inspect "ghcr.io/ryanmish/homeport:rollback" > /dev/null 2>&1; then
    write_status "error" "No rollback image available. Cannot roll back." true false
    echo "ERROR: No rollback image found" >> "$LOG_FILE"
    exit 1
fi
echo "Rollback image found" >> "$LOG_FILE"

# Update .env to use rollback version
write_status "rolling_back" "Rolling back to previous version..." false false
cd "$COMPOSE_DIR"
if [ -f ".env" ]; then
    if grep -q "^HOMEPORT_VERSION=" .env; then
        sed -i "s/^HOMEPORT_VERSION=.*/HOMEPORT_VERSION=rollback/" .env
    else
        echo "HOMEPORT_VERSION=rollback" >> .env
    fi
else
    echo "HOMEPORT_VERSION=rollback" > .env
fi

# Restart with rollback image
echo "Restarting with rollback image..." >> "$LOG_FILE"
export HOMEPORT_VERSION="rollback"
docker compose up -d >> "$LOG_FILE" 2>&1

# Health check
write_status "verifying" "Verifying rollback..." false false
echo "Waiting for health check..." >> "$LOG_FILE"
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/api/status > /dev/null 2>&1; then
        write_status "rolled_back" "Rollback complete!" false true
        echo "Health check passed on attempt $i" >> "$LOG_FILE"
        echo "=== Rollback completed successfully at $(date) ===" >> "$LOG_FILE"
        exit 0
    fi
    sleep 1
done

# Health check failed
write_status "error" "Rollback failed. Manual intervention required." true false
echo "ERROR: Rollback health check failed" >> "$LOG_FILE"
exit 1
