#!/bin/sh
# Homeport entrypoint script
# Fixes permissions that get broken by Docker volume mounts, then runs as homeport user

# Set environment variables
export HOME=/home/homeport
export USER=homeport
export LANG=C.UTF-8
export LC_ALL=C.UTF-8

# Fix .config directory ownership (Docker creates it as root when mounting gh config)
if [ -d /home/homeport/.config ]; then
    chown homeport:homeport /home/homeport/.config
fi

# Configure git to use gh CLI for GitHub authentication
su-exec homeport git config --global credential.helper '!gh auth git-credential'

# Add GitHub to SSH known_hosts (prevents "authenticity of host" prompt)
mkdir -p /home/homeport/.ssh
ssh-keyscan -t ed25519 github.com >> /home/homeport/.ssh/known_hosts 2>/dev/null
chown -R homeport:homeport /home/homeport/.ssh
chmod 700 /home/homeport/.ssh
chmod 600 /home/homeport/.ssh/known_hosts

# Mark repos directory as safe (avoids "dubious ownership" errors with mounted volumes)
su-exec homeport git config --global --add safe.directory '*'

# Configure git user from GitHub if authenticated (for commits)
# Use env to ensure HOME is passed to su-exec'd process
if env HOME=$HOME su-exec homeport gh auth status >/dev/null 2>&1; then
    GH_USER=$(env HOME=$HOME su-exec homeport gh api user --jq '.login' 2>/dev/null)
    GH_EMAIL=$(env HOME=$HOME su-exec homeport gh api user --jq '.email // ""' 2>/dev/null)
    if [ -n "$GH_USER" ]; then
        su-exec homeport git config --global user.name "$GH_USER"
        # Use noreply email if no public email set
        if [ -z "$GH_EMAIL" ]; then
            GH_EMAIL="${GH_USER}@users.noreply.github.com"
        fi
        su-exec homeport git config --global user.email "$GH_EMAIL"
    fi
fi

# Run homeportd as the homeport user
exec su-exec homeport homeportd "$@"
