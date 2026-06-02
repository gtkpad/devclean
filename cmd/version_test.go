package cmd

import (
	"bytes"
	"testing"
)

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
