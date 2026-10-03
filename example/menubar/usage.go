package main

import (
	"fmt"
	"math"
)

// Usage is the disk space of the volume holding Path.
type Usage struct {
	Path  string `json:"path"`
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
	// UsedPercent is the share of Total in use, 0 to 100.
	UsedPercent float64 `json:"usedPercent"`
}

// readUsage measures the volume holding path.
func readUsage(path string) (Usage, error) {
	total, free, err := diskSpace(path)
	if err != nil {
		return Usage{}, fmt.Errorf("disk space of %s: %w", path, err)
	}
	u := Usage{Path: path, Total: total, Free: free}
	if total > 0 {
		u.UsedPercent = math.Round(float64(total-free)/float64(total)*1000) / 10
	}
	return u, nil
}

// Used is the space in use.
func (u Usage) Used() uint64 { return u.Total - u.Free }

// formatBytes renders n with a decimal unit, as the OS file managers do:
// 1 decimal below 10, none above (9.5 GB, 87 GB).
func formatBytes(n uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	v := float64(n)
	i := 0
	for v >= 1000 && i < len(units)-1 {
		v /= 1000
		i++
	}
	if v < 10 && i > 0 {
		return fmt.Sprintf("%.1f %s", v, units[i])
	}
	return fmt.Sprintf("%.0f %s", v, units[i])
}

// trayTitle is the status shown next to the tray icon.
func (u Usage) trayTitle() string { return formatBytes(u.Free) + " free" }

// tooltip is shown when hovering the tray icon.
func (u Usage) tooltip() string {
	return fmt.Sprintf("%s: %s of %s used (%.1f%%)", u.Path, formatBytes(u.Used()), formatBytes(u.Total), u.UsedPercent)
}
