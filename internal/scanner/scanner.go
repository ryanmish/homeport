package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gethomeport/homeport/internal/store"
)

type Scanner struct {
	minPort  int
	maxPort  int
	reposDir string
}

func New(minPort, maxPort int, reposDir string) *Scanner {
	return &Scanner{
		minPort:  minPort,
		maxPort:  maxPort,
		reposDir: reposDir,
	}
}

// Scan detects listening ports in the configured range
// Uses platform-specific implementation (lsof on Mac, /proc on Linux)
func (s *Scanner) Scan() ([]store.Port, error) {
	rawPorts, err := scanPorts(s.minPort, s.maxPort)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	ports := make([]store.Port, 0, len(rawPorts))

	for _, rp := range rawPorts {
		p := store.Port{
			Port:        rp.Port,
			PID:         rp.PID,
			ProcessName: rp.ProcessName,
			Command:     rp.Command,
			ShareMode:   "private",
			FirstSeen:   now,
			LastSeen:    now,
		}

		// Try to associate with a repo by checking process CWD
		if rp.PID > 0 {
			if repoID := s.findRepoForPID(rp.PID); repoID != "" {
				p.RepoID = repoID
			}
			// Get full command if not already set
			if p.Command == "" {
				if cmd, err := getProcessCommand(rp.PID); err == nil {
					p.Command = cmd
				}
			}
		}

		ports = append(ports, p)
	}

	return ports, nil
}

// findRepoForPID checks if the process is running from within a repo directory
func (s *Scanner) findRepoForPID(pid int) string {
	cwd, err := getProcessCWD(pid)
	if err != nil {
		return ""
	}

	// Normalize paths
	cwd = filepath.Clean(cwd)
	reposDir := filepath.Clean(s.reposDir)

	// Check if CWD is under repos directory
	if !strings.HasPrefix(cwd, reposDir) {
		return ""
	}

	// Extract repo name (first directory under reposDir)
	rel, err := filepath.Rel(reposDir, cwd)
	if err != nil {
		return ""
	}

	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) == 0 || parts[0] == "." || parts[0] == "" {
		return ""
	}

	return parts[0]
}

// RawPort represents a detected listening port
type RawPort struct {
	Port        int
	PID         int
	ProcessName string
	Command     string
}

// Platform-specific functions are defined in scanner_darwin.go and scanner_linux.go:
// - scanPorts(minPort, maxPort int) ([]RawPort, error)
// - getProcessCWD(pid int) (string, error)
// - getProcessCommand(pid int) (string, error)
