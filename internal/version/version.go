package version

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Version is set at build time via ldflags
var (
	Version   = "0.1.0"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// UpdateInfo contains information about available updates
type UpdateInfo struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url,omitempty"`
	CheckedAt       string `json:"checked_at,omitempty"`
	Error           string `json:"error,omitempty"`
}

var (
	cachedUpdate *UpdateInfo
	cacheMu      sync.RWMutex
	cacheTime    time.Time
)

// GetInfo returns version information
func GetInfo() map[string]string {
	return map[string]string{
		"version":    Version,
		"git_commit": GitCommit,
		"build_time": BuildTime,
	}
}

// CheckForUpdates checks GitHub for newer releases
func CheckForUpdates(repoOwner, repoName string) *UpdateInfo {
	// Check cache (valid for 1 hour)
	cacheMu.RLock()
	if cachedUpdate != nil && time.Since(cacheTime) < time.Hour {
		cacheMu.RUnlock()
		return cachedUpdate
	}
	cacheMu.RUnlock()

	info := &UpdateInfo{
		CurrentVersion: Version,
		CheckedAt:      time.Now().Format(time.RFC3339),
	}

	// Fetch latest release from GitHub API
	url := "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		info.Error = "Failed to check for updates"
		return info
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		info.Error = "No releases found"
		return info
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		info.Error = "Failed to parse release info"
		return info
	}

	// Strip 'v' prefix if present
	latestVersion := release.TagName
	if len(latestVersion) > 0 && latestVersion[0] == 'v' {
		latestVersion = latestVersion[1:]
	}

	info.LatestVersion = latestVersion
	info.ReleaseURL = release.HTMLURL
	info.UpdateAvailable = latestVersion != Version

	// Cache the result
	cacheMu.Lock()
	cachedUpdate = info
	cacheTime = time.Now()
	cacheMu.Unlock()

	return info
}
