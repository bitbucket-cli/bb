package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManGeneration(t *testing.T) {
	dir := t.TempDir()
	if err := manCmd.RunE(manCmd, []string{dir}); err != nil {
		t.Fatalf("man generation error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected man pages to be generated")
	}

	// The root page should exist.
	if _, err := os.Stat(filepath.Join(dir, "bb.1")); err != nil {
		t.Errorf("expected bb.1 root man page: %v", err)
	}
}
