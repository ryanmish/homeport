#!/bin/bash
# Code-server entrypoint script
# Configures git for GitHub operations, then runs code-server

# Configure git to use gh CLI for GitHub authentication (if gh config exists)
if [ -f /home/coder/.config/gh/hosts.yml ]; then
    git config --global credential.helper '!gh auth git-credential'
    git config --global --add safe.directory '*'

    # Configure git user from GitHub if authenticated
    if gh auth status >/dev/null 2>&1; then
        GH_USER=$(gh api user --jq '.login' 2>/dev/null)
        GH_EMAIL=$(gh api user --jq '.email // ""' 2>/dev/null)
        if [ -n "$GH_USER" ]; then
            git config --global user.name "$GH_USER"
            # Use noreply email if no public email set
            if [ -z "$GH_EMAIL" ]; then
                GH_EMAIL="${GH_USER}@users.noreply.github.com"
            fi
            git config --global user.email "$GH_EMAIL"
        fi
    fi
fi

# Run the original code-server entrypoint
exec /usr/bin/entrypoint.sh "$@"
