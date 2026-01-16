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
	Step      string `json:"step"`      // "idle", "starting", "checking", "pulling", "building", "restarting", "verifying", "complete", "rolling_back", "rolled_back", "error"
	Message   string `json:"message"`   // Human-readable status message
	Error     bool   `json:"error"`     // Whether an error occurred
	Completed bool   `json:"completed"` // Whether the upgrade is complete
	Version   string `json:"version"`   // Target version being upgraded to
}

// handleStartUpgrade triggers the upgrade process
// POST /api/upgrade
func (s *Server) handleStartUpgrade(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Version = "latest"
	}
	if req.Version == "" {
		req.Version = "latest"
	}

	// Get repo path from env (set during install)
	repoDir := os.Getenv("HOMEPORT_REPO_PATH")
	if repoDir == "" {
		errorResponse(w, http.StatusInternalServerError, "HOMEPORT_REPO_PATH environment variable not set")
		return
	}

	// Get home dir for gh config
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/root"
	}

	// Check if upgrader already running (use container name, not PID)
	checkCmd := exec.Command("docker", "ps", "-q", "-f", "name=homeport-upgrader")
	if output, _ := checkCmd.Output(); len(strings.TrimSpace(string(output))) > 0 {
		errorResponse(w, http.StatusConflict, "Upgrade already in progress")
		return
	}

	// Clean up any stale upgrader container
	exec.Command("docker", "rm", "-f", "homeport-upgrader").Run()

	// Spawn the upgrader container
	args := []string{
		"run", "--rm", "-d",
		"--name", "homeport-upgrader",
		"--network", "host",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", repoDir + ":/homeport",
		"-v", "docker_homeport-data:/srv/homeport/data",
		"-v", homeDir + "/.config/gh:/home/homeport/.config/gh:ro",
		"-e", "VERSION=" + req.Version,
		"-e", "COMPOSE_PROJECT_NAME=docker",
		"-e", "DOCKER_API_VERSION=1.44",
		"-e", "DATA_DIR=/srv/homeport/data",
		"-e", "REPO_DIR=/homeport",
		"-e", "COMPOSE_DIR=/homeport/docker",
		"homeport:latest",
		"/bin/bash", "/homeport/docker/upgrade.sh", req.Version,
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to start upgrade container: %v - %s", err, string(output))
		errorResponse(w, http.StatusInternalServerError,
			"Failed to start upgrade: "+err.Error())
		return
	}

	log.Printf("Started upgrade container for version %s", req.Version)
	jsonResponse(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"version": req.Version,
	})
}

// handleRollback triggers a rollback to the previous version
// POST /api/rollback
func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	// Get repo path from env
	repoDir := os.Getenv("HOMEPORT_REPO_PATH")
	if repoDir == "" {
		errorResponse(w, http.StatusInternalServerError, "HOMEPORT_REPO_PATH environment variable not set")
		return
	}

	// Get home dir for gh config
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/root"
	}

	// Check if upgrader/rollback already running
	checkCmd := exec.Command("docker", "ps", "-q", "-f", "name=homeport-upgrader")
	if output, _ := checkCmd.Output(); len(strings.TrimSpace(string(output))) > 0 {
		errorResponse(w, http.StatusConflict, "Upgrade already in progress")
		return
	}
	checkCmd2 := exec.Command("docker", "ps", "-q", "-f", "name=homeport-rollback")
	if output, _ := checkCmd2.Output(); len(strings.TrimSpace(string(output))) > 0 {
		errorResponse(w, http.StatusConflict, "Rollback already in progress")
		return
	}

	// Clean up any stale container
	exec.Command("docker", "rm", "-f", "homeport-rollback").Run()

	// Spawn the rollback container
	args := []string{
		"run", "--rm", "-d",
		"--name", "homeport-rollback",
		"--network", "host",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", repoDir + ":/homeport",
		"-v", "docker_homeport-data:/srv/homeport/data",
		"-v", homeDir + "/.config/gh:/home/homeport/.config/gh:ro",
		"-e", "COMPOSE_PROJECT_NAME=docker",
		"-e", "DOCKER_API_VERSION=1.44",
		"-e", "DATA_DIR=/srv/homeport/data",
		"-e", "REPO_DIR=/homeport",
		"-e", "COMPOSE_DIR=/homeport/docker",
		"homeport:latest",
		"/bin/bash", "/homeport/docker/rollback.sh",
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to start rollback container: %v - %s", err, string(output))
		errorResponse(w, http.StatusInternalServerError,
			"Failed to start rollback: "+err.Error())
		return
	}

	log.Printf("Started rollback container")
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
