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

func TestNormalizePNGSize(t *testing.T) {
	t.Parallel()
	cases := map[int]int{0: 128, 64: 128, 100: 128, 128: 128, 200: 256, 256: 256, 512: 256, 1024: 256}
	for in, want := range cases {
		if got := NormalizePNGSize(in); got != want {
			t.Fatalf("NormalizePNGSize(%d)=%d want %d", in, got, want)
		}
	}
}

func TestOpenOriginalNoFetch(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, nil, nil)
	if _, _, err := s.OpenOriginal("Missing", "png", 128); err == nil {
		t.Fatal("expected miss")
	}
	path := originalDiskPath(dir, "Moon Pendant", "png", 128)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Request 256 → fall back to 128 on disk.
	got, ct, err := s.OpenOriginal("Moon Pendant", "png", 256)
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/png" || string(got) != "x" {
		t.Fatalf("got %q ct %q", got, ct)
	}
}
