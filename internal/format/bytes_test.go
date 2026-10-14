package format_test

import (
	"testing"

	"github.com/gtkpad/devclean/internal/format"
)

func TestHumanSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero bytes", bytes: 0, want: "0 B"},
		{name: "512 bytes", bytes: 512, want: "512 B"},
		{name: "1023 bytes", bytes: 1023, want: "1023 B"},
		{name: "1 KB", bytes: 1024, want: "1.0 KB"},
		{name: "1.5 KB", bytes: 1536, want: "1.5 KB"},
		{name: "1 MB", bytes: 1024 * 1024, want: "1.0 MB"},
		{name: "1.5 GB", bytes: int64(1.5 * 1024 * 1024 * 1024), want: "1.5 GB"},
		{name: "1 TB", bytes: 1024 * 1024 * 1024 * 1024, want: "1.0 TB"},
		{name: "10 TB", bytes: 10 * 1024 * 1024 * 1024 * 1024, want: "10.0 TB"},
		{name: "1 PB", bytes: 1024 * 1024 * 1024 * 1024 * 1024, want: "1.0 PB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.HumanSize(tt.bytes)
			if got != tt.want {
				t.Errorf("HumanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestParseBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "empty string", input: "", want: 0, wantErr: false},
		{name: "whitespace only", input: "   ", want: 0, wantErr: false},
		{name: "plain number", input: "1024", want: 1024, wantErr: false},
		{name: "bytes suffix", input: "512B", want: 512, wantErr: false},
		{name: "lowercase bytes suffix", input: "512b", want: 512, wantErr: false},
		{name: "kilobytes KB", input: "10KB", want: 10 * 1024, wantErr: false},
		{name: "kilobytes K", input: "10K", want: 10 * 1024, wantErr: false},
		{name: "megabytes MB", input: "500MB", want: 500 * 1024 * 1024, wantErr: false},
		{name: "megabytes M", input: "10M", want: 10 * 1024 * 1024, wantErr: false},
		{name: "gigabytes GB", input: "2GB", want: 2 * 1024 * 1024 * 1024, wantErr: false},
		{name: "gigabytes G", input: "2G", want: 2 * 1024 * 1024 * 1024, wantErr: false},
		{name: "fractional gigabytes", input: "1.5GB", want: int64(1.5 * 1024 * 1024 * 1024), wantErr: false},
		{name: "terabytes TB", input: "1TB", want: 1024 * 1024 * 1024 * 1024, wantErr: false},
		{name: "petabytes PB", input: "1PB", want: 1024 * 1024 * 1024 * 1024 * 1024, wantErr: false},
		{name: "with spaces", input: "  100 MB  ", want: 100 * 1024 * 1024, wantErr: false},
		{name: "invalid characters", input: "invalid", want: 0, wantErr: true},
		{name: "invalid suffix", input: "100XYZ", want: 0, wantErr: true},
		{name: "non-numeric prefix", input: "abcGB", want: 0, wantErr: true},
		{name: "just suffix", input: "GB", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := format.ParseBytes(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseBytes(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseBytes(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseBytes(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
