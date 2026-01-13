package github

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client wraps the gh CLI for GitHub operations
type Client struct {
	reposDir string
}

// Repo represents a GitHub repository
type Repo struct {
	Name        string `json:"name"`
	FullName    string `json:"nameWithOwner"`
	Description string `json:"description"`
	URL         string `json:"url"`
	IsPrivate   bool   `json:"isPrivate"`
	IsFork      bool   `json:"isFork"`
}

func NewClient(reposDir string) *Client {
	return &Client{reposDir: reposDir}
}

// User represents GitHub user info
type User struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
}

// IsAuthenticated checks if gh CLI is authenticated
func (c *Client) IsAuthenticated() bool {
	cmd := exec.Command("gh", "auth", "status")
	return cmd.Run() == nil
}

// GetUser returns the authenticated GitHub user info
func (c *Client) GetUser() (*User, error) {
	cmd := exec.Command("gh", "api", "user")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse JSON response - GitHub API uses different field names
	var apiUser struct {
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(output, &apiUser); err != nil {
		return nil, err
	}

	return &User{
		Login:     apiUser.Login,
		Name:      apiUser.Name,
		Email:     apiUser.Email,
		AvatarURL: apiUser.AvatarURL,
	}, nil
}

// ListRepos lists the user's GitHub repositories
func (c *Client) ListRepos(limit int) ([]Repo, error) {
	if limit <= 0 {
		limit = 100
	}

	// gh repo list --json name,nameWithOwner,description,url,isPrivate,isFork --limit 100
	cmd := exec.Command("gh", "repo", "list",
		"--json", "name,nameWithOwner,description,url,isPrivate,isFork",
		"--limit", fmt.Sprintf("%d", limit),
	)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh repo list failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	var repos []Repo
	if err := json.Unmarshal(output, &repos); err != nil {
		return nil, err
	}

	return repos, nil
}

// Clone clones a repository to the repos directory
func (c *Client) Clone(repoFullName string) (string, error) {
	// Extract just the repo name for the local directory
	parts := strings.Split(repoFullName, "/")
	repoName := parts[len(parts)-1]
	localPath := filepath.Join(c.reposDir, repoName)

	// Check if already exists
	if _, err := os.Stat(localPath); err == nil {
		return "", fmt.Errorf("repository already exists at %s", localPath)
	}

	// gh repo clone owner/repo path
	cmd := exec.Command("gh", "repo", "clone", repoFullName, localPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to clone %s: %w", repoFullName, err)
	}

	return localPath, nil
}

// Pull runs git pull in the specified repo directory
func (c *Client) Pull(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "pull")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetRepoURL returns the GitHub URL for a cloned repo
func (c *Client) GetRepoURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRemoteURL returns the git remote URL, or empty string if not available
func (c *Client) GetRemoteURL(repoPath string) string {
	url, _ := c.GetRepoURL(repoPath)
	return url
}

// Init creates a new local git repository
func (c *Client) Init(name string) (string, error) {
	localPath := filepath.Join(c.reposDir, name)

	// Check if already exists
	if _, err := os.Stat(localPath); err == nil {
		return "", fmt.Errorf("directory already exists at %s", localPath)
	}

	// Create directory
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// git init
	cmd := exec.Command("git", "init", localPath)
	if err := cmd.Run(); err != nil {
		os.RemoveAll(localPath) // cleanup on failure
		return "", fmt.Errorf("failed to init git repo: %w", err)
	}

	return localPath, nil
}

// Search searches for repositories matching the query
func (c *Client) Search(query string, limit int) ([]Repo, error) {
	if limit <= 0 {
		limit = 20
	}

	// gh search repos "query" --json name,nameWithOwner,description,url,isPrivate,isFork --limit 20
	cmd := exec.Command("gh", "search", "repos", query,
		"--json", "name,nameWithOwner,description,url,isPrivate,isFork",
		"--limit", fmt.Sprintf("%d", limit),
	)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh search failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	var repos []Repo
	if err := json.Unmarshal(output, &repos); err != nil {
		return nil, err
	}

	return repos, nil
}

// GitStatus represents the status of a git repository
type GitStatus struct {
	Branch        string `json:"branch"`
	IsDirty       bool   `json:"is_dirty"`
	Ahead         int    `json:"ahead"`
	Behind        int    `json:"behind"`
	LastCommit    string `json:"last_commit,omitempty"`
	LastCommitMsg string `json:"last_commit_msg,omitempty"`
}

// GetStatus returns the git status for a repository
func (c *Client) GetStatus(repoPath string) (*GitStatus, error) {
	status := &GitStatus{}

	// Get current branch
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if output, err := cmd.Output(); err == nil {
		status.Branch = strings.TrimSpace(string(output))
	}

	// Check if dirty (uncommitted changes)
	cmd = exec.Command("git", "-C", repoPath, "status", "--porcelain")
	if output, err := cmd.Output(); err == nil {
		status.IsDirty = len(strings.TrimSpace(string(output))) > 0
	}

	// Get ahead/behind counts
	cmd = exec.Command("git", "-C", repoPath, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if output, err := cmd.Output(); err == nil {
		parts := strings.Fields(string(output))
		if len(parts) >= 2 {
			if ahead, err := fmt.Sscanf(parts[0], "%d", &status.Ahead); err == nil && ahead > 0 {
				// parsed
			}
			if behind, err := fmt.Sscanf(parts[1], "%d", &status.Behind); err == nil && behind > 0 {
				// parsed
			}
		}
	}

	// Get last commit info
	cmd = exec.Command("git", "-C", repoPath, "log", "-1", "--format=%h|%s")
	if output, err := cmd.Output(); err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(output)), "|", 2)
		if len(parts) >= 1 {
			status.LastCommit = parts[0]
		}
		if len(parts) >= 2 {
			status.LastCommitMsg = parts[1]
			// Truncate long messages
			if len(status.LastCommitMsg) > 50 {
				status.LastCommitMsg = status.LastCommitMsg[:47] + "..."
			}
		}
	}

	return status, nil
}

