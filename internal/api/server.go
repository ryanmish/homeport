package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/gethomeport/homeport/internal/auth"
	"github.com/gethomeport/homeport/internal/config"
	"github.com/gethomeport/homeport/internal/github"
	"github.com/gethomeport/homeport/internal/process"
	"github.com/gethomeport/homeport/internal/proxy"
	"github.com/gethomeport/homeport/internal/scanner"
	"github.com/gethomeport/homeport/internal/share"
	"github.com/gethomeport/homeport/internal/store"
)

// generateID creates a random 8-character hex ID
func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type Server struct {
	cfg      *config.Config
	store    *store.Store
	scanner  *scanner.Scanner
	github   *github.Client
	procs    *process.Manager
	auth     *auth.Auth
	router   chi.Router
	stopScan chan struct{}
}

func NewServer(cfg *config.Config, st *store.Store) *Server {
	s := &Server{
		cfg:      cfg,
		store:    st,
		scanner:  scanner.New(cfg.PortRangeMin, cfg.PortRangeMax, cfg.ReposDir),
		github:   github.NewClient(cfg.ReposDir),
		procs:    process.NewManager(),
		auth:     auth.New(cfg.PasswordHash, cfg.CookieSecret),
		stopScan: make(chan struct{}),
	}

	s.setupRouter()
	return s
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(corsMiddleware)

	// Public routes (no auth required)
	r.Get("/login", s.handleLoginPage)
	r.Post("/login", s.handleLogin)
	r.Get("/logout", s.handleLogout)

	// Dynamic port proxy - handles its own auth via portAuthMiddleware
	// Must be outside protected group so public/password ports work without Homeport login
	r.Route("/{port:[0-9]+}", func(r chi.Router) {
		r.Use(s.portAuthMiddleware)
		r.HandleFunc("/*", s.handleProxyDirect)
		r.HandleFunc("/", s.handleProxyDirect)
	})

	// Protected routes (auth required)
	r.Group(func(r chi.Router) {
		r.Use(s.auth.Middleware)

		// API routes
		r.Route("/api", func(r chi.Router) {
			r.Get("/status", s.handleStatus)
			r.Get("/ports", s.handleListPorts)
			r.Get("/access-logs", s.handleAccessLogs)
			r.Get("/access-logs/{port}", s.handlePortAccessLogs)

			r.Route("/repos", func(r chi.Router) {
				r.Get("/", s.handleListRepos)
				r.Post("/", s.handleCloneRepo)
				r.Post("/init", s.handleInitRepo)
				r.Delete("/{id}", s.handleDeleteRepo)
				r.Patch("/{id}", s.handleUpdateRepo)
				r.Post("/{id}/pull", s.handlePullRepo)
				r.Get("/{id}/status", s.handleGetRepoStatus)
				r.Get("/{id}/info", s.handleGetRepoInfo)
				r.Get("/{id}/branches", s.handleListBranches)
				r.Post("/{id}/checkout", s.handleCheckoutBranch)
				r.Post("/{id}/exec", s.handleExecCommand)
				r.Post("/{id}/commit", s.handleGitCommit)
				r.Post("/{id}/push", s.handleGitPush)
			})

			r.Route("/github", func(r chi.Router) {
				r.Get("/repos", s.handleGitHubRepos)
				r.Get("/search", s.handleGitHubSearch)
				r.Get("/status", s.handleGitHubStatus)
			})

			r.Route("/share", func(r chi.Router) {
				r.Post("/{port}", s.handleSharePort)
				r.Delete("/{port}", s.handleUnsharePort)
			})

			r.Route("/processes", func(r chi.Router) {
				r.Get("/", s.handleListProcesses)
				r.Post("/{repoId}/start", s.handleStartProcess)
				r.Post("/{repoId}/stop", s.handleStopProcess)
				r.Get("/{repoId}/logs", s.handleGetProcessLogs)
			})

			r.Get("/version", s.handleVersion)
			r.Get("/updates", s.handleCheckUpdates)
			r.Get("/activity", s.handleGetActivity)

			// Auth management endpoints
			r.Post("/auth/change-password", s.handleChangePassword)
		})

		// Code Server proxy at /code/*
		r.Route("/code", func(r chi.Router) {
			r.HandleFunc("/*", s.handleCodeServerProxy)
			r.HandleFunc("/", s.handleCodeServerProxy)
		})

		// Serve UI at root
		r.Get("/", s.handleServeUI)
	})

	// Referer-based fallback for SPA assets (catch-all, must be last)
	// Handles requests like /_next/static/... that don't have a port prefix
	// Uses same auth logic as port proxy based on the port's sharing mode
	r.HandleFunc("/*", s.handleRefererFallback)

	s.router = r
}

