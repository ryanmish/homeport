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

        /* Share dropdown */
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

        /* Share dropdown */
        .share-dropdown {
            z-index: 10001;
        }

        .share-dropdown-content {
            background: white;
            border: 1px solid #e5e7eb;
            border-radius: 12px;
            padding: 16px;
            min-width: 320px;
            box-shadow: 0 8px 24px rgba(0,0,0,0.15);
        }

        .share-label {
            font-size: 12px;
            font-weight: 500;
            color: #6b7280;
            margin-bottom: 8px;
        }

        .share-mode-buttons {
            display: flex;
            gap: 4px;
            margin-bottom: 12px;
        }

        .share-mode-btn {
            flex: 1;
            padding: 8px 12px;
            font-size: 13px;
            font-weight: 500;
            border: 1px solid #e5e7eb;
            background: #f9fafb;
            color: #374151;
            border-radius: 6px;
            cursor: pointer;
            transition: all 0.15s;
        }

        .share-mode-btn:hover {
            background: #f3f4f6;
            border-color: #d1d5db;
        }

        .share-mode-btn.active {
            background: #111827;
            color: white;
            border-color: #111827;
        }

        .share-password-field {
            margin-bottom: 12px;
        }

        .password-input-wrapper {
            position: relative;
            display: flex;
            align-items: center;
        }

        .password-input-wrapper input {
            width: 100%%;
            padding: 8px 40px 8px 12px;
            font-size: 13px;
            border: 1px solid #e5e7eb;
            border-radius: 8px;
            outline: none;
        }

        .password-input-wrapper input:focus {
            border-color: #9ca3af;
            box-shadow: 0 0 0 2px rgba(156, 163, 175, 0.2);
        }

        .password-toggle {
            position: absolute;
            right: 8px;
            background: none;
            border: none;
            cursor: pointer;
            padding: 4px;
            color: #9ca3af;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .password-toggle:hover {
            color: #6b7280;
        }

        .password-toggle svg {
            width: 16px;
            height: 16px;
        }

        .share-actions {
            display: flex;
            gap: 8px;
            padding-top: 8px;
            border-top: 1px solid #f3f4f6;
        }

        .share-action-btn {
            padding: 8px 12px;
            font-size: 13px;
            font-weight: 500;
            border-radius: 8px;
            cursor: pointer;
            transition: all 0.15s;
            border: 1px solid #e5e7eb;
        }

        .share-action-btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }

        .share-action-btn.cancel {
            background: white;
            color: #374151;
        }

        .share-action-btn.cancel:hover {
            background: #f9fafb;
        }

        .share-action-btn.copy {
            background: white;
            color: #374151;
            flex: 1;
        }

        .share-action-btn.copy:hover:not(:disabled) {
            background: #f9fafb;
        }

        .share-action-btn.apply {
            background: #111827;
            color: white;
            border-color: #111827;
        }

        .share-action-btn.apply:hover:not(:disabled) {
            background: #374151;
        }

        @keyframes spin {
            from { transform: rotate(0deg); }
            to { transform: rotate(360deg); }
        }

        .animate-spin {
            animation: spin 1s linear infinite;
        }

        /* Port selector */
        .port-selector {
            position: relative;
        }

        .server-status {
            cursor: pointer;
        }

        .port-selector-btn {
            display: none;
            align-items: center;
            gap: 4px;
            background: none;
            border: none;
            padding: 0;
            margin-left: 6px;
            cursor: pointer;
            color: #166534;
        }

        .port-selector-btn.visible {
            display: flex;
        }

        .port-selector-btn svg {
            width: 14px;
            height: 14px;
        }

        .port-selector-btn .port-count {
            font-size: 11px;
            font-weight: 600;
            background: #166534;
            color: white;
            padding: 1px 5px;
            border-radius: 10px;
        }

        .port-selector-dropdown {
            display: none;
            position: absolute;
            top: calc(100%% + 8px);
            left: 0;
            min-width: 220px;
            background: white;
            border: 1px solid #e5e7eb;
            border-radius: 10px;
            box-shadow: 0 8px 24px rgba(0,0,0,0.15);
            padding: 6px;
            z-index: 100;
        }

        .port-selector-dropdown.visible {
            display: block;
        }

        .port-option {
            display: flex;
            align-items: center;
            gap: 8px;
            width: 100%%;
            padding: 8px 10px;
            background: none;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            text-align: left;
            transition: background 0.15s;
        }

        .port-option:hover {
            background: #f3f4f6;
        }

        .port-option.active {
            background: #f0fdf4;
        }

        .port-option-num {
            font-family: monospace;
            font-weight: 600;
            font-size: 13px;
            color: #111827;
        }

        .port-option-name {
            flex: 1;
            font-size: 12px;
            color: #6b7280;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .port-option-mode {
            font-size: 10px;
            font-weight: 500;
            padding: 2px 6px;
            border-radius: 4px;
        }

        .port-mode-private {
            background: #f3f4f6;
            color: #6b7280;
        }

        .port-mode-password {
            background: #fef3c7;
            color: #92400e;
        }

        .port-mode-public {
            background: #d1fae5;
            color: #065f46;
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
                    <div class="port-selector">
                        <div class="server-status" onclick="togglePortSelector()">
                            <span class="dot"></span>
                            <span class="port-num" id="portNum">:3000</span>
                            <button class="port-selector-btn" id="portSelectorBtn">
                                <span class="port-count">2</span>
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                    <polyline points="6 9 12 15 18 9"/>
                                </svg>
                            </button>
                        </div>
                        <div class="port-selector-dropdown" id="portSelectorDropdown"></div>
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

    <iframe src="%s" allow="clipboard-read; clipboard-write"></iframe>

    <script>
        const REPO_NAME = '%s';
        const EXTERNAL_URL = '%s';

        // Track all running ports and the currently selected one
        let allPorts = [];
        let activePort = null;
        let showPortSelector = false;

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

                allPorts = relevantPorts;

                // Update header controls
                const startBtn = document.getElementById('startServerBtn');
                const runningControls = document.getElementById('serverRunningControls');
                const portNum = document.getElementById('portNum');
                const portSelector = document.getElementById('portSelectorBtn');

                if (relevantPorts.length > 0) {
                    // Keep current active port if still running, otherwise switch to first
                    if (!activePort || !relevantPorts.find(p => p.port === activePort.port)) {
                        activePort = relevantPorts[0];
                    } else {
                        // Update activePort data (share_mode might have changed)
                        activePort = relevantPorts.find(p => p.port === activePort.port);
                    }
                    startBtn.classList.add('hidden');
                    runningControls.classList.add('active');
                    portNum.textContent = ':' + activePort.port;

                    // Show/hide port selector based on number of ports
                    if (relevantPorts.length > 1) {
                        portSelector.classList.add('visible');
                        portSelector.querySelector('.port-count').textContent = relevantPorts.length;
                    } else {
                        portSelector.classList.remove('visible');
                    }
                } else {
                    activePort = null;
                    startBtn.classList.remove('hidden');
                    runningControls.classList.remove('active');
                    portSelector.classList.remove('visible');
                }
            } catch (e) {
                console.error('Port check failed:', e);
            }
        }

        function togglePortSelector() {
            showPortSelector = !showPortSelector;
            const dropdown = document.getElementById('portSelectorDropdown');
            if (showPortSelector) {
                renderPortSelector();
                dropdown.classList.add('visible');
            } else {
                dropdown.classList.remove('visible');
            }
        }

        function renderPortSelector() {
            const dropdown = document.getElementById('portSelectorDropdown');
            dropdown.innerHTML = allPorts.map(p => ` + "`" + `
                <button class="port-option ${p.port === activePort?.port ? 'active' : ''}" onclick="selectPort(${p.port})">
                    <span class="port-option-num">:${p.port}</span>
                    <span class="port-option-name">${p.process_name || 'Dev server'}</span>
                    <span class="port-option-mode port-mode-${p.share_mode}">${p.share_mode}</span>
                </button>
            ` + "`" + `).join('');
        }

        function selectPort(portNum) {
            activePort = allPorts.find(p => p.port === portNum);
            document.getElementById('portNum').textContent = ':' + activePort.port;
            togglePortSelector();
        }

        // Close dropdown when clicking outside
        document.addEventListener('click', (e) => {
            if (showPortSelector && !e.target.closest('.port-selector')) {
                showPortSelector = false;
                document.getElementById('portSelectorDropdown').classList.remove('visible');
            }
        });

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

        let isStarting = false;
        let isStopping = false;

        async function startServer() {
            if (isStarting) return;
            isStarting = true;
            const btn = document.getElementById('startServerBtn');
            btn.disabled = true;
            btn.innerHTML = '<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg> Starting...';

            try {
                const resp = await fetch('/api/repos', { credentials: 'include' });
                if (!resp.ok) { isStarting = false; return; }
                const repos = await resp.json();
                const repo = repos.find(r => r.name === REPO_NAME);
                if (!repo) {
                    alert('Repository not found. Please configure a start command in the dashboard.');
                    isStarting = false;
                    btn.disabled = false;
                    btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg> Start Server';
                    return;
                }
                if (!repo.start_command) {
                    alert('No start command configured. Please set one in the dashboard.');
                    isStarting = false;
                    btn.disabled = false;
                    btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg> Start Server';
                    return;
                }
                await fetch('/api/processes/' + repo.id + '/start', {
                    method: 'POST',
                    credentials: 'include'
                });
                // Immediate check for port
                setTimeout(checkPorts, 500);
                setTimeout(checkPorts, 1500);
            } catch (e) {
                console.error('Failed to start server:', e);
            } finally {
                isStarting = false;
                btn.disabled = false;
                btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg> Start Server';
            }
        }

        async function stopServer() {
            if (!activePort || isStopping) return;
            isStopping = true;
            const btn = document.getElementById('stopBtn');
            btn.disabled = true;
            btn.innerHTML = '<svg class="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>';

            try {
                const resp = await fetch('/api/repos', { credentials: 'include' });
                if (!resp.ok) { isStopping = false; return; }
                const repos = await resp.json();
                const repo = repos.find(r => r.name === REPO_NAME);
                if (repo) {
                    await fetch('/api/processes/' + repo.id + '/stop', {
                        method: 'POST',
                        credentials: 'include'
                    });
                    // Immediate check for port
                    setTimeout(checkPorts, 500);
                    setTimeout(checkPorts, 1500);
                }
            } catch (e) {
                console.error('Failed to stop server:', e);
            } finally {
                isStopping = false;
                btn.disabled = false;
                btn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/></svg> Stop';
            }
        }

        let shareDropdown = null;
        let selectedMode = null;
        let sharePassword = '';
        let showPassword = false;

        function showShareModal(port, currentMode) {
            if (shareDropdown) shareDropdown.remove();
            selectedMode = currentMode;
            sharePassword = '';
            showPassword = false;

            const dropdown = document.createElement('div');
            dropdown.className = 'share-dropdown';
            dropdown.innerHTML = renderShareDropdown(port, currentMode);

            // Position next to share button
            const shareBtn = document.getElementById('shareBtn');
            const rect = shareBtn.getBoundingClientRect();
            dropdown.style.position = 'fixed';
            dropdown.style.top = (rect.bottom + 8) + 'px';
            dropdown.style.right = (window.innerWidth - rect.right) + 'px';

            document.body.appendChild(dropdown);
            shareDropdown = dropdown;

            // Close on outside click
            setTimeout(() => {
                document.addEventListener('click', closeShareDropdown);
            }, 0);
        }

        function closeShareDropdown(e) {
            if (shareDropdown && !shareDropdown.contains(e.target) && !e.target.closest('#shareBtn')) {
                shareDropdown.remove();
                shareDropdown = null;
                document.removeEventListener('click', closeShareDropdown);
            }
        }

        function renderShareDropdown(port, currentMode) {
            return ` + "`" + `
                <div class="share-dropdown-content">
                    <div class="share-label">Sharing Mode</div>
                    <div class="share-mode-buttons">
                        <button class="share-mode-btn ${selectedMode === 'private' ? 'active' : ''}" onclick="selectShareMode('private', ${port})">Private</button>
                        <button class="share-mode-btn ${selectedMode === 'password' ? 'active' : ''}" onclick="selectShareMode('password', ${port})">Password</button>
                        <button class="share-mode-btn ${selectedMode === 'public' ? 'active' : ''}" onclick="selectShareMode('public', ${port})">Public</button>
                    </div>
                    ${selectedMode === 'password' ? ` + "`" + `
                        <div class="share-password-field">
                            <div class="share-label">Password</div>
                            <div class="password-input-wrapper">
                                <input type="${showPassword ? 'text' : 'password'}" id="sharePasswordInput" value="${sharePassword}" placeholder="Enter password..." oninput="sharePassword = this.value" />
                                <button class="password-toggle" onclick="togglePasswordVisibility(${port})">
                                    ${showPassword ? '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>' : '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>'}
                                </button>
                            </div>
                        </div>
                    ` + "`" + ` : ''}
                    <div class="share-actions">
                        <button class="share-action-btn cancel" onclick="shareDropdown.remove(); shareDropdown = null;">Cancel</button>
                        <button class="share-action-btn copy" onclick="applyShareMode(${port}, true)" ${selectedMode === 'password' && !sharePassword ? 'disabled' : ''}>Apply & Copy URL</button>
                        <button class="share-action-btn apply" onclick="applyShareMode(${port}, false)" ${selectedMode === 'password' && !sharePassword ? 'disabled' : ''}>Apply</button>
                    </div>
                </div>
            ` + "`" + `;
        }

        function togglePasswordVisibility(port) {
            showPassword = !showPassword;
            if (shareDropdown) {
                shareDropdown.innerHTML = renderShareDropdown(port, selectedMode);
                const input = document.getElementById('sharePasswordInput');
                if (input) {
                    input.value = sharePassword;
                    input.focus();
                }
            }
        }

        function selectShareMode(mode, port) {
            selectedMode = mode;
            if (shareDropdown) {
                shareDropdown.innerHTML = renderShareDropdown(port, mode);
            }
        }

        async function applyShareMode(port, copyUrl) {
            if (!selectedMode) {
                if (shareDropdown) shareDropdown.remove();
                shareDropdown = null;
                return;
            }

            try {
                let body = { mode: selectedMode };
                if (selectedMode === 'password') {
                    if (!sharePassword) return;
                    body.password = sharePassword;
                }

                await fetch('/api/share/' + port, {
                    method: 'POST',
                    credentials: 'include',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });

                if (copyUrl) {
                    const url = EXTERNAL_URL + '/' + port + '/';
                    navigator.clipboard.writeText(url);
                }

                if (shareDropdown) shareDropdown.remove();
                shareDropdown = null;
                document.removeEventListener('click', closeShareDropdown);

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
