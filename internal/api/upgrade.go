package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gethomeport/homeport/internal/version"
)

// UpgradeStatus represents the current state of an upgrade operation
type UpgradeStatus struct {
	Step      string `json:"step"`      // "pulling", "restarting", "verifying", "complete", "rolling_back", "rolled_back", "error"
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
	// Inside container: /homeport-compose/upgrade.sh (mounted from docker directory)
	// On host: /home/ryan/homeport/docker/upgrade.sh or similar
	scriptPaths := []string{
		"/homeport-compose/upgrade.sh",
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

// handleRollback triggers a rollback to the previous version
// POST /api/rollback
func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	// Check if rollback is already in progress
	lockFile := filepath.Join(s.cfg.DataDir, "upgrade.lock")
	if _, err := os.Stat(lockFile); err == nil {
		errorResponse(w, http.StatusConflict, "Operation already in progress")
		return
	}

	// Find the rollback script
	scriptPaths := []string{
		"/homeport-compose/rollback.sh",
		"/home/ryan/homeport/docker/rollback.sh",
		filepath.Join(s.cfg.DataDir, "..", "docker", "rollback.sh"),
		"./docker/rollback.sh",
	}

	var scriptPath string
	for _, p := range scriptPaths {
		if _, err := os.Stat(p); err == nil {
			scriptPath = p
			break
		}
	}

	if scriptPath == "" {
		errorResponse(w, http.StatusInternalServerError, "Rollback script not found")
		return
	}

	// Execute the rollback script
	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = filepath.Dir(scriptPath)
	cmd.Env = append(os.Environ(),
		"DATA_DIR="+s.cfg.DataDir,
		"COMPOSE_DIR="+filepath.Dir(scriptPath),
	)

	if err := cmd.Start(); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to start rollback: "+err.Error())
		return
	}

	go func() {
		cmd.Wait()
	}()

	jsonResponse(w, http.StatusAccepted, map[string]string{
		"status": "started",
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

// CheckUpgradeCompletion should be called on server startup to handle
// incomplete upgrades (e.g., when the upgrade script was interrupted by container restart)
func CheckUpgradeCompletion(dataDir string) {
	statusFile := filepath.Join(dataDir, "upgrade-status.json")
	lockFile := filepath.Join(dataDir, "upgrade.lock")

	// Read current status
	data, err := os.ReadFile(statusFile)
	if err != nil {
		// No status file, nothing to do
		return
	}

	var status UpgradeStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return
	}

	// Only handle interrupted upgrades (restarting, verifying, or rolling_back states)
	if status.Step != "restarting" && status.Step != "verifying" && status.Step != "rolling_back" {
		return
	}

	log.Printf("Detected interrupted upgrade (status: %s, target: %s)", status.Step, status.Version)

	// Check if current version matches target
	currentVersion := version.Version
	targetVersion := status.Version

	// Normalize versions for comparison (strip 'v' prefix)
	currentNorm := strings.TrimPrefix(currentVersion, "v")
	targetNorm := strings.TrimPrefix(targetVersion, "v")

	// Check if we were rolling back
	if status.Step == "rolling_back" {
		// If daemon is running, rollback succeeded
		log.Printf("Rollback completed successfully (now running %s)", currentVersion)
		status.Step = "rolled_back"
		status.Message = "Upgrade failed. Rolled back to previous version."
		status.Completed = false
		status.Error = true
	} else if currentNorm == targetNorm || targetVersion == "latest" {
		// Upgrade succeeded - update status
		log.Printf("Upgrade completed successfully (now running %s)", currentVersion)
		status.Step = "complete"
		status.Message = "Upgrade complete!"
		status.Completed = true
		status.Error = false
	} else {
		// Upgrade may have failed - report error
		log.Printf("Upgrade may have failed (expected %s, running %s)", targetVersion, currentVersion)
		status.Step = "error"
		status.Message = "Upgrade interrupted. Please try again."
		status.Error = true
		status.Completed = false
	}

	// Write updated status
	updatedData, _ := json.Marshal(status)
	os.WriteFile(statusFile, updatedData, 0644)

	// Clean up lock file
	os.Remove(lockFile)
}
