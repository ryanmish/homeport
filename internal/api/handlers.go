package api

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/gethomeport/homeport/internal/activity"
	"github.com/gethomeport/homeport/internal/process"
	"github.com/gethomeport/homeport/internal/repo"
	"github.com/gethomeport/homeport/internal/stats"
	"github.com/gethomeport/homeport/internal/store"
	"github.com/gethomeport/homeport/internal/version"
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

	activity.LogClone(repoName)
	jsonResponse(w, http.StatusCreated, repo)
}

func (s *Server) handleInitRepo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		errorResponse(w, http.StatusBadRequest, "name is required")
		return
	}

	// Sanitize name - only allow alphanumeric, dash, underscore
	for _, c := range req.Name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			errorResponse(w, http.StatusBadRequest, "name can only contain letters, numbers, dashes, and underscores")
			return
		}
	}

	localPath, err := s.github.Init(req.Name)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Add to database
	repo := &store.Repo{
		ID:        generateID(),
		Name:      req.Name,
		Path:      localPath,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.store.CreateRepo(repo); err != nil {
		os.RemoveAll(localPath) // cleanup on failure
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

	activity.LogDelete(id, repo.Name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePullRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repo, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	result, err := s.github.PullWithDetails(repo.Path)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	activity.LogPull(id, repo.Name)
	jsonResponse(w, http.StatusOK, result)
}

func (s *Server) handleGetRepoStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repo, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	status, err := s.github.GetStatus(repo.Path)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, status)
}

func (s *Server) handleUpdateRepo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repo, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	var req struct {
		StartCommand string `json:"start_command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	repo.StartCommand = req.StartCommand
	repo.UpdatedAt = time.Now()

	if err := s.store.UpdateRepo(repo); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, repo)
}

// GitHub endpoints

func (s *Server) handleGitHubStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := s.github.IsAuthenticated()

	response := map[string]interface{}{
		"authenticated": authenticated,
	}

	if authenticated {
		if user, err := s.github.GetUser(); err == nil {
			response["user"] = user
		}
	}

	jsonResponse(w, http.StatusOK, response)
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

	activity.LogShare(port, req.Mode)

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

	activity.LogUnshare(port)
	jsonResponse(w, http.StatusOK, map[string]string{"status": "unshared"})
}

// Process management endpoints

func (s *Server) handleListProcesses(w http.ResponseWriter, r *http.Request) {
	processes := s.procs.List()
	if processes == nil {
		processes = []*process.Process{}
	}
	jsonResponse(w, http.StatusOK, processes)
}

func (s *Server) handleStartProcess(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoId")

	repo, err := s.store.GetRepo(repoID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	if repo.StartCommand == "" {
		errorResponse(w, http.StatusBadRequest, "no start command configured for this repo")
		return
	}

	proc, err := s.procs.Start(repo.ID, repo.Name, repo.Path, repo.StartCommand)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	activity.LogStart(repo.ID, repo.Name)
	jsonResponse(w, http.StatusOK, proc)
}

func (s *Server) handleStopProcess(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoId")

	// Get repo name for logging
	repo, _ := s.store.GetRepo(repoID)
	repoName := repoID
	if repo != nil {
		repoName = repo.Name
	}

	if err := s.procs.Stop(repoID); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	activity.LogStop(repoID, repoName)
	jsonResponse(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleGetProcessLogs(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoId")

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	logs := s.procs.GetLogs(repoID, limit)
	if logs == nil {
		logs = []process.LogEntry{}
	}

	jsonResponse(w, http.StatusOK, logs)
}

// Version endpoints

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, version.GetInfo())
}

func (s *Server) handleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	// TODO: Make repo owner/name configurable
	info := version.CheckForUpdates("gethomeport", "homeport")
	jsonResponse(w, http.StatusOK, info)
}

// Activity log endpoint

func (s *Server) handleGetActivity(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	entries := activity.Global().Recent(limit)
	if entries == nil {
		entries = []activity.Entry{}
	}

	jsonResponse(w, http.StatusOK, entries)
}

// Repo info and branch endpoints

func (s *Server) handleGetRepoInfo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repoData, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	info, err := repo.Detect(repoData.Path)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, info)
}

// BranchInfo represents a git branch
type BranchInfo struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"is_current"`
	IsRemote  bool   `json:"is_remote"`
}

func (s *Server) handleListBranches(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repoData, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	// Get current branch
	currentCmd := exec.Command("git", "-C", repoData.Path, "rev-parse", "--abbrev-ref", "HEAD")
	currentOutput, _ := currentCmd.Output()
	currentBranch := strings.TrimSpace(string(currentOutput))

	// List local branches
	cmd := exec.Command("git", "-C", repoData.Path, "branch", "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "failed to list branches")
		return
	}

	var branches []BranchInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		branches = append(branches, BranchInfo{
			Name:      line,
			IsCurrent: line == currentBranch,
			IsRemote:  false,
		})
	}

	// List remote branches (optional, based on query param)
	if r.URL.Query().Get("include_remote") == "true" {
		remoteCmd := exec.Command("git", "-C", repoData.Path, "branch", "-r", "--format=%(refname:short)")
		remoteOutput, _ := remoteCmd.Output()
		for _, line := range strings.Split(strings.TrimSpace(string(remoteOutput)), "\n") {
			if line == "" || strings.Contains(line, "HEAD") {
				continue
			}
			// Strip "origin/" prefix for display
			name := strings.TrimPrefix(line, "origin/")
			// Skip if we already have this as a local branch
			exists := false
			for _, b := range branches {
				if b.Name == name {
					exists = true
					break
				}
			}
			if !exists {
				branches = append(branches, BranchInfo{
					Name:     name,
					IsRemote: true,
				})
			}
		}
	}

	jsonResponse(w, http.StatusOK, branches)
}

