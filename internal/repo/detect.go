package repo

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// PackageJSON represents a subset of package.json fields we care about
type PackageJSON struct {
	Name    string            `json:"name"`
	Scripts map[string]string `json:"scripts"`
}

// RepoInfo contains detected information about a repository
type RepoInfo struct {
	HasPackageJSON  bool              `json:"has_package_json"`
	HasNodeModules  bool              `json:"has_node_modules"`
	NeedsInstall    bool              `json:"needs_install"`
	DetectedCommand string            `json:"detected_command,omitempty"`
	AvailableScripts map[string]string `json:"available_scripts,omitempty"`
	PackageManager  string            `json:"package_manager,omitempty"` // npm, yarn, pnpm, bun
}

// Detect analyzes a repository and returns information about it
func Detect(repoPath string) (*RepoInfo, error) {
	info := &RepoInfo{}

	// Check for package.json
	packageJSONPath := filepath.Join(repoPath, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		info.HasPackageJSON = true

		// Parse package.json
		data, err := os.ReadFile(packageJSONPath)
		if err == nil {
			var pkg PackageJSON
			if json.Unmarshal(data, &pkg) == nil {
				info.AvailableScripts = pkg.Scripts
				info.DetectedCommand = detectStartCommand(pkg.Scripts)
			}
		}
	}

	// Check for node_modules
	nodeModulesPath := filepath.Join(repoPath, "node_modules")
	if stat, err := os.Stat(nodeModulesPath); err == nil && stat.IsDir() {
		info.HasNodeModules = true
	}

	// Needs install if package.json exists but no node_modules
	info.NeedsInstall = info.HasPackageJSON && !info.HasNodeModules

	// Detect package manager
	info.PackageManager = detectPackageManager(repoPath)

	return info, nil
}

// detectStartCommand returns the best start command based on package.json scripts
func detectStartCommand(scripts map[string]string) string {
	if scripts == nil {
		return ""
	}

	// Priority order for dev commands
	devScripts := []string{"dev", "start:dev", "serve", "develop"}
	for _, name := range devScripts {
		if _, ok := scripts[name]; ok {
			return "npm run " + name
		}
	}

	// Fall back to start
	if _, ok := scripts["start"]; ok {
		return "npm start"
	}

	return ""
}

// detectPackageManager determines which package manager the project uses
func detectPackageManager(repoPath string) string {
	// Check for lock files in order of preference
	lockFiles := map[string]string{
		"bun.lockb":         "bun",
		"pnpm-lock.yaml":    "pnpm",
		"yarn.lock":         "yarn",
		"package-lock.json": "npm",
	}

	for lockFile, manager := range lockFiles {
		if _, err := os.Stat(filepath.Join(repoPath, lockFile)); err == nil {
			return manager
		}
	}

	// Default to npm
	return "npm"
}

// GetInstallCommand returns the install command for the detected package manager
func (r *RepoInfo) GetInstallCommand() string {
	switch r.PackageManager {
	case "bun":
		return "bun install"
	case "pnpm":
		return "pnpm install"
	case "yarn":
		return "yarn"
	default:
		return "npm install"
	}
}

// GetFormattedCommand formats the detected command for the actual package manager
func (r *RepoInfo) GetFormattedCommand() string {
	if r.DetectedCommand == "" {
		return ""
	}

	// If we detected "npm run dev" but they use pnpm, format accordingly
	switch r.PackageManager {
	case "bun":
		if r.DetectedCommand == "npm start" {
			return "bun run start"
		}
		return "bun run " + extractScriptName(r.DetectedCommand)
	case "pnpm":
		if r.DetectedCommand == "npm start" {
			return "pnpm start"
		}
		return "pnpm run " + extractScriptName(r.DetectedCommand)
	case "yarn":
		if r.DetectedCommand == "npm start" {
			return "yarn start"
		}
		return "yarn " + extractScriptName(r.DetectedCommand)
	default:
		return r.DetectedCommand
	}
}

func extractScriptName(cmd string) string {
	// "npm run dev" -> "dev"
	// "npm start" -> "start"
	if len(cmd) > 8 && cmd[:8] == "npm run " {
		return cmd[8:]
	}
	if cmd == "npm start" {
		return "start"
	}
	return cmd
}
