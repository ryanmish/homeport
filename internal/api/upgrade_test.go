package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gethomeport/homeport/internal/version"
)

func TestUpgradeStatusStruct(t *testing.T) {
	status := UpgradeStatus{
		Step:       "building",
		Message:    "Building new version...",
		Error:      false,
		Completed:  false,
		Version:    "v1.2.0",
		Progress:   45,
		StepNumber: 3,
		TotalSteps: 5,
		StartedAt:  1705312800,
		Duration:   120,
	}

	if status.Step != "building" {
		t.Errorf("Step = %q, want %q", status.Step, "building")
	}
	if status.Progress != 45 {
		t.Errorf("Progress = %d, want %d", status.Progress, 45)
	}
	if status.StepNumber != 3 {
		t.Errorf("StepNumber = %d, want %d", status.StepNumber, 3)
	}
	if status.TotalSteps != 5 {
		t.Errorf("TotalSteps = %d, want %d", status.TotalSteps, 5)
	}
}

func TestUpgradeStatusJSON(t *testing.T) {
	status := UpgradeStatus{
		Step:       "pulling",
		Message:    "Pulling latest code...",
		Error:      false,
		Completed:  false,
		Version:    "v1.3.0",
		Progress:   20,
		StepNumber: 2,
		TotalSteps: 5,
		StartedAt:  1705312800,
		Duration:   30,
	}

	// Test JSON marshaling
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Test JSON unmarshaling
	var decoded UpgradeStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Step != status.Step {
		t.Errorf("decoded.Step = %q, want %q", decoded.Step, status.Step)
	}
	if decoded.Version != status.Version {
		t.Errorf("decoded.Version = %q, want %q", decoded.Version, status.Version)
	}
	if decoded.Progress != status.Progress {
		t.Errorf("decoded.Progress = %d, want %d", decoded.Progress, status.Progress)
	}
}

func TestUpgradeStatusSteps(t *testing.T) {
	// Test all valid step values
	validSteps := []string{
		"idle",
		"starting",
		"checking",
		"pulling",
		"building",
		"restarting",
		"verifying",
		"complete",
		"rolling_back",
		"rolled_back",
		"error",
	}

	for _, step := range validSteps {
		status := UpgradeStatus{Step: step}
		if status.Step != step {
			t.Errorf("Step = %q, want %q", status.Step, step)
		}
	}
}

func TestUpgradeStatusIdleState(t *testing.T) {
	// Test the default idle state (no upgrade in progress)
	status := UpgradeStatus{
		Step:       "idle",
		Message:    "No upgrade in progress",
		Error:      false,
		Completed:  false,
		Progress:   0,
		StepNumber: 0,
		TotalSteps: 5,
		StartedAt:  0,
		Duration:   0,
	}

	if status.Step != "idle" {
		t.Errorf("idle state Step = %q, want %q", status.Step, "idle")
	}
	if status.Progress != 0 {
		t.Errorf("idle state Progress = %d, want %d", status.Progress, 0)
	}
	if status.StartedAt != 0 {
		t.Errorf("idle state StartedAt = %d, want %d", status.StartedAt, 0)
	}
}

func TestUpgradeStatusCompleteState(t *testing.T) {
	// Test the complete state
	status := UpgradeStatus{
		Step:       "complete",
		Message:    "Upgrade complete!",
		Error:      false,
		Completed:  true,
		Version:    "v1.2.0",
		Progress:   100,
		StepNumber: 5,
		TotalSteps: 5,
	}

	if !status.Completed {
		t.Error("complete state Completed should be true")
	}
	if status.Error {
		t.Error("complete state Error should be false")
	}
	if status.Progress != 100 {
		t.Errorf("complete state Progress = %d, want %d", status.Progress, 100)
	}
}

func TestUpgradeStatusErrorState(t *testing.T) {
	// Test the error state
	status := UpgradeStatus{
		Step:      "error",
		Message:   "Failed to pull updates: network error",
		Error:     true,
		Completed: false,
		Version:   "v1.2.0",
		Progress:  15,
	}

	if !status.Error {
		t.Error("error state Error should be true")
	}
	if status.Completed {
		t.Error("error state Completed should be false")
	}
}

