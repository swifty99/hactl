package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatal("version command produced no output")
	}
	if !strings.Contains(out, "hactl") {
		t.Errorf("version output missing 'hactl': got %q", out)
	}
}

func TestRootCommandHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatal("help produced no output")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		bytes int64
		want  int64
	}{
		{0, 0},
		{4, 1},
		{100, 25},
		{1000, 250},
		{3, 1},
		{5, 2},
	}
	for _, tt := range tests {
		got := estimateTokens(tt.bytes)
		if got != tt.want {
			t.Errorf("estimateTokens(%d) = %d, want %d", tt.bytes, got, tt.want)
		}
	}
}

func TestStatsWriter(t *testing.T) {
	var inner bytes.Buffer
	sw := &statsWriter{inner: &inner}

	n, err := sw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("Write returned %d, want 5", n)
	}
	if sw.bytes != 5 {
		t.Errorf("bytes = %d, want 5", sw.bytes)
	}

	_, _ = sw.Write([]byte(" world"))
	if sw.bytes != 11 {
		t.Errorf("bytes = %d, want 11", sw.bytes)
	}
}

func TestWriteStats(t *testing.T) {
	var buf bytes.Buffer
	writeStats(&buf, 100)
	out := buf.String()
	if !strings.Contains(out, "100 bytes") {
		t.Errorf("stats output missing byte count: %s", out)
	}
	if !strings.Contains(out, "~25 tokens") {
		t.Errorf("stats output missing token count: %s", out)
	}
}
