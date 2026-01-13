//go:build linux

package scanner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// scanPorts parses /proc/net/tcp to find listening ports on Linux
func scanPorts(minPort, maxPort int) ([]RawPort, error) {
	ports, err := scanProcNetTCP("/proc/net/tcp", minPort, maxPort)
	if err != nil {
		return nil, err
	}

	// Also check IPv6
	ports6, err := scanProcNetTCP("/proc/net/tcp6", minPort, maxPort)
	if err == nil {
		// Merge, avoiding duplicates
		seen := make(map[int]bool)
		for _, p := range ports {
			seen[p.Port] = true
		}
		for _, p := range ports6 {
			if !seen[p.Port] {
				ports = append(ports, p)
				seen[p.Port] = true
			}
		}
	}

	return ports, nil
}

func scanProcNetTCP(path string, minPort, maxPort int) ([]RawPort, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ports []RawPort
	scanner := bufio.NewScanner(file)

	// Skip header line
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 12 {
			continue
		}

		// Field 1: local_address (hex IP:port)
		// Field 3: st (state) - 0A = LISTEN
		localAddr := fields[1]
		state := fields[3]

		// Only interested in LISTEN state
		if state != "0A" {
			continue
		}

		// Parse port from local_address
		port, err := parseHexPort(localAddr)
		if err != nil {
			continue
		}

		if port < minPort || port > maxPort {
			continue
		}

		// Field 9 is inode, we can use it to find the process
		inode := fields[9]
		pid, processName := findProcessByInode(inode)

		ports = append(ports, RawPort{
			Port:        port,
			PID:         pid,
			ProcessName: processName,
		})
	}

	return ports, nil
}

// parseHexPort extracts port from hex address like "0100007F:0BB8"
func parseHexPort(addr string) (int, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid address format")
	}

	port64, err := strconv.ParseInt(parts[1], 16, 32)
	if err != nil {
		return 0, err
	}

	return int(port64), nil
}

// findProcessByInode searches /proc/*/fd/* for a socket with the given inode
func findProcessByInode(inode string) (int, string) {
	procs, err := os.ReadDir("/proc")
	if err != nil {
		return 0, ""
	}

	socketLink := fmt.Sprintf("socket:[%s]", inode)

	for _, proc := range procs {
		if !proc.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(proc.Name())
		if err != nil {
			continue
		}

		fdPath := filepath.Join("/proc", proc.Name(), "fd")
		fds, err := os.ReadDir(fdPath)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdPath, fd.Name()))
			if err != nil {
				continue
			}

			if link == socketLink {
				// Found the process, get its name
				commPath := filepath.Join("/proc", proc.Name(), "comm")
				comm, err := os.ReadFile(commPath)
				if err != nil {
					return pid, ""
				}
				return pid, strings.TrimSpace(string(comm))
			}
		}
	}

	return 0, ""
}

// getProcessCWD reads /proc/<pid>/cwd symlink
func getProcessCWD(pid int) (string, error) {
	cwdPath := fmt.Sprintf("/proc/%d/cwd", pid)
	cwd, err := os.Readlink(cwdPath)
	if err != nil {
		return "", err
	}
	return cwd, nil
}

// getProcessCommand reads /proc/<pid>/cmdline to get the full command
func getProcessCommand(pid int) (string, error) {
	cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)
	data, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return "", err
	}
	// cmdline is null-separated, replace with spaces
	cmd := strings.ReplaceAll(string(data), "\x00", " ")
	return strings.TrimSpace(cmd), nil
}
