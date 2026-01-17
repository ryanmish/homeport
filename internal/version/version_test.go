package version

import (
	"testing"
)

func TestGetVersion(t *testing.T) {
	// Test that GetVersion strips the 'v' prefix
	tests := []struct {
		version  string
		expected string
	}{
		{"v1.0.0", "1.0.0"},
		{"v0.1.0", "0.1.0"},
		{"1.0.0", "1.0.0"},
		{"latest", "latest"},
		{"dev", "dev"},
	}

	for _, tt := range tests {
		// Save original version
		originalVersion := Version
		Version = tt.version

		result := GetVersion()
		if result != tt.expected {
			t.Errorf("GetVersion() with Version=%q = %q, want %q", tt.version, result, tt.expected)
		}

		// Restore original version
		Version = originalVersion
	}
}

func TestGetInfo(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := GitCommit
	originalBuildTime := BuildTime

	// Set test values
	Version = "v1.2.3"
	GitCommit = "abc123"
	BuildTime = "2024-01-15T10:00:00Z"

	defer func() {
		// Restore original values
		Version = originalVersion
		GitCommit = originalCommit
		BuildTime = originalBuildTime
	}()

	info := GetInfo()

	if info["version"] != "1.2.3" {
		t.Errorf("GetInfo()[version] = %q, want %q", info["version"], "1.2.3")
	}
	if info["git_commit"] != "abc123" {
		t.Errorf("GetInfo()[git_commit] = %q, want %q", info["git_commit"], "abc123")
	}
	if info["build_time"] != "2024-01-15T10:00:00Z" {
		t.Errorf("GetInfo()[build_time] = %q, want %q", info["build_time"], "2024-01-15T10:00:00Z")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// Equal versions
		{"equal simple", "1.0.0", "1.0.0", 0},
		{"equal with v prefix", "v1.0.0", "v1.0.0", 0},
		{"equal mixed prefix", "v1.0.0", "1.0.0", 0},

		// v1 < v2 (negative result)
		{"major less", "1.0.0", "2.0.0", -1},
		{"minor less", "1.1.0", "1.2.0", -1},
		{"patch less", "1.0.1", "1.0.2", -1},
		{"combined less", "1.2.3", "1.2.4", -1},

		// v1 > v2 (positive result)
		{"major greater", "2.0.0", "1.0.0", 1},
		{"minor greater", "1.2.0", "1.1.0", 1},
		{"patch greater", "1.0.2", "1.0.1", 1},
		{"combined greater", "2.1.0", "1.9.9", 1},

		// Different segment counts
		{"two vs three segments equal", "1.0", "1.0.0", 0},
		{"two vs three segments less", "1.0", "1.0.1", -1},
		{"three vs two segments greater", "1.0.1", "1.0", 1},
		{"single segment", "2", "1", 1},

		// Pre-release versions
		{"pre-release ignored", "1.0.0-beta", "1.0.0", 0},
		{"pre-release vs pre-release", "1.0.0-alpha", "1.0.0-beta", 0},

		// Special versions
		{"latest vs version", "latest", "1.0.0", 1},
		{"version vs latest", "1.0.0", "latest", -1},
		{"dev vs version", "dev", "1.0.0", -1},
		{"version vs dev", "1.0.0", "dev", 1},
		{"unknown vs version", "unknown", "1.0.0", -1},
		{"version vs unknown", "1.0.0", "unknown", 1},
		{"latest vs latest", "latest", "latest", 0},

		// Edge cases
		{"zero versions", "0.0.0", "0.0.1", -1},
		{"large numbers", "10.20.30", "10.20.29", 1},
		{"v prefix both", "v2.0.0", "v1.9.9", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestCompareVersionsSymmetry(t *testing.T) {
	// Test that compareVersions is antisymmetric: compare(a,b) = -compare(b,a)
	pairs := [][2]string{
		{"1.0.0", "2.0.0"},
		{"1.2.3", "1.2.4"},
		{"v1.0.0", "v1.0.1"},
		{"2.0.0", "1.9.9"},
	}

	for _, pair := range pairs {
		result1 := compareVersions(pair[0], pair[1])
		result2 := compareVersions(pair[1], pair[0])

		if result1 != -result2 {
			t.Errorf("compareVersions(%q, %q) = %d, but compareVersions(%q, %q) = %d (expected %d)",
				pair[0], pair[1], result1, pair[1], pair[0], result2, -result1)
		}
	}
}

func TestCompareVersionsTransitivity(t *testing.T) {
	// Test transitivity: if a < b and b < c, then a < c
	versions := []string{"1.0.0", "1.1.0", "2.0.0"}

	for i := 0; i < len(versions)-2; i++ {
		a, b, c := versions[i], versions[i+1], versions[i+2]

		ab := compareVersions(a, b)
		bc := compareVersions(b, c)
		ac := compareVersions(a, c)

		if ab < 0 && bc < 0 && ac >= 0 {
			t.Errorf("transitivity violated: %q < %q and %q < %q but %q >= %q",
				a, b, b, c, a, c)
		}
	}
}

func TestUpdateInfoStruct(t *testing.T) {
	info := UpdateInfo{
		CurrentVersion:  "1.0.0",
		LatestVersion:   "1.1.0",
		UpdateAvailable: true,
		ReleaseURL:      "https://github.com/example/releases/v1.1.0",
		ReleaseNotes:    "Bug fixes and improvements",
		CheckedAt:       "2024-01-15T10:00:00Z",
		Error:           "",
	}

	if info.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want %q", info.CurrentVersion, "1.0.0")
	}
	if !info.UpdateAvailable {
		t.Error("UpdateAvailable should be true")
	}
	if info.Error != "" {
		t.Errorf("Error should be empty, got %q", info.Error)
	}
}

func TestUpdateInfoWithError(t *testing.T) {
	info := UpdateInfo{
		CurrentVersion:  "1.0.0",
		UpdateAvailable: false,
		Error:           "Failed to check for updates",
	}

	if info.UpdateAvailable {
		t.Error("UpdateAvailable should be false when there's an error")
	}
	if info.Error == "" {
		t.Error("Error should not be empty")
	}
}

func TestVersionVariables(t *testing.T) {
	// Test that version variables have reasonable defaults
	if Version == "" {
		t.Error("Version should not be empty")
	}

	// GitCommit and BuildTime can be "unknown" as defaults
	if GitCommit == "" {
		t.Error("GitCommit should not be empty (can be 'unknown')")
	}
	if BuildTime == "" {
		t.Error("BuildTime should not be empty (can be 'unknown')")
	}
}

func TestCompareVersionsRealWorldCases(t *testing.T) {
	// Test real-world version comparison scenarios
	tests := []struct {
		name     string
		current  string
		latest   string
		expected int
	}{
		{"homeport initial release", "0.1.0", "0.1.0", 0},
		{"homeport minor update", "0.1.0", "0.2.0", -1},
		{"homeport major update", "0.9.9", "1.0.0", -1},
		{"homeport patch update", "1.0.0", "1.0.1", -1},
		{"running dev build", "dev", "1.0.0", -1},
		{"running latest build", "latest", "1.0.0", 1},
		{"github release format", "v1.2.3", "v1.2.4", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.current, tt.latest)
			if result != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d, want %d",
					tt.current, tt.latest, result, tt.expected)
			}
		})
	}
}
