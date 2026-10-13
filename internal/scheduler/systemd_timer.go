package scheduler

import (
	"fmt"
	"strconv"
	"strings"
)

var cronDayOfWeekNames = map[int]string{
	0: "Sun",
	1: "Mon",
	2: "Tue",
	3: "Wed",
	4: "Thu",
	5: "Fri",
	6: "Sat",
	7: "Sun",
}

// CronToOnCalendar converts a 5-field cron expression into systemd OnCalendar syntax.
func CronToOnCalendar(cronExpr string) (string, error) {
	// Normalize multiple spaces/tabs before parsing
	normalized := strings.Join(strings.Fields(strings.TrimSpace(cronExpr)), " ")
	fields, err := ParseCron(normalized)
	if err != nil {
		return "", err
	}

	dow := "*"
	if fields.DayOfWeek != "*" {
		mappedDow, err := mapCronDayOfWeek(fields.DayOfWeek)
		if err != nil {
			return "", err
		}
		dow = mappedDow
	}

	month := padTwoDigits(fields.Month)
	day := padTwoDigits(fields.DayOfMonth)
	hour := padTwoDigits(fields.Hour)
	minute := padTwoDigits(fields.Minute)

	datePart := fmt.Sprintf("*-%s-%s", month, day)
	timePart := fmt.Sprintf("%s:%s:00", hour, minute)

	if dow != "*" {
		return fmt.Sprintf("%s %s %s", dow, datePart, timePart), nil
	}
	return fmt.Sprintf("%s %s", datePart, timePart), nil
}

func mapCronDayOfWeek(dow string) (string, error) {
	if dow == "*" {
		return "*", nil
	}
	if strings.Contains(dow, "/") {
		return "", fmt.Errorf("step values not supported for day of week: %s", dow)
	}

	parts := strings.Split(dow, ",")
	var mapped []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			lo, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return "", fmt.Errorf("invalid day of week: %s", part)
			}
			hi, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return "", fmt.Errorf("invalid day of week: %s", part)
			}
			loName, okLo := cronDayOfWeekNames[lo]
			hiName, okHi := cronDayOfWeekNames[hi]
			if !okLo || !okHi {
				return "", fmt.Errorf("day of week out of range: %s", part)
			}
			mapped = append(mapped, fmt.Sprintf("%s..%s", loName, hiName))
		} else {
			val, err := strconv.Atoi(part)
			if err != nil {
				return "", fmt.Errorf("invalid day of week: %s", part)
			}
			name, ok := cronDayOfWeekNames[val]
			if !ok {
				return "", fmt.Errorf("day of week out of range: %d", val)
			}
			mapped = append(mapped, name)
		}
	}
	return strings.Join(mapped, ","), nil
}

func padTwoDigits(s string) string {
	if s == "*" {
		return "*"
	}
	if strings.HasPrefix(s, "*/") {
		step := strings.TrimPrefix(s, "*/")
		return fmt.Sprintf("00/%s", step)
	}
	if strings.Contains(s, "/") {
		parts := strings.SplitN(s, "/", 2)
		return fmt.Sprintf("%s/%s", padTwoDigits(parts[0]), parts[1])
	}
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		var out []string
		for _, p := range parts {
			out = append(out, padTwoDigits(strings.TrimSpace(p)))
		}
		return strings.Join(out, ",")
	}
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		return fmt.Sprintf("%s..%s", padTwoDigits(parts[0]), padTwoDigits(parts[1]))
	}
	if val, err := strconv.Atoi(s); err == nil {
		return fmt.Sprintf("%02d", val)
	}
	return s
}
