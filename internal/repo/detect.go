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
	HasPackageJSON   bool              `json:"has_package_json"`
	HasNodeModules   bool              `json:"has_node_modules"`
	NeedsInstall     bool              `json:"needs_install"`
	DetectedCommand  string            `json:"detected_command,omitempty"`
	AvailableScripts map[string]string `json:"available_scripts,omitempty"`
	PackageManager   string            `json:"package_manager,omitempty"` // npm, yarn, pnpm, bun
	ProjectType      string            `json:"project_type,omitempty"`    // node, python, rust, go
	InstallCommand   string            `json:"install_command,omitempty"` // Full install command
}

// Detect analyzes a repository and returns information about it
func Detect(repoPath string) (*RepoInfo, error) {
	info := &RepoInfo{}

	// Check for Node.js project (package.json)
	packageJSONPath := filepath.Join(repoPath, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		info.HasPackageJSON = true
		info.ProjectType = "node"

		// Parse package.json
		data, err := os.ReadFile(packageJSONPath)
		if err == nil {
			var pkg PackageJSON
			if json.Unmarshal(data, &pkg) == nil {
				info.AvailableScripts = pkg.Scripts
				info.DetectedCommand = detectStartCommand(pkg.Scripts)
			}
		}

		// Check for node_modules
		nodeModulesPath := filepath.Join(repoPath, "node_modules")
		if stat, err := os.Stat(nodeModulesPath); err == nil && stat.IsDir() {
			info.HasNodeModules = true
		}

		// Needs install if package.json exists but no node_modules
		info.NeedsInstall = !info.HasNodeModules

		// Detect package manager
		info.PackageManager = detectPackageManager(repoPath)
		info.InstallCommand = info.GetInstallCommand()

		return info, nil
	}

	// Check for Python project
	if hasPythonProject(repoPath) {
		info.ProjectType = "python"
		info.PackageManager, info.InstallCommand = detectPythonPackageManager(repoPath)
		info.NeedsInstall = !hasPythonVenv(repoPath)
		return info, nil
	}

	// Check for Rust project (Cargo.toml)
	if _, err := os.Stat(filepath.Join(repoPath, "Cargo.toml")); err == nil {
		info.ProjectType = "rust"
		info.PackageManager = "cargo"
		info.InstallCommand = "cargo build"
		// Rust doesn't really have an "install" step in the same way
		info.NeedsInstall = false
		return info, nil
	}

	// Check for Go project (go.mod)
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); err == nil {
		info.ProjectType = "go"
		info.PackageManager = "go"
		info.InstallCommand = "go mod download"
		info.NeedsInstall = false // Go downloads deps on build
		return info, nil
	}

	return info, nil
}

// hasPythonProject checks if this is a Python project
func hasPythonProject(repoPath string) bool {
	pythonFiles := []string{
		"requirements.txt",
		"pyproject.toml",
		"Pipfile",
		"setup.py",
		"setup.cfg",
	}
	for _, f := range pythonFiles {
		if _, err := os.Stat(filepath.Join(repoPath, f)); err == nil {
			return true
		}
	}
	return false
}

// hasPythonVenv checks if a virtual environment exists
func hasPythonVenv(repoPath string) bool {
	venvDirs := []string{"venv", ".venv", "env", ".env"}
	for _, d := range venvDirs {
		if stat, err := os.Stat(filepath.Join(repoPath, d)); err == nil && stat.IsDir() {
			return true
		}
	}
	return false
}

// detectPythonPackageManager determines which Python package manager to use
func detectPythonPackageManager(repoPath string) (manager, installCmd string) {
	// Poetry (pyproject.toml with [tool.poetry])
	if _, err := os.Stat(filepath.Join(repoPath, "poetry.lock")); err == nil {
		return "poetry", "poetry install"
	}

	// Pipenv
	if _, err := os.Stat(filepath.Join(repoPath, "Pipfile.lock")); err == nil {
		return "pipenv", "pipenv install"
	}
	if _, err := os.Stat(filepath.Join(repoPath, "Pipfile")); err == nil {
		return "pipenv", "pipenv install"
	}

	// uv (modern Python package installer)
	if _, err := os.Stat(filepath.Join(repoPath, "uv.lock")); err == nil {
		return "uv", "uv sync"
	}

	// Default to pip with requirements.txt
	if _, err := os.Stat(filepath.Join(repoPath, "requirements.txt")); err == nil {
		return "pip", "pip install -r requirements.txt"
	}

	// pyproject.toml without poetry
	if _, err := os.Stat(filepath.Join(repoPath, "pyproject.toml")); err == nil {
		return "pip", "pip install -e ."
	}

	return "pip", "pip install -r requirements.txt"
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
