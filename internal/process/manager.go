package process

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Process represents a running dev server process
type Process struct {
	RepoID    string    `json:"repo_id"`
	RepoName  string    `json:"repo_name"`
	Command   string    `json:"command"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"` // "running", "stopped", "failed"

	cmd        *exec.Cmd
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	logs       []LogEntry
	logsMu     sync.RWMutex
	maxLogSize int
}

// LogEntry represents a log line from a process
type LogEntry struct {
	Time    time.Time `json:"time"`
	Stream  string    `json:"stream"` // "stdout" or "stderr"
	Message string    `json:"message"`
}

// Manager manages dev server processes
type Manager struct {
	processes map[string]*Process // keyed by repo ID
	mu        sync.RWMutex
}

// NewManager creates a new process manager
func NewManager() *Manager {
	return &Manager{
		processes: make(map[string]*Process),
	}
}

// Start starts a dev server for a repo
func (m *Manager) Start(repoID, repoName, repoPath, command string) (*Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if proc, exists := m.processes[repoID]; exists && proc.Status == "running" {
		return nil, fmt.Errorf("process already running for repo %s", repoName)
	}

	// Parse command - use shell to handle complex commands
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"FORCE_COLOR=1",
		"TERM=xterm-256color",
	)

	// Set up process group so we can kill all children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr: %w", err)
	}

	proc := &Process{
		RepoID:     repoID,
		RepoName:   repoName,
		Command:    command,
		StartedAt:  time.Now(),
		Status:     "running",
		cmd:        cmd,
		stdout:     stdout,
		stderr:     stderr,
		logs:       make([]LogEntry, 0, 1000),
		maxLogSize: 1000,
	}

	if err := cmd.Start(); err != nil {
		proc.Status = "failed"
		return nil, fmt.Errorf("failed to start: %w", err)
	}

	proc.PID = cmd.Process.Pid
	m.processes[repoID] = proc

	// Capture logs in background
	go proc.captureLogs("stdout", stdout)
	go proc.captureLogs("stderr", stderr)

	// Monitor process in background
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		if err != nil {
			proc.Status = "failed"
		} else {
			proc.Status = "stopped"
		}
		m.mu.Unlock()
	}()

	return proc, nil
}

// Stop stops a dev server for a repo
func (m *Manager) Stop(repoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proc, exists := m.processes[repoID]
	if !exists {
		return fmt.Errorf("no process found for repo")
	}

	if proc.Status != "running" {
		return fmt.Errorf("process is not running")
	}

	// Kill the process group (includes all child processes)
	pgid, err := syscall.Getpgid(proc.PID)
	if err == nil {
		syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		proc.cmd.Process.Signal(syscall.SIGTERM)
	}

	// Give it 5 seconds to gracefully shutdown
	done := make(chan bool, 1)
	go func() {
		proc.cmd.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Process exited gracefully
	case <-time.After(5 * time.Second):
		// Force kill
		if pgid, err := syscall.Getpgid(proc.PID); err == nil {
			syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			proc.cmd.Process.Kill()
		}
	}

	proc.Status = "stopped"
	return nil
}

// Get returns a process by repo ID
func (m *Manager) Get(repoID string) *Process {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.processes[repoID]
}

// List returns all processes
func (m *Manager) List() []*Process {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Process, 0, len(m.processes))
	for _, p := range m.processes {
		result = append(result, p)
	}
	return result
}

// GetLogs returns recent logs for a process
func (m *Manager) GetLogs(repoID string, limit int) []LogEntry {
	m.mu.RLock()
	proc, exists := m.processes[repoID]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	proc.logsMu.RLock()
	defer proc.logsMu.RUnlock()

	if limit <= 0 || limit > len(proc.logs) {
		limit = len(proc.logs)
	}

	// Return last N entries
	start := len(proc.logs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]LogEntry, limit)
	copy(result, proc.logs[start:])
	return result
}

// captureLogs reads from a pipe and stores log entries
func (p *Process) captureLogs(stream string, reader io.ReadCloser) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		entry := LogEntry{
			Time:    time.Now(),
			Stream:  stream,
			Message: scanner.Text(),
		}

		p.logsMu.Lock()
		p.logs = append(p.logs, entry)
		// Trim if too large
		if len(p.logs) > p.maxLogSize {
			p.logs = p.logs[len(p.logs)-p.maxLogSize:]
		}
		p.logsMu.Unlock()
	}
}
