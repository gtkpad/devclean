package cmd

import (
	"testing"
	"time"
)

func TestParseDuration_Days(t *testing.T) {
	d, err := parseDuration("7d")
	if err != nil {
		t.Fatalf("parseDuration(7d) failed: %v", err)
	}
	expected := 7 * 24 * time.Hour
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestParseDuration_Hours(t *testing.T) {
	d, err := parseDuration("2h")
	if err != nil {
		t.Fatalf("parseDuration(2h) failed: %v", err)
	}
	if d != 2*time.Hour {
		t.Errorf("expected 2h, got %v", d)
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	_, err := parseDuration("abc")
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}
