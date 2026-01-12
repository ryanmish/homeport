package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/gethomeport/homeport/internal/stats"
	"github.com/gethomeport/homeport/internal/store"
)

// Response helpers

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

// Status endpoint

type StatusResponse struct {
	Status  string       `json:"status"`
	Version string       `json:"version"`
	Uptime  string       `json:"uptime"`
	Stats   *stats.Stats `json:"stats"`
	Config  StatusConfig `json:"config"`
}

type StatusConfig struct {
	PortRange   string `json:"port_range"`
	ExternalURL string `json:"external_url"`
	DevMode     bool   `json:"dev_mode"`
}

var startTime = time.Now()

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	sysStats := stats.Get()

	resp := StatusResponse{
		Status:  "ok",
		Version: "0.1.0",
		Uptime:  time.Since(startTime).Round(time.Second).String(),
		Stats:   sysStats,
		Config: StatusConfig{
			PortRange:   strconv.Itoa(s.cfg.PortRangeMin) + "-" + strconv.Itoa(s.cfg.PortRangeMax),
			ExternalURL: s.cfg.ExternalURL,
			DevMode:     s.cfg.DevMode,
		},
	}

	jsonResponse(w, http.StatusOK, resp)
}

// Port endpoints

func (s *Server) handleListPorts(w http.ResponseWriter, r *http.Request) {
	ports, err := s.store.ListPorts()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if ports == nil {
		ports = []store.Port{}
	}

	jsonResponse(w, http.StatusOK, ports)
}

// Repo endpoints

func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := s.store.ListRepos()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if repos == nil {
		repos = []store.Repo{}
	}

	// Also get running ports for each repo
	ports, _ := s.store.ListPorts()
	portsByRepo := make(map[string][]store.Port)
	for _, p := range ports {
		if p.RepoID != "" {
			portsByRepo[p.RepoID] = append(portsByRepo[p.RepoID], p)
		}
	}

	type RepoWithPorts struct {
		store.Repo
		Ports []store.Port `json:"ports"`
	}

	result := make([]RepoWithPorts, len(repos))
	for i, repo := range repos {
		result[i] = RepoWithPorts{
			Repo:  repo,
			Ports: portsByRepo[repo.ID],
		}
		if result[i].Ports == nil {
			result[i].Ports = []store.Port{}
		}
	}

	jsonResponse(w, http.StatusOK, result)
}

type CloneRequest struct {
	RepoFullName string `json:"repo"` // e.g., "owner/repo"
}

func (s *Server) handleCloneRepo(w http.ResponseWriter, r *http.Request) {
	var req CloneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RepoFullName == "" {
		errorResponse(w, http.StatusBadRequest, "repo is required")
		return
	}

	// Clone the repo
	localPath, err := s.github.Clone(req.RepoFullName)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get the repo name from the path
	repoName := filepath.Base(localPath)

	// Get GitHub URL
	githubURL, _ := s.github.GetRepoURL(localPath)

	// Add to database
	repo := &store.Repo{
		ID:        repoName,
		Name:      repoName,
		Path:      localPath,
		GitHubURL: githubURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.store.CreateRepo(repo); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, repo)
}

func (s *Server) handleDeleteRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repo, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	// Delete from filesystem
	if err := os.RemoveAll(repo.Path); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Delete from database
	if err := s.store.DeleteRepo(id); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePullRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repo, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	if err := s.github.Pull(repo.Path); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "pulled"})
}

// GitHub endpoints

func (s *Server) handleGitHubStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := s.github.IsAuthenticated()
	jsonResponse(w, http.StatusOK, map[string]bool{"authenticated": authenticated})
}

func (s *Server) handleGitHubRepos(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	repos, err := s.github.ListRepos(limit)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, repos)
}

func (s *Server) handleGitHubSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		errorResponse(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	repos, err := s.github.Search(query, limit)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, repos)
}

// Share endpoints

type ShareRequest struct {
	Mode      string `json:"mode"`       // "private", "password", "public"
	Password  string `json:"password"`   // required if mode is "password"
	ExpiresIn string `json:"expires_in"` // optional: "1h", "24h", "7d", "30d", or empty for never
}

func (s *Server) handleSharePort(w http.ResponseWriter, r *http.Request) {
	portStr := chi.URLParam(r, "port")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid port")
		return
	}

	var req ShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Mode == "" {
		req.Mode = "private"
	}

	if req.Mode != "private" && req.Mode != "password" && req.Mode != "public" {
		errorResponse(w, http.StatusBadRequest, "mode must be 'private', 'password', or 'public'")
		return
	}

	var passwordHash string
	if req.Mode == "password" {
		if req.Password == "" {
			errorResponse(w, http.StatusBadRequest, "password is required for password mode")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		passwordHash = string(hash)
	}

	// Parse expiration
	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		var duration time.Duration
		switch req.ExpiresIn {
		case "1h":
			duration = time.Hour
		case "24h":
			duration = 24 * time.Hour
		case "7d":
			duration = 7 * 24 * time.Hour
		case "30d":
			duration = 30 * 24 * time.Hour
		default:
			// Try parsing as a Go duration
			d, err := time.ParseDuration(req.ExpiresIn)
			if err != nil {
				errorResponse(w, http.StatusBadRequest, "invalid expires_in: use '1h', '24h', '7d', '30d', or a valid Go duration")
				return
			}
			duration = d
		}
		t := time.Now().Add(duration)
		expiresAt = &t
	}

	if err := s.store.UpdatePortShare(port, req.Mode, passwordHash, expiresAt); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return the shareable URL
	url := s.cfg.ExternalURL + "/" + portStr

	resp := map[string]interface{}{
		"status": "shared",
		"mode":   req.Mode,
		"url":    url,
	}
	if expiresAt != nil {
		resp["expires_at"] = expiresAt.Format(time.RFC3339)
	}

	jsonResponse(w, http.StatusOK, resp)
}

func (s *Server) handleUnsharePort(w http.ResponseWriter, r *http.Request) {
	portStr := chi.URLParam(r, "port")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid port")
		return
	}

	// Reset to private (default)
	if err := s.store.UpdatePortShare(port, "private", "", nil); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "unshared"})
}

