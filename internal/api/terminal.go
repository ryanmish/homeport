package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/gethomeport/homeport/internal/auth"
	"github.com/gethomeport/homeport/internal/terminal"
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
	Type      string `json:"type"` // "input", "resize", "ping"
	Data      string `json:"data,omitempty"`
	Cols      int    `json:"cols,omitempty"`
	Rows      int    `json:"rows,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// handleTerminalSessions lists all sessions for a repo
func (s *Server) handleTerminalSessions(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoId")
	sessions := s.termMgr.ListSessions(repoID)

	type sessionInfo struct {
		ID        string `json:"id"`
		RepoID    string `json:"repo_id"`
		CreatedAt int64  `json:"created_at"`
	}

	var result []sessionInfo
	for _, sess := range sessions {
		result = append(result, sessionInfo{
			ID:        sess.ID,
			RepoID:    sess.RepoID,
			CreatedAt: sess.CreatedAt.Unix(),
		})
	}

	if result == nil {
		result = []sessionInfo{}
	}

	jsonResponse(w, http.StatusOK, result)
}

// handleCreateTerminalSession creates a new terminal session
func (s *Server) handleCreateTerminalSession(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoId")
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "Repository not found")
		return
	}

	session, err := s.termMgr.CreateSession(repoID, repo.Path)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"id":      session.ID,
		"repo_id": session.RepoID,
	})
}

// handleDeleteTerminalSession deletes a terminal session
func (s *Server) handleDeleteTerminalSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	s.termMgr.DeleteSession(sessionID)
	jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleTerminalWebSocket handles WebSocket connections for the terminal
func (s *Server) handleTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate via session cookie
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil || !s.auth.ValidateSession(cookie.Value) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	repoID := chi.URLParam(r, "repoId")
	sessionID := r.URL.Query().Get("session")

	// Get or create session
	var session *terminal.Session
	if sessionID != "" {
		session = s.termMgr.GetSession(sessionID)
	}

	// If no session or session doesn't exist, create new one
	if session == nil {
		repo, err := s.store.GetRepo(repoID)
		if err != nil {
			http.Error(w, "Repository not found", http.StatusNotFound)
			return
		}
		session, err = s.termMgr.CreateSession(repoID, repo.Path)
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Track client connection
	session.AddClient()
	defer session.RemoveClient()

	// Send session ID to client
	conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"session","id":"%s"}`, session.ID)))

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
				n, err := session.Read(buf)
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

			var msg TerminalMessage
			if err := json.Unmarshal(message, &msg); err == nil {
				switch msg.Type {
				case "input":
					session.Write([]byte(msg.Data))
				case "resize":
					if msg.Cols > 0 && msg.Rows > 0 {
						session.Resize(uint16(msg.Cols), uint16(msg.Rows))
					}
				case "ping":
					conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong"}`))
				}
			} else {
				session.Write(message)
			}
		}
	}()

	wg.Wait()
	log.Printf("Terminal WebSocket closed for session %s", session.ID)
}

