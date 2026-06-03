package config

import (
	"strings"
	"testing"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error for defaults, got: %v", err)
	}
}

func TestValidate_InvalidHookTimeout(t *testing.T) {
	cfg := Defaults()
	cfg.Hooks.Node.Timeout = "xyz"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should mention timeout: %v", err)
	}
}

func TestValidate_InvalidPlatform(t *testing.T) {
	cfg := Defaults()
	cfg.Hooks.Go.Platform = []string{"windows95"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid platform")
	}
	if !strings.Contains(err.Error(), "windows95") {
		t.Errorf("error should mention platform: %v", err)
	}
}

func TestValidate_EmptyScanPath(t *testing.T) {
	cfg := Defaults()
	cfg.ScanPaths = []string{"/valid", ""}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty scan path")
	}
	if !strings.Contains(err.Error(), "empty path") {
		t.Errorf("error should mention empty path: %v", err)
	}
}
