#!/bin/bash
# Wrapper script to run homeport CLI
# Talks directly to the daemon API

HOMEPORT_API="${HOMEPORT_API:-http://localhost:8080/api}"

case "$1" in
    list)
        curl -s "$HOMEPORT_API/ports" | jq -r '
            ["PORT", "PROCESS", "REPO", "SHARE MODE"],
            (.[] | [.port, .process_name // "-", .repo_name // "-", .share_mode])
            | @tsv' | column -t
        ;;
    status)
        curl -s "$HOMEPORT_API/status" | jq -r '
            "Homeport v\(.version)",
            "Status: \(.status)",
            "Uptime: \(.uptime)",
            "Port range: \(.config.port_range)",
            "External URL: \(.config.external_url)",
            "Mode: \(if .config.dev_mode then "development" else "production" end)"'
        ;;
    repos)
        curl -s "$HOMEPORT_API/repos" | jq -r '
            if length == 0 then "No repositories cloned"
            else ["NAME", "PATH"], (.[] | [.name, .path]) | @tsv
            end' | column -t
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
                -d "{\"mode\":\"$MODE\",\"password\":\"$PASS\"}" | jq -r '"Port \(.port // $PORT) shared as \(.mode)"'
            exit 0
        fi
        curl -s -X POST "$HOMEPORT_API/share/$PORT" \
            -H "Content-Type: application/json" \
            -d "{\"mode\":\"$MODE\"}" | jq -r '"Port shared as \(.mode)\nURL: \(.url)"'
        ;;
    unshare)
        if [ -z "$2" ]; then
            echo "Usage: homeport unshare <port>"
            exit 1
        fi
        curl -s -X DELETE "$HOMEPORT_API/share/$2" | jq -r '"Port $2 unshared"'
        ;;
    url)
        if [ -z "$2" ]; then
            echo "Usage: homeport url <port>"
            exit 1
        fi
        STATUS=$(curl -s "$HOMEPORT_API/status")
        URL=$(echo "$STATUS" | jq -r '.config.external_url')
        echo "$URL/$2/"
        ;;
    uninstall)
        # Find the uninstall script
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
        echo "HOMEPORT - Mission Control CLI"
        echo ""
        echo "Commands:"
        echo "  list              Show active docking bays"
        echo "  status            Station status report"
        echo "  repos             List docked repositories"
        echo "  share <port>      Open airlock for external access"
        echo "    --public        Open to all vessels"
        echo "    --password      Require access code"
        echo "  unshare <port>    Seal airlock"
        echo "  url <port>        Get docking coordinates"
        echo "  uninstall         Decommission station"
        ;;
esac
