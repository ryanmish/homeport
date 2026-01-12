//go:build darwin

package stats

// Get returns system statistics (stub for macOS dev mode)
// Real stats are only implemented for Linux production
func Get() *Stats {
	return &Stats{
		CPUPercent:     0,
		MemoryPercent:  0,
		MemoryUsedGB:   0,
		MemoryTotalGB:  0,
		DiskPercent:    0,
		DiskUsedGB:     0,
		DiskTotalGB:    0,
	}
}
