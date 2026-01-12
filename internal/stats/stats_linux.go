//go:build linux

package stats

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// CPUStats holds CPU timing info for calculating usage
type cpuTimes struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

var lastCPU cpuTimes
var lastCPUTime time.Time

// Get returns current system statistics
func Get() *Stats {
	stats := &Stats{}

	// Memory from /proc/meminfo
	stats.MemoryPercent, stats.MemoryUsedGB, stats.MemoryTotalGB = getMemoryStats()

	// CPU from /proc/stat
	stats.CPUPercent = getCPUStats()

	// Disk from syscall.Statfs
	stats.DiskPercent, stats.DiskUsedGB, stats.DiskTotalGB = getDiskStats("/")

	return stats
}

func getMemoryStats() (percent, usedGB, totalGB float64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0
	}
	defer file.Close()

	var memTotal, memAvailable uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			memTotal = value
		case "MemAvailable:":
			memAvailable = value
		}
	}

	if memTotal == 0 {
		return 0, 0, 0
	}

	// /proc/meminfo values are in kB
	totalGB = float64(memTotal) / 1024 / 1024
	usedGB = float64(memTotal-memAvailable) / 1024 / 1024
	percent = (float64(memTotal-memAvailable) / float64(memTotal)) * 100

	return percent, usedGB, totalGB
}

func getCPUStats() float64 {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 8 {
				return 0
			}

			current := cpuTimes{
				user:    parseUint(fields[1]),
				nice:    parseUint(fields[2]),
				system:  parseUint(fields[3]),
				idle:    parseUint(fields[4]),
				iowait:  parseUint(fields[5]),
				irq:     parseUint(fields[6]),
				softirq: parseUint(fields[7]),
			}
			if len(fields) > 8 {
				current.steal = parseUint(fields[8])
			}

			// Calculate CPU usage since last sample
			now := time.Now()
			if lastCPUTime.IsZero() {
				lastCPU = current
				lastCPUTime = now
				return 0
			}

			// Total time elapsed
			prevTotal := lastCPU.user + lastCPU.nice + lastCPU.system + lastCPU.idle +
				lastCPU.iowait + lastCPU.irq + lastCPU.softirq + lastCPU.steal
			currTotal := current.user + current.nice + current.system + current.idle +
				current.iowait + current.irq + current.softirq + current.steal

			totalDelta := float64(currTotal - prevTotal)
			if totalDelta == 0 {
				return 0
			}

			// Idle time
			prevIdle := lastCPU.idle + lastCPU.iowait
			currIdle := current.idle + current.iowait
			idleDelta := float64(currIdle - prevIdle)

			// Update last sample
			lastCPU = current
			lastCPUTime = now

			// CPU usage = (total - idle) / total * 100
			return ((totalDelta - idleDelta) / totalDelta) * 100
		}
	}
	return 0
}

func getDiskStats(path string) (percent, usedGB, totalGB float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0
	}

	// Calculate sizes in bytes, then convert to GB
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes

	totalGB = float64(totalBytes) / 1024 / 1024 / 1024
	usedGB = float64(usedBytes) / 1024 / 1024 / 1024

	if totalBytes > 0 {
		percent = (float64(usedBytes) / float64(totalBytes)) * 100
	}

	return percent, usedGB, totalGB
}

func parseUint(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}