// refererPortRegex matches /{port}/ or /{port} in URLs
var refererPortRegex = regexp.MustCompile(`/(\d+)(?:/|$)`)

// handleServeUI serves the main UI index.html
func (s *Server) handleServeUI(w http.ResponseWriter, r *http.Request) {
	// Clear any port context cookie when visiting root
	http.SetCookie(w, &http.Cookie{
		Name:     "homeport_ctx",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Serve index.html from UI directory
	if s.cfg.UIDir != "" {
		http.ServeFile(w, r, s.cfg.UIDir+"/index.html")
		return
	}
	http.NotFound(w, r)
}

// handleRefererFallback routes requests without a port prefix based on Referer header or context cookie
func (s *Server) handleRefererFallback(w http.ResponseWriter, r *http.Request) {
	var port int

	// First, try to extract port from Referer
	referer := r.Header.Get("Referer")
	if referer != "" {
		matches := refererPortRegex.FindStringSubmatch(referer)
		if len(matches) >= 2 {
			if p, err := strconv.Atoi(matches[1]); err == nil {
				port = p
			}
		}
	}

	// If no port from Referer, try the context cookie
	// This handles nested imports where Referer loses the port context
	if port == 0 {
		if cookie, err := r.Cookie("homeport_ctx"); err == nil {
			if p, err := strconv.Atoi(cookie.Value); err == nil {
				port = p
			}
		}
	}

	// Skip system ports to avoid infinite loops (8080 = homeportd, 8443 = code-server)
	if port == 8080 || port == 8443 {
		port = 0
	}

	// If still no port, serve UI
	if port == 0 {
		// Serve built UI from configured directory
		if s.cfg.UIDir != "" {
			http.FileServer(http.Dir(s.cfg.UIDir)).ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
		return
	}

	// Check if this port exists and get its share mode
	portInfo, err := s.store.GetPort(port)
	if err != nil {
		// Port not tracked - still proxy it (it might be a valid dev server)
		log.Printf("Referer-based proxy to untracked port %d for %s", port, r.URL.Path)
		proxy.HandlerDirect(port).ServeHTTP(w, r)
		return
	}

	// Check authentication based on share mode
	// Note: For Referer-based requests, we're more lenient because the user
	// already authenticated when they accessed the main page
	switch portInfo.ShareMode {
	case "public":
		// Anyone can access
	case "password":
		// Check for valid session cookie
		if !share.ValidateAuthCookie(r, port) {
			// For assets, we can't show a login form, so just return 401
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	case "private":
		// Check for valid homeport session cookie
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil || !s.auth.ValidateSession(cookie.Value) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	log.Printf("Referer-based proxy: %s -> port %d", r.URL.Path, port)
	proxy.HandlerDirect(port).ServeHTTP(w, r)
}

// handleCodeServerProxy serves a wrapper page with navigation header,
// or proxies requests to code-server
func (s *Server) handleCodeServerProxy(w http.ResponseWriter, r *http.Request) {
	// Get the path after /code
	path := strings.TrimPrefix(r.URL.Path, "/code")

	// If requesting the root (with or without query params), serve the wrapper page
	// The wrapper iframe will use ?_wrapped=1 to indicate it's the inner iframe
	if (path == "" || path == "/") && r.URL.Query().Get("_wrapped") != "1" {
		s.serveCodeServerWrapper(w, r)
		return
	}

	// Otherwise proxy to code-server (removes _wrapped param if present)
	proxy.HandlerWithBase(8443, "/code").ServeHTTP(w, r)
}

// serveCodeServerWrapper serves an HTML page that wraps code-server with a nav header
func (s *Server) serveCodeServerWrapper(w http.ResponseWriter, r *http.Request) {
	// Build iframe URL preserving query params but adding _wrapped=1
	iframeSrc := "/code/?_wrapped=1"
	folder := r.URL.Query().Get("folder")
	if folder != "" {
		iframeSrc += "&folder=" + folder
	} else {
		iframeSrc += "&folder=/home/coder/repos"
		folder = "/home/coder/repos"
	}

	// Extract repo name from folder path
	repoName := ""
	if strings.Contains(folder, "/repos/") {
		parts := strings.Split(folder, "/repos/")
		if len(parts) > 1 {
			repoName = strings.Split(parts[1], "/")[0]
		}
	}

	wrapper := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>VS Code - Homeport</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        html, body { height: 100%%; overflow: hidden; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }

        .homeport-header {
            height: 56px;
            background: #ffffff;
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0 24px;
        }

        .header-left {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .logo-box {
            width: 36px;
            height: 36px;
            background: #111827;
            border-radius: 10px;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .logo-box svg {
            width: 20px;
            height: 20px;
            color: white;
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

        .server-controls {
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .server-status {
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 6px 12px;
            background: #f0fdf4;
            border: 1px solid #bbf7d0;
            border-radius: 8px;
            font-size: 13px;
            color: #166534;
        }

        .server-status .dot {
            width: 8px;
            height: 8px;
            background: #22c55e;
            border-radius: 50%%;
        }

        .server-status .port-num {
            font-family: monospace;
            font-weight: 600;
        }

        .header-btn {
            padding: 8px 12px;
            border-radius: 8px;
            font-size: 13px;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.15s;
            display: flex;
            align-items: center;
            gap: 6px;
            border: 1px solid #e5e7eb;
            background: white;
            color: #374151;
            text-decoration: none;
        }

        .header-btn:hover {
            background: #f9fafb;
            border-color: #d1d5db;
        }

        .header-btn.primary {
            background: #111827;
            color: white;
            border-color: #111827;
        }

        .header-btn.primary:hover {
            background: #374151;
        }

        .header-btn.danger {
            color: #dc2626;
            border-color: #fecaca;
        }

        .header-btn.danger:hover {
            background: #fef2f2;
        }

        .header-btn svg {
            width: 14px;
            height: 14px;
        }

        .start-server-btn {
            display: flex;
        }

        .server-running-controls {
            display: none;
            align-items: center;
            gap: 8px;
        }

        .server-running-controls.active {
            display: flex;
        }

        .start-server-btn.hidden {
            display: none;
        }

        iframe {
            width: 100%%;
            height: calc(100%% - 56px);
            border: none;
        }

        /* Toast notifications */
        .toast-container {
            position: fixed;
            top: 68px;
            right: 24px;
            z-index: 9999;
            display: flex;
            flex-direction: column;
            gap: 8px;
        }

        .toast {
            background: white;
            border: 1px solid #e5e7eb;
            border-radius: 8px;
            padding: 12px 16px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
            display: flex;
            flex-direction: column;
            gap: 8px;
            min-width: 320px;
            animation: slideIn 0.3s ease;
        }

        @keyframes slideIn {
            from { transform: translateX(100%%); opacity: 0; }
            to { transform: translateX(0); opacity: 1; }
        }

        .toast-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
        }

        .toast-title {
            font-weight: 600;
            font-size: 13px;
            color: #111827;
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .toast-title .dot {
            width: 8px;
            height: 8px;
            background: #10b981;
            border-radius: 50%%;
            animation: pulse 2s infinite;
        }

        @keyframes pulse {
            0%%, 100%% { opacity: 1; }
            50%% { opacity: 0.5; }
        }

        .toast-close {
            background: none;
            border: none;
            cursor: pointer;
            color: #9ca3af;
            padding: 4px;
        }

        .toast-close:hover {
            color: #6b7280;
        }

        .toast-body {
            font-size: 12px;
            color: #6b7280;
        }

        .toast-actions {
            display: flex;
            gap: 8px;
            margin-top: 4px;
        }

        .toast-btn {
            padding: 6px 12px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.15s;
            border: 1px solid #e5e7eb;
            background: white;
            color: #374151;
        }

        .toast-btn:hover {
            background: #f9fafb;
        }

        .toast-btn.primary {
            background: #3b82f6;
            color: white;
            border-color: #3b82f6;
        }

        .toast-btn.primary:hover {
            background: #2563eb;
        }

        /* Share modal */
        .modal-overlay {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0,0,0,0.5);
            display: flex;
            align-items: center;
            justify-content: center;
            z-index: 10000;
        }

        .modal {
            background: white;
            border-radius: 12px;
            padding: 20px;
            min-width: 360px;
            box-shadow: 0 8px 24px rgba(0,0,0,0.2);
        }

        .modal-title {
            font-size: 16px;
            font-weight: 600;
            margin-bottom: 16px;
        }

        .share-options {
            display: flex;
            flex-direction: column;
            gap: 8px;
            margin-bottom: 16px;
        }

        .share-option {
            padding: 12px;
            border: 1px solid #e5e7eb;
            border-radius: 8px;
            cursor: pointer;
            transition: all 0.15s;
        }

        .share-option:hover {
            border-color: #3b82f6;
        }

        .share-option.active {
            border-color: #3b82f6;
            background: #eff6ff;
        }

        .share-option-title {
            font-weight: 500;
            font-size: 13px;
        }

        .share-option-desc {
            font-size: 12px;
            color: #6b7280;
            margin-top: 2px;
        }

        .modal-actions {
            display: flex;
            justify-content: flex-end;
            gap: 8px;
        }
    </style>
</head>
<body>
    <header class="homeport-header">
        <div class="header-left">
            <a href="/" style="text-decoration: none;">
                <div class="logo-box">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>
                        <polyline points="9 22 9 12 15 12 15 22"/>
                    </svg>
                </div>
            </a>
            <div class="nav-breadcrumb">
                <a href="/">Dashboard</a>
                <span class="sep">/</span>
                <span class="current">%s</span>
            </div>
        </div>
        <div class="header-right">
            <div class="server-controls">
                <button class="header-btn primary start-server-btn" id="startServerBtn" onclick="startServer()">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polygon points="5 3 19 12 5 21 5 3"/>
                    </svg>
                    Start Server
                </button>
                <div class="server-running-controls" id="serverRunningControls">
                    <div class="server-status">
                        <span class="dot"></span>
                        <span class="port-num" id="portNum">:3000</span>
                    </div>
                    <button class="header-btn" id="copyBtn" onclick="copyPortUrl()" title="Copy URL">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                        </svg>
                    </button>
                    <button class="header-btn" id="openBtn" onclick="openPort()" title="Open in new tab">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
                            <polyline points="15 3 21 3 21 9"/>
                            <line x1="10" y1="14" x2="21" y2="3"/>
                        </svg>
                    </button>
                    <button class="header-btn" id="shareBtn" onclick="showShareModal(activePort.port, activePort.share_mode)" title="Share settings">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <circle cx="18" cy="5" r="3"/>
                            <circle cx="6" cy="12" r="3"/>
                            <circle cx="18" cy="19" r="3"/>
                            <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/>
                            <line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
                        </svg>
                    </button>
                    <button class="header-btn danger" id="stopBtn" onclick="stopServer()" title="Stop server">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
                        </svg>
                        Stop
                    </button>
                </div>
            </div>
            <a href="/" class="header-btn">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>
                </svg>
            </a>
        </div>
    </header>

    <div class="toast-container" id="toasts"></div>

    <iframe src="%s" allow="clipboard-read; clipboard-write"></iframe>

    <script>
        const REPO_NAME = '%s';
        const EXTERNAL_URL = '%s';
        let knownPorts = new Set();
        let dismissedPorts = new Set();

        // Poll for new ports
        let activePort = null;

        async function checkPorts() {
            try {
                const resp = await fetch('/api/ports', { credentials: 'include' });
                if (!resp.ok) return;
                const ports = await resp.json();

                // Filter to ports for this repo (or unknown ports in this folder's range)
                const relevantPorts = ports.filter(p => {
                    // Skip system ports (homeportd and code-server)
                    if (p.port === 8080 || p.port === 8443) return false;
                    // Include if it's for this repo or if repo is missing/empty and it's a dev port
                    return p.repo_id === REPO_NAME ||
                           (!p.repo_id && p.port >= 3000 && p.port <= 9999);
                });

                // Update header controls
                const startBtn = document.getElementById('startServerBtn');
                const runningControls = document.getElementById('serverRunningControls');
                const portNum = document.getElementById('portNum');

                if (relevantPorts.length > 0) {
                    activePort = relevantPorts[0];
                    startBtn.classList.add('hidden');
                    runningControls.classList.add('active');
                    portNum.textContent = ':' + activePort.port;
                } else {
                    activePort = null;
                    startBtn.classList.remove('hidden');
                    runningControls.classList.remove('active');
                }

                for (const port of relevantPorts) {
                    if (!knownPorts.has(port.port) && !dismissedPorts.has(port.port)) {
                        knownPorts.add(port.port);
                        showPortToast(port);
                    }
                }

                // Remove ports that are no longer running
                const currentPorts = new Set(relevantPorts.map(p => p.port));
                knownPorts = new Set([...knownPorts].filter(p => currentPorts.has(p)));
            } catch (e) {
                console.error('Port check failed:', e);
            }
        }

        function showPortToast(port) {
            const container = document.getElementById('toasts');
            const toast = document.createElement('div');
            toast.className = 'toast';
            toast.id = 'toast-' + port.port;

            const url = EXTERNAL_URL + '/' + port.port + '/';
            const modeText = port.share_mode === 'private' ? 'privately' :
                            port.share_mode === 'public' ? 'publicly' : 'with password';

            toast.innerHTML = ` + "`" + `
                <div class="toast-header">
                    <div class="toast-title">
                        <span class="dot"></span>
                        Port ${port.port} is running
                    </div>
                    <button class="toast-close" onclick="dismissToast(${port.port})">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M18 6L6 18M6 6l12 12"/>
                        </svg>
                    </button>
                </div>
                <div class="toast-body">
                    ${port.process_name || 'Dev server'} • Currently accessible ${modeText}
                </div>
                <div class="toast-actions">
                    <button class="toast-btn primary" onclick="window.open('${url}', '_blank')">
                        Open
                    </button>
                    <button class="toast-btn" onclick="copyUrl('${url}')">
                        Copy URL
                    </button>
                    <button class="toast-btn" onclick="showShareModal(${port.port}, '${port.share_mode}')">
                        Share Settings
                    </button>
                </div>
            ` + "`" + `;

            container.appendChild(toast);

            // Auto-dismiss after 30 seconds
            setTimeout(() => {
                if (document.getElementById('toast-' + port.port)) {
                    dismissToast(port.port);
                }
            }, 30000);
        }

        function dismissToast(port) {
            const toast = document.getElementById('toast-' + port);
            if (toast) {
                dismissedPorts.add(port);
                toast.remove();
            }
        }

        function copyUrl(url) {
            navigator.clipboard.writeText(url);
        }

        function copyPortUrl() {
            if (!activePort) return;
            const url = EXTERNAL_URL + '/' + activePort.port + '/';
            navigator.clipboard.writeText(url);
        }

        function openPort() {
            if (!activePort) return;
            window.open('/' + activePort.port + '/', '_blank');
        }

        async function startServer() {
            // Find repo ID from repo name
            try {
                const resp = await fetch('/api/repos', { credentials: 'include' });
                if (!resp.ok) return;
                const repos = await resp.json();
                const repo = repos.find(r => r.name === REPO_NAME);
                if (!repo) {
                    alert('Repository not found. Please configure a start command in the dashboard.');
                    return;
                }
                if (!repo.start_command) {
                    alert('No start command configured. Please set one in the dashboard.');
                    return;
                }
                await fetch('/api/repos/' + repo.id + '/start', {
                    method: 'POST',
                    credentials: 'include'
                });
            } catch (e) {
                console.error('Failed to start server:', e);
            }
        }

        async function stopServer() {
            if (!activePort) return;
            try {
                const resp = await fetch('/api/repos', { credentials: 'include' });
                if (!resp.ok) return;
                const repos = await resp.json();
                const repo = repos.find(r => r.name === REPO_NAME);
                if (repo) {
                    await fetch('/api/repos/' + repo.id + '/stop', {
                        method: 'POST',
                        credentials: 'include'
                    });
                }
            } catch (e) {
                console.error('Failed to stop server:', e);
            }
        }

        let shareModal = null;

        function showShareModal(port, currentMode) {
            if (shareModal) shareModal.remove();

            const overlay = document.createElement('div');
            overlay.className = 'modal-overlay';
            overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };

            overlay.innerHTML = ` + "`" + `
                <div class="modal">
                    <div class="modal-title">Share Port ${port}</div>
                    <div class="share-options">
                        <div class="share-option ${currentMode === 'private' ? 'active' : ''}" onclick="selectShareMode(this, 'private')">
                            <div class="share-option-title">Private</div>
                            <div class="share-option-desc">Only accessible when logged into Homeport</div>
                        </div>
                        <div class="share-option ${currentMode === 'password' ? 'active' : ''}" onclick="selectShareMode(this, 'password')">
                            <div class="share-option-title">Password</div>
                            <div class="share-option-desc">Anyone with the password can access</div>
                        </div>
                        <div class="share-option ${currentMode === 'public' ? 'active' : ''}" onclick="selectShareMode(this, 'public')">
                            <div class="share-option-title">Public</div>
                            <div class="share-option-desc">Anyone with the link can access</div>
                        </div>
                    </div>
                    <div class="modal-actions">
                        <button class="toast-btn" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
                        <button class="toast-btn primary" onclick="applyShareMode(${port})">Apply</button>
                    </div>
                </div>
            ` + "`" + `;

            document.body.appendChild(overlay);
            shareModal = overlay;
        }

        let selectedMode = null;

        function selectShareMode(el, mode) {
            document.querySelectorAll('.share-option').forEach(o => o.classList.remove('active'));
            el.classList.add('active');
            selectedMode = mode;
        }

        async function applyShareMode(port) {
            if (!selectedMode) {
                shareModal.remove();
                return;
            }

            try {
                let body = { mode: selectedMode };
                if (selectedMode === 'password') {
                    const pw = prompt('Enter share password:');
                    if (!pw) return;
                    body.password = pw;
                }

                await fetch('/api/ports/' + port + '/share', {
                    method: 'POST',
                    credentials: 'include',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });

                shareModal.remove();
                // Refresh the toast to show new mode
                dismissedPorts.delete(port);
                knownPorts.delete(port);
                checkPorts();
            } catch (e) {
                console.error('Failed to update share mode:', e);
            }
        }

        // Start polling
        checkPorts();
        setInterval(checkPorts, 3000);
    </script>
</body>
</html>`, repoName, iframeSrc, repoName, s.cfg.ExternalURL)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(wrapper))
}

// handleProxyDirect proxies requests to localhost:{port}
// Auth is already handled by portAuthMiddleware
func (s *Server) handleProxyDirect(w http.ResponseWriter, r *http.Request) {
	portStr := chi.URLParam(r, "port")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "Invalid port", http.StatusBadRequest)
		return
	}

	// Set context cookie for Referer-based routing of nested imports
	// This cookie tells the fallback handler which port to route to when
	// the Referer header loses the port context (e.g., nested module imports)
	http.SetCookie(w, &http.Cookie{
		Name:     "homeport_ctx",
		Value:    portStr,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Proxy the request
	proxy.Handler(port).ServeHTTP(w, r)
}

func (s *Server) Router() chi.Router {
	return s.router
}

func (s *Server) Start() error {
	// Start background port scanner
	go s.scanLoop()

	// Sync existing repos from filesystem
	if err := s.syncReposFromFilesystem(); err != nil {
		log.Printf("Warning: failed to sync repos: %v", err)
	}

	log.Printf("Starting server on %s", s.cfg.ListenAddr)
	return http.ListenAndServe(s.cfg.ListenAddr, s.router)
}

func (s *Server) Stop() {
	close(s.stopScan)
}

func (s *Server) scanLoop() {
	ticker := time.NewTicker(time.Duration(s.cfg.ScanInterval) * time.Second)
	defer ticker.Stop()

	// Initial scan
	s.doScan()

	for {
		select {
		case <-ticker.C:
			s.doScan()
		case <-s.stopScan:
			return
		}
	}
}

func (s *Server) doScan() {
	ports, err := s.scanner.Scan()
	if err != nil {
		log.Printf("Scan error: %v", err)
		return
	}

	// Update database
	for _, p := range ports {
		// Check if port already exists to preserve share settings
		existing, err := s.store.GetPort(p.Port)
		if err == nil {
			p.ShareMode = existing.ShareMode
			p.FirstSeen = existing.FirstSeen
		}
		if err := s.store.UpsertPort(&p); err != nil {
			log.Printf("Failed to upsert port %d: %v", p.Port, err)
		}
	}

	// Clean up stale ports (not seen in last 30 seconds)
	staleThreshold := time.Now().Add(-30 * time.Second)
	if err := s.store.CleanupStalePorts(staleThreshold); err != nil {
		log.Printf("Failed to cleanup stale ports: %v", err)
	}
}

func (s *Server) syncReposFromFilesystem() error {
	// Walk reposDir and find git repositories
	entries, err := os.ReadDir(s.cfg.ReposDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No repos dir yet, that's fine
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		repoPath := filepath.Join(s.cfg.ReposDir, entry.Name())
		gitDir := filepath.Join(repoPath, ".git")

		// Check if it's a git repo
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			continue
		}

		// Check if already in database
		repos, err := s.store.ListRepos()
		if err != nil {
			log.Printf("Warning: failed to list repos: %v", err)
			continue
		}

		exists := false
		for _, r := range repos {
			if r.Path == repoPath {
				exists = true
				break
			}
		}

		if !exists {
			// Try to get GitHub URL from git remote
			githubURL := s.github.GetRemoteURL(repoPath)

			repo := &store.Repo{
				ID:        generateID(),
				Name:      entry.Name(),
				Path:      repoPath,
				GitHubURL: githubURL,
			}

			if err := s.store.CreateRepo(repo); err != nil {
				log.Printf("Warning: failed to add repo %s: %v", entry.Name(), err)
			} else {
				log.Printf("Synced existing repo: %s", entry.Name())
			}
		}
	}

	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// contextKey type for context values
type contextKey string

const portContextKey contextKey = "port"

func withPort(ctx context.Context, port int) context.Context {
	return context.WithValue(ctx, portContextKey, port)
}

func portFromContext(ctx context.Context) int {
	if v := ctx.Value(portContextKey); v != nil {
		return v.(int)
	}
	return 0
}