// handleTerminalPage serves the terminal wrapper HTML page
func (s *Server) handleTerminalPage(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoId")
	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
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
        html, body { height: 100%%; overflow: hidden; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        body.dark { background: #1e1e1e; }
        body.light { background: #ffffff; }

        .terminal-header {
            height: 64px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0 16px;
            border-bottom: 1px solid;
        }
        body.dark .terminal-header { background: #252526; border-color: #3c3c3c; }
        body.light .terminal-header { background: #ffffff; border-color: #e5e7eb; }

        .header-left { display: flex; align-items: center; gap: 12px; }

        .logo-box { width: 36px; height: 36px; border-radius: 12px; overflow: hidden; }
        .logo-box img { width: 100%%; height: 100%%; object-fit: cover; }

        .nav-breadcrumb { display: flex; align-items: center; gap: 8px; font-size: 14px; }
        body.dark .nav-breadcrumb { color: #9ca3af; }
        body.light .nav-breadcrumb { color: #6b7280; }
        .nav-breadcrumb a { text-decoration: none; transition: color 0.15s; }
        body.dark .nav-breadcrumb a { color: #9ca3af; }
        body.light .nav-breadcrumb a { color: #6b7280; }
        body.dark .nav-breadcrumb a:hover { color: #ffffff; }
        body.light .nav-breadcrumb a:hover { color: #111827; }
        .nav-breadcrumb .sep { opacity: 0.5; }
        body.dark .nav-breadcrumb .current { color: #ffffff; font-weight: 500; }
        body.light .nav-breadcrumb .current { color: #111827; font-weight: 500; }

        .header-right { display: flex; align-items: center; gap: 8px; }

        .header-btn {
            height: 32px; width: 32px; padding: 0;
            border-radius: 6px; font-size: 14px; font-weight: 500;
            cursor: pointer; transition: all 0.15s;
            display: inline-flex; align-items: center; justify-content: center;
            border: none; background: transparent;
        }
        body.dark .header-btn { color: #9ca3af; }
        body.light .header-btn { color: #6b7280; }
        body.dark .header-btn:hover { background: #3c3c3c; color: #ffffff; }
        body.light .header-btn:hover { background: #f3f4f6; color: #111827; }
        .header-btn svg { width: 18px; height: 18px; }

        .header-btn.text { width: auto; padding: 0 12px; gap: 6px; border: 1px solid; }
        body.dark .header-btn.text { border-color: #3c3c3c; }
        body.light .header-btn.text { border-color: #e5e7eb; }
        .header-btn.text svg { width: 16px; height: 16px; }

        /* Tab bar */
        .tab-bar {
            height: 36px;
            display: flex;
            align-items: center;
            padding: 0 8px;
            gap: 4px;
            overflow-x: auto;
            -webkit-overflow-scrolling: touch;
        }
        body.dark .tab-bar { background: #1e1e1e; border-bottom: 1px solid #3c3c3c; }
        body.light .tab-bar { background: #f9fafb; border-bottom: 1px solid #e5e7eb; }

        .tab {
            display: flex; align-items: center; gap: 6px;
            padding: 6px 12px; border-radius: 6px;
            font-size: 13px; cursor: pointer;
            white-space: nowrap; transition: all 0.15s;
            border: 1px solid transparent;
        }
        body.dark .tab { color: #9ca3af; }
        body.light .tab { color: #6b7280; }
        body.dark .tab:hover { background: #2d2d2d; }
        body.light .tab:hover { background: #f3f4f6; }
        body.dark .tab.active { background: #3c3c3c; color: #ffffff; border-color: #4c4c4c; }
        body.light .tab.active { background: #ffffff; color: #111827; border-color: #e5e7eb; box-shadow: 0 1px 2px rgba(0,0,0,0.05); }

        .tab-close {
            width: 16px; height: 16px;
            display: flex; align-items: center; justify-content: center;
            border-radius: 4px; opacity: 0; transition: all 0.15s;
        }
        .tab:hover .tab-close { opacity: 0.6; }
        .tab-close:hover { opacity: 1 !important; background: rgba(255,255,255,0.1); }
        body.light .tab-close:hover { background: rgba(0,0,0,0.1); }

        .new-tab-btn {
            width: 28px; height: 28px;
            display: flex; align-items: center; justify-content: center;
            border-radius: 6px; cursor: pointer;
            transition: all 0.15s; flex-shrink: 0;
        }
        body.dark .new-tab-btn { color: #9ca3af; }
        body.light .new-tab-btn { color: #6b7280; }
        body.dark .new-tab-btn:hover { background: #3c3c3c; color: #ffffff; }
        body.light .new-tab-btn:hover { background: #e5e7eb; color: #111827; }

        .terminal-container {
            height: calc(100%% - 100px);
            padding: 8px;
            position: relative;
        }
        body.dark .terminal-container { background: #1e1e1e; }
        body.light .terminal-container { background: #ffffff; }

        .terminal-pane { position: absolute; inset: 8px; display: none; }
        .terminal-pane.active { display: block; }

        .connection-status {
            position: fixed; bottom: 16px; right: 16px;
            padding: 8px 16px; border-radius: 8px;
            font-size: 13px; font-weight: 500; z-index: 100;
            transition: all 0.3s;
        }
        .connection-status.connected { background: #065f46; color: #d1fae5; }
        .connection-status.disconnected { background: #991b1b; color: #fecaca; }
        .connection-status.connecting { background: #92400e; color: #fef3c7; }
        .connection-status.hidden { opacity: 0; pointer-events: none; }

        /* Mobile touch hints */
        @media (max-width: 640px) {
            .nav-breadcrumb { display: none; }
            .terminal-container { padding: 4px; }
            .terminal-pane { inset: 4px; }
        }
    </style>
</head>
<body class="dark">
    <header class="terminal-header">
        <div class="header-left">
            <a href="/" style="text-decoration: none;">
                <div class="logo-box"><img src="/favicon.webp" alt="Homeport"></div>
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
            <button class="header-btn" onclick="toggleTheme()" title="Toggle theme">
                <svg id="themeIcon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/>
                    <line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
                    <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/>
                    <line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
                    <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
                </svg>
            </button>
            <a href="/code/?folder=/home/coder/repos/%s" class="header-btn text">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>
                </svg>
                VS Code
            </a>
        </div>
    </header>

    <div class="tab-bar" id="tabBar">
        <div class="new-tab-btn" onclick="createTab()" title="New terminal">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
            </svg>
        </div>
    </div>

    <div class="terminal-container" id="terminalContainer"></div>
    <div id="connectionStatus" class="connection-status connecting">Connecting...</div>

    <script src="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/@xterm/addon-web-links@0.11.0/lib/addon-web-links.min.js"></script>
    <script>
        const REPO_ID = '%s';
        const STORAGE_KEY = 'homeport_terminal_' + REPO_ID;

        let tabs = [];
        let activeTabId = null;
        let theme = localStorage.getItem('terminal_theme') || 'dark';

        const darkTheme = {
            background: '#1e1e1e', foreground: '#d4d4d4', cursor: '#d4d4d4', cursorAccent: '#1e1e1e',
            selection: 'rgba(255, 255, 255, 0.3)', black: '#000000', red: '#cd3131', green: '#0dbc79',
            yellow: '#e5e510', blue: '#2472c8', magenta: '#bc3fbc', cyan: '#11a8cd', white: '#e5e5e5',
            brightBlack: '#666666', brightRed: '#f14c4c', brightGreen: '#23d18b', brightYellow: '#f5f543',
            brightBlue: '#3b8eea', brightMagenta: '#d670d6', brightCyan: '#29b8db', brightWhite: '#ffffff'
        };

        const lightTheme = {
            background: '#ffffff', foreground: '#1e1e1e', cursor: '#1e1e1e', cursorAccent: '#ffffff',
            selection: 'rgba(0, 0, 0, 0.2)', black: '#000000', red: '#cd3131', green: '#00bc7c',
            yellow: '#949800', blue: '#0451a5', magenta: '#bc05bc', cyan: '#0598bc', white: '#555555',
            brightBlack: '#666666', brightRed: '#cd3131', brightGreen: '#14ce14', brightYellow: '#b5ba00',
            brightBlue: '#0451a5', brightMagenta: '#bc05bc', brightCyan: '#0598bc', brightWhite: '#1e1e1e'
        };

        function applyTheme() {
            document.body.className = theme;
            tabs.forEach(tab => {
                if (tab.term) {
                    tab.term.options.theme = theme === 'dark' ? darkTheme : lightTheme;
                }
            });
            const icon = document.getElementById('themeIcon');
            if (theme === 'dark') {
                icon.innerHTML = '<circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>';
            } else {
                icon.innerHTML = '<path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>';
            }
        }

        function toggleTheme() {
            theme = theme === 'dark' ? 'light' : 'dark';
            localStorage.setItem('terminal_theme', theme);
            applyTheme();
        }

        function updateStatus(status, message) {
            const el = document.getElementById('connectionStatus');
            el.className = 'connection-status ' + status;
            el.textContent = message;
            if (status === 'connected') setTimeout(() => el.classList.add('hidden'), 2000);
            else el.classList.remove('hidden');
        }

        function saveSessions() {
            const sessionIds = tabs.map(t => t.sessionId).filter(Boolean);
            localStorage.setItem(STORAGE_KEY, JSON.stringify(sessionIds));
        }

        function loadSessions() {
            try {
                const saved = localStorage.getItem(STORAGE_KEY);
                return saved ? JSON.parse(saved) : [];
            } catch { return []; }
        }

        function renderTabs() {
            const tabBar = document.getElementById('tabBar');
            const newBtn = tabBar.querySelector('.new-tab-btn');
            tabBar.querySelectorAll('.tab').forEach(el => el.remove());

            tabs.forEach((tab, i) => {
                const el = document.createElement('div');
                el.className = 'tab' + (tab.id === activeTabId ? ' active' : '');
                el.innerHTML = '<span>Terminal ' + (i + 1) + '</span><div class="tab-close" onclick="event.stopPropagation(); closeTab(\'' + tab.id + '\')"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></div>';
                el.onclick = () => switchTab(tab.id);
                tabBar.insertBefore(el, newBtn);
            });
        }

        function createTab(sessionId = null) {
            const id = 'tab_' + Date.now();
            const pane = document.createElement('div');
            pane.id = 'pane_' + id;
            pane.className = 'terminal-pane';
            document.getElementById('terminalContainer').appendChild(pane);

            const term = new Terminal({
                cursorBlink: true, fontSize: 14,
                fontFamily: 'Menlo, Monaco, "Courier New", monospace',
                theme: theme === 'dark' ? darkTheme : lightTheme
            });

            const fitAddon = new FitAddon.FitAddon();
            term.loadAddon(fitAddon);
            term.loadAddon(new WebLinksAddon.WebLinksAddon());
            term.open(pane);
            fitAddon.fit();

            const tab = { id, term, fitAddon, pane, ws: null, sessionId, reconnectAttempts: 0 };
            tabs.push(tab);

            term.onData(data => {
                if (tab.ws && tab.ws.readyState === WebSocket.OPEN) {
                    tab.ws.send(JSON.stringify({ type: 'input', data }));
                }
            });

            connectTab(tab, sessionId);
            switchTab(id);
            renderTabs();
            return tab;
        }

        function connectTab(tab, sessionId = null) {
            const wsUrl = (location.protocol === 'https:' ? 'wss:' : 'ws:') + '//' + location.host + '/api/terminal/' + REPO_ID + (sessionId ? '?session=' + sessionId : '');
            updateStatus('connecting', 'Connecting...');

            tab.ws = new WebSocket(wsUrl);
            tab.ws.binaryType = 'arraybuffer';

            tab.ws.onopen = () => {
                tab.reconnectAttempts = 0;
                updateStatus('connected', 'Connected');
                tab.ws.send(JSON.stringify({ type: 'resize', cols: tab.term.cols, rows: tab.term.rows }));
            };

            tab.ws.onmessage = (e) => {
                if (e.data instanceof ArrayBuffer) {
                    tab.term.write(new Uint8Array(e.data));
                } else {
                    try {
                        const msg = JSON.parse(e.data);
                        if (msg.type === 'session' && msg.id) {
                            tab.sessionId = msg.id;
                            saveSessions();
                        }
                    } catch { tab.term.write(e.data); }
                }
            };

            tab.ws.onclose = () => {
                if (tab.reconnectAttempts < 5) {
                    tab.reconnectAttempts++;
                    updateStatus('connecting', 'Reconnecting (' + tab.reconnectAttempts + '/5)...');
                    setTimeout(() => connectTab(tab, tab.sessionId), 1000 * tab.reconnectAttempts);
                } else {
                    updateStatus('disconnected', 'Connection lost');
                }
            };
        }

        function switchTab(id) {
            activeTabId = id;
            tabs.forEach(tab => {
                tab.pane.classList.toggle('active', tab.id === id);
                if (tab.id === id) {
                    tab.fitAddon.fit();
                    tab.term.focus();
                    if (tab.ws && tab.ws.readyState === WebSocket.OPEN) {
                        tab.ws.send(JSON.stringify({ type: 'resize', cols: tab.term.cols, rows: tab.term.rows }));
                    }
                }
            });
            renderTabs();
        }

        function closeTab(id) {
            const idx = tabs.findIndex(t => t.id === id);
            if (idx === -1) return;

            const tab = tabs[idx];
            if (tab.ws) tab.ws.close();
            tab.pane.remove();
            tabs.splice(idx, 1);

            if (tabs.length === 0) {
                createTab();
            } else if (activeTabId === id) {
                switchTab(tabs[Math.max(0, idx - 1)].id);
            }
            saveSessions();
            renderTabs();
        }

        window.addEventListener('resize', () => {
            tabs.forEach(tab => {
                tab.fitAddon.fit();
                if (tab.ws && tab.ws.readyState === WebSocket.OPEN) {
                    tab.ws.send(JSON.stringify({ type: 'resize', cols: tab.term.cols, rows: tab.term.rows }));
                }
            });
        });

        // Mobile swipe gesture for tab switching
        let touchStartX = 0;
        document.addEventListener('touchstart', e => { touchStartX = e.touches[0].clientX; }, { passive: true });
        document.addEventListener('touchend', e => {
            const diff = e.changedTouches[0].clientX - touchStartX;
            if (Math.abs(diff) > 100) {
                const currentIdx = tabs.findIndex(t => t.id === activeTabId);
                if (diff > 0 && currentIdx > 0) switchTab(tabs[currentIdx - 1].id);
                else if (diff < 0 && currentIdx < tabs.length - 1) switchTab(tabs[currentIdx + 1].id);
            }
        }, { passive: true });

        // Initialize
        applyTheme();
        const savedSessions = loadSessions();
        if (savedSessions.length > 0) {
            savedSessions.forEach(sid => createTab(sid));
        } else {
            createTab();
        }
    </script>
</body>
</html>`, repo.Name, repo.Name, repo.Name, repoID)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(page))
}
