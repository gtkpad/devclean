package scheduler

import (
	"bufio"
	"io"
	"strconv"
	"strings"
	"time"
)

// LogRun represents an individual structured DevClean execution cycle parsed from logs.
type LogRun struct {
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Version      string    `json:"version"`
	Args         []string  `json:"args"`
	Status       string    `json:"status"` // "OK", "PARTIAL", "FAIL"
	TargetsCount int       `json:"targets_count"`
	BytesFreed   int64     `json:"bytes_freed"`
	FreedHuman   string    `json:"freed_human"`
	Duration     string    `json:"duration"`
	Body         string    `json:"body,omitempty"`
	ErrorLines   []string  `json:"error_lines,omitempty"`
}

const timeFormat = "2006-01-02 15:04:05"

// ParseLogRuns parses an input stream containing schedule.log entries,
// identifying delimited === [START] === and === [END] === blocks,
// extracting metadata, execution metrics, and error lines.
func ParseLogRuns(r io.Reader) ([]LogRun, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var runs []LogRun
	var currentRun *LogRun
	var bodyLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if isStartMarker(line) {
			if currentRun != nil {
				// Incomplete previous run
				currentRun.Body = strings.Join(bodyLines, "\n")
				currentRun.Status = "FAIL"
				currentRun.ErrorLines = extractErrorLines(bodyLines)
				runs = append(runs, *currentRun)
			}

			currentRun = parseStartMarker(line)
			bodyLines = nil
			continue
		}

		if isEndMarker(line) {
			if currentRun != nil {
				populateEndMarker(currentRun, line)
				currentRun.Body = strings.Join(bodyLines, "\n")
				currentRun.ErrorLines = extractErrorLines(bodyLines)
				runs = append(runs, *currentRun)
				currentRun = nil
				bodyLines = nil
			}
			continue
		}

		if currentRun != nil {
			bodyLines = append(bodyLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if currentRun != nil {
		// EOF encountered before end marker
		currentRun.Body = strings.Join(bodyLines, "\n")
		if currentRun.Status == "" {
			currentRun.Status = "FAIL"
		}
		currentRun.ErrorLines = extractErrorLines(bodyLines)
		runs = append(runs, *currentRun)
	}

	return runs, nil
}

func isStartMarker(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "=== [START]") && strings.HasSuffix(trimmed, "===")
}

func isEndMarker(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "=== [END]") && strings.HasSuffix(trimmed, "===")
}

func parseStartMarker(line string) *LogRun {
	run := &LogRun{}
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "=== [START]")
	trimmed = strings.TrimSuffix(trimmed, "===")
	trimmed = strings.TrimSpace(trimmed)

	parts := strings.Split(trimmed, "|")
	if len(parts) > 0 {
		if t, err := time.Parse(timeFormat, strings.TrimSpace(parts[0])); err == nil {
			run.StartTime = t
		}
	}

	for _, part := range parts[1:] {
		p := strings.TrimSpace(part)
		if strings.HasPrefix(p, "version=") {
			run.Version = strings.TrimPrefix(p, "version=")
		} else if strings.HasPrefix(p, "args=") {
			argsRaw := strings.TrimPrefix(p, "args=")
			argsRaw = strings.Trim(argsRaw, "[]")
			argsRaw = strings.TrimSpace(argsRaw)
			if argsRaw != "" {
				run.Args = strings.Fields(argsRaw)
			}
		}
	}

	return run
}

func populateEndMarker(run *LogRun, line string) {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "=== [END]")
	trimmed = strings.TrimSuffix(trimmed, "===")
	trimmed = strings.TrimSpace(trimmed)

	parts := strings.Split(trimmed, "|")
	if len(parts) > 0 {
		if t, err := time.Parse(timeFormat, strings.TrimSpace(parts[0])); err == nil {
			run.EndTime = t
		}
	}

	for _, part := range parts[1:] {
		p := strings.TrimSpace(part)
		if strings.HasPrefix(p, "status=") {
			run.Status = strings.TrimPrefix(p, "status=")
		} else if strings.HasPrefix(p, "targets=") {
			if count, err := strconv.Atoi(strings.TrimPrefix(p, "targets=")); err == nil {
				run.TargetsCount = count
			}
		} else if strings.HasPrefix(p, "freed=") {
			freedStr := strings.TrimPrefix(p, "freed=")
			run.FreedHuman = freedStr
			run.BytesFreed = ParseFreedBytes(freedStr)
		} else if strings.HasPrefix(p, "duration=") {
			run.Duration = strings.TrimPrefix(p, "duration=")
		}
	}
}

// ParseFreedBytes parses a human-readable size string (e.g., "145.2 MB", "0 B") into bytes.
func ParseFreedBytes(s string) int64 {
	s = strings.TrimSpace(s)
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return 0
	}

	val, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}

	if len(parts) == 1 {
		return int64(val)
	}

	unit := strings.ToUpper(parts[1])
	var multiplier float64 = 1
	switch unit {
	case "B":
		multiplier = 1
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	return int64(val * multiplier)
}

func extractErrorLines(lines []string) []string {
	var errLines []string
	for _, l := range lines {
		lower := strings.ToLower(l)
		if strings.Contains(l, "[ERROR]") ||
			strings.Contains(lower, "error:") ||
			strings.Contains(lower, "permission denied") {
			errLines = append(errLines, l)
		}
	}
	return errLines
}
