package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/gethomeport/homeport/internal/auth"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins since we're behind auth
	},
}

// TerminalMessage represents a message between client and server
type TerminalMessage struct {
	Type string `json:"type"` // "input", "resize", "ping"
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// handleTerminalWebSocket handles WebSocket connections for the terminal
func (s *Server) handleTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate via session cookie
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil || !s.auth.ValidateSession(cookie.Value) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get repo ID and find repo
	repoID := chi.URLParam(r, "repoId")
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Determine shell to use
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
		if _, err := os.Stat(shell); os.IsNotExist(err) {
			shell = "/bin/sh"
		}
	}

	// Start PTY with shell
	cmd := exec.Command(shell)
	cmd.Dir = repo.Path
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("Failed to start PTY: %v", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to start terminal"))
		return
	}
	defer func() {
		ptmx.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Set initial size
	pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Read from PTY and write to WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			select {
			case <-done:
				return
			default:
				n, err := ptmx.Read(buf)
				if err != nil {
					return
				}
				if n > 0 {
					if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
						return
					}
				}
			}
		}
	}()

	// Read from WebSocket and write to PTY
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			// Try to parse as JSON message
			var msg TerminalMessage
			if err := json.Unmarshal(message, &msg); err == nil {
				switch msg.Type {
				case "input":
					ptmx.Write([]byte(msg.Data))
				case "resize":
					if msg.Cols > 0 && msg.Rows > 0 {
						pty.Setsize(ptmx, &pty.Winsize{
							Rows: uint16(msg.Rows),
							Cols: uint16(msg.Cols),
						})
					}
				case "ping":
					conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong"}`))
				}
			} else {
				// Raw input (for compatibility)
				ptmx.Write(message)
			}
		}
	}()

	// Wait for command to exit or connection to close
	wg.Wait()
	log.Printf("Terminal session ended for repo %s", repo.Name)
}

