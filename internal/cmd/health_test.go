package cmd

import (
	"strings"
	"testing"

	"github.com/swifty99/hactl/internal/analyze"
)

func TestCountErrorEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []analyze.LogEntry
		want    int
	}{
		{
			name:    "no errors",
			entries: []analyze.LogEntry{{Level: "INFO"}, {Level: "WARNING"}},
			want:    0,
		},
		{
			name:    "two errors",
			entries: []analyze.LogEntry{{Level: "ERROR"}, {Level: "INFO"}, {Level: "ERROR"}},
			want:    2,
		},
		{
			name:    "empty",
			entries: nil,
			want:    0,
		},
		{
			name:    "case-sensitive: only exact ERROR counts",
			entries: []analyze.LogEntry{{Level: "ERROR"}, {Level: "error"}, {Level: "Error"}},
			want:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countErrorEntries(tt.entries)
			if got != tt.want {
				t.Errorf("countErrorEntries() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHealthCommand_NoEnv(t *testing.T) {
	// Call health without a valid instance directory → should fail with useful error.
	dir := t.TempDir() // no .env file

	rootCmd.SetArgs([]string{"health", "--dir", dir})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot open .env") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "cannot open .env")
	}
}
