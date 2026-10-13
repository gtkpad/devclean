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

func TestValidate_NewHooksValidation(t *testing.T) {
	t.Run("NextJS invalid timeout", func(t *testing.T) {
		cfg := Defaults()
		cfg.Hooks.NextJS.Timeout = "invalido"
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "hooks.nextjs.timeout") {
			t.Errorf("expected hooks.nextjs.timeout validation error, got: %v", err)
		}
	})

	t.Run("Xcode invalid timeout", func(t *testing.T) {
		cfg := Defaults()
		cfg.Hooks.Xcode.Timeout = "999xyz"
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "hooks.xcode.timeout") {
			t.Errorf("expected hooks.xcode.timeout validation error, got: %v", err)
		}
	})

	t.Run("AI invalid platform", func(t *testing.T) {
		cfg := Defaults()
		cfg.Hooks.AI.Platform = []string{"solaris"}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "hooks.ai.platform") {
			t.Errorf("expected hooks.ai.platform validation error, got: %v", err)
		}
	})

	t.Run("CPP invalid timeout", func(t *testing.T) {
		cfg := Defaults()
		cfg.Hooks.CPP.Timeout = "bad"
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "hooks.cpp.timeout") {
			t.Errorf("expected hooks.cpp.timeout error, got: %v", err)
		}
	})

	t.Run("Flutter invalid platform", func(t *testing.T) {
		cfg := Defaults()
		cfg.Hooks.Flutter.Platform = []string{"freebsd"}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "hooks.flutter.platform") {
			t.Errorf("expected hooks.flutter.platform error, got: %v", err)
		}
	})

	t.Run("Terraform invalid timeout", func(t *testing.T) {
		cfg := Defaults()
		cfg.Hooks.Terraform.Timeout = "invalid"
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "hooks.terraform.timeout") {
			t.Errorf("expected hooks.terraform.timeout error, got: %v", err)
		}
	})

	t.Run("CocoaPods invalid platform", func(t *testing.T) {
		cfg := Defaults()
		cfg.Hooks.CocoaPods.Platform = []string{"amiga"}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "hooks.cocoapods.platform") {
			t.Errorf("expected hooks.cocoapods.platform error, got: %v", err)
		}
	})

	t.Run("Elixir invalid timeout", func(t *testing.T) {
		cfg := Defaults()
		cfg.Hooks.Elixir.Timeout = "bad"
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "hooks.elixir.timeout") {
			t.Errorf("expected hooks.elixir.timeout error, got: %v", err)
		}
	})
}

