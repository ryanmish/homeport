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

// IsAuthenticated checks if gh CLI is authenticated
func (c *Client) IsAuthenticated() bool {
	cmd := exec.Command("gh", "auth", "status")
	return cmd.Run() == nil
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
