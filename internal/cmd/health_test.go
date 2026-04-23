package cmd

import (
	"strings"
	"testing"
)

func TestCountErrorLines(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want int
	}{
		{
			name: "no errors",
			log:  "INFO something\nWARNING mild\n",
			want: 0,
		},
		{
			name: "two errors",
			log:  "ERROR one\nINFO ok\nERROR two\n",
			want: 2,
		},
		{
			name: "empty log",
			log:  "",
			want: 0,
		},
		{
			name: "error in middle of line",
			log:  "2025-01-15 12:00:00 ERROR (MainThread) [ha] broke\n",
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countErrorLines(tt.log)
			if got != tt.want {
				t.Errorf("countErrorLines() = %d, want %d", got, tt.want)
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
