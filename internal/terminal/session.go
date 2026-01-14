package terminal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
)

const (
	// MaxScrollback is the maximum size of the scrollback buffer (100KB)
	MaxScrollback = 100 * 1024
)

// Alternate screen escape sequences (smcup/rmcup)
var (
	altScreenEnter = [][]byte{
		[]byte("\x1b[?1049h"), // Most common (xterm)
		[]byte("\x1b[?47h"),   // Legacy
		[]byte("\x1b[?1047h"), // Another variant
	}
	altScreenExit = [][]byte{
		[]byte("\x1b[?1049l"),
		[]byte("\x1b[?47l"),
		[]byte("\x1b[?1047l"),
	}
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

	// Scrollback buffer for replay on reconnect
	scrollback   []byte
	scrollbackMu sync.RWMutex

	// Track alternate screen mode (TUI apps use this)
	// When in alternate screen, we don't save to scrollback
	inAltScreen bool

	// Subscribers for live output broadcast
	subscribers   []chan []byte
	subscribersMu sync.Mutex
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

	// Set default size (larger default to accommodate TUI apps before resize)
	pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 120})

	session := &Session{
		ID:          uuid.New().String(),
		RepoID:      repoID,
		RepoPath:    repoPath,
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
		ptmx:        ptmx,
		cmd:         cmd,
		scrollback:  make([]byte, 0, MaxScrollback),
		subscribers: make([]chan []byte, 0),
	}

	// Start background reader to capture output
	go session.readLoop()

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

// readLoop continuously reads from PTY and broadcasts to subscribers
func (s *Session) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			// Check for alternate screen mode transitions
			s.scrollbackMu.Lock()
			for _, seq := range altScreenEnter {
				if bytes.Contains(data, seq) {
					s.inAltScreen = true
					break
				}
			}
			for _, seq := range altScreenExit {
				if bytes.Contains(data, seq) {
					s.inAltScreen = false
					break
				}
			}

			// Only add to scrollback when NOT in alternate screen mode
			// TUI apps (like Claude Code) use alternate screen and shouldn't
			// corrupt the scrollback buffer
			if !s.inAltScreen {
				s.scrollback = append(s.scrollback, data...)
				// Trim if exceeds max size
				if len(s.scrollback) > MaxScrollback {
					s.scrollback = s.scrollback[len(s.scrollback)-MaxScrollback:]
				}
			}
			s.scrollbackMu.Unlock()

			// Broadcast to all subscribers
			s.subscribersMu.Lock()
			for _, ch := range s.subscribers {
				select {
				case ch <- data:
				default:
					// Skip slow subscribers
				}
			}
			s.subscribersMu.Unlock()

			s.mu.Lock()
			s.LastUsed = time.Now()
			s.mu.Unlock()
		}
	}
}

// Subscribe returns a channel that receives terminal output
func (s *Session) Subscribe() chan []byte {
	ch := make(chan []byte, 256)
	s.subscribersMu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.subscribersMu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel
func (s *Session) Unsubscribe(ch chan []byte) {
	s.subscribersMu.Lock()
	defer s.subscribersMu.Unlock()
	for i, sub := range s.subscribers {
		if sub == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// GetScrollback returns the current scrollback buffer
func (s *Session) GetScrollback() []byte {
	s.scrollbackMu.RLock()
	defer s.scrollbackMu.RUnlock()
	result := make([]byte, len(s.scrollback))
	copy(result, s.scrollback)
	return result
}

// Read reads from the PTY (deprecated - use Subscribe instead)
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