// handleTerminalPage serves the terminal wrapper HTML page
func (s *Server) handleTerminalPage(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoId")
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	// Determine WebSocket URL
	wsProtocol := "ws"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		wsProtocol = "wss"
	}

	page := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>Terminal - %s</title>
    <link rel="icon" type="image/webp" href="/favicon.webp">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.min.css">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        html, body { height: 100%%; overflow: hidden; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #1e1e1e; }

        .terminal-header {
            height: 64px;
            background: #ffffff;
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0 16px;
        }

        .header-left {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .logo-box {
            width: 36px;
            height: 36px;
            border-radius: 12px;
            overflow: hidden;
        }

        .logo-box img {
            width: 100%%;
            height: 100%%;
            object-fit: cover;
        }

        .nav-breadcrumb {
            display: flex;
            align-items: center;
            gap: 8px;
            color: #6b7280;
            font-size: 14px;
        }

        .nav-breadcrumb a {
            color: #6b7280;
            text-decoration: none;
            transition: color 0.15s;
        }

        .nav-breadcrumb a:hover {
            color: #111827;
        }

        .nav-breadcrumb .sep {
            color: #d1d5db;
        }

        .nav-breadcrumb .current {
            color: #111827;
            font-weight: 500;
        }

        .header-right {
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .header-btn {
            height: 36px;
            padding: 0 12px;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.15s;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 6px;
            border: 1px solid #e5e7eb;
            background: white;
            color: #374151;
            text-decoration: none;
        }

        .header-btn:hover {
            background: #f3f4f6;
        }

        .header-btn svg {
            width: 16px;
            height: 16px;
        }

        .terminal-container {
            height: calc(100%% - 64px);
            padding: 8px;
            background: #1e1e1e;
        }

        #terminal {
            height: 100%%;
        }

        .xterm {
            height: 100%%;
        }

        .connection-status {
            position: fixed;
            bottom: 16px;
            right: 16px;
            padding: 8px 16px;
            border-radius: 8px;
            font-size: 13px;
            font-weight: 500;
            z-index: 100;
            transition: all 0.3s;
        }

        .connection-status.connected {
            background: #065f46;
            color: #d1fae5;
        }

        .connection-status.disconnected {
            background: #991b1b;
            color: #fecaca;
        }

        .connection-status.connecting {
            background: #92400e;
            color: #fef3c7;
        }

        .connection-status.hidden {
            opacity: 0;
            pointer-events: none;
        }
    </style>
</head>
<body>
    <header class="terminal-header">
        <div class="header-left">
            <a href="/" style="text-decoration: none;">
                <div class="logo-box">
                    <img src="/favicon.webp" alt="Homeport">
                </div>
            </a>
            <div class="nav-breadcrumb">
                <a href="/">Dashboard</a>
                <span class="sep">/</span>
                <span class="current">%s</span>
                <span class="sep">/</span>
                <span class="current">Terminal</span>
            </div>
        </div>
        <div class="header-right">
            <a href="/code/?folder=/home/coder/repos/%s" class="header-btn">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="16 18 22 12 16 6"/>
                    <polyline points="8 6 2 12 8 18"/>
                </svg>
                VS Code
            </a>
        </div>
    </header>

    <div class="terminal-container">
        <div id="terminal"></div>
    </div>

    <div id="connectionStatus" class="connection-status connecting">Connecting...</div>

    <script src="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/@xterm/addon-web-links@0.11.0/lib/addon-web-links.min.js"></script>
    <script>
        const REPO_ID = '%s';
        const WS_URL = '%s://' + window.location.host + '/api/terminal/' + REPO_ID;

        let term;
        let ws;
        let fitAddon;
        let reconnectAttempts = 0;
        const maxReconnectAttempts = 5;

        function updateStatus(status, message) {
            const el = document.getElementById('connectionStatus');
            el.className = 'connection-status ' + status;
            el.textContent = message;

            if (status === 'connected') {
                setTimeout(() => el.classList.add('hidden'), 2000);
            } else {
                el.classList.remove('hidden');
            }
        }

        function connect() {
            updateStatus('connecting', 'Connecting...');

            ws = new WebSocket(WS_URL);
            ws.binaryType = 'arraybuffer';

            ws.onopen = () => {
                reconnectAttempts = 0;
                updateStatus('connected', 'Connected');

                // Send initial size
                sendResize();
            };

            ws.onmessage = (event) => {
                if (event.data instanceof ArrayBuffer) {
                    term.write(new Uint8Array(event.data));
                } else {
                    term.write(event.data);
                }
            };

            ws.onclose = () => {
                updateStatus('disconnected', 'Disconnected');

                if (reconnectAttempts < maxReconnectAttempts) {
                    reconnectAttempts++;
                    updateStatus('connecting', 'Reconnecting (' + reconnectAttempts + '/' + maxReconnectAttempts + ')...');
                    setTimeout(connect, 1000 * reconnectAttempts);
                } else {
                    updateStatus('disconnected', 'Connection lost. Refresh to reconnect.');
                }
            };

            ws.onerror = (err) => {
                console.error('WebSocket error:', err);
            };
        }

        function sendResize() {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({
                    type: 'resize',
                    cols: term.cols,
                    rows: term.rows
                }));
            }
        }

        function init() {
            term = new Terminal({
                cursorBlink: true,
                fontSize: 14,
                fontFamily: 'Menlo, Monaco, "Courier New", monospace',
                theme: {
                    background: '#1e1e1e',
                    foreground: '#d4d4d4',
                    cursor: '#d4d4d4',
                    cursorAccent: '#1e1e1e',
                    selection: 'rgba(255, 255, 255, 0.3)',
                    black: '#000000',
                    red: '#cd3131',
                    green: '#0dbc79',
                    yellow: '#e5e510',
                    blue: '#2472c8',
                    magenta: '#bc3fbc',
                    cyan: '#11a8cd',
                    white: '#e5e5e5',
                    brightBlack: '#666666',
                    brightRed: '#f14c4c',
                    brightGreen: '#23d18b',
                    brightYellow: '#f5f543',
                    brightBlue: '#3b8eea',
                    brightMagenta: '#d670d6',
                    brightCyan: '#29b8db',
                    brightWhite: '#ffffff'
                }
            });

            fitAddon = new FitAddon.FitAddon();
            term.loadAddon(fitAddon);

            const webLinksAddon = new WebLinksAddon.WebLinksAddon();
            term.loadAddon(webLinksAddon);

            term.open(document.getElementById('terminal'));
            fitAddon.fit();

            // Handle input
            term.onData((data) => {
                if (ws && ws.readyState === WebSocket.OPEN) {
                    ws.send(JSON.stringify({ type: 'input', data: data }));
                }
            });

            // Handle resize
            window.addEventListener('resize', () => {
                fitAddon.fit();
                sendResize();
            });

            // Connect
            connect();

            // Focus terminal
            term.focus();
        }

        // Initialize when DOM is ready
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', init);
        } else {
            init();
        }
    </script>
</body>
</html>`, repo.Name, repo.Name, repo.Name, repoID, wsProtocol)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(page))
}
