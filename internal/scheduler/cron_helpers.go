package scheduler

import (
	"bytes"
	"fmt"
	"strings"
)

const cronMarker = "# devclean-scheduled"

func removeCronEntry(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	skip := false
	for _, line := range lines {
		if line == cronMarker {
			skip = true
			continue
		}
		if skip {
			skip = false
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func extractCronExpr(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if line == cronMarker && i+1 < len(lines) {
			parts := strings.Fields(lines[i+1])
			if len(parts) >= 5 {
				return strings.Join(parts[:5], " ")
			}
		}
	}
	return ""
}

func extractBinaryPath(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if line == cronMarker && i+1 < len(lines) {
			parts := strings.Fields(lines[i+1])
			if len(parts) >= 6 {
				return parts[5]
			}
		}
	}
	return ""
}

// GenerateCronEntry creates a crontab entry string for devclean.
func GenerateCronEntry(cronExpr, binaryPath string) string {
	var buf bytes.Buffer
	buf.WriteString("\n")
	buf.WriteString(cronMarker)
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("%s %s schedule run\n", cronExpr, binaryPath))
	return buf.String()
}

// RemoveCronEntryFromContent removes devclean entries from crontab content.
func RemoveCronEntryFromContent(content string) string {
	return removeCronEntry(content)
}
