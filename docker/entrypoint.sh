#!/bin/sh
# Homeport entrypoint script
# Fixes permissions that get broken by Docker volume mounts, then runs as homeport user

# Fix .config directory ownership (Docker creates it as root when mounting gh config)
if [ -d /home/homeport/.config ]; then
    chown homeport:homeport /home/homeport/.config
fi

# Run homeportd as the homeport user with correct HOME
export HOME=/home/homeport
exec su-exec homeport homeportd "$@"
