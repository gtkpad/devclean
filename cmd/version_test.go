package cmd

import (
	"bytes"
	"testing"
)

func TestVersionCmd(t *testing.T) {
	cmd := newVersionCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("devclean version")) {
		t.Errorf("output missing 'devclean version':\n%s", out)
	}
}

func TestVersionCmd_ExportedVersionOverride(t *testing.T) {
	oldVersion := Version
	oldInternal := version
	defer func() {
		Version = oldVersion
		version = oldInternal
	}()

	version = "dev"
	Version = "v2.4.0-test"

	cmd := newVersionCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("v2.4.0-test")) {
		t.Errorf("output missing overridden Version 'v2.4.0-test':\n%s", out)
	}
}

func TestNewVersionCmd_Use(t *testing.T) {
	cmd := newVersionCmd()
	if cmd.Use != "version" {
		t.Errorf("expected Use 'version', got %q", cmd.Use)
	}
}

func TestVersionCmd_OutputContainsFields(t *testing.T) {
	cmd := newVersionCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"devclean", "version", "commit", "built"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestVersionCmd_DefaultValues(t *testing.T) {
	cmd := newVersionCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	defaults := map[string]string{
		"dev":     "default version",
		"none":    "default commit",
		"unknown": "default date",
	}
	for val, desc := range defaults {
		if !bytes.Contains([]byte(out), []byte(val)) {
			t.Errorf("output missing %s value %q:\n%s", desc, val, out)
		}
	}
}