func TestUpgradeStatusRollbackState(t *testing.T) {
	// Test the rolled_back state
	status := UpgradeStatus{
		Step:      "rolled_back",
		Message:   "Upgrade failed. Rolled back to previous version.",
		Error:     true,
		Completed: false,
		Progress:  100,
	}

	if status.Step != "rolled_back" {
		t.Errorf("rollback state Step = %q, want %q", status.Step, "rolled_back")
	}
	if !status.Error {
		t.Error("rolled_back state Error should be true")
	}
}

func TestCheckUpgradeCompletionNoStatusFile(t *testing.T) {
	// Test when there's no status file
	tmpDir := t.TempDir()

	// Should not panic or error when status file doesn't exist
	CheckUpgradeCompletion(tmpDir)
}

func TestCheckUpgradeCompletionIdleStatus(t *testing.T) {
	// Test when status is "idle" - should not modify anything
	tmpDir := t.TempDir()
	statusFile := filepath.Join(tmpDir, "upgrade-status.json")

	status := UpgradeStatus{
		Step:    "idle",
		Message: "No upgrade in progress",
	}
	data, _ := json.Marshal(status)
	os.WriteFile(statusFile, data, 0644)

	CheckUpgradeCompletion(tmpDir)

	// Read status back
	newData, err := os.ReadFile(statusFile)
	if err != nil {
		t.Fatalf("failed to read status file: %v", err)
	}

	var newStatus UpgradeStatus
	json.Unmarshal(newData, &newStatus)

	// Status should remain unchanged
	if newStatus.Step != "idle" {
		t.Errorf("idle status should not change, got Step = %q", newStatus.Step)
	}
}

func TestCheckUpgradeCompletionInterruptedUpgradeSuccess(t *testing.T) {
	// Test when upgrade was interrupted but current version matches target
	tmpDir := t.TempDir()
	statusFile := filepath.Join(tmpDir, "upgrade-status.json")
	lockFile := filepath.Join(tmpDir, "upgrade.lock")

	// Save original version
	originalVersion := version.Version
	version.Version = "v1.2.0"
	defer func() { version.Version = originalVersion }()

	// Create status indicating interrupted upgrade at "restarting" step
	status := UpgradeStatus{
		Step:    "restarting",
		Version: "v1.2.0", // Matches current version
		Message: "Restarting services...",
	}
	data, _ := json.Marshal(status)
	os.WriteFile(statusFile, data, 0644)
	os.WriteFile(lockFile, []byte(""), 0644)

	CheckUpgradeCompletion(tmpDir)

	// Read status back
	newData, _ := os.ReadFile(statusFile)
	var newStatus UpgradeStatus
	json.Unmarshal(newData, &newStatus)

	// Status should be updated to complete
	if newStatus.Step != "complete" {
		t.Errorf("successful upgrade Step = %q, want %q", newStatus.Step, "complete")
	}
	if !newStatus.Completed {
		t.Error("successful upgrade Completed should be true")
	}
	if newStatus.Error {
		t.Error("successful upgrade Error should be false")
	}
	if newStatus.Progress != 100 {
		t.Errorf("successful upgrade Progress = %d, want %d", newStatus.Progress, 100)
	}

	// Lock file should be removed
	if _, err := os.Stat(lockFile); !os.IsNotExist(err) {
		t.Error("lock file should be removed after successful upgrade completion")
	}
}

func TestCheckUpgradeCompletionInterruptedRollback(t *testing.T) {
	// Test when rollback was interrupted
	tmpDir := t.TempDir()
	statusFile := filepath.Join(tmpDir, "upgrade-status.json")

	// Save original version
	originalVersion := version.Version
	version.Version = "v1.1.0" // Previous version (rollback target)
	defer func() { version.Version = originalVersion }()

	status := UpgradeStatus{
		Step:    "rolling_back",
		Version: "v1.2.0", // Original upgrade target
		Message: "Rolling back...",
	}
	data, _ := json.Marshal(status)
	os.WriteFile(statusFile, data, 0644)

	CheckUpgradeCompletion(tmpDir)

	// Read status back
	newData, _ := os.ReadFile(statusFile)
	var newStatus UpgradeStatus
	json.Unmarshal(newData, &newStatus)

	// Status should indicate rollback completed
	if newStatus.Step != "rolled_back" {
		t.Errorf("rollback complete Step = %q, want %q", newStatus.Step, "rolled_back")
	}
	if !newStatus.Error {
		t.Error("rolled_back state Error should be true")
	}
}

