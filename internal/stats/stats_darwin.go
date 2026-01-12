//go:build darwin

package stats

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// CPU tracking for delta calculation
var lastCPUTimes struct {
	user   uint64
	system uint64
	idle   uint64
	total  uint64
	time   time.Time
}

// Get returns system statistics for macOS
func Get() *Stats {
	stats := &Stats{}
	stats.MemoryPercent, stats.MemoryUsedGB, stats.MemoryTotalGB = getMemoryStats()
	stats.CPUPercent = getCPUStats()
	stats.DiskPercent, stats.DiskUsedGB, stats.DiskTotalGB = getDiskStats("/")
	return stats
}

func getMemoryStats() (percent, usedGB, totalGB float64) {
	// Get total memory from sysctl
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, 0, 0
	}
	totalBytes, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, 0, 0
	}
	totalGB = float64(totalBytes) / 1024 / 1024 / 1024

	// Get memory usage from vm_stat
	out, err = exec.Command("vm_stat").Output()
	if err != nil {
		return 0, 0, totalGB
	}

	// Parse vm_stat output
	var pageSize uint64 = 4096 // default page size
	var pagesActive, pagesWired, pagesCompressed uint64

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Mach Virtual Memory Statistics") {
			// Extract page size from header
			if idx := strings.Index(line, "page size of"); idx != -1 {
				parts := strings.Fields(line[idx:])
				if len(parts) >= 4 {
					if ps, err := strconv.ParseUint(parts[3], 10, 64); err == nil {
						pageSize = ps
					}
				}
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(strings.TrimSuffix(parts[1], "."))
		val, _ := strconv.ParseUint(value, 10, 64)

		switch key {
		case "Pages active":
			pagesActive = val
		case "Pages wired down":
			pagesWired = val
		case "Pages occupied by compressor":
			pagesCompressed = val
		}
	}

	usedBytes := (pagesActive + pagesWired + pagesCompressed) * pageSize
	usedGB = float64(usedBytes) / 1024 / 1024 / 1024
	percent = (usedGB / totalGB) * 100

	return percent, usedGB, totalGB
}

func getCPUStats() float64 {
	// Use top command to get CPU usage
	out, err := exec.Command("top", "-l", "1", "-n", "0", "-stats", "cpu").Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "CPU usage:") {
			// Parse "CPU usage: X.X% user, Y.Y% sys, Z.Z% idle"
			parts := strings.Split(line, ",")
			var user, sys float64
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.HasSuffix(part, "user") {
					fields := strings.Fields(part)
					if len(fields) >= 1 {
						user, _ = strconv.ParseFloat(strings.TrimSuffix(fields[0], "%"), 64)
					}
				} else if strings.HasSuffix(part, "sys") {
					fields := strings.Fields(part)
					if len(fields) >= 1 {
						sys, _ = strconv.ParseFloat(strings.TrimSuffix(fields[0], "%"), 64)
					}
				}
			}
			return user + sys
		}
	}

	return 0
}

func getDiskStats(path string) (percent, usedGB, totalGB float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes

	totalGB = float64(totalBytes) / 1024 / 1024 / 1024
	usedGB = float64(usedBytes) / 1024 / 1024 / 1024
	percent = (float64(usedBytes) / float64(totalBytes)) * 100

	return percent, usedGB, totalGB
}
