package cmd

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestParseBytes(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"2GB", 2 * 1024 * 1024 * 1024, false},
		{"500MB", 500 * 1024 * 1024, false},
		{"1TB", 1024 * 1024 * 1024 * 1024, false},
		{"1024", 1024, false},
		{"512B", 512, false},
		{"1.5GB", int64(1.5 * 1024 * 1024 * 1024), false},
		{"", 0, true},
		{"invalid", 0, true},
		{"100XYZ", 0, true},
		{"abcGB", 0, true},
	}


	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseBytes(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseBytes(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBytes(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseBytes(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestWatchCmd_IntervalFlag_TriggersDaemon(t *testing.T) {
	cfgFile := createEmptyConfig(t)
	origConfigPath := configPath
	configPath = cfgFile
	defer func() { configPath = origConfigPath }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cmd := newWatchCmd()
	cmd.SetArgs([]string{"--interval", "10ms"})

	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Fatalf("expected graceful shutdown without error, got: %v", err)
	}
}

func TestWatchCmd_InvalidInterval_ReturnsError(t *testing.T) {
	cfgFile := createEmptyConfig(t)
	origConfigPath := configPath
	configPath = cfgFile
	defer func() { configPath = origConfigPath }()

	cmd := newWatchCmd()
	cmd.SetArgs([]string{"--interval", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid interval, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --interval value") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestWatchCmd_OneShotMode(t *testing.T) {
	cfgFile := createEmptyConfig(t)
	origConfigPath := configPath
	configPath = cfgFile
	defer func() { configPath = origConfigPath }()

	cmd := newWatchCmd()
	cmd.SetArgs([]string{"--threshold", "100GB"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error in watch one-shot mode: %v", err)
	}

	// Invalid threshold flag
	cmdInvalid := newWatchCmd()
	cmdInvalid.SetArgs([]string{"--threshold", "invalid"})
	if err := cmdInvalid.Execute(); err == nil {
		t.Error("expected error for invalid threshold, got nil")
	}
}

func TestWatchCmd_AboveThreshold(t *testing.T) {
	_, cfgPath := createTestWorkspace(t)
	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	cmd := newWatchCmd()
	// Set threshold to 1B so node_modules will exceed it
	cmd.SetArgs([]string{"--threshold", "1B"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected watch one-shot above threshold to succeed, got: %v", err)
	}
}


