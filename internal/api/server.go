package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/gethomeport/homeport/internal/config"
	"github.com/gethomeport/homeport/internal/github"
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
	router   chi.Router
	stopScan chan struct{}
}

func NewServer(cfg *config.Config, st *store.Store) *Server {
	s := &Server{
		cfg:      cfg,
		store:    st,
		scanner:  scanner.New(cfg.PortRangeMin, cfg.PortRangeMax, cfg.ReposDir),
		github:   github.NewClient(cfg.ReposDir),
		stopScan: make(chan struct{}),
	}

	s.setupRouter()
	return s
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(corsMiddleware)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/status", s.handleStatus)
		r.Get("/ports", s.handleListPorts)

		r.Route("/repos", func(r chi.Router) {
			r.Get("/", s.handleListRepos)
			r.Post("/", s.handleCloneRepo)
			r.Delete("/{id}", s.handleDeleteRepo)
			r.Post("/{id}/pull", s.handlePullRepo)
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
	})

	// Dynamic port proxy - matches /{port}/* patterns
	// This must come after /api to avoid conflicts
	r.Route("/{port:[0-9]+}", func(r chi.Router) {
		r.Use(s.portAuthMiddleware)
		r.HandleFunc("/*", s.handleProxyDirect)
		r.HandleFunc("/", s.handleProxyDirect)
	})

	// Serve UI at root
	r.Get("/", s.handleServeUI)

	// Referer-based fallback for SPA assets
	// When a request comes in without a port prefix (e.g., /@vite/client),
	// check the Referer header to determine which port to proxy to.
	// This allows path-based routing to work with SPAs that use absolute paths.
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
		// Check for Cloudflare Access header in production
		if !s.cfg.DevMode {
			cfEmail := r.Header.Get("CF-Access-Authenticated-User-Email")
			if cfEmail == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
	}

	log.Printf("Referer-based proxy: %s -> port %d", r.URL.Path, port)
	proxy.HandlerDirect(port).ServeHTTP(w, r)
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