func (s *Server) handleCheckoutBranch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repoData, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	var req struct {
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Branch == "" {
		errorResponse(w, http.StatusBadRequest, "branch is required")
		return
	}

	// Validate branch name (basic security check)
	if strings.ContainsAny(req.Branch, ";|&`$") {
		errorResponse(w, http.StatusBadRequest, "invalid branch name")
		return
	}

	cmd := exec.Command("git", "-C", repoData.Path, "checkout", req.Branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to checkout remote branch
		cmd = exec.Command("git", "-C", repoData.Path, "checkout", "-b", req.Branch, "origin/"+req.Branch)
		output, err = cmd.CombinedOutput()
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, strings.TrimSpace(string(output)))
			return
		}
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"branch":  req.Branch,
		"message": strings.TrimSpace(string(output)),
	})
}

// ExecRequest represents a command execution request
type ExecRequest struct {
	Command string `json:"command"` // Predefined command: "install", "fetch", "reset"
}

// Allowed quick commands (for security)
var allowedCommands = map[string]func(repoPath string, info *repo.RepoInfo) []string{
	"install": func(repoPath string, info *repo.RepoInfo) []string {
		switch info.PackageManager {
		case "bun":
			return []string{"bun", "install"}
		case "pnpm":
			return []string{"pnpm", "install"}
		case "yarn":
			return []string{"yarn"}
		default:
			return []string{"npm", "install"}
		}
	},
	"fetch": func(repoPath string, info *repo.RepoInfo) []string {
		return []string{"git", "fetch", "--all"}
	},
	"reset": func(repoPath string, info *repo.RepoInfo) []string {
		return []string{"git", "reset", "--hard", "HEAD"}
	},
}

func (s *Server) handleExecCommand(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repoData, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cmdBuilder, ok := allowedCommands[req.Command]
	if !ok {
		errorResponse(w, http.StatusBadRequest, "unknown command: "+req.Command)
		return
	}

	// Get repo info for package manager detection
	info, _ := repo.Detect(repoData.Path)
	if info == nil {
		info = &repo.RepoInfo{PackageManager: "npm"}
	}

	args := cmdBuilder(repoData.Path, info)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = repoData.Path

	output, err := cmd.CombinedOutput()
	success := err == nil

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": success,
		"command": strings.Join(args, " "),
		"output":  string(output),
	})
}

// Git commit and push handlers

func (s *Server) handleGitCommit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repoData, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Message == "" {
		errorResponse(w, http.StatusBadRequest, "commit message is required")
		return
	}

	// Stage all changes
	stageCmd := exec.Command("git", "-C", repoData.Path, "add", "-A")
	if output, err := stageCmd.CombinedOutput(); err != nil {
		errorResponse(w, http.StatusInternalServerError, "failed to stage changes: "+strings.TrimSpace(string(output)))
		return
	}

	// Check if there are changes to commit
	diffCmd := exec.Command("git", "-C", repoData.Path, "diff", "--cached", "--quiet")
	if err := diffCmd.Run(); err == nil {
		// No changes staged
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "No changes to commit",
		})
		return
	}

	// Commit
	commitCmd := exec.Command("git", "-C", repoData.Path, "commit", "-m", req.Message)
	output, err := commitCmd.CombinedOutput()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "commit failed: "+strings.TrimSpace(string(output)))
		return
	}

	// Get the commit hash
	hashCmd := exec.Command("git", "-C", repoData.Path, "rev-parse", "--short", "HEAD")
	hashOutput, _ := hashCmd.Output()
	commitHash := strings.TrimSpace(string(hashOutput))

	activity.LogCommit(id, repoData.Name, commitHash)
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     "Committed successfully",
		"commit_hash": commitHash,
	})
}

func (s *Server) handleGitPush(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	repoData, err := s.store.GetRepo(id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "repo not found")
		return
	}

	// Check if there's an upstream
	upstreamCmd := exec.Command("git", "-C", repoData.Path, "rev-parse", "--abbrev-ref", "@{upstream}")
	if _, err := upstreamCmd.Output(); err != nil {
		// No upstream, try to set it
		branchCmd := exec.Command("git", "-C", repoData.Path, "rev-parse", "--abbrev-ref", "HEAD")
		branchOutput, _ := branchCmd.Output()
		branch := strings.TrimSpace(string(branchOutput))

		pushCmd := exec.Command("git", "-C", repoData.Path, "push", "-u", "origin", branch)
		output, err := pushCmd.CombinedOutput()
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "push failed: "+strings.TrimSpace(string(output)))
			return
		}

		activity.LogPush(id, repoData.Name)
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Pushed to origin/" + branch,
		})
		return
	}

	// Normal push
	pushCmd := exec.Command("git", "-C", repoData.Path, "push")
	output, err := pushCmd.CombinedOutput()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "push failed: "+strings.TrimSpace(string(output)))
		return
	}

	// Check if there were any updates
	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" || strings.Contains(outputStr, "Everything up-to-date") {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Already up to date",
		})
		return
	}

	activity.LogPush(id, repoData.Name)
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Pushed successfully",
	})
}

