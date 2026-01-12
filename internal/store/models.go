package store

import "time"

type Repo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	GitHubURL    string    `json:"github_url,omitempty"`
	StartCommand string    `json:"start_command,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Port struct {
	Port         int        `json:"port"`
	RepoID       string     `json:"repo_id,omitempty"`
	RepoName     string     `json:"repo_name,omitempty"`
	PID          int        `json:"pid,omitempty"`
	ProcessName  string     `json:"process_name,omitempty"`
	ShareMode    string     `json:"share_mode"` // "private", "password", "public"
	PasswordHash string     `json:"-"`
	FirstSeen    time.Time  `json:"first_seen"`
	LastSeen     time.Time  `json:"last_seen"`
}

type AccessLog struct {
	ID            int64     `json:"id"`
	Port          int       `json:"port"`
	IP            string    `json:"ip"`
	UserAgent     string    `json:"user_agent"`
	Timestamp     time.Time `json:"timestamp"`
	Authenticated bool      `json:"authenticated"`
}

