package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
)

// Session represents a terminal session with a PTY
type Session struct {
	ID        string    `json:"id"`
	RepoID    string    `json:"repo_id"`
	RepoPath  string    `json:"repo_path"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used"`

	ptmx    *os.File
	cmd     *exec.Cmd
	mu      sync.Mutex
	closed  bool
	clients int // number of connected WebSocket clients
}

// Manager manages terminal sessions
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new terminal session manager
func NewManager() *Manager {
	m := &Manager{
		sessions: make(map[string]*Session),
	}

	// Start cleanup goroutine to remove idle sessions
	go m.cleanupLoop()

	return m
}

// CreateSession creates a new terminal session for a repo
func (m *Manager) CreateSession(repoID, repoPath string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Determine shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
		if _, err := os.Stat(shell); os.IsNotExist(err) {
			shell = "/bin/sh"
		}
	}

	// Start PTY
	cmd := exec.Command(shell)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	// Set default size
	pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})

	session := &Session{
		ID:        uuid.New().String(),
		RepoID:    repoID,
		RepoPath:  repoPath,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		ptmx:      ptmx,
		cmd:       cmd,
	}

	m.sessions[session.ID] = session

	return session, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// ListSessions returns all sessions for a repo
func (m *Manager) ListSessions(repoID string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*Session
	for _, s := range m.sessions {
		if s.RepoID == repoID && !s.closed {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// DeleteSession closes and removes a session
func (m *Manager) DeleteSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[id]; ok {
		session.Close()
		delete(m.sessions, id)
	}
}

// cleanupLoop removes idle sessions periodically
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		for id, session := range m.sessions {
			// Remove sessions idle for more than 30 minutes with no clients
			if session.clients == 0 && time.Since(session.LastUsed) > 30*time.Minute {
				session.Close()
				delete(m.sessions, id)
			}
			// Remove closed sessions
			if session.closed {
				delete(m.sessions, id)
			}
		}
		m.mu.Unlock()
	}
}

// Read reads from the PTY
func (s *Session) Read(p []byte) (int, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, io.EOF
	}
	s.LastUsed = time.Now()
	s.mu.Unlock()

	return s.ptmx.Read(p)
}

// Write writes to the PTY
func (s *Session) Write(p []byte) (int, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, io.EOF
	}
	s.LastUsed = time.Now()
	s.mu.Unlock()

	return s.ptmx.Write(p)
}

// Resize resizes the PTY
func (s *Session) Resize(cols, rows uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("session closed")
	}

	s.LastUsed = time.Now()
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

// AddClient increments the client count
func (s *Session) AddClient() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients++
	s.LastUsed = time.Now()
}

// RemoveClient decrements the client count
func (s *Session) RemoveClient() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients--
	if s.clients < 0 {
		s.clients = 0
	}
}

// Close closes the session
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	s.ptmx.Close()
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}
}

// IsClosed returns whether the session is closed
func (s *Session) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}
