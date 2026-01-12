//go:build darwin

package scanner

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// scanPorts uses lsof to find listening TCP ports on macOS
func scanPorts(minPort, maxPort int) ([]RawPort, error) {
	// lsof -i -P -n -sTCP:LISTEN
	// -i: select internet connections
	// -P: don't resolve port names
	// -n: don't resolve hostnames
	// -sTCP:LISTEN: only TCP in LISTEN state
	cmd := exec.Command("lsof", "-i", "-P", "-n", "-sTCP:LISTEN")
	output, err := cmd.Output()
	if err != nil {
		// lsof returns exit code 1 if no matching files found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}

	var ports []RawPort
	seen := make(map[int]bool)

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		// Skip header
		if strings.HasPrefix(line, "COMMAND") {
			continue
		}

		// Example line:
		// node    12345 user   23u  IPv4 0x1234  0t0  TCP *:3000 (LISTEN)
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		processName := fields[0]
		pidStr := fields[1]
		nameField := fields[8] // e.g., "*:3000" or "localhost:3000"

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Parse port from name field
		port := parsePortFromLsof(nameField)
		if port < minPort || port > maxPort {
			continue
		}

		// Skip duplicates (same port can appear multiple times for IPv4/IPv6)
		if seen[port] {
			continue
		}
		seen[port] = true

		ports = append(ports, RawPort{
			Port:        port,
			PID:         pid,
			ProcessName: processName,
		})
	}

	return ports, nil
}

// parsePortFromLsof extracts port number from lsof name field
// e.g., "*:3000", "localhost:3000", "[::1]:3000"
func parsePortFromLsof(name string) int {
	// Find last colon
	idx := strings.LastIndex(name, ":")
	if idx == -1 {
		return 0
	}

	portStr := name[idx+1:]
	// Remove any trailing info like "(LISTEN)"
	portStr = strings.TrimSuffix(portStr, ")")
	if spaceIdx := strings.Index(portStr, " "); spaceIdx != -1 {
		portStr = portStr[:spaceIdx]
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}

// getProcessCWD uses lsof to get process CWD on macOS
func getProcessCWD(pid int) (string, error) {
	// lsof -a -p <pid> -d cwd -Fn
	// -a: AND conditions
	// -p: process ID
	// -d cwd: file descriptor type = current working directory
	// -Fn: output name field only
	cmd := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Output format:
	// p12345
	// n/path/to/cwd
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "n") {
			return line[1:], nil
		}
	}

	return "", fmt.Errorf("cwd not found for pid %d", pid)
}
