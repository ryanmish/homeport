package activity

import (
	"sync"
	"time"
)

// Entry represents a single activity log entry
type Entry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`      // "clone", "delete", "share", "unshare", "commit", "push", "pull", "start", "stop"
	RepoID    string    `json:"repo_id,omitempty"`
	RepoName  string    `json:"repo_name,omitempty"`
	Port      int       `json:"port,omitempty"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
}

// Log stores recent activity entries
type Log struct {
	entries  []Entry
	maxSize  int
	nextID   int64
	mu       sync.RWMutex
}

var globalLog = &Log{
	entries: make([]Entry, 0, 100),
	maxSize: 100,
	nextID:  1,
}

// Global returns the global activity log
func Global() *Log {
	return globalLog
}

// Add adds a new entry to the log
func (l *Log) Add(entryType, repoID, repoName string, port int, message, details string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := Entry{
		ID:        l.nextID,
		Timestamp: time.Now(),
		Type:      entryType,
		RepoID:    repoID,
		RepoName:  repoName,
		Port:      port,
		Message:   message,
		Details:   details,
	}
	l.nextID++

	l.entries = append(l.entries, entry)

	// Trim if too large
	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}
}

// Recent returns the most recent entries
func (l *Log) Recent(limit int) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > len(l.entries) {
		limit = len(l.entries)
	}

	// Return in reverse order (newest first)
	result := make([]Entry, limit)
	for i := 0; i < limit; i++ {
		result[i] = l.entries[len(l.entries)-1-i]
	}
	return result
}

// Helper functions for common activity types

func LogClone(repoName string) {
	Global().Add("clone", "", repoName, 0, "Cloned repository", "")
}

func LogDelete(repoID, repoName string) {
	Global().Add("delete", repoID, repoName, 0, "Deleted repository", "")
}

func LogShare(port int, mode string) {
	Global().Add("share", "", "", port, "Shared port", mode)
}

func LogUnshare(port int) {
	Global().Add("unshare", "", "", port, "Unshared port", "")
}

func LogCommit(repoID, repoName, hash string) {
	Global().Add("commit", repoID, repoName, 0, "Committed changes", hash)
}

func LogPush(repoID, repoName string) {
	Global().Add("push", repoID, repoName, 0, "Pushed to remote", "")
}

func LogPull(repoID, repoName string) {
	Global().Add("pull", repoID, repoName, 0, "Pulled from remote", "")
}

func LogStart(repoID, repoName string) {
	Global().Add("start", repoID, repoName, 0, "Started dev server", "")
}

func LogStop(repoID, repoName string) {
	Global().Add("stop", repoID, repoName, 0, "Stopped dev server", "")
}
