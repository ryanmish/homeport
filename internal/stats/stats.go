package stats

// Stats holds system resource statistics
type Stats struct {
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryPercent  float64 `json:"memory_percent"`
	MemoryUsedGB   float64 `json:"memory_used_gb"`
	MemoryTotalGB  float64 `json:"memory_total_gb"`
	DiskPercent    float64 `json:"disk_percent"`
	DiskUsedGB     float64 `json:"disk_used_gb"`
	DiskTotalGB    float64 `json:"disk_total_gb"`
}
