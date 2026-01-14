#!/bin/sh
# Homeport entrypoint script
# Fixes permissions that get broken by Docker volume mounts, then runs as homeport user

# Set HOME early for all subsequent commands
export HOME=/home/homeport

# Fix .config directory ownership (Docker creates it as root when mounting gh config)
if [ -d /home/homeport/.config ]; then
    chown homeport:homeport /home/homeport/.config
fi

# Configure git to use gh CLI for GitHub authentication
su-exec homeport git config --global credential.helper '!gh auth git-credential'

# Mark repos directory as safe (avoids "dubious ownership" errors with mounted volumes)
su-exec homeport git config --global --add safe.directory '*'

# Run homeportd as the homeport user
exec su-exec homeport homeportd "$@"
