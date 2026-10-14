package format

import (
	"fmt"
	"strconv"
	"strings"
)

// HumanSize converts a byte count into a decimal human-readable string (e.g. 1.5 GB).
func HumanSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	size := float64(bytes) / 1024
	i := 0
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", size, units[i])
}

// ParseBytes converts strings like "100MB", "1.5GB", "10M" into raw int64 byte counts.
func ParseBytes(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, nil
	}
	var mult int64 = 1
	switch {
	case strings.HasSuffix(s, "PB"):
		mult = 1024 * 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "PB")
	case strings.HasSuffix(s, "TB"):
		mult = 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "TB")
	case strings.HasSuffix(s, "GB"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "G"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "G")
	case strings.HasSuffix(s, "MB"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "M"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "KB"):
		mult = 1024
		s = strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "K"):
		mult = 1024
		s = strings.TrimSuffix(s, "K")
	case strings.HasSuffix(s, "B"):
		mult = 1
		s = strings.TrimSuffix(s, "B")
	}

	cleanStr := strings.TrimSpace(s)
	if cleanStr == "" {
		return 0, fmt.Errorf("invalid byte size: missing numeric value")
	}

	val, err := strconv.ParseFloat(cleanStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte size %q: %w", s, err)
	}
	if val < 0 {
		return 0, fmt.Errorf("invalid byte size %q: cannot be negative", s)
	}
	return int64(val * float64(mult)), nil
}
