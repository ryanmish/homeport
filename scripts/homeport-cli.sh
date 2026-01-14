#!/bin/bash
# Homeport CLI wrapper script
# Talks directly to the daemon API

HOMEPORT_API="${HOMEPORT_API:-http://localhost:8080/api}"

# Helper to find repo by name or ID
find_repo() {
    local search="$1"
    local repos=$(curl -s "$HOMEPORT_API/repos")

    # Try exact match on ID
    local match=$(echo "$repos" | jq -r ".[] | select(.id == \"$search\") | .id + \":\" + .name + \":\" + .path")
    if [ -n "$match" ]; then
        echo "$match"
        return
    fi

    # Try exact match on name
    match=$(echo "$repos" | jq -r ".[] | select(.name == \"$search\") | .id + \":\" + .name + \":\" + .path")
    if [ -n "$match" ]; then
        echo "$match"
        return
    fi

    # Try case-insensitive match on name
    match=$(echo "$repos" | jq -r ".[] | select(.name | ascii_downcase == (\"$search\" | ascii_downcase)) | .id + \":\" + .name + \":\" + .path")
    if [ -n "$match" ]; then
        echo "$match"
        return
    fi
}

case "$1" in
    list)
        curl -s "$HOMEPORT_API/ports" | jq -r '
            if length == 0 then "No ports detected"
            else ["PORT", "PROCESS", "REPO", "SHARE MODE"],
            (.[] | [.port, .process_name // "-", .repo_name // "-", .share_mode])
            | @tsv
            end' | column -t
        ;;

    status)
        curl -s "$HOMEPORT_API/status" | jq -r '
            "Homeport v\(.version)",
            "Status: \(.status)",
            "Uptime: \(.uptime)",
            "Port range: \(.config.port_range)",
            "External URL: \(.config.external_url)",
            "Mode: \(if .config.dev_mode then "development" else "production" end)"'
        PORTS=$(curl -s "$HOMEPORT_API/ports" | jq length)
        echo "Active ports: $PORTS"
        ;;

    repos)
        curl -s "$HOMEPORT_API/repos" | jq -r '
            if length == 0 then "No repositories cloned"
            else ["NAME", "ID", "PATH"], (.[] | [.name, .id, .path]) | @tsv
            end' | column -t
        ;;

    clone)
        if [ -z "$2" ]; then
            echo "Usage: homeport clone <owner/repo>"
            echo "Example: homeport clone facebook/react"
            exit 1
        fi
        if [[ "$2" != *"/"* ]]; then
            echo "Error: invalid format. Use owner/repo (e.g., facebook/react)"
            exit 1
        fi
        echo "Cloning $2..."
        RESULT=$(curl -s -X POST "$HOMEPORT_API/repos/" \
            -H "Content-Type: application/json" \
            -d "{\"repo\":\"$2\"}")
        if echo "$RESULT" | jq -e '.error' > /dev/null 2>&1; then
            echo "Error: $(echo "$RESULT" | jq -r '.error')"
            exit 1
        fi
        echo "Cloned $(echo "$RESULT" | jq -r '.name') to $(echo "$RESULT" | jq -r '.path')"
        echo "Repo ID: $(echo "$RESULT" | jq -r '.id')"
        ;;

    start)
        if [ -z "$2" ]; then
            echo "Usage: homeport start <repo-name>"
            exit 1
        fi
        REPO=$(find_repo "$2")
        if [ -z "$REPO" ]; then
            echo "Error: repository '$2' not found"
            echo "Run 'homeport repos' to see available repositories"
            exit 1
        fi
        REPO_ID=$(echo "$REPO" | cut -d: -f1)
        REPO_NAME=$(echo "$REPO" | cut -d: -f2)
        echo "Starting dev server for $REPO_NAME..."
        RESULT=$(curl -s -X POST "$HOMEPORT_API/process/$REPO_ID/start")
        if echo "$RESULT" | jq -e '.error' > /dev/null 2>&1; then
            echo "Error: $(echo "$RESULT" | jq -r '.error')"
            exit 1
        fi
        echo "Dev server started for $REPO_NAME"
        echo "Use 'homeport list' to see the port"
        ;;

    stop)
        if [ -z "$2" ]; then
            echo "Usage: homeport stop <repo-name>"
            exit 1
        fi
        REPO=$(find_repo "$2")
        if [ -z "$REPO" ]; then
            echo "Error: repository '$2' not found"
            echo "Run 'homeport repos' to see available repositories"
            exit 1
        fi
        REPO_ID=$(echo "$REPO" | cut -d: -f1)
        REPO_NAME=$(echo "$REPO" | cut -d: -f2)
        echo "Stopping dev server for $REPO_NAME..."
        RESULT=$(curl -s -X POST "$HOMEPORT_API/process/$REPO_ID/stop")
        if echo "$RESULT" | jq -e '.error' > /dev/null 2>&1; then
            echo "Error: $(echo "$RESULT" | jq -r '.error')"
            exit 1
        fi
        echo "Dev server stopped for $REPO_NAME"
        ;;

    logs)
        if [ -z "$2" ]; then
            echo "Usage: homeport logs <repo-name> [-n lines] [-f]"
            exit 1
        fi
        REPO=$(find_repo "$2")
        if [ -z "$REPO" ]; then
            echo "Error: repository '$2' not found"
            echo "Run 'homeport repos' to see available repositories"
            exit 1
        fi
        REPO_ID=$(echo "$REPO" | cut -d: -f1)
        LINES=50
        FOLLOW=false
        shift 2
        while [ $# -gt 0 ]; do
            case "$1" in
                -n) LINES="$2"; shift 2 ;;
                -f) FOLLOW=true; shift ;;
                *) shift ;;
            esac
        done

        if [ "$FOLLOW" = true ]; then
            LAST_TIME=""
            while true; do
                LOGS=$(curl -s "$HOMEPORT_API/process/$REPO_ID/logs?limit=$LINES")
                echo "$LOGS" | jq -r --arg last "$LAST_TIME" '.[] | select(.time > $last) | (if .stream == "stderr" then "[ERR] " else "" end) + .message'
                LAST_TIME=$(echo "$LOGS" | jq -r '.[-1].time // ""')
                sleep 2
            done
        else
            curl -s "$HOMEPORT_API/process/$REPO_ID/logs?limit=$LINES" | jq -r '.[] | (if .stream == "stderr" then "[ERR] " else "" end) + .message'
        fi
        ;;

    open)
        if [ -z "$2" ]; then
            echo "Usage: homeport open <repo-name>"
            exit 1
        fi
        REPO=$(find_repo "$2")
        if [ -z "$REPO" ]; then
            echo "Error: repository '$2' not found"
            echo "Run 'homeport repos' to see available repositories"
            exit 1
        fi
        REPO_NAME=$(echo "$REPO" | cut -d: -f2)
        EXTERNAL_URL=$(curl -s "$HOMEPORT_API/status" | jq -r '.config.external_url')
        echo "$EXTERNAL_URL/code/?folder=/home/coder/repos/$REPO_NAME"
        ;;

    terminal)
        if [ -z "$2" ]; then
            echo "Usage: homeport terminal <repo-name>"
            exit 1
        fi
        REPO=$(find_repo "$2")
        if [ -z "$REPO" ]; then
            echo "Error: repository '$2' not found"
            echo "Run 'homeport repos' to see available repositories"
            exit 1
        fi
        REPO_ID=$(echo "$REPO" | cut -d: -f1)
        EXTERNAL_URL=$(curl -s "$HOMEPORT_API/status" | jq -r '.config.external_url')
        echo "$EXTERNAL_URL/terminal/$REPO_ID"
        ;;

    share)
        if [ -z "$2" ]; then
            echo "Usage: homeport share <port> [--public|--password]"
            exit 1
        fi
        PORT=$2
        MODE="private"
        if [ "$3" == "--public" ]; then
            MODE="public"
        elif [ "$3" == "--password" ]; then
            MODE="password"
            read -s -p "Enter password: " PASS
            echo
            curl -s -X POST "$HOMEPORT_API/share/$PORT" \
                -H "Content-Type: application/json" \
                -d "{\"mode\":\"$MODE\",\"password\":\"$PASS\"}" | jq -r '"Port \(.port // '$PORT') shared as \(.mode)\nURL: \(.url)"'
            exit 0
        fi
        curl -s -X POST "$HOMEPORT_API/share/$PORT" \
            -H "Content-Type: application/json" \
            -d "{\"mode\":\"$MODE\"}" | jq -r '"Port \(.port // '$PORT') shared as \(.mode)\nURL: \(.url)"'
        ;;

    unshare)
        if [ -z "$2" ]; then
            echo "Usage: homeport unshare <port>"
            exit 1
        fi
        curl -s -X DELETE "$HOMEPORT_API/share/$2" | jq -r '"Port '$2' unshared (now private)"'
        ;;

    url)
        if [ -z "$2" ]; then
            echo "Usage: homeport url <port>"
            exit 1
        fi
        EXTERNAL_URL=$(curl -s "$HOMEPORT_API/status" | jq -r '.config.external_url')
        echo "$EXTERNAL_URL/$2/"
        ;;

    uninstall)
        if [ -f ~/homeport/scripts/uninstall.sh ]; then
            bash ~/homeport/scripts/uninstall.sh
        elif [ -f /opt/homeport/scripts/uninstall.sh ]; then
            bash /opt/homeport/scripts/uninstall.sh
        else
            echo "Could not find uninstall script. Run manually:"
            echo "  bash ~/homeport/scripts/uninstall.sh"
        fi
        ;;

    *)
        echo "Homeport CLI - manage your dev servers"
        echo ""
        echo "Repository commands:"
        echo "  repos                List cloned repositories"
        echo "  clone <owner/repo>   Clone a GitHub repository"
        echo ""
        echo "Dev server commands:"
        echo "  start <repo>         Start the dev server"
        echo "  stop <repo>          Stop the dev server"
        echo "  logs <repo>          Show dev server logs (-n lines, -f follow)"
        echo ""
        echo "Port commands:"
        echo "  list                 List all active ports"
        echo "  url <port>           Get the external URL for a port"
        echo "  share <port>         Share a port (--public, --password)"
        echo "  unshare <port>       Remove sharing from a port"
        echo ""
        echo "URL commands:"
        echo "  open <repo>          Get VS Code URL for a repository"
        echo "  terminal <repo>      Get terminal URL for a repository"
        echo ""
        echo "System commands:"
        echo "  status               Show Homeport status"
        echo "  uninstall            Uninstall Homeport"
        ;;
esac
