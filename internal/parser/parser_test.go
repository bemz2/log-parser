package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseDBCSV(t *testing.T) {
	t.Parallel()

	parsed, err := New().ParseFile(context.Background(), filepath.Join("..", "..", "data", "ibdiagnet2.db_csv"))
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	nodes, ports := parsed.Counts()
	if nodes != 5 {
		t.Fatalf("nodes count = %d, want 5", nodes)
	}
	if ports == 0 {
		t.Fatal("ports count = 0, want non-zero")
	}
}

func TestParseSharpInfo(t *testing.T) {
	t.Parallel()

	parsed, err := New().ParseFile(context.Background(), filepath.Join("..", "..", "data", "ibdiagnet2.sharp_an_info"))
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	nodes, ports := parsed.Counts()
	if nodes != 4 {
		t.Fatalf("nodes count = %d, want 4", nodes)
	}
	if ports != 0 {
		t.Fatalf("ports count = %d, want 0", ports)
	}
}

func TestParseRejectsInvalidLog(t *testing.T) {
	t.Parallel()

	tmp := filepath.Join(t.TempDir(), "broken.log")
	if err := os.WriteFile(tmp, []byte("invalid"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := New().ParseFile(context.Background(), tmp)
	if err == nil {
		t.Fatal("ParseFile() error = nil, want error")
	}
}