func TestCheckUpgradeCompletionLatestVersion(t *testing.T) {
	// Test when upgrading to "latest" version
	tmpDir := t.TempDir()
	statusFile := filepath.Join(tmpDir, "upgrade-status.json")

	// Save original version
	originalVersion := version.Version
	version.Version = "v1.3.0" // Any version
	defer func() { version.Version = originalVersion }()

	status := UpgradeStatus{
		Step:    "verifying",
		Version: "latest", // Special case
		Message: "Verifying upgrade...",
	}
	data, _ := json.Marshal(status)
	os.WriteFile(statusFile, data, 0644)

	CheckUpgradeCompletion(tmpDir)

	// Read status back
	newData, _ := os.ReadFile(statusFile)
	var newStatus UpgradeStatus
	json.Unmarshal(newData, &newStatus)

	// Should be marked as complete (latest always matches)
	if newStatus.Step != "complete" {
		t.Errorf("latest version upgrade Step = %q, want %q", newStatus.Step, "complete")
	}
}

func TestCheckUpgradeCompletionVersionMismatch(t *testing.T) {
	// Test when version doesn't match (upgrade may have failed)
	tmpDir := t.TempDir()
	statusFile := filepath.Join(tmpDir, "upgrade-status.json")

	// Save original version
	originalVersion := version.Version
	version.Version = "v1.1.0" // Current version
	defer func() { version.Version = originalVersion }()

	status := UpgradeStatus{
		Step:    "restarting",
		Version: "v1.2.0", // Target version doesn't match current
		Message: "Restarting services...",
	}
	data, _ := json.Marshal(status)
	os.WriteFile(statusFile, data, 0644)

	CheckUpgradeCompletion(tmpDir)

	// Read status back
	newData, _ := os.ReadFile(statusFile)
	var newStatus UpgradeStatus
	json.Unmarshal(newData, &newStatus)

	// Status should indicate error
	if newStatus.Step != "error" {
		t.Errorf("version mismatch Step = %q, want %q", newStatus.Step, "error")
	}
	if !newStatus.Error {
		t.Error("version mismatch Error should be true")
	}
}

func TestCheckUpgradeCompletionInvalidJSON(t *testing.T) {
	// Test with invalid JSON in status file
	tmpDir := t.TempDir()
	statusFile := filepath.Join(tmpDir, "upgrade-status.json")

	os.WriteFile(statusFile, []byte("not valid json"), 0644)

	// Should not panic
	CheckUpgradeCompletion(tmpDir)
}

func TestCheckUpgradeCompletionBuildingStep(t *testing.T) {
	// Test that "building" step is not considered an interrupted upgrade
	tmpDir := t.TempDir()
	statusFile := filepath.Join(tmpDir, "upgrade-status.json")

	status := UpgradeStatus{
		Step:    "building",
		Message: "Building new version...",
		Version: "v1.2.0",
	}
	data, _ := json.Marshal(status)
	os.WriteFile(statusFile, data, 0644)

	CheckUpgradeCompletion(tmpDir)

	// Read status back
	newData, _ := os.ReadFile(statusFile)
	var newStatus UpgradeStatus
	json.Unmarshal(newData, &newStatus)

	// Status should remain unchanged (building is not an interrupted state)
	if newStatus.Step != "building" {
		t.Errorf("building step should not change, got Step = %q", newStatus.Step)
	}
}

func TestUpgradeStatusProgressValues(t *testing.T) {
	// Test progress values at different steps
	testCases := []struct {
		step           string
		expectedMinPct int
		expectedMaxPct int
	}{
		{"idle", 0, 0},
		{"starting", 0, 10},
		{"checking", 5, 15},
		{"pulling", 15, 30},
		{"building", 30, 80},
		{"restarting", 80, 95},
		{"verifying", 95, 99},
		{"complete", 100, 100},
	}

	for _, tc := range testCases {
		status := UpgradeStatus{Step: tc.step}
		// This test documents expected progress ranges for each step
		// The actual values are set by the upgrade.sh script
		t.Logf("Step %q: expected progress range %d-%d%%", tc.step, tc.expectedMinPct, tc.expectedMaxPct)
	}
}

func TestUpgradeStatusDuration(t *testing.T) {
	status := UpgradeStatus{
		Step:      "building",
		StartedAt: 1705312800,
		Duration:  300, // 5 minutes
	}

	if status.Duration != 300 {
		t.Errorf("Duration = %d, want %d", status.Duration, 300)
	}
}