// PullResult contains the result of a git pull
type PullResult struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	FilesChanged int      `json:"files_changed"`
	Insertions   int      `json:"insertions"`
	Deletions    int      `json:"deletions"`
	Files        []string `json:"files,omitempty"`
}

// PullWithDetails runs git pull and returns detailed results
// It checks for uncommitted changes first and warns if any are found
func (c *Client) PullWithDetails(repoPath string) (*PullResult, error) {
	result := &PullResult{Success: true}

	// Check for uncommitted changes first
	status, err := c.GetStatus(repoPath)
	if err == nil && status.IsDirty {
		result.Success = false
		result.Message = "Cannot pull: you have uncommitted changes. Please commit or stash your changes first."
		return result, nil
	}

	cmd := exec.Command("git", "-C", repoPath, "pull", "--stat")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		result.Success = false
		result.Message = strings.TrimSpace(outputStr)
		return result, nil
	}

	// Check if already up to date
	if strings.Contains(outputStr, "Already up to date") {
		result.Message = "Already up to date"
		return result, nil
	}

	// Parse the stats line like: "3 files changed, 10 insertions(+), 5 deletions(-)"
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "files changed") || strings.Contains(line, "file changed") {
			fmt.Sscanf(line, " %d file", &result.FilesChanged)
			if strings.Contains(line, "insertion") {
				fmt.Sscanf(line[strings.Index(line, ",")+1:], " %d insertion", &result.Insertions)
			}
			if strings.Contains(line, "deletion") {
				lastComma := strings.LastIndex(line, ",")
				if lastComma > 0 {
					fmt.Sscanf(line[lastComma+1:], " %d deletion", &result.Deletions)
				}
			}
		}
	}

	result.Message = fmt.Sprintf("Updated: %d files changed", result.FilesChanged)
	return result, nil
}
