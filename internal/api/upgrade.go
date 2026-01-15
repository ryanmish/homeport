package api

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

// UpgradeStatus represents the current state of an upgrade operation
type UpgradeStatus struct {
	Step      string `json:"step"`      // "pulling", "restarting", "verifying", "complete", "error"
	Message   string `json:"message"`   // Human-readable status message
	Error     bool   `json:"error"`     // Whether an error occurred
	Completed bool   `json:"completed"` // Whether the upgrade is complete
	Version   string `json:"version"`   // Target version being upgraded to
}

// handleStartUpgrade triggers the upgrade process
// POST /api/upgrade
func (s *Server) handleStartUpgrade(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"` // Target version, defaults to "latest"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body, default to latest
		req.Version = "latest"
	}
	if req.Version == "" {
		req.Version = "latest"
	}

	// Check if upgrade is already in progress
	lockFile := filepath.Join(s.cfg.DataDir, "upgrade.lock")
	if _, err := os.Stat(lockFile); err == nil {
		errorResponse(w, http.StatusConflict, "Upgrade already in progress")
		return
	}

	// Find the upgrade script
	// In production, it's at /home/ryan/homeport/docker/upgrade.sh
	// The script path should be configurable or discoverable
	scriptPaths := []string{
		"/home/ryan/homeport/docker/upgrade.sh",
		filepath.Join(s.cfg.DataDir, "..", "docker", "upgrade.sh"),
		"./docker/upgrade.sh",
	}

	var scriptPath string
	for _, p := range scriptPaths {
		if _, err := os.Stat(p); err == nil {
			scriptPath = p
			break
		}
	}

	if scriptPath == "" {
		errorResponse(w, http.StatusInternalServerError, "Upgrade script not found")
		return
	}

	// Execute the upgrade script in the background
	// The script handles its own backgrounding with disown
	cmd := exec.Command("bash", scriptPath, req.Version)
	cmd.Dir = filepath.Dir(scriptPath)

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"DATA_DIR="+s.cfg.DataDir,
		"COMPOSE_DIR="+filepath.Dir(scriptPath),
	)

	if err := cmd.Start(); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to start upgrade: "+err.Error())
		return
	}

	// Don't wait for the command - it handles its own lifecycle
	go func() {
		cmd.Wait()
	}()

	jsonResponse(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"version": req.Version,
	})
}

// handleUpgradeStatus returns the current upgrade status
// GET /api/upgrade/status
func (s *Server) handleUpgradeStatus(w http.ResponseWriter, r *http.Request) {
	statusFile := filepath.Join(s.cfg.DataDir, "upgrade-status.json")

	data, err := os.ReadFile(statusFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No upgrade in progress
			jsonResponse(w, http.StatusOK, UpgradeStatus{
				Step:      "idle",
				Message:   "No upgrade in progress",
				Error:     false,
				Completed: false,
			})
			return
		}
		errorResponse(w, http.StatusInternalServerError, "Failed to read upgrade status")
		return
	}

	var status UpgradeStatus
	if err := json.Unmarshal(data, &status); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to parse upgrade status")
		return
	}

	jsonResponse(w, http.StatusOK, status)
}
