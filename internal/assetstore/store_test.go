package assetstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeSegmentStripsTraversal(t *testing.T) {
	got := SafeSegment("../etc/passwd")
	if got == "../etc/passwd" || got == "" {
		t.Fatalf("SafeSegment = %q", got)
	}
	if filepath.IsAbs(got) || got == ".." {
		t.Fatalf("unsafe %q", got)
	}
}

func TestGetOriginalUsesDisk(t *testing.T) {
	dir := t.TempDir()
	path := originalDiskPath(dir, "Moon Pendant", "png", 128)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("png-bytes")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(dir, nil, nil)
	got, ct, err := s.GetOriginal(t.Context(), "Moon Pendant", "png", 128)
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/png" {
		t.Fatalf("ct = %q", ct)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q", got)
	}
}
