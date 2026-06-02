package scheduler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// CronFields holds the parsed components of a 5-field cron expression.
type CronFields struct {
	Minute     string
	Hour       string
	DayOfMonth string
	Month      string
	DayOfWeek  string
}

// ParseCron splits a 5-field cron expression into its components.
func ParseCron(expr string) (CronFields, error) {
	if err := ValidateCron(expr); err != nil {
		return CronFields{}, err
	}
	parts := strings.Fields(expr)
	return CronFields{
		Minute:     parts[0],
		Hour:       parts[1],
		DayOfMonth: parts[2],
		Month:      parts[3],
		DayOfWeek:  parts[4],
	}, nil
}

// ValidateCron checks that a cron expression is a valid 5-field format.
// Rejects expressions containing shell injection characters.
func ValidateCron(expr string) error {
	if strings.ContainsAny(expr, ";|`&") {
		return errors.New("cron expression contains forbidden characters")
	}

	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	limits := []struct {
		name string
		min  int
		max  int
	}{
		{"minute", 0, 59},
		{"hour", 0, 23},
		{"day of month", 1, 31},
		{"month", 1, 12},
		{"day of week", 0, 7},
	}

	for i, part := range parts {
		if err := validateField(part, limits[i].min, limits[i].max, limits[i].name); err != nil {
			return err
		}
	}
	return nil
}

func validateField(field string, min, max int, name string) error {
	if field == "*" {
		return nil
	}

	// Handle step: */N or range/N
	if idx := strings.Index(field, "/"); idx >= 0 {
		base := field[:idx]
		step := field[idx+1:]
		stepVal, err := strconv.Atoi(step)
		if err != nil || stepVal < 1 {
			return fmt.Errorf("invalid step value in %s: %s", name, field)
		}
		if base == "*" {
			return nil
		}
		return validateField(base, min, max, name)
	}

	// Handle list: 1,3,5
	if strings.Contains(field, ",") {
		for _, item := range strings.Split(field, ",") {
			if err := validateField(strings.TrimSpace(item), min, max, name); err != nil {
				return err
			}
		}
		return nil
	}

	// Handle range: 1-5
	if strings.Contains(field, "-") {
		rangeParts := strings.SplitN(field, "-", 2)
		lo, err := strconv.Atoi(rangeParts[0])
		if err != nil {
			return fmt.Errorf("invalid range in %s: %s", name, field)
		}
		hi, err := strconv.Atoi(rangeParts[1])
		if err != nil {
			return fmt.Errorf("invalid range in %s: %s", name, field)
		}
		if lo < min || lo > max || hi < min || hi > max {
			return fmt.Errorf("value out of range in %s: %s (allowed %d-%d)", name, field, min, max)
		}
		if lo > hi {
			return fmt.Errorf("invalid range in %s: %d > %d", name, lo, hi)
		}
		return nil
	}

	// Single value
	val, err := strconv.Atoi(field)
	if err != nil {
		return fmt.Errorf("invalid value in %s: %s", name, field)
	}
	if val < min || val > max {
		return fmt.Errorf("value out of range in %s: %d (allowed %d-%d)", name, val, min, max)
	}
	return nil
}
