package version

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
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
	ReleaseNotes    string `json:"release_notes,omitempty"`
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

// compareVersions compares two semantic versions.
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Strip 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Handle special versions
	if v1 == v2 {
		return 0
	}
	if v1 == "dev" || v1 == "unknown" {
		return -1 // dev/unknown is always "older"
	}
	if v2 == "dev" || v2 == "unknown" {
		return 1
	}

	// Split into parts (major.minor.patch)
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			// Handle pre-release suffixes (e.g., "1-beta")
			numStr := strings.Split(parts1[i], "-")[0]
			n1, _ = strconv.Atoi(numStr)
		}
		if i < len(parts2) {
			numStr := strings.Split(parts2[i], "-")[0]
			n2, _ = strconv.Atoi(numStr)
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return 0
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
		Body    string `json:"body"`
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
	info.ReleaseNotes = release.Body
	// Use proper semver comparison: update available if latest > current
	info.UpdateAvailable = compareVersions(latestVersion, Version) > 0

	// Cache the result
	cacheMu.Lock()
	cachedUpdate = info
	cacheTime = time.Now()
	cacheMu.Unlock()

	return info
}
